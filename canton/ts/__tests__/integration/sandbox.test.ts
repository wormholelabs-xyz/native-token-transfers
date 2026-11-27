// Opt-in integration test of the Canton NTT SDK against a ledger and the NTT flow service.
//
//   1. A ledger with the JSON Ledger API, for example a local sandbox (from an empty directory):
//        DPM_SDK_VERSION=3.7.3 dpm sandbox --json-api-port 7575
//   2. The deployments, under fresh parties (the DAR uploads ntt, core and token-cip0056):
//        cd canton/ts/__tests__/integration/daml && dpm build
//        dpm script --dar .daml/dist/ntt-sdk-integration-setup-0.1.0.dar \
//          --script-name SdkSetup:sdkSetup --ledger-host localhost --ledger-port 6865 \
//          --upload-dar yes --input-file setup-in.json --output-file setup-out.json
//      with setup-in.json: {"runLabel": "r1", "peerChainId": 2, "peerManager": "<64 hex>",
//        "peerTransceiver": "<64 hex>", "peerDecimals": 18, "userCoins": "100.0"}
//   3. The flow service (canton/disclosure/service) against that ledger, as the setup's
//      guardianGovernance and nttCustody:
//        ./disclosure-service --json-api http://127.0.0.1:7575 --listen 127.0.0.1:7599 \
//          --guardian-governance-party <gg> --ntt-custody-party <nttCustody>
//   4. CANTON_JSON_API_URL=http://localhost:7575 CANTON_NTT_FLOW_SERVICE_URL=http://127.0.0.1:7599 \
//      CANTON_NTT_SETUP=<setup-out.json> bun run test
//
// Skipped unless CANTON_JSON_API_URL is set. Optional: CANTON_TOKEN (bearer token), CANTON_USER_ID
// (default ntt-sdk-test; a ledger without auth needs one on each submission),
// CANTON_NTT_FIXTURE_OUT (a file to write the recorded service and ledger responses to).
// It can run more than once against one setup: VAA sequences are fresh and balances are compared
// before and after, the admin tests unpause and hand the admin role back, and the peer they set
// is replaced on a rerun.
import { jest } from "@jest/globals";
import fs from "node:fs";
import type { Chain, Network } from "@wormhole-foundation/sdk-base";
import { encoding } from "@wormhole-foundation/sdk-base";
import type { VAA } from "@wormhole-foundation/sdk-definitions";
import {
  UniversalAddress,
  createVAA,
  deserialize,
  serialize,
} from "@wormhole-foundation/sdk-definitions";
import { mocks } from "@wormhole-foundation/sdk-definitions/testing";
import type { Ntt } from "@wormhole-foundation/sdk-definitions-ntt";
import type {
  FetchFunction,
  JsTransaction,
} from "@wormhole-foundation/sdk-canton";
import {
  CantonAddress,
  CantonLedgerClient,
  CantonLedgerError,
  CantonLocalSigner,
  partyAddress,
  readHoldings,
  staticTokenProvider,
  unlockedBalance,
} from "@wormhole-foundation/sdk-canton";
import type { CantonNttContracts, NttFlowSource } from "../../src/index.js";
import {
  CantonNtt,
  HttpNttFlowSource,
  NttAnchorError,
  register,
} from "../../src/index.js";

jest.setTimeout(300_000);

const LEDGER_URL = process.env["CANTON_JSON_API_URL"];
const FLOW_URL =
  process.env["CANTON_NTT_FLOW_SERVICE_URL"] ?? "http://127.0.0.1:7599";
const SETUP_FILE = process.env["CANTON_NTT_SETUP"];
const TOKEN = process.env["CANTON_TOKEN"];
const USER_ID = process.env["CANTON_USER_ID"] ?? "ntt-sdk-test";
const FIXTURE_OUT = process.env["CANTON_NTT_FIXTURE_OUT"];

// The devnet guardian key the setup's guardian set 0 holds; never a real guardian.
const GUARDIAN_PRIVATE_KEY =
  "cfb12303a19cde580bb4dd771639b0d26bc68353645571a8cff516ab2ee113a0";
const EVM_RECIPIENT = new UniversalAddress("00".repeat(31) + "aa");
/** Past the flow service's default 5-second cache */
const FLOW_CACHE_MS = 5_500;

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

interface DeploymentSetup {
  managerAddress: string;
  transceiverAddress: string;
  admin: string;
  reserveCustodian: string;
  instrumentAdmin: string;
  instrumentId: string;
  tokenDecimals: number;
}

interface Setup {
  guardianGovernance: string;
  nttCustody: string;
  user: string;
  other: string;
  burning: DeploymentSetup;
  locking: DeploymentSetup;
  /** Lock/unlock under a third-party admin, with nothing locked (the admin transfer test) */
  owned: DeploymentSetup;
}

interface Recorded {
  url: string;
  method: string;
  request?: unknown;
  status: number;
  body: unknown;
}

const recorded: Recorded[] = [];
const named: Record<string, unknown> = {};

/** fetch that records each request and its response */
function recordingFetch(): FetchFunction {
  return async (url, init) => {
    const response = await fetch(url, init);
    const text = await response.text();
    let body: unknown = text;
    try {
      body = JSON.parse(text);
    } catch {
      // not JSON
    }
    recorded.push({
      url: new URL(url).pathname + new URL(url).search,
      method: init.method ?? "GET",
      ...(typeof init.body === "string"
        ? { request: JSON.parse(init.body) }
        : {}),
      status: response.status,
      body,
    });
    return new Response(text, {
      status: response.status,
      headers: response.headers,
    });
  };
}

async function collect<T>(generator: AsyncGenerator<T>): Promise<T[]> {
  const items: T[] = [];
  for await (const item of generator) items.push(item);
  return items;
}

if (!LEDGER_URL)
  describe.skip("Canton NTT on a ledger", () => {
    it("needs CANTON_JSON_API_URL, CANTON_NTT_FLOW_SERVICE_URL and CANTON_NTT_SETUP", () => {});
  });
else
  describe("Canton NTT on a ledger", () => {
    register();
    const setup = JSON.parse(
      fs.readFileSync(SETUP_FILE ?? "", "utf8")
    ) as Setup;
    const fetchFunction = recordingFetch();
    const ledger = new CantonLedgerClient({
      baseUrl: LEDGER_URL ?? "http://localhost:1",
      fetch: fetchFunction,
      ...(TOKEN ? { tokenProvider: staticTokenProvider(TOKEN) } : {}),
    });
    const flowSource = new HttpNttFlowSource({
      baseUrl: FLOW_URL,
      fetch: fetchFunction,
    });
    const peerChain: Chain = "Ethereum";

    function contractsOf(
      deployment: DeploymentSetup,
      canton: CantonNttContracts["canton"] = {}
    ) {
      return {
        coreBridge: setup.guardianGovernance,
        ntt: {
          manager: deployment.managerAddress,
          token: `${deployment.instrumentAdmin}/${deployment.instrumentId}`,
          tokenDecimals: deployment.tokenDecimals,
          transceiver: { wormhole: deployment.transceiverAddress },
          canton: { reserveCustodian: deployment.reserveCustodian, ...canton },
        },
      };
    }

    function nttOf(
      deployment: DeploymentSetup,
      options: {
        flowSource?: NttFlowSource;
        canton?: CantonNttContracts["canton"];
        adminParty?: string;
      } = {}
    ) {
      return CantonNtt.withOptions<Network, "Canton">(
        "Devnet",
        "Canton",
        ledger,
        contractsOf(deployment, options.canton),
        {
          flowSource: options.flowSource ?? flowSource,
          readAs: [setup.user],
          ...(options.adminParty ? { adminParty: options.adminParty } : {}),
        }
      );
    }

    function signerOf(party: string) {
      return new CantonLocalSigner("Canton", ledger, party, {
        userId: USER_ID,
        onTransaction: (transaction) => committed.push(transaction),
      });
    }

    /** The last request to a path (and the query prefix, for the flow routes) */
    function lastRecorded(prefix: string) {
      return recorded.filter((entry) => entry.url.startsWith(prefix)).at(-1);
    }

    const burning = nttOf(setup.burning);
    const locking = nttOf(setup.locking);
    const committed: JsTransaction[] = [];
    let redeemedRelease: Ntt.Attestation | undefined;
    const signer = new CantonLocalSigner("Canton", ledger, setup.user, {
      userId: USER_ID,
      onTransaction: (transaction) => committed.push(transaction),
    });

    async function balance(deployment: DeploymentSetup): Promise<bigint> {
      return unlockedBalance(await readHoldings(ledger, setup.user), {
        admin: deployment.instrumentAdmin,
        id: deployment.instrumentId,
      });
    }

    function submitCount(): number {
      return recorded.filter((entry) =>
        entry.url.startsWith("/v2/commands/submit-and-wait")
      ).length;
    }

    /** An Ethereum -> Canton transfer VAA signed by the devnet guardian */
    function transferVaa(
      deployment: DeploymentSetup,
      recipient: string,
      amount: bigint
    ): Ntt.Attestation {
      const peer = {
        manager: "00".repeat(12) + "b1ff5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f",
      };
      const transceiver = new UniversalAddress(
        "00".repeat(12) + "c2aa5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f"
      );
      const sequence =
        BigInt(Date.now()) * 1000n + BigInt(Math.floor(Math.random() * 1000));
      const vaa = createVAA("Ntt:WormholeTransfer", {
        guardianSet: 0,
        timestamp: Math.floor(Date.now() / 1000),
        nonce: 0,
        emitterChain: peerChain,
        emitterAddress: transceiver,
        sequence,
        consistencyLevel: 200,
        signatures: [],
        payload: {
          sourceNttManager: new UniversalAddress(peer.manager),
          recipientNttManager: new UniversalAddress(deployment.managerAddress),
          nttManagerPayload: {
            id: encoding.bignum.toBytes(sequence, 32),
            sender: new UniversalAddress("00".repeat(31) + "ee"),
            payload: {
              trimmedAmount: { amount, decimals: 8 },
              sourceToken: new UniversalAddress("00".repeat(31) + "dd"),
              recipientAddress: new UniversalAddress(partyAddress(recipient)),
              recipientChain: "Canton" as Chain,
              additionalPayload: new Uint8Array(),
            },
          },
          transceiverPayload: new Uint8Array(),
        },
      } as never) as VAA<"Ntt:WormholeTransfer">;
      const signed = new mocks.MockGuardians(0, [
        GUARDIAN_PRIVATE_KEY,
      ]).addSignatures(vaa, [0]);
      return deserialize(
        "Ntt:WormholeTransfer",
        serialize(signed)
      ) as Ntt.Attestation;
    }

    afterAll(() => {
      if (FIXTURE_OUT)
        fs.writeFileSync(
          FIXTURE_OUT,
          JSON.stringify({ setup, named, recorded }, null, 2)
        );
    });

    it("reads the getters of both deployments", async () => {
      expect(await burning.getMode()).toBe("burning");
      expect(await locking.getMode()).toBe("locking");
      for (const [ntt, deployment] of [
        [burning, setup.burning],
        [locking, setup.locking],
      ] as const) {
        expect(await ntt.isPaused()).toBe(false);
        expect(await ntt.getCustodyAddress()).toBe(deployment.reserveCustodian);
        expect(await ntt.getTokenDecimals()).toBe(deployment.tokenDecimals);
        expect(await ntt.getThreshold()).toBe(1);
        expect((await ntt.getOwner()).toString()).toBe(deployment.admin);
        const peer = await ntt.getPeer(peerChain);
        expect(peer?.tokenDecimals).toBe(18);
        expect(peer?.inboundLimit).toBe(0n);
        expect(peer?.address.address.toString()).toBe(
          "0x000000000000000000000000b1ff5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f"
        );
        expect(await ntt.getPeer("Solana")).toBeNull();
        const transceiver = (await ntt.getTransceiver(0))!;
        expect((await transceiver.getAddress()).address.toString()).toBe(
          deployment.transceiverAddress
        );
        expect((await transceiver.getPeer(peerChain))?.address.toString()).toBe(
          "0x000000000000000000000000c2aa5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f"
        );
        expect(await ntt.verifyAddresses()).toBeNull();
        expect(await ntt.quoteDeliveryPrice(peerChain, { queue: false })).toBe(
          0n
        );
      }
      named["managers"] = recorded.find(
        (entry) => entry.url === "/v1/managers"
      )?.body;
    });

    it("redeems into the burning deployment with Mint, then refuses the replay", async () => {
      const before = await balance(setup.burning);
      const vaa = transferVaa(setup.burning, setup.user, 250_000_000n); // 2.5 at 8 decimals
      const txs = await collect(
        burning.redeem([vaa], new CantonAddress(setup.user) as never)
      );
      expect(txs).toHaveLength(1);
      named["mintEnvelope"] = txs[0]!.transaction;
      named["mintVaa"] = encoding.hex.encode(serialize(vaa));
      const [updateId] = await signer.signAndSend(txs);
      expect(updateId).toMatch(/^1220[0-9a-f]{64}$/);
      // The holding is at the 10 decimals of a Daml Decimal.
      expect((await balance(setup.burning)) - before).toBe(25_000_000_000n);

      // The same redeem again: the command id derives from the digest, so the participant
      // deduplicates it.
      const again = await collect(
        burning.redeem([vaa], new CantonAddress(setup.user) as never)
      );
      const duplicate = await signer
        .signAndSend(again)
        .catch((err: Error) => err);
      expect(duplicate).toBeInstanceOf(CantonLedgerError);
      expect((duplicate as CantonLedgerError).code).toBe("DUPLICATE_COMMAND");
      named["mintDuplicateError"] = (duplicate as CantonLedgerError).body.slice(
        0,
        2000
      );

      // Another executor gets a new command: the mint flow still serves a covering node for the
      // consumed digest, and the ledger refuses the replay.
      await sleep(FLOW_CACHE_MS);
      const otherSigner = new CantonLocalSigner("Canton", ledger, setup.other, {
        userId: USER_ID,
      });
      const byOther = await collect(
        burning.redeem([vaa], new CantonAddress(setup.other) as never, {
          recipient: setup.user,
        })
      );
      named["mintReplayFlow"] = recorded
        .filter((entry) => entry.url.startsWith("/v1/flows/mint"))
        .at(-1);
      const replay = await otherSigner
        .signAndSend(byOther)
        .catch((err: Error) => err);
      expect(replay).toBeInstanceOf(CantonLedgerError);
      named["mintReplayError"] = (replay as CantonLedgerError).body.slice(
        0,
        2000
      );
      expect((await balance(setup.burning)) - before).toBe(25_000_000_000n);
    });

    it("transfers out of the burning deployment and reads the message id", async () => {
      const before = await balance(setup.burning);
      const txs = await collect(
        burning.transfer(
          new CantonAddress(setup.user) as never,
          100_000_000n, // 1.0 at 8 decimals
          { chain: peerChain, address: EVM_RECIPIENT },
          { queue: false }
        )
      );
      named["burnTransferEnvelope"] = txs[0]!.transaction;
      const [updateId] = await signer.signAndSend(txs);
      const transaction = committed.at(-1)!;
      expect(transaction.updateId).toBe(updateId);
      named["burnTransferTransaction"] = transaction;
      const fromResult = burning.messageIdFromTransaction(transaction);
      expect(fromResult.emitter.toString()).toBe(
        `0x${setup.burning.transceiverAddress}`
      );
      const fromLedger = await burning.getMessageId(updateId!, setup.user);
      expect(fromLedger.sequence).toBe(fromResult.sequence);
      expect(before - (await balance(setup.burning))).toBe(10_000_000_000n);
    });

    it("transfers out of the locking deployment, refusing dust first", async () => {
      const submitsBefore = submitCount();
      await expect(
        collect(
          locking.transfer(
            new CantonAddress(setup.user) as never,
            12_345_678_901n, // 1.2345678901 at 10 decimals; the wire carries 8
            { chain: peerChain, address: EVM_RECIPIENT },
            { queue: false }
          )
        )
      ).rejects.toThrow(
        /dust the peer cannot represent.*send 12345678900 instead/
      );
      expect(submitCount()).toBe(submitsBefore);

      const before = await balance(setup.locking);
      const txs = await collect(
        locking.transfer(
          new CantonAddress(setup.user) as never,
          15_000_000_000n, // 1.5 at 10 decimals
          { chain: peerChain, address: EVM_RECIPIENT },
          { queue: false, nonce: 7 }
        )
      );
      named["lockTransferEnvelope"] = txs[0]!.transaction;
      const [updateId] = await signer.signAndSend(txs);
      const transaction = committed.at(-1)!;
      named["lockTransferTransaction"] = transaction;
      const messageId = locking.messageIdFromTransaction(transaction);
      expect(messageId.emitter.toString()).toBe(
        `0x${setup.locking.transceiverAddress}`
      );
      expect((await locking.getMessageId(updateId!, setup.user)).sequence).toBe(
        messageId.sequence
      );
      expect(before - (await balance(setup.locking))).toBe(15_000_000_000n);
    });

    it("redeems into the locking deployment with Release, waiting out a stale LockedLedger", async () => {
      // Right after the lock, the flow service's cache can still hold the LockedLedger from before
      // it. The SDK waits the cache out and reads the flow again, rather than submit a Release that
      // the ledger would refuse on that balance (an assertion, not a stale-contract rejection).
      const before = await balance(setup.locking);
      const releaseReads = () =>
        recorded.filter((entry) => entry.url.startsWith("/v1/flows/release"))
          .length;
      const readsBefore = releaseReads();
      const vaa = transferVaa(setup.locking, setup.user, 50_000_000n); // 0.5 at 8 decimals
      const txs = await collect(
        locking.redeem([vaa], new CantonAddress(setup.user) as never)
      );
      named["releaseEnvelope"] = txs[0]!.transaction;
      named["releaseVaa"] = encoding.hex.encode(serialize(vaa));
      named["releaseFlowReads"] = releaseReads() - readsBefore;
      await signer.signAndSend(txs);
      expect((await balance(setup.locking)) - before).toBe(5_000_000_000n);
      redeemedRelease = vaa;
    });

    it("refuses a redeem to a recipient without a preapproval, before anything is submitted", async () => {
      const submitsBefore = submitCount();
      const toOther = (deployment: DeploymentSetup) =>
        transferVaa(deployment, setup.other, 1_000n);
      // Mint: the flow service answers 404 (no DepositPreapproval).
      const mint = await collect(
        burning.redeem(
          [toOther(setup.burning)],
          new CantonAddress(setup.user) as never,
          {
            recipient: setup.other,
          }
        )
      ).catch((err: Error) => err);
      expect(String(mint)).toMatch(/the VAA stays redeemable/);
      // Release: the CoinFactory needs the recipient's TransferPreapproval, which the flow lacks.
      const release = await collect(
        locking.redeem(
          [toOther(setup.locking)],
          new CantonAddress(setup.user) as never,
          {
            recipient: setup.other,
          }
        )
      ).catch((err: Error) => err);
      expect(release).toBeInstanceOf(NttAnchorError);
      expect(String(release)).toMatch(
        /cannot supply: recipient TransferPreapproval/
      );
      // The VAA names another party than the payer and no recipient is given.
      await expect(
        collect(
          burning.redeem(
            [toOther(setup.burning)],
            new CantonAddress(setup.user) as never
          )
        )
      ).rejects.toThrow(/pass the recipient party/);
      expect(submitCount()).toBe(submitsBefore);
      named["releaseMissingPreapproval"] = recorded
        .filter((entry) => entry.url.startsWith("/v1/flows/release"))
        .at(-1)?.body;
    });

    it("rebuilds a transfer whose contracts went stale and resubmits it under the same command id", async () => {
      const destination = { chain: peerChain, address: EVM_RECIPIENT };
      await sleep(FLOW_CACHE_MS);
      const [first] = await collect(
        locking.transfer(
          new CantonAddress(setup.user) as never,
          10_000_000_000n,
          destination,
          {
            queue: false,
          }
        )
      );
      const [second] = await collect(
        locking.transfer(
          new CantonAddress(setup.user) as never,
          10_000_000_000n,
          destination,
          {
            queue: false,
          }
        )
      );
      await signer.signAndSend([first!]);
      // The first transfer consumed the transceiver Emitter, the LockedLedger and the holdings the
      // second one names.
      const submitsBefore = submitCount();
      const [updateId] = await signer.signAndSend([second!]);
      const submits = recorded
        .filter((entry) => entry.url.startsWith("/v2/commands/submit-and-wait"))
        .slice(submitsBefore);
      expect(submits).toHaveLength(2);
      expect(submits[0]!.status).not.toBe(200);
      expect(submits[1]!.status).toBe(200);
      const commandIds = submits.map(
        (entry) =>
          (entry.request as { commands: { commandId: string } }).commands
            .commandId
      );
      expect(new Set(commandIds).size).toBe(1);
      expect(commandIds[0]).toBe(second!.transaction.commandId);
      named["staleRejection"] = submits[0];
      expect(
        locking.messageIdFromTransaction(committed.at(-1)!).sequence
      ).toBeGreaterThan(0n);
      expect(updateId).toBe(committed.at(-1)!.updateId);
    });

    it("refuses another deployment's manager, a wrong custodian and an unpinned package", async () => {
      const submitsBefore = submitCount();
      const send = (ntt: CantonNtt<Network, "Canton">) =>
        collect(
          ntt.transfer(
            new CantonAddress(setup.user) as never,
            100_000_000n,
            {
              chain: peerChain,
              address: EVM_RECIPIENT,
            },
            { queue: false }
          )
        );
      // A service that answers every flow with the burning deployment's.
      const hostile: NttFlowSource = {
        flow: (flow, parameters) =>
          flowSource.flow(flow, {
            ...parameters,
            manager: setup.burning.managerAddress,
          }),
        managers: () => flowSource.managers(),
      };
      await expect(
        send(nttOf(setup.locking, { flowSource: hostile }))
      ).rejects.toThrow(/is not signed by SdkLockAdmin|layer 3/);
      // nttCustody signs every manager; a deployment whose admin is not gg cannot have it as custodian.
      await expect(
        send(
          nttOf(setup.burning, {
            canton: { reserveCustodian: setup.nttCustody },
          })
        )
      ).rejects.toThrow(
        /layer 3: the NttManager reserveCustodian .* differs from the configured/
      );
      await expect(
        send(
          nttOf(setup.burning, {
            canton: { nttPackageIds: { current: "ab".repeat(32) } },
          })
        )
      ).rejects.toThrow(/layer 1/);
      expect(submitCount()).toBe(submitsBefore);
    });

    // ---------------------------------------------------------------------------------------------
    // getIsExecuted and the admin operations (milestone N4)
    // ---------------------------------------------------------------------------------------------

    it("answers getIsExecuted from the covering replay node: false before a redeem, true after", async () => {
      const vaa = transferVaa(setup.burning, setup.user, 100_000_000n); // 1.0 at 8 decimals
      expect(await burning.getIsExecuted(vaa)).toBe(false);
      expect(await burning.getIsApproved(vaa)).toBe(false);
      named["executedFlowBefore"] = lastRecorded("/v1/flows/set-peer");
      named["executedVaa"] = encoding.hex.encode(serialize(vaa));
      await signer.signAndSend(
        await collect(
          burning.redeem([vaa], new CantonAddress(setup.user) as never)
        )
      );
      // The flow service caches the answer for this digest.
      await sleep(FLOW_CACHE_MS);
      expect(await burning.getIsExecuted(vaa)).toBe(true);
      expect(await burning.getIsApproved(vaa)).toBe(true);
      named["executedFlowAfter"] = lastRecorded("/v1/flows/set-peer");
      // The Release of an earlier test, on the lock/unlock deployment.
      if (redeemedRelease)
        expect(await locking.getIsExecuted(redeemedRelease)).toBe(true);
    });

    it("pauses and unpauses as the admin: a paused manager refuses a transfer", async () => {
      const admin = setup.burning.admin;
      const asAdmin = nttOf(setup.burning, { adminParty: admin });
      const adminSigner = signerOf(admin);
      const destination = { chain: peerChain, address: EVM_RECIPIENT };
      // Only the admin: another party does not see the manager.
      await expect(
        collect(burning.pause(new CantonAddress(setup.user) as never))
      ).rejects.toThrow(/sees no active NttManager/);
      // The locking deployment's admin is gg: a governance action.
      await expect(
        collect(
          nttOf(setup.locking).pause(
            new CantonAddress(setup.guardianGovernance) as never
          )
        )
      ).rejects.toThrow(
        /is guardianGovernance: to pause is a governance action/
      );

      const pause = await collect(asAdmin.pause());
      named["pauseEnvelope"] = pause[0]!.transaction;
      named["pauseAcs"] = lastRecorded("/v2/state/active-contracts")?.body;
      await adminSigner.signAndSend(pause);
      // The admin's own view has the change at once; /v1/managers may still serve the old contract.
      expect(await asAdmin.isPaused()).toBe(true);
      named["pausedAcs"] = lastRecorded("/v2/state/active-contracts")?.body;
      named["pausedManagers"] = lastRecorded("/v1/managers")?.body;
      await expect(collect(asAdmin.pause())).rejects.toThrow(/already paused/);
      await sleep(FLOW_CACHE_MS);
      expect(await burning.isPaused()).toBe(true);
      await expect(
        collect(
          burning.transfer(
            new CantonAddress(setup.user) as never,
            100_000_000n,
            destination,
            { queue: false }
          )
        )
      ).rejects.toThrow(/the NttManager is paused/);

      const unpause = await collect(asAdmin.unpause());
      named["unpauseEnvelope"] = unpause[0]!.transaction;
      await adminSigner.signAndSend(unpause);
      expect(await asAdmin.isPaused()).toBe(false);
      await sleep(FLOW_CACHE_MS);
      const before = await balance(setup.burning);
      await signer.signAndSend(
        await collect(
          burning.transfer(
            new CantonAddress(setup.user) as never,
            100_000_000n,
            destination,
            { queue: false }
          )
        )
      );
      expect(before - (await balance(setup.burning))).toBe(10_000_000_000n);
    });

    it("sets a peer as the admin; getPeer reads it and a transfer uses it", async () => {
      const admin = setup.burning.admin;
      const asAdmin = nttOf(setup.burning, { adminParty: admin });
      const adminSigner = signerOf(admin);
      const sui: Chain = "Sui";
      const manager = new UniversalAddress("5a".repeat(32));
      const transceiver = new UniversalAddress("5b".repeat(32));
      // An update needs an existing peer.
      await expect(
        collect(asAdmin.setPeer({ chain: "Aptos", address: manager }, 8, 0n))
      ).rejects.toThrow(/has no peer for Aptos to update/);

      const full = await collect(
        asAdmin.setPeerFull(sui, manager, transceiver, 9)
      );
      named["setPeerFullEnvelope"] = full[0]!.transaction;
      named["setPeerFullFlow"] = lastRecorded("/v1/flows/transfer");
      named["setPeerFullAcs"] = lastRecorded(
        "/v2/state/active-contracts"
      )?.body;
      named["setPeerFullManagers"] = lastRecorded("/v1/managers")?.body;
      await adminSigner.signAndSend(full);
      named["setPeerFullTransaction"] = committed.at(-1);
      expect(await asAdmin.getPeer(sui)).toEqual({
        address: { chain: sui, address: manager },
        tokenDecimals: 9,
        inboundLimit: 0n,
      });
      expect((await asAdmin.getTransceiverPeer(0, sui))?.address).toEqual(
        transceiver
      );

      // setPeer keeps the transceiver; setTransceiverPeer keeps the manager and decimals.
      const manager2 = new UniversalAddress("5c".repeat(32));
      const update = await collect(
        asAdmin.setPeer({ chain: sui, address: manager2 }, 8, 0n)
      );
      named["setPeerEnvelope"] = update[0]!.transaction;
      await sleep(FLOW_CACHE_MS); // the transceiver Emitter changed with the last publish
      await adminSigner.signAndSend(update);
      const transceiver2 = new UniversalAddress("5d".repeat(32));
      await sleep(FLOW_CACHE_MS);
      const transceiverUpdate = await collect(
        (await asAdmin.getTransceiver(0))!.setPeer(
          { chain: sui, address: transceiver2 },
          new CantonAddress(admin) as never
        )
      );
      named["setTransceiverPeerEnvelope"] = transceiverUpdate[0]!.transaction;
      await adminSigner.signAndSend(transceiverUpdate);
      expect(await asAdmin.getPeer(sui)).toEqual({
        address: { chain: sui, address: manager2 },
        tokenDecimals: 8,
        inboundLimit: 0n,
      });
      expect((await asAdmin.getTransceiverPeer(0, sui))?.address).toEqual(
        transceiver2
      );

      // Everyone else reads the peer from /v1/managers once its cache turns over; a transfer to
      // the new peer goes through.
      await sleep(FLOW_CACHE_MS);
      expect((await burning.getPeer(sui))?.tokenDecimals).toBe(8);
      const before = await balance(setup.burning);
      const txs = await collect(
        burning.transfer(
          new CantonAddress(setup.user) as never,
          100_000_000n,
          { chain: sui, address: new UniversalAddress("00".repeat(31) + "5e") },
          { queue: false }
        )
      );
      await signer.signAndSend(txs);
      expect(before - (await balance(setup.burning))).toBe(10_000_000_000n);
      expect(
        burning.messageIdFromTransaction(committed.at(-1)!).emitter.toString()
      ).toBe(`0x${setup.burning.transceiverAddress}`);
    });

    it("transfers the admin role: the admin proposes, the new admin accepts, and back", async () => {
      const owned = setup.owned;
      const asOwner = nttOf(owned, { adminParty: owned.admin });
      // The new admin becomes the reserve custodian, so its config pins none.
      const asOther = nttOf(owned, {
        adminParty: setup.other,
        canton: { reserveCustodian: undefined },
      });
      const factory = (await asOwner.verifiedManager()).factory.contractId;
      expect((await asOwner.getOwner()).toString()).toBe(owned.admin);

      const propose = await collect(
        asOwner.setOwner(new CantonAddress(setup.other) as never)
      );
      named["setOwnerEnvelope"] = propose[0]!.transaction;
      named["setOwnerAcs"] = lastRecorded("/v2/state/active-contracts")?.body;
      await signerOf(owned.admin).signAndSend(propose);
      // Proposing changes nothing yet.
      expect((await asOther.getOwner()).toString()).toBe(owned.admin);

      // Only the named party sees the proposal, and it must name the committed factory.
      await expect(
        collect(
          asOther.acceptOwnership(
            new CantonAddress(setup.user) as never,
            factory
          )
        )
      ).rejects.toThrow(/sees no AdminTransferProposal/);
      const wrongFactory = `00${"ab".repeat(32)}`;
      await expect(
        collect(
          asOther.acceptOwnership(
            new CantonAddress(setup.other) as never,
            wrongFactory
          )
        )
      ).rejects.toThrow(/not the vetted/);

      const accept = await collect(
        asOther.acceptOwnership(
          new CantonAddress(setup.other) as never,
          factory
        )
      );
      named["acceptEnvelope"] = accept[0]!.transaction;
      named["acceptAcs"] = lastRecorded("/v2/state/active-contracts")?.body;
      named["acceptFlow"] = lastRecorded("/v1/flows/transfer");
      named["acceptManagers"] = lastRecorded("/v1/managers")?.body;
      await signerOf(setup.other).signAndSend(accept);
      expect((await asOther.getOwner()).toString()).toBe(setup.other);
      expect(await asOther.getCustodyAddress()).toBe(setup.other);
      // The old admin no longer sees the manager.
      await expect(collect(asOwner.pause())).rejects.toThrow(
        /sees no active NttManager/
      );

      // And back: the new admin proposes, the first one accepts.
      await signerOf(setup.other).signAndSend(
        await collect(asOther.setOwner(new CantonAddress(owned.admin) as never))
      );
      await sleep(FLOW_CACHE_MS);
      const acceptBack = await collect(
        asOwner.acceptOwnership(
          new CantonAddress(owned.admin) as never,
          factory
        )
      );
      named["acceptBackFlow"] = lastRecorded("/v1/flows/transfer");
      named["acceptBackManagers"] = lastRecorded("/v1/managers")?.body;
      await signerOf(owned.admin).signAndSend(acceptBack);
      expect((await asOwner.getOwner()).toString()).toBe(owned.admin);
      await sleep(FLOW_CACHE_MS);
      expect((await nttOf(owned).getOwner()).toString()).toBe(owned.admin);
    });
  });
