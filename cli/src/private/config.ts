// Private-deployment registry: types, loader, validation, chain-alias parsing.
// Deliberately SDK-free (no @wormhole-foundation/* imports) so this loads and
// tests before the Canton-aware SDK fork lands. Chain names are local string
// literals, not the SDK's Chain union.
import * as fs from "node:fs";
import * as path from "node:path";
import { fileURLToPath } from "node:url";

export type ChainName = "Canton" | "Ethereum" | "Solana";
export type Mode = "locking" | "burning";
export type DeploymentName = "canlock" | "canhub" | "ethlock" | "canburn";

export const DEPLOYMENT_NAMES: readonly DeploymentName[] = [
  "canlock",
  "canhub",
  "ethlock",
  "canburn",
];

export interface KeycloakConfig {
  url: string;
  realm: string;
  clientId: string;
}

export interface CantonConnection {
  jsonApi: string;
  keycloak: KeycloakConfig;
  namespace: string;
  parties: { user: string; operator: string; governance: string };
  guardianPubkey: string;
}

export interface CantonChainConfig {
  mode: Mode;
  decimals: number;
  instrument: string;
  manager: string; // 32-byte hex, no 0x
  emitter: string; // 32-byte hex, no 0x
}

export interface EthereumChainConfig {
  mode: Mode;
  decimals: number;
  token: string; // 0x + 40 hex
  manager: string; // 0x + 40 hex
  transceiver: string; // 0x + 40 hex
  emitter: string; // 32-byte hex, no 0x (padded transceiver address)
}

export interface SolanaChainConfig {
  mode: Mode;
  decimals: number;
  idlVersion: string;
  manager: string; // base58 program id
  mint: string; // base58
  transceiver: string; // base58
  emitter: string; // 32-byte hex, no 0x
}

export interface DeploymentChains {
  Canton: CantonChainConfig;
  Ethereum: EthereumChainConfig;
  Solana?: SolanaChainConfig;
}

export interface DeploymentConfig {
  chains: DeploymentChains;
}

export interface PrivateDeploymentsConfig {
  network: string;
  chainIds: Record<ChainName, number>;
  canton: CantonConnection;
  deployments: Record<DeploymentName, DeploymentConfig>;
}

/** Full Canton party id ("hint::namespace") for a registered party hint. */
export function partyId(canton: CantonConnection, hint: string): string {
  return `${hint}::${canton.namespace}`;
}

const CHAIN_ALIASES: Record<string, ChainName> = {
  canton: "Canton",
  ethereum: "Ethereum",
  eth: "Ethereum",
  solana: "Solana",
  sol: "Solana",
};

/** Case-insensitive chain alias -> SDK Chain name (canton/ethereum/eth/solana/sol). */
export function parseChainAlias(input: string): ChainName {
  const key = input.trim().toLowerCase();
  const chain = CHAIN_ALIASES[key];
  if (!chain) {
    throw new Error(
      `unknown chain "${input}"; expected one of canton, ethereum/eth, solana/sol`
    );
  }
  return chain;
}

// --- validation --------------------------------------------------------

const HEX32_RE = /^[0-9a-fA-F]{64}$/;
const EVM_ADDR_RE = /^0x[0-9a-fA-F]{40}$/;
const BASE58_RE = /^[1-9A-HJ-NP-Za-km-z]+$/;

function fail(context: string, msg: string): never {
  throw new Error(`private-deployments.json: ${context}: ${msg}`);
}

function expectString(v: unknown, context: string): string {
  if (typeof v !== "string" || v.length === 0) {
    fail(context, `expected a non-empty string, got ${JSON.stringify(v)}`);
  }
  return v as string;
}

function expectInt(v: unknown, context: string): number {
  if (typeof v !== "number" || !Number.isInteger(v)) {
    fail(context, `expected an integer, got ${JSON.stringify(v)}`);
  }
  return v as number;
}

function expectMode(v: unknown, context: string): Mode {
  if (v !== "locking" && v !== "burning") {
    fail(context, `expected mode "locking" or "burning", got ${JSON.stringify(v)}`);
  }
  return v as Mode;
}

function expectHex32(v: unknown, context: string): string {
  const s = expectString(v, context);
  if (!HEX32_RE.test(s)) {
    fail(context, `expected 32-byte hex (64 chars, no 0x), got "${s}"`);
  }
  return s;
}

function expectEvmAddress(v: unknown, context: string): string {
  const s = expectString(v, context);
  if (!EVM_ADDR_RE.test(s)) {
    fail(context, `expected a 0x-prefixed 20-byte address, got "${s}"`);
  }
  return s;
}

function expectBase58(v: unknown, context: string): string {
  const s = expectString(v, context);
  if (!BASE58_RE.test(s)) {
    fail(context, `expected a base58 address, got "${s}"`);
  }
  return s;
}

function expectObject(v: unknown, context: string): Record<string, unknown> {
  if (typeof v !== "object" || v === null || Array.isArray(v)) {
    fail(context, `expected an object, got ${JSON.stringify(v)}`);
  }
  return v as Record<string, unknown>;
}

function validateCantonChain(v: unknown, context: string): CantonChainConfig {
  const o = expectObject(v, context);
  return {
    mode: expectMode(o.mode, `${context}.mode`),
    decimals: expectInt(o.decimals, `${context}.decimals`),
    instrument: expectString(o.instrument, `${context}.instrument`),
    manager: expectHex32(o.manager, `${context}.manager`),
    emitter: expectHex32(o.emitter, `${context}.emitter`),
  };
}

function validateEthereumChain(v: unknown, context: string): EthereumChainConfig {
  const o = expectObject(v, context);
  return {
    mode: expectMode(o.mode, `${context}.mode`),
    decimals: expectInt(o.decimals, `${context}.decimals`),
    token: expectEvmAddress(o.token, `${context}.token`),
    manager: expectEvmAddress(o.manager, `${context}.manager`),
    transceiver: expectEvmAddress(o.transceiver, `${context}.transceiver`),
    emitter: expectHex32(o.emitter, `${context}.emitter`),
  };
}

function validateSolanaChain(v: unknown, context: string): SolanaChainConfig {
  const o = expectObject(v, context);
  return {
    mode: expectMode(o.mode, `${context}.mode`),
    decimals: expectInt(o.decimals, `${context}.decimals`),
    idlVersion: expectString(o.idlVersion, `${context}.idlVersion`),
    manager: expectBase58(o.manager, `${context}.manager`),
    mint: expectBase58(o.mint, `${context}.mint`),
    transceiver: expectBase58(o.transceiver, `${context}.transceiver`),
    emitter: expectHex32(o.emitter, `${context}.emitter`),
  };
}

function validateDeployment(v: unknown, name: string): DeploymentConfig {
  const o = expectObject(v, `deployments.${name}`);
  const chains = expectObject(o.chains, `deployments.${name}.chains`);
  if (!("Canton" in chains)) fail(`deployments.${name}.chains`, `missing chain "Canton"`);
  if (!("Ethereum" in chains)) fail(`deployments.${name}.chains`, `missing chain "Ethereum"`);
  const result: DeploymentChains = {
    Canton: validateCantonChain(chains.Canton, `deployments.${name}.chains.Canton`),
    Ethereum: validateEthereumChain(
      chains.Ethereum,
      `deployments.${name}.chains.Ethereum`
    ),
  };
  if ("Solana" in chains && chains.Solana !== undefined) {
    result.Solana = validateSolanaChain(
      chains.Solana,
      `deployments.${name}.chains.Solana`
    );
  }
  return { chains: result };
}

function validateCanton(v: unknown): CantonConnection {
  const o = expectObject(v, "canton");
  const keycloakO = expectObject(o.keycloak, "canton.keycloak");
  const partiesO = expectObject(o.parties, "canton.parties");
  return {
    jsonApi: expectString(o.jsonApi, "canton.jsonApi"),
    keycloak: {
      url: expectString(keycloakO.url, "canton.keycloak.url"),
      realm: expectString(keycloakO.realm, "canton.keycloak.realm"),
      clientId: expectString(keycloakO.clientId, "canton.keycloak.clientId"),
    },
    namespace: expectString(o.namespace, "canton.namespace"),
    parties: {
      user: expectString(partiesO.user, "canton.parties.user"),
      operator: expectString(partiesO.operator, "canton.parties.operator"),
      governance: expectString(
        partiesO.governance,
        "canton.parties.governance"
      ),
    },
    guardianPubkey: expectString(o.guardianPubkey, "canton.guardianPubkey"),
  };
}

/** Validates a parsed JSON value against the private-deployments schema. */
export function validateConfig(raw: unknown): PrivateDeploymentsConfig {
  const o = expectObject(raw, "<root>");
  const network = expectString(o.network, "network");
  const chainIdsO = expectObject(o.chainIds, "chainIds");
  const chainIds: Record<ChainName, number> = {
    Canton: expectInt(chainIdsO.Canton, "chainIds.Canton"),
    Ethereum: expectInt(chainIdsO.Ethereum, "chainIds.Ethereum"),
    Solana: expectInt(chainIdsO.Solana, "chainIds.Solana"),
  };
  const canton = validateCanton(o.canton);
  const deploymentsO = expectObject(o.deployments, "deployments");
  for (const name of DEPLOYMENT_NAMES) {
    if (!(name in deploymentsO)) {
      fail("deployments", `missing required deployment "${name}"`);
    }
  }
  const deployments: Record<DeploymentName, DeploymentConfig> = {} as Record<
    DeploymentName,
    DeploymentConfig
  >;
  for (const name of DEPLOYMENT_NAMES) {
    deployments[name] = validateDeployment(deploymentsO[name], name);
  }
  return { network, chainIds, canton, deployments };
}

// --- loader --------------------------------------------------------------

const CONFIG_FILENAME = "private-deployments.json";

function findUpward(startDir: string, filename: string): string | undefined {
  let dir = startDir;
  for (;;) {
    const candidate = path.join(dir, filename);
    if (fs.existsSync(candidate)) return candidate;
    const parent = path.dirname(dir);
    if (parent === dir) return undefined;
    dir = parent;
  }
}

/**
 * Resolves the config path with precedence: explicit path arg ->
 * NTT_PRIVATE_DEPLOYMENTS env -> ./private-deployments.json in CWD -> walk
 * upward from this module's directory (covers running from cli/ or repo root).
 */
export function resolveConfigPath(explicitPath?: string): string {
  if (explicitPath) return explicitPath;
  const envPath = process.env.NTT_PRIVATE_DEPLOYMENTS;
  if (envPath) return envPath;
  const cwdPath = path.resolve(process.cwd(), CONFIG_FILENAME);
  if (fs.existsSync(cwdPath)) return cwdPath;
  const moduleDir = path.dirname(fileURLToPath(import.meta.url));
  const found = findUpward(moduleDir, CONFIG_FILENAME);
  if (found) return found;
  throw new Error(
    `${CONFIG_FILENAME} not found: pass --deployments, set NTT_PRIVATE_DEPLOYMENTS, ` +
      `or place it in ${process.cwd()} or an ancestor of ${moduleDir}`
  );
}

/** Loads and validates the private-deployments registry. */
export function loadConfig(explicitPath?: string): PrivateDeploymentsConfig {
  const configPath = resolveConfigPath(explicitPath);
  let text: string;
  try {
    text = fs.readFileSync(configPath, "utf8");
  } catch (err) {
    throw new Error(`failed to read ${configPath}: ${(err as Error).message}`);
  }
  let raw: unknown;
  try {
    raw = JSON.parse(text);
  } catch (err) {
    throw new Error(`failed to parse ${configPath}: ${(err as Error).message}`);
  }
  return validateConfig(raw);
}
