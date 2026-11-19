// Drives Solana sends/redeems directly against @solana/web3.js + SolanaNtt
// (the vendored solana/ts SDK, whose AsyncGenerators natively produce
// @solana/web3.js Transaction/VersionedTransaction objects — there is no kit
// equivalent for this path). Never import sdk-base / sdk-definitions /
// sdk-connect / sdk-definitions-ntt by bare name: those resolve to the
// nested Canton-less 2.x copies under cli/node_modules.
import {
  Connection,
  Keypair,
  PublicKey,
  VersionedTransaction,
  type Signer,
  type SimulatedTransactionResponse,
} from "@solana/web3.js";
import * as splToken from "@solana/spl-token";
import * as fs from "node:fs";

import { encoding, toUniversal, type Network } from "@wormhole-foundation/sdk";
import { SolanaAddress, isVersionedTransaction } from "@wormhole-foundation/sdk-solana";
import { SolanaNtt } from "@wormhole-foundation/sdk-solana-ntt";

import { toRaw } from "./amounts.js";
import { hexToBytes } from "./binding.js";
import type {
  ChainName,
  DeploymentConfig,
  PrivateDeploymentsConfig,
  SolanaChainConfig,
} from "./config.js";

const DEFAULT_SOL_RPC = "https://rpc.labsapis.com/mainnet/solana";
const PROXY_ORIGIN = "https://portalbridge.com";

// Private-network Solana core bridge, fixed for every deployment (not part
// of the per-deployment registry — see port-ntt-lib-to-cli-data.md).
const SOLANA_CORE_BRIDGE = "worMq7dr1fGM24dUV2ciwMShz3p979L8LNcat9AhKsj";

const SEQ_LOG_PREFIX = "Program log: Sequence: ";

// Descriptions (from solana/ts/sdk/ntt.ts) whose created account another
// yielded tx in the same generator depends on. Cross-RPC-node replica lag
// means "confirmed" on our own send doesn't guarantee visibility to whatever
// node handles the next sendTransaction call, so we wait for "finalized"
// before proceeding past these.
const NEEDS_FINALITY = new Set(["Core.VerifySignature", "Core.PostVAA", "Redeem.CreateATA"]);

export interface SolanaOpOptions {
  dryRun?: boolean;
}

export interface SolanaSendResult {
  signature: string;
  sequence?: string;
}

export interface SolanaRedeemResult {
  signature?: string;
}

function buildSolanaConnection(): Connection {
  const url = process.env.SOL_RPC ?? DEFAULT_SOL_RPC;
  return new Connection(url, { commitment: "confirmed", httpHeaders: { Origin: PROXY_ORIGIN } });
}

/** Keypair from SOLANA_PRIVATE_KEY (base58 secret key) or SOLANA_PAYER_PATH (JSON keypair file). */
function loadSolanaKeypair(): Keypair {
  const payerPath = process.env.SOLANA_PAYER_PATH;
  if (payerPath) {
    const raw = JSON.parse(fs.readFileSync(payerPath, "utf8"));
    return Keypair.fromSecretKey(new Uint8Array(raw));
  }
  const pk = process.env.SOLANA_PRIVATE_KEY;
  if (!pk) {
    throw new Error("SOLANA_PRIVATE_KEY or SOLANA_PAYER_PATH env var not set");
  }
  return Keypair.fromSecretKey(encoding.b58.decode(pk));
}

/** Payer pubkey (base58), pure aside from a local keypair load — no RPC calls. */
export function deriveSolanaAddress(): string {
  return loadSolanaKeypair().publicKey.toBase58();
}

function buildSolanaNtt(
  cfg: PrivateDeploymentsConfig,
  connection: Connection,
  sol: SolanaChainConfig
): SolanaNtt<Network, "Solana"> {
  return new SolanaNtt(
    cfg.network as Network,
    "Solana",
    connection,
    {
      coreBridge: SOLANA_CORE_BRIDGE,
      ntt: {
        manager: sol.manager,
        token: sol.mint,
        transceiver: { wormhole: sol.transceiver },
      },
    },
    sol.idlVersion
  );
}

function logSimulation(description: string, sim: SimulatedTransactionResponse): void {
  if (sim.err) {
    console.log(`  [dry-run] ${description}: ERROR ${JSON.stringify(sim.err)}`);
  } else {
    console.log(`  [dry-run] ${description}: ok, ~${sim.unitsConsumed ?? "?"} CU`);
  }
  for (const log of sim.logs ?? []) console.log(`    ${log}`);
}

/** Signs and sends (or simulates, in dry-run mode) one yielded unsigned tx. */
async function driveTx(
  connection: Connection,
  payer: Keypair,
  utx: { transaction: { transaction: any; signers?: Signer[] }; description: string },
  dryRun: boolean | undefined
): Promise<string | undefined> {
  const { transaction, signers } = utx.transaction;
  const { blockhash, lastValidBlockHeight } = await connection.getLatestBlockhash();

  if (isVersionedTransaction(transaction)) {
    transaction.message.recentBlockhash = blockhash;
    transaction.sign([payer, ...(signers ?? [])]);
    if (dryRun) {
      const sim = await connection.simulateTransaction(transaction);
      logSimulation(utx.description, sim.value);
      return undefined;
    }
    const sig = await connection.sendTransaction(transaction);
    await confirmByStatusPoll(connection, sig);
    console.log(`  ${utx.description}: tx ${sig}`);
    return sig;
  }

  transaction.recentBlockhash = blockhash;
  transaction.lastValidBlockHeight = lastValidBlockHeight;
  transaction.feePayer = transaction.feePayer ?? payer.publicKey;

  if (dryRun) {
    // Simulate via the non-deprecated VersionedTransaction overload rather
    // than simulateTransaction(Transaction, ...), which web3.js deprecates.
    const versioned = new VersionedTransaction(transaction.compileMessage());
    const sim = await connection.simulateTransaction(versioned, { sigVerify: false });
    logSimulation(utx.description, sim.value);
    return undefined;
  }

  transaction.partialSign(payer, ...(signers ?? []));
  const sig = await connection.sendRawTransaction(transaction.serialize());
  await confirmByStatusPoll(connection, sig);
  console.log(`  ${utx.description}: tx ${sig}`);
  return sig;
}

/**
 * connection.confirmTransaction's block-height strategy misses statuses behind
 * the load-balanced proxy (txs land but the wait expires). Poll
 * getSignatureStatus with history search instead.
 */
async function confirmByStatusPoll(
  connection: Connection,
  signature: string,
  retries = 45,
  delayMs = 2000
): Promise<void> {
  for (let i = 0; i < retries; i++) {
    const status = await connection.getSignatureStatus(signature, {
      searchTransactionHistory: true,
    });
    const v = status.value;
    if (v?.err) throw new Error(`${signature} failed on-chain: ${JSON.stringify(v.err)}`);
    if (v?.confirmationStatus === "confirmed" || v?.confirmationStatus === "finalized") return;
    await new Promise((r) => setTimeout(r, delayMs));
  }
  throw new Error(`${signature} not confirmed after ${(retries * delayMs) / 1000}s`);
}

/** Bounded poll for a signature to reach "finalized" — the replica-lag guard. */
async function waitFinalized(
  connection: Connection,
  signature: string,
  retries = 20,
  delayMs = 1000
): Promise<void> {
  for (let i = 0; i < retries; i++) {
    const status = await connection.getSignatureStatus(signature, {
      searchTransactionHistory: true,
    });
    if (status.value?.confirmationStatus === "finalized") return;
    await new Promise((r) => setTimeout(r, delayMs));
  }
  throw new Error(
    `timed out waiting for ${signature} to reach finalized commitment (replica lag?) — ` +
      `safe to re-run: posted-VAA and redeem are idempotent`
  );
}

async function driveGenerator(
  connection: Connection,
  payer: Keypair,
  generator: AsyncGenerator<any>,
  dryRun: boolean | undefined
): Promise<string | undefined> {
  let lastSig: string | undefined;
  for await (const utx of generator) {
    const sig = await driveTx(connection, payer, utx, dryRun);
    if (sig) {
      if (NEEDS_FINALITY.has(utx.description)) await waitFinalized(connection, sig);
      lastSig = sig;
    }
  }
  return lastSig;
}

async function fetchSequence(connection: Connection, signature: string): Promise<string | undefined> {
  const tx = await connection.getTransaction(signature, {
    commitment: "confirmed",
    maxSupportedTransactionVersion: 0,
  });
  const line = tx?.meta?.logMessages?.find((l) => l.startsWith(SEQ_LOG_PREFIX));
  return line ? line.slice(SEQ_LOG_PREFIX.length) : undefined;
}

export async function solanaSend(
  cfg: PrivateDeploymentsConfig,
  dep: DeploymentConfig,
  amountHuman: string,
  destChain: ChainName,
  recipient32Hex: string,
  opts: SolanaOpOptions = {}
): Promise<SolanaSendResult | undefined> {
  const sol = dep.chains.Solana;
  if (!sol) throw new Error("deployment has no Solana leg");

  const connection = buildSolanaConnection();
  const payer = loadSolanaKeypair();
  const ntt = buildSolanaNtt(cfg, connection, sol);
  const rawAmount = toRaw(amountHuman, sol.decimals);

  const sender = new SolanaAddress(payer.publicKey);
  const destination = {
    chain: destChain,
    address: toUniversal(destChain, hexToBytes(recipient32Hex)),
  };

  const generator = ntt.transfer(sender as any, rawAmount, destination as any, {
    queue: false,
    automatic: false,
  });

  const lastSig = await driveGenerator(connection, payer, generator, opts.dryRun);
  if (opts.dryRun || !lastSig) return undefined;
  const sequence = await fetchSequence(connection, lastSig);
  return { signature: lastSig, sequence };
}

/**
 * Fails fast with an actionable error if the recipient's ATA doesn't exist.
 * SolanaNtt.redeem only ever creates an ATA for the *payer* (the signer
 * submitting the redeem), not for the VAA's recipient — a foreign recipient
 * (recipient != payer) whose ATA was never created would otherwise fail deep
 * inside the final combined instruction, after everything else has already
 * landed on-chain.
 */
async function checkRecipientAtaExists(
  connection: Connection,
  ntt: SolanaNtt<Network, "Solana">,
  attestation: any
): Promise<void> {
  const config = await ntt.getConfig();
  const recipient = new PublicKey(
    attestation.payload.nttManagerPayload.payload.recipientAddress.toUint8Array()
  );
  const ata = splToken.getAssociatedTokenAddressSync(
    config.mint,
    recipient,
    true,
    config.tokenProgram
  );
  const info = await connection.getAccountInfo(ata);
  if (!info) {
    throw new Error(
      `recipient ${recipient.toBase58()}'s associated token account ${ata.toBase58()} ` +
        `does not exist; SolanaNtt.redeem only creates the ATA for the payer submitting the ` +
        `redeem, not for a foreign recipient — create it first (e.g. via an ATA-create tx funded ` +
        `by any payer) and re-run. The VAA is unaffected and stays redeemable.`
    );
  }
}

export async function solanaRedeem(
  cfg: PrivateDeploymentsConfig,
  dep: DeploymentConfig,
  attestation: unknown,
  opts: SolanaOpOptions = {}
): Promise<SolanaRedeemResult | undefined> {
  const sol = dep.chains.Solana;
  if (!sol) throw new Error("deployment has no Solana leg");

  const connection = buildSolanaConnection();
  const payer = loadSolanaKeypair();
  const ntt = buildSolanaNtt(cfg, connection, sol);

  if (!opts.dryRun) {
    await checkRecipientAtaExists(connection, ntt, attestation);
  }

  const payerAddr = new SolanaAddress(payer.publicKey);
  const generator = ntt.redeem([attestation as any], payerAddr as any);

  const lastSig = await driveGenerator(connection, payer, generator, opts.dryRun);
  return opts.dryRun ? undefined : { signature: lastSig };
}
