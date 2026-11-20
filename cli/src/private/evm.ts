// ethers adapter for EthereumSigner. RPC through the labs proxy with the required Origin header.
import { FetchRequest, JsonRpcProvider, Wallet } from "ethers";

import type { NetworkName } from "./config.js";
import type { EthereumReceiptSummary, EthereumSigner } from "./evm-send.js";

const DEFAULT_ETHEREUM_RPC_BY_NETWORK: Record<NetworkName, string> = {
  Mainnet: "https://rpc.labsapis.com/mainnet/ethereum",
  Testnet: "https://rpc.labsapis.com/testnet/sepolia",
};
const RPC_PROXY_ORIGIN = "https://portalbridge.com";
const ETHEREUM_RPC_TIMEOUT_MS = 15_000;

function readEthereumPrivateKeyHex(): string {
  const ethereumPrivateKeyHex = process.env.ETH_PRIVATE_KEY;
  if (!ethereumPrivateKeyHex)
    throw new Error("ETH_PRIVATE_KEY env var not set");
  return ethereumPrivateKeyHex;
}

/** Wallet address from ETH_PRIVATE_KEY, computed locally. */
export function deriveEthereumAddress(): string {
  return new Wallet(readEthereumPrivateKeyHex()).address;
}

export function buildEthersSigner(network: NetworkName): EthereumSigner {
  const rpcRequest = new FetchRequest(
    process.env.ETH_RPC ?? DEFAULT_ETHEREUM_RPC_BY_NETWORK[network]
  );
  rpcRequest.setHeader("Origin", RPC_PROXY_ORIGIN);
  rpcRequest.timeout = ETHEREUM_RPC_TIMEOUT_MS;
  const provider = new JsonRpcProvider(rpcRequest, undefined, {
    staticNetwork: false,
  });
  const wallet = new Wallet(readEthereumPrivateKeyHex(), provider);

  return {
    getAddress: () => wallet.getAddress(),
    getChainId: async () => (await provider.getNetwork()).chainId,
    call: (request) => wallet.call(request),
    estimateGas: (request) => wallet.estimateGas(request),
    async broadcast(request) {
      const sent = await wallet.sendTransaction(request);
      return {
        hash: sent.hash,
        async waitForReceipt(
          confirmations,
          timeoutMs
        ): Promise<EthereumReceiptSummary | null> {
          const receipt = await sent.wait(confirmations, timeoutMs);
          if (!receipt) return null;
          return {
            hash: receipt.hash,
            status: receipt.status ?? 0,
            logs: receipt.logs.map((log) => ({
              address: log.address,
              topics: log.topics,
              data: log.data,
            })),
          };
        },
      };
    },
  };
}
