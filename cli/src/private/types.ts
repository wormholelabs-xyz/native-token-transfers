// Branded domain types. Each type has exactly one parser; downstream code takes the type.
import { encoding } from "@wormhole-foundation/sdk-base";
import { getAddress } from "ethers";

declare const brand: unique symbol;
type Branded<Base, Name extends string> = Base & { readonly [brand]: Name };

export type Hex32 = Branded<string, "Hex32">;
export type CantonPartyId = Branded<string, "CantonPartyId">;
export type CantonContractId = Branded<string, "CantonContractId">;
export type CantonPackageId = Branded<string, "CantonPackageId">;
export type EvmAddress = Branded<string, "EvmAddress">;
export type RawAmount = Branded<bigint, "RawAmount">;
export type TokenDecimals = Branded<number, "TokenDecimals">;
export type WormholeChainId = Branded<number, "WormholeChainId">;
export type VaaDigestHex32 = Branded<string, "VaaDigestHex32">;
export type DeploymentName = Branded<string, "DeploymentName">;
export type TransferNonceU32 = Branded<number, "TransferNonceU32">;

const HEX32_PATTERN = /^[0-9a-f]{64}$/;

/** 32 bytes as 64 lowercase hex characters, no 0x. An optional 0x prefix is stripped. */
export function parseHex32(text: string): Hex32 {
  const hex = text.startsWith("0x") ? text.slice(2) : text;
  if (!HEX32_PATTERN.test(hex)) {
    throw new Error(
      `expected 32 bytes as 64 lowercase hex characters, got "${text}"`
    );
  }
  return hex as Hex32;
}

export function hex32FromBytes(bytes: Uint8Array): Hex32 {
  if (bytes.length !== 32) {
    throw new Error(`expected exactly 32 bytes, got ${bytes.length}`);
  }
  return parseHex32(Buffer.from(bytes).toString("hex"));
}

export function hex32ToBytes(value: Hex32): Uint8Array {
  const bytes = Uint8Array.from(Buffer.from(value, "hex"));
  if (bytes.length !== 32)
    throw new Error("Hex32 decoded to a length other than 32");
  return bytes;
}

// Canton party id: "<hint>::1220<64 lowercase hex>". The hint rule is Canton's
// (UniqueIdentifier.verifyValidString): 1 to 185 characters of [A-Za-z0-9 :_-], split at the
// first "::". Same rule as canton/disclosure/service/party_id.go.
const CANTON_PARTY_SEPARATOR = "::";
const CANTON_PARTY_HINT_MAX_LENGTH = 185;
const CANTON_PARTY_HINT_PATTERN = /^[A-Za-z0-9 :_-]+$/;
const CANTON_NAMESPACE_FINGERPRINT_PATTERN = /^1220[0-9a-f]{64}$/;
export const CANTON_PARTY_ID_MAX_LENGTH = 255;

export function parseCantonPartyId(text: string): CantonPartyId {
  const separatorIndex = text.indexOf(CANTON_PARTY_SEPARATOR);
  if (separatorIndex < 0) {
    throw new Error(
      `Canton party id must be "<hint>::1220<64 lowercase hex>", got "${text}" (no "::")`
    );
  }
  const hint = text.slice(0, separatorIndex);
  const fingerprint = text.slice(
    separatorIndex + CANTON_PARTY_SEPARATOR.length
  );
  if (hint.length < 1 || hint.length > CANTON_PARTY_HINT_MAX_LENGTH) {
    throw new Error(
      `Canton party hint has ${hint.length} characters; it must have 1 to ${CANTON_PARTY_HINT_MAX_LENGTH}`
    );
  }
  if (!CANTON_PARTY_HINT_PATTERN.test(hint)) {
    throw new Error(
      `Canton party hint "${hint}" has a character outside [A-Za-z0-9 :_-]`
    );
  }
  if (hint.endsWith(":")) {
    throw new Error(`Canton party hint "${hint}" ends with ":"`);
  }
  if (!CANTON_NAMESPACE_FINGERPRINT_PATTERN.test(fingerprint)) {
    throw new Error(
      `Canton party namespace must be exactly "1220" + 64 lowercase hex, got "${fingerprint}"`
    );
  }
  if (text.length > CANTON_PARTY_ID_MAX_LENGTH) {
    throw new Error(
      `Canton party id exceeds ${CANTON_PARTY_ID_MAX_LENGTH} characters`
    );
  }
  return text as CantonPartyId;
}

// Contract id version prefix "00" (V1) or "01" (V2), then lowercase hex.
const CANTON_CONTRACT_ID_PATTERN = /^0[01][0-9a-f]{64,}$/;
const CANTON_CONTRACT_ID_MAX_LENGTH = 512;

export function parseCantonContractId(text: string): CantonContractId {
  if (
    text.length > CANTON_CONTRACT_ID_MAX_LENGTH ||
    !CANTON_CONTRACT_ID_PATTERN.test(text)
  ) {
    throw new Error(`not a Canton contract id: "${text.slice(0, 80)}"`);
  }
  return text as CantonContractId;
}

const CANTON_PACKAGE_ID_PATTERN = /^[0-9a-f]{64}$/;

export function parseCantonPackageId(text: string): CantonPackageId {
  if (!CANTON_PACKAGE_ID_PATTERN.test(text)) {
    throw new Error(
      `Canton package id must be 64 lowercase hex characters, got "${text}"`
    );
  }
  return text as CantonPackageId;
}

/** Package id part of a "<package id>:<module>:<entity>" template id. */
export function templatePackageId(templateId: string): CantonPackageId {
  const parts = templateId.split(":");
  if (parts.length !== 3) {
    throw new Error(
      `template id must be "<package id>:<module>:<entity>", got "${templateId}"`
    );
  }
  return parseCantonPackageId(parts[0]);
}

/** "<module>:<entity>" part of a template id. */
export function templateModuleAndEntity(templateId: string): string {
  const parts = templateId.split(":");
  if (parts.length !== 3) {
    throw new Error(
      `template id must be "<package id>:<module>:<entity>", got "${templateId}"`
    );
  }
  return `${parts[1]}:${parts[2]}`;
}

/**
 * EIP-55 address. A mixed-case value must have a valid checksum. An all-lowercase value is
 * accepted only with acceptUnchecksummed.
 */
export function parseEvmAddress(
  text: string,
  options: { acceptUnchecksummed: boolean } = { acceptUnchecksummed: false }
): EvmAddress {
  if (!/^0x[0-9a-fA-F]{40}$/.test(text)) {
    throw new Error(
      `EVM address must be 0x + 40 hex characters, got "${text}"`
    );
  }
  const body = text.slice(2);
  const isAllLowercase = body === body.toLowerCase();
  if (isAllLowercase && !options.acceptUnchecksummed) {
    throw new Error(
      `EVM address "${text}" has no EIP-55 checksum; use the checksummed form or pass --accept-unchecksummed-address`
    );
  }
  let checksummed: string;
  try {
    checksummed = getAddress(text);
  } catch {
    throw new Error(`EVM address "${text}" has a bad EIP-55 checksum`);
  }
  if (!isAllLowercase && checksummed !== text) {
    throw new Error(`EVM address "${text}" has a bad EIP-55 checksum`);
  }
  return checksummed as EvmAddress;
}

export function evmAddressToHex32(address: EvmAddress): Hex32 {
  return parseHex32("0".repeat(24) + address.slice(2).toLowerCase());
}

export const SOLANA_ADDRESS_BYTE_LENGTH = 32;

/** A Solana address (base58) that decodes to exactly 32 bytes, as Hex32. */
export function parseSolanaAddressBytes32(base58Text: string): Hex32 {
  if (base58Text.length === 0) throw new Error("Solana address is empty");
  let addressBytes: Uint8Array;
  try {
    addressBytes = encoding.b58.decode(base58Text);
  } catch {
    throw new Error(`Solana address "${base58Text}" is not valid base58`);
  }
  if (addressBytes.length !== SOLANA_ADDRESS_BYTE_LENGTH) {
    throw new Error(
      `Solana address must decode to exactly ${SOLANA_ADDRESS_BYTE_LENGTH} bytes, got ${addressBytes.length}`
    );
  }
  return hex32FromBytes(addressBytes);
}

export const TOKEN_DECIMALS_MIN = 0;
export const TOKEN_DECIMALS_MAX = 18;

export function parseTokenDecimals(value: unknown): TokenDecimals {
  if (typeof value !== "number" || !Number.isInteger(value)) {
    throw new Error(
      `token decimals must be an integer, got ${JSON.stringify(value)}`
    );
  }
  if (value < TOKEN_DECIMALS_MIN)
    throw new Error(`token decimals ${value} is below ${TOKEN_DECIMALS_MIN}`);
  if (value > TOKEN_DECIMALS_MAX)
    throw new Error(`token decimals ${value} is above ${TOKEN_DECIMALS_MAX}`);
  return value as TokenDecimals;
}

export const WORMHOLE_CHAIN_ID_MIN = 1;
export const WORMHOLE_CHAIN_ID_MAX = 65535;

export function parseWormholeChainId(value: unknown): WormholeChainId {
  if (typeof value !== "number" || !Number.isInteger(value)) {
    throw new Error(
      `Wormhole chain id must be an integer, got ${JSON.stringify(value)}`
    );
  }
  if (value < WORMHOLE_CHAIN_ID_MIN)
    throw new Error(
      `Wormhole chain id ${value} is below ${WORMHOLE_CHAIN_ID_MIN}`
    );
  if (value > WORMHOLE_CHAIN_ID_MAX)
    throw new Error(
      `Wormhole chain id ${value} is above ${WORMHOLE_CHAIN_ID_MAX}`
    );
  return value as WormholeChainId;
}

export function parseVaaDigestHex32(text: string): VaaDigestHex32 {
  return parseHex32(text) as string as VaaDigestHex32;
}

const DEPLOYMENT_NAME_PATTERN = /^[a-z0-9-]{1,32}$/;

export function parseDeploymentName(text: string): DeploymentName {
  if (!DEPLOYMENT_NAME_PATTERN.test(text)) {
    throw new Error(
      `deployment name must match ${DEPLOYMENT_NAME_PATTERN}, got "${text}"`
    );
  }
  return text as DeploymentName;
}

export const TRANSFER_NONCE_U32_MAX = 0xffffffff;

export function parseTransferNonceU32(value: unknown): TransferNonceU32 {
  const nonce =
    typeof value === "string" && /^[0-9]{1,10}$/.test(value)
      ? Number(value)
      : value;
  if (typeof nonce !== "number" || !Number.isInteger(nonce)) {
    throw new Error(
      `transfer nonce must be an integer, got ${JSON.stringify(value)}`
    );
  }
  if (nonce < 0) throw new Error(`transfer nonce ${nonce} is below 0`);
  if (nonce > TRANSFER_NONCE_U32_MAX)
    throw new Error(
      `transfer nonce ${nonce} is above ${TRANSFER_NONCE_U32_MAX}`
    );
  return nonce as TransferNonceU32;
}

/** A positive raw amount. Zero is rejected: every chain rejects a zero transfer. */
export function parseRawAmount(value: bigint): RawAmount {
  if (value <= 0n)
    throw new Error(`amount must be greater than zero, got ${value}`);
  if (value >= 2n ** 256n)
    throw new Error(`amount ${value} does not fit in 256 bits`);
  return value as RawAmount;
}
