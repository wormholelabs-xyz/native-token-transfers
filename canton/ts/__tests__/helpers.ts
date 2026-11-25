// Shared test helpers: a routed fetch mock, and CantonNtt instances over the recorded sandbox run.
import type { Network } from "@wormhole-foundation/sdk-base";
import type { FetchFunction } from "@wormhole-foundation/sdk-canton";
import { CantonLedgerClient } from "@wormhole-foundation/sdk-canton";
import type {
  CantonNttContracts,
  CantonNttOptions,
  NttFlowSource,
} from "../src/index.js";
import type { NttFlowName } from "../src/index.js";
import {
  CantonNtt,
  StaticNttFlowSource,
  parseFlowResponse,
  parseManagersResponse,
} from "../src/index.js";
import { SANDBOX } from "./fixtures/sandbox.js";

export interface MockReply {
  status?: number;
  json: unknown;
}

export interface MockRequest {
  path: string;
  method: string;
  body: unknown;
}

/**
 * A fetch that answers by URL path (with the query for the flow routes). Each path has a queue of
 * replies; the last one repeats.
 */
export function mockFetch(routes: Record<string, MockReply[]>): {
  fetchFunction: FetchFunction;
  requests: MockRequest[];
} {
  const requests: MockRequest[] = [];
  const queues = new Map(
    Object.entries(routes).map(([path, replies]) => [path, [...replies]])
  );
  const fetchFunction: FetchFunction = async (url, init) => {
    const parsed = new URL(url);
    const pathWithQuery = `${parsed.pathname}${parsed.search}`;
    const key = queues.has(pathWithQuery) ? pathWithQuery : parsed.pathname;
    requests.push({
      path: pathWithQuery,
      method: init.method ?? "GET",
      body: typeof init.body === "string" ? JSON.parse(init.body) : undefined,
    });
    const queue = queues.get(key);
    if (!queue || queue.length === 0)
      throw new Error(`unexpected request ${init.method} ${pathWithQuery}`);
    const reply = queue.length > 1 ? queue.shift()! : queue[0]!;
    return new Response(JSON.stringify(reply.json), {
      status: reply.status ?? 200,
    });
  };
  return { fetchFunction, requests };
}

export const SETUP = SANDBOX.setup;
export const MANAGERS = parseManagersResponse(SANDBOX.managers);

export type DeploymentName = "burning" | "locking" | "owned";

export function contractsOf(
  deployment: DeploymentName,
  canton: CantonNttContracts["canton"] = {},
  overrides: Partial<CantonNttContracts> = {}
) {
  const setup = SETUP[deployment];
  return {
    coreBridge: SETUP.guardianGovernance,
    ntt: {
      manager: setup.managerAddress,
      token: `${setup.instrumentAdmin}/${setup.instrumentId}`,
      tokenDecimals: setup.tokenDecimals,
      transceiver: { wormhole: setup.transceiverAddress },
      canton: { reserveCustodian: setup.reserveCustodian, ...canton },
      ...overrides,
    },
  };
}

/** The recorded flows, as a static source keyed by their recorded parameters */
export function recordedFlowSource(
  managersResponse: unknown = SANDBOX.managers
): StaticNttFlowSource {
  const entries = [
    SANDBOX.burnTransfer,
    SANDBOX.lockTransfer,
    SANDBOX.mint,
    SANDBOX.release,
  ].map((recorded) => {
    const url = new URL(recorded.flowUrl, "http://flows.test");
    const flow = url.pathname.split("/").at(-1) as
      | "transfer"
      | "mint"
      | "release";
    return {
      flow,
      parameters: Object.fromEntries(url.searchParams.entries()),
      response: parseFlowResponse(recorded.flow, flow),
    };
  });
  return new StaticNttFlowSource({
    flows: entries,
    managers: parseManagersResponse(managersResponse),
  });
}

export function nttOf(
  deployment: DeploymentName,
  options: {
    flowSource?: NttFlowSource;
    ledgerFetch?: FetchFunction;
    canton?: CantonNttContracts["canton"];
    overrides?: Partial<CantonNttContracts>;
    nttOptions?: CantonNttOptions;
  } = {}
): CantonNtt<Network, "Canton"> {
  const ledger = new CantonLedgerClient({
    baseUrl: "http://participant.test",
    ...(options.ledgerFetch ? { fetch: options.ledgerFetch } : {}),
  });
  return CantonNtt.withOptions<Network, "Canton">(
    "Devnet",
    "Canton",
    ledger,
    contractsOf(deployment, options.canton, options.overrides),
    {
      flowSource: options.flowSource ?? recordedFlowSource(),
      rebuildDelayMs: 0,
      ...options.nttOptions,
    }
  );
}

export async function collect<T>(generator: AsyncGenerator<T>): Promise<T[]> {
  const items: T[] = [];
  for await (const item of generator) items.push(item);
  return items;
}

/** A deep copy of recorded JSON, to alter in a test */
export function copy<T>(value: T): T {
  return JSON.parse(JSON.stringify(value)) as T;
}

/** A static source of recorded flows (URL and response body) and a recorded /v1/managers body */
export function flowSourceOf(
  flows: ReadonlyArray<{ flowUrl: string; flow: unknown }>,
  managersResponse: unknown
): StaticNttFlowSource {
  return new StaticNttFlowSource({
    flows: flows.map((recorded) => {
      const url = new URL(recorded.flowUrl, "http://flows.test");
      const flow = url.pathname.split("/").at(-1) as NttFlowName;
      return {
        flow,
        parameters: Object.fromEntries(url.searchParams.entries()),
        response: parseFlowResponse(recorded.flow, flow),
      };
    }),
    managers: parseManagersResponse(managersResponse),
  });
}

/** A participant that answers ledger-end and one active-contracts response */
export function acsLedger(activeContracts: unknown) {
  return mockFetch({
    "/v2/state/ledger-end": [{ json: { offset: 100 } }],
    "/v2/state/active-contracts": [{ json: activeContracts }],
  });
}

/** An active-contracts response holding served flow entries, as the JSON Ledger API writes it */
export function activeContractsOf(
  entries: ReadonlyArray<{
    contractId: string;
    templateId: string;
    createdEventBlob: string;
    synchronizerId: string;
    createArgument?: unknown;
  }>
): unknown[] {
  return entries.map((entry) => ({
    workflowId: "",
    contractEntry: {
      JsActiveContract: {
        createdEvent: {
          contractId: entry.contractId,
          templateId: entry.templateId,
          createdEventBlob: entry.createdEventBlob,
          createArgument: entry.createArgument ?? null,
        },
        synchronizerId: entry.synchronizerId,
        reassignmentCounter: 0,
      },
    },
  }));
}
