// Exact decimal-string <-> raw bigint conversion. No floats anywhere: a naive
// Number()*10**decimals introduces artifacts (e.g. 0.1 * 1e18 is not exactly
// 1e17) that would silently over/under-transfer on-chain.

const DECIMAL_RE = /^(-?)(\d+)(?:\.(\d+))?$/;

/** Converts a human decimal string to a raw bigint at the given decimals. */
export function toRaw(human: string, decimals: number): bigint {
  const trimmed = human.trim();
  const m = DECIMAL_RE.exec(trimmed);
  if (!m) throw new Error(`"${human}" is not a valid decimal amount`);
  const [, sign, intPart, fracPart = ""] = m;
  if (sign === "-") throw new Error(`amount must be non-negative, got "${human}"`);
  if (fracPart.length > decimals && /[^0]/.test(fracPart.slice(decimals))) {
    throw new Error(`"${human}" has more precision than ${decimals} decimals`);
  }
  const fracPadded = fracPart.slice(0, decimals).padEnd(decimals, "0");
  const digits = `${intPart}${fracPadded}`.replace(/^0+(?=\d)/, "");
  return BigInt(digits.length === 0 ? "0" : digits);
}

/** Converts a raw bigint at the given decimals back to a human decimal string. */
export function fromRaw(raw: bigint, decimals: number): string {
  if (raw < 0n) throw new Error("raw amount must be non-negative");
  const s = raw.toString().padStart(decimals + 1, "0");
  const cut = s.length - decimals;
  const intPart = s.slice(0, cut) || "0";
  const fracPart = decimals === 0 ? "" : s.slice(cut).replace(/0+$/, "");
  return fracPart ? `${intPart}.${fracPart}` : intPart;
}

/** Smallest representable step, as a decimal string, for `n` decimal places. */
function stepString(step: number): string {
  return step === 0 ? "1" : `0.${"0".repeat(step - 1)}1`;
}

/**
 * Rejects amounts that would round/truncate on the destination chain. The
 * dust threshold is min(8, srcDecimals, dstDecimals) — same rule ntt-lib.sh
 * enforces client-side rather than letting an on-chain assert fail later.
 */
export function checkDust(
  human: string,
  srcDecimals: number,
  dstDecimals: number
): void {
  const step = Math.min(8, srcDecimals, dstDecimals);
  const raw = toRaw(human, srcDecimals);
  const scale = srcDecimals - step;
  const divisor = 10n ** BigInt(scale);
  if (raw % divisor !== 0n) {
    throw new Error(
      `amount must be a multiple of ${stepString(step)} for this route`
    );
  }
}
