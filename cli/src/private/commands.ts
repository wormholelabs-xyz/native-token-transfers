// yargs registration for `ntt private ...`. Reuses config.ts/binding.ts/
// amounts.ts/vaa.ts plus the per-chain drivers (canton-ops.ts, evm.ts,
// solana.ts) — this file is glue: argument parsing, recipient defaulting,
// confirmation printing, dispatch.
import type { Argv } from "yargs";
import chalk from "chalk";

import {
  DEPLOYMENT_NAMES,
  loadConfig,
  partyId,
  parseChainAlias,
  type CantonConnection,
  type ChainName,
  type DeploymentConfig,
  type DeploymentName,
  type PrivateDeploymentsConfig,
} from "./config.js";
import { base58ToHex32, pad32, recipientBinding, strip0x } from "./binding.js";
import { checkDust, fromRaw } from "./amounts.js";
import { parseTransferVaa, validateAgainstDeployment } from "./vaa.js";
import { CantonClient } from "./canton/client.js";
import { cantonRedeem, cantonSend } from "./canton-ops.js";
import { deriveEthAddress, evmRedeem, evmSend } from "./evm.js";
import { deriveSolanaAddress, solanaRedeem, solanaSend } from "./solana.js";

const CHAIN_CHOICES = ["canton", "ethereum", "eth", "solana", "sol"] as const;

// --- recipient resolution (pure, unit-testable) ---------------------------

export interface RecipientDefaults {
  ethereumAddress?: string;
  solanaPubkey?: string;
}

export interface ResolvedRecipient {
  recipient32Hex: string; // no 0x, lowercase
  description: string;
  cantonParty?: string; // set only when destChain === "Canton" and not a raw override
  dangerousRaw: boolean;
}

/**
 * Recipient defaulting table: Ethereum -> the sending wallet's own address;
 * Solana -> the sending payer's own pubkey; Canton -> the registry user
 * party, always run through recipientBinding() (never accepted raw without
 * --recipient-raw, since a wrong binding strands funds permanently).
 */
export function resolveSendRecipient(
  destChain: ChainName,
  canton: CantonConnection,
  opts: { recipientRaw?: string; recipient?: string },
  defaults: RecipientDefaults
): ResolvedRecipient {
  if (opts.recipientRaw) {
    const hex = strip0x(opts.recipientRaw);
    if (!/^[0-9a-f]{64}$/.test(hex)) {
      throw new Error(
        `--recipient-raw must be exactly 32 bytes (64 hex chars), got "${opts.recipientRaw}"`
      );
    }
    return { recipient32Hex: hex, description: `raw bytes32 0x${hex}`, dangerousRaw: true };
  }

  if (destChain === "Canton") {
    const hint = opts.recipient ?? canton.parties.user;
    const party = partyId(canton, hint);
    const binding = strip0x(recipientBinding(party));
    return {
      recipient32Hex: binding,
      description: `Canton party ${party} -> binding 0x${binding}`,
      cantonParty: party,
      dangerousRaw: false,
    };
  }

  if (destChain === "Ethereum") {
    const addr = opts.recipient ?? defaults.ethereumAddress;
    if (!addr) {
      throw new Error("no Ethereum recipient: pass --recipient <address> or set ETH_PRIVATE_KEY");
    }
    return { recipient32Hex: strip0x(pad32(addr)), description: `Ethereum address ${addr}`, dangerousRaw: false };
  }

  const pubkey = opts.recipient ?? defaults.solanaPubkey;
  if (!pubkey) {
    throw new Error(
      "no Solana recipient: pass --recipient <pubkey> or set SOLANA_PRIVATE_KEY/SOLANA_PAYER_PATH"
    );
  }
  return {
    recipient32Hex: base58ToHex32(pubkey),
    description: `Solana pubkey ${pubkey}`,
    dangerousRaw: false,
  };
}

/** The Canton party + binding that a redeem's --recipient resolves to. */
export function resolveRedeemCantonRecipient(
  canton: CantonConnection,
  recipientHint: string | undefined
): { partyHint: string; party: string; binding: string } {
  const partyHint = recipientHint ?? canton.parties.user;
  const party = partyId(canton, partyHint);
  return { partyHint, party, binding: strip0x(recipientBinding(party)) };
}

// --- helpers ---------------------------------------------------------------

function requireDeployment(cfg: PrivateDeploymentsConfig, name: string): DeploymentName {
  if (!(DEPLOYMENT_NAMES as readonly string[]).includes(name)) {
    throw new Error(`unknown deployment "${name}"; try one of ${DEPLOYMENT_NAMES.join(", ")}`);
  }
  return name as DeploymentName;
}

/** Decimals for a chain leg of a deployment; throws if the leg doesn't exist (e.g. no Solana). */
function decimalsFor(dep: DeploymentConfig, chain: ChainName): number {
  if (chain === "Canton") return dep.chains.Canton.decimals;
  if (chain === "Ethereum") return dep.chains.Ethereum.decimals;
  if (!dep.chains.Solana) throw new Error(`deployment has no Solana leg`);
  return dep.chains.Solana.decimals;
}

function buildCantonClient(cfg: PrivateDeploymentsConfig): CantonClient {
  const secret = process.env.CANTON_KEYCLOAK_CLIENT_SECRET;
  if (!secret) throw new Error("CANTON_KEYCLOAK_CLIENT_SECRET env var not set");
  return new CantonClient({
    jsonApi: cfg.canton.jsonApi,
    keycloak: cfg.canton.keycloak,
    clientSecret: secret,
  });
}

function loadRecipientDefaults(): RecipientDefaults {
  const defaults: RecipientDefaults = {};
  try {
    defaults.ethereumAddress = deriveEthAddress();
  } catch {
    // ETH_PRIVATE_KEY not set — fine unless the send targets Ethereum without --recipient.
  }
  try {
    defaults.solanaPubkey = deriveSolanaAddress();
  } catch {
    // SOLANA_PRIVATE_KEY/SOLANA_PAYER_PATH not set — fine unless the send targets Solana without --recipient.
  }
  return defaults;
}

const NEXT_STEPS_FINALITY_NOTE =
  "Ethereum VAAs sign after ~13 min finality; Canton immediate; Solana ~13 s";

function printNextSteps(cfg: PrivateDeploymentsConfig, sourceChain: ChainName, sequence: string, deploymentName: string): void {
  console.log("");
  console.log("next steps:");
  console.log(`  get_vaa ${cfg.chainIds[sourceChain]} ${sequence}   # on the guardian VM`);
  console.log(`  ntt private redeem --deployment ${deploymentName} --vaa <paste>`);
  console.log(`  (${NEXT_STEPS_FINALITY_NOTE})`);
}

// --- list --------------------------------------------------------------

function runList(cfg: PrivateDeploymentsConfig): void {
  console.log(chalk.bold(`private deployments (network: ${cfg.network})`));
  for (const name of DEPLOYMENT_NAMES) {
    const dep = cfg.deployments[name];
    const legs: string[] = [];
    legs.push(`Canton(${dep.chains.Canton.mode},d${dep.chains.Canton.decimals})`);
    legs.push(`Ethereum(${dep.chains.Ethereum.mode},d${dep.chains.Ethereum.decimals})`);
    if (dep.chains.Solana) {
      legs.push(`Solana(${dep.chains.Solana.mode},d${dep.chains.Solana.decimals})`);
    }
    console.log(`  ${chalk.cyan(name.padEnd(8))} ${legs.join("  <->  ")}`);
  }
}

// --- send --------------------------------------------------------------

async function runSend(
  cfg: PrivateDeploymentsConfig,
  deploymentName: DeploymentName,
  amountHuman: string,
  fromChain: ChainName,
  toChain: ChainName,
  recipientOpt: string | undefined,
  recipientRawOpt: string | undefined,
  dryRun: boolean
): Promise<void> {
  const dep = cfg.deployments[deploymentName];
  if (fromChain === toChain) throw new Error("--from and --to must be different chains");

  checkDust(amountHuman, decimalsFor(dep, fromChain), decimalsFor(dep, toChain));

  const resolved = resolveSendRecipient(
    toChain,
    cfg.canton,
    { recipient: recipientOpt, recipientRaw: recipientRawOpt },
    loadRecipientDefaults()
  );

  console.log(`sending ${amountHuman} on ${deploymentName}: ${fromChain} -> ${toChain}`);
  console.log(`recipient: ${resolved.description}`);
  if (resolved.dangerousRaw && toChain === "Canton") {
    console.log(
      chalk.bold.red(
        "  WARNING: raw bytes32 recipient bypasses the derived Canton binding. " +
          "A wrong value strands funds permanently."
      )
    );
  }
  if (dryRun) console.log(chalk.yellow("--dry-run: nothing will be submitted"));

  let sequence: string | undefined;

  if (fromChain === "Canton") {
    const client = buildCantonClient(cfg);
    const result = await cantonSend(
      client,
      cfg.canton,
      dep.chains.Canton,
      amountHuman,
      cfg.chainIds[toChain],
      resolved.recipient32Hex,
      { dryRun }
    );
    if (result) {
      sequence = result.sequence;
      console.log(`sent: canton sequence ${sequence}`);
    }
  } else if (fromChain === "Ethereum") {
    const result = await evmSend(cfg, dep, amountHuman, toChain, resolved.recipient32Hex, { dryRun });
    if (result) {
      sequence = result.sequence;
      console.log(`sent: tx ${result.txHash}${sequence ? `, sequence ${sequence}` : " (sequence not found in receipt logs)"}`);
    }
  } else {
    const result = await solanaSend(cfg, dep, amountHuman, toChain, resolved.recipient32Hex, { dryRun });
    if (result) {
      sequence = result.sequence;
      console.log(`sent: tx ${result.signature}${sequence ? `, sequence ${sequence}` : " (sequence not found in tx logs)"}`);
    }
  }

  if (!dryRun && sequence) printNextSteps(cfg, fromChain, sequence, deploymentName);
}

// --- redeem --------------------------------------------------------------

async function runRedeem(
  cfg: PrivateDeploymentsConfig,
  deploymentName: DeploymentName,
  vaaHex: string,
  recipientOpt: string | undefined,
  dryRun: boolean
): Promise<void> {
  const dep = cfg.deployments[deploymentName];
  const parsed = parseTransferVaa(vaaHex);
  const { sourceChain, destinationChain } = validateAgainstDeployment(parsed, cfg, deploymentName);

  const amountHuman = fromRaw(parsed.trimmedAmount.amount, parsed.trimmedAmount.decimals);
  console.log(
    `redeeming ${deploymentName}: ${sourceChain} -> ${destinationChain}, ` +
      `amount ${amountHuman} (sequence ${parsed.sequence})`
  );
  if (dryRun) console.log(chalk.yellow("--dry-run: nothing will be submitted"));

  if (destinationChain !== "Canton" && recipientOpt) {
    console.log(chalk.yellow(`note: --recipient is ignored for a ${destinationChain} redeem (Canton leg only)`));
  }

  if (destinationChain === "Canton") {
    const { partyHint, party, binding } = resolveRedeemCantonRecipient(cfg.canton, recipientOpt);
    console.log(`  canton party: ${party}`);
    console.log(`  binding:      0x${binding}`);
    if (binding !== parsed.recipientAddress) {
      throw new Error(
        `the VAA's embedded recipient (0x${parsed.recipientAddress}) does not equal ` +
          `recipientBinding(${party}) = 0x${binding}; pass the correct --recipient or you will ` +
          `attempt to release/mint to the wrong party (and likely fail authorization)`
      );
    }
    const client = buildCantonClient(cfg);
    const result = await cantonRedeem(client, cfg.canton, dep.chains.Canton, vaaHex, partyHint, { dryRun });
    if (result) {
      for (const coin of result.minted) {
        console.log(`  minted/released ${coin.amount} ${coin.instrumentId} to ${coin.owner.split("::")[0]}`);
      }
      if (result.minted.length === 0) console.log("  (no Coin creations found in the transaction tree)");
    }
  } else if (destinationChain === "Ethereum") {
    const result = await evmRedeem(cfg, dep, parsed.bytes, { dryRun });
    if (result) console.log(`redeemed: tx ${result.txHash}`);
  } else {
    const result = await solanaRedeem(cfg, dep, parsed.vaa, { dryRun });
    if (result?.signature) console.log(`redeemed: tx ${result.signature}`);
  }
}

// --- registration --------------------------------------------------------

export function registerPrivateCommands(yargs: Argv): Argv {
  return yargs
    .command(
      "list",
      "List configured private deployments",
      (y) => y,
      (argv) => {
        const cfg = loadConfig(argv["deployments"] as string | undefined);
        runList(cfg);
      }
    )
    .command(
      "send <amount>",
      "Send an NTT transfer on the private network",
      (y) =>
        y
          .positional("amount", {
            describe: "amount in human units (e.g. 1.5)",
            type: "string",
            demandOption: true,
          })
          .option("deployment", {
            describe: "deployment name",
            type: "string",
            choices: DEPLOYMENT_NAMES,
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
              "recipient override: EVM address, Solana base58 pubkey, or Canton party hint",
            type: "string",
          })
          .option("recipient-raw", {
            describe:
              "raw 32-byte hex recipient, bypassing all defaulting/binding (DANGEROUS on Canton)",
            type: "string",
          })
          .option("dry-run", {
            describe: "simulate only; submit/send nothing",
            type: "boolean",
            default: false,
          })
          .option("deployments", {
            describe: "path to private-deployments.json (overrides NTT_PRIVATE_DEPLOYMENTS/CWD lookup)",
            type: "string",
          }),
      async (argv) => {
        const cfg = loadConfig(argv["deployments"] as string | undefined);
        const deploymentName = requireDeployment(cfg, argv["deployment"] as string);
        const fromChain = parseChainAlias(argv["from"] as string);
        const toChain = parseChainAlias(argv["to"] as string);
        await runSend(
          cfg,
          deploymentName,
          String(argv["amount"]),
          fromChain,
          toChain,
          argv["recipient"] as string | undefined,
          argv["recipient-raw"] as string | undefined,
          Boolean(argv["dry-run"])
        );
      }
    )
    .command(
      "redeem",
      "Redeem a pasted VAA on the private network",
      (y) =>
        y
          .option("deployment", {
            describe: "deployment name",
            type: "string",
            choices: DEPLOYMENT_NAMES,
            demandOption: true,
          })
          .option("vaa", {
            describe: "guardian-signed VAA hex, copied from the guardian VM",
            type: "string",
            demandOption: true,
          })
          .option("recipient", {
            describe: "Canton-leg recipient party hint (ignored for Ethereum/Solana destinations)",
            type: "string",
          })
          .option("dry-run", {
            describe: "simulate only; submit/send nothing",
            type: "boolean",
            default: false,
          })
          .option("deployments", {
            describe: "path to private-deployments.json (overrides NTT_PRIVATE_DEPLOYMENTS/CWD lookup)",
            type: "string",
          }),
      async (argv) => {
        const cfg = loadConfig(argv["deployments"] as string | undefined);
        const deploymentName = requireDeployment(cfg, argv["deployment"] as string);
        await runRedeem(
          cfg,
          deploymentName,
          argv["vaa"] as string,
          argv["recipient"] as string | undefined,
          Boolean(argv["dry-run"])
        );
      }
    )
    .demandCommand();
}
