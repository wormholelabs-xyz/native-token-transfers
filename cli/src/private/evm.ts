// Drives Ethereum sends/redeems directly against ethers + EvmNtt — never
// through ChainContext/signSendWait, so we control the RPC Origin header and
// can target a Canton destination chain the platform layer knows nothing
// about. Never import sdk-base / sdk-definitions / sdk-connect /
// sdk-definitions-ntt by bare name: those resolve to the nested Canton-less
// 2.x copies under cli/node_modules (pulled in by sdk-sui-ntt).
import {
  Contract,
  FetchRequest,
  JsonRpcProvider,
  Wallet,
  type TransactionReceipt,
  type TransactionRequest,
} from "ethers";

import { toUniversal, type Network } from "@wormhole-foundation/sdk";
import { EvmAddress } from "@wormhole-foundation/sdk-evm";
import { EvmNtt } from "@wormhole-foundation/sdk-evm-ntt";

import { toRaw } from "./amounts.js";
import { hexToBytes, strip0x } from "./binding.js";
import type {
  ChainName,
  DeploymentConfig,
  EthereumChainConfig,
  PrivateDeploymentsConfig,
} from "./config.js";

const DEFAULT_ETH_RPC = "https://rpc.labsapis.com/mainnet/ethereum";
const PROXY_ORIGIN = "https://portalbridge.com";

// keccak256("TransferSent(bytes32,bytes32,uint256,uint256,uint16,uint64)") —
// the Wormhole NttManager's outbound event; first 32 bytes of `data` is the sequence.
const NTT_TRANSFER_SENT_TOPIC =
  "0x6eb224fb001ed210e379b335e35efe88672a8ce935d981a6896b27ffdf52a3b2";

export interface EvmOpOptions {
  dryRun?: boolean;
}

export interface EvmSendResult {
  txHash: string;
  sequence?: string;
}

export interface EvmRedeemResult {
  txHash: string;
}

function buildEthProvider(): JsonRpcProvider {
  const url = process.env.ETH_RPC ?? DEFAULT_ETH_RPC;
  const req = new FetchRequest(url);
  req.setHeader("Origin", PROXY_ORIGIN);
  return new JsonRpcProvider(req);
}

function buildEthWallet(provider: JsonRpcProvider): Wallet {
  const pk = process.env.ETH_PRIVATE_KEY;
  if (!pk) throw new Error("ETH_PRIVATE_KEY env var not set");
  return new Wallet(pk, provider);
}

/** Wallet address derived from ETH_PRIVATE_KEY, pure (no RPC calls). */
export function deriveEthAddress(): string {
  const pk = process.env.ETH_PRIVATE_KEY;
  if (!pk) throw new Error("ETH_PRIVATE_KEY env var not set");
  return new Wallet(pk).address;
}

function parseSequenceFromReceipt(receipt: TransactionReceipt): string | undefined {
  for (const log of receipt.logs) {
    if (log.topics[0]?.toLowerCase() === NTT_TRANSFER_SENT_TOPIC) {
      const first32 = "0x" + log.data.slice(2, 66);
      return BigInt(first32).toString();
    }
  }
  return undefined;
}

/**
 * Signs and sends (or, in dry-run mode, statically calls + estimates gas)
 * one unsigned transaction. Returns the receipt, or undefined in dry-run.
 */
async function sendOrSimulate(
  wallet: Wallet,
  tx: TransactionRequest,
  description: string,
  dryRun: boolean | undefined
): Promise<TransactionReceipt | undefined> {
  if (dryRun) {
    try {
      await wallet.call(tx);
    } catch (err) {
      throw new Error(`[dry-run] ${description} would revert: ${(err as Error).message}`);
    }
    const gas = await wallet.estimateGas(tx);
    console.log(`  [dry-run] ${description}: ok, ~${gas} gas`);
    return undefined;
  }
  const sent = await wallet.sendTransaction(tx);
  console.log(`  ${description}: tx ${sent.hash}`);
  const receipt = await sent.wait();
  if (!receipt) {
    throw new Error(`${description}: transaction ${sent.hash} did not confirm`);
  }
  return receipt;
}

async function evmSendViaSdk(
  cfg: PrivateDeploymentsConfig,
  eth: EthereumChainConfig,
  wallet: Wallet,
  rawAmount: bigint,
  destChain: ChainName,
  recipient32Hex: string,
  opts: EvmOpOptions
): Promise<EvmSendResult | undefined> {
  const provider = wallet.provider as JsonRpcProvider;
  const ntt = new EvmNtt(cfg.network as Network, "Ethereum", provider, {
    ntt: {
      manager: eth.manager,
      token: eth.token,
      transceiver: { wormhole: eth.transceiver },
    },
  });

  const sender = new EvmAddress(await wallet.getAddress());
  const destination = {
    chain: destChain,
    address: toUniversal(destChain, hexToBytes(recipient32Hex)),
  };

  const generator = ntt.transfer(sender as any, rawAmount, destination as any, {
    queue: false,
    automatic: false,
  });

  let lastReceipt: TransactionReceipt | undefined;
  for await (const utx of generator) {
    const receipt = await sendOrSimulate(wallet, utx.transaction, utx.description, opts.dryRun);
    if (receipt) lastReceipt = receipt;
  }

  if (opts.dryRun || !lastReceipt) return undefined;
  return { txHash: lastReceipt.hash, sequence: parseSequenceFromReceipt(lastReceipt) };
}

/**
 * ntt-lib.sh's exact fallback: raw approve + the 3-arg
 * transfer(uint256,uint16,bytes32) overload, bypassing the SDK's
 * quoter/6-arg path entirely. Used when that path throws at runtime.
 */
async function evmSendRaw(
  cfg: PrivateDeploymentsConfig,
  eth: EthereumChainConfig,
  wallet: Wallet,
  rawAmount: bigint,
  destChain: ChainName,
  recipient32Hex: string,
  opts: EvmOpOptions
): Promise<EvmSendResult | undefined> {
  const chainId = cfg.chainIds[destChain];
  const token = new Contract(
    eth.token,
    [
      "function approve(address,uint256) returns (bool)",
      "function allowance(address,address) view returns (uint256)",
    ],
    wallet
  );
  const manager = new Contract(
    eth.manager,
    ["function transfer(uint256,uint16,bytes32) returns (uint64)"],
    wallet
  );

  const senderAddress = await wallet.getAddress();
  const allowance: bigint = await token.allowance(senderAddress, eth.manager);
  const needsApprove = allowance < rawAmount;
  if (needsApprove) {
    const approveTx = await token.approve.populateTransaction(eth.manager, rawAmount);
    await sendOrSimulate(wallet, approveTx, "approve", opts.dryRun);
  }

  const recipientBytes32 = "0x" + strip0x(recipient32Hex);
  const transferTx = await manager.transfer.populateTransaction(
    rawAmount,
    chainId,
    recipientBytes32
  );
  let receipt: TransactionReceipt | undefined;
  try {
    receipt = await sendOrSimulate(wallet, transferTx, "transfer (raw 3-arg)", opts.dryRun);
  } catch (err) {
    // eth_call cannot persist the simulated approve, so an allowance revert in
    // dry-run only reflects that sequencing gap, not a real failure.
    if (opts.dryRun && needsApprove && (err as Error).message.includes("insufficient allowance")) {
      console.log(
        "  [dry-run] transfer (raw 3-arg): allowance revert expected here — " +
          "the simulated approve does not persist across eth_call; calldata built and verified"
      );
      return undefined;
    }
    throw err;
  }

  if (opts.dryRun || !receipt) return undefined;
  return { txHash: receipt.hash, sequence: parseSequenceFromReceipt(receipt) };
}

/**
 * Sends an outbound NTT transfer from Ethereum. Tries EvmNtt.transfer (the
 * quoter-aware 6-arg overload) first; if that throws at runtime (e.g. the
 * deployed manager predates the quoter), falls back to the raw 3-arg
 * overload ntt-lib.sh used exclusively.
 */
export async function evmSend(
  cfg: PrivateDeploymentsConfig,
  dep: DeploymentConfig,
  amountHuman: string,
  destChain: ChainName,
  recipient32Hex: string,
  opts: EvmOpOptions = {}
): Promise<EvmSendResult | undefined> {
  const eth = dep.chains.Ethereum;
  const provider = buildEthProvider();
  const wallet = buildEthWallet(provider);
  const rawAmount = toRaw(amountHuman, eth.decimals);

  try {
    return await evmSendViaSdk(cfg, eth, wallet, rawAmount, destChain, recipient32Hex, opts);
  } catch (sdkErr) {
    console.error(
      `[evm] EvmNtt.transfer failed (${(sdkErr as Error).message}); ` +
        `falling back to the raw 3-arg transfer(uint256,uint16,bytes32) ntt-lib.sh used`
    );
    return await evmSendRaw(cfg, eth, wallet, rawAmount, destChain, recipient32Hex, opts);
  }
}

/**
 * Redeems a VAA on Ethereum via a raw receiveMessage(bytes) call against the
 * transceiver, using the exact guardian-signed bytes (never re-serialized).
 */
export async function evmRedeem(
  _cfg: PrivateDeploymentsConfig,
  dep: DeploymentConfig,
  vaaBytes: Uint8Array,
  opts: EvmOpOptions = {}
): Promise<EvmRedeemResult | undefined> {
  const eth = dep.chains.Ethereum;
  const provider = buildEthProvider();
  const wallet = buildEthWallet(provider);
  const xcvr = new Contract(eth.transceiver, ["function receiveMessage(bytes)"], wallet);
  const hex = "0x" + Buffer.from(vaaBytes).toString("hex");

  const tx = await xcvr.receiveMessage.populateTransaction(hex);
  const receipt = await sendOrSimulate(wallet, tx, "receiveMessage", opts.dryRun);
  if (opts.dryRun || !receipt) return undefined;
  return { txHash: receipt.hash };
}
