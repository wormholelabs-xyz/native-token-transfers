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
  broadcastThrowsFor?: Set<"approve" | "transfer">;
  tokenDecimals?: number;
  managerToken?: string;
}

const tokenReadAbi = new Interface([
  "function decimals() view returns (uint8)",
]);
const managerTokenAbi = new Interface([
  "function token() view returns (address)",
]);

function fakeSigner(options: FakeOptions) {
  const broadcasts: { kind: string; request: EthereumTransactionRequest }[] =
    [];
  const signedKinds = new Map<string, string>();
  let signCount = 0;
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
      if (selector === tokenReadAbi.getFunction("decimals")!.selector) {
        return tokenReadAbi.encodeFunctionResult("decimals", [
          options.tokenDecimals ?? ethereum.decimals,
        ]);
      }
      if (selector === managerTokenAbi.getFunction("token")!.selector) {
        return managerTokenAbi.encodeFunctionResult("token", [
          options.managerToken ?? ethereum.token,
        ]);
      }
      return "0x";
    },
    async estimateGas() {
      return 21000n;
    },
    async signTransaction(request) {
      signCount++;
      const kind = request.data.startsWith(
        tokenAbi.getFunction("approve")!.selector
      )
        ? "approve"
        : "transfer";
      const hash = `0x${String(signCount).padStart(64, "0")}`;
      signedKinds.set(hash, kind);
      broadcasts.push({ kind, request });
      return { signedTransactionHex: `0xsigned${signCount}`, hash };
    },
    async broadcastSignedTransaction(signed) {
      const kind = signedKinds.get(signed.hash) as "approve" | "transfer";
      if (options.broadcastThrowsFor?.has(kind))
        throw new Error("proxy timeout after the node accepted it");
    },
    async waitForReceipt(hash): Promise<EthereumReceiptSummary> {
      const kind = signedKinds.get(hash) as "approve" | "transfer";
      if (options.failWaitFor?.has(kind)) throw new Error("timeout");
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

  test.each([
    ["transfer", 10n ** 30n, ["transfer"], `0x${"1".padStart(64, "0")}`],
    ["approve", 0n, ["approve"], `0x${"1".padStart(64, "0")}`],
  ] as const)(
    "%s broadcast call throws after signing: unconfirmed error with the hash, no further send",
    async (failingKind, allowance, expectedKinds, expectedHash) => {
      const { signer, broadcasts } = fakeSigner({
        allowance,
        broadcastThrowsFor: new Set([failingKind]),
      });
      const sendPromise = evmSendWithSigner(signer, sendParameters, {
        dryRun: false,
      });
      await expect(sendPromise).rejects.toBeInstanceOf(
        TransferBroadcastUnconfirmedError
      );
      await sendPromise.catch((err: TransferBroadcastUnconfirmedError) => {
        expect(err.txHash).toBe(expectedHash);
        expect(err.message).toMatch(/do not re-run; check the hash/);
      });
      expect(broadcasts.map((broadcast) => broadcast.kind)).toEqual([
        ...expectedKinds,
      ]);
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
    [
      "token decimals differ from the registry",
      { tokenDecimals: 6 },
      /decimals/,
    ],
    [
      "manager token differs from the registry",
      { managerToken: "0x000000000000000000000000000000000000bEEF" },
      /token/,
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

  test("the hash is printed before the broadcast call", async () => {
    const { signer } = fakeSigner({ allowance: 10n ** 30n });
    const events: string[] = [];
    const originalBroadcast = signer.broadcastSignedTransaction;
    signer.broadcastSignedTransaction = async (signed) => {
      events.push(`broadcast ${signed.hash}`);
      return originalBroadcast(signed);
    };
    const originalLog = console.log;
    console.log = (...parts: unknown[]) => {
      const line = parts.join(" ");
      if (line.includes("tx 0x"))
        events.push(`print ${line.trim().split("tx ")[1]}`);
    };
    try {
      await evmSendWithSigner(signer, sendParameters, { dryRun: false });
    } finally {
      console.log = originalLog;
    }
    const hash = `0x${"1".padStart(64, "0")}`;
    expect(events).toEqual([`print ${hash}`, `broadcast ${hash}`]);
  });

  test("dry run: 0 broadcasts", async () => {
    const { signer, broadcasts } = fakeSigner({ allowance: 0n });
    expect(
      await evmSendWithSigner(signer, sendParameters, { dryRun: true })
    ).toBeUndefined();
    expect(broadcasts.length).toBe(0);
  });
});
