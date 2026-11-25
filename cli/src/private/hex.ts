// Hex text <-> bytes for VAAs and EVM calldata.
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
