// yargs registration for `ntt private list|send|redeem`: argument parsing and dispatch.
import { randomInt } from "node:crypto";
import * as readline from "node:readline/promises";
import type { Argv } from "yargs";

import { colors } from "../colors.js";
import {
  checkAmountHasNoDust,
  formatRawAmount,
  parseHumanAmount,
} from "./amounts.js";
import { recipientBinding } from "./binding.js";
import { cantonRedeem, cantonSend } from "./canton-ops.js";
import { HttpDisclosureServiceClient } from "./canton/disclosure-client.js";
import { CantonLedgerClient } from "./canton/ledger-client.js";
import {
  CHAIN_NAMES,
  loadRegistry,
  requireDeployment,
  type ChainName,
  type PrivateRegistry,
  type RegistrySource,
} from "./config.js";
import {
  confirmationLines,
  requireTypedConfirmation,
  type SubmitConfirmation,
} from "./confirmation.js";
import { buildEthersSigner, deriveEthereumAddress } from "./evm.js";
import { evmRedeemWithSigner, evmSendWithSigner } from "./evm-send.js";
import { resolveSendRecipient, type RecipientDefaults } from "./recipient.js";
import {
  chainLegIdentity,
  checkTransferVaaMatchesDeployment,
} from "./transfer-route.js";
import {
  TRANSFER_NONCE_U32_MAX,
  parseCantonPartyId,
  parseTransferNonceU32,
  type CantonPartyId,
} from "./types.js";
import { parseTransferVaa } from "./vaa.js";

const CHAIN_ALIASES: Record<string, ChainName> = {
  canton: "Canton",
  ethereum: "Ethereum",
  eth: "Ethereum",
  solana: "Solana",
  sol: "Solana",
};
const CHAIN_CHOICES = Object.keys(CHAIN_ALIASES);
const REGISTRY_SHA256_PREFIX_LENGTH = 16;

export function parseChainAlias(text: string): ChainName {
  const chain = CHAIN_ALIASES[text.trim().toLowerCase()];
  if (!chain)
    throw new Error(
      `unknown chain "${text}"; expected one of ${CHAIN_CHOICES.join(", ")}`
    );
  return chain;
}

function loadAndPrintRegistry(explicitPath: string | undefined): {
  registry: PrivateRegistry;
  source: RegistrySource;
} {
  const loaded = loadRegistry(explicitPath);
  console.log(
    `registry: ${loaded.source.absolutePath} (sha256 ${loaded.source.sha256Hex.slice(0, REGISTRY_SHA256_PREFIX_LENGTH)})`
  );
  if (!loaded.source.isRepositoryFile) {
    console.log(
      colors.red(
        "  WARNING: this registry is not the repository file. Check its endpoints and addresses."
      )
    );
  }
  return loaded;
}

function buildCantonClients(
  registry: PrivateRegistry,
  userPartyId?: CantonPartyId
) {
  const participant = registry.canton.participant;
  if (!participant)
    throw new Error(
      "the registry has no canton.participant section; pass --deployments with a local registry"
    );
  const oauthClientSecret = process.env.CANTON_OAUTH_CLIENT_SECRET;
  if (!oauthClientSecret)
    throw new Error("CANTON_OAUTH_CLIENT_SECRET env var not set");
  return {
    ledger: new CantonLedgerClient({
      ...participant,
      userPartyId: userPartyId ?? participant.userPartyId,
      oauthClientSecret,
    }),
    disclosure: new HttpDisclosureServiceClient(
      registry.canton.disclosureServiceUrl
    ),
  };
}

function readRecipientDefaults(): RecipientDefaults {
  const defaults: RecipientDefaults = {};
  if (process.env.ETH_PRIVATE_KEY)
    defaults.ethereumAddress = deriveEthereumAddress();
  return defaults;
}

async function promptOnTerminal(question: string): Promise<string> {
  const terminal = readline.createInterface({
    input: process.stdin,
    output: process.stdout,
  });
  try {
    return await terminal.question(question);
  } finally {
    terminal.close();
  }
}

async function confirmSubmit(
  confirmation: SubmitConfirmation,
  dryRun: boolean,
  assumeYes: boolean
): Promise<void> {
  for (const line of confirmationLines(confirmation)) console.log(`  ${line}`);
  if (dryRun) {
    console.log(colors.yellow("--dry-run: nothing will be submitted"));
    return;
  }
  await requireTypedConfirmation(confirmation, {
    assumeYes,
    stdinIsTty: Boolean(process.stdin.isTTY),
    prompt: promptOnTerminal,
  });
}

interface SendArguments {
  deployments?: string;
  deployment: string;
  amount: string;
  from: string;
  to: string;
  recipient?: string;
  recipientRaw?: string;
  acceptRawRecipient: boolean;
  acceptUnchecksummedAddress: boolean;
  transferNonce?: string;
  dryRun: boolean;
  yes: boolean;
}

async function runSend(sendArguments: SendArguments): Promise<void> {
  const { registry } = loadAndPrintRegistry(sendArguments.deployments);
  const deployment = requireDeployment(registry, sendArguments.deployment);
  const sourceChain = parseChainAlias(sendArguments.from);
  const destinationChain = parseChainAlias(sendArguments.to);
  if (sourceChain === destinationChain)
    throw new Error("--from and --to must be different chains");
  const source = chainLegIdentity(deployment, sourceChain);
  const destination = chainLegIdentity(deployment, destinationChain);
  const rawAmount = parseHumanAmount(sendArguments.amount, source.decimals);
  checkAmountHasNoDust(rawAmount, source.decimals, destination.decimals);

  const resolved = resolveSendRecipient(
    destinationChain,
    registry,
    {
      recipient: sendArguments.recipient,
      recipientRaw: sendArguments.recipientRaw,
      acceptRawRecipient: sendArguments.acceptRawRecipient,
      acceptUnchecksummedAddress: sendArguments.acceptUnchecksummedAddress,
    },
    readRecipientDefaults()
  );
  if (resolved.isRaw)
    console.log(
      colors.red(
        "  WARNING: raw bytes32 recipient. A wrong value strands the funds."
      )
    );
  await confirmSubmit(
    {
      network: registry.network,
      operation: "send",
      deploymentName: deployment.name,
      route: `${sourceChain} -> ${destinationChain}`,
      amountText: `${formatRawAmount(rawAmount, source.decimals)} (${rawAmount} raw)`,
      recipientDescription: resolved.description,
      recipient: resolved.recipient,
      cantonRecipientPartyId: resolved.cantonPartyId,
    },
    sendArguments.dryRun,
    sendArguments.yes
  );

  const options = { dryRun: sendArguments.dryRun };
  const sendParameters = {
    registry,
    deployment,
    destinationChain,
    rawAmount,
    recipient: resolved.recipient,
  };
  let sequence: string | undefined;
  if (sourceChain === "Canton") {
    const transferNonce = parseTransferNonceU32(
      sendArguments.transferNonce ?? randomInt(0, TRANSFER_NONCE_U32_MAX)
    );
    console.log(
      `  transfer nonce: ${transferNonce} (pass --transfer-nonce ${transferNonce} to retry this exact transfer)`
    );
    const { ledger, disclosure } = buildCantonClients(registry);
    const result = await cantonSend(
      ledger,
      disclosure,
      { ...sendParameters, transferNonce },
      options
    );
    if (result)
      console.log(
        `sent: command ${result.commandId}, sequence ${result.sequence}`
      );
    sequence = result?.sequence;
  } else if (sourceChain === "Ethereum") {
    const result = await evmSendWithSigner(
      buildEthersSigner(registry.network),
      sendParameters,
      options
    );
    if (result)
      console.log(
        `sent: tx ${result.txHash}, sequence ${result.sequence ?? "(not found in the receipt)"}`
      );
    sequence = result?.sequence;
  } else {
    const { solanaSend } = await import("./solana.js");
    const result = await solanaSend(sendParameters, options);
    if (result)
      console.log(
        `sent: tx ${result.signature}, sequence ${result.sequence ?? "(not found in the logs)"}`
      );
    sequence = result?.sequence;
  }
  if (sequence) {
    console.log(
      `next: fetch VAA (chain ${registry.chainIds[sourceChain]}, emitter 0x${source.emitter}, sequence ${sequence}),`
    );
    console.log(
      `      then: ntt private redeem --deployment ${deployment.name} --vaa <hex>`
    );
  }
}

interface RedeemArguments {
  deployments?: string;
  deployment: string;
  vaa: string;
  recipient?: string;
  dryRun: boolean;
  yes: boolean;
}

async function runRedeem(redeemArguments: RedeemArguments): Promise<void> {
  const { registry } = loadAndPrintRegistry(redeemArguments.deployments);
  const deployment = requireDeployment(registry, redeemArguments.deployment);
  const parsedVaa = parseTransferVaa(redeemArguments.vaa);
  const { sourceChain, destinationChain } = checkTransferVaaMatchesDeployment(
    parsedVaa,
    registry,
    deployment
  );
  const options = { dryRun: redeemArguments.dryRun };

  let cantonRecipientPartyId: CantonPartyId | undefined;
  if (destinationChain === "Canton") {
    const partyText =
      redeemArguments.recipient ?? registry.canton.participant?.userPartyId;
    if (!partyText)
      throw new Error(
        "no Canton recipient: pass --recipient <hint>::1220<64 hex>"
      );
    cantonRecipientPartyId = parseCantonPartyId(partyText);
    if (
      recipientBinding(cantonRecipientPartyId) !== parsedVaa.recipientAddress
    ) {
      throw new Error(
        `the VAA recipient 0x${parsedVaa.recipientAddress} is not the binding of ${cantonRecipientPartyId}; pass the correct --recipient`
      );
    }
  } else if (redeemArguments.recipient) {
    throw new Error(
      `--recipient applies to a Canton redeem only; this VAA goes to ${destinationChain}`
    );
  }

  await confirmSubmit(
    {
      network: registry.network,
      operation: "redeem",
      deploymentName: deployment.name,
      route: `${sourceChain} -> ${destinationChain}, sequence ${parsedVaa.sequence}`,
      amountText: `${formatRawAmount(parsedVaa.trimmedAmount.amount, parsedVaa.trimmedAmount.decimals)} (wire amount)`,
      recipientDescription: cantonRecipientPartyId
        ? `Canton party ${cantonRecipientPartyId}`
        : destinationChain,
      recipient: parsedVaa.recipientAddress,
      cantonRecipientPartyId,
    },
    redeemArguments.dryRun,
    redeemArguments.yes
  );

  if (destinationChain === "Canton") {
    const { ledger, disclosure } = buildCantonClients(
      registry,
      cantonRecipientPartyId
    );
    const result = await cantonRedeem(
      ledger,
      disclosure,
      { registry, deployment, parsedVaa, recipient: cantonRecipientPartyId! },
      options
    );
    if (result)
      console.log(
        `redeemed: command ${result.commandId}, ${result.transaction.events.length} events`
      );
  } else if (destinationChain === "Ethereum") {
    const transceiver = deployment.chains.Ethereum!.transceiver;
    const result = await evmRedeemWithSigner(
      buildEthersSigner(registry.network),
      registry.network,
      transceiver,
      parsedVaa.bytes,
      options
    );
    if (result) console.log(`redeemed: tx ${result.txHash}`);
  } else {
    const { solanaRedeem } = await import("./solana.js");
    const result = await solanaRedeem(
      { registry, deployment, vaaBytes: parsedVaa.bytes },
      options
    );
    if (result.signature) console.log(`redeemed: tx ${result.signature}`);
  }
}

function runList(explicitPath: string | undefined): void {
  const { registry } = loadAndPrintRegistry(explicitPath);
  console.log(
    colors.blue(`private deployments (network: ${registry.network})`)
  );
  if (registry.deployments.size === 0)
    console.log("  (none; pass --deployments with a local registry)");
  for (const deployment of registry.deployments.values()) {
    const legs = CHAIN_NAMES.filter((chain) => deployment.chains[chain]).map(
      (chain) =>
        `${chain}(${deployment.chains[chain]!.mode},d${deployment.chains[chain]!.decimals})`
    );
    console.log(
      `  ${colors.cyan(deployment.name.padEnd(12))} ${legs.join("  <->  ")}`
    );
  }
}

const DEPLOYMENTS_OPTION = {
  describe:
    "path to a private-deployments registry (else NTT_PRIVATE_DEPLOYMENTS, else the repository file)",
  type: "string",
} as const;
const DRY_RUN_OPTION = {
  describe: "read and simulate only; submit nothing",
  type: "boolean",
  default: false,
} as const;
const YES_OPTION = {
  describe: "skip the typed confirmation (scripts)",
  type: "boolean",
  default: false,
} as const;

export function registerPrivateCommands(yargs: Argv): Argv {
  return yargs
    .command(
      "list",
      "List the private deployments in the registry",
      (y) => y.option("deployments", DEPLOYMENTS_OPTION),
      (argv) => runList(argv.deployments)
    )
    .command(
      "send <amount>",
      "Send an NTT transfer on the private network",
      (y) =>
        y
          .positional("amount", {
            describe: "amount in human units, e.g. 1.5",
            type: "string",
            demandOption: true,
          })
          .option("deployment", {
            describe: "deployment name from the registry",
            type: "string",
            demandOption: true,
          })
          .option("from", {
            describe: "source chain",
            type: "string",
            choices: CHAIN_CHOICES,
            demandOption: true,
          })
          .option("to", {
            describe: "destination chain",
            type: "string",
            choices: CHAIN_CHOICES,
            demandOption: true,
          })
          .option("recipient", {
            describe:
              "EVM address (EIP-55), Solana address, or complete Canton party id <hint>::1220<64 hex>",
            type: "string",
          })
          .option("recipient-raw", {
            describe:
              "raw 32-byte hex recipient (needs --i-accept-raw-recipient)",
            type: "string",
          })
          .option("i-accept-raw-recipient", {
            describe: "accept --recipient-raw",
            type: "boolean",
            default: false,
          })
          .option("accept-unchecksummed-address", {
            describe: "accept an all-lowercase EVM address",
            type: "boolean",
            default: false,
          })
          .option("transfer-nonce", {
            describe: "Canton transfer nonce (u32) of a transfer to retry",
            type: "string",
          })
          .option("dry-run", DRY_RUN_OPTION)
          .option("yes", YES_OPTION)
          .option("deployments", DEPLOYMENTS_OPTION),
      (argv) =>
        runSend({
          deployments: argv.deployments,
          deployment: argv.deployment,
          amount: String(argv.amount),
          from: argv.from,
          to: argv.to,
          recipient: argv.recipient,
          recipientRaw: argv["recipient-raw"],
          acceptRawRecipient: argv["i-accept-raw-recipient"],
          acceptUnchecksummedAddress: argv["accept-unchecksummed-address"],
          transferNonce: argv["transfer-nonce"],
          dryRun: argv["dry-run"],
          yes: argv.yes,
        })
    )
    .command(
      "redeem",
      "Redeem a guardian-signed VAA on the private network",
      (y) =>
        y
          .option("deployment", {
            describe: "deployment name from the registry",
            type: "string",
            demandOption: true,
          })
          .option("vaa", {
            describe: "guardian-signed VAA as hex",
            type: "string",
            demandOption: true,
          })
          .option("recipient", {
            describe: "Canton recipient party id (Canton redeem only)",
            type: "string",
          })
          .option("dry-run", DRY_RUN_OPTION)
          .option("yes", YES_OPTION)
          .option("deployments", DEPLOYMENTS_OPTION),
      (argv) =>
        runRedeem({
          deployments: argv.deployments,
          deployment: argv.deployment,
          vaa: argv.vaa,
          recipient: argv.recipient,
          dryRun: argv["dry-run"],
          yes: argv.yes,
        })
    )
    .demandCommand();
}
