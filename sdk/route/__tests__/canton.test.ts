// The NTT routes with a Canton leg, in both directions. The Canton Ntt, the signer, Wormholescan
// and the participant are mocks; the Canton platform, its addresses and decimals are the real ones.
import { jest } from "@jest/globals";
import {
  Network,
  TransferState,
  Wormhole,
  amount,
} from "@wormhole-foundation/sdk-connect";
import * as routes from "@wormhole-foundation/sdk-connect/routes";
import type { ChainsConfig, Signer } from "@wormhole-foundation/sdk-connect";

import { register as registerDefinitionsNtt } from "@wormhole-foundation/sdk-definitions-ntt";
import { register as registerEvmNtt } from "@wormhole-foundation/sdk-evm-ntt";

registerDefinitionsNtt();
registerEvmNtt();

import { EvmPlatform } from "@wormhole-foundation/sdk-evm";
import { CantonPlatform } from "@wormhole-foundation/sdk-canton";
import { nttManualRoute } from "../src/manual.js";
import { nttExecutorRoute } from "../src/executor/executor.js";
import { NttRoute } from "../src/types.js";

const NAMESPACE = `1220${"ab".repeat(32)}`;
const GG = `guardianGovernance::${NAMESPACE}`;
const CUSTODIAN = `custodian::${NAMESPACE}`;
const ALICE = `alice::${NAMESPACE}`;
const BOB = `bob::${NAMESPACE}`;
const CANTON_TOKEN = `${GG}/wormhole-ntt:${"cd".repeat(32)}`;
const CANTON_MANAGER = "aa".repeat(32);
const CANTON_TRANSCEIVER = "bb".repeat(32);
const SEPOLIA_TOKEN = "0x738141EFf659625F2eAD4feECDfCD94155C67f18";
const SEPOLIA_RECIPIENT = "0x9b2A3B92b1D86938D3Ed37B0519952C227bA6D09";
const CANTON_SETTINGS: NttRoute.CantonSettings = {
  reserveCustodian: CUSTODIAN,
  flowServiceUrl: "http://flows.test",
  nttPackageIds: { current: "43".repeat(32) },
  corePackageIds: { current: "32".repeat(32) },
  mode: "burning",
};

function config(cantonDecimals?: number): NttRoute.Config {
  return {
    tokens: {
      TestToken: [
        {
          chain: "Canton",
          token: CANTON_TOKEN,
          manager: CANTON_MANAGER,
          transceiver: [{ type: "wormhole", address: CANTON_TRANSCEIVER }],
          ...(cantonDecimals !== undefined
            ? { tokenDecimals: cantonDecimals }
            : {}),
          canton: CANTON_SETTINGS,
        },
        {
          chain: "Sepolia",
          token: SEPOLIA_TOKEN,
          manager: "0x649fF7B32C2DE771043ea105c4aAb2D724497238",
          transceiver: [
            {
              type: "wormhole",
              address: "0x06413c42e913327Bc9a08B7C1E362BAE7C0b9598",
            },
          ],
          tokenDecimals: 18,
        },
      ],
    },
  };
}

/** The Canton platform with the NTT token's decimals, as a route user configures it */
class CantonWithDecimals<N extends Network> extends CantonPlatform<N> {
  constructor(network: N, chains?: ChainsConfig<N, "Canton">) {
    super(network, chains, { instrumentDecimals: { [CANTON_TOKEN]: 8 } });
  }
}

const CHAINS = {
  chains: {
    Canton: {
      rpc: "http://participant.test",
      contracts: { coreBridge: GG },
    },
  },
};

function wormholeWith(cantonPlatform: typeof CantonPlatform) {
  return new Wormhole("Testnet", [EvmPlatform, cantonPlatform], CHAINS);
}

interface FakeNtt {
  calls: { method: string; args: unknown[] }[];
  getProtocol: (name: string, contracts: unknown) => Promise<unknown>;
  contracts: unknown[];
}

/** A Canton or EVM Ntt that records its calls; the generators yield one marker transaction */
function fakeNtt(
  options: {
    tokenDecimals?: number;
    approved?: boolean;
    executed?: boolean;
  } = {}
): FakeNtt {
  const calls: FakeNtt["calls"] = [];
  const contracts: unknown[] = [];
  const ntt = {
    getTokenDecimals: async () => options.tokenDecimals ?? 8,
    getRateLimitDuration: async () => 0n,
    getCurrentInboundCapacity: async () => 0n,
    getIsApproved: async () => options.approved ?? false,
    getIsExecuted: async () => options.executed ?? false,
    getInboundQueuedTransfer: async () => null,
    async *transfer(...args: unknown[]) {
      calls.push({ method: "transfer", args });
      yield { description: "transfer" };
    },
    async *redeem(...args: unknown[]) {
      calls.push({ method: "redeem", args });
      yield { description: "redeem" };
    },
  };
  return {
    calls,
    contracts,
    getProtocol: async (name: string, given: unknown) => {
      if (name !== "Ntt") throw new Error(`unexpected protocol ${name}`);
      contracts.push(given);
      return ntt;
    },
  };
}

/** A signer that returns one transaction id per transaction */
function fakeSigner(chain: string, address: string) {
  const signed: unknown[] = [];
  const signer = {
    chain: () => chain,
    address: () => address,
    signAndSend: async (txs: unknown[]) => {
      signed.push(...txs);
      return txs.map((_, index) => `${chain.toLowerCase()}-tx-${index}`);
    },
  } as unknown as Signer;
  return { signer, signed };
}

/** An attested VAA, as the route reads it */
function attestedVaa(from: "Canton" | "Sepolia") {
  return {
    emitterChain: from,
    emitterAddress: { toString: () => "emitter" },
    sequence: 7n,
    payload: { nttManagerPayload: { payload: {} } },
  } as never;
}

describe("Manual route with a Canton leg", () => {
  const wh = wormholeWith(CantonWithDecimals);
  const route = nttManualRoute(config(8));
  const canton = wh.getChain("Canton");
  const sepolia = wh.getChain("Sepolia");

  afterEach(() => {
    jest.restoreAllMocks();
  });

  it("offers Canton as a chain and the instrument as a token", async () => {
    expect(route.supportedChains("Testnet")).toEqual(["Canton", "Sepolia"]);
    const tokens = await route.supportedSourceTokens(canton);
    expect(tokens.map((token) => token.address.toString())).toEqual([
      CANTON_TOKEN,
    ]);
    const destinations = await route.supportedDestinationTokens(
      Wormhole.tokenId("Sepolia", SEPOLIA_TOKEN),
      sepolia,
      canton
    );
    expect(destinations.map((token) => token.address.toString())).toEqual([
      CANTON_TOKEN,
    ]);
  });

  async function request(from: "Canton" | "Sepolia", w = wh) {
    const cantonToken = Wormhole.tokenId("Canton", CANTON_TOKEN);
    const sepoliaToken = Wormhole.tokenId("Sepolia", SEPOLIA_TOKEN);
    return routes.RouteTransferRequest.create(w, {
      source: from === "Canton" ? cantonToken : sepoliaToken,
      destination: from === "Canton" ? sepoliaToken : cantonToken,
      ...(from === "Sepolia" ? { sourceDecimals: 18 } : {}),
      ...(from === "Canton" ? { destinationDecimals: 18 } : {}),
    });
  }

  it("Canton -> Sepolia: validates, quotes, and transfers the amount at the manager's decimals with the Canton settings", async () => {
    const req = await request("Canton");
    expect(req.source.decimals).toBe(8);
    const rt = new route(wh);
    const validated = await rt.validate(req, { amount: "1.5" });
    if (!validated.valid) throw validated.error;
    const params = validated.params as NttRoute.ValidatedParams;
    expect(params.normalizedParams.sourceContracts).toMatchObject({
      token: CANTON_TOKEN,
      manager: CANTON_MANAGER,
      tokenDecimals: 8,
      transceiver: { wormhole: CANTON_TRANSCEIVER },
      canton: CANTON_SETTINGS,
    });
    expect(params.normalizedParams.destinationContracts.canton).toBeUndefined();

    const source = fakeNtt();
    const destination = fakeNtt();
    jest
      .spyOn(req.fromChain, "getProtocol")
      .mockImplementation(source.getProtocol as never);
    jest
      .spyOn(req.toChain, "getProtocol")
      .mockImplementation(destination.getProtocol as never);
    const quote = await rt.quote(req, params);
    if (!quote.success) throw quote.error;
    expect(amount.units(quote.sourceToken.amount)).toBe(150_000_000n);
    expect(quote.eta).toBeGreaterThan(0);

    const { signer, signed } = fakeSigner("Canton", ALICE);
    const to = Wormhole.chainAddress("Sepolia", SEPOLIA_RECIPIENT);
    const receipt = await rt.initiate(req, signer, quote, to);
    expect(receipt.state).toBe(TransferState.SourceInitiated);
    expect(receipt.originTxs).toEqual([
      { chain: "Canton", txid: "canton-tx-0" },
    ]);
    expect(signed).toHaveLength(1);
    const [transfer] = source.calls;
    expect(transfer!.method).toBe("transfer");
    const [sender, units, destinationAddress, options] = transfer!.args as [
      { toString(): string },
      bigint,
      unknown,
      unknown,
    ];
    expect(sender.toString()).toBe(ALICE);
    expect(units).toBe(150_000_000n);
    expect(destinationAddress).toBe(to);
    expect(options).toEqual({
      queue: false,
      automatic: false,
      wrapNative: false,
    });
    // The Canton Ntt got the Canton settings with its contracts.
    expect(source.contracts.at(-1)).toMatchObject({
      ntt: { canton: CANTON_SETTINGS },
    });
    // A transfer out of Canton records no recipient.
    expect(receipt.params.normalizedParams.recipient).toBeUndefined();
  });

  it("Sepolia -> Canton: records the recipient party, and the redeem names it when the signer is another party", async () => {
    const req = await request("Sepolia");
    expect(req.destination.decimals).toBe(8);
    const rt = new route(wh);
    const validated = await rt.validate(req, { amount: "2" });
    if (!validated.valid) throw validated.error;
    const params = validated.params as NttRoute.ValidatedParams;
    expect(params.normalizedParams.destinationContracts.canton).toEqual(
      CANTON_SETTINGS
    );

    const source = fakeNtt();
    const destination = fakeNtt();
    jest
      .spyOn(req.fromChain, "getProtocol")
      .mockImplementation(source.getProtocol as never);
    jest
      .spyOn(req.toChain, "getProtocol")
      .mockImplementation(destination.getProtocol as never);
    const quote = await rt.quote(req, params);
    if (!quote.success) throw quote.error;
    expect(amount.units(quote.destinationToken.amount)).toBe(200_000_000n);

    const { signer } = fakeSigner("Sepolia", SEPOLIA_RECIPIENT);
    const receipt = await rt.initiate(
      req,
      signer,
      quote,
      Wormhole.chainAddress("Canton", ALICE)
    );
    expect(receipt.params.normalizedParams.recipient).toBe(ALICE);

    // The redeem by another party (an executor) carries the recipient.
    const attested = {
      ...receipt,
      state: TransferState.Attested,
      attestation: { id: {} as never, attestation: attestedVaa("Sepolia") },
    } as NttRoute.ManualTransferReceipt;
    const { signer: bobSigner } = fakeSigner("Canton", BOB);
    const completed = await rt.complete(bobSigner, attested);
    expect(completed.state).toBe(TransferState.DestinationInitiated);
    expect(completed.destinationTxs).toEqual([
      { chain: "Canton", txid: "canton-tx-0" },
    ]);
    const [redeem] = destination.calls;
    expect(redeem!.method).toBe("redeem");
    expect((redeem!.args[1] as { toString(): string }).toString()).toBe(BOB);
    expect(redeem!.args[2]).toEqual({ recipient: ALICE });
    expect(destination.contracts.at(-1)).toMatchObject({
      ntt: { manager: CANTON_MANAGER, canton: CANTON_SETTINGS },
    });
  });

  it("an EVM redeem gets no recipient option", async () => {
    const req = await request("Canton");
    const rt = new route(wh);
    const validated = await rt.validate(req, { amount: "1" });
    if (!validated.valid) throw validated.error;
    const destination = fakeNtt();
    jest
      .spyOn(sepolia, "getProtocol")
      .mockImplementation(destination.getProtocol as never);
    const receipt = {
      from: "Canton",
      to: "Sepolia",
      state: TransferState.Attested,
      originTxs: [],
      params: validated.params,
      attestation: { id: {} as never, attestation: attestedVaa("Canton") },
    } as unknown as NttRoute.ManualTransferReceipt;
    await rt.complete(fakeSigner("Sepolia", SEPOLIA_RECIPIENT).signer, receipt);
    expect(destination.calls[0]!.args[2]).toBeUndefined();
  });

  it("tracks a transfer into Canton with getIsApproved and getIsExecuted", async () => {
    const req = await request("Sepolia");
    const rt = new route(wh);
    const validated = await rt.validate(req, { amount: "1" });
    if (!validated.valid) throw validated.error;
    const destination = fakeNtt({ approved: true, executed: true });
    jest
      .spyOn(canton, "getProtocol")
      .mockImplementation(destination.getProtocol as never);
    const receipt = {
      from: "Sepolia",
      to: "Canton",
      state: TransferState.Attested,
      originTxs: [{ chain: "Sepolia", txid: "0x01" }],
      params: validated.params,
      attestation: { id: {} as never, attestation: attestedVaa("Sepolia") },
    } as unknown as NttRoute.ManualTransferReceipt;
    const states: TransferState[] = [];
    for await (const update of rt.track(receipt)) states.push(update.state);
    expect(states).toEqual([
      TransferState.DestinationInitiated,
      TransferState.DestinationFinalized,
      TransferState.DestinationFinalized,
    ]);
  });

  it("tracks a transfer out of Canton by the message id of its update, not by Wormholescan's transaction index", async () => {
    const req = await request("Canton");
    const rt = new route(wh);
    const validated = await rt.validate(req, { amount: "1" });
    if (!validated.valid) throw validated.error;
    const messageId = {
      chain: "Canton",
      emitter: { toString: () => CANTON_TRANSCEIVER },
      sequence: 7n,
    };
    const parse = jest
      .spyOn(canton, "parseTransaction")
      .mockResolvedValue([messageId] as never);
    const vaa = attestedVaa("Canton");
    const getVaa = jest.spyOn(wh, "getVaa").mockResolvedValue(vaa);
    jest
      .spyOn(sepolia, "getProtocol")
      .mockImplementation(fakeNtt().getProtocol as never);
    const receipt = {
      from: "Canton",
      to: "Sepolia",
      state: TransferState.SourceInitiated,
      originTxs: [{ chain: "Canton", txid: "1220" + "ef".repeat(32) }],
      params: validated.params,
    } as unknown as NttRoute.ManualTransferReceipt;
    const tracker = rt.track(receipt);
    const first = await tracker.next();
    expect(first.value?.state).toBe(TransferState.Attested);
    expect(parse).toHaveBeenCalledWith("1220" + "ef".repeat(32));
    expect(getVaa).toHaveBeenCalledWith(
      messageId,
      "Ntt:WormholeTransfer",
      undefined
    );
  });

  it("refuses a request built with the platform's default 10 decimals for the 8-decimal token", async () => {
    const plain = wormholeWith(CantonPlatform);
    const req = await request("Canton", plain);
    expect(req.source.decimals).toBe(10);
    const validated = await new route(plain).validate(req, { amount: "1" });
    expect(validated.valid).toBe(false);
    expect((validated as { error: Error }).error.message).toMatch(
      /The Canton source token .* has 8 decimals, the transfer request 10\. Set CantonPlatformOptions\.instrumentDecimals/
    );
  });

  it("without tokenDecimals in the config, the quote and initiate read the manager's", async () => {
    const plain = wormholeWith(CantonPlatform);
    const unconfigured = nttManualRoute(config());
    const req = await request("Canton", plain);
    const rt = new unconfigured(plain);
    const validated = await rt.validate(req, { amount: "1" });
    if (!validated.valid) throw validated.error;
    const source = fakeNtt({ tokenDecimals: 8 });
    jest
      .spyOn(req.fromChain, "getProtocol")
      .mockImplementation(source.getProtocol as never);
    jest
      .spyOn(req.toChain, "getProtocol")
      .mockImplementation(fakeNtt().getProtocol as never);
    const quote = await rt.quote(req, validated.params);
    expect(quote.success).toBe(false);
    expect((quote as { error: Error }).error.message).toMatch(
      /has 8 decimals, the transfer request 10/
    );
    // initiate checks again, so a stale quote cannot send 100 times the amount.
    const forged = {
      success: true,
      params: validated.params,
    } as never;
    await expect(
      rt.initiate(
        req,
        fakeSigner("Canton", ALICE).signer,
        forged,
        Wormhole.chainAddress("Sepolia", SEPOLIA_RECIPIENT)
      )
    ).rejects.toThrow(/has 8 decimals, the transfer request 10/);
    expect(source.calls).toHaveLength(0);
  });
});

describe("Executor route with a Canton leg", () => {
  const wh = wormholeWith(CantonWithDecimals);
  const route = nttExecutorRoute({ ntt: config(8) });

  it("does not offer Canton, and refuses a Canton request clearly", async () => {
    expect(route.supportedChains("Testnet")).toEqual(["Sepolia"]);
    const req = await routes.RouteTransferRequest.create(wh, {
      source: Wormhole.tokenId("Canton", CANTON_TOKEN),
      destination: Wormhole.tokenId("Sepolia", SEPOLIA_TOKEN),
      destinationDecimals: 18,
    });
    const validated = await new route(wh).validate(req, { amount: "1" });
    expect(validated.valid).toBe(false);
    expect((validated as { error: Error }).error.message).toMatch(
      /The NTT executor route does not serve Canton: no executor relays to or from Canton\. Use the manual route/
    );
  });
});
