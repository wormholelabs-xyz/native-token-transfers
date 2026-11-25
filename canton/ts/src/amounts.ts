// NTT amount trimming, as NttManager.Transfer does it (Wormhole.Ntt.Amount trimAmountTo,
// untrimAmount), so a transfer with dust is refused before anything is submitted.
import { NTT_WIRE_DECIMALS_MAX } from "./constants.js";

/** A wire amount: `amount` units at `decimals` places (at most 8) */
export interface CantonTrimmedAmount {
  amount: bigint;
  decimals: number;
}

function checkDecimals(decimals: number, what: string): void {
  if (!Number.isInteger(decimals) || decimals < 0 || decimals > 255)
    throw new Error(
      `${what} must be an integer from 0 to 255, got ${decimals}`
    );
}

/**
 * The wire form of `rawAmount` units at `fromDecimals`, trimmed to min(8, fromDecimals,
 * toDecimals) decimals; precision below that is dropped (Amount.daml trimAmountTo).
 */
export function trimAmount(
  rawAmount: bigint,
  fromDecimals: number,
  toDecimals: number
): CantonTrimmedAmount {
  checkDecimals(fromDecimals, "fromDecimals");
  checkDecimals(toDecimals, "toDecimals");
  if (rawAmount < 0n)
    throw new Error(`amount must not be negative, got ${rawAmount}`);
  const decimals = Math.min(NTT_WIRE_DECIMALS_MAX, fromDecimals, toDecimals);
  return {
    amount: rawAmount / 10n ** BigInt(fromDecimals - decimals),
    decimals,
  };
}

/** A wire amount rescaled to `toDecimals`, truncating (Amount.daml untrimAmount) */
export function untrimAmount(
  trimmed: CantonTrimmedAmount,
  toDecimals: number
): bigint {
  checkDecimals(toDecimals, "toDecimals");
  return toDecimals >= trimmed.decimals
    ? trimmed.amount * 10n ** BigInt(toDecimals - trimmed.decimals)
    : trimmed.amount / 10n ** BigInt(trimmed.decimals - toDecimals);
}

/**
 * The largest amount up to `rawAmount` that a transfer to a peer with `peerDecimals` carries
 * without dust: what Transfer would lock or burn if the caller dropped the dust.
 */
export function sendableAmount(
  rawAmount: bigint,
  tokenDecimals: number,
  peerDecimals: number
) {
  return untrimAmount(
    trimAmount(rawAmount, tokenDecimals, peerDecimals),
    tokenDecimals
  );
}

/**
 * The trimmed wire amount of a transfer. Throws, as NttManager.Transfer would, for an amount that
 * is zero after trimming or that has dust the peer cannot represent; the message names the
 * sendable amount.
 */
export function checkTransferAmount(
  rawAmount: bigint,
  tokenDecimals: number,
  peerDecimals: number
): CantonTrimmedAmount {
  if (rawAmount <= 0n)
    throw new Error(`The transfer amount must be positive, got ${rawAmount}`);
  const trimmed = trimAmount(rawAmount, tokenDecimals, peerDecimals);
  if (trimmed.amount === 0n)
    throw new Error(
      `The transfer amount ${rawAmount} (at ${tokenDecimals} decimals) is zero after trimming to ${trimmed.decimals} decimals`
    );
  const sendable = untrimAmount(trimmed, tokenDecimals);
  if (sendable !== rawAmount)
    throw new Error(
      `The transfer amount ${rawAmount} (at ${tokenDecimals} decimals) has dust the peer cannot represent: ` +
        `the step is 10^${tokenDecimals - trimmed.decimals} units; send ${sendable} instead`
    );
  return trimmed;
}
