// Canton recipient binding (Manager.daml recipientAddressFromText) and hex helpers.
import { keccak256 } from "ethers";

import { parseHex32, type CantonPartyId, type Hex32 } from "./types.js";

const RECIPIENT_BINDING_TAG = "wormhole:ntt-recipient:v1";
const LENGTH_PREFIX_BYTES = 4;

/**
 * The bytes32 that an outbound transfer must carry to reach a Canton party:
 * keccak256(tag ‖ u32be(len(party)) ‖ party). A wrong value strands the funds.
 */
export function recipientBinding(party: CantonPartyId): Hex32 {
  const tagBytes = new TextEncoder().encode(RECIPIENT_BINDING_TAG);
  const partyBytes = new TextEncoder().encode(party);
  const preimage = new Uint8Array(
    tagBytes.length + LENGTH_PREFIX_BYTES + partyBytes.length
  );
  preimage.set(tagBytes, 0);
  new DataView(preimage.buffer).setUint32(
    tagBytes.length,
    partyBytes.length,
    false
  );
  preimage.set(partyBytes, tagBytes.length + LENGTH_PREFIX_BYTES);
  return parseHex32(keccak256(preimage));
}

const EVEN_LENGTH_HEX_PATTERN = /^(?:[0-9a-fA-F]{2})*$/;

/** Hex text (optional 0x) -> bytes. */
export function hexToBytes(hexText: string): Uint8Array {
  const hex =
    hexText.startsWith("0x") || hexText.startsWith("0X")
      ? hexText.slice(2)
      : hexText;
  if (!EVEN_LENGTH_HEX_PATTERN.test(hex)) {
    throw new Error(
      `not an even-length hex string: "${hexText.slice(0, 24)}..."`
    );
  }
  return Uint8Array.from(Buffer.from(hex, "hex"));
}

export function bytesToHex(bytes: Uint8Array): string {
  return Buffer.from(bytes).toString("hex");
}
