// Private-deployments registry, schema version 2: types, path rules, loader.
import { createHash } from "node:crypto";
import { toChainId } from "@wormhole-foundation/sdk-base";
import * as fs from "node:fs";
import * as path from "node:path";
import { fileURLToPath } from "node:url";

import {
  checkOnlyKnownKeys,
  failRegistryField,
  parseEndpointUrl,
  readNonEmptyString,
  readObject,
  readWithParser,
} from "./registry-fields.js";
import {
  parseCantonPackageId,
  parseCantonPartyId,
  parseDeploymentName,
  parseEvmAddress,
  parseHex32,
  parseSolanaAddressBytes32,
  parseTokenDecimals,
  parseWormholeChainId,
  type CantonPackageId,
  type CantonPartyId,
  type DeploymentName,
  type EvmAddress,
  type Hex32,
  type TokenDecimals,
  type WormholeChainId,
} from "./types.js";

export const REGISTRY_SCHEMA_VERSION = 2;
export const REGISTRY_FILE_BYTES_MAX = 1024 * 1024;
export const DEPLOYMENT_COUNT_MAX = 64;
/** A Canton amount is a Daml Decimal: 10 fractional digits */
export const CANTON_TOKEN_DECIMALS_MAX = 10;

export const CHAIN_NAMES = ["Canton", "Ethereum", "Solana"] as const;
export type ChainName = (typeof CHAIN_NAMES)[number];
export type TokenMode = "locking" | "burning";
export type NetworkName = "Mainnet" | "Testnet";

export interface CantonInstrumentId {
  admin: CantonPartyId;
  id: string;
}

export interface CantonVettedPackageIds {
  ntt: CantonPackageId;
  core: CantonPackageId;
}

export interface CantonChainConfig {
  mode: TokenMode;
  decimals: TokenDecimals;
  manager: Hex32;
  emitter: Hex32;
  instrument: CantonInstrumentId;
  reserveCustodian: CantonPartyId;
  /** The core's guardianGovernance party: it signs every NTT manager (the SDK's contracts.coreBridge) */
  guardianGovernance: CantonPartyId;
  vettedPackageIds: CantonVettedPackageIds;
}

export interface EthereumChainConfig {
  mode: TokenMode;
  decimals: TokenDecimals;
  token: EvmAddress;
  manager: EvmAddress;
  transceiver: EvmAddress;
  emitter: Hex32;
}

export interface SolanaChainConfig {
  mode: TokenMode;
  decimals: TokenDecimals;
  idlVersion: string;
  coreBridge: string;
  manager: string;
  mint: string;
  transceiver: string;
  managerHex32: Hex32;
  emitter: Hex32;
}

export interface DeploymentChains {
  Canton?: CantonChainConfig;
  Ethereum?: EthereumChainConfig;
  Solana?: SolanaChainConfig;
}

export interface Deployment {
  name: DeploymentName;
  chains: DeploymentChains;
}

export interface CantonParticipantConfig {
  jsonApiUrl: URL;
  oauthTokenUrl: URL;
  oauthClientId: string;
  userPartyId: CantonPartyId;
}

export interface CantonConnectionConfig {
  disclosureServiceUrl: URL;
  participant?: CantonParticipantConfig;
}

export interface RegistrySource {
  absolutePath: string;
  sha256Hex: string;
  isRepositoryFile: boolean;
}

export interface PrivateRegistry {
  network: NetworkName;
  chainIds: Record<ChainName, WormholeChainId>;
  canton: CantonConnectionConfig;
  deployments: ReadonlyMap<DeploymentName, Deployment>;
}

function readTokenMode(value: unknown, context: string): TokenMode {
  if (value !== "locking" && value !== "burning") {
    failRegistryField(
      context,
      `expected "locking" or "burning", got ${JSON.stringify(value)}`
    );
  }
  return value;
}

function readCantonChain(value: unknown, context: string): CantonChainConfig {
  const chain = readObject(value, context);
  checkOnlyKnownKeys(
    chain,
    [
      "mode",
      "decimals",
      "manager",
      "emitter",
      "instrument",
      "reserveCustodian",
      "guardianGovernance",
      "vettedPackageIds",
    ],
    context
  );
  const instrument = readObject(chain.instrument, `${context}.instrument`);
  checkOnlyKnownKeys(instrument, ["admin", "id"], `${context}.instrument`);
  const packageIds = readObject(
    chain.vettedPackageIds,
    `${context}.vettedPackageIds`
  );
  checkOnlyKnownKeys(
    packageIds,
    ["ntt", "core"],
    `${context}.vettedPackageIds`
  );
  const decimals = readWithParser(
    chain.decimals,
    `${context}.decimals`,
    parseTokenDecimals
  );
  if (decimals > CANTON_TOKEN_DECIMALS_MAX)
    failRegistryField(
      `${context}.decimals`,
      `a Canton amount is a Daml Decimal with at most ${CANTON_TOKEN_DECIMALS_MAX} decimals, got ${decimals}`
    );
  return {
    mode: readTokenMode(chain.mode, `${context}.mode`),
    decimals,
    manager: readWithParser(chain.manager, `${context}.manager`, parseHex32),
    emitter: readWithParser(chain.emitter, `${context}.emitter`, parseHex32),
    instrument: {
      admin: readWithParser(
        instrument.admin,
        `${context}.instrument.admin`,
        parseCantonPartyId
      ),
      id: readNonEmptyString(instrument.id, `${context}.instrument.id`),
    },
    reserveCustodian: readWithParser(
      chain.reserveCustodian,
      `${context}.reserveCustodian`,
      parseCantonPartyId
    ),
    guardianGovernance: readWithParser(
      readNonEmptyString(
        chain.guardianGovernance,
        `${context}.guardianGovernance`
      ),
      `${context}.guardianGovernance`,
      parseCantonPartyId
    ),
    vettedPackageIds: {
      ntt: readWithParser(
        packageIds.ntt,
        `${context}.vettedPackageIds.ntt`,
        parseCantonPackageId
      ),
      core: readWithParser(
        packageIds.core,
        `${context}.vettedPackageIds.core`,
        parseCantonPackageId
      ),
    },
  };
}

function readEthereumChain(
  value: unknown,
  context: string
): EthereumChainConfig {
  const chain = readObject(value, context);
  checkOnlyKnownKeys(
    chain,
    ["mode", "decimals", "token", "manager", "transceiver", "emitter"],
    context
  );
  return {
    mode: readTokenMode(chain.mode, `${context}.mode`),
    decimals: readWithParser(
      chain.decimals,
      `${context}.decimals`,
      parseTokenDecimals
    ),
    token: readWithParser(chain.token, `${context}.token`, parseEvmAddress),
    manager: readWithParser(
      chain.manager,
      `${context}.manager`,
      parseEvmAddress
    ),
    transceiver: readWithParser(
      chain.transceiver,
      `${context}.transceiver`,
      parseEvmAddress
    ),
    emitter: readWithParser(chain.emitter, `${context}.emitter`, parseHex32),
  };
}

function readSolanaAddress(value: unknown, context: string): string {
  const base58Text = readNonEmptyString(value, context, 44);
  readWithParser(base58Text, context, parseSolanaAddressBytes32);
  return base58Text;
}

function readSolanaChain(value: unknown, context: string): SolanaChainConfig {
  const chain = readObject(value, context);
  checkOnlyKnownKeys(
    chain,
    [
      "mode",
      "decimals",
      "idlVersion",
      "coreBridge",
      "manager",
      "mint",
      "transceiver",
      "emitter",
    ],
    context
  );
  const manager = readSolanaAddress(chain.manager, `${context}.manager`);
  return {
    mode: readTokenMode(chain.mode, `${context}.mode`),
    decimals: readWithParser(
      chain.decimals,
      `${context}.decimals`,
      parseTokenDecimals
    ),
    idlVersion: readNonEmptyString(
      chain.idlVersion,
      `${context}.idlVersion`,
      16
    ),
    coreBridge: readSolanaAddress(chain.coreBridge, `${context}.coreBridge`),
    manager,
    mint: readSolanaAddress(chain.mint, `${context}.mint`),
    transceiver: readSolanaAddress(chain.transceiver, `${context}.transceiver`),
    managerHex32: parseSolanaAddressBytes32(manager),
    emitter: readWithParser(chain.emitter, `${context}.emitter`, parseHex32),
  };
}

function readDeployment(value: unknown, name: DeploymentName): Deployment {
  const context = `deployments.${name}`;
  const deployment = readObject(value, context);
  checkOnlyKnownKeys(deployment, ["chains"], context);
  const chainsObject = readObject(deployment.chains, `${context}.chains`);
  checkOnlyKnownKeys(chainsObject, CHAIN_NAMES, `${context}.chains`);
  const chains: DeploymentChains = {};
  if (chainsObject.Canton !== undefined)
    chains.Canton = readCantonChain(
      chainsObject.Canton,
      `${context}.chains.Canton`
    );
  if (chainsObject.Ethereum !== undefined) {
    chains.Ethereum = readEthereumChain(
      chainsObject.Ethereum,
      `${context}.chains.Ethereum`
    );
  }
  if (chainsObject.Solana !== undefined)
    chains.Solana = readSolanaChain(
      chainsObject.Solana,
      `${context}.chains.Solana`
    );
  if (Object.keys(chains).length < 2)
    failRegistryField(
      `${context}.chains`,
      "a deployment needs at least two chains"
    );
  return { name, chains };
}

function readCantonConnection(value: unknown): CantonConnectionConfig {
  const canton = readObject(value, "canton");
  checkOnlyKnownKeys(canton, ["disclosureServiceUrl", "participant"], "canton");
  const disclosureServiceUrl = parseEndpointUrl(
    readNonEmptyString(
      canton.disclosureServiceUrl,
      "canton.disclosureServiceUrl"
    ),
    "canton.disclosureServiceUrl"
  );
  if (canton.participant === undefined) return { disclosureServiceUrl };
  const participant = readObject(canton.participant, "canton.participant");
  checkOnlyKnownKeys(
    participant,
    ["jsonApiUrl", "oauthTokenUrl", "oauthClientId", "userPartyId"],
    "canton.participant"
  );
  return {
    disclosureServiceUrl,
    participant: {
      jsonApiUrl: parseEndpointUrl(
        readNonEmptyString(
          participant.jsonApiUrl,
          "canton.participant.jsonApiUrl"
        ),
        "canton.participant.jsonApiUrl"
      ),
      oauthTokenUrl: parseEndpointUrl(
        readNonEmptyString(
          participant.oauthTokenUrl,
          "canton.participant.oauthTokenUrl"
        ),
        "canton.participant.oauthTokenUrl"
      ),
      oauthClientId: readNonEmptyString(
        participant.oauthClientId,
        "canton.participant.oauthClientId"
      ),
      userPartyId: readWithParser(
        participant.userPartyId,
        "canton.participant.userPartyId",
        parseCantonPartyId
      ),
    },
  };
}

function readNetwork(value: unknown): NetworkName {
  if (value !== "Mainnet" && value !== "Testnet") {
    failRegistryField(
      "network",
      `expected "Mainnet" or "Testnet", got ${JSON.stringify(value)}`
    );
  }
  return value;
}

/** Validates a parsed JSON value against schema version 2. */
export function validateRegistry(raw: unknown): PrivateRegistry {
  const root = readObject(raw, "<root>");
  if (root.schemaVersion !== REGISTRY_SCHEMA_VERSION) {
    failRegistryField(
      "schemaVersion",
      `must be ${REGISTRY_SCHEMA_VERSION}, got ${JSON.stringify(root.schemaVersion)}`
    );
  }
  checkOnlyKnownKeys(
    root,
    ["schemaVersion", "network", "chainIds", "canton", "deployments"],
    "<root>"
  );
  const chainIdsObject = readObject(root.chainIds, "chainIds");
  checkOnlyKnownKeys(chainIdsObject, CHAIN_NAMES, "chainIds");
  const chainIds = {} as Record<ChainName, WormholeChainId>;
  for (const chain of CHAIN_NAMES) {
    chainIds[chain] = readWithParser(
      chainIdsObject[chain],
      `chainIds.${chain}`,
      parseWormholeChainId
    );
  }
  const network = readNetwork(root.network);
  for (const chain of CHAIN_NAMES) {
    const sdkChain = sdkChainForRegistryChain(network, chain);
    if (chainIds[chain] !== toChainId(sdkChain)) {
      failRegistryField(
        `chainIds.${chain}`,
        `${chainIds[chain]} differs from the SDK chain id ${toChainId(sdkChain)} of ${sdkChain} on ${network}`
      );
    }
  }
  if (new Set(Object.values(chainIds)).size !== CHAIN_NAMES.length)
    failRegistryField("chainIds", "chain ids are not unique");

  const deploymentsObject = readObject(root.deployments, "deployments");
  const deploymentNames = Object.keys(deploymentsObject);
  if (deploymentNames.length > DEPLOYMENT_COUNT_MAX) {
    failRegistryField(
      "deployments",
      `more than ${DEPLOYMENT_COUNT_MAX} deployments`
    );
  }
  const deployments = new Map<DeploymentName, Deployment>();
  for (const rawName of deploymentNames) {
    const name = readWithParser(
      rawName,
      `deployments.${rawName}`,
      parseDeploymentName
    );
    deployments.set(name, readDeployment(deploymentsObject[rawName], name));
  }
  return {
    network,
    chainIds,
    canton: readCantonConnection(root.canton),
    deployments,
  };
}

/** SDK chain of a registry chain name: the Ethereum leg is Sepolia on Testnet. */
export function sdkChainForRegistryChain(
  network: NetworkName,
  chain: ChainName
): "Canton" | "Ethereum" | "Sepolia" | "Solana" {
  if (chain === "Ethereum" && network === "Testnet") return "Sepolia";
  return chain;
}

export function requireDeployment(
  registry: PrivateRegistry,
  name: string
): Deployment {
  const deployment = registry.deployments.get(name as DeploymentName);
  if (!deployment) {
    const known = [...registry.deployments.keys()];
    throw new Error(
      `unknown deployment "${name}"; the registry has ${known.length === 0 ? "no deployments" : known.join(", ")}`
    );
  }
  return deployment;
}

export function chainNameForId(
  registry: PrivateRegistry,
  chainId: WormholeChainId
): ChainName {
  const chain = CHAIN_NAMES.find((name) => registry.chainIds[name] === chainId);
  if (!chain)
    throw new Error(
      `Wormhole chain id ${chainId} is not in the registry chainIds`
    );
  return chain;
}

export const REGISTRY_FILENAME = "private-deployments.json";

/** <repository>/private-deployments.json, at a fixed depth from cli/src/private/. */
export const REPOSITORY_REGISTRY_PATH = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  "..",
  "..",
  "..",
  REGISTRY_FILENAME
);

/** Order: explicit path, then NTT_PRIVATE_DEPLOYMENTS, then the repository file. */
export function resolveRegistryPath(
  explicitPath: string | undefined,
  environmentPath: string | undefined
): string {
  if (explicitPath) return path.resolve(explicitPath);
  if (environmentPath) return path.resolve(environmentPath);
  return REPOSITORY_REGISTRY_PATH;
}

export function loadRegistry(
  explicitPath: string | undefined,
  environmentPath: string | undefined = process.env.NTT_PRIVATE_DEPLOYMENTS
): { registry: PrivateRegistry; source: RegistrySource } {
  const absolutePath = resolveRegistryPath(explicitPath, environmentPath);
  const fileBytes = fs.readFileSync(absolutePath);
  if (fileBytes.length > REGISTRY_FILE_BYTES_MAX) {
    throw new Error(
      `${absolutePath} is larger than ${REGISTRY_FILE_BYTES_MAX} bytes`
    );
  }
  let raw: unknown;
  try {
    raw = JSON.parse(fileBytes.toString("utf8"));
  } catch (err) {
    throw new Error(
      `failed to parse ${absolutePath}: ${(err as Error).message}`
    );
  }
  return {
    registry: validateRegistry(raw),
    source: {
      absolutePath,
      sha256Hex: createHash("sha256").update(fileBytes).digest("hex"),
      isRepositoryFile: absolutePath === REPOSITORY_REGISTRY_PATH,
    },
  };
}
