// Checks a parsed transfer VAA against one registry deployment.
import {
  chainNameForId,
  type ChainName,
  type Deployment,
  type PrivateRegistry,
} from "./config.js";
import { evmAddressToHex32, type Hex32, type TokenDecimals } from "./types.js";
import type { ParsedTransferVaa } from "./vaa.js";

export interface ChainLegIdentity {
  manager: Hex32;
  emitter: Hex32;
  decimals: TokenDecimals;
}

export function chainLegIdentity(
  deployment: Deployment,
  chain: ChainName
): ChainLegIdentity {
  const { Canton, Ethereum, Solana } = deployment.chains;
  if (chain === "Canton" && Canton)
    return {
      manager: Canton.manager,
      emitter: Canton.emitter,
      decimals: Canton.decimals,
    };
  if (chain === "Ethereum" && Ethereum) {
    return {
      manager: evmAddressToHex32(Ethereum.manager),
      emitter: Ethereum.emitter,
      decimals: Ethereum.decimals,
    };
  }
  if (chain === "Solana" && Solana) {
    return {
      manager: Solana.managerHex32,
      emitter: Solana.emitter,
      decimals: Solana.decimals,
    };
  }
  throw new Error(`deployment "${deployment.name}" has no ${chain} leg`);
}

export function checkTransferVaaMatchesDeployment(
  parsed: ParsedTransferVaa,
  registry: PrivateRegistry,
  deployment: Deployment
): { sourceChain: ChainName; destinationChain: ChainName } {
  const sourceChain = chainNameForId(registry, parsed.emitterChainId);
  const destinationChain = chainNameForId(registry, parsed.recipientChainId);
  if (sourceChain === destinationChain) {
    throw new Error(`VAA source and destination chain are both ${sourceChain}`);
  }
  const source = chainLegIdentity(deployment, sourceChain);
  const destination = chainLegIdentity(deployment, destinationChain);
  if (parsed.emitterAddress !== source.emitter) {
    throw new Error(
      `VAA emitter 0x${parsed.emitterAddress} does not match deployment "${deployment.name}" ${sourceChain} emitter 0x${source.emitter}`
    );
  }
  if (parsed.sourceManager !== source.manager) {
    throw new Error(
      `VAA source manager 0x${parsed.sourceManager} does not match deployment "${deployment.name}" ${sourceChain} manager 0x${source.manager}`
    );
  }
  if (parsed.recipientManager !== destination.manager) {
    throw new Error(
      `VAA recipient manager 0x${parsed.recipientManager} does not match deployment "${deployment.name}" ${destinationChain} manager 0x${destination.manager}`
    );
  }
  return { sourceChain, destinationChain };
}
