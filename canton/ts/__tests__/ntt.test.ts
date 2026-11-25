import {
  UniversalAddress,
  deserialize,
  protocolIsRegistered,
} from "@wormhole-foundation/sdk-definitions";
import { encoding } from "@wormhole-foundation/sdk-base";
import type { Ntt } from "@wormhole-foundation/sdk-definitions-ntt";
import type { CantonCommandEnvelope } from "@wormhole-foundation/sdk-canton";
import {
  CantonAddress,
  CantonLocalSigner,
  CantonLedgerClient,
} from "@wormhole-foundation/sdk-canton";
import {
  CantonNtt,
  NttAnchorError,
  StaticNttFlowSource,
  parseFlowResponse,
  register,
  transferCommandId,
} from "../src/index.js";
import { SANDBOX } from "./fixtures/sandbox.js";
import {
  MANAGERS,
  SETUP,
  collect,
  contractsOf,
  copy,
  mockFetch,
  nttOf,
  recordedFlowSource,
} from "./helpers.js";

register();

const USER = new CantonAddress(SETUP.user) as never;
const ETHEREUM_RECIPIENT = {
  chain: "Ethereum" as const,
  address: new UniversalAddress("00".repeat(31) + "aa"),
};

function holdingsLedger(holdings: unknown) {
  return mockFetch({
    "/v2/state/ledger-end": [{ json: { offset: 100 } }],
    "/v2/state/active-contracts": [{ json: holdings }],
  });
}

function attestation(vaaHex: string): Ntt.Attestation {
  return deserialize(
    "Ntt:WormholeTransfer",
    encoding.hex.decode(vaaHex)
  ) as Ntt.Attestation;
}

/** The envelope without the command id, for an envelope built with another nonce */
function withoutCommandId(
  envelope: CantonCommandEnvelope | Record<string, unknown>
) {
  const { commandId: _commandId, ...rest } = envelope as Record<
    string,
    unknown
  >;
  return rest;
}

describe("registration and construction", () => {
  it("registers the Ntt protocol on Canton", () => {
    expect(protocolIsRegistered("Canton" as never, "Ntt")).toBe(true);
  });

  it("needs the NTT contracts, the gg party and a 32-byte manager address", () => {
    const ledger = new CantonLedgerClient({
      baseUrl: "http://participant.test",
    });
    expect(
      () =>
        new CantonNtt("Devnet", "Canton", ledger, {
          coreBridge: SETUP.guardianGovernance,
        })
    ).toThrow(/NTT contracts not found/);
    const contracts = contractsOf("burning");
    expect(
      () => new CantonNtt("Devnet", "Canton", ledger, { ntt: contracts.ntt })
    ).toThrow(/contracts.coreBridge/);
    expect(
      () =>
        new CantonNtt("Devnet", "Canton", ledger, {
          ...contracts,
          ntt: { ...contracts.ntt, manager: "ab" },
        })
    ).toThrow(/contracts.ntt.manager must be 32 bytes/);
    expect(() => new CantonNtt("Mainnet", "Canton", ledger, contracts)).toThrow(
      /No ntt package id is known for Canton Mainnet/
    );
  });

  it("uses the flow service URL of the config, and refuses to run without one", async () => {
    const { fetchFunction, requests } = mockFetch({
      "/v1/managers": [{ json: SANDBOX.managers }],
    });
    const ledger = new CantonLedgerClient({
      baseUrl: "http://participant.test",
    });
    const withUrl = CantonNtt.withOptions(
      "Devnet",
      "Canton",
      ledger,
      contractsOf("burning", { flowServiceUrl: "https://flows.test/" }),
      { fetch: fetchFunction }
    );
    expect(await withUrl.getMode()).toBe("burning");
    expect(requests[0]!.path).toBe("/v1/managers");
    const without = new CantonNtt(
      "Devnet",
      "Canton",
      ledger,
      contractsOf("burning")
    );
    await expect(without.getMode()).rejects.toThrow(
      /no flow service URL; set contracts.ntt.canton.flowServiceUrl/
    );
  });
});

describe("getters", () => {
  const burning = nttOf("burning");
  const locking = nttOf("locking");

  it("reads mode, pause, custodian, owner and decimals from the manager summary", async () => {
    expect(await burning.getMode()).toBe("burning");
    expect(await locking.getMode()).toBe("locking");
    expect(await burning.isPaused()).toBe(false);
    expect(await burning.getCustodyAddress()).toBe(SETUP.burning.admin);
    expect(await locking.getCustodyAddress()).toBe(SETUP.nttCustody);
    expect((await locking.getOwner()).toString()).toBe(
      SETUP.guardianGovernance
    );
    expect(await burning.getTokenDecimals()).toBe(8);
    expect(await burning.getThreshold()).toBe(1);
    expect(await burning.getPauser()).toBeNull();
  });

  it("refuses a summary that disagrees with the config", async () => {
    await expect(
      nttOf("burning", {
        canton: { reserveCustodian: SETUP.other },
      }).getCustodyAddress()
    ).rejects.toThrow(
      /names reserveCustodian SdkBurnAdmin.*, the config SdkOther/
    );
    await expect(
      nttOf("burning", { overrides: { tokenDecimals: 6 } }).getTokenDecimals()
    ).rejects.toThrow(/tokenDecimals 8, the config 6/);
  });

  it("falls back to the config when the summary has no tokenDecimals or peers", async () => {
    const managers = copy(SANDBOX.managers) as {
      managers: Array<Record<string, unknown>>;
    };
    for (const manager of managers.managers) {
      delete manager["tokenDecimals"];
      delete manager["peers"];
    }
    const ntt = nttOf("burning", {
      flowSource: recordedFlowSource(managers),
      overrides: {
        peers: { Solana: { manager: "11".repeat(32), tokenDecimals: 9 } },
      },
    });
    expect(await ntt.getTokenDecimals()).toBe(8);
    const peer = await ntt.getPeer("Solana");
    expect(peer).toEqual({
      address: {
        chain: "Solana",
        address: new UniversalAddress("11".repeat(32)),
      },
      tokenDecimals: 9,
      inboundLimit: 0n,
    });
    expect(await ntt.getPeer("Ethereum")).toBeNull();
    const withoutDecimals = nttOf("burning", {
      flowSource: recordedFlowSource(managers),
      overrides: { tokenDecimals: undefined },
    });
    await expect(withoutDecimals.getTokenDecimals()).rejects.toThrow(
      /No token decimals/
    );
  });

  it("reads peers from the summary and checks them against the config", async () => {
    const peer = await burning.getPeer("Ethereum");
    expect(peer?.tokenDecimals).toBe(18);
    expect(peer?.inboundLimit).toBe(0n);
    expect(peer?.address.address.toString()).toBe(
      "0x000000000000000000000000b1ff5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f"
    );
    const mismatched = nttOf("burning", {
      overrides: {
        peers: { Ethereum: { manager: "22".repeat(32), tokenDecimals: 18 } },
      },
    });
    await expect(mismatched.getPeer("Ethereum")).rejects.toThrow(
      /peer of Ethereum is 0000.*, the config/
    );
  });

  it("has one Wormhole transceiver at the configured address", async () => {
    const transceiver = (await burning.getTransceiver(0))!;
    expect(await burning.getTransceiver(1)).toBeNull();
    expect(await transceiver.getTransceiverType()).toBe("wormhole");
    expect(await burning.getTransceiverType()).toBe("wormhole");
    await expect(burning.getTransceiverType(1)).rejects.toThrow(
      /one transceiver/
    );
    expect((await transceiver.getAddress()).address.toString()).toBe(
      SETUP.burning.transceiverAddress
    );
    expect((await transceiver.getPeer("Ethereum"))?.address.toString()).toBe(
      "0x000000000000000000000000c2aa5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f"
    );
    expect(await transceiver.getPauser()).toBeNull();
  });

  it("verifies the configured addresses against the anchored manager", async () => {
    expect(await burning.verifyAddresses()).toBeNull();
    expect(await locking.verifyAddresses()).toBeNull();
    const wrong = nttOf("burning", {
      overrides: {
        transceiver: { wormhole: "ab".repeat(32) },
        token: `${SETUP.other}/x`,
      },
    });
    expect(await wrong.verifyAddresses()).toEqual({
      transceiver: { wormhole: SETUP.burning.transceiverAddress },
      token: `${SETUP.burning.instrumentAdmin}/${SETUP.burning.instrumentId}`,
    });
  });

  it("quotes the core message fee, and has no rate limits or queue", async () => {
    expect(await burning.quoteDeliveryPrice("Ethereum", { queue: false })).toBe(
      0n
    );
    expect(await burning.getRateLimitDuration()).toBe(0n);
    expect(await burning.getCurrentOutboundCapacity()).toBe(0n);
    expect(await burning.getOutboundLimit()).toBe(0n);
    expect(await burning.getCurrentInboundCapacity("Ethereum")).toBe(0n);
    expect(await burning.getInboundLimit("Ethereum")).toBe(0n);
    const vaa = attestation(SANDBOX.mint.vaa);
    expect(await burning.getIsTransferInboundQueued(vaa)).toBe(false);
    expect(
      await burning.getInboundQueuedTransfer(
        "Ethereum",
        vaa.payload.nttManagerPayload
      )
    ).toBeNull();
  });

  it("throws clear errors for what Canton does not have", async () => {
    const vaa = attestation(SANDBOX.mint.vaa);
    const payer = USER;
    const cases: Array<[AsyncGenerator<unknown>, RegExp]> = [
      [burning.setPauser(payer), /unsupported: Canton NTT has no pauser role/],
      [burning.setThreshold(2), /unsupported: .*the threshold is 1/],
      [
        burning.setOutboundLimit(1n),
        /unsupported: Canton NTT has no rate limits/,
      ],
      [
        burning.setInboundLimit("Ethereum", 1n),
        /unsupported: Canton NTT has no rate limits/,
      ],
      [
        burning.completeInboundQueuedTransfer(
          "Ethereum",
          vaa.payload.nttManagerPayload
        ),
        /no inbound queue/,
      ],
    ];
    for (const [generator, pattern] of cases)
      await expect(collect(generator)).rejects.toThrow(pattern);
    const transceiver = (await burning.getTransceiver(0))!;
    await expect(collect(transceiver.setPauser(payer))).rejects.toThrow(
      /no pauser role/
    );
  });
});

describe("transfer", () => {
  it("builds the recorded locking Transfer: holdings, ledger, preapproval context, layer 4", async () => {
    const { fetchFunction, requests } = holdingsLedger(
      SANDBOX.lockTransfer.holdings
    );
    const ntt = nttOf("locking", { ledgerFetch: fetchFunction });
    const txs = await collect(
      ntt.transfer(USER, 15_000_000_000n, ETHEREUM_RECIPIENT, {
        queue: false,
        nonce: 7,
      })
    );
    expect(txs).toHaveLength(1);
    expect(txs[0]!.transaction).toEqual(SANDBOX.lockTransfer.envelope);
    expect(txs[0]!.transaction.commandId).toBe(
      transferCommandId({
        user: SETUP.user,
        manager: SETUP.locking.managerAddress,
        rawAmount: 15_000_000_000n,
        recipientChainId: 2,
        recipient: "00".repeat(31) + "aa",
        nonce: 7,
      })
    );
    // The holdings are read as the sender; nothing is submitted by building.
    expect(requests.map((request) => request.path)).toEqual([
      "/v2/state/ledger-end",
      "/v2/state/active-contracts",
    ]);
    expect(
      Object.keys(
        (requests[1]!.body as { eventFormat: { filtersByParty: object } })
          .eventFormat.filtersByParty
      )
    ).toEqual([SETUP.user]);
  });

  it("builds the recorded burning Transfer (its nonce was random)", async () => {
    const { fetchFunction } = holdingsLedger(SANDBOX.burnTransfer.holdings);
    const ntt = nttOf("burning", { ledgerFetch: fetchFunction });
    const [tx] = await collect(
      ntt.transfer(USER, 100_000_000n, ETHEREUM_RECIPIENT, { queue: false })
    );
    const recordedArgument = (
      SANDBOX.burnTransfer.envelope.commands[0] as {
        ExerciseCommand: { choiceArgument: { nonce: string } };
      }
    ).ExerciseCommand.choiceArgument;
    const built = copy(
      tx!.transaction
    ) as unknown as typeof SANDBOX.burnTransfer.envelope;
    (
      built.commands[0] as {
        ExerciseCommand: { choiceArgument: { nonce: string } };
      }
    ).ExerciseCommand.choiceArgument.nonce = recordedArgument.nonce;
    expect(withoutCommandId(built)).toEqual(
      withoutCommandId(SANDBOX.burnTransfer.envelope)
    );
    const argument = (
      tx!.transaction.commands[0] as {
        ExerciseCommand: { choiceArgument: Record<string, unknown> };
      }
    ).ExerciseCommand.choiceArgument;
    expect(argument["ledgerCid"]).toBeNull();
    expect(argument["expectedManagerAddress"]).toEqual({
      tag: "NttManagerAddress",
      value: SETUP.burning.managerAddress,
    });
  });

  it("refuses dust, a chain without a peer, automatic relaying, and a missing custodian before reading holdings", async () => {
    const { fetchFunction, requests } = holdingsLedger(
      SANDBOX.lockTransfer.holdings
    );
    const locking = nttOf("locking", { ledgerFetch: fetchFunction });
    await expect(
      collect(
        locking.transfer(USER, 12_345_678_901n, ETHEREUM_RECIPIENT, {
          queue: false,
        })
      )
    ).rejects.toThrow(
      /has dust the peer cannot represent: the step is 10\^2 units; send 12345678900 instead/
    );
    await expect(
      collect(
        locking.transfer(
          USER,
          10_000_000_000n,
          { chain: "Solana", address: new UniversalAddress("11".repeat(32)) },
          {
            queue: false,
          }
        )
      )
    ).rejects.toThrow(
      /the NttManager has no peer for Solana; nothing has been submitted/
    );
    await expect(
      collect(
        locking.transfer(USER, 10_000_000_000n, ETHEREUM_RECIPIENT, {
          queue: false,
          automatic: true,
        })
      )
    ).rejects.toThrow(/unsupported: no relayer serves Canton yet/);
    await expect(
      collect(
        nttOf("locking", { canton: { reserveCustodian: undefined } }).transfer(
          USER,
          10_000_000_000n,
          ETHEREUM_RECIPIENT,
          {
            queue: false,
          }
        )
      )
    ).rejects.toThrow(NttAnchorError);
    await expect(
      collect(
        locking.transfer(USER, 10_000_000_000n, ETHEREUM_RECIPIENT, {
          queue: false,
          nonce: -1,
        })
      )
    ).rejects.toThrow(/nonce must be a uint32/);
    expect(requests).toHaveLength(0);
  });

  it("refuses holdings that do not cover the amount", async () => {
    const { fetchFunction } = holdingsLedger([]);
    await expect(
      collect(
        nttOf("locking", { ledgerFetch: fetchFunction }).transfer(
          USER,
          10_000_000_000n,
          ETHEREUM_RECIPIENT,
          {
            queue: false,
          }
        )
      )
    ).rejects.toThrow(/unlocked holdings of sdk-lock-s\d .* cover 0 raw/);
  });

  it("refuses a configured tokenDecimals that is not the manager's, before reading holdings", async () => {
    // A caller that scaled the amount with 10 decimals would burn 100 times what it meant on a
    // manager with 8.
    const { fetchFunction, requests } = holdingsLedger(
      SANDBOX.burnTransfer.holdings
    );
    await expect(
      collect(
        nttOf("burning", {
          ledgerFetch: fetchFunction,
          overrides: { tokenDecimals: 10 },
        }).transfer(USER, 100_000_000n, ETHEREUM_RECIPIENT, { queue: false })
      )
    ).rejects.toThrow(
      /the NttManager has tokenDecimals 8, the config 10; nothing has been submitted/
    );
    expect(requests).toHaveLength(0);
  });

  it("reads the message id from the committed Transfer, as the guardians do", async () => {
    const locking = nttOf("locking");
    const transaction = SANDBOX.lockTransfer.transaction as never;
    const messageId = locking.messageIdFromTransaction(transaction);
    expect(messageId).toEqual({
      chain: "Canton",
      emitter: new UniversalAddress(SETUP.locking.transceiverAddress),
      sequence: 2n,
    });
    expect(() =>
      nttOf("burning").messageIdFromTransaction(transaction)
    ).toThrow(/has 0 messages from the transceiver/);
    const { fetchFunction, requests } = mockFetch({
      "/v2/updates/update-by-id": [
        {
          json: {
            update: {
              Transaction: { value: SANDBOX.lockTransfer.transaction },
            },
          },
        },
      ],
    });
    const fromLedger = nttOf("locking", { ledgerFetch: fetchFunction });
    expect(
      await fromLedger.getMessageId(
        SANDBOX.lockTransfer.transaction.updateId,
        SETUP.user
      )
    ).toEqual(messageId);
    expect(
      Object.keys(
        (
          requests[0]!.body as never as {
            updateFormat: {
              includeTransactions: { eventFormat: { filtersByParty: object } };
            };
          }
        ).updateFormat.includeTransactions.eventFormat.filtersByParty
      )
    ).toEqual([SETUP.user]);
  });
});

describe("redeem", () => {
  it("builds the recorded Mint and Release", async () => {
    const mint = await collect(
      nttOf("burning").redeem([attestation(SANDBOX.mint.vaa)], USER)
    );
    expect(mint[0]!.transaction).toEqual(SANDBOX.mint.envelope);
    const release = await collect(
      nttOf("locking").redeem([attestation(SANDBOX.release.vaa)], USER)
    );
    expect(release[0]!.transaction).toEqual(SANDBOX.release.envelope);
  });

  it("takes the recipient from the option when it is not the payer", async () => {
    // The recorded mint is to the user; the payer here is another party.
    const [tx] = await collect(
      nttOf("burning").redeem(
        [attestation(SANDBOX.mint.vaa)],
        new CantonAddress(SETUP.other) as never,
        {
          recipient: SETUP.user,
        }
      )
    );
    const argument = (
      tx!.transaction.commands[0] as {
        ExerciseCommand: { choiceArgument: Record<string, unknown> };
      }
    ).ExerciseCommand.choiceArgument;
    expect(argument["executor"]).toBe(SETUP.other);
    expect(argument["recipient"]).toBe(SETUP.user);
    expect(tx!.transaction.actAs).toEqual([SETUP.other]);
    expect(tx!.transaction.commandId).toBe(SANDBOX.mint.envelope.commandId);
  });

  it("refuses a VAA for another party, manager or chain, and a missing payer", async () => {
    const burning = nttOf("burning");
    const vaa = attestation(SANDBOX.mint.vaa);
    await expect(
      collect(burning.redeem([vaa], new CantonAddress(SETUP.other) as never))
    ).rejects.toThrow(
      /is not the payer SdkOther.*; pass the recipient party in options.recipient/
    );
    await expect(
      collect(burning.redeem([vaa], USER, { recipient: SETUP.other }))
    ).rejects.toThrow(/is not the hash of SdkOther/);
    await expect(collect(nttOf("locking").redeem([vaa], USER))).rejects.toThrow(
      /The VAA is for manager .*, not the configured/
    );
    await expect(collect(burning.redeem([vaa]))).rejects.toThrow(
      /needs the payer/
    );
    await expect(collect(burning.redeem([vaa, vaa], USER))).rejects.toThrow(
      /expected 1 attestation, got 2/
    );
  });

  it("keeps the VAA redeemable when the flow service has no DepositPreapproval", async () => {
    const source = new StaticNttFlowSource({ flows: [], managers: MANAGERS });
    await expect(
      collect(
        nttOf("burning", { flowSource: source }).redeem(
          [attestation(SANDBOX.mint.vaa)],
          USER
        )
      )
    ).rejects.toThrow(
      /nothing has been submitted, the VAA stays redeemable \(a burning-mode recipient needs a DepositPreapproval\)/
    );
  });
});

describe("stale contracts", () => {
  it("rebuilds from a fresh flow and resubmits under the same command id", async () => {
    // Recorded: the participant's answer to a disclosed contract that another transaction archived.
    const { fetchFunction, requests } = mockFetch({
      "/v2/commands/submit-and-wait-for-transaction": [
        {
          status: SANDBOX.staleRejection.status,
          json: SANDBOX.staleRejection.body,
        },
        { json: { transaction: SANDBOX.mint.transaction } },
      ],
    });
    const flows: string[] = [];
    const recorded = recordedFlowSource();
    const counting = {
      flow: (
        flow: "transfer" | "release" | "mint",
        parameters: Readonly<Record<string, string>>
      ) => {
        flows.push(flow);
        return recorded.flow(flow, parameters);
      },
      managers: () => recorded.managers(),
    };
    const ntt = nttOf("burning", {
      flowSource: counting,
      ledgerFetch: fetchFunction,
    });
    const txs = await collect(
      ntt.redeem([attestation(SANDBOX.mint.vaa)], USER)
    );
    const signer = new CantonLocalSigner("Canton", ntt.connection, SETUP.user, {
      userId: "test",
    });
    const [updateId] = await signer.signAndSend(txs);
    expect(updateId).toBe(SANDBOX.mint.transaction.updateId);
    const submits = requests.filter((request) =>
      request.path.startsWith("/v2/commands/")
    );
    expect(submits).toHaveLength(2);
    const commandIds = submits.map(
      (request) =>
        (request.body as { commands: { commandId: string } }).commands.commandId
    );
    expect(commandIds).toEqual([
      SANDBOX.mint.envelope.commandId,
      SANDBOX.mint.envelope.commandId,
    ]);
    expect(flows).toEqual(["mint", "mint"]);
  });

  it("does not rebuild after any other rejection", async () => {
    const { fetchFunction, requests } = mockFetch({
      "/v2/commands/submit-and-wait-for-transaction": [
        { status: 400, json: JSON.parse(SANDBOX.mintReplayError) },
      ],
    });
    const ntt = nttOf("burning", { ledgerFetch: fetchFunction });
    const txs = await collect(
      ntt.redeem([attestation(SANDBOX.mint.vaa)], USER)
    );
    const signer = new CantonLocalSigner("Canton", ntt.connection, SETUP.user, {
      userId: "test",
    });
    await expect(signer.signAndSend(txs)).rejects.toThrow(
      /HTTP 400: {"code":"DAML_FAILURE"/
    );
    expect(requests).toHaveLength(1);
  });
});

describe("the recorded flows", () => {
  it("are the flows the anchor checks pass", () => {
    expect(parseFlowResponse(SANDBOX.mint.flow, "mint").missingRoles).toEqual(
      []
    );
  });
});

describe("release coverage", () => {
  // The flow service can serve the LockedLedger from before a lock for a few seconds. Stand-in:
  // the recorded release flow with the ledger of the first lock's transfer flow (balance 0, no pot).
  function staleReleaseFlow() {
    const flow = parseFlowResponse(SANDBOX.release.flow, "release");
    const staleLedger = parseFlowResponse(
      SANDBOX.lockTransfer.flow,
      "transfer"
    ).disclosures.find((entry) => entry.role === "LockedLedger")!;
    flow.disclosures = flow.disclosures
      .filter((entry) => entry.role !== "custody pot Holding")
      .map((entry) => (entry.role === "LockedLedger" ? staleLedger : entry));
    return flow;
  }

  function sourceServing(releaseFlows: ReturnType<typeof staleReleaseFlow>[]) {
    const recorded = recordedFlowSource();
    let served = 0;
    return {
      source: {
        flow: async (
          flow: "transfer" | "release" | "mint",
          parameters: Readonly<Record<string, string>>
        ) => {
          if (flow !== "release") return recorded.flow(flow, parameters);
          return releaseFlows[Math.min(served++, releaseFlows.length - 1)]!;
        },
        managers: () => recorded.managers(),
      },
      served: () => served,
    };
  }

  it("waits out a stale LockedLedger once and builds from the fresh flow", async () => {
    const fresh = parseFlowResponse(SANDBOX.release.flow, "release");
    const { source, served } = sourceServing([staleReleaseFlow(), fresh]);
    const [tx] = await collect(
      nttOf("locking", { flowSource: source }).redeem(
        [attestation(SANDBOX.release.vaa)],
        USER
      )
    );
    expect(tx!.transaction).toEqual(SANDBOX.release.envelope);
    expect(served()).toBe(2);
  });

  it("refuses a release that the ledger cannot cover, before anything is submitted", async () => {
    const { source, served } = sourceServing([staleReleaseFlow()]);
    await expect(
      collect(
        nttOf("locking", { flowSource: source }).redeem(
          [attestation(SANDBOX.release.vaa)],
          USER
        )
      )
    ).rejects.toThrow(
      /the LockedLedger names no custody pot; the VAA stays redeemable; nothing has been submitted/
    );
    expect(served()).toBe(2);
  });

  it("refuses a zero recipient address", async () => {
    await expect(
      collect(
        nttOf("locking").transfer(
          USER,
          10_000_000_000n,
          { chain: "Ethereum", address: new UniversalAddress("00".repeat(32)) },
          {
            queue: false,
          }
        )
      )
    ).rejects.toThrow(/recipient address is zero/);
  });
});

describe("fromRpc", () => {
  it("builds the client from the one configured Canton chain", async () => {
    const ledger = new CantonLedgerClient({
      baseUrl: "http://participant.test",
    });
    const chainConfig = {
      key: "Canton",
      platform: "Canton",
      network: "Devnet",
      chainId: 75,
      finalityThreshold: 0,
      blockTime: 1000,
      nativeTokenDecimals: 10,
      rpc: "http://participant.test",
      contracts: contractsOf("locking"),
    };
    const ntt = await CantonNtt.fromRpc(ledger, {
      Canton: chainConfig,
    } as never);
    expect(ntt.managerAddress).toBe(SETUP.locking.managerAddress);
    expect(ntt.network).toBe("Devnet");
    await expect(CantonNtt.fromRpc(ledger, {} as never)).rejects.toThrow(
      /exactly one configured Canton chain/
    );
  });
});
