// Exact decimal-string <-> raw bigint conversion with integer arithmetic only.
import { parseRawAmount, type RawAmount, type TokenDecimals } from "./types.js";

const HUMAN_AMOUNT_PATTERN = /^(\d{1,78})(?:\.(\d{1,78}))?$/;

/** Human decimal string -> positive raw amount at the given decimals. */
export function parseHumanAmount(
  human: string,
  decimals: TokenDecimals
): RawAmount {
  const match = HUMAN_AMOUNT_PATTERN.exec(human.trim());
  if (!match)
    throw new Error(`"${human}" is not a non-negative decimal amount`);
  const [, integerDigits, fractionDigits = ""] = match;
  if (
    fractionDigits.length > decimals &&
    /[^0]/.test(fractionDigits.slice(decimals))
  ) {
    throw new Error(`"${human}" has more precision than ${decimals} decimals`);
  }
  const scaledFraction = fractionDigits
    .slice(0, decimals)
    .padEnd(decimals, "0");
  return parseRawAmount(BigInt(`${integerDigits}${scaledFraction}`));
}

/** Raw amount at the given decimals -> human decimal string. */
export function formatRawAmount(raw: bigint, decimals: number): string {
  if (raw < 0n) throw new Error("raw amount must be non-negative");
  const digits = raw.toString().padStart(decimals + 1, "0");
  const integerLength = digits.length - decimals;
  const integerDigits = digits.slice(0, integerLength);
  const fractionDigits =
    decimals === 0 ? "" : digits.slice(integerLength).replace(/0+$/, "");
  return fractionDigits ? `${integerDigits}.${fractionDigits}` : integerDigits;
}

const NTT_WIRE_DECIMALS_MAX = 8;

function smallestStepText(stepDecimals: number): string {
  return stepDecimals === 0 ? "1" : `0.${"0".repeat(stepDecimals - 1)}1`;
}

/**
 * Rejects an amount that the NTT trim would truncate. The step is
 * min(8, source decimals, destination decimals), as in the NTT managers.
 */
export function checkAmountHasNoDust(
  rawAmount: RawAmount,
  sourceDecimals: TokenDecimals,
  destinationDecimals: TokenDecimals
): void {
  const stepDecimals = Math.min(
    NTT_WIRE_DECIMALS_MAX,
    sourceDecimals,
    destinationDecimals
  );
  const divisor = 10n ** BigInt(sourceDecimals - stepDecimals);
  if (rawAmount % divisor !== 0n) {
    throw new Error(
      `amount must be a multiple of ${smallestStepText(stepDecimals)} for this route`
    );
  }
}
