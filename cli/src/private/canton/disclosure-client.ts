// Client of the NTT disclosure service flow routes (canton/disclosure/README.md). The service is
// untrusted: callers compare its answers with the registry before a submit.
import type { CantonInstrumentId } from "../config.js";
import {
  parseCantonContractId,
  parseCantonPartyId,
  parseHex32,
  type CantonContractId,
  type CantonPartyId,
  type Hex32,
} from "../types.js";
import { errorBodyExcerpt, fetchJsonBounded } from "./http.js";

export type FlowName = "transfer" | "release" | "mint";
export const FLOW_DISCLOSURES_MAX = 32;
export const MANAGERS_MAX = 512;

export interface FlowDisclosure {
  role: string;
  templateId: string;
  contractId: CantonContractId;
  createdEventBlob: string;
  synchronizerId: string;
}

export interface FlowResponse {
  flow: FlowName;
  disclosures: FlowDisclosure[];
  missingRoles: string[];
}

export interface ManagerSummary {
  managerAddress: Hex32;
  contractId: CantonContractId;
  tokenConfig: "LockUnlock" | "BurnMint";
  instrumentId: CantonInstrumentId;
  reserveCustodian: CantonPartyId;
  paused: boolean;
}

export interface DisclosureServiceClient {
  fetchFlow(
    flow: FlowName,
    parameters: Record<string, string>
  ): Promise<FlowResponse>;
  fetchManagers(): Promise<ManagerSummary[]>;
}

export class DisclosureServiceNotFoundError extends Error {
  constructor(message: string) {
    super(message);
    this.name = "DisclosureServiceNotFoundError";
  }
}

function readNonEmptyText(value: unknown, fieldName: string): string {
  if (typeof value !== "string" || value.length === 0) {
    throw new Error(
      `disclosure service: ${fieldName} must be a non-empty string`
    );
  }
  return value;
}

export function parseFlowResponse(
  responseJson: unknown,
  requestedFlow: FlowName
): FlowResponse {
  if (
    typeof responseJson !== "object" ||
    responseJson === null ||
    Array.isArray(responseJson)
  ) {
    throw new Error("disclosure service: flow response is not an object");
  }
  const response = responseJson as {
    flow?: unknown;
    disclosures?: unknown;
    missing?: unknown;
  };
  if (response.flow !== requestedFlow) {
    throw new Error(
      `disclosure service: asked for flow "${requestedFlow}", got flow ${JSON.stringify(response.flow)}`
    );
  }
  if (
    !Array.isArray(response.disclosures) ||
    !Array.isArray(response.missing)
  ) {
    throw new Error(
      "disclosure service: flow response needs disclosures and missing arrays"
    );
  }
  if (response.disclosures.length > FLOW_DISCLOSURES_MAX) {
    throw new Error(
      `disclosure service: more than ${FLOW_DISCLOSURES_MAX} disclosures`
    );
  }
  const seenRoles = new Set<string>();
  const disclosures = response.disclosures.map((entry: any) => {
    const role = readNonEmptyText(entry?.role, "role");
    if (seenRoles.has(role))
      throw new Error(
        `disclosure service: role "${role}" appears more than once`
      );
    seenRoles.add(role);
    return {
      role,
      templateId: readNonEmptyText(entry.templateId, `${role} templateId`),
      contractId: parseCantonContractId(
        readNonEmptyText(entry.contractId, `${role} contractId`)
      ),
      createdEventBlob: readNonEmptyText(
        entry.createdEventBlob,
        `${role} createdEventBlob`
      ),
      synchronizerId: readNonEmptyText(
        entry.synchronizerId,
        `${role} synchronizerId`
      ),
    };
  });
  const synchronizerIds = new Set(
    disclosures.map((disclosure) => disclosure.synchronizerId)
  );
  if (synchronizerIds.size > 1)
    throw new Error(
      "disclosure service: disclosures name more than one synchronizer"
    );
  const missingRoles = response.missing.map((entry: any) =>
    readNonEmptyText(entry?.role, "missing role")
  );
  return { flow: requestedFlow, disclosures, missingRoles };
}

export function requireFlowRole(
  flow: FlowResponse,
  role: string
): FlowDisclosure {
  const matches = flow.disclosures.filter(
    (disclosure) => disclosure.role === role
  );
  if (matches.length !== 1) {
    throw new Error(
      `disclosure service flow "${flow.flow}" has no "${role}" role; nothing has been submitted`
    );
  }
  return matches[0];
}

function parseManagerSummary(entry: any): ManagerSummary {
  if (
    entry?.tokenConfig !== "LockUnlock" &&
    entry?.tokenConfig !== "BurnMint"
  ) {
    throw new Error(
      `disclosure service: unknown tokenConfig ${JSON.stringify(entry?.tokenConfig)}`
    );
  }
  if (typeof entry.paused !== "boolean")
    throw new Error("disclosure service: paused must be a boolean");
  return {
    managerAddress: parseHex32(
      readNonEmptyText(entry.managerAddress, "managerAddress")
    ),
    contractId: parseCantonContractId(
      readNonEmptyText(entry.contractId, "manager contractId")
    ),
    tokenConfig: entry.tokenConfig,
    instrumentId: {
      admin: parseCantonPartyId(
        readNonEmptyText(entry.instrumentId?.admin, "instrumentId.admin")
      ),
      id: readNonEmptyText(entry.instrumentId?.id, "instrumentId.id"),
    },
    reserveCustodian: parseCantonPartyId(
      readNonEmptyText(entry.reserveCustodian, "reserveCustodian")
    ),
    paused: entry.paused,
  };
}

export class HttpDisclosureServiceClient implements DisclosureServiceClient {
  constructor(private readonly serviceUrl: URL) {}

  private async getJson(
    servicePath: string
  ): Promise<{ status: number; json: unknown; text: string }> {
    return fetchJsonBounded(new URL(servicePath, this.serviceUrl).href, {
      method: "GET",
      headers: { Accept: "application/json" },
    });
  }

  async fetchFlow(
    flow: FlowName,
    parameters: Record<string, string>
  ): Promise<FlowResponse> {
    const response = await this.getJson(
      `/v1/flows/${flow}?${new URLSearchParams(parameters).toString()}`
    );
    const errorText = (response.json as { error?: unknown } | undefined)?.error;
    if (response.status === 404) {
      throw new DisclosureServiceNotFoundError(
        `flow ${flow}: ${typeof errorText === "string" ? errorText : "not found"}`
      );
    }
    if (response.status !== 200) {
      throw new Error(
        `disclosure service flow ${flow} failed with HTTP ${response.status}: ${errorBodyExcerpt(response.text)}`
      );
    }
    return parseFlowResponse(response.json, flow);
  }

  async fetchManagers(): Promise<ManagerSummary[]> {
    const response = await this.getJson("/v1/managers");
    const managers = (response.json as { managers?: unknown } | undefined)
      ?.managers;
    if (response.status !== 200 || !Array.isArray(managers)) {
      throw new Error(
        `disclosure service /v1/managers failed with HTTP ${response.status}`
      );
    }
    if (managers.length > MANAGERS_MAX)
      throw new Error(`disclosure service: more than ${MANAGERS_MAX} managers`);
    return managers.map(parseManagerSummary);
  }
}
