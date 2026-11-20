// Ethereum NTT send and redeem over an injected signer. One path per operation, chosen before
// any broadcast; a broadcast whose receipt is not confirmed ends the operation.
import { AbiCoder, Interface, id } from "ethers";

import type {
  ChainName,
  Deployment,
  NetworkName,
  PrivateRegistry,
} from "./config.js";
import { chainLegIdentity } from "./transfer-route.js";
import { bytesToHex } from "./binding.js";
import {
  parseHex32,
  type EvmAddress,
  type Hex32,
  type RawAmount,
} from "./types.js";

export const EVM_RECEIPT_CONFIRMATIONS = 1;
export const EVM_RECEIPT_WAIT_TIMEOUT_MS = 180_000;

const ETHEREUM_CHAIN_ID_BY_NETWORK: Record<NetworkName, bigint> = {
  Mainnet: 1n,
  Testnet: 11155111n,
};

// Encoded empty TransceiverInstructions; the 3-argument transfer passes new bytes(1).
const EMPTY_TRANSCEIVER_INSTRUCTIONS = "0x00";
const LOG_MESSAGE_PUBLISHED_TOPIC = id(
  "LogMessagePublished(address,uint64,uint32,bytes,uint8)"
);

const managerInterface = new Interface([
  "function getPeer(uint16) view returns (tuple(bytes32 peerAddress, uint8 tokenDecimals))",
  "function quoteDeliveryPrice(uint16, bytes) view returns (uint256[], uint256)",
  "function transfer(uint256, uint16, bytes32) payable returns (uint64)",
]);
const tokenInterface = new Interface([
  "function allowance(address, address) view returns (uint256)",
  "function approve(address, uint256) returns (bool)",
]);
const tokenReadInterface = new Interface([
  "function decimals() view returns (uint8)",
]);
const managerTokenInterface = new Interface([
  "function token() view returns (address)",
]);
const transceiverInterface = new Interface(["function receiveMessage(bytes)"]);

export interface EthereumTransactionRequest {
  to: string;
  data: string;
  value: bigint;
}

export interface EthereumLogSummary {
  address: string;
  topics: readonly string[];
  data: string;
}

export interface EthereumReceiptSummary {
  hash: string;
  status: number;
  logs: readonly EthereumLogSummary[];
}

export interface SignedEthereumTransaction {
  signedTransactionHex: string;
  hash: string;
}

export interface EthereumSigner {
  getAddress(): Promise<string>;
  getChainId(): Promise<bigint>;
  call(request: EthereumTransactionRequest): Promise<string>;
  estimateGas(request: EthereumTransactionRequest): Promise<bigint>;
  signTransaction(
    request: EthereumTransactionRequest
  ): Promise<SignedEthereumTransaction>;
  broadcastSignedTransaction(signed: SignedEthereumTransaction): Promise<void>;
  waitForReceipt(
    hash: string,
    confirmations: number,
    timeoutMs: number
  ): Promise<EthereumReceiptSummary | null>;
}

export class TransferBroadcastUnconfirmedError extends Error {
  constructor(
    readonly description: string,
    readonly txHash: string,
    cause: Error
  ) {
    super(
      `${description} was broadcast as ${txHash} but its receipt was not confirmed (${cause.message}); ` +
        `do not re-run; check the hash on an explorer`
    );
    this.name = "TransferBroadcastUnconfirmedError";
  }
}

type BroadcastOutcome =
  | { kind: "confirmed"; receipt: EthereumReceiptSummary }
  | { kind: "reverted"; txHash: string }
  | { kind: "broadcastUnconfirmed"; txHash: string; cause: Error };

/** Signs, prints the hash, then broadcasts. Every error after the signature is "unconfirmed". */
async function broadcastAndWait(
  signer: EthereumSigner,
  request: EthereumTransactionRequest,
  description: string
): Promise<BroadcastOutcome> {
  const signed = await signer.signTransaction(request);
  console.log(`  ${description}: tx ${signed.hash}`);
  try {
    await signer.broadcastSignedTransaction(signed);
    const receipt = await signer.waitForReceipt(
      signed.hash,
      EVM_RECEIPT_CONFIRMATIONS,
      EVM_RECEIPT_WAIT_TIMEOUT_MS
    );
    if (!receipt) {
      return {
        kind: "broadcastUnconfirmed",
        txHash: signed.hash,
        cause: new Error("no receipt"),
      };
    }
    if (receipt.status !== 1) return { kind: "reverted", txHash: signed.hash };
    return { kind: "confirmed", receipt };
  } catch (err) {
    return {
      kind: "broadcastUnconfirmed",
      txHash: signed.hash,
      cause: err as Error,
    };
  }
}

async function broadcastConfirmedOrThrow(
  signer: EthereumSigner,
  request: EthereumTransactionRequest,
  description: string
): Promise<EthereumReceiptSummary> {
  const outcome = await broadcastAndWait(signer, request, description);
  if (outcome.kind === "reverted") {
    throw new Error(`${description} ${outcome.txHash} reverted on chain`);
  }
  if (outcome.kind === "broadcastUnconfirmed") {
    throw new TransferBroadcastUnconfirmedError(
      description,
      outcome.txHash,
      outcome.cause
    );
  }
  return outcome.receipt;
}

async function simulate(
  signer: EthereumSigner,
  request: EthereumTransactionRequest,
  description: string
) {
  try {
    await signer.call(request);
  } catch (err) {
    throw new Error(
      `[dry-run] ${description} would revert: ${(err as Error).message}`
    );
  }
  console.log(
    `  [dry-run] ${description}: ok, ~${await signer.estimateGas(request)} gas`
  );
}

export async function checkEthereumChainId(
  signer: EthereumSigner,
  network: NetworkName
): Promise<void> {
  const chainId = await signer.getChainId();
  const expectedChainId = ETHEREUM_CHAIN_ID_BY_NETWORK[network];
  if (chainId !== expectedChainId) {
    throw new Error(
      `Ethereum RPC chain id is ${chainId}; network ${network} needs chain id ${expectedChainId}`
    );
  }
}

export async function checkManagerPeerIsRegistryManager(
  signer: EthereumSigner,
  manager: EvmAddress,
  destinationChainId: number,
  expectedPeerManager: Hex32
): Promise<void> {
  const result = await signer.call({
    to: manager,
    data: managerInterface.encodeFunctionData("getPeer", [destinationChainId]),
    value: 0n,
  });
  const [peer] = managerInterface.decodeFunctionResult("getPeer", result);
  const peerManager = parseHex32(String(peer.peerAddress));
  if (peerManager !== expectedPeerManager) {
    throw new Error(
      `NttManager ${manager} peer for chain ${destinationChainId} is 0x${peerManager}; the registry manager is 0x${expectedPeerManager}`
    );
  }
}

/** The manager token and its decimals must equal the registry values. */
export async function checkTokenMatchesRegistry(
  signer: EthereumSigner,
  manager: EvmAddress,
  token: EvmAddress,
  registryDecimals: number
): Promise<void> {
  const managerTokenResult = await signer.call({
    to: manager,
    data: managerTokenInterface.encodeFunctionData("token", []),
    value: 0n,
  });
  const [managerToken] = managerTokenInterface.decodeFunctionResult(
    "token",
    managerTokenResult
  );
  if (String(managerToken).toLowerCase() !== token.toLowerCase()) {
    throw new Error(
      `NttManager ${manager} token is ${managerToken}; the registry token is ${token}`
    );
  }
  const decimalsResult = await signer.call({
    to: token,
    data: tokenReadInterface.encodeFunctionData("decimals", []),
    value: 0n,
  });
  const [onChainDecimals] = tokenReadInterface.decodeFunctionResult(
    "decimals",
    decimalsResult
  );
  if (Number(onChainDecimals) !== registryDecimals) {
    throw new Error(
      `token ${token} has ${onChainDecimals} decimals; the registry says ${registryDecimals}`
    );
  }
}

async function quoteDeliveryPriceWei(
  signer: EthereumSigner,
  manager: EvmAddress,
  destinationChainId: number
) {
  try {
    const result = await signer.call({
      to: manager,
      data: managerInterface.encodeFunctionData("quoteDeliveryPrice", [
        destinationChainId,
        EMPTY_TRANSCEIVER_INSTRUCTIONS,
      ]),
      value: 0n,
    });
    const [, totalPriceWei] = managerInterface.decodeFunctionResult(
      "quoteDeliveryPrice",
      result
    );
    return BigInt(totalPriceWei);
  } catch (err) {
    throw new Error(`quoteDeliveryPrice failed: ${(err as Error).message}`);
  }
}

/** Wormhole core sequence from the LogMessagePublished event that the transceiver emitted. */
export function coreSequenceFromReceipt(
  receipt: EthereumReceiptSummary,
  transceiver: EvmAddress
): string | undefined {
  const transceiverTopic = AbiCoder.defaultAbiCoder()
    .encode(["address"], [transceiver])
    .toLowerCase();
  const matches = receipt.logs.filter(
    (log) =>
      log.topics[0]?.toLowerCase() === LOG_MESSAGE_PUBLISHED_TOPIC &&
      log.topics[1]?.toLowerCase() === transceiverTopic
  );
  if (matches.length !== 1) return undefined;
  const [sequence] = AbiCoder.defaultAbiCoder().decode(
    ["uint64", "uint32", "bytes", "uint8"],
    matches[0].data
  );
  return BigInt(sequence).toString();
}

export interface EvmSendParameters {
  registry: PrivateRegistry;
  deployment: Deployment;
  destinationChain: ChainName;
  rawAmount: RawAmount;
  recipient: Hex32;
}

export interface EvmSendResult {
  txHash: string;
  sequence?: string;
}

export async function evmSendWithSigner(
  signer: EthereumSigner,
  parameters: EvmSendParameters,
  options: { dryRun: boolean }
): Promise<EvmSendResult | undefined> {
  const ethereum = parameters.deployment.chains.Ethereum;
  if (!ethereum)
    throw new Error(
      `deployment "${parameters.deployment.name}" has no Ethereum leg`
    );
  const destinationChainId =
    parameters.registry.chainIds[parameters.destinationChain];
  const expectedPeerManager = chainLegIdentity(
    parameters.deployment,
    parameters.destinationChain
  ).manager;

  await checkEthereumChainId(signer, parameters.registry.network);
  await checkTokenMatchesRegistry(
    signer,
    ethereum.manager,
    ethereum.token,
    ethereum.decimals
  );
  await checkManagerPeerIsRegistryManager(
    signer,
    ethereum.manager,
    destinationChainId,
    expectedPeerManager
  );
  const deliveryPriceWei = await quoteDeliveryPriceWei(
    signer,
    ethereum.manager,
    destinationChainId
  );

  const sender = await signer.getAddress();
  const allowanceResult = await signer.call({
    to: ethereum.token,
    data: tokenInterface.encodeFunctionData("allowance", [
      sender,
      ethereum.manager,
    ]),
    value: 0n,
  });
  const [allowance] = tokenInterface.decodeFunctionResult(
    "allowance",
    allowanceResult
  );
  const approveRequest: EthereumTransactionRequest | undefined =
    BigInt(allowance) < parameters.rawAmount
      ? {
          to: ethereum.token,
          data: tokenInterface.encodeFunctionData("approve", [
            ethereum.manager,
            parameters.rawAmount,
          ]),
          value: 0n,
        }
      : undefined;
  const transferRequest: EthereumTransactionRequest = {
    to: ethereum.manager,
    data: managerInterface.encodeFunctionData("transfer", [
      parameters.rawAmount,
      destinationChainId,
      `0x${parameters.recipient}`,
    ]),
    value: deliveryPriceWei,
  };

  if (options.dryRun) {
    if (approveRequest) {
      await simulate(signer, approveRequest, "approve");
      console.log(
        "  [dry-run] transfer: not simulated, it needs the approve on chain first"
      );
    } else {
      await simulate(signer, transferRequest, "transfer");
    }
    return undefined;
  }

  if (approveRequest)
    await broadcastConfirmedOrThrow(signer, approveRequest, "approve");
  const receipt = await broadcastConfirmedOrThrow(
    signer,
    transferRequest,
    "transfer"
  );
  return {
    txHash: receipt.hash,
    sequence: coreSequenceFromReceipt(receipt, ethereum.transceiver),
  };
}

export async function evmRedeemWithSigner(
  signer: EthereumSigner,
  network: NetworkName,
  transceiver: EvmAddress,
  vaaBytes: Uint8Array,
  options: { dryRun: boolean }
): Promise<{ txHash: string } | undefined> {
  await checkEthereumChainId(signer, network);
  const request: EthereumTransactionRequest = {
    to: transceiver,
    data: transceiverInterface.encodeFunctionData("receiveMessage", [
      `0x${bytesToHex(vaaBytes)}`,
    ]),
    value: 0n,
  };
  if (options.dryRun) {
    await simulate(signer, request, "receiveMessage");
    return undefined;
  }
  const receipt = await broadcastConfirmedOrThrow(
    signer,
    request,
    "receiveMessage"
  );
  return { txHash: receipt.hash };
}
