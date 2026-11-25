// Milestone N4 against the recorded sandbox run: getIsExecuted and getIsApproved, and the admin
// operations (pause, unpause, the peer setters, setOwner, acceptOwnership), with their failure
// paths. The admin's own active contract set is served by a mocked participant.
import { Buffer } from "node:buffer";
import { encoding } from "@wormhole-foundation/sdk-base";
import {
  UniversalAddress,
  deserialize,
} from "@wormhole-foundation/sdk-definitions";
import type { Ntt } from "@wormhole-foundation/sdk-definitions-ntt";
import type { LfType } from "@wormhole-foundation/sdk-canton";
import {
  CantonAddress,
  readCreatedEventBlob,
} from "@wormhole-foundation/sdk-canton";
import { CORE_STATE_LAYOUT } from "@wormhole-foundation/sdk-canton-core";
import type { NttFlowSource } from "../src/index.js";
import {
  NttAnchorError,
  StaticNttFlowSource,
  parseManagersResponse,
  proposeAdminCommandId,
  register,
} from "../src/index.js";
import { SANDBOX } from "./fixtures/sandbox.js";
import {
  SETUP,
  acsLedger,
  activeContractsOf,
  collect,
  copy,
  flowSourceOf,
  nttOf,
} from "./helpers.js";

register();

const party = (id: string) => new CantonAddress(id) as never;
const BURN_ADMIN = SETUP.burning.admin;

function attestation(vaaHex: string): Ntt.Attestation {
  return deserialize(
    "Ntt:WormholeTransfer",
    encoding.hex.decode(vaaHex)
  ) as Ntt.Attestation;
}

type FlowBody = { disclosures: Array<Record<string, unknown>> };

/** The served entry of a role in a recorded flow body */
function entryOf(flow: unknown, role: string) {
  const entry = (flow as FlowBody).disclosures.find(
    (disclosure) => disclosure["role"] === role
  );
  if (!entry) throw new Error(`no ${role} in the recorded flow`);
  return entry as {
    contractId: string;
    templateId: string;
    createdEventBlob: string;
    synchronizerId: string;
    createArgument?: unknown;
  };
}

function choiceArgumentOf(envelope: {
  commands: readonly unknown[];
}): Record<string, unknown> {
  return (
    envelope.commands[0] as {
      ExerciseCommand: { choiceArgument: Record<string, unknown> };
    }
  ).ExerciseCommand.choiceArgument;
}

function readVarint(bytes: Uint8Array, at: number): [number, number] {
  let value = 0;
  let shift = 0;
  for (let position = at; ; position++) {
    const byte = bytes[position]!;
    value += (byte & 0x7f) * 2 ** shift;
    shift += 7;
    if ((byte & 0x80) === 0) return [value, position + 1];
  }
}

/**
 * A flow body whose CoreState blob carries another message fee. The fee is an Int field of the
 * create argument's record; a small fee keeps every length, so only that varint changes.
 */
function withMessageFee(flow: unknown, fee: number): unknown {
  const changed = copy(flow) as FlowBody;
  const entry = changed.disclosures.find(
    (disclosure) => disclosure["role"] === "CoreState"
  )!;
  const blob = Buffer.from(entry["createdEventBlob"] as string, "base64");
  const argument = readCreatedEventBlob(blob).createArgument;
  let position = blob.indexOf(Buffer.from(argument));
  expect(blob[position]).toBe(0x6a); // Value.record
  [, position] = readVarint(blob, position + 1);
  const index = (
    CORE_STATE_LAYOUT as Extract<LfType, { kind: "record" }>
  ).fields.findIndex(([name]) => name === "messageFee");
  for (let field = 0; ; field++) {
    expect(blob[position]).toBe(0x0a); // Record.fields
    const [length, start] = readVarint(blob, position + 1);
    if (field === index) {
      // Field { value: Value { int64: 0 } }
      expect([...blob.subarray(start, start + 4)]).toEqual([
        0x0a, 0x02, 0x18, 0x00,
      ]);
      blob[start + 3] = fee * 2; // zigzag
      break;
    }
    position = start + length;
  }
  entry["createdEventBlob"] = blob.toString("base64");
  return changed;
}

describe("getIsExecuted and getIsApproved", () => {
  const vaa = attestation(SANDBOX.executed.vaa);
  const sourceOf = (recorded: { flowUrl: string; flow: unknown }) =>
    flowSourceOf([recorded], SANDBOX.managers);

  it("reads the consumed set of the covering ReplayNode: false before the redeem, true after", async () => {
    const before = nttOf("burning", {
      flowSource: sourceOf(SANDBOX.executed.before),
    });
    expect(await before.getIsExecuted(vaa)).toBe(false);
    expect(await before.getIsApproved(vaa)).toBe(false);
    const after = nttOf("burning", {
      flowSource: sourceOf(SANDBOX.executed.after),
    });
    expect(await after.getIsExecuted(vaa)).toBe(true);
    expect(await after.getIsApproved(vaa)).toBe(true);
  });

  it("asks the set-peer flow for the VAA digest; no recipient is needed", async () => {
    const asked: Array<[string, Readonly<Record<string, string>>]> = [];
    const recorded = sourceOf(SANDBOX.executed.after);
    const counting: NttFlowSource = {
      flow: (flow, parameters) => {
        asked.push([flow, parameters]);
        return recorded.flow(flow, parameters);
      },
      managers: () => recorded.managers(),
    };
    await nttOf("burning", { flowSource: counting }).getIsExecuted(vaa);
    const query = new URL(SANDBOX.executed.after.flowUrl, "http://x")
      .searchParams;
    expect(asked).toEqual([
      [
        "set-peer",
        { manager: SETUP.burning.managerAddress, digest: query.get("digest") },
      ],
    ]);
  });

  it("refuses a VAA for another manager", async () => {
    await expect(
      nttOf("locking", {
        flowSource: sourceOf(SANDBOX.executed.after),
      }).getIsExecuted(vaa)
    ).rejects.toThrow(/The VAA is for manager .*, not the configured/);
  });

  it("refuses a ReplayNode of another trie, and one that does not cover the digest", async () => {
    // The release flow's node is in the lock/unlock deployment's namespace.
    const otherTrie = copy(SANDBOX.executed.after.flow) as FlowBody;
    otherTrie.disclosures = otherTrie.disclosures.map((entry) =>
      entry["role"] === "covering ReplayNode"
        ? entryOf(SANDBOX.release.flow, "covering ReplayNode")
        : entry
    );
    await expect(
      nttOf("burning", {
        flowSource: sourceOf({
          flowUrl: SANDBOX.executed.after.flowUrl,
          flow: otherTrie,
        }),
      }).getIsExecuted(vaa)
    ).rejects.toThrow(/has namespace .*, not the NttManager's/);
  });

  it("refuses another contract's blob in the ReplayNode's place", async () => {
    // A CoreState blob under the ReplayNode's role and template id: same package and signer.
    // (Layout mismatches inside a ReplayNode blob: nttContracts.test.ts, decodeReplayNode.)
    const wrongLayout = copy(SANDBOX.executed.after.flow) as FlowBody;
    const coreState = entryOf(wrongLayout, "CoreState");
    wrongLayout.disclosures = wrongLayout.disclosures.map((entry) =>
      entry["role"] === "covering ReplayNode"
        ? {
            ...entry,
            contractId: coreState.contractId,
            createdEventBlob: coreState.createdEventBlob,
          }
        : entry
    );
    // The blob's envelope names the CoreState template: the anchor refuses before decoding.
    await expect(
      nttOf("burning", {
        flowSource: sourceOf({
          flowUrl: SANDBOX.executed.after.flowUrl,
          flow: wrongLayout,
        }),
      }).getIsExecuted(vaa)
    ).rejects.toThrow(NttAnchorError);
  });
});

describe("pause and unpause", () => {
  const managers = parseManagersResponse(SANDBOX.managers);
  const source = () => new StaticNttFlowSource({ flows: [], managers });

  it("builds the recorded SetPaused commands from the admin's own view", async () => {
    const { fetchFunction, requests } = acsLedger(SANDBOX.pause.acs);
    const asAdmin = nttOf("burning", {
      ledgerFetch: fetchFunction,
      flowSource: source(),
      nttOptions: { adminParty: BURN_ADMIN },
    });
    const [pause] = await collect(asAdmin.pause());
    expect(pause!.transaction).toEqual(SANDBOX.pause.envelope);
    // No flow is read: the manager is the admin's own, and nothing is disclosed.
    expect(requests.map((request) => request.path)).toEqual([
      "/v2/state/ledger-end",
      "/v2/state/active-contracts",
    ]);
    expect(requests[1]!.body).toMatchObject({
      eventFormat: {
        filtersByParty: {
          [BURN_ADMIN]: {
            cumulative: [
              {
                identifierFilter: {
                  TemplateFilter: {
                    value: {
                      templateId: "#ntt:Wormhole.Ntt.Manager:NttManager",
                      includeCreatedEventBlob: true,
                    },
                  },
                },
              },
            ],
          },
        },
      },
    });
    // The payer is the admin when given.
    const [byPayer] = await collect(
      nttOf("burning", {
        ledgerFetch: acsLedger(SANDBOX.pause.acs).fetchFunction,
        flowSource: source(),
      }).pause(party(BURN_ADMIN))
    );
    expect(byPayer!.transaction).toEqual(SANDBOX.pause.envelope);
    const [unpause] = await collect(
      nttOf("burning", {
        ledgerFetch: acsLedger(SANDBOX.unpause.acs).fetchFunction,
        flowSource: source(),
      }).unpause(party(BURN_ADMIN))
    );
    expect(unpause!.transaction).toEqual(SANDBOX.unpause.envelope);
  });

  it("refuses without an admin party, for a party that does not see the manager, and twice", async () => {
    await expect(
      collect(nttOf("burning", { flowSource: source() }).pause())
    ).rejects.toThrow(
      /needs the admin party: pass the payer, or set options.adminParty/
    );
    await expect(
      collect(
        nttOf("burning", {
          ledgerFetch: acsLedger([]).fetchFunction,
          flowSource: source(),
        }).pause(party(SETUP.user))
      )
    ).rejects.toThrow(/SdkUser.* sees no active NttManager with address/);
    await expect(
      collect(
        nttOf("burning", {
          ledgerFetch: acsLedger(SANDBOX.unpause.acs).fetchFunction,
          flowSource: source(),
        }).pause(party(BURN_ADMIN))
      )
    ).rejects.toThrow(/already paused; nothing has been submitted/);
    await expect(
      collect(
        nttOf("burning", {
          ledgerFetch: acsLedger(SANDBOX.pause.acs).fetchFunction,
          flowSource: source(),
        }).unpause(party(BURN_ADMIN))
      )
    ).rejects.toThrow(/already unpaused/);
  });

  it("refuses a signatory that is not the admin, and a gg-administered deployment", async () => {
    // nttCustody signs every manager, so it sees this one, but it is not its admin.
    await expect(
      collect(
        nttOf("burning", {
          ledgerFetch: acsLedger(SANDBOX.pause.acs).fetchFunction,
          flowSource: source(),
        }).pause(party(SETUP.nttCustody))
      )
    ).rejects.toThrow(
      /SdkNttCustody.* is not the admin of NttManager .*; its admin is SdkBurnAdmin/
    );
    const lockingManager = activeContractsOf([
      entryOf(SANDBOX.lockTransfer.flow, "NttManager"),
    ]);
    await expect(
      collect(
        nttOf("locking", {
          ledgerFetch: acsLedger(lockingManager).fetchFunction,
          flowSource: source(),
        }).pause(party(SETUP.guardianGovernance))
      )
    ).rejects.toThrow(/is guardianGovernance: to pause is a governance action/);
  });

  it("ignores a manager of an unpinned ntt package", async () => {
    const entries = copy(SANDBOX.pause.acs) as Array<{
      contractEntry: {
        JsActiveContract: { createdEvent: { templateId: string } };
      };
    }>;
    for (const entry of entries)
      entry.contractEntry.JsActiveContract.createdEvent.templateId = `${"ab".repeat(32)}:Wormhole.Ntt.Manager:NttManager`;
    await expect(
      collect(
        nttOf("burning", {
          ledgerFetch: acsLedger(entries).fetchFunction,
          flowSource: source(),
        }).pause(party(BURN_ADMIN))
      )
    ).rejects.toThrow(/sees no active NttManager/);
  });
});

describe("getters from the admin's own view", () => {
  const ownPaused = (managersResponse: unknown) =>
    nttOf("burning", {
      ledgerFetch: acsLedger(SANDBOX.unpause.acs).fetchFunction,
      flowSource: new StaticNttFlowSource({
        flows: [],
        managers: parseManagersResponse(managersResponse),
      }),
      nttOptions: { adminParty: BURN_ADMIN },
    });

  /** The paused manager's contract id, from the admin's own view */
  const pausedContractId = (
    SANDBOX.unpause.acs as Array<{
      contractEntry: {
        JsActiveContract: { createdEvent: { contractId: string } };
      };
    }>
  )[0]!.contractEntry.JsActiveContract.createdEvent.contractId;

  it("prefers the own view over a /v1/managers entry of an older contract", async () => {
    // Right after the pause, /v1/managers still served the unpaused contract.
    const served = parseManagersResponse(SANDBOX.unpause.managers).find(
      (entry) => entry.managerAddress === SETUP.burning.managerAddress
    )!;
    expect(served.contractId).not.toBe(pausedContractId);
    expect(served.paused).toBe(false);
    const ntt = ownPaused(SANDBOX.unpause.managers);
    expect(await ntt.isPaused()).toBe(true);
    expect((await ntt.getOwner()).toString()).toBe(BURN_ADMIN);
    expect(await ntt.getMode()).toBe("burning");
    expect(await ntt.getCustodyAddress()).toBe(BURN_ADMIN);
    expect(await ntt.getTokenDecimals()).toBe(8);
    expect((await ntt.getPeer("Ethereum"))?.tokenDecimals).toBe(18);
  });

  it("refuses a /v1/managers entry of the same contract that disagrees", async () => {
    const managers = copy(SANDBOX.unpause.managers) as {
      managers: Array<Record<string, unknown>>;
    };
    const entry = managers.managers.find(
      (manager) => manager["managerAddress"] === SETUP.burning.managerAddress
    )!;
    entry["contractId"] = pausedContractId;
    entry["paused"] = false;
    await expect(ownPaused(managers).isPaused()).rejects.toThrow(
      /\/v1\/managers entry of NttManager .* differs from the contract in the view of SdkBurnAdmin.* on paused/
    );
    entry["paused"] = true;
    expect(await ownPaused(managers).isPaused()).toBe(true);
  });

  it("uses the own view when /v1/managers cannot be read, and /v1/managers when the party sees nothing", async () => {
    const failing: NttFlowSource = {
      flow: () => Promise.reject(new Error("unreachable")),
      managers: () => Promise.reject(new Error("service down")),
    };
    const own = nttOf("burning", {
      ledgerFetch: acsLedger(SANDBOX.unpause.acs).fetchFunction,
      flowSource: failing,
      nttOptions: { adminParty: BURN_ADMIN },
    });
    expect(await own.isPaused()).toBe(true);
    const formerAdmin = nttOf("burning", {
      ledgerFetch: acsLedger([]).fetchFunction,
      nttOptions: { adminParty: SETUP.other },
    });
    expect(await formerAdmin.isPaused()).toBe(false);
  });
});

describe("the peer setters", () => {
  const admin = (options: { flow?: unknown } = {}) =>
    nttOf("burning", {
      ledgerFetch: acsLedger(SANDBOX.setPeerFull.acs).fetchFunction,
      flowSource: flowSourceOf(
        [
          {
            flowUrl: SANDBOX.setPeerFull.flowUrl,
            flow: options.flow ?? SANDBOX.setPeerFull.flow,
          },
        ],
        SANDBOX.setPeerFull.managers
      ),
      nttOptions: { adminParty: BURN_ADMIN },
    });
  const manager = new UniversalAddress("5a".repeat(32));
  const transceiver = new UniversalAddress("5b".repeat(32));
  const ethereumPeer = {
    managerAddress:
      "000000000000000000000000b1ff5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f",
    transceiverAddress:
      "000000000000000000000000c2aa5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f",
  };

  it("builds the recorded SetPeer: the admin's manager, CoreState and Emitter from the transfer flow", async () => {
    const [tx] = await collect(
      admin().setPeerFull("Sui", manager, transceiver, 9)
    );
    expect(tx!.transaction).toEqual(SANDBOX.setPeerFull.envelope);
    expect(
      tx!.transaction.disclosedContracts.map((entry) => entry.contractId)
    ).toEqual([
      entryOf(SANDBOX.setPeerFull.flow, "transceiver Emitter").contractId,
      entryOf(SANDBOX.setPeerFull.flow, "CoreState").contractId,
    ]);
    // Strings work too: 64 hex characters, or a native address of the chain.
    const [fromHex] = await collect(
      admin().setPeerFull("Sui", "5a".repeat(32), `0x${"5b".repeat(32)}`, 9)
    );
    expect(choiceArgumentOf(fromHex!.transaction)).toEqual(
      choiceArgumentOf(SANDBOX.setPeerFull.envelope)
    );
  });

  it("updates one field of an existing peer and keeps the others", async () => {
    const [managerUpdate] = await collect(
      admin().setPeer({ chain: "Ethereum", address: manager }, 6, 0n)
    );
    expect(choiceArgumentOf(managerUpdate!.transaction)["peer"]).toEqual({
      managerAddress: { tag: "NttManagerAddress", value: "5a".repeat(32) },
      transceiverAddress: {
        tag: "EmitterAddress",
        value: ethereumPeer.transceiverAddress,
      },
      decimals: "6",
    });
    const ntt = admin();
    const [transceiverUpdate] = await collect(
      (await ntt.getTransceiver(0))!.setPeer({
        chain: "Ethereum",
        address: transceiver,
      })
    );
    expect(choiceArgumentOf(transceiverUpdate!.transaction)["peer"]).toEqual({
      managerAddress: {
        tag: "NttManagerAddress",
        value: ethereumPeer.managerAddress,
      },
      transceiverAddress: { tag: "EmitterAddress", value: "5b".repeat(32) },
      decimals: "18",
    });
    expect(choiceArgumentOf(transceiverUpdate!.transaction)["chainId"]).toBe(
      "2"
    );
  });

  it("refuses an update without an existing peer, and arguments Canton cannot take", async () => {
    await expect(
      collect(admin().setPeer({ chain: "Sui", address: manager }, 9, 0n))
    ).rejects.toThrow(
      /no peer for Sui to update; setPeer changes one field .* use setPeerFull|Add the peer with setPeerFull/
    );
    await expect(
      collect(
        admin().setTransceiverPeer(0, { chain: "Sui", address: transceiver })
      )
    ).rejects.toThrow(/no peer for Sui to update; setTransceiverPeer/);
    await expect(
      collect(
        admin().setTransceiverPeer(1, { chain: "Sui", address: transceiver })
      )
    ).rejects.toThrow(/Transceiver index 1 is not supported/);
    await expect(
      collect(admin().setPeer({ chain: "Ethereum", address: manager }, 18, 5n))
    ).rejects.toThrow(/no rate limits; the inbound limit must be 0/);
    await expect(
      collect(admin().setPeerFull("Sui", manager, transceiver, 0))
    ).rejects.toThrow(/decimals must be an integer from 1 to 255/);
    await expect(
      collect(admin().setPeerFull("Sui", "00".repeat(32), transceiver, 9))
    ).rejects.toThrow(/The peer manager address is the zero address/);
    await expect(
      collect(admin().setPeerFull("Canton", manager, transceiver, 9))
    ).rejects.toThrow(/cannot be on this deployment's own chain Canton/);
  });

  it("refuses a core message fee above zero, as transfer does", async () => {
    await expect(
      collect(
        admin({
          flow: withMessageFee(SANDBOX.setPeerFull.flow, 1),
        }).setPeerFull("Sui", manager, transceiver, 9)
      )
    ).rejects.toThrow(
      /the core message fee is 1 .*only a zero fee is supported until milestone N5; nothing has been submitted/
    );
  });

  it("refuses for a party that is not the admin", async () => {
    await expect(
      collect(
        nttOf("burning", {
          ledgerFetch: acsLedger([]).fetchFunction,
        }).setPeerFull("Sui", manager, transceiver, 9, party(SETUP.user))
      )
    ).rejects.toThrow(/sees no active NttManager/);
  });
});

describe("setOwner and acceptOwnership", () => {
  const OWNED_ADMIN = SETUP.owned.admin;
  const factory = (
    choiceArgumentOf(SANDBOX.accept.envelope)["vettedFactory"] as {
      value: string;
    }
  ).value;
  const acceptSource = () =>
    flowSourceOf(
      [{ flowUrl: SANDBOX.accept.flowUrl, flow: SANDBOX.accept.flow }],
      SANDBOX.accept.managers
    );
  const acceptor = (activeContracts: unknown = SANDBOX.accept.acs) =>
    nttOf("owned", {
      ledgerFetch: acsLedger(activeContracts).fetchFunction,
      flowSource: acceptSource(),
      canton: { reserveCustodian: undefined },
    });

  it("proposes: an AdminTransferProposal create signed by the admin", async () => {
    const { fetchFunction } = acsLedger(SANDBOX.setOwner.acs);
    const [tx] = await collect(
      nttOf("owned", { ledgerFetch: fetchFunction }).setOwner(
        party(SETUP.other),
        party(OWNED_ADMIN)
      )
    );
    expect(tx!.transaction).toEqual(SANDBOX.setOwner.envelope);
    const managerContractId = (
      SANDBOX.setOwner.acs as Array<{
        contractEntry: {
          JsActiveContract: { createdEvent: { contractId: string } };
        };
      }>
    )[0]!.contractEntry.JsActiveContract.createdEvent.contractId;
    expect(tx!.transaction.commandId).toBe(
      proposeAdminCommandId({
        admin: OWNED_ADMIN,
        newAdmin: SETUP.other,
        manager: SETUP.owned.managerAddress,
        managerContractId,
      })
    );
  });

  it("refuses a proposal to the admin itself or to the custody party, and by a non-admin", async () => {
    const owned = () =>
      nttOf("owned", {
        ledgerFetch: acsLedger(SANDBOX.setOwner.acs).fetchFunction,
      });
    await expect(
      collect(owned().setOwner(party(OWNED_ADMIN), party(OWNED_ADMIN)))
    ).rejects.toThrow(/is already the admin/);
    await expect(
      collect(owned().setOwner(party(SETUP.nttCustody), party(OWNED_ADMIN)))
    ).rejects.toThrow(/custody party .* cannot hold the admin role/);
    await expect(
      collect(owned().setOwner(party(SETUP.other), party(SETUP.nttCustody)))
    ).rejects.toThrow(/is not the admin of NttManager/);
  });

  it("accepts: AcceptAdminTransfer on the proposal, with the anchored manager and ledger disclosed", async () => {
    const [tx] = await collect(
      acceptor().acceptOwnership(party(SETUP.other), factory)
    );
    expect(tx!.transaction).toEqual(SANDBOX.accept.envelope);
    expect(
      tx!.transaction.disclosedContracts.map((entry) => entry.contractId)
    ).toEqual([
      entryOf(SANDBOX.accept.flow, "NttManager").contractId,
      entryOf(SANDBOX.accept.flow, "LockedLedger").contractId,
    ]);
    expect(tx!.transaction.actAs).toEqual([SETUP.other]);
  });

  it("refuses a missing proposal, a proposal to another party, and an unpinned one", async () => {
    await expect(
      collect(acceptor([]).acceptOwnership(party(SETUP.other), factory))
    ).rejects.toThrow(
      /SdkOther.* sees no AdminTransferProposal for NttManager .*; its admin SdkOwnedAdmin.* creates one with setOwner/
    );
    // The proposal names SdkOther; SdkUser cannot accept it.
    await expect(
      collect(acceptor().acceptOwnership(party(SETUP.user), factory))
    ).rejects.toThrow(/SdkUser.* sees no AdminTransferProposal/);
    const unpinned = copy(SANDBOX.accept.acs) as Array<{
      contractEntry: {
        JsActiveContract: { createdEvent: { templateId: string } };
      };
    }>;
    for (const entry of unpinned)
      entry.contractEntry.JsActiveContract.createdEvent.templateId = `${"cd".repeat(32)}:Wormhole.Ntt.Manager:AdminTransferProposal`;
    await expect(
      collect(acceptor(unpinned).acceptOwnership(party(SETUP.other), factory))
    ).rejects.toThrow(/sees no AdminTransferProposal/);
  });

  it("refuses a factory other than the committed one, and an acceptance by gg", async () => {
    await expect(
      collect(
        acceptor().acceptOwnership(party(SETUP.other), `00${"ab".repeat(32)}`)
      )
    ).rejects.toThrow(
      /commits the factory .* \(TransferFactoryCid\), not the vetted 00abab.*; nothing has been submitted/
    );
    await expect(
      collect(
        acceptor().acceptOwnership(party(SETUP.guardianGovernance), factory)
      )
    ).rejects.toThrow(/completes with AcceptAdminTransferByVaa/);
  });

  it("refuses a stale proposal: the manager's admin is not the proposer any more", async () => {
    // The transfer flow read when the role went back: by then SdkOther was the admin, and the
    // recorded proposal (from SdkOwnedAdmin) no longer matches the manager.
    const stale = nttOf("owned", {
      ledgerFetch: acsLedger(SANDBOX.accept.acs).fetchFunction,
      flowSource: flowSourceOf(
        [
          {
            flowUrl: SANDBOX.acceptBack.flowUrl,
            flow: SANDBOX.acceptBack.flow,
          },
        ],
        SANDBOX.acceptBack.managers
      ),
      canton: { reserveCustodian: undefined },
    });
    await expect(
      collect(stale.acceptOwnership(party(SETUP.other), factory))
    ).rejects.toThrow(
      /are from SdkOwnedAdmin.*, not from its admin SdkOther.*; they are stale/
    );
  });
});
