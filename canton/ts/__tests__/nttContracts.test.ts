import {
  createArgumentToJson,
  readCreatedEventBlob,
} from "@wormhole-foundation/sdk-canton";
import {
  ADMIN_TRANSFER_PROPOSAL_LAYOUT,
  LOCKED_LEDGER_LAYOUT,
  NTT_MANAGER_LAYOUT,
  adminTransferProposalArgument,
  decodeAdminTransferProposal,
  decodeReplayNode,
  replayNodeRecordsDigest,
  setPausedArgument,
  setPeerArgument,
  decodeLockedLedger,
  decodeNttManager,
  decodeTransferResult,
  extraArgs,
  mintArgument,
  releaseArgument,
  transferArgument,
} from "../src/index.js";
import { SANDBOX } from "./fixtures/sandbox.js";
import { SETUP, copy } from "./helpers.js";

const managerEntries = [
  SANDBOX.burnTransfer.flow,
  SANDBOX.lockTransfer.flow,
].map((flow) => flow.disclosures.find((entry) => entry.role === "NttManager")!);

describe("NttManager layout (ntt 0.7.0)", () => {
  it.each(
    managerEntries.map(
      (entry) => [entry.contractId.slice(0, 12), entry] as const
    )
  )("reads blob %s as the JSON the participant served", (_id, entry) => {
    const fromBlob = createArgumentToJson(
      readCreatedEventBlob(entry.createdEventBlob).createArgument,
      NTT_MANAGER_LAYOUT
    );
    expect(fromBlob).toEqual(entry.createArgument);
  });

  it("decodes the fields, with peers as a Daml Map of [key, value] pairs", () => {
    const manager = decodeNttManager(managerEntries[1]!.createArgument);
    expect(Array.isArray(managerEntries[1]!.createArgument.peers)).toBe(true);
    expect(manager).toMatchObject({
      managerAddress: SETUP.locking.managerAddress,
      tokenConfig: "LockUnlock",
      tokenDecimals: 10,
      chainId: 75,
      managerId: 1n,
      factory: { tag: "TransferFactoryCid" },
      paused: false,
    });
    expect(manager.peers.get(2)?.decimals).toBe(18);
  });

  it("accepts peers as an object keyed by chain id, and refuses malformed fields", () => {
    const payload = copy(managerEntries[0]!.createArgument) as Record<
      string,
      unknown
    >;
    const pairs = payload["peers"] as Array<[string, unknown]>;
    expect(
      decodeNttManager({
        ...payload,
        peers: Object.fromEntries(pairs),
      }).peers.get(2)
    ).toBeDefined();
    expect(() =>
      decodeNttManager({ ...payload, tokenConfig: "Other" })
    ).toThrow(/unknown value "Other"/);
    expect(() => decodeNttManager({ ...payload, tokenDecimals: "11" })).toThrow(
      /expected 0 to 10, got 11/
    );
    expect(() =>
      decodeNttManager({ ...payload, peers: [pairs[0], pairs[0]] })
    ).toThrow(/appears twice/);
    expect(() =>
      decodeNttManager({
        ...payload,
        managerAddress: { tag: "NttManagerAddress", value: "ab" },
      })
    ).toThrow(/expected 32 bytes/);
    expect(() =>
      decodeNttManager({ ...payload, factory: { tag: "Other", value: "00" } })
    ).toThrow(/unknown constructor/);
  });
});

describe("LockedLedger layout (ntt 0.7.0)", () => {
  const ledgerOf = (flow: { disclosures: readonly { role: string }[] }) =>
    flow.disclosures.find(
      (entry) => entry.role === "LockedLedger"
    ) as unknown as {
      createdEventBlob: string;
      createArgument: Record<string, unknown>;
    };

  it("reads the balance and the custody pot from the blob", () => {
    const withPot = ledgerOf(SANDBOX.release.flow);
    const json = createArgumentToJson(
      readCreatedEventBlob(withPot.createdEventBlob).createArgument,
      LOCKED_LEDGER_LAYOUT
    );
    expect(json).toEqual(withPot.createArgument);
    const ledger = decodeLockedLedger(json);
    expect(ledger.managerAddress).toBe(SETUP.locking.managerAddress);
    expect(ledger.balance).toBe(15_000_000_000n);
    expect(ledger.custodyHoldingCid).toMatch(/^00[0-9a-f]+$/);
  });

  it("reads a ledger with no pot: a trailing None the served payload omits", () => {
    const first = ledgerOf(SANDBOX.lockTransfer.flow);
    expect("custodyHoldingCid" in first.createArgument).toBe(false);
    const json = createArgumentToJson(
      readCreatedEventBlob(first.createdEventBlob).createArgument,
      LOCKED_LEDGER_LAYOUT
    );
    expect(json).toEqual({ ...first.createArgument, custodyHoldingCid: null });
    expect(decodeLockedLedger(first.createArgument)).toEqual(
      decodeLockedLedger(json)
    );
    expect(decodeLockedLedger(json).balance).toBe(0n);
  });
});

describe("choice arguments", () => {
  const recorded = (envelope: { commands: readonly unknown[] }) =>
    (
      envelope.commands[0] as {
        ExerciseCommand: { choiceArgument: Record<string, unknown> };
      }
    ).ExerciseCommand.choiceArgument;

  it("builds Transfer with the CLI's fields and expectedManagerAddress", () => {
    const argument = recorded(SANDBOX.lockTransfer.envelope);
    expect(Object.keys(argument).sort()).toEqual(
      [
        "user",
        "transceiverEmitterCid",
        "coreStateCid",
        "ledgerCid",
        "recipientChain",
        "recipientAddress",
        "rawAmount",
        "nonce",
        "consistencyLevel",
        "inputHoldingCids",
        "feeAllocation",
        "extraArgs",
        "expectedManagerAddress",
      ].sort()
    );
    const rebuilt = transferArgument({
      user: argument["user"] as string,
      transceiverEmitterCid: argument["transceiverEmitterCid"] as string,
      coreStateCid: argument["coreStateCid"] as string,
      ledgerCid: argument["ledgerCid"] as string,
      recipientChain: 2,
      recipientAddress: "00".repeat(31) + "aa",
      rawAmount: 15_000_000_000n,
      nonce: 7,
      consistencyLevel: 0,
      inputHoldingCids: argument["inputHoldingCids"] as string[],
      transferPreapprovalCid: SANDBOX.lockTransfer.flow.disclosures.find(
        (entry) => entry.role === "custodian TransferPreapproval"
      )!.contractId,
      expectedManagerAddress: SETUP.locking.managerAddress,
    });
    expect(rebuilt).toEqual(argument);
    expect(() =>
      transferArgument({
        ...(rebuilt as Record<string, unknown>),
        nonce: 2 ** 32,
        rawAmount: 1n,
      } as never)
    ).toThrow(/uint32/);
  });

  it("puts a receiver TransferPreapproval in the token-cip0056 choice context", () => {
    expect(extraArgs()).toEqual({
      context: { values: {} },
      meta: { values: {} },
    });
    const cid = "00" + "ab".repeat(32);
    expect(extraArgs(cid)).toEqual({
      context: {
        values: {
          "token-cip0056.transferPreapprovalCid": {
            tag: "AV_ContractId",
            value: cid,
          },
        },
      },
      meta: { values: {} },
    });
  });

  it("builds Release and Mint as the ledger accepted them", () => {
    const release = recorded(SANDBOX.release.envelope);
    expect(
      releaseArgument({
        executor: release["executor"] as string,
        coreStateCid: release["coreStateCid"] as string,
        replayNodeCid: release["replayNodeCid"] as string,
        vaaBytes: Uint8Array.from(Buffer.from(SANDBOX.release.vaa, "hex")),
        recipient: release["recipient"] as string,
        ledgerCid: release["ledgerCid"] as string,
        transferPreapprovalCid: SANDBOX.release.flow.disclosures.find(
          (entry) => entry.role === "recipient TransferPreapproval"
        )!.contractId,
      })
    ).toEqual(release);
    const mint = recorded(SANDBOX.mint.envelope);
    expect(
      mintArgument({
        executor: mint["executor"] as string,
        coreStateCid: mint["coreStateCid"] as string,
        replayNodeCid: mint["replayNodeCid"] as string,
        vaaBytes: Uint8Array.from(Buffer.from(SANDBOX.mint.vaa, "hex")),
        recipient: mint["recipient"] as string,
        depositPreapprovalCid: mint["depositPreapprovalCid"] as string,
      })
    ).toEqual(mint);
  });

  it("decodes the Transfer result: the WormholeMessage of its nested publish", () => {
    const exercised = SANDBOX.lockTransfer.transaction.events
      .map(
        (event) =>
          (
            event as {
              ExercisedEvent?: { choice: string; exerciseResult: unknown };
            }
          ).ExercisedEvent
      )
      .find((event) => event?.choice === "Transfer")!;
    const message = decodeTransferResult(exercised.exerciseResult);
    expect(message.registrar).toBe(SETUP.guardianGovernance);
    expect(message.nonce).toBe(7);
    expect(message.payload.startsWith("9945ff10")).toBe(true);
  });
});

// ---------------------------------------------------------------------------------------------
// The core's ReplayNode (getIsExecuted) and the admin layouts
// ---------------------------------------------------------------------------------------------

/** Encoders of a serialized Daml-LF Value (lfValue.ts), to build blobs the reader must refuse */
const lf = (() => {
  const varint = (value: number): number[] => {
    const out: number[] = [];
    let rest = value;
    while (rest >= 0x80) {
      out.push((rest & 0x7f) | 0x80);
      rest = Math.floor(rest / 128);
    }
    out.push(rest);
    return out;
  };
  const delimited = (field: number, bytes: number[]): number[] => [
    (field << 3) | 2,
    ...varint(bytes.length),
    ...bytes,
  ];
  const utf8 = (text: string) => [...new TextEncoder().encode(text)];
  const value = {
    unit: () => delimited(1, []),
    int64: (n: number) => [3 << 3, ...varint(n * 2)],
    party: (text: string) => delimited(7, utf8(text)),
    text: (text: string) => delimited(8, utf8(text)),
    variant: (constructor: string, inner: number[]) =>
      delimited(14, [
        ...delimited(1, utf8(constructor)),
        ...delimited(2, inner),
      ]),
    record: (fields: number[][]) =>
      delimited(
        13,
        fields.flatMap((field) => delimited(1, delimited(1, field)))
      ),
    map: (entries: Array<[number[], number[]]>) =>
      delimited(
        12,
        entries.flatMap(([key, entry]) =>
          delimited(1, [...delimited(1, key), ...delimited(2, entry)])
        )
      ),
  };
  return value;
})();

interface ReplayNodeInput {
  prefix?: string;
  suffixes?: string[];
  setValue?: number[];
  extraField?: boolean;
  suffixConstructor?: string;
  setFields?: number;
}

function replayNodeBytes(input: ReplayNodeInput = {}): Uint8Array {
  const prefix = input.prefix ?? "";
  const set = lf.record([
    lf.map(
      (input.suffixes ?? []).map((suffix) => [
        lf.variant(
          input.suffixConstructor ?? "DigestNibbleSuffix",
          lf.text(suffix)
        ),
        input.setValue ?? lf.unit(),
      ])
    ),
    ...(input.setFields === 2 ? [lf.unit()] : []),
  ]);
  return new Uint8Array(
    lf.record([
      lf.party("gg::1220aa"),
      lf.party("gg::1220aa"),
      lf.text("ntt:namespace"),
      lf.variant("DigestNibblePrefix", lf.text(prefix)),
      lf.int64(128),
      set,
      ...(input.extraField ? [lf.unit()] : []),
    ])
  );
}

describe("ReplayNode layout (wormhole-core 0.12.3)", () => {
  const digest = "7e".repeat(32);

  it("reads the recorded covering node: the redeemed digest is in its consumed set", () => {
    const entry = SANDBOX.executed.after.flow.disclosures.find(
      (disclosure) => disclosure.role === "covering ReplayNode"
    )!;
    const node = decodeReplayNode(
      readCreatedEventBlob(entry.createdEventBlob).createArgument
    );
    // The served payload, as the JSON Ledger API writes a Set: {"map": [[key, {}], ...]}.
    const served = entry.createArgument as unknown as {
      consumer: string;
      namespace: string;
      prefix: { value: string };
      consumed: { map: Array<[{ tag: string; value: string }, unknown]> };
    };
    expect(node.consumer).toBe(served.consumer);
    expect(node.guardianGovernance).toBe(SETUP.guardianGovernance);
    expect(node.namespace).toBe(served.namespace);
    expect(node.prefix).toBe(served.prefix.value);
    expect(node.splitThreshold).toBe(128n);
    expect([...node.consumed].sort()).toEqual(
      served.consumed.map.map(([key]) => key.value).sort()
    );
    const executedDigest = new URL(
      SANDBOX.executed.after.flowUrl,
      "http://x"
    ).searchParams.get("digest")!;
    expect(replayNodeRecordsDigest(node, executedDigest)).toBe(true);
    const before = SANDBOX.executed.before.flow.disclosures.find(
      (disclosure) => disclosure.role === "covering ReplayNode"
    )!;
    expect(
      replayNodeRecordsDigest(
        decodeReplayNode(
          readCreatedEventBlob(before.createdEventBlob).createArgument
        ),
        executedDigest
      )
    ).toBe(false);
  });

  it("stores a digest below the node's prefix: the suffix is the rest of the nibbles", () => {
    const node = decodeReplayNode(
      replayNodeBytes({ prefix: "7e", suffixes: ["7e".repeat(31)] })
    );
    expect(replayNodeRecordsDigest(node, digest)).toBe(true);
    expect(replayNodeRecordsDigest(node, `7e${"00".repeat(31)}`)).toBe(false);
    expect(() => replayNodeRecordsDigest(node, "ab".repeat(32))).toThrow(
      /prefix "7e" does not cover digest/
    );
    expect(() => replayNodeRecordsDigest(node, "AB".repeat(32))).toThrow(
      /64 lowercase hex/
    );
  });

  it.each<[string, ReplayNodeInput, RegExp]>([
    ["an extra field", { extraField: true }, /7 fields, expected 6/],
    ["a Set with two fields", { setFields: 2 }, /Set has 2 fields/],
    [
      "a set value that is not unit",
      { suffixes: ["7e".repeat(32)], setValue: lf.int64(1) },
      /consumed.map\[0\].value is int64, expected unit/,
    ],
    [
      "a suffix of another width",
      { prefix: "7", suffixes: ["7e".repeat(32)] },
      /64 nibbles under a 1-nibble prefix/,
    ],
    [
      "a suffix under another constructor",
      { suffixes: ["7e".repeat(32)], suffixConstructor: "VaaDigest" },
      /constructor VaaDigest, expected DigestNibbleSuffix/,
    ],
    [
      "a repeated suffix",
      { suffixes: ["7e".repeat(32), "7e".repeat(32)] },
      /repeats a suffix/,
    ],
    [
      "uppercase nibbles",
      { suffixes: ["7E".repeat(32)] },
      /not lowercase hex nibbles/,
    ],
    ["a 64-nibble prefix", { prefix: "7e".repeat(32) }, /at most 63/],
  ])("refuses %s", (_name, input, pattern) => {
    expect(() => decodeReplayNode(replayNodeBytes(input))).toThrow(pattern);
  });

  it("refuses a value that is not a record, and bytes that are not a Value", () => {
    expect(() => decodeReplayNode(new Uint8Array(lf.text("x")))).toThrow(
      /ReplayNode layout: ReplayNode is text, expected a record/
    );
    expect(() => decodeReplayNode(new Uint8Array([0xff]))).toThrow(
      /ReplayNode layout/
    );
  });
});

describe("AdminTransferProposal layout and the admin choice arguments", () => {
  it("reads the recorded proposal blob as the JSON the participant served", () => {
    const active = (
      SANDBOX.accept.acs as Array<{
        contractEntry: {
          JsActiveContract: {
            createdEvent: { createdEventBlob: string; createArgument: unknown };
          };
        };
      }>
    )[0]!.contractEntry.JsActiveContract.createdEvent;
    const fromBlob = createArgumentToJson(
      readCreatedEventBlob(active.createdEventBlob).createArgument,
      ADMIN_TRANSFER_PROPOSAL_LAYOUT
    );
    expect(fromBlob).toEqual(active.createArgument);
    expect(decodeAdminTransferProposal(fromBlob)).toEqual({
      admin: SETUP.owned.admin,
      newAdmin: SETUP.other,
      managerAddress: SETUP.owned.managerAddress,
    });
  });

  it("builds SetPaused, SetPeer and the proposal as the ledger accepted them", () => {
    const exercised = (envelope: { commands: readonly unknown[] }) =>
      (
        envelope.commands[0] as {
          ExerciseCommand: { choiceArgument: Record<string, unknown> };
        }
      ).ExerciseCommand.choiceArgument;
    expect(setPausedArgument(true)).toEqual(exercised(SANDBOX.pause.envelope));
    const setPeer = exercised(SANDBOX.setPeerFull.envelope) as {
      transceiverEmitterCid: string;
      coreStateCid: string;
      payer: string;
    };
    expect(
      setPeerArgument({
        chainId: 21,
        peer: {
          managerAddress: "5a".repeat(32),
          transceiverAddress: "5b".repeat(32),
          decimals: 9,
        },
        transceiverEmitterCid: setPeer.transceiverEmitterCid,
        coreStateCid: setPeer.coreStateCid,
        consistencyLevel: 0,
        payer: setPeer.payer,
      })
    ).toEqual(setPeer);
    expect(() =>
      setPeerArgument({
        chainId: 0,
        peer: {
          managerAddress: "5a".repeat(32),
          transceiverAddress: "5b".repeat(32),
          decimals: 9,
        },
        transceiverEmitterCid: setPeer.transceiverEmitterCid,
        coreStateCid: setPeer.coreStateCid,
        consistencyLevel: 0,
        payer: setPeer.payer,
      })
    ).toThrow(/positive uint16/);
    expect(
      adminTransferProposalArgument({
        admin: SETUP.owned.admin,
        newAdmin: SETUP.other,
        managerAddress: SETUP.owned.managerAddress,
      })
    ).toEqual(
      (
        SANDBOX.setOwner.envelope.commands[0] as {
          CreateCommand: { createArguments: unknown };
        }
      ).CreateCommand.createArguments
    );
  });
});
