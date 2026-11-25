import {
  FLOW_DISCLOSURES_MAX,
  HttpNttFlowSource,
  MANAGERS_MAX,
  NttFlowServiceError,
  StaticNttFlowSource,
  parseFlowResponse,
  parseManagerSummary,
  parseManagersResponse,
} from "../src/index.js";
import { SANDBOX } from "./fixtures/sandbox.js";
import { SETUP, copy, mockFetch } from "./helpers.js";

const LOCK_FLOW = SANDBOX.lockTransfer.flow;

describe("parseFlowResponse", () => {
  it("parses a recorded flow (the first lock: no custody pot yet), keeping the served payload", () => {
    const flow = parseFlowResponse(LOCK_FLOW, "transfer");
    expect(flow.disclosures.map((entry) => entry.role)).toEqual([
      "NttManager",
      "transceiver Emitter",
      "CoreState",
      "LockedLedger",
      "committed TransferFactory",
      "custodian TransferPreapproval",
    ]);
    expect(flow.missingRoles).toEqual([]);
    expect(flow.disclosures[0]!.payload).toEqual(
      LOCK_FLOW.disclosures[0]!.createArgument
    );
  });

  it("names the missing roles", () => {
    expect(
      parseFlowResponse(SANDBOX.releaseMissingPreapproval, "release")
        .missingRoles
    ).toEqual(["recipient TransferPreapproval"]);
  });

  const entry = LOCK_FLOW.disclosures[2]!;
  const rejections: Array<[string, unknown, RegExp]> = [
    ["a response that is not an object", [], /not an object/],
    [
      "another flow",
      { ...LOCK_FLOW, flow: "mint" },
      /asked for flow "transfer", got flow "mint"/,
    ],
    [
      "no missing array",
      { flow: "transfer", disclosures: [] },
      /disclosures and missing arrays/,
    ],
    [
      "a repeated role",
      { ...LOCK_FLOW, disclosures: [entry, entry] },
      /role "CoreState" appears more than once/,
    ],
    [
      "an empty blob",
      { ...LOCK_FLOW, disclosures: [{ ...entry, createdEventBlob: "" }] },
      /CoreState createdEventBlob must be a non-empty string/,
    ],
    [
      "a blob that is not base64",
      {
        ...LOCK_FLOW,
        disclosures: [{ ...entry, createdEventBlob: "not base64!" }],
      },
      /createdEventBlob is not base64/,
    ],
    [
      "a malformed contract id",
      { ...LOCK_FLOW, disclosures: [{ ...entry, contractId: "xyz" }] },
      /Invalid Canton contract id/,
    ],
    [
      "two synchronizers",
      {
        ...LOCK_FLOW,
        disclosures: [
          entry,
          { ...LOCK_FLOW.disclosures[0]!, synchronizerId: "other::1220aa" },
        ],
      },
      /more than one synchronizer/,
    ],
    [
      "too many disclosures",
      {
        ...LOCK_FLOW,
        disclosures: Array.from(
          { length: FLOW_DISCLOSURES_MAX + 1 },
          (_, index) => ({
            ...entry,
            role: `role ${index}`,
          })
        ),
      },
      /more than 32 disclosures/,
    ],
  ];
  it.each(rejections)("rejects %s", (_name, response, pattern) => {
    expect(() => parseFlowResponse(response, "transfer")).toThrow(pattern);
  });
});

describe("parseManagersResponse", () => {
  it("parses the recorded summary, with the A6 fields", () => {
    const managers = parseManagersResponse(SANDBOX.managers);
    const locking = managers.find(
      (manager) => manager.managerAddress === SETUP.locking.managerAddress
    )!;
    expect(locking).toMatchObject({
      chainId: 75,
      tokenConfig: "LockUnlock",
      instrumentId: {
        admin: SETUP.locking.instrumentAdmin,
        id: SETUP.locking.instrumentId,
      },
      reserveCustodian: SETUP.nttCustody,
      admin: SETUP.guardianGovernance,
      paused: false,
      tokenDecimals: 10,
    });
    expect(typeof locking.managerId).toBe("bigint");
    expect(locking.peers?.get(2)).toEqual({
      managerAddress:
        "000000000000000000000000b1ff5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f",
      transceiverAddress:
        "000000000000000000000000c2aa5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f",
      decimals: 18,
    });
  });

  const manager = SANDBOX.managers.managers[0]!;
  const rejections: Array<[string, unknown, RegExp]> = [
    [
      "an unknown tokenConfig",
      { ...manager, tokenConfig: "Other" },
      /unknown tokenConfig "Other"/,
    ],
    [
      "a paused that is not a boolean",
      { ...manager, paused: "false" },
      /paused must be a boolean/,
    ],
    [
      "a managerId that is a number",
      { ...manager, managerId: 0 },
      /managerId must be a non-empty string/,
    ],
    [
      "a managerId past a Daml Int",
      { ...manager, managerId: "9223372036854775808" },
      /not a decimal string of a Daml Int/,
    ],
    [
      "uppercase hex",
      { ...manager, managerAddress: manager.managerAddress.toUpperCase() },
      /64 lowercase hex/,
    ],
    [
      "tokenDecimals above 10",
      { ...manager, tokenDecimals: 11 },
      /tokenDecimals must be an integer from 0 to 10/,
    ],
    [
      "a peer key that is not a chain id",
      { ...manager, peers: { x: manager.peers["2"] } },
      /peers key x is not a chain id/,
    ],
  ];
  it.each(rejections)("rejects %s", (_name, entry, pattern) => {
    expect(() => parseManagerSummary(entry)).toThrow(pattern);
  });

  it("accepts integers written as decimal strings, and no A6 fields", () => {
    const {
      tokenDecimals: _decimals,
      peers: _peers,
      ...withoutA6
    } = copy(manager);
    const parsed = parseManagerSummary({ ...withoutA6, chainId: "75" });
    expect(parsed.chainId).toBe(75);
    expect(parsed.tokenDecimals).toBeUndefined();
    expect(parsed.peers).toBeUndefined();
  });

  it("rejects a repeated manager address and more than the cap", () => {
    expect(() =>
      parseManagersResponse({ managers: [manager, manager] })
    ).toThrow(/more than once/);
    expect(() =>
      parseManagersResponse({
        managers: Array.from({ length: MANAGERS_MAX + 1 }, () => manager),
      })
    ).toThrow(/more than 512 managers/);
    expect(() => parseManagersResponse({})).toThrow(/no managers array/);
  });
});

describe("HttpNttFlowSource", () => {
  it("requests a flow with its parameters and the manager summary", async () => {
    const query = `manager=${SETUP.locking.managerAddress}`;
    const { fetchFunction, requests } = mockFetch({
      [`/base/v1/flows/transfer?${query}`]: [{ json: LOCK_FLOW }],
      "/base/v1/managers": [{ json: SANDBOX.managers }],
    });
    const source = new HttpNttFlowSource({
      baseUrl: "https://flows.test/base/",
      fetch: fetchFunction,
    });
    const flow = await source.flow("transfer", {
      manager: SETUP.locking.managerAddress,
    });
    expect(flow.disclosures).toHaveLength(6);
    expect(await source.managers()).toHaveLength(3);
    expect(requests.map((request) => request.path)).toEqual([
      `/base/v1/flows/transfer?${query}`,
      "/base/v1/managers",
    ]);
  });

  it("reports the service's status, message and request id", async () => {
    const { fetchFunction } = mockFetch({
      "/v1/flows/mint": [{ status: 404, json: SANDBOX.mintNotFound }],
      "/v1/managers": [
        { status: 503, json: { error: "service busy", requestId: "r-1" } },
      ],
    });
    const source = new HttpNttFlowSource({
      baseUrl: "http://flows.test",
      fetch: fetchFunction,
    });
    const notFound = await source
      .flow("mint", { manager: "ab".repeat(32) })
      .catch((err) => err);
    expect(notFound).toBeInstanceOf(NttFlowServiceError);
    expect(notFound.notFound).toBe(true);
    expect(notFound.message).toMatch(
      /HTTP 404: disclosure-service: no DepositPreapproval found/
    );
    const busy = await source.managers().catch((err) => err);
    expect(busy.retryable).toBe(true);
    expect(busy.message).toMatch(/HTTP 503: service busy \(request r-1\)/);
  });

  it("refuses a URL with a query or another scheme", () => {
    expect(
      () => new HttpNttFlowSource({ baseUrl: "http://flows.test/?x=1" })
    ).toThrow(/query/);
    expect(
      () => new HttpNttFlowSource({ baseUrl: "ftp://flows.test" })
    ).toThrow(/http or https/);
  });
});

describe("StaticNttFlowSource", () => {
  it("answers recorded flows by exact parameters and 404 otherwise", async () => {
    const flow = parseFlowResponse(LOCK_FLOW, "transfer");
    const source = new StaticNttFlowSource({
      flows: [
        { flow: "transfer", parameters: { manager: "aa" }, response: flow },
      ],
      managers: [],
    });
    expect(await source.flow("transfer", { manager: "aa" })).toBe(flow);
    await expect(source.flow("transfer", { manager: "bb" })).rejects.toThrow(
      /HTTP 404/
    );
  });
});
