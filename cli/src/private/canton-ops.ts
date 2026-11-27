// Canton send and redeem through the Canton NTT SDK (@wormhole-foundation/sdk-canton-ntt): the
// registry's Canton leg becomes the SDK config; the SDK reads the flow, runs the anchor checks
// against the registry values, and builds the one command; a local-party signer submits it.
import type { Network } from "@wormhole-foundation/sdk-base";
import { encoding } from "@wormhole-foundation/sdk-base";
import {
  UniversalAddress,
  deserialize,
  serialize,
} from "@wormhole-foundation/sdk-definitions";
import type { Contracts } from "@wormhole-foundation/sdk-definitions";
import type { Ntt } from "@wormhole-foundation/sdk-definitions-ntt";
import {
  CantonAddress,
  CantonLedgerError,
  CantonLocalSigner,
  CantonUnsignedTransaction,
} from "@wormhole-foundation/sdk-canton";
import type {
  CantonLedgerClient,
  FetchFunction,
  JsTransaction,
} from "@wormhole-foundation/sdk-canton";
import {
  CantonNtt,
  register as registerCantonNtt,
} from "@wormhole-foundation/sdk-canton-ntt";
import type {
  CantonNttContracts,
  NttFlowSource,
} from "@wormhole-foundation/sdk-canton-ntt";

import {
  sdkChainForRegistryChain,
  type CantonChainConfig,
  type ChainName,
  type Deployment,
  type PrivateRegistry,
} from "./config.js";
import {
  hex32ToBytes,
  type CantonPartyId,
  type Hex32,
  type RawAmount,
  type TransferNonceU32,
} from "./types.js";
import type { ParsedTransferVaa } from "./vaa.js";

// The NTT payload layouts (VAA decoding) and the Ntt protocol on Canton.
registerCantonNtt();

/** The ledger, the acting party and the flow source of one Canton send or redeem */
export interface CantonClients {
  /** JSON Ledger API client of the user's participant */
  ledger: CantonLedgerClient;
  /** The party the command acts as: the sender, or the redeem executor */
  partyId: CantonPartyId;
  /** Where flows come from; default: the flow service at the registry's disclosureServiceUrl */
  flowSource?: NttFlowSource;
  /** fetch for the default flow source (tests) */
  fetch?: FetchFunction;
  /** Wait before a rebuild after a stale contract reads the flow again; default: the SDK's (5.5 s) */
  rebuildDelayMs?: number;
  /** User id on each submission, for a ledger whose token is not a user token (tests, sandboxes) */
  userId?: string;
}

function cantonLeg(deployment: Deployment): CantonChainConfig {
  const canton = deployment.chains.Canton;
  if (!canton)
    throw new Error(`deployment "${deployment.name}" has no Canton leg`);
  return canton;
}

/**
 * The SDK config of a registry's Canton leg: manager -> ntt.manager, emitter ->
 * ntt.transceiver.wormhole, instrument -> ntt.token "<admin>/<id>", decimals -> ntt.tokenDecimals,
 * guardianGovernance -> coreBridge, and reserveCustodian, mode, the vetted package ids and the
 * disclosure service URL -> ntt.canton. The SDK pins each of them in its anchor checks.
 */
export function cantonNttContracts(
  registry: PrivateRegistry,
  deployment: Deployment
): Contracts & { ntt: CantonNttContracts } {
  const canton = cantonLeg(deployment);
  return {
    coreBridge: canton.guardianGovernance,
    ntt: {
      manager: canton.manager,
      token: `${canton.instrument.admin}/${canton.instrument.id}`,
      tokenDecimals: canton.decimals,
      transceiver: { wormhole: canton.emitter },
      canton: {
        reserveCustodian: canton.reserveCustodian,
        flowServiceUrl: registry.canton.disclosureServiceUrl.href,
        nttPackageIds: { current: canton.vettedPackageIds.ntt },
        corePackageIds: { current: canton.vettedPackageIds.core },
        mode: canton.mode,
      },
    },
  };
}

/** The SDK's Ntt client of a deployment's Canton leg */
export function cantonNttFor(
  registry: PrivateRegistry,
  deployment: Deployment,
  clients: CantonClients
): CantonNtt<Network, "Canton"> {
  return CantonNtt.withOptions<Network, "Canton">(
    registry.network,
    "Canton",
    clients.ledger,
    cantonNttContracts(registry, deployment),
    {
      ...(clients.flowSource ? { flowSource: clients.flowSource } : {}),
      ...(clients.fetch ? { fetch: clients.fetch } : {}),
      ...(clients.rebuildDelayMs !== undefined
        ? { rebuildDelayMs: clients.rebuildDelayMs }
        : {}),
    }
  );
}

async function onlyTransaction(
  generator: AsyncGenerator<CantonUnsignedTransaction<Network, "Canton">>
): Promise<CantonUnsignedTransaction<Network, "Canton">> {
  const transactions: CantonUnsignedTransaction<Network, "Canton">[] = [];
  for await (const transaction of generator) transactions.push(transaction);
  if (transactions.length !== 1)
    throw new Error(
      `expected one Canton transaction, the SDK built ${transactions.length}`
    );
  return transactions[0]!;
}

/** The transaction with a rebuild that says so, as the CLI did before each new flow */
function announcingRebuilds(
  transaction: CantonUnsignedTransaction<Network, "Canton">
): CantonUnsignedTransaction<Network, "Canton"> {
  const { rebuild } = transaction;
  if (!rebuild) return transaction;
  let attemptNumber = 1;
  return new CantonUnsignedTransaction(
    transaction.transaction,
    transaction.network,
    transaction.chain,
    transaction.description,
    transaction.parallelizable,
    async () => {
      attemptNumber++;
      console.log(
        `  a disclosed contract changed; new flow, attempt ${attemptNumber}`
      );
      return rebuild();
    }
  );
}

function dryRunLine(
  transaction: CantonUnsignedTransaction<Network, "Canton">
): string {
  return `  [dry-run] would submit command ${transaction.transaction.commandId} with ${transaction.transaction.disclosedContracts.length} disclosed contracts`;
}

/** Submits one transaction as the clients' party; returns the update id and the transaction */
async function submit(
  clients: CantonClients,
  transaction: CantonUnsignedTransaction<Network, "Canton">
): Promise<{ updateId: string; committed: JsTransaction | undefined }> {
  let committed: JsTransaction | undefined;
  const signer = new CantonLocalSigner(
    "Canton",
    clients.ledger,
    clients.partyId,
    {
      ...(clients.userId !== undefined ? { userId: clients.userId } : {}),
      onTransaction: (tx) => {
        committed = tx;
      },
    }
  );
  const [updateId] = await signer.signAndSend([
    announcingRebuilds(transaction),
  ]);
  return { updateId: updateId!, committed };
}

/**
 * True when the participant answered a submission with a definite refusal: a 4xx status with a
 * ledger error code. Every other failed answer leaves the result unknown: a 5xx or 408 can come
 * from the participant or a proxy while the command still commits, a 429 is a refusal to answer,
 * and a body with no error code is not the participant's verdict.
 */
export function isDefiniteLedgerRejection(err: unknown): boolean {
  return (
    err instanceof CantonLedgerError &&
    err.status >= 400 &&
    err.status <= 499 &&
    err.status !== 408 &&
    err.status !== 429 &&
    err.code !== undefined
  );
}

export interface CantonSendInput {
  registry: PrivateRegistry;
  deployment: Deployment;
  destinationChain: ChainName;
  rawAmount: RawAmount;
  /** The decimals rawAmount is in; the SDK refuses the transfer when the NttManager has others */
  tokenDecimals: number;
  recipient: Hex32;
  transferNonce: TransferNonceU32;
}

export interface CantonSendResult {
  commandId: string;
  updateId: string;
  sequence?: string;
}

/**
 * One NttManager Transfer as the clients' party. The command id derives from the user, manager,
 * amount, destination and nonce (the same formula as before the SDK), so a retry with the same
 * --transfer-nonce is deduplicated. Returns undefined for a dry run (built, not submitted).
 */
export async function cantonSend(
  clients: CantonClients,
  input: CantonSendInput,
  options: { dryRun: boolean }
): Promise<CantonSendResult | undefined> {
  const ntt = cantonNttFor(input.registry, input.deployment, clients);
  const transaction = await onlyTransaction(
    ntt.transfer(
      new CantonAddress(clients.partyId),
      input.rawAmount,
      {
        chain: sdkChainForRegistryChain(
          input.registry.network,
          input.destinationChain
        ),
        address: new UniversalAddress(hex32ToBytes(input.recipient)),
      },
      {
        queue: false,
        nonce: input.transferNonce,
        tokenDecimals: input.tokenDecimals,
      }
    )
  );
  const { commandId } = transaction.transaction;
  if (options.dryRun) {
    console.log(dryRunLine(transaction));
    return undefined;
  }
  let submitted: Awaited<ReturnType<typeof submit>>;
  try {
    submitted = await submit(clients, transaction);
  } catch (err) {
    const retry = `Run the same send with --transfer-nonce ${input.transferNonce}: the command id is then the same, and the participant rejects a duplicate.`;
    if (isDefiniteLedgerRejection(err))
      throw new Error(
        `Canton submit of command ${commandId} was rejected (${(err as Error).message}). ${retry}`
      );
    // Anything else leaves the result unknown: no answer (the signer says so for a timeout or a
    // reset), or an answer that is not a definite refusal (isDefiniteLedgerRejection). The
    // command can still commit, so the only safe retry is the same command id.
    const message = (err as Error).message;
    throw new Error(
      /result unknown/.test(message)
        ? `${message} ${retry}`
        : `Canton submit of command ${commandId}: result unknown (${message}). ${retry}`
    );
  }
  const { updateId, committed } = submitted;
  try {
    if (!committed) throw new Error("the participant returned no transaction");
    const messageId = ntt.messageIdFromTransaction(committed);
    return { commandId, updateId, sequence: messageId.sequence.toString() };
  } catch (err) {
    console.log(
      `  command ${commandId} committed (update ${updateId}); the sequence is not readable (${(err as Error).message})`
    );
    return { commandId, updateId };
  }
}

export interface CantonRedeemInput {
  registry: PrivateRegistry;
  deployment: Deployment;
  parsedVaa: ParsedTransferVaa;
  recipient: CantonPartyId;
}

export interface CantonRedeemResult {
  commandId: string;
  updateId: string;
  eventCount?: number;
}

/** The SDK attestation of a parsed VAA; its encoding must be the VAA's own bytes */
export function attestationOf(parsedVaa: ParsedTransferVaa): Ntt.Attestation {
  const attestation = deserialize(
    "Ntt:WormholeTransfer",
    parsedVaa.bytes
  ) as Ntt.Attestation;
  if (
    encoding.hex.encode(serialize(attestation)) !==
    encoding.hex.encode(parsedVaa.bytes)
  )
    throw new Error(
      "the VAA does not re-encode to its own bytes; nothing has been submitted"
    );
  return attestation;
}

/**
 * Release (locking) or Mint (burning) as the clients' party, for the recipient party. The command
 * id derives from the VAA digest. After a stale contract the signer rebuilds the command from a
 * new flow under the same command id, three attempts in all. Returns undefined for a dry run.
 */
export async function cantonRedeem(
  clients: CantonClients,
  input: CantonRedeemInput,
  options: { dryRun: boolean }
): Promise<CantonRedeemResult | undefined> {
  const ntt = cantonNttFor(input.registry, input.deployment, clients);
  const transaction = await onlyTransaction(
    ntt.redeem(
      [attestationOf(input.parsedVaa)],
      new CantonAddress(clients.partyId),
      { recipient: input.recipient }
    )
  );
  if (options.dryRun) {
    console.log(dryRunLine(transaction));
    return undefined;
  }
  const { updateId, committed } = await submit(clients, transaction);
  return {
    commandId: transaction.transaction.commandId,
    updateId,
    ...(committed ? { eventCount: committed.events.length } : {}),
  };
}
