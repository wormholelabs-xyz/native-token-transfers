// SolanaNtt adapter. SolanaNtt yields @solana/web3.js transactions, so this module keeps web3.js;
// address parsing uses parseSolanaAddressBytes32.
import {
  Connection,
  Keypair,
  PublicKey,
  VersionedTransaction,
  type Signer,
} from "@solana/web3.js";
import * as splToken from "@solana/spl-token";
import * as fs from "node:fs";

import {
  deserialize,
  encoding,
  toUniversal,
  type Chain,
  type Network,
} from "@wormhole-foundation/sdk";
import { register as registerNttPayloads } from "@wormhole-foundation/sdk-definitions-ntt";
import {
  SolanaAddress,
  isVersionedTransaction,
} from "@wormhole-foundation/sdk-solana";
import { SolanaNtt } from "@wormhole-foundation/sdk-solana-ntt";

import {
  hex32ToBytes,
  hex32FromBytes,
  type Hex32,
  type RawAmount,
} from "./types.js";
import type {
  ChainName,
  Deployment,
  PrivateRegistry,
  SolanaChainConfig,
} from "./config.js";
import { chainLegIdentity } from "./transfer-route.js";
import {
  checkSolanaPeerIsRegistryManager,
  parseSolanaKeypairFileText,
  runSolanaRedeemSteps,
  waitForSolanaConfirmation,
  type SolanaStatusRpc,
} from "./solana-pure.js";

registerNttPayloads();

const DEFAULT_SOLANA_RPC_BY_NETWORK = {
  Mainnet: "https://rpc.labsapis.com/mainnet/solana",
  Testnet: "https://rpc.labsapis.com/testnet/solana",
} as const;
const RPC_PROXY_ORIGIN = "https://portalbridge.com";
const SEQUENCE_LOG_PREFIX = "Program log: Sequence: ";
const SOLANA_KEYPAIR_FILE_BYTES_MAX = 4096;
const FINALIZED_WAIT_MAX_POLLS = 30;
const FINALIZED_WAIT_POLL_INTERVAL_MS = 1_000;
// A later transaction in the same generator reads the account that these create.
const DESCRIPTIONS_THAT_NEED_FINALITY = new Set([
  "Core.VerifySignature",
  "Core.PostVAA",
  "Redeem.CreateATA",
]);

function buildSolanaConnection(registry: PrivateRegistry): Connection {
  return new Connection(
    process.env.SOL_RPC ?? DEFAULT_SOLANA_RPC_BY_NETWORK[registry.network],
    {
      commitment: "confirmed",
      httpHeaders: { Origin: RPC_PROXY_ORIGIN },
    }
  );
}

function loadSolanaKeypair(): Keypair {
  const payerPath = process.env.SOLANA_PAYER_PATH;
  if (payerPath) {
    if (fs.statSync(payerPath).size > SOLANA_KEYPAIR_FILE_BYTES_MAX) {
      throw new Error(
        `SOLANA_PAYER_PATH is larger than ${SOLANA_KEYPAIR_FILE_BYTES_MAX} bytes`
      );
    }
    return Keypair.fromSecretKey(
      parseSolanaKeypairFileText(fs.readFileSync(payerPath, "utf8"))
    );
  }
  const secretKeyBase58 = process.env.SOLANA_PRIVATE_KEY;
  if (!secretKeyBase58)
    throw new Error("SOLANA_PRIVATE_KEY or SOLANA_PAYER_PATH env var not set");
  return Keypair.fromSecretKey(encoding.b58.decode(secretKeyBase58));
}

export function deriveSolanaAddress(): string {
  return loadSolanaKeypair().publicKey.toBase58();
}

function solanaLeg(deployment: Deployment): SolanaChainConfig {
  const solana = deployment.chains.Solana;
  if (!solana)
    throw new Error(`deployment "${deployment.name}" has no Solana leg`);
  return solana;
}

function buildSolanaNtt(
  registry: PrivateRegistry,
  connection: Connection,
  solana: SolanaChainConfig
) {
  return new SolanaNtt(
    registry.network as Network,
    "Solana",
    connection,
    {
      coreBridge: solana.coreBridge,
      ntt: {
        manager: solana.manager,
        token: solana.mint,
        transceiver: { wormhole: solana.transceiver },
      },
    },
    solana.idlVersion
  );
}

function statusRpc(connection: Connection): SolanaStatusRpc {
  return {
    async getSignatureConfirmationStatus(signature) {
      const status = (
        await connection.getSignatureStatus(signature, {
          searchTransactionHistory: true,
        })
      ).value;
      if (!status) return undefined;
      return {
        confirmationStatus: status.confirmationStatus ?? "processed",
        failed: status.err !== null,
      };
    },
    getBlockHeight: () => connection.getBlockHeight("confirmed"),
  };
}

async function waitFinalized(
  connection: Connection,
  signature: string
): Promise<void> {
  for (
    let pollNumber = 1;
    pollNumber <= FINALIZED_WAIT_MAX_POLLS;
    pollNumber++
  ) {
    const status = await connection.getSignatureStatus(signature, {
      searchTransactionHistory: true,
    });
    if (status.value?.confirmationStatus === "finalized") return;
    await new Promise((resolve) =>
      setTimeout(resolve, FINALIZED_WAIT_POLL_INTERVAL_MS)
    );
  }
  throw new Error(
    `${signature} did not reach finalized in ${FINALIZED_WAIT_MAX_POLLS} polls; posted VAA and redeem are idempotent, a re-run is safe`
  );
}

async function driveTransaction(
  connection: Connection,
  payer: Keypair,
  unsigned: {
    transaction: { transaction: any; signers?: Signer[] };
    description: string;
  },
  dryRun: boolean
): Promise<string | undefined> {
  const { transaction, signers } = unsigned.transaction;
  const { blockhash, lastValidBlockHeight } =
    await connection.getLatestBlockhash();
  let signature: string;
  if (isVersionedTransaction(transaction)) {
    transaction.message.recentBlockhash = blockhash;
    transaction.sign([payer, ...(signers ?? [])]);
    if (dryRun) {
      const simulation = await connection.simulateTransaction(transaction);
      console.log(
        `  [dry-run] ${unsigned.description}: ${simulation.value.err ? `ERROR ${JSON.stringify(simulation.value.err)}` : "ok"}`
      );
      return undefined;
    }
    signature = await connection.sendTransaction(transaction);
  } else {
    transaction.recentBlockhash = blockhash;
    transaction.lastValidBlockHeight = lastValidBlockHeight;
    transaction.feePayer = transaction.feePayer ?? payer.publicKey;
    if (dryRun) {
      const simulation = await connection.simulateTransaction(
        new VersionedTransaction(transaction.compileMessage()),
        {
          sigVerify: false,
        }
      );
      console.log(
        `  [dry-run] ${unsigned.description}: ${simulation.value.err ? `ERROR ${JSON.stringify(simulation.value.err)}` : "ok"}`
      );
      return undefined;
    }
    transaction.partialSign(payer, ...(signers ?? []));
    signature = await connection.sendRawTransaction(transaction.serialize());
  }
  console.log(`  ${unsigned.description}: tx ${signature}`);
  await waitForSolanaConfirmation(
    statusRpc(connection),
    signature,
    lastValidBlockHeight
  );
  return signature;
}

async function driveGenerator(
  connection: Connection,
  payer: Keypair,
  generator: AsyncGenerator<any>,
  dryRun: boolean
) {
  let lastSignature: string | undefined;
  for await (const unsigned of generator) {
    const signature = await driveTransaction(
      connection,
      payer,
      unsigned,
      dryRun
    );
    if (signature && DESCRIPTIONS_THAT_NEED_FINALITY.has(unsigned.description))
      await waitFinalized(connection, signature);
    lastSignature = signature ?? lastSignature;
  }
  return lastSignature;
}

async function checkPeer(
  ntt: SolanaNtt<Network, "Solana">,
  deployment: Deployment,
  peerChain: ChainName
) {
  const peer = await ntt.getPeer(peerChain as Chain);
  if (!peer) throw new Error(`Solana NttManager has no peer for ${peerChain}`);
  const observedPeer = hex32FromBytes(peer.address.address.toUint8Array());
  checkSolanaPeerIsRegistryManager(
    observedPeer,
    chainLegIdentity(deployment, peerChain).manager,
    peerChain
  );
}

export async function solanaSend(
  parameters: {
    registry: PrivateRegistry;
    deployment: Deployment;
    destinationChain: ChainName;
    rawAmount: RawAmount;
    recipient: Hex32;
  },
  options: { dryRun: boolean }
): Promise<{ signature: string; sequence?: string } | undefined> {
  const solana = solanaLeg(parameters.deployment);
  const connection = buildSolanaConnection(parameters.registry);
  const payer = loadSolanaKeypair();
  const ntt = buildSolanaNtt(parameters.registry, connection, solana);
  await checkPeer(ntt, parameters.deployment, parameters.destinationChain);

  const destination = {
    chain: parameters.destinationChain as Chain,
    address: toUniversal(
      parameters.destinationChain as Chain,
      hex32ToBytes(parameters.recipient)
    ),
  };
  const generator = ntt.transfer(
    new SolanaAddress(payer.publicKey),
    parameters.rawAmount,
    destination,
    {
      queue: false,
      automatic: false,
    }
  );
  const lastSignature = await driveGenerator(
    connection,
    payer,
    generator,
    options.dryRun
  );
  if (options.dryRun || !lastSignature) return undefined;
  const transaction = await connection.getTransaction(lastSignature, {
    commitment: "confirmed",
    maxSupportedTransactionVersion: 0,
  });
  const sequenceLine = transaction?.meta?.logMessages?.find((line) =>
    line.startsWith(SEQUENCE_LOG_PREFIX)
  );
  return {
    signature: lastSignature,
    sequence: sequenceLine?.slice(SEQUENCE_LOG_PREFIX.length),
  };
}

export async function solanaRedeem(
  parameters: {
    registry: PrivateRegistry;
    deployment: Deployment;
    vaaBytes: Uint8Array;
  },
  options: { dryRun: boolean }
): Promise<{ signature?: string }> {
  const solana = solanaLeg(parameters.deployment);
  const connection = buildSolanaConnection(parameters.registry);
  const payer = loadSolanaKeypair();
  const ntt = buildSolanaNtt(parameters.registry, connection, solana);
  const attestation = deserialize("Ntt:WormholeTransfer", parameters.vaaBytes);

  const signature = await runSolanaRedeemSteps(
    {
      async checkRecipientTokenAccount() {
        const nttConfig = await ntt.getConfig();
        const recipient = new PublicKey(
          attestation.payload.nttManagerPayload.payload.recipientAddress.toUint8Array()
        );
        const tokenAccount = splToken.getAssociatedTokenAddressSync(
          nttConfig.mint,
          recipient,
          true,
          nttConfig.tokenProgram
        );
        if (!(await connection.getAccountInfo(tokenAccount))) {
          throw new Error(
            `recipient ${recipient.toBase58()} has no associated token account ${tokenAccount.toBase58()}; create it, then run the redeem again. The VAA stays redeemable.`
          );
        }
      },
      driveRedeem: (dryRun) =>
        driveGenerator(
          connection,
          payer,
          ntt.redeem([attestation], new SolanaAddress(payer.publicKey)),
          dryRun
        ),
    },
    options
  );
  return { signature };
}
