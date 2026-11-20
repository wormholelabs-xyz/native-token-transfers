// Checked by `tsc --noEmit` (typecheck:cli). Each @ts-expect-error line must stay a type error.
import type {
  CantonPartyId,
  Hex32,
  RawAmount,
  TokenDecimals,
  VaaDigestHex32,
} from "../types.js";

declare const plainText: string;
declare const rawAmount: RawAmount;
declare const hex32: Hex32;

// @ts-expect-error a string is not a CantonPartyId
export const partyFromText: CantonPartyId = plainText;
// @ts-expect-error a RawAmount is not TokenDecimals
export const decimalsFromAmount: TokenDecimals = rawAmount;
// @ts-expect-error a Hex32 is not a VaaDigestHex32
export const digestFromHex32: VaaDigestHex32 = hex32;
