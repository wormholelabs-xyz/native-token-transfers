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
  constructor(signature: string, waitedMs: number, sendErrorText?: string) {
    super(
      `${signature}: result unknown${sendErrorText ? ` (send error: ${sendErrorText})` : ` after ${waitedMs} ms`}; ` +
        `check the signature on an explorer before a retry`
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
  for (let pollNumber = 1; pollNumber <= maxPolls; pollNumber++) {
    const status = await rpc.getSignatureConfirmationStatus(signature);
    if (status?.failed) throw new Error(`${signature} failed on chain`);
    if (
      status?.confirmationStatus === "confirmed" ||
      status?.confirmationStatus === "finalized"
    )
      return;
    if ((await rpc.getBlockHeight()) > lastValidBlockHeight) {
      // The status and height reads can reach different nodes: read the status again.
      const recheckedStatus =
        await rpc.getSignatureConfirmationStatus(signature);
      if (recheckedStatus?.failed)
        throw new Error(`${signature} failed on chain`);
      if (
        recheckedStatus?.confirmationStatus === "confirmed" ||
        recheckedStatus?.confirmationStatus === "finalized"
      )
        return;
      if (recheckedStatus) {
        throw new SolanaConfirmationUnknownError(
          signature,
          Date.now() - startedMs
        );
      }
      throw new SolanaTransactionExpiredError(signature, lastValidBlockHeight);
    }
    if (Date.now() - startedMs >= bounds.wallClockCapMs) break;
    await bounds.sleep(bounds.pollIntervalMs);
  }
  throw new SolanaConfirmationUnknownError(signature, Date.now() - startedMs);
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
      (err as Error).message
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
