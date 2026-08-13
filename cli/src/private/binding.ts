// Canton recipient binding + address encoding helpers. Pure, no network/SDK deps.
import { keccak256 } from "ethers";

const RECIPIENT_BINDING_TAG = "wormhole:ntt-recipient:v1";

function utf8Bytes(s: string): Uint8Array {
  return new TextEncoder().encode(s);
}

function u32be(n: number): Uint8Array {
  const b = new Uint8Array(4);
  b[0] = (n >>> 24) & 0xff;
  b[1] = (n >>> 16) & 0xff;
  b[2] = (n >>> 8) & 0xff;
  b[3] = n & 0xff;
  return b;
}

/**
 * The bytes32 an outbound transfer must target to reach a Canton party.
 * binding = keccak256(tag ‖ u32be(len(party)) ‖ party); party is the full "hint::namespace" id.
 * A wrong value strands funds permanently — always compute, never accept raw bytes without a warning.
 */
export function recipientBinding(party: string): string {
  const tag = utf8Bytes(RECIPIENT_BINDING_TAG);
  const body = utf8Bytes(party);
  const buf = new Uint8Array(tag.length + 4 + body.length);
  buf.set(tag, 0);
  buf.set(u32be(body.length), tag.length);
  buf.set(body, tag.length + 4);
  return keccak256(buf);
}

/** Left-pads a 20-byte hex address to 32 bytes. Lowercase, no 0x prefix (matches ntt-lib.sh pad32). */
export function pad32(addr20hex: string): string {
  const hex =
    addr20hex.startsWith("0x") || addr20hex.startsWith("0X")
      ? addr20hex.slice(2)
      : addr20hex;
  if (!/^[0-9a-fA-F]{40}$/.test(hex)) {
    throw new Error(`pad32 expects a 20-byte hex address, got "${addr20hex}"`);
  }
  return `${"0".repeat(24)}${hex.toLowerCase()}`;
}

/** Strips an optional 0x/0X prefix and lowercases. */
export function strip0x(hex: string): string {
  return (hex.startsWith("0x") || hex.startsWith("0X") ? hex.slice(2) : hex).toLowerCase();
}

/** Parses a hex string (with or without 0x) into bytes. */
export function hexToBytes(hex: string): Uint8Array {
  const stripped = strip0x(hex);
  if (stripped.length % 2 !== 0 || !/^[0-9a-f]*$/.test(stripped)) {
    throw new Error(`not a valid hex string: "${hex.slice(0, 24)}..."`);
  }
  const bytes = new Uint8Array(stripped.length / 2);
  for (let i = 0; i < bytes.length; i++) {
    bytes[i] = parseInt(stripped.slice(i * 2, i * 2 + 2), 16);
  }
  return bytes;
}

const BASE58_ALPHABET =
  "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz";

/** Decodes a base58 pubkey (e.g. a Solana address) to 32-byte hex. Lowercase, no 0x prefix. */
export function base58ToHex32(b58: string): string {
  if (b58.length === 0) throw new Error("base58ToHex32: empty input");
  let n = 0n;
  for (const c of b58) {
    const idx = BASE58_ALPHABET.indexOf(c);
    if (idx === -1) {
      throw new Error(`invalid base58 character "${c}" in "${b58}"`);
    }
    n = n * 58n + BigInt(idx);
  }
  const hex = n.toString(16);
  if (hex.length > 64) {
    throw new Error(`base58 value "${b58}" does not fit in 32 bytes`);
  }
  return hex.padStart(64, "0");
}
