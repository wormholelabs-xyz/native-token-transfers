// Solana key file parse, confirmation wait and checks, over injected RPC interfaces.
import type { Hex32 } from "./types.js";

export const SOLANA_SECRET_KEY_BYTES = 64;
export const SOLANA_CONFIRMATION_POLL_INTERVAL_MS = 2_000;
export const SOLANA_CONFIRMATION_WALL_CLOCK_CAP_MS = 180_000;

/** SOLANA_PAYER_PATH file text -> 64 secret key bytes. The error text holds a fixed message. */
export function parseSolanaKeypairFileText(fileText: string): Uint8Array {
  const formatError = new Error(
    `SOLANA_PAYER_PATH is not a JSON array of ${SOLANA_SECRET_KEY_BYTES} numbers from 0 to 255`
  );
  let keyValues: unknown;
  try {
    keyValues = JSON.parse(fileText);
  } catch {
    throw formatError;
  }
  if (!Array.isArray(keyValues) || keyValues.length !== SOLANA_SECRET_KEY_BYTES)
    throw formatError;
  for (const byteValue of keyValues) {
    if (
      typeof byteValue !== "number" ||
      !Number.isInteger(byteValue) ||
      byteValue < 0 ||
      byteValue > 255
    ) {
      throw formatError;
    }
  }
  return Uint8Array.from(keyValues as number[]);
}

export class SolanaTransactionExpiredError extends Error {
  constructor(signature: string, lastValidBlockHeight: number) {
    super(
      `${signature} expired: block height passed ${lastValidBlockHeight} before it confirmed; it cannot land, safe to retry`
    );
    this.name = "SolanaTransactionExpiredError";
  }
}

export class SolanaConfirmationUnknownError extends Error {
  constructor(signature: string, waitedMs: number, detailText?: string) {
    super(
      `${signature}: result unknown after ${waitedMs} ms${detailText ? ` (${detailText})` : ""}; ` +
        `do not re-run; check the signature on an explorer`
    );
    this.name = "SolanaConfirmationUnknownError";
  }
}

export interface SolanaStatusRpc {
  getSignatureConfirmationStatus(
    signature: string
  ): Promise<{ confirmationStatus: string; failed: boolean } | undefined>;
  getBlockHeight(): Promise<number>;
}

export interface SolanaConfirmationBounds {
  pollIntervalMs: number;
  wallClockCapMs: number;
  sleep: (durationMs: number) => Promise<void>;
}

export const DEFAULT_SOLANA_CONFIRMATION_BOUNDS: SolanaConfirmationBounds = {
  pollIntervalMs: SOLANA_CONFIRMATION_POLL_INTERVAL_MS,
  wallClockCapMs: SOLANA_CONFIRMATION_WALL_CLOCK_CAP_MS,
  sleep: (durationMs) =>
    new Promise((resolve) => setTimeout(resolve, durationMs)),
};

/** Ends on confirmation, on a block height past lastValidBlockHeight, or at the wall-clock cap. */
type SolanaPollOutcome =
  | "confirmed"
  | "failedOnChain"
  | "pending"
  | "expired"
  | "presentButUnconfirmedAfterLimit";

function isConfirmedStatus(
  status: { confirmationStatus: string } | undefined
): boolean {
  return (
    status?.confirmationStatus === "confirmed" ||
    status?.confirmationStatus === "finalized"
  );
}

async function readSolanaPollOutcome(
  rpc: SolanaStatusRpc,
  signature: string,
  lastValidBlockHeight: number
): Promise<SolanaPollOutcome> {
  const status = await rpc.getSignatureConfirmationStatus(signature);
  if (status?.failed) return "failedOnChain";
  if (isConfirmedStatus(status)) return "confirmed";
  if ((await rpc.getBlockHeight()) <= lastValidBlockHeight) return "pending";
  // The status and height reads can reach different nodes: read the status again.
  const recheckedStatus = await rpc.getSignatureConfirmationStatus(signature);
  if (recheckedStatus?.failed) return "failedOnChain";
  if (isConfirmedStatus(recheckedStatus)) return "confirmed";
  return recheckedStatus ? "presentButUnconfirmedAfterLimit" : "expired";
}

/**
 * Ends on confirmation, on a confirmed absence after lastValidBlockHeight, or at the wall-clock
 * cap. A read error continues the poll; a poll that ends without an answer is "unknown".
 */
export async function waitForSolanaConfirmation(
  rpc: SolanaStatusRpc,
  signature: string,
  lastValidBlockHeight: number,
  bounds: SolanaConfirmationBounds = DEFAULT_SOLANA_CONFIRMATION_BOUNDS
): Promise<void> {
  const startedMs = Date.now();
  const maxPolls = Math.max(
    1,
    Math.ceil(bounds.wallClockCapMs / Math.max(1, bounds.pollIntervalMs)) + 1
  );
  let lastReadErrorText: string | undefined;
  for (let pollNumber = 1; pollNumber <= maxPolls; pollNumber++) {
    let outcome: SolanaPollOutcome;
    try {
      outcome = await readSolanaPollOutcome(
        rpc,
        signature,
        lastValidBlockHeight
      );
    } catch (err) {
      lastReadErrorText = (err as Error).message;
      outcome = "pending";
    }
    if (outcome === "confirmed") return;
    if (outcome === "failedOnChain")
      throw new Error(`${signature} failed on chain`);
    if (outcome === "expired") {
      throw new SolanaTransactionExpiredError(signature, lastValidBlockHeight);
    }
    if (outcome === "presentButUnconfirmedAfterLimit") break;
    if (Date.now() - startedMs >= bounds.wallClockCapMs) break;
    await bounds.sleep(bounds.pollIntervalMs);
  }
  throw new SolanaConfirmationUnknownError(
    signature,
    Date.now() - startedMs,
    lastReadErrorText === undefined
      ? undefined
      : `last read error: ${lastReadErrorText}`
  );
}

/** Sends a signed transaction. An error after the signature leaves the result unknown. */
export async function sendSignedSolanaTransaction(
  rpc: SolanaStatusRpc,
  signature: string,
  lastValidBlockHeight: number,
  send: () => Promise<unknown>,
  bounds: SolanaConfirmationBounds = DEFAULT_SOLANA_CONFIRMATION_BOUNDS
): Promise<void> {
  try {
    await send();
  } catch (err) {
    throw new SolanaConfirmationUnknownError(
      signature,
      0,
      `send error: ${(err as Error).message}`
    );
  }
  await waitForSolanaConfirmation(rpc, signature, lastValidBlockHeight, bounds);
}

const KEY_FILE_GROUP_AND_OTHER_BITS = 0o077;

export function checkSolanaKeyFileMode(fileMode: number): void {
  if ((fileMode & KEY_FILE_GROUP_AND_OTHER_BITS) !== 0) {
    throw new Error(
      `SOLANA_PAYER_PATH has mode ${(fileMode & 0o777).toString(8)}; other users can read it. Run chmod 600 on it.`
    );
  }
}

export function checkRegistryDecimalsMatchChain(
  onChainDecimals: number,
  registryDecimals: number,
  context: string
): void {
  if (onChainDecimals !== registryDecimals) {
    throw new Error(
      `${context} has ${onChainDecimals} decimals; the registry says ${registryDecimals}`
    );
  }
}

export function checkSolanaPeerIsRegistryManager(
  observedPeer: Hex32,
  expectedPeer: Hex32,
  peerChain: string
): void {
  if (observedPeer !== expectedPeer) {
    throw new Error(
      `Solana NttManager peer for ${peerChain} is 0x${observedPeer}; the registry manager is 0x${expectedPeer}`
    );
  }
}

export interface SolanaRedeemSteps {
  checkRecipientTokenAccount(): Promise<void>;
  driveRedeem(dryRun: boolean): Promise<string | undefined>;
}

/** The recipient token account check is a read, so it runs in a dry run too. */
export async function runSolanaRedeemSteps(
  steps: SolanaRedeemSteps,
  options: { dryRun: boolean }
) {
  await steps.checkRecipientTokenAccount();
  return steps.driveRedeem(options.dryRun);
}
