// Client of the NTT flow service (canton/disclosure): the disclosure sets of the NTT flows and
// the manager summary. Ported from the ntt CLI's former disclosure client, which this replaces,
// with the same bounds and checks. The service is trusted for liveness only: every flow passes
// the anchor checks (anchor.ts) before use, and a served payload is never a trust anchor.
import type {
  DisclosedContract,
  FetchFunction,
} from "@wormhole-foundation/sdk-canton";
import {
  fetchJsonBounded,
  parseCantonContractId,
  parseCantonPartyId,
} from "@wormhole-foundation/sdk-canton";
import {
  FLOW_DISCLOSURES_MAX,
  FLOW_REQUEST_TIMEOUT_MS,
  FLOW_RESPONSE_MAX_BYTES,
  MANAGERS_MAX,
} from "./constants.js";
import type { NttTokenConfig } from "./nttContracts.js";

/**
 * The flows this client reads. `set-peer` serves the governance path (SetPeerByVaa); this client
 * reads it only for the covering ReplayNode of a digest, the one flow that serves that node for
 * either mode without a recipient (getIsExecuted).
 */
export type NttFlowName = "transfer" | "release" | "mint" | "set-peer";

/** One flow's disclosure set */
export interface NttFlow {
  flow: NttFlowName;
  /** Each entry's `payload` is the served createArgument, when the service sent one */
  disclosures: DisclosedContract[];
  /** Roles the service could not see; the client must supply them or stop */
  missingRoles: string[];
}

/** A peer of a manager, as /v1/managers summarises it */
export interface NttManagerSummaryPeer {
  /** Lowercase hex */
  managerAddress: string;
  /** Lowercase hex */
  transceiverAddress: string;
  decimals: number;
}

/** One /v1/managers entry. For display: never used for a safety check on its own. */
export interface NttManagerSummary {
  /** Lowercase hex */
  managerAddress: string;
  contractId: string;
  chainId: number;
  /** The decimal value of a Daml Int */
  managerId: bigint;
  tokenConfig: NttTokenConfig;
  instrumentId: { admin: string; id: string };
  reserveCustodian: string;
  paused: boolean;
  /** Absent when the service read no tokenDecimals */
  tokenDecimals?: number;
  admin: string;
  /** By Wormhole chain id; absent when the service read no peers */
  peers?: Map<number, NttManagerSummaryPeer>;
}

/** Where NTT flow disclosure sets and the manager summary come from */
export interface NttFlowSource {
  /** One flow's disclosure set. Parameters are the service's query parameters. */
  flow(
    flow: NttFlowName,
    parameters: Readonly<Record<string, string>>
  ): Promise<NttFlow>;
  /** The visible deployments */
  managers(): Promise<NttManagerSummary[]>;
}

/** A request the flow service refused or could not serve */
export class NttFlowServiceError extends Error {
  constructor(
    readonly route: string,
    readonly status: number,
    /** The service's message (a 4xx explains the request; a 5xx is fixed text) */
    readonly publicMessage: string | undefined,
    readonly requestId: string | undefined
  ) {
    super(
      `NTT flow service ${route} answered HTTP ${status}` +
        (publicMessage ? `: ${publicMessage}` : "") +
        (requestId ? ` (request ${requestId})` : "")
    );
    this.name = "NttFlowServiceError";
  }

  /** 404: the service found no contract for the request (an unknown manager, no preapproval) */
  get notFound(): boolean {
    return this.status === 404;
  }

  /** 503: the service is busy or not ready; the same request can succeed later */
  get retryable(): boolean {
    return this.status === 503;
  }
}

function isObject(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function nonEmptyText(value: unknown, fieldName: string): string {
  if (typeof value !== "string" || value.length === 0)
    throw new Error(
      `NTT flow service: ${fieldName} must be a non-empty string`
    );
  return value;
}

const BASE64 = /^[A-Za-z0-9+/]+={0,2}$/;
const HEX_32 = /^[0-9a-f]{64}$/;

function hex32(value: unknown, fieldName: string): string {
  const text = nonEmptyText(value, fieldName);
  if (!HEX_32.test(text))
    throw new Error(
      `NTT flow service: ${fieldName} must be 64 lowercase hex characters`
    );
  return text;
}

/** An integer the service writes as a JSON number or as a decimal string */
function integerField(
  value: unknown,
  fieldName: string,
  min: number,
  max: number
): number {
  const number =
    typeof value === "number"
      ? value
      : typeof value === "string" && /^-?\d{1,20}$/.test(value)
        ? Number(value)
        : Number.NaN;
  if (!Number.isSafeInteger(number) || number < min || number > max)
    throw new Error(
      `NTT flow service: ${fieldName} must be an integer from ${min} to ${max}`
    );
  return number;
}

/** Parses and bounds a flow response (port of the CLI's parseFlowResponse) */
export function parseFlowResponse(
  responseJson: unknown,
  requestedFlow: NttFlowName
): NttFlow {
  if (!isObject(responseJson))
    throw new Error("NTT flow service: flow response is not an object");
  if (responseJson["flow"] !== requestedFlow)
    throw new Error(
      `NTT flow service: asked for flow "${requestedFlow}", got flow ${JSON.stringify(
        responseJson["flow"]
      )?.slice(0, 40)}`
    );
  const disclosuresJson = responseJson["disclosures"];
  const missingJson = responseJson["missing"];
  if (!Array.isArray(disclosuresJson) || !Array.isArray(missingJson))
    throw new Error(
      "NTT flow service: flow response needs disclosures and missing arrays"
    );
  if (disclosuresJson.length > FLOW_DISCLOSURES_MAX)
    throw new Error(
      `NTT flow service: more than ${FLOW_DISCLOSURES_MAX} disclosures`
    );
  if (missingJson.length > FLOW_DISCLOSURES_MAX)
    throw new Error(
      `NTT flow service: more than ${FLOW_DISCLOSURES_MAX} missing roles`
    );
  const seenRoles = new Set<string>();
  const disclosures = disclosuresJson.map(
    (entry: unknown): DisclosedContract => {
      if (!isObject(entry))
        throw new Error("NTT flow service: a disclosure is not an object");
      const role = nonEmptyText(entry["role"], "role");
      if (seenRoles.has(role))
        throw new Error(
          `NTT flow service: role "${role.slice(0, 64)}" appears more than once`
        );
      seenRoles.add(role);
      const createdEventBlob = nonEmptyText(
        entry["createdEventBlob"],
        `${role} createdEventBlob`
      );
      if (!BASE64.test(createdEventBlob))
        throw new Error(
          `NTT flow service: ${role} createdEventBlob is not base64`
        );
      const createArgument = entry["createArgument"];
      return {
        role,
        templateId: nonEmptyText(entry["templateId"], `${role} templateId`),
        contractId: parseCantonContractId(
          nonEmptyText(entry["contractId"], `${role} contractId`)
        ),
        createdEventBlob,
        synchronizerId: nonEmptyText(
          entry["synchronizerId"],
          `${role} synchronizerId`
        ),
        ...(createArgument !== undefined && createArgument !== null
          ? { payload: createArgument }
          : {}),
      };
    }
  );
  const synchronizerIds = new Set(
    disclosures.map((disclosure) => disclosure.synchronizerId)
  );
  if (synchronizerIds.size > 1)
    throw new Error(
      "NTT flow service: disclosures name more than one synchronizer"
    );
  const missingRoles = missingJson.map((entry: unknown) =>
    nonEmptyText(isObject(entry) ? entry["role"] : undefined, "missing role")
  );
  return { flow: requestedFlow, disclosures, missingRoles };
}

function parsePeers(
  value: unknown,
  what: string
): Map<number, NttManagerSummaryPeer> {
  if (!isObject(value))
    throw new Error(`NTT flow service: ${what}.peers is not an object`);
  const peers = new Map<number, NttManagerSummaryPeer>();
  for (const [key, peer] of Object.entries(value)) {
    if (!/^\d{1,5}$/.test(key))
      throw new Error(
        `NTT flow service: ${what}.peers key ${key.slice(0, 20)} is not a chain id`
      );
    const chainId = integerField(key, `${what}.peers key`, 1, 65535);
    if (!isObject(peer))
      throw new Error(
        `NTT flow service: ${what}.peers[${chainId}] is not an object`
      );
    peers.set(chainId, {
      managerAddress: hex32(
        peer["managerAddress"],
        `${what}.peers[${chainId}].managerAddress`
      ),
      transceiverAddress: hex32(
        peer["transceiverAddress"],
        `${what}.peers[${chainId}].transceiverAddress`
      ),
      decimals: integerField(
        peer["decimals"],
        `${what}.peers[${chainId}].decimals`,
        1,
        255
      ),
    });
  }
  return peers;
}

/** Parses one /v1/managers entry (port of the CLI's parseManagerSummary, plus the A6 fields) */
export function parseManagerSummary(entry: unknown): NttManagerSummary {
  if (!isObject(entry))
    throw new Error("NTT flow service: a manager entry is not an object");
  const tokenConfig = entry["tokenConfig"];
  if (tokenConfig !== "LockUnlock" && tokenConfig !== "BurnMint")
    throw new Error(
      `NTT flow service: unknown tokenConfig ${JSON.stringify(tokenConfig)?.slice(0, 40)}`
    );
  if (typeof entry["paused"] !== "boolean")
    throw new Error("NTT flow service: paused must be a boolean");
  const managerAddress = hex32(entry["managerAddress"], "managerAddress");
  const what = `manager ${managerAddress}`;
  const managerIdText = nonEmptyText(entry["managerId"], `${what}.managerId`);
  if (
    !/^(?:0|[1-9]\d{0,18})$/.test(managerIdText) ||
    BigInt(managerIdText) > 2n ** 63n - 1n
  )
    throw new Error(
      `NTT flow service: ${what}.managerId is not a decimal string of a Daml Int`
    );
  const instrument = entry["instrumentId"];
  if (!isObject(instrument))
    throw new Error(`NTT flow service: ${what}.instrumentId is not an object`);
  return {
    managerAddress,
    contractId: parseCantonContractId(
      nonEmptyText(entry["contractId"], `${what}.contractId`)
    ),
    chainId: integerField(entry["chainId"], `${what}.chainId`, 1, 65535),
    managerId: BigInt(managerIdText),
    tokenConfig,
    instrumentId: {
      admin: parseCantonPartyId(
        nonEmptyText(instrument["admin"], `${what}.instrumentId.admin`)
      ),
      id: nonEmptyText(instrument["id"], `${what}.instrumentId.id`),
    },
    reserveCustodian: parseCantonPartyId(
      nonEmptyText(entry["reserveCustodian"], `${what}.reserveCustodian`)
    ),
    paused: entry["paused"],
    ...(entry["tokenDecimals"] !== undefined
      ? {
          tokenDecimals: integerField(
            entry["tokenDecimals"],
            `${what}.tokenDecimals`,
            0,
            10
          ),
        }
      : {}),
    admin: parseCantonPartyId(nonEmptyText(entry["admin"], `${what}.admin`)),
    ...(entry["peers"] !== undefined
      ? { peers: parsePeers(entry["peers"], what) }
      : {}),
  };
}

/** Parses and bounds a /v1/managers response */
export function parseManagersResponse(
  responseJson: unknown
): NttManagerSummary[] {
  const managers = isObject(responseJson)
    ? responseJson["managers"]
    : undefined;
  if (!Array.isArray(managers))
    throw new Error(
      "NTT flow service: /v1/managers response has no managers array"
    );
  if (managers.length > MANAGERS_MAX)
    throw new Error(`NTT flow service: more than ${MANAGERS_MAX} managers`);
  const parsed = managers.map(parseManagerSummary);
  if (
    new Set(parsed.map((manager) => manager.managerAddress)).size !==
    parsed.length
  )
    throw new Error(
      "NTT flow service: /v1/managers lists a manager address more than once"
    );
  return parsed;
}

export interface HttpNttFlowSourceOptions {
  /** Base URL of the service, for example an entry of nttFlowServiceUrls */
  baseUrl: string;
  fetch?: FetchFunction;
}

/** The NTT flow service over HTTP */
export class HttpNttFlowSource implements NttFlowSource {
  readonly baseUrl: string;
  private readonly fetchFunction: FetchFunction | undefined;

  constructor(options: HttpNttFlowSourceOptions) {
    const parsed = new URL(options.baseUrl);
    if (parsed.protocol !== "https:" && parsed.protocol !== "http:")
      throw new Error(
        `NTT flow service URL must be http or https, got ${parsed.protocol}`
      );
    if (parsed.search !== "" || parsed.hash !== "")
      throw new Error(
        "NTT flow service URL must not have a query or a fragment"
      );
    this.baseUrl = `${parsed.origin}${parsed.pathname.replace(/\/+$/, "")}`;
    this.fetchFunction = options.fetch;
  }

  private async getJson(route: string, query = ""): Promise<unknown> {
    const response = await fetchJsonBounded(
      `${this.baseUrl}${route}${query ? `?${query}` : ""}`,
      { method: "GET", headers: { Accept: "application/json" } },
      { timeoutMs: FLOW_REQUEST_TIMEOUT_MS, maxBytes: FLOW_RESPONSE_MAX_BYTES },
      this.fetchFunction
    );
    if (response.status !== 200) {
      const json = isObject(response.json) ? response.json : {};
      throw new NttFlowServiceError(
        route,
        response.status,
        typeof json["error"] === "string"
          ? json["error"].slice(0, 200)
          : undefined,
        typeof json["requestId"] === "string"
          ? json["requestId"].slice(0, 64)
          : undefined
      );
    }
    return response.json;
  }

  async flow(
    flow: NttFlowName,
    parameters: Readonly<Record<string, string>>
  ): Promise<NttFlow> {
    const json = await this.getJson(
      `/v1/flows/${flow}`,
      new URLSearchParams(parameters as Record<string, string>).toString()
    );
    return parseFlowResponse(json, flow);
  }

  async managers(): Promise<NttManagerSummary[]> {
    return parseManagersResponse(await this.getJson("/v1/managers"));
  }
}

/** One recorded flow of a StaticNttFlowSource, matched on its exact parameters */
export interface StaticNttFlowEntry {
  flow: NttFlowName;
  parameters: Readonly<Record<string, string>>;
  response: NttFlow;
}

/** Flows and managers the caller supplies: tests, and integrators with their own index */
export class StaticNttFlowSource implements NttFlowSource {
  constructor(
    private readonly entries: {
      flows: readonly StaticNttFlowEntry[];
      managers: readonly NttManagerSummary[];
    }
  ) {}

  async flow(
    flow: NttFlowName,
    parameters: Readonly<Record<string, string>>
  ): Promise<NttFlow> {
    const key = JSON.stringify(Object.entries(parameters).sort());
    const entry = this.entries.flows.find(
      (candidate) =>
        candidate.flow === flow &&
        JSON.stringify(Object.entries(candidate.parameters).sort()) === key
    );
    if (!entry)
      throw new NttFlowServiceError(
        `/v1/flows/${flow}`,
        404,
        "no recorded flow",
        undefined
      );
    return entry.response;
  }

  async managers(): Promise<NttManagerSummary[]> {
    return [...this.entries.managers];
  }
}
