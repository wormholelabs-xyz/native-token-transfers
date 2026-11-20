import { describe, expect, test } from "bun:test";
import { AbiCoder, Interface, id } from "ethers";

import { validateRegistry, requireDeployment } from "../config.js";
import {
  TransferBroadcastUnconfirmedError,
  evmSendWithSigner,
  type EthereumReceiptSummary,
  type EthereumSigner,
  type EthereumTransactionRequest,
} from "../evm-send.js";
import { parseHex32, parseRawAmount, type Hex32 } from "../types.js";
import { retiredCanburnRegistryJson } from "./fixtures/registry.js";

const registry = validateRegistry(retiredCanburnRegistryJson());
const deployment = requireDeployment(registry, "canburn");
const ethereum = deployment.chains.Ethereum!;
const CANTON_MANAGER = deployment.chains.Canton!.manager;
const SENDER = "0x000000000000000000000000000000000000dEaD";
const RECIPIENT = parseHex32("11".repeat(32));
const DELIVERY_PRICE_WEI = 12345n;

const managerAbi = new Interface([
  "function getPeer(uint16) view returns (tuple(bytes32 peerAddress, uint8 tokenDecimals))",
  "function quoteDeliveryPrice(uint16, bytes) view returns (uint256[], uint256)",
  "function transfer(uint256, uint16, bytes32) payable returns (uint64)",
]);
const tokenAbi = new Interface([
  "function allowance(address, address) view returns (uint256)",
  "function approve(address, uint256) returns (bool)",
]);
const LOG_MESSAGE_PUBLISHED_TOPIC = id(
  "LogMessagePublished(address,uint64,uint32,bytes,uint8)"
);

interface FakeOptions {
  chainId?: bigint;
  allowance?: bigint;
  peer?: Hex32;
  quoteReverts?: boolean;
  failWaitFor?: Set<"approve" | "transfer">;
}

function fakeSigner(options: FakeOptions) {
  const broadcasts: { kind: string; request: EthereumTransactionRequest }[] =
    [];
  const signer: EthereumSigner = {
    async getAddress() {
      return SENDER;
    },
    async getChainId() {
      return options.chainId ?? 1n;
    },
    async call(request) {
      const selector = request.data.slice(0, 10);
      if (selector === managerAbi.getFunction("getPeer")!.selector) {
        return managerAbi.encodeFunctionResult("getPeer", [
          [`0x${options.peer ?? CANTON_MANAGER}`, 10],
        ]);
      }
      if (selector === managerAbi.getFunction("quoteDeliveryPrice")!.selector) {
        if (options.quoteReverts) throw new Error("execution reverted");
        return managerAbi.encodeFunctionResult("quoteDeliveryPrice", [
          [DELIVERY_PRICE_WEI],
          DELIVERY_PRICE_WEI,
        ]);
      }
      if (selector === tokenAbi.getFunction("allowance")!.selector) {
        return tokenAbi.encodeFunctionResult("allowance", [
          options.allowance ?? 0n,
        ]);
      }
      return "0x";
    },
    async estimateGas() {
      return 21000n;
    },
    async broadcast(request) {
      const kind = request.data.startsWith(
        tokenAbi.getFunction("approve")!.selector
      )
        ? "approve"
        : "transfer";
      broadcasts.push({ kind, request });
      const hash = `0x${String(broadcasts.length).padStart(64, "0")}`;
      return {
        hash,
        async waitForReceipt(): Promise<EthereumReceiptSummary> {
          if (options.failWaitFor?.has(kind as "approve" | "transfer"))
            throw new Error("timeout");
          const logs =
            kind === "transfer"
              ? [
                  {
                    address: "0x98f3c9e6E3fAce36bAAd05FE09d375Ef1464288B",
                    topics: [
                      LOG_MESSAGE_PUBLISHED_TOPIC,
                      AbiCoder.defaultAbiCoder().encode(
                        ["address"],
                        [ethereum.transceiver]
                      ),
                    ],
                    data: AbiCoder.defaultAbiCoder().encode(
                      ["uint64", "uint32", "bytes", "uint8"],
                      [42n, 0, "0x", 1]
                    ),
                  },
                ]
              : [];
          return { hash, status: 1, logs };
        },
      };
    },
  };
  return { signer, broadcasts };
}

const sendParameters = {
  registry,
  deployment,
  destinationChain: "Canton" as const,
  rawAmount: parseRawAmount(10n ** 18n),
  recipient: RECIPIENT,
};

describe("evmSendWithSigner", () => {
  test("allowance sufficient: 0 approve, 1 transfer with value == quote, sequence from the core log", async () => {
    const { signer, broadcasts } = fakeSigner({ allowance: 10n ** 30n });
    const result = await evmSendWithSigner(signer, sendParameters, {
      dryRun: false,
    });
    expect(broadcasts.map((broadcast) => broadcast.kind)).toEqual(["transfer"]);
    expect(broadcasts[0].request.value).toBe(DELIVERY_PRICE_WEI);
    expect(result?.sequence).toBe("42");
  });

  test.each([
    [
      "transfer wait fails",
      new Set<"approve" | "transfer">(["transfer"]),
      ["approve", "transfer"],
    ],
  ])(
    "%s after broadcast: no second send, error carries the hash",
    async (_caseName, failWaitFor, expectedKinds) => {
      const { signer, broadcasts } = fakeSigner({ allowance: 0n, failWaitFor });
      const sendPromise = evmSendWithSigner(signer, sendParameters, {
        dryRun: false,
      });
      await expect(sendPromise).rejects.toBeInstanceOf(
        TransferBroadcastUnconfirmedError
      );
      await sendPromise.catch((err: TransferBroadcastUnconfirmedError) => {
        expect(err.txHash).toBe(`0x${"2".padStart(64, "0")}`);
        expect(err.message).toMatch(/do not re-run/);
      });
      expect(broadcasts.map((broadcast) => broadcast.kind)).toEqual(
        expectedKinds
      );
    }
  );

  test("approve wait fails: stop after the approve, no transfer", async () => {
    const { signer, broadcasts } = fakeSigner({
      allowance: 0n,
      failWaitFor: new Set(["approve"]),
    });
    await expect(
      evmSendWithSigner(signer, sendParameters, { dryRun: false })
    ).rejects.toThrow(/approve/);
    expect(broadcasts.map((broadcast) => broadcast.kind)).toEqual(["approve"]);
  });

  test.each([
    ["quote reverts", { quoteReverts: true }, /quoteDeliveryPrice/],
    ["wrong chain id for Mainnet", { chainId: 11155111n }, /chain id/],
    [
      "peer differs by 1 byte",
      { peer: parseHex32("00" + CANTON_MANAGER.slice(2)) },
      /peer/,
    ],
  ])("%s: 0 broadcasts", async (_caseName, options, expectedError) => {
    const { signer, broadcasts } = fakeSigner({
      allowance: 10n ** 30n,
      ...options,
    });
    await expect(
      evmSendWithSigner(signer, sendParameters, { dryRun: false })
    ).rejects.toThrow(expectedError);
    expect(broadcasts.length).toBe(0);
  });

  test("dry run: 0 broadcasts", async () => {
    const { signer, broadcasts } = fakeSigner({ allowance: 0n });
    expect(
      await evmSendWithSigner(signer, sendParameters, { dryRun: true })
    ).toBeUndefined();
    expect(broadcasts.length).toBe(0);
  });
});
