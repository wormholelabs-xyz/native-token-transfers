// `ntt private` Canton send and redeem on the Canton NTT SDK, over the recorded sandbox run: the
// flow service and the participant are a fake network that answers with the recorded responses.
import { afterEach, beforeEach, describe, expect, spyOn, test } from "bun:test";
import { AbiCoder, keccak256 } from "ethers";

import {
  attestationOf,
  cantonNttContracts,
  cantonRedeem,
  cantonSend,
  type CantonClients,
} from "../canton-ops.js";
import { CantonLedgerClient } from "@wormhole-foundation/sdk-canton";
import {
  redeemCommandId,
  transferCommandId,
  vaaDigest,
} from "@wormhole-foundation/sdk-canton-ntt";
import { requireDeployment, validateRegistry } from "../config.js";
import {
  parseCantonPartyId,
  parseHex32,
  parseRawAmount,
  parseTransferNonceU32,
  type CantonPartyId,
} from "../types.js";
import { parseTransferVaa } from "../vaa.js";
import {
  FLOW_SERVICE_URL,
  JSON_API_URL,
  SANDBOX,
  SETUP,
  sandboxRegistryJson,
} from "./fixtures/sandbox.js";
import { TEST_NTT_PACKAGE_ID } from "./fixtures/registry.js";

type Reply = { status?: number; json: unknown } | { error: Error };

interface Recorded {
  url: URL;
  method: string;
  body: any;
}

/**
 * One fetch for the participant and the flow service. Replies are keyed by path with the query
 * (the flow routes) or by path; each key has a queue whose last reply repeats.
 */
function fakeNetwork(routes: Record<string, Reply[]>) {
  const requests: Recorded[] = [];
  const queues = new Map(
    Object.entries(routes).map(([key, replies]) => [key, [...replies]])
  );
  const fetchFunction = async (url: string, init: RequestInit) => {
    const parsed = new URL(url);
    requests.push({
      url: parsed,
      method: init.method ?? "GET",
      body: typeof init.body === "string" ? JSON.parse(init.body) : undefined,
    });
    const withQuery = `${parsed.pathname}${parsed.search}`;
    const queue =
      queues.get(withQuery) ?? queues.get(parsed.pathname) ?? undefined;
    if (!queue || queue.length === 0)
      return new Response(JSON.stringify({ error: `no route ${withQuery}` }), {
        status: 404,
      });
    const reply = queue.length > 1 ? queue.shift()! : queue[0]!;
    if ("error" in reply) throw reply.error;
    return new Response(JSON.stringify(reply.json), {
      status: reply.status ?? 200,
    });
  };
  return { fetchFunction, requests };
}

function copy<T>(value: T): T {
  return JSON.parse(JSON.stringify(value)) as T;
}

const USER = parseCantonPartyId(SETUP.user);
const OTHER = parseCantonPartyId(SETUP.other);
const ETH_RECIPIENT = parseHex32(`${"00".repeat(31)}aa`);
const SUBMIT = "/v2/commands/submit-and-wait-for-transaction";

function registryWith(edit: (json: any) => void = () => {}) {
  const json = sandboxRegistryJson();
  edit(json);
  return validateRegistry(json);
}

function setup(options: {
  deployment: "sbxburn" | "sbxlock";
  routes?: Record<string, Reply[]>;
  registry?: (json: any) => void;
  partyId?: CantonPartyId;
}) {
  const registry = registryWith(options.registry);
  const deployment = requireDeployment(registry, options.deployment);
  const recorded =
    options.deployment === "sbxburn"
      ? SANDBOX.burnTransfer
      : SANDBOX.lockTransfer;
  const network = fakeNetwork({
    "/v1/managers": [{ json: SANDBOX.managers }],
    [recorded.flowUrl]: [{ json: recorded.flow }],
    [SANDBOX.mint.flowUrl]: [{ json: SANDBOX.mint.flow }],
    [SANDBOX.release.flowUrl]: [{ json: SANDBOX.release.flow }],
    "/v2/state/ledger-end": [{ json: { offset: 100 } }],
    "/v2/state/active-contracts": [{ json: recorded.holdings }],
    [SUBMIT]: [{ json: { transaction: recorded.transaction } }],
    ...options.routes,
  });
  const clients: CantonClients = {
    ledger: new CantonLedgerClient({
      baseUrl: JSON_API_URL,
      fetch: network.fetchFunction,
    }),
    partyId: options.partyId ?? USER,
    fetch: network.fetchFunction,
    rebuildDelayMs: 0,
  };
  const submits = () =>
    network.requests.filter((request) => request.url.pathname === SUBMIT);
  const flowRequests = () =>
    network.requests.filter((request) =>
      request.url.pathname.startsWith("/v1/flows/")
    );
  return { registry, deployment, clients, network, submits, flowRequests };
}

function sendInput(
  context: ReturnType<typeof setup>,
  rawAmount = 100_000_000n
) {
  return {
    registry: context.registry,
    deployment: context.deployment,
    destinationChain: "Ethereum" as const,
    rawAmount: parseRawAmount(rawAmount),
    recipient: ETH_RECIPIENT,
    transferNonce: parseTransferNonceU32(7),
  };
}

/** The command id formula of the CLI before the SDK (ethers' ABI encoder), as a test oracle */
function oldTransferCommandId(fields: {
  user: string;
  manager: string;
  rawAmount: bigint;
  recipientChainId: number;
  recipient: string;
  transferNonce: number;
}): string {
  const encoded = AbiCoder.defaultAbiCoder().encode(
    ["string", "bytes32", "uint256", "uint16", "bytes32", "uint32"],
    [
      fields.user,
      `0x${fields.manager}`,
      fields.rawAmount,
      fields.recipientChainId,
      `0x${fields.recipient}`,
      fields.transferNonce,
    ]
  );
  return `ntt-private-send-${keccak256(encoded).slice(2)}`;
}

function exerciseOf(request: Recorded) {
  return request.body.commands.commands[0].ExerciseCommand;
}

let logs: string[];
let logSpy: ReturnType<typeof spyOn>;
beforeEach(() => {
  logs = [];
  logSpy = spyOn(console, "log").mockImplementation((...args: unknown[]) => {
    logs.push(args.join(" "));
  });
});
afterEach(() => logSpy.mockRestore());

describe("registry -> SDK config", () => {
  test("maps every Canton leg field, guardianGovernance included", () => {
    const registry = registryWith();
    const contracts = cantonNttContracts(
      registry,
      requireDeployment(registry, "sbxburn")
    );
    expect(contracts).toEqual({
      coreBridge: SETUP.guardianGovernance,
      ntt: {
        manager: SETUP.burning.managerAddress,
        token: `${SETUP.burning.instrumentAdmin}/${SETUP.burning.instrumentId}`,
        tokenDecimals: 8,
        transceiver: { wormhole: SETUP.burning.transceiverAddress },
        canton: {
          reserveCustodian: SETUP.burning.reserveCustodian,
          flowServiceUrl: `${FLOW_SERVICE_URL}/`,
          nttPackageIds: { current: TEST_NTT_PACKAGE_ID },
          corePackageIds: {
            current: registry.deployments.get("sbxburn" as never)!.chains
              .Canton!.vettedPackageIds.core,
          },
          mode: "burning",
        },
      },
    });
  });
});

describe("cantonSend", () => {
  test("body: actAs == [user], no readAs, disclosures in flow order, holding of the instrument, the CLI's command id", async () => {
    const context = setup({ deployment: "sbxburn" });
    const result = await cantonSend(context.clients, sendInput(context), {
      dryRun: false,
    });
    const submits = context.submits();
    expect(submits).toHaveLength(1);
    const commands = submits[0]!.body.commands;
    expect(commands.actAs).toEqual([USER]);
    expect("readAs" in commands).toBe(false);
    expect(
      commands.disclosedContracts.map((contract: any) => contract.contractId)
    ).toEqual(
      SANDBOX.burnTransfer.flow.disclosures.map(
        (disclosure: any) => disclosure.contractId
      )
    );
    const exercise = exerciseOf(submits[0]!);
    expect(exercise.choice).toBe("Transfer");
    expect(exercise.contractId).toBe(
      SANDBOX.burnTransfer.flow.disclosures[0]!.contractId
    );
    const argument = exercise.choiceArgument;
    expect(argument.user).toBe(USER);
    expect(argument.ledgerCid).toBeNull();
    expect(argument.nonce).toBe("7");
    expect(argument.rawAmount).toBe("100000000");
    expect(argument.recipientChain).toBe("2");
    expect(argument.recipientAddress).toEqual({
      tag: "UniversalAddress",
      value: ETH_RECIPIENT,
    });
    expect(argument.expectedManagerAddress).toEqual({
      tag: "NttManagerAddress",
      value: SETUP.burning.managerAddress,
    });
    expect(argument.feeAllocation).toBeNull();
    expect(argument.inputHoldingCids).toHaveLength(1);
    // A retry across the old CLI and the SDK path is deduplicated: the command id is the same.
    expect(commands.commandId).toBe(
      oldTransferCommandId({
        user: USER,
        manager: SETUP.burning.managerAddress,
        rawAmount: 100_000_000n,
        recipientChainId: 2,
        recipient: ETH_RECIPIENT,
        transferNonce: 7,
      })
    );
    expect(result).toEqual({
      commandId: commands.commandId,
      updateId: SANDBOX.burnTransfer.transaction.updateId,
      sequence: expect.stringMatching(/^[0-9]+$/),
    });
    // The flow is read for the registry manager from the registry's flow service URL.
    const flows = context.flowRequests();
    expect(flows.map((request) => request.url.origin)).toEqual([
      FLOW_SERVICE_URL,
    ]);
    expect(Object.fromEntries(flows[0]!.url.searchParams)).toEqual({
      manager: SETUP.burning.managerAddress,
    });
  });

  test("locking: the custodian's TransferPreapproval goes into extraArgs (the CoinFactory lock needs it)", async () => {
    const context = setup({ deployment: "sbxlock" });
    await cantonSend(context.clients, sendInput(context, 15_000_000_000n), {
      dryRun: false,
    });
    const argument = exerciseOf(context.submits()[0]!).choiceArgument;
    const preapproval = SANDBOX.lockTransfer.flow.disclosures.find(
      (disclosure: any) => disclosure.role === "custodian TransferPreapproval"
    )!;
    expect(JSON.stringify(argument.extraArgs)).toContain(
      preapproval.contractId
    );
    expect(argument.ledgerCid).toBe(
      SANDBOX.lockTransfer.flow.disclosures.find(
        (disclosure: any) => disclosure.role === "LockedLedger"
      )!.contractId
    );
  });

  test.each([["NttManager"], ["transceiver Emitter"], ["CoreState"]])(
    "role %s missing: 0 submits",
    async (absentRole) => {
      const flow: any = copy(SANDBOX.burnTransfer.flow);
      flow.disclosures = flow.disclosures.filter(
        (disclosure: any) => disclosure.role !== absentRole
      );
      const context = setup({
        deployment: "sbxburn",
        routes: { [SANDBOX.burnTransfer.flowUrl]: [{ json: flow }] },
      });
      await expect(
        cantonSend(context.clients, sendInput(context), { dryRun: false })
      ).rejects.toThrow(
        new RegExp(`${absentRole}.*nothing has been submitted`)
      );
      expect(context.submits()).toHaveLength(0);
    }
  );

  test("locking mode without LockedLedger: 0 submits", async () => {
    const flow: any = copy(SANDBOX.lockTransfer.flow);
    flow.disclosures = flow.disclosures.filter(
      (disclosure: any) => disclosure.role !== "LockedLedger"
    );
    const context = setup({
      deployment: "sbxlock",
      routes: { [SANDBOX.lockTransfer.flowUrl]: [{ json: flow }] },
    });
    await expect(
      cantonSend(context.clients, sendInput(context, 15_000_000_000n), {
        dryRun: false,
      })
    ).rejects.toThrow(/LockedLedger/);
    expect(context.submits()).toHaveLength(0);
  });

  test("missing not empty: throw naming the role, 0 submits", async () => {
    const flow: any = copy(SANDBOX.burnTransfer.flow);
    flow.missing = [
      { role: "transceiver Emitter", reason: "not visible to any reader" },
    ];
    const context = setup({
      deployment: "sbxburn",
      routes: { [SANDBOX.burnTransfer.flowUrl]: [{ json: flow }] },
    });
    await expect(
      cantonSend(context.clients, sendInput(context), { dryRun: false })
    ).rejects.toThrow(/cannot supply: transceiver Emitter/);
    expect(context.submits()).toHaveLength(0);
  });

  test("NttManager role differs from /v1/managers: 0 submits", async () => {
    const managers = copy(SANDBOX.managers);
    const entry = managers.managers.find(
      (manager: any) => manager.managerAddress === SETUP.burning.managerAddress
    )!;
    entry.contractId = `00${"77".repeat(32)}`;
    const context = setup({
      deployment: "sbxburn",
      routes: { "/v1/managers": [{ json: managers }] },
    });
    await expect(
      cantonSend(context.clients, sendInput(context), { dryRun: false })
    ).rejects.toThrow(/NttManager contract id of the flow differs/);
    expect(context.submits()).toHaveLength(0);
  });

  test.each([
    [
      "reserveCustodian",
      (json: any) => {
        json.deployments.sbxburn.chains.Canton.reserveCustodian = SETUP.other;
      },
      // The configured custodian must sign the manager (anchor layer 3).
      /NttManager .* is not signed by SdkOther/,
    ],
    [
      "mode",
      (json: any) => {
        json.deployments.sbxburn.chains.Canton.mode = "locking";
      },
      /tokenConfig BurnMint does not match the configured LockUnlock/,
    ],
    [
      "instrument",
      (json: any) => {
        json.deployments.sbxburn.chains.Canton.instrument.id = "other";
      },
      /instrument differs/,
    ],
    [
      "guardianGovernance",
      (json: any) => {
        json.deployments.sbxburn.chains.Canton.guardianGovernance = SETUP.other;
      },
      /is not signed by SdkOther/,
    ],
    [
      "vetted ntt package",
      (json: any) => {
        json.deployments.sbxburn.chains.Canton.vettedPackageIds.ntt =
          "ab".repeat(32);
      },
      /layer 1/,
    ],
    [
      "decimals",
      (json: any) => {
        json.deployments.sbxburn.chains.Canton.decimals = 10;
      },
      /the NttManager has tokenDecimals 8, the config 10/,
    ],
  ])(
    "registry %s differs from the deployment: 0 submits",
    async (_field, edit, expected) => {
      const context = setup({ deployment: "sbxburn", registry: edit });
      await expect(
        cantonSend(context.clients, sendInput(context), { dryRun: false })
      ).rejects.toThrow(expected);
      expect(context.submits()).toHaveLength(0);
    }
  );

  test("dust the peer cannot represent: 0 submits, the sendable amount named", async () => {
    const context = setup({ deployment: "sbxlock" });
    await expect(
      cantonSend(context.clients, sendInput(context, 12_345_678_901n), {
        dryRun: false,
      })
    ).rejects.toThrow(/send 12345678900 instead/);
    expect(context.submits()).toHaveLength(0);
  });

  test("dry run: built, 0 submits, the command id printed", async () => {
    const context = setup({ deployment: "sbxburn" });
    expect(
      await cantonSend(context.clients, sendInput(context), { dryRun: true })
    ).toBeUndefined();
    expect(context.submits()).toHaveLength(0);
    expect(logs).toEqual([
      `  [dry-run] would submit command ${oldTransferCommandId({
        user: USER,
        manager: SETUP.burning.managerAddress,
        rawAmount: 100_000_000n,
        recipientChainId: 2,
        recipient: ETH_RECIPIENT,
        transferNonce: 7,
      })} with ${SANDBOX.burnTransfer.flow.disclosures.length} disclosed contracts`,
    ]);
  });
});

describe("cantonSend result handling", () => {
  test("submit error: result unknown, names the command id and the nonce", async () => {
    const context = setup({
      deployment: "sbxburn",
      routes: {
        [SUBMIT]: [{ error: new Error("POST timed out after 120000 ms") }],
      },
    });
    const error = await cantonSend(context.clients, sendInput(context), {
      dryRun: false,
    }).catch((err: Error) => err);
    expect(error).toBeInstanceOf(Error);
    const { message } = error as Error;
    expect(message).toContain("result unknown");
    expect(message).toContain("--transfer-nonce 7");
    expect(message).toContain(context.submits()[0]!.body.commands.commandId);
  });

  test("ledger rejection: says rejected, names the command id and the nonce", async () => {
    const context = setup({
      deployment: "sbxburn",
      routes: {
        [SUBMIT]: [{ status: 400, json: JSON.parse(SANDBOX.mintReplayError) }],
      },
    });
    const error = await cantonSend(context.clients, sendInput(context), {
      dryRun: false,
    }).catch((err: Error) => err);
    expect((error as Error).message).toMatch(
      /^Canton submit of command ntt-private-send-[0-9a-f]{64} was rejected \(.*DAML_FAILURE.*\)\. Run the same send with --transfer-nonce 7/
    );
    expect(context.submits()).toHaveLength(1);
  });

  test("committed with an unreadable result: returns the command id without a sequence", async () => {
    const transaction = {
      ...copy(SANDBOX.burnTransfer.transaction),
      events: [],
    };
    const context = setup({
      deployment: "sbxburn",
      routes: { [SUBMIT]: [{ json: { transaction } }] },
    });
    const result = await cantonSend(context.clients, sendInput(context), {
      dryRun: false,
    });
    expect(result?.commandId).toMatch(/^ntt-private-send-/);
    expect(result?.sequence).toBeUndefined();
    expect(logs.join("\n")).toContain("the sequence is not readable");
  });
});

describe("cantonRedeem", () => {
  const mintVaa = parseTransferVaa(SANDBOX.mint.vaa);
  const releaseVaa = parseTransferVaa(SANDBOX.release.vaa);

  function redeemInput(
    context: ReturnType<typeof setup>,
    parsedVaa = mintVaa,
    recipient = USER
  ) {
    return {
      registry: context.registry,
      deployment: context.deployment,
      parsedVaa,
      recipient,
    };
  }

  test("mint: replayNodeCid from the covering ReplayNode role, digest in the request, id from the digest", async () => {
    const context = setup({
      deployment: "sbxburn",
      routes: {
        [SUBMIT]: [{ json: { transaction: SANDBOX.mint.transaction } }],
      },
    });
    const result = await cantonRedeem(context.clients, redeemInput(context), {
      dryRun: false,
    });
    // The CLI's VAA digest and the SDK's agree, so the redeem command id is the CLI's.
    expect(vaaDigest(attestationOf(mintVaa).hash)).toBe(mintVaa.digest);
    expect(
      Object.fromEntries(context.flowRequests()[0]!.url.searchParams)
    ).toEqual({
      manager: SETUP.burning.managerAddress,
      digest: mintVaa.digest,
      recipient: USER,
    });
    const submits = context.submits();
    expect(submits).toHaveLength(1);
    const commands = submits[0]!.body.commands;
    expect(commands.commandId).toBe(`ntt-private-redeem-${mintVaa.digest}`);
    expect(commands.commandId).toBe(redeemCommandId(mintVaa.digest));
    expect(commands.actAs).toEqual([USER]);
    const exercise = exerciseOf(submits[0]!);
    expect(exercise.choice).toBe("Mint");
    const argument = exercise.choiceArgument;
    expect(argument.replayNodeCid).toBe(
      SANDBOX.mint.flow.disclosures.find(
        (disclosure: any) => disclosure.role === "covering ReplayNode"
      )!.contractId
    );
    expect(argument.executor).toBe(USER);
    expect(argument.recipient).toBe(USER);
    expect(argument.vaaBytes).toEqual({
      tag: "EncodedVaa",
      value: Buffer.from(mintVaa.bytes).toString("hex"),
    });
    expect("pubKeys" in argument).toBe(false);
    expect(result).toEqual({
      commandId: commands.commandId,
      updateId: SANDBOX.mint.transaction.updateId,
      eventCount: SANDBOX.mint.transaction.events.length,
    });
  });

  test("release: the recipient's TransferPreapproval goes into extraArgs", async () => {
    const context = setup({
      deployment: "sbxlock",
      routes: {
        [SUBMIT]: [{ json: { transaction: SANDBOX.release.transaction } }],
      },
    });
    await cantonRedeem(context.clients, redeemInput(context, releaseVaa), {
      dryRun: false,
    });
    const exercise = exerciseOf(context.submits()[0]!);
    expect(exercise.choice).toBe("Release");
    const preapproval = SANDBOX.release.flow.disclosures.find(
      (disclosure: any) => disclosure.role === "recipient TransferPreapproval"
    )!;
    expect(JSON.stringify(exercise.choiceArgument.extraArgs)).toContain(
      preapproval.contractId
    );
  });

  test("another executor names the recipient; actAs is the executor", async () => {
    const context = setup({ deployment: "sbxburn", partyId: OTHER });
    await cantonRedeem(context.clients, redeemInput(context), {
      dryRun: true,
    });
    // The flow is read for the recipient, not for the executor.
    expect(context.flowRequests()[0]!.url.searchParams.get("recipient")).toBe(
      USER
    );
    await expect(
      cantonRedeem(context.clients, redeemInput(context, mintVaa, OTHER), {
        dryRun: true,
      })
    ).rejects.toThrow(/is not the hash of SdkOther/);
  });

  test("flow without the covering ReplayNode: 0 submits", async () => {
    const flow: any = copy(SANDBOX.mint.flow);
    flow.disclosures = flow.disclosures.filter(
      (disclosure: any) => disclosure.role !== "covering ReplayNode"
    );
    const context = setup({
      deployment: "sbxburn",
      routes: { [SANDBOX.mint.flowUrl]: [{ json: flow }] },
    });
    await expect(
      cantonRedeem(context.clients, redeemInput(context), { dryRun: false })
    ).rejects.toThrow(/covering ReplayNode/);
    expect(context.submits()).toHaveLength(0);
  });

  test("404 for mint: the VAA stays redeemable", async () => {
    const context = setup({
      deployment: "sbxburn",
      routes: {
        [SANDBOX.mint.flowUrl]: [{ status: 404, json: SANDBOX.mintNotFound }],
      },
    });
    await expect(
      cantonRedeem(context.clients, redeemInput(context), { dryRun: false })
    ).rejects.toThrow(/nothing has been submitted, the VAA stays redeemable/);
    expect(context.submits()).toHaveLength(0);
  });

  test("retry after contract not active stops at 3 attempts with one command id", async () => {
    const context = setup({
      deployment: "sbxburn",
      routes: {
        [SUBMIT]: [
          {
            status: SANDBOX.staleRejection.status,
            json: SANDBOX.staleRejection.body,
          },
        ],
      },
    });
    await expect(
      cantonRedeem(context.clients, redeemInput(context), { dryRun: false })
    ).rejects.toThrow(/INACTIVE_CONTRACTS|CONTRACT_NOT_FOUND/);
    const submits = context.submits();
    expect(submits).toHaveLength(3);
    expect(
      new Set(submits.map((request) => request.body.commands.commandId)).size
    ).toBe(1);
    expect(context.flowRequests()).toHaveLength(3);
    expect(logs.filter((line) => line.includes("new flow, attempt"))).toEqual([
      "  a disclosed contract changed; new flow, attempt 2",
      "  a disclosed contract changed; new flow, attempt 3",
    ]);
  });

  test("dry run: 0 submits", async () => {
    const context = setup({ deployment: "sbxburn" });
    expect(
      await cantonRedeem(context.clients, redeemInput(context), {
        dryRun: true,
      })
    ).toBeUndefined();
    expect(context.submits()).toHaveLength(0);
    expect(logs[0]).toMatch(
      /^ {2}\[dry-run\] would submit command ntt-private-redeem-[0-9a-f]{64} with \d+ disclosed contracts$/
    );
  });

  test("a VAA whose encoding is not its own bytes is refused", () => {
    // Extra bytes after the payload are dropped by the decoder; the re-encoding differs.
    const padded = parseTransferVaa(SANDBOX.mint.vaa);
    const bytes = new Uint8Array(padded.bytes.length + 1);
    bytes.set(padded.bytes);
    expect(() => attestationOf({ ...padded, bytes })).toThrow();
  });
});

describe("command ids", () => {
  test("the SDK formula is the CLI's", () => {
    const fields = {
      user: `alice::1220${"85".repeat(32)}`,
      manager:
        "d03c78c99b240126e5e18bf07bae40847bb41b345b1850c71afb75e809f24d08",
      rawAmount: 5n,
      recipientChainId: 2,
      recipient: "22".repeat(32),
    };
    for (const transferNonce of [0, 1, 4294967295]) {
      expect(transferCommandId({ ...fields, nonce: transferNonce })).toBe(
        oldTransferCommandId({ ...fields, transferNonce })
      );
    }
  });
});
