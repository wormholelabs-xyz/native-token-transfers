// Parses + validates a pasted VAA hex against the private-deployment registry.
// Uses the Canton-aware hoisted SDK singleton — never import sdk-base /
// sdk-definitions / sdk-definitions-ntt by bare name here: those resolve to
// the nested Canton-less 2.x copies under cli/node_modules (pulled in by
// sdk-sui-ntt). Import "@wormhole-foundation/sdk-evm-ntt" for its side
// effect only: it registers the "Ntt" payload types into the hoisted
// sdk-definitions registry that "@wormhole-foundation/sdk" reads from.
import { deserialize } from "@wormhole-foundation/sdk";
import "@wormhole-foundation/sdk-evm-ntt";

import { base58ToHex32, hexToBytes, pad32, strip0x } from "./binding.js";
import type {
  ChainName,
  DeploymentChains,
  DeploymentName,
  PrivateDeploymentsConfig,
} from "./config.js";

export interface ParsedTransferVaa {
  /** Original guardian-signed bytes — never re-serialize for on-chain submission. */
  bytes: Uint8Array;
  /** Opaque deserialized VAA (an Ntt:WormholeTransfer attestation), for callers driving redeem. */
  vaa: unknown;
  sourceChain: string;
  destinationChain: string;
  sourceManager: string; // 32-byte hex, no 0x, lowercase
  recipientManager: string; // 32-byte hex, no 0x, lowercase
  recipientAddress: string; // 32-byte hex, no 0x, lowercase
  trimmedAmount: { amount: bigint; decimals: number };
  sequence: bigint;
}

/**
 * Parses a pasted VAA hex as an Ntt:WormholeTransfer payload. Throws a clear
 * error if the VAA is actually a governance message (e.g. a
 * TransceiverRegistration, as guardian sequence-1 messages on this network
 * are) rather than a transfer.
 */
export function parseTransferVaa(hex: string): ParsedTransferVaa {
  const bytes = hexToBytes(hex);

  let vaa: any;
  try {
    vaa = deserialize("Ntt:WormholeTransfer", bytes);
  } catch (transferErr) {
    let registration: any;
    try {
      registration = deserialize("Ntt:TransceiverRegistration", bytes);
    } catch {
      registration = undefined;
    }
    if (registration) {
      throw new Error(
        `this VAA is a Ntt:TransceiverRegistration governance message, not a transfer ` +
          `(emitted by ${String(registration.emitterChain)}, sequence ${registration.sequence}, ` +
          `registers ${String(registration.payload.chain)} transceiver ${registration.payload.transceiver.toString()})`
      );
    }
    throw new Error(
      `not a valid Ntt:WormholeTransfer VAA: ${(transferErr as Error).message}`
    );
  }

  const inner = vaa.payload.nttManagerPayload.payload;
  return {
    bytes,
    vaa,
    sourceChain: String(vaa.emitterChain),
    destinationChain: String(inner.recipientChain),
    sourceManager: strip0x(vaa.payload.sourceNttManager.toString()),
    recipientManager: strip0x(vaa.payload.recipientNttManager.toString()),
    recipientAddress: strip0x(inner.recipientAddress.toString()),
    trimmedAmount: {
      amount: inner.trimmedAmount.amount,
      decimals: inner.trimmedAmount.decimals,
    },
    sequence: vaa.sequence,
  };
}

function managerHex32(
  chains: DeploymentChains,
  chain: ChainName,
  deploymentName: string
): string {
  if (chain === "Canton") return chains.Canton.manager.toLowerCase();
  if (chain === "Ethereum") return pad32(chains.Ethereum.manager).toLowerCase();
  if (!chains.Solana) {
    throw new Error(`deployment "${deploymentName}" has no Solana leg`);
  }
  return base58ToHex32(chains.Solana.manager).toLowerCase();
}

function asChainName(raw: string, deploymentName: string): ChainName {
  if (raw === "Canton" || raw === "Ethereum" || raw === "Solana") return raw;
  throw new Error(
    `VAA references chain "${raw}", which deployment "${deploymentName}" does not support`
  );
}

/**
 * Validates a parsed transfer VAA against a deployment's registered
 * managers. Refuses same-chain VAAs and manager mismatches (i.e. the wrong
 * --deployment was passed).
 */
export function validateAgainstDeployment(
  parsed: ParsedTransferVaa,
  config: PrivateDeploymentsConfig,
  deploymentName: DeploymentName
): { sourceChain: ChainName; destinationChain: ChainName } {
  const dep = config.deployments[deploymentName];
  if (!dep) throw new Error(`unknown deployment "${deploymentName}"`);

  const sourceChain = asChainName(parsed.sourceChain, deploymentName);
  const destinationChain = asChainName(parsed.destinationChain, deploymentName);

  if (sourceChain === destinationChain) {
    throw new Error(
      `VAA source and destination chain are both "${sourceChain}"; refusing a same-chain transfer`
    );
  }

  const expectedSourceManager = managerHex32(dep.chains, sourceChain, deploymentName);
  if (parsed.sourceManager !== expectedSourceManager) {
    throw new Error(
      `VAA source manager 0x${parsed.sourceManager} does not match deployment "${deploymentName}" ` +
        `${sourceChain} manager 0x${expectedSourceManager}`
    );
  }

  const expectedRecipientManager = managerHex32(dep.chains, destinationChain, deploymentName);
  if (parsed.recipientManager !== expectedRecipientManager) {
    throw new Error(
      `VAA recipient manager 0x${parsed.recipientManager} does not match deployment "${deploymentName}" ` +
        `${destinationChain} manager 0x${expectedRecipientManager}`
    );
  }

  return { sourceChain, destinationChain };
}
