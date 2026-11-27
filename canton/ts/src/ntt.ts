import type { Chain, Network } from "@wormhole-foundation/sdk-base";
import { encoding, toChainId } from "@wormhole-foundation/sdk-base";
import type {
  AccountAddress,
  ChainAddress,
  ChainsConfig,
  Contracts,
  TxHash,
  WormholeMessageId,
} from "@wormhole-foundation/sdk-definitions";
import {
  UniversalAddress,
  serialize,
  toUniversal,
} from "@wormhole-foundation/sdk-definitions";
import type {
  Ntt,
  NttTransceiver,
} from "@wormhole-foundation/sdk-definitions-ntt";
import type {
  AnyCantonAddress,
  CantonChains,
  CantonCommand,
  CantonCommandEnvelope,
  CantonLedgerClient,
  CantonPlatformType,
  CorePackageIds,
  DisclosedContract,
  FetchFunction,
  JsTransaction,
  TransactionReaders,
} from "@wormhole-foundation/sdk-canton";
import {
  CantonAddress,
  CantonUnsignedTransaction,
  buildCommandEnvelope,
  createArgumentToJson,
  disclosedContractFromActive,
  emitterAddress,
  parseCantonContractId,
  parseCantonPartyId,
  parseCantonTemplateId,
  partyAddress,
  pinnedCorePackageIds,
  readCreatedEventBlob,
  readHoldings,
  selectInputHoldings,
  DAML_DECIMAL_SCALE,
} from "@wormhole-foundation/sdk-canton";
import { extractPublishedMessages } from "@wormhole-foundation/sdk-canton-core";
import type {
  FlowContext,
  NttAnchorPolicy,
  VerifiedNttFlow,
} from "./anchor.js";
import {
  NttAnchorError,
  checkAdminTransferProposal,
  checkManagerContract,
  flowCoreState,
  flowDisclosures,
  flowLockedLedger,
  flowNeedsTransferPreapproval,
  flowReplayNodeRecordsDigest,
  optionalRoleContractId,
  roleContractId,
  verifyFlow,
} from "./anchor.js";
import type { CantonTrimmedAmount } from "./amounts.js";
import { checkTransferAmount, untrimAmount } from "./amounts.js";
import {
  proposeAdminCommandId,
  redeemCommandId,
  transferCommandId,
  vaaDigest,
} from "./commandIds.js";
import type { NttPackageIds } from "./constants.js";
import {
  FLOW_REBUILD_DELAY_MS,
  REDEEM_FLOW_READS,
  NTT_PACKAGE_NAME,
  TRANSFER_CONSISTENCY_LEVEL,
  nttFlowServiceUrls,
  pinnedNttPackageIds,
} from "./constants.js";
import type {
  NttFlowName,
  NttFlowSource,
  NttManagerSummary,
} from "./flowSource.js";
import { HttpNttFlowSource, NttFlowServiceError } from "./flowSource.js";
import type {
  AdminTransferProposalView,
  NttManagerView,
  NttPeerView,
  NttTokenConfig,
} from "./nttContracts.js";
import {
  FLOW_ROLES,
  NTT_CHOICES,
  NTT_MANAGER_LAYOUT,
  NTT_TEMPLATES,
  acceptAdminTransferArgument,
  adminTransferProposalArgument,
  decodeNttManager,
  decodeTransferResult,
  mintArgument,
  releaseArgument,
  setPausedArgument,
  setPeerArgument,
  transferArgument,
} from "./nttContracts.js";
import { CantonNttWormholeTransceiver } from "./transceiver.js";

/** Canton settings of an NTT deployment, beside the generic NTT contracts (plan 3.2) */
export interface CantonNttSettings {
  /**
   * The party that owns the lock/unlock reserve: the admin, or nttCustody when gg is admin. The
   * anchor checks pin it (plan 3.5, layer 3); a transfer needs it.
   */
  reserveCustodian?: string;
  /** Base URL of the NTT flow service; default: the network's entry of nttFlowServiceUrls */
  flowServiceUrl?: string;
  /** Pinned ntt package ids; default: the network's entry of nttPackageIds */
  nttPackageIds?: NttPackageIds;
  /** Pinned wormhole-core package ids; default: the network's entry of corePackageIds */
  corePackageIds?: CorePackageIds;
  /** The deployment's mode; when set, every flow must agree */
  mode?: Ntt.Mode;
}

/**
 * Ntt.Contracts for Canton: `manager` and `transceiver.wormhole` are 64-hex addresses, `token` is
 * the instrument "<admin>/<id>", `peers` hold the peer managers by chain (plan 3.2).
 */
export type CantonNttContracts = Ntt.Contracts & { canton?: CantonNttSettings };

/** Settings of a CantonNtt that are objects, not config values */
export interface CantonNttOptions {
  /** Where flows and the manager summary come from; default: the flow service over HTTP */
  flowSource?: NttFlowSource;
  /** fetch for the default flow source */
  fetch?: FetchFunction;
  /** Wait before a rebuild reads a flow again (default 5.5 s, past the service cache) */
  rebuildDelayMs?: number;
  /** Parties whose view getMessageId reads when the call names none */
  readAs?: readonly string[];
  /**
   * The party this client administers the deployment as, hosted on the connected participant.
   * When set, the getters read the NttManager from this party's own active contract set (it is a
   * signatory), and the admin methods act as it when no payer is given. A party that no longer
   * sees the manager (after an admin transfer) falls back to /v1/managers for the getters.
   */
  adminParty?: string;
}

/** An NttManager read from a party's own active contract set, after the anchor checks */
export interface OwnNttManager {
  manager: NttManagerView;
  /** The contract, as a command exercises it */
  contract: DisclosedContract;
}

/** An AdminTransferProposal read from a party's own active contract set */
export interface OwnAdminTransferProposal {
  proposal: AdminTransferProposalView;
  contract: DisclosedContract;
}

/** Ntt.TransferOptions plus the Canton ones */
export interface CantonNttTransferOptions extends Ntt.TransferOptions {
  /** The transfer nonce (uint32); random when absent. Retrying with the same nonce is deduplicated. */
  nonce?: number;
  /**
   * The decimals the caller scaled the amount with (Ntt.TransferOptions). The transfer is refused
   * when the NttManager it submits against has other decimals. Set it whenever the amount was
   * scaled with decimals read apart from this transfer: getTokenDecimals answers from
   * /v1/managers, which nothing authenticates.
   */
  tokenDecimals?: number;
}

/** Canton options of a redeem */
export interface CantonNttRedeemOptions extends Ntt.RedeemOptions {
  /**
   * The recipient party. The VAA holds only its hash; without this option the payer is used when
   * its hash is the VAA recipient.
   */
  recipient?: string;
}

function modeOf(tokenConfig: NttTokenConfig): Ntt.Mode {
  return tokenConfig === "LockUnlock" ? "locking" : "burning";
}

function tokenConfigOf(mode: Ntt.Mode): NttTokenConfig {
  return mode === "locking" ? "LockUnlock" : "BurnMint";
}

/** An error for an Ntt method that Canton does not have */
function unsupported(method: string, why: string): Error {
  return new Error(`CantonNtt.${method} is unsupported: ${why}`);
}

const HEX_32 = /^(?:0x)?[0-9a-fA-F]{64}$/;

function hex32(text: string, what: string): string {
  if (!HEX_32.test(text))
    throw new Error(`${what} must be 32 bytes of hex, got "${text}"`);
  return text.replace(/^0x/, "").toLowerCase();
}

/** A peer address as 32 bytes of hex: a universal or native address, or a string of the chain */
function peerAddressHex(
  address: { toUniversalAddress(): UniversalAddress } | string,
  chain: Chain,
  what: string
): string {
  const universal =
    typeof address !== "string"
      ? address.toUniversalAddress()
      : HEX_32.test(address)
        ? new UniversalAddress(hex32(address, what))
        : toUniversal(chain, address);
  const hex = encoding.hex.encode(universal.toUint8Array());
  if (/^0+$/.test(hex)) throw new Error(`${what} is the zero address`);
  return hex;
}

function checkPeerDecimals(decimals: number): void {
  if (!Number.isInteger(decimals) || decimals < 1 || decimals > 255)
    throw new Error(
      `The peer token decimals must be an integer from 1 to 255, got ${decimals}`
    );
}

/** The manager summary form of an NttManager read from a ledger */
function summaryOf(own: OwnNttManager): NttManagerSummary {
  const { manager } = own;
  return {
    managerAddress: manager.managerAddress,
    contractId: own.contract.contractId,
    chainId: manager.chainId,
    managerId: manager.managerId,
    tokenConfig: manager.tokenConfig,
    instrumentId: { ...manager.instrumentId },
    reserveCustodian: manager.reserveCustodian,
    paused: manager.paused,
    tokenDecimals: manager.tokenDecimals,
    admin: manager.admin,
    peers: new Map(manager.peers),
  };
}

/** The fields where two summaries of one manager contract differ */
function summaryDifferences(
  own: NttManagerSummary,
  served: NttManagerSummary
): string[] {
  const differences: string[] = [];
  const compare = (field: string, ownValue: unknown, servedValue: unknown) => {
    if (ownValue !== servedValue) differences.push(field);
  };
  compare("tokenConfig", own.tokenConfig, served.tokenConfig);
  compare("paused", own.paused, served.paused);
  compare("admin", own.admin, served.admin);
  compare("reserveCustodian", own.reserveCustodian, served.reserveCustodian);
  compare("chainId", own.chainId, served.chainId);
  compare("managerId", own.managerId, served.managerId);
  compare(
    "instrumentId.admin",
    own.instrumentId.admin,
    served.instrumentId.admin
  );
  compare("instrumentId.id", own.instrumentId.id, served.instrumentId.id);
  if (served.tokenDecimals !== undefined)
    compare("tokenDecimals", own.tokenDecimals, served.tokenDecimals);
  if (served.peers !== undefined) {
    const ownPeers = own.peers ?? new Map<number, NttPeerView>();
    const chains = new Set([...ownPeers.keys(), ...served.peers.keys()]);
    for (const chainId of chains) {
      const mine = ownPeers.get(chainId);
      const theirs = served.peers.get(chainId);
      if (
        !mine ||
        !theirs ||
        mine.managerAddress !== theirs.managerAddress ||
        mine.transceiverAddress !== theirs.transceiverAddress ||
        mine.decimals !== theirs.decimals
      )
        differences.push(`peers[${chainId}]`);
    }
  }
  return differences;
}

function randomNonce(): number {
  const bytes = new Uint32Array(1);
  globalThis.crypto.getRandomValues(bytes);
  return bytes[0]!;
}

function sleep(ms: number): Promise<void> {
  return ms > 0
    ? new Promise((resolve) => setTimeout(resolve, ms))
    : Promise.resolve();
}

/**
 * The NTT protocol on Canton (ntt 0.7.0). Reads come from the NTT flow service: the manager
 * summary for the getters, the anchored NttManager blob of a flow for everything a submission
 * depends on. Transfers and redeems are one command each, built for a local party; the unsigned
 * transaction carries a rebuild function that a Canton signer calls after a stale-contract
 * rejection.
 */
export class CantonNtt<N extends Network, C extends CantonChains>
  implements Ntt<N, C>
{
  /** The configured guardianGovernance party (contracts.coreBridge) */
  readonly guardianGovernance: string;
  /** The configured manager address, lowercase hex */
  readonly managerAddress: string;
  readonly nttPackageIds: readonly string[];
  readonly corePackageIds: readonly string[];
  readonly options: CantonNttOptions;
  private defaultFlowSource: NttFlowSource | undefined;

  constructor(
    readonly network: N,
    readonly chain: C,
    readonly connection: CantonLedgerClient,
    readonly contracts: Contracts & { ntt?: CantonNttContracts },
    _version?: string,
    options: CantonNttOptions = {}
  ) {
    if (!contracts.ntt) throw new Error("NTT contracts not found");
    if (!contracts.coreBridge)
      throw new Error(
        `No guardianGovernance party (contracts.coreBridge) is configured for Canton ${network}`
      );
    this.guardianGovernance = parseCantonPartyId(contracts.coreBridge);
    this.managerAddress = hex32(contracts.ntt.manager, "contracts.ntt.manager");
    const settings = contracts.ntt.canton ?? {};
    if (settings.reserveCustodian !== undefined)
      parseCantonPartyId(settings.reserveCustodian);
    this.nttPackageIds = pinnedNttPackageIds(network, settings.nttPackageIds);
    this.corePackageIds = pinnedCorePackageIds(
      network,
      settings.corePackageIds
    );
    this.options = options;
  }

  /** A client with options (flow source, fetch, rebuild delay, readers) */
  static withOptions<N extends Network, C extends CantonChains>(
    network: N,
    chain: C,
    connection: CantonLedgerClient,
    contracts: Contracts & { ntt?: CantonNttContracts },
    options: CantonNttOptions
  ): CantonNtt<N, C> {
    return new CantonNtt(
      network,
      chain,
      connection,
      contracts,
      undefined,
      options
    );
  }

  /**
   * Builds the client from the platform config, which must hold exactly one Canton chain with
   * `contracts.ntt` (as CantonWormholeCore.fromRpc does). No ledger call is made.
   */
  static async fromRpc<N extends Network>(
    connection: CantonLedgerClient,
    config: ChainsConfig<N, CantonPlatformType>
  ): Promise<CantonNtt<N, CantonChains>> {
    const configured = Object.entries(config).filter(
      ([, chainConfig]) => !!chainConfig
    );
    if (configured.length !== 1)
      throw new Error(
        `CantonNtt.fromRpc needs exactly one configured Canton chain, found ${configured.length}`
      );
    const [chain, chainConfig] = configured[0]! as [
      CantonChains,
      (typeof config)[CantonChains],
    ];
    const contracts = chainConfig!.contracts as Contracts & {
      ntt?: CantonNttContracts;
    };
    if (!contracts.ntt) throw new Error("NTT contracts not found");
    return new CantonNtt(
      chainConfig!.network as N,
      chain,
      connection,
      contracts
    );
  }

  private get settings(): CantonNttSettings {
    return this.contracts.ntt!.canton ?? {};
  }

  /** The flow source: configured, else the flow service at the configured or network URL */
  flowSource(): NttFlowSource {
    if (this.options.flowSource) return this.options.flowSource;
    if (!this.defaultFlowSource) {
      const baseUrl =
        this.settings.flowServiceUrl ?? nttFlowServiceUrls[this.network];
      if (!baseUrl)
        throw new Error(
          `No NTT flow source is configured and Canton ${this.network} has no flow service URL; set contracts.ntt.canton.flowServiceUrl`
        );
      this.defaultFlowSource = new HttpNttFlowSource({
        baseUrl,
        ...(this.options.fetch ? { fetch: this.options.fetch } : {}),
      });
    }
    return this.defaultFlowSource;
  }

  /** The configured instrument, when contracts.ntt.token is one */
  private configuredInstrument() {
    const token = this.contracts.ntt!.token;
    if (!token || CantonAddress.kindOf(token) !== "instrument")
      return undefined;
    return new CantonAddress(token).toInstrument();
  }

  /** The anchor policy of this deployment */
  anchorPolicy(): NttAnchorPolicy {
    const instrument = this.configuredInstrument();
    return {
      guardianGovernance: this.guardianGovernance,
      managerAddress: this.managerAddress,
      nttPackageIds: this.nttPackageIds,
      corePackageIds: this.corePackageIds,
      ...(this.settings.reserveCustodian !== undefined
        ? { reserveCustodian: this.settings.reserveCustodian }
        : {}),
      ...(this.settings.mode !== undefined
        ? { tokenConfig: tokenConfigOf(this.settings.mode) }
        : {}),
      ...(instrument ? { instrument } : {}),
    };
  }

  // ---------------------------------------------------------------------------------------------
  // Reads
  // ---------------------------------------------------------------------------------------------

  /** The /v1/managers entry of the configured manager. For display; never a trust anchor. */
  async managerSummary(): Promise<NttManagerSummary> {
    const matches = (await this.flowSource().managers()).filter(
      (manager) => manager.managerAddress === this.managerAddress
    );
    if (matches.length !== 1)
      throw new Error(
        `The NTT flow service lists ${matches.length} managers with address ${this.managerAddress}`
      );
    return matches[0]!;
  }

  /** The active contracts of a template of the pinned ntt packages in a party's own view */
  private async ownContracts(
    party: string,
    moduleAndEntity: string,
    role: string
  ): Promise<DisclosedContract[]> {
    const contracts = await this.connection.activeContracts(
      parseCantonPartyId(party),
      {
        kind: "templates",
        templateIds: [`#${NTT_PACKAGE_NAME}:${moduleAndEntity}`],
      },
      { includeCreatedEventBlob: true }
    );
    // A contract of another ntt package has a layout this client does not know; it is never ours.
    return contracts
      .map((active) => disclosedContractFromActive(active, role))
      .filter((contract) =>
        this.nttPackageIds.includes(
          parseCantonTemplateId(contract.templateId).packageId
        )
      );
  }

  /**
   * The configured NttManager as a party sees it in its own active contract set: the admin and
   * the other signatories see it, nobody else does. The participant is the party's own, so this
   * read needs no flow service; it still passes the manager checks (pinned package, signatories,
   * address, custodian relation; anchor.ts checkManagerContract). Undefined when the party sees
   * no such manager.
   */
  async ownManager(
    party: string,
    policy: NttAnchorPolicy = this.readPolicy()
  ): Promise<OwnNttManager | undefined> {
    const matches: DisclosedContract[] = [];
    for (const contract of await this.ownContracts(
      party,
      NTT_TEMPLATES.NttManager,
      FLOW_ROLES.NttManager
    )) {
      let address: string;
      try {
        address = decodeNttManager(
          createArgumentToJson(
            readCreatedEventBlob(contract.createdEventBlob).createArgument,
            NTT_MANAGER_LAYOUT
          )
        ).managerAddress;
      } catch (err) {
        throw new NttAnchorError(
          `NttManager ${contract.contractId} in the view of ${party}: ${(err as Error).message}`
        );
      }
      if (address === this.managerAddress) matches.push(contract);
    }
    if (matches.length > 1)
      throw new NttAnchorError(
        `${party} sees ${matches.length} active NttManagers with address ${this.managerAddress}`
      );
    if (matches.length === 0) return undefined;
    return {
      manager: checkManagerContract(matches[0]!, policy),
      contract: matches[0]!,
    };
  }

  /**
   * The policy of a read: the configured token, mode and custodian are left to the getters, which
   * compare them with the config and report the difference.
   */
  private readPolicy(): NttAnchorPolicy {
    const {
      instrument: _instrument,
      tokenConfig: _tokenConfig,
      reserveCustodian: _reserveCustodian,
      ...policy
    } = this.anchorPolicy();
    return policy;
  }

  /**
   * The manager as the getters read it. With options.adminParty set and the manager in that
   * party's own active contract set, the own view wins. /v1/managers is then a cross-check only:
   * when it names the same contract, any field that differs is an error; when it names another
   * contract (its cache trails a change by a few seconds), or it cannot be read, it is not used.
   * Without an own view, the /v1/managers entry is the answer, as before.
   */
  async managerState(): Promise<NttManagerSummary> {
    const party = this.options.adminParty;
    const own = party === undefined ? undefined : await this.ownManager(party);
    if (!own) return this.managerSummary();
    const state = summaryOf(own);
    let served: NttManagerSummary;
    try {
      served = await this.managerSummary();
    } catch {
      return state;
    }
    if (served.contractId !== state.contractId) return state;
    const differences = summaryDifferences(state, served);
    if (differences.length > 0)
      throw new Error(
        `The /v1/managers entry of NttManager ${state.contractId} differs from the contract in the view of ${party} on ${differences.join(", ")}`
      );
    return state;
  }

  /**
   * A flow after the anchor checks (layers 1 to 3). The flow's NttManager blob gives the manager
   * fields every submission depends on.
   */
  async verifiedFlow(
    flow: NttFlowName,
    parameters: Readonly<Record<string, string>>,
    context: FlowContext = {},
    policy: NttAnchorPolicy = this.anchorPolicy()
  ): Promise<VerifiedNttFlow> {
    // The manager is always this deployment's; a parameter cannot replace it.
    if ("manager" in parameters)
      throw new Error("A flow parameter cannot name the manager");
    const source = this.flowSource();
    const [served, managers] = await Promise.all([
      source.flow(flow, { manager: this.managerAddress, ...parameters }),
      source.managers(),
    ]);
    return verifyFlow(served, managers, policy, context);
  }

  /**
   * The manager fields read from the anchored blob of the transfer flow. The configured token
   * and mode are not pinned here, so verifyAddresses can report them. Only the NttManager is
   * used: a role the service cannot supply for a transfer (a custodian's preapproval or pot) does
   * not stop the read.
   */
  async verifiedManager(): Promise<NttManagerView> {
    const {
      instrument: _instrument,
      tokenConfig: _tokenConfig,
      ...policy
    } = this.anchorPolicy();
    return (
      await this.verifiedFlow(
        "transfer",
        {},
        { uses: [FLOW_ROLES.NttManager] },
        policy
      )
    ).manager;
  }

  async getMode(): Promise<Ntt.Mode> {
    return modeOf((await this.managerState()).tokenConfig);
  }

  async isPaused(): Promise<boolean> {
    return (await this.managerState()).paused;
  }

  /** The manager's admin: from the admin's own view when options.adminParty sees it (managerState) */
  async getOwner(): Promise<AccountAddress<C>> {
    return new CantonAddress(
      (await this.managerState()).admin
    ) as unknown as AccountAddress<C>;
  }

  /** Canton NTT has no pauser role */
  async getPauser(): Promise<AccountAddress<C> | null> {
    return null;
  }

  /** One transceiver, so the threshold is always 1 */
  async getThreshold(): Promise<number> {
    return 1;
  }

  /** The reserve custodian; it must equal the configured one when the config has it */
  async getCustodyAddress(): Promise<string> {
    const { reserveCustodian } = await this.managerState();
    const configured = this.settings.reserveCustodian;
    if (configured !== undefined && configured !== reserveCustodian)
      throw new Error(
        `The flow service names reserveCustodian ${reserveCustodian}, the config ${configured}`
      );
    return reserveCustodian;
  }

  /**
   * The token decimals: the config's, else the manager summary's; both must agree when both exist.
   * Without a configured value and without options.adminParty this is the flow service's word
   * (/v1/managers). A caller that scales a transfer amount with it must pass the same value as
   * the transfer's tokenDecimals option, which is compared with the NttManager the command
   * submits against.
   */
  async getTokenDecimals(): Promise<number> {
    const configured = this.contracts.ntt!.tokenDecimals;
    let served: number | undefined;
    try {
      served = (await this.managerState()).tokenDecimals;
    } catch (err) {
      if (configured === undefined) throw err;
    }
    if (
      configured !== undefined &&
      served !== undefined &&
      configured !== served
    )
      throw new Error(
        `The NttManager has tokenDecimals ${served}, the config ${configured}`
      );
    const decimals = configured ?? served;
    if (decimals === undefined)
      throw new Error(
        "No token decimals are configured and the flow service gives none"
      );
    return decimals;
  }

  private configuredPeer<PC extends Chain>(
    chain: PC
  ): { manager: UniversalAddress; tokenDecimals?: number } | undefined {
    const peer = this.contracts.ntt!.peers?.[chain];
    if (!peer) return undefined;
    const manager = HEX_32.test(peer.manager)
      ? new UniversalAddress(hex32(peer.manager, `peers.${chain}.manager`))
      : toUniversal(chain, peer.manager);
    return {
      manager,
      ...(peer.tokenDecimals !== undefined
        ? { tokenDecimals: peer.tokenDecimals }
        : {}),
    };
  }

  /**
   * The peer of a chain: from the manager summary, else from the config. A peer in both must
   * agree. Canton has no rate limits, so the inbound limit is 0 with a rate limit duration of 0.
   */
  async getPeer<PC extends Chain>(chain: PC): Promise<Ntt.Peer<PC> | null> {
    const configured = this.configuredPeer(chain);
    let servedPeers: NttManagerSummary["peers"];
    try {
      servedPeers = (await this.managerState()).peers;
    } catch (err) {
      if (!configured) throw err;
    }
    const served = servedPeers?.get(toChainId(chain));
    if (served && configured) {
      if (
        !configured.manager.equals(new UniversalAddress(served.managerAddress))
      )
        throw new Error(
          `The NttManager peer of ${chain} is ${served.managerAddress}, the config ${configured.manager}`
        );
      if (
        configured.tokenDecimals !== undefined &&
        configured.tokenDecimals !== served.decimals
      )
        throw new Error(
          `The NttManager peer of ${chain} has ${served.decimals} decimals, the config ${configured.tokenDecimals}`
        );
    }
    if (served)
      return {
        address: {
          chain,
          address: new UniversalAddress(served.managerAddress),
        } as ChainAddress<PC>,
        tokenDecimals: served.decimals,
        inboundLimit: 0n,
      };
    if (!configured) return null;
    if (configured.tokenDecimals === undefined)
      throw new Error(`The configured peer of ${chain} has no tokenDecimals`);
    return {
      address: { chain, address: configured.manager } as ChainAddress<PC>,
      tokenDecimals: configured.tokenDecimals,
      inboundLimit: 0n,
    };
  }

  /** The peer transceiver of a chain, from the manager summary */
  async getTransceiverPeer<PC extends Chain>(
    ix: number,
    chain: PC
  ): Promise<ChainAddress<PC> | null> {
    if (ix !== 0) return null;
    const peer = (await this.managerState()).peers?.get(toChainId(chain));
    return peer
      ? ({
          chain,
          address: new UniversalAddress(peer.transceiverAddress),
        } as ChainAddress<PC>)
      : null;
  }

  /** The configured transceiver address, lowercase hex */
  transceiverAddress(): string {
    const configured = this.contracts.ntt!.transceiver["wormhole"];
    if (!configured)
      throw new Error(
        "No wormhole transceiver is configured (contracts.ntt.transceiver.wormhole)"
      );
    return hex32(configured, "contracts.ntt.transceiver.wormhole");
  }

  async getTransceiver(
    ix: number
  ): Promise<NttTransceiver<N, C, Ntt.Attestation> | null> {
    if (ix !== 0) return null;
    return new CantonNttWormholeTransceiver(this);
  }

  async getTransceiverType(transceiverIndex: number = 0): Promise<string> {
    if (transceiverIndex !== 0)
      throw new Error(
        `Transceiver index ${transceiverIndex} is not supported: Canton has one transceiver`
      );
    return "wormhole";
  }

  /**
   * Compares the config with the anchored manager: the transceiver address is recomputed from gg
   * and the manager's transceiver emitter id (Emitter owner gg), and the token, its decimals and
   * the reserve custodian are compared. Returns the configured values that differ, or null.
   */
  async verifyAddresses(): Promise<Partial<Ntt.Contracts> | null> {
    const manager = await this.verifiedManager();
    const mismatches: Partial<Ntt.Contracts> = {};
    const transceiver = encoding.hex.encode(
      emitterAddress(
        this.guardianGovernance,
        this.guardianGovernance,
        manager.transceiverEmitterId
      ).toUint8Array()
    );
    const configuredTransceiver = this.contracts.ntt!.transceiver["wormhole"];
    if (
      configuredTransceiver === undefined ||
      !HEX_32.test(configuredTransceiver) ||
      hex32(configuredTransceiver, "transceiver") !== transceiver
    )
      mismatches.transceiver = { wormhole: transceiver };
    const token = `${manager.instrumentId.admin}/${manager.instrumentId.id}`;
    if (this.contracts.ntt!.token !== token) mismatches.token = token;
    const configuredDecimals = this.contracts.ntt!.tokenDecimals;
    if (
      configuredDecimals !== undefined &&
      configuredDecimals !== manager.tokenDecimals
    )
      mismatches.tokenDecimals = manager.tokenDecimals;
    return Object.keys(mismatches).length > 0 ? mismatches : null;
  }

  /** The core message fee, in 10^-10 units of the fee instrument (read from the anchored CoreState) */
  async quoteDeliveryPrice(
    _destination: Chain,
    _options: Ntt.TransferOptions
  ): Promise<bigint> {
    return flowCoreState(
      await this.verifiedFlow(
        "transfer",
        {},
        { uses: [FLOW_ROLES.NttManager, FLOW_ROLES.CoreState] }
      )
    ).messageFee;
  }

  // Canton NTT has no rate limits and no inbound queue: "no limit" values, as XRPL returns them.

  async getCurrentOutboundCapacity(): Promise<bigint> {
    return 0n;
  }

  async getOutboundLimit(): Promise<bigint> {
    return 0n;
  }

  async getCurrentInboundCapacity<PC extends Chain>(
    _fromChain: PC
  ): Promise<bigint> {
    return 0n;
  }

  async getInboundLimit<PC extends Chain>(_fromChain: PC): Promise<bigint> {
    return 0n;
  }

  /** 0: no rate limiting */
  async getRateLimitDuration(): Promise<bigint> {
    return 0n;
  }

  async getIsTransferInboundQueued(
    _attestation: Ntt.Attestation
  ): Promise<boolean> {
    return false;
  }

  async getInboundQueuedTransfer<PC extends Chain>(
    _fromChain: PC,
    _transceiverMessage: Ntt.Message
  ): Promise<Ntt.InboundQueuedTransfer<C> | null> {
    return null;
  }

  /**
   * Whether the VAA's transfer has been redeemed: its digest is in the consumed set of the
   * covering ReplayNode of this deployment's trie (consumer gg, namespace the manager's). Release
   * and Mint consume the digest in the same transaction that delivers the tokens, and Canton has
   * no inbound queue, so a consumed digest is a completed transfer.
   *
   * The node comes from the flow service's set-peer flow: the release and mint flows serve a node
   * for a consumed digest too (so a flow answer alone tells nothing), and the mint flow needs the
   * recipient party and its DepositPreapproval, which a VAA does not name. The set-peer flow
   * serves the covering node of any digest in either mode. The node passes the anchor checks and
   * is decoded from its blob; any layout mismatch throws.
   *
   * The answer is the flow service's word. Nothing here is submitted, so the ledger does not
   * bind the node blob to a contract, and the checks only show that the blob is well formed: a
   * hostile service can serve a forged node and report a digest as consumed (or not). With an
   * honest service true is final and false is as fresh as its view of the trie (anchor.ts
   * flowReplayNodeRecordsDigest). A false true costs no funds: the VAA stays redeemable, and
   * Release and Mint check the replay state on the ledger.
   */
  async getIsExecuted(attestation: Ntt.Attestation): Promise<boolean> {
    this.checkInboundVaa(attestation);
    const digest = vaaDigest(attestation.hash);
    const flow = await this.verifiedFlow(
      "set-peer",
      { digest },
      { uses: [FLOW_ROLES.NttManager, FLOW_ROLES.coveringReplayNode] }
    );
    return flowReplayNodeRecordsDigest(flow, digest);
  }

  /**
   * The same as getIsExecuted. On EVM a message is approved once the transceivers attest it on
   * the destination, before (and apart from) its execution; the routes read "approved" as "the
   * destination has started the redeem". Canton has no such step: Release and Mint verify the
   * guardian-signed VAA and consume its digest in one transaction, and nothing about the VAA is
   * on the ledger before that. Answering true for any guardian-signed VAA would report a redeem
   * that has not begun, so approved means consumed, as on chains whose redeem is one step.
   */
  async getIsApproved(attestation: Ntt.Attestation): Promise<boolean> {
    return this.getIsExecuted(attestation);
  }

  // ---------------------------------------------------------------------------------------------
  // Admin methods (milestone N4). The admin signs each one; it is read from the payer argument or
  // options.adminParty. Each command acts on the NttManager the admin sees in its own active
  // contract set; a command that publishes (SetPeer) takes the CoreState and the transceiver
  // Emitter from the anchored transfer flow. When gg is the admin, admin changes are governance
  // actions (SetPeerByVaa, AcceptAdminTransferByVaa, SetFactoryByVaa) and these methods refuse.
  // ---------------------------------------------------------------------------------------------

  /** The admin party of an admin method: the payer, else options.adminParty */
  private adminPartyOf(
    payer: AccountAddress<C> | undefined,
    method: string
  ): string {
    if (payer !== undefined)
      return new CantonAddress(payer as unknown as AnyCantonAddress).toParty();
    if (this.options.adminParty !== undefined)
      return parseCantonPartyId(this.options.adminParty);
    throw new Error(
      `CantonNtt.${method} needs the admin party: pass the payer, or set options.adminParty`
    );
  }

  /** The NttManager in the admin's own view; the party must be its admin, and not gg */
  async adminManager(admin: string, action: string): Promise<OwnNttManager> {
    const own = await this.ownManager(admin, this.anchorPolicy());
    if (!own)
      throw new Error(
        `${admin} sees no active NttManager with address ${this.managerAddress} on the connected participant; only the deployment's admin, hosted there, can ${action}`
      );
    if (own.manager.admin !== admin)
      throw new Error(
        `${admin} is not the admin of NttManager ${this.managerAddress}; its admin is ${own.manager.admin}`
      );
    if (admin === this.guardianGovernance)
      throw new Error(
        `The admin of NttManager ${this.managerAddress} is guardianGovernance: to ${action} is a governance action there, relayed with a guardian-signed VAA, not a command of this client`
      );
    return own;
  }

  /** One exercise on a contract, with the contracts the command discloses */
  private exerciseEnvelope(
    target: DisclosedContract,
    choice: string,
    choiceArgument: unknown,
    actAs: string,
    disclosures: readonly DisclosedContract[],
    commandId?: string
  ): CantonCommandEnvelope {
    if (
      disclosures.some(
        (entry) => entry.synchronizerId !== target.synchronizerId
      )
    )
      throw new NttAnchorError(
        `the disclosed contracts are not on the synchronizer of the ${target.role} ${target.contractId}`
      );
    return buildCommandEnvelope({
      command: {
        ExerciseCommand: {
          templateId: target.templateId,
          contractId: target.contractId,
          choice,
          choiceArgument,
        },
      },
      actAs: [actAs],
      disclosedContracts: disclosures,
      synchronizerId: target.synchronizerId,
      packageIdSelectionPreference: [this.nttPackageIds[0]!],
      ...(commandId !== undefined ? { commandId } : {}),
    });
  }

  private async setPausedEnvelope(
    admin: string,
    paused: boolean
  ): Promise<CantonCommandEnvelope> {
    const action = paused ? "pause" : "unpause";
    const { manager, contract } = await this.adminManager(admin, action);
    if (manager.paused === paused)
      throw new Error(
        `The NttManager is already ${paused ? "paused" : "unpaused"}; nothing has been submitted`
      );
    return this.exerciseEnvelope(
      contract,
      NTT_CHOICES.SetPaused,
      setPausedArgument(paused),
      admin,
      []
    );
  }

  /**
   * SetPaused with newPaused = true, as the admin (the payer, else options.adminParty). Pausing
   * blocks Transfer, Release and Mint; the admin choices keep working. Refuses a manager that is
   * already paused. The manager is read from the admin's own view; no flow is read.
   */
  async *pause(
    payer?: AccountAddress<C>
  ): AsyncGenerator<CantonUnsignedTransaction<N, C>> {
    const admin = this.adminPartyOf(payer, "pause");
    const build = () => this.setPausedEnvelope(admin, true);
    yield this.unsignedTransaction(await build(), "CantonNtt.pause", build);
  }

  /** SetPaused with newPaused = false, as the admin; see pause */
  async *unpause(
    payer?: AccountAddress<C>
  ): AsyncGenerator<CantonUnsignedTransaction<N, C>> {
    const admin = this.adminPartyOf(payer, "unpause");
    const build = () => this.setPausedEnvelope(admin, false);
    yield this.unsignedTransaction(await build(), "CantonNtt.unpause", build);
  }

  /**
   * One SetPeer command. `peerOf` makes the peer from the existing one (undefined when the chain
   * has none), read at build time, so a rebuild merges into the current peer table.
   */
  private async setPeerEnvelope(
    admin: string,
    chain: Chain,
    peerOf: (existing: NttPeerView | undefined) => NttPeerView
  ): Promise<CantonCommandEnvelope> {
    const own = await this.adminManager(admin, "set a peer");
    const chainId = toChainId(chain);
    if (chainId === own.manager.chainId)
      throw new Error(
        `A peer cannot be on this deployment's own chain ${chain} (${chainId})`
      );
    const peer = peerOf(own.manager.peers.get(chainId));
    checkPeerDecimals(peer.decimals);
    // The admin is no stakeholder of the CoreState and the transceiver Emitter; the anchored
    // transfer flow serves both. The flow's own NttManager may trail the admin's view by the
    // service's cache; it must still be the same deployment.
    const flow = await this.verifiedFlow(
      "transfer",
      {},
      {
        uses: [
          FLOW_ROLES.NttManager,
          FLOW_ROLES.CoreState,
          FLOW_ROLES.transceiverEmitter,
        ],
      }
    );
    if (flow.manager.transceiverEmitterId !== own.manager.transceiverEmitterId)
      throw new NttAnchorError(
        `the transfer flow's NttManager has transceiver emitter ${flow.manager.transceiverEmitterId}, the admin's ${own.manager.transceiverEmitterId}`
      );
    const fee = flowCoreState(flow).messageFee;
    if (fee !== 0n)
      throw new NttAnchorError(
        `the core message fee is ${fee} (10^-10 units of the fee instrument); SetPeer publishes a registration message, and only a zero fee is supported until milestone N5`
      );
    return this.exerciseEnvelope(
      own.contract,
      NTT_CHOICES.SetPeer,
      setPeerArgument({
        chainId,
        peer,
        transceiverEmitterCid: roleContractId(
          flow,
          FLOW_ROLES.transceiverEmitter
        ),
        coreStateCid: roleContractId(flow, FLOW_ROLES.CoreState),
        // Canton finality is immediate; the registration publishes like a transfer.
        consistencyLevel: TRANSFER_CONSISTENCY_LEVEL,
        // The direct admin path self-pays (Manager.daml SetPeer).
        payer: admin,
      }),
      admin,
      flowDisclosures(flow, [
        FLOW_ROLES.CoreState,
        FLOW_ROLES.transceiverEmitter,
      ])
    );
  }

  private async *setPeerTransactions(
    method: string,
    payer: AccountAddress<C> | undefined,
    chain: Chain,
    peerOf: (existing: NttPeerView | undefined) => NttPeerView
  ): AsyncGenerator<CantonUnsignedTransaction<N, C>> {
    const admin = this.adminPartyOf(payer, method);
    const build = () => this.setPeerEnvelope(admin, chain, peerOf);
    const rebuild = async () => {
      await sleep(this.options.rebuildDelayMs ?? FLOW_REBUILD_DELAY_MS);
      return build();
    };
    yield this.unsignedTransaction(
      await build(),
      `CantonNtt.${method} (${chain})`,
      rebuild
    );
  }

  /**
   * Sets or replaces the peer of a chain: one SetPeer, which stores the peer manager address, the
   * peer transceiver address and the peer token decimals together, and publishes a transceiver
   * registration message (through the core, so the core message fee must be zero until N5). As
   * the admin (the payer, else options.adminParty), who also pays the message. Addresses are
   * universal or native addresses of `chain`, or 64-hex strings.
   */
  async *setPeerFull(
    chain: Chain,
    manager: { toUniversalAddress(): UniversalAddress } | string,
    transceiver: { toUniversalAddress(): UniversalAddress } | string,
    decimals: number,
    payer?: AccountAddress<C>
  ): AsyncGenerator<CantonUnsignedTransaction<N, C>> {
    checkPeerDecimals(decimals);
    const peer: NttPeerView = {
      managerAddress: peerAddressHex(
        manager,
        chain,
        "The peer manager address"
      ),
      transceiverAddress: peerAddressHex(
        transceiver,
        chain,
        "The peer transceiver address"
      ),
      decimals,
    };
    yield* this.setPeerTransactions("setPeerFull", payer, chain, () => peer);
  }

  /**
   * Changes the peer manager address and token decimals of a chain that has a peer, keeping its
   * transceiver address (Canton's SetPeer sets all three; setPeerFull adds a peer). Canton has no
   * rate limits, so inboundLimit must be 0.
   */
  async *setPeer(
    peer: ChainAddress,
    tokenDecimals: number,
    inboundLimit: bigint,
    payer?: AccountAddress<C>
  ): AsyncGenerator<CantonUnsignedTransaction<N, C>> {
    if (inboundLimit !== 0n)
      throw new Error(
        `CantonNtt.setPeer: Canton NTT has no rate limits; the inbound limit must be 0, got ${inboundLimit}`
      );
    checkPeerDecimals(tokenDecimals);
    const managerAddress = peerAddressHex(
      peer.address,
      peer.chain,
      "The peer manager address"
    );
    yield* this.setPeerTransactions(
      "setPeer",
      payer,
      peer.chain,
      (existing) => {
        if (!existing) throw this.noPeerToUpdate(peer.chain, "setPeer");
        return { ...existing, managerAddress, decimals: tokenDecimals };
      }
    );
  }

  /**
   * Changes the peer transceiver address of a chain that has a peer, keeping its manager address
   * and decimals. Canton has one transceiver, index 0.
   */
  async *setTransceiverPeer(
    ix: number,
    peer: ChainAddress,
    payer?: AccountAddress<C>
  ): AsyncGenerator<CantonUnsignedTransaction<N, C>> {
    if (ix !== 0)
      throw new Error(
        `Transceiver index ${ix} is not supported: Canton has one transceiver`
      );
    const transceiverAddress = peerAddressHex(
      peer.address,
      peer.chain,
      "The peer transceiver address"
    );
    yield* this.setPeerTransactions(
      "setTransceiverPeer",
      payer,
      peer.chain,
      (existing) => {
        if (!existing)
          throw this.noPeerToUpdate(peer.chain, "setTransceiverPeer");
        return { ...existing, transceiverAddress };
      }
    );
  }

  private noPeerToUpdate(chain: Chain, method: string): Error {
    return new Error(
      `The NttManager has no peer for ${chain} to update; ${method} changes one field of an existing peer (Canton sets manager, transceiver and decimals together). Add the peer with setPeerFull(chain, manager, transceiver, decimals); nothing has been submitted`
    );
  }

  /**
   * Proposes `newOwner` as the next admin: creates an AdminTransferProposal signed by the current
   * admin (the payer, else options.adminParty). This is the first half of Canton's two-step admin
   * transfer. Nothing changes until the new admin accepts with acceptOwnership, signing as itself
   * (AcceptAdminTransfer carries the proposer's standing consent and nests TransferAdmin, which
   * needs both). A proposal to guardianGovernance is the custody opt-in: it completes only with
   * AcceptAdminTransferByVaa, relayed with a guardian-signed governance VAA, not with
   * acceptOwnership. The proposal stays until accepted, or until the proposer retracts it
   * (RetractAdminTransfer, not built here); it goes stale when the admin changes.
   */
  async *setOwner(
    newOwner: AccountAddress<C>,
    payer?: AccountAddress<C>
  ): AsyncGenerator<CantonUnsignedTransaction<N, C>> {
    const admin = this.adminPartyOf(payer, "setOwner");
    const newAdmin = new CantonAddress(
      newOwner as unknown as AnyCantonAddress
    ).toParty();
    const build = async (): Promise<CantonCommandEnvelope> => {
      const { manager, contract } = await this.adminManager(
        admin,
        "propose an admin transfer"
      );
      if (newAdmin === admin)
        throw new Error(`${newAdmin} is already the admin`);
      if (newAdmin === manager.nttCustody)
        throw new Error(
          `The custody party ${newAdmin} cannot hold the admin role`
        );
      // The proposal's package is the manager's, so AcceptAdminTransfer fetches it as is.
      const { packageId } = parseCantonTemplateId(contract.templateId);
      const command: CantonCommand = {
        CreateCommand: {
          templateId: `${packageId}:${NTT_TEMPLATES.AdminTransferProposal}`,
          createArguments: adminTransferProposalArgument({
            admin,
            newAdmin,
            managerAddress: this.managerAddress,
          }),
        },
      };
      return buildCommandEnvelope({
        command,
        actAs: [admin],
        synchronizerId: contract.synchronizerId,
        packageIdSelectionPreference: [this.nttPackageIds[0]!],
        commandId: proposeAdminCommandId({
          admin,
          newAdmin,
          manager: this.managerAddress,
          managerContractId: contract.contractId,
        }),
      });
    };
    yield this.unsignedTransaction(await build(), "CantonNtt.setOwner", build);
  }

  /**
   * The AdminTransferProposals to `newAdmin` for the configured manager, from its own view (it
   * is their observer). Each passes checkAdminTransferProposal.
   */
  async ownAdminTransferProposals(
    newAdmin: string
  ): Promise<OwnAdminTransferProposal[]> {
    const proposals: OwnAdminTransferProposal[] = [];
    for (const contract of await this.ownContracts(
      newAdmin,
      NTT_TEMPLATES.AdminTransferProposal,
      "AdminTransferProposal"
    )) {
      const proposal = checkAdminTransferProposal(contract, this.nttPackageIds);
      if (
        proposal.managerAddress === this.managerAddress &&
        proposal.newAdmin === newAdmin
      )
        proposals.push({ proposal, contract });
    }
    return proposals.sort((left, right) =>
      left.contract.contractId < right.contract.contractId ? -1 : 1
    );
  }

  private async acceptOwnershipEnvelope(
    newAdmin: string,
    vettedFactory: string
  ): Promise<CantonCommandEnvelope> {
    const proposals = await this.ownAdminTransferProposals(newAdmin);
    // The acceptor is no stakeholder of the manager yet: the anchored transfer flow serves it.
    // The configured custodian is not pinned here; the acceptor's config may already name the
    // custodianship it takes over. The custodian relation itself is still checked.
    const { reserveCustodian: _reserveCustodian, ...policy } =
      this.anchorPolicy();
    const flow = await this.verifiedFlow(
      "transfer",
      {},
      { uses: [FLOW_ROLES.NttManager, FLOW_ROLES.LockedLedger] },
      policy
    );
    const { manager } = flow;
    const current = proposals.filter(
      ({ proposal }) => proposal.admin === manager.admin
    );
    if (current.length === 0)
      throw new Error(
        proposals.length === 0
          ? `${newAdmin} sees no AdminTransferProposal for NttManager ${this.managerAddress}; its admin ${manager.admin} creates one with setOwner. Nothing has been submitted`
          : `The AdminTransferProposals ${newAdmin} sees for NttManager ${this.managerAddress} are from ${[...new Set(proposals.map(({ proposal }) => proposal.admin))].join(", ")}, not from its admin ${manager.admin}; they are stale. Nothing has been submitted`
      );
    if (newAdmin === manager.nttCustody)
      throw new Error(
        `The custody party ${newAdmin} cannot hold the admin role`
      );
    if (manager.factory.contractId !== vettedFactory)
      throw new NttAnchorError(
        `the NttManager commits the factory ${manager.factory.contractId} (${manager.factory.tag}), not the vetted ${vettedFactory}`
      );
    const roles: string[] = [FLOW_ROLES.NttManager];
    let ledgerCid: string | undefined;
    if (manager.tokenConfig === "LockUnlock") {
      const ledger = flowLockedLedger(flow);
      ledgerCid = roleContractId(flow, FLOW_ROLES.LockedLedger);
      roles.push(FLOW_ROLES.LockedLedger);
      if (ledger.balance > 0n)
        throw new NttAnchorError(
          `the LockedLedger holds ${ledger.balance} (10^-10 units). Accepting moves it from the custody pot to the new custodian, which needs the pot, the committed factory and the new custodian's preapproval; this client does not build that transfer yet`
        );
      if (ledger.custodyHoldingCid !== undefined) {
        // TransferAdmin reads the pot even when the balance is zero.
        const pot = flow.roles.get(FLOW_ROLES.custodyPotHolding);
        if (pot?.contractId !== ledger.custodyHoldingCid)
          throw new NttAnchorError(
            `the flow service does not serve the custody pot ${ledger.custodyHoldingCid} the LockedLedger names (it cannot read a third-party custodian's holdings)`
          );
        roles.push(FLOW_ROLES.custodyPotHolding);
      }
    }
    const proposal = current[0]!;
    return this.exerciseEnvelope(
      proposal.contract,
      NTT_CHOICES.AcceptAdminTransfer,
      acceptAdminTransferArgument({
        mgrCid: roleContractId(flow, FLOW_ROLES.NttManager),
        ...(ledgerCid !== undefined ? { ledgerCid } : {}),
        vettedFactory: manager.factory,
      }),
      newAdmin,
      flowDisclosures(flow, roles)
    );
  }

  /**
   * The second half of Canton's two-step admin transfer (setOwner is the first): `newOwner`
   * accepts the standing AdminTransferProposal from the current admin, signing as itself.
   * AcceptAdminTransfer nests TransferAdmin with the proposer's stored consent, so the old admin
   * does not sign again. The new admin names the committed factory it vetted (its contract id);
   * the command is refused before submission when the manager commits another. For lock/unlock,
   * the reserve custodian follows the admin and the LockedLedger's observer moves with it; a
   * ledger that holds a balance is refused (moving the reserve is not built yet). The proposal is
   * read from the new admin's own view, the manager and the ledger from the anchored transfer flow.
   */
  async *acceptOwnership(
    newOwner: AccountAddress<C>,
    vettedFactory: string
  ): AsyncGenerator<CantonUnsignedTransaction<N, C>> {
    const newAdmin = new CantonAddress(
      newOwner as unknown as AnyCantonAddress
    ).toParty();
    if (newAdmin === this.guardianGovernance)
      throw new Error(
        "An admin transfer to guardianGovernance completes with AcceptAdminTransferByVaa, relayed with a guardian-signed governance VAA, not with acceptOwnership"
      );
    const factory = parseCantonContractId(vettedFactory);
    const build = () => this.acceptOwnershipEnvelope(newAdmin, factory);
    const rebuild = async () => {
      await sleep(this.options.rebuildDelayMs ?? FLOW_REBUILD_DELAY_MS);
      return build();
    };
    yield this.unsignedTransaction(
      await build(),
      "CantonNtt.acceptOwnership",
      rebuild
    );
  }

  async *setPauser(
    _newPauser: AccountAddress<C>,
    _payer?: AccountAddress<C>
  ): AsyncGenerator<CantonUnsignedTransaction<N, C>> {
    throw unsupported("setPauser", "Canton NTT has no pauser role");
  }

  async *setThreshold(
    _threshold: number,
    _payer?: AccountAddress<C>
  ): AsyncGenerator<CantonUnsignedTransaction<N, C>> {
    throw unsupported(
      "setThreshold",
      "Canton NTT has one transceiver; the threshold is 1"
    );
  }

  async *setOutboundLimit(
    _limit: bigint,
    _payer?: AccountAddress<C>
  ): AsyncGenerator<CantonUnsignedTransaction<N, C>> {
    throw unsupported("setOutboundLimit", "Canton NTT has no rate limits");
  }

  async *setInboundLimit<PC extends Chain>(
    _fromChain: PC,
    _limit: bigint,
    _payer?: AccountAddress<C>
  ): AsyncGenerator<CantonUnsignedTransaction<N, C>> {
    throw unsupported("setInboundLimit", "Canton NTT has no rate limits");
  }

  async *completeInboundQueuedTransfer<PC extends Chain>(
    _fromChain: PC,
    _transceiverMessage: Ntt.Message,
    _payer?: AccountAddress<C>
  ): AsyncGenerator<CantonUnsignedTransaction<N, C>> {
    throw unsupported(
      "completeInboundQueuedTransfer",
      "Canton NTT has no inbound queue"
    );
  }

  // ---------------------------------------------------------------------------------------------
  // Transfer
  // ---------------------------------------------------------------------------------------------

  private unsignedTransaction(
    envelope: CantonCommandEnvelope,
    description: string,
    rebuild: () => Promise<CantonCommandEnvelope>
  ): CantonUnsignedTransaction<N, C> {
    return new CantonUnsignedTransaction(
      envelope,
      this.network,
      this.chain,
      description,
      false,
      rebuild
    );
  }

  private envelope(
    flow: VerifiedNttFlow,
    choice: string,
    choiceArgument: unknown,
    actAs: string,
    commandId: string
  ): CantonCommandEnvelope {
    const managerContract = flow.roles.get(FLOW_ROLES.NttManager)!;
    const command: CantonCommand = {
      ExerciseCommand: {
        templateId: managerContract.templateId,
        contractId: managerContract.contractId,
        choice,
        choiceArgument,
      },
    };
    return buildCommandEnvelope({
      command,
      actAs: [actAs],
      disclosedContracts: flow.disclosures,
      synchronizerId: flow.synchronizerId,
      packageIdSelectionPreference: [this.nttPackageIds[0]!],
      commandId,
    });
  }

  private async transferEnvelope(
    user: string,
    rawAmount: bigint,
    destination: ChainAddress,
    nonce: number,
    callerDecimals: number | undefined
  ): Promise<CantonCommandEnvelope> {
    if (this.settings.reserveCustodian === undefined)
      throw new NttAnchorError(
        "a transfer needs the configured reserve custodian (contracts.ntt.canton.reserveCustodian)"
      );
    const flow = await this.verifiedFlow("transfer", {});
    const { manager } = flow;
    if (manager.paused) throw new NttAnchorError("the NttManager is paused");
    // The amount is in units of the manager's tokenDecimals. A caller that scaled it with other
    // decimals (a config, a platform default or a /v1/managers answer that differs) would move a
    // multiple of the amount it meant. manager is read from the blob this command discloses, so
    // these comparisons are against the contract the ledger runs the Transfer on.
    const configuredDecimals = this.contracts.ntt!.tokenDecimals;
    if (
      configuredDecimals !== undefined &&
      configuredDecimals !== manager.tokenDecimals
    )
      throw new NttAnchorError(
        `the NttManager has tokenDecimals ${manager.tokenDecimals}, the config ${configuredDecimals}`
      );
    if (
      callerDecimals !== undefined &&
      callerDecimals !== manager.tokenDecimals
    )
      throw new NttAnchorError(
        `the NttManager has tokenDecimals ${manager.tokenDecimals}, the amount was scaled with ${callerDecimals}`
      );
    const fee = flowCoreState(flow).messageFee;
    if (fee !== 0n)
      throw new NttAnchorError(
        `the core message fee is ${fee} (10^-10 units of the fee instrument); only a zero fee is supported until milestone N5`
      );
    const recipientChain = toChainId(destination.chain);
    const peer = manager.peers.get(recipientChain);
    if (!peer)
      throw new NttAnchorError(
        `the NttManager has no peer for ${destination.chain}`
      );
    checkTransferAmount(rawAmount, manager.tokenDecimals, peer.decimals);
    const recipient = encoding.hex.encode(
      destination.address.toUniversalAddress().toUint8Array()
    );
    if (/^0+$/.test(recipient))
      throw new Error("The transfer recipient address is zero");
    // Select at the 10 decimals of a Daml Decimal, so a holding with finer digits is still usable.
    const inputHoldingCids = selectInputHoldings(
      await readHoldings(this.connection, user),
      manager.instrumentId,
      rawAmount * 10n ** BigInt(DAML_DECIMAL_SCALE - manager.tokenDecimals),
      DAML_DECIMAL_SCALE
    );
    const locking = manager.tokenConfig === "LockUnlock";
    const preapproval = flowNeedsTransferPreapproval(flow)
      ? roleContractId(flow, FLOW_ROLES.custodianTransferPreapproval)
      : undefined;
    const choiceArgument = transferArgument({
      user,
      transceiverEmitterCid: roleContractId(
        flow,
        FLOW_ROLES.transceiverEmitter
      ),
      coreStateCid: roleContractId(flow, FLOW_ROLES.CoreState),
      ...(locking
        ? { ledgerCid: roleContractId(flow, FLOW_ROLES.LockedLedger) }
        : {}),
      recipientChain,
      recipientAddress: recipient,
      rawAmount,
      nonce,
      consistencyLevel: TRANSFER_CONSISTENCY_LEVEL,
      inputHoldingCids,
      ...(preapproval !== undefined
        ? { transferPreapprovalCid: preapproval }
        : {}),
      expectedManagerAddress: this.managerAddress,
    });
    const commandId = transferCommandId({
      user,
      manager: this.managerAddress,
      rawAmount,
      recipientChainId: recipientChain,
      recipient,
      nonce,
    });
    return this.envelope(
      flow,
      NTT_CHOICES.Transfer,
      choiceArgument,
      user,
      commandId
    );
  }

  /**
   * One Transfer command: locks (locking mode) or burns (burning mode) `amount` units at the
   * manager's tokenDecimals from the sender's unlocked holdings, and publishes the NTT message.
   * A configured tokenDecimals, or an options.tokenDecimals, that differs from the manager's is
   * refused, so an amount scaled with the wrong decimals never moves. With neither, nothing
   * checks the caller's scale. An amount with dust the peer cannot represent is refused
   * before anything is built (see
   * sendableAmount). The command id derives from the transfer and the nonce, so a retry with the
   * same nonce is deduplicated. Read the message id from the committed transaction with
   * messageIdFromTransaction or getMessageId.
   */
  async *transfer(
    sender: AccountAddress<C>,
    amount: bigint,
    destination: ChainAddress,
    options: CantonNttTransferOptions
  ): AsyncGenerator<CantonUnsignedTransaction<N, C>> {
    if (options.automatic)
      throw unsupported(
        "transfer with automatic relaying",
        "no relayer serves Canton yet"
      );
    if (options.wrapNative)
      throw unsupported(
        "transfer with wrapNative",
        "Canton has no wrapped native token"
      );
    const user = new CantonAddress(
      sender as unknown as AnyCantonAddress
    ).toParty();
    const nonce = options.nonce ?? randomNonce();
    if (!Number.isInteger(nonce) || nonce < 0 || nonce > 0xffffffff)
      throw new Error(`The transfer nonce must be a uint32, got ${nonce}`);
    const callerDecimals = options.tokenDecimals;
    if (
      callerDecimals !== undefined &&
      (!Number.isInteger(callerDecimals) || callerDecimals < 0)
    )
      throw new Error(
        `The transfer tokenDecimals must be a non-negative integer, got ${callerDecimals}`
      );
    const build = () =>
      this.transferEnvelope(user, amount, destination, nonce, callerDecimals);
    const rebuild = async () => {
      await sleep(this.options.rebuildDelayMs ?? FLOW_REBUILD_DELAY_MS);
      return build();
    };
    yield this.unsignedTransaction(
      await build(),
      `CantonNtt.transfer (nonce ${nonce})`,
      rebuild
    );
  }

  /**
   * The Wormhole message of a committed Transfer: the PublishMessage nested in it whose emitter is
   * this deployment's transceiver, read as the guardian watcher reads it. The sequence must equal
   * the Transfer result's.
   */
  messageIdFromTransaction(transaction: JsTransaction): WormholeMessageId {
    const { messages, dropped } = extractPublishedMessages(transaction.events, {
      guardianGovernance: this.guardianGovernance,
      packageIds: this.corePackageIds,
    });
    const transceiver = new UniversalAddress(this.transceiverAddress());
    const ours = messages.filter((message) =>
      message.emitter.equals(transceiver)
    );
    if (ours.length !== 1)
      throw new Error(
        `Update ${transaction.updateId} has ${ours.length} messages from the transceiver ${transceiver}` +
          (dropped.length > 0 ? `; dropped: ${dropped.join("; ")}` : "")
      );
    const message = ours[0]!;
    const transferResults = transaction.events
      .map(
        (event) =>
          (event as { ExercisedEvent?: Record<string, unknown> }).ExercisedEvent
      )
      .filter(
        (exercised) =>
          exercised?.["choice"] === NTT_CHOICES.Transfer &&
          typeof exercised["templateId"] === "string" &&
          (exercised["templateId"] as string).endsWith(
            `:${NTT_TEMPLATES.NttManager}`
          )
      );
    if (transferResults.length === 1) {
      const result = decodeTransferResult(
        transferResults[0]!["exerciseResult"]
      );
      if (result.sequence !== message.sequence)
        throw new Error(
          `The Transfer result has sequence ${result.sequence}, its PublishMessage ${message.sequence}`
        );
    }
    return {
      chain: this.chain,
      emitter: message.emitter,
      sequence: message.sequence,
    };
  }

  /**
   * The message id of a committed Transfer, read from the participant. The reader (`readAs`,
   * options.readAs, or any party) must be an informee: the sender is one.
   */
  async getMessageId(
    txid: TxHash,
    readAs?: string | readonly string[]
  ): Promise<WormholeMessageId> {
    const parties =
      readAs === undefined
        ? this.options.readAs
        : typeof readAs === "string"
          ? [readAs]
          : readAs;
    const readers: TransactionReaders =
      parties === undefined
        ? { anyParty: true }
        : parties.map((party) => parseCantonPartyId(party));
    return this.messageIdFromTransaction(
      await this.connection.transactionById(txid, readers)
    );
  }

  // ---------------------------------------------------------------------------------------------
  // Redeem
  // ---------------------------------------------------------------------------------------------

  /** The VAA must transfer to the configured manager on this chain */
  private checkInboundVaa(vaa: Ntt.Attestation): void {
    const { payload } = vaa;
    if (
      encoding.hex.encode(payload.recipientNttManager.toUint8Array()) !==
      this.managerAddress
    )
      throw new Error(
        `The VAA is for manager ${payload.recipientNttManager}, not the configured ${this.managerAddress}`
      );
    const recipientChain = payload.nttManagerPayload.payload.recipientChain;
    if ((recipientChain as string) !== (this.chain as string))
      throw new Error(
        `The VAA transfers to ${recipientChain}, not ${this.chain}`
      );
  }

  /**
   * The redeem command. The flow is read at most REDEEM_FLOW_READS times: a second read follows
   * when the first serves a LockedLedger that cannot cover the release (redeemEnvelopeFromFlow).
   */
  private async redeemEnvelope(
    executor: string,
    recipient: string,
    vaaBytes: Uint8Array,
    digest: string,
    wireAmount: CantonTrimmedAmount
  ): Promise<CantonCommandEnvelope> {
    for (let read = 1; read <= REDEEM_FLOW_READS; read++) {
      const envelope = await this.redeemEnvelopeFromFlow(
        executor,
        recipient,
        vaaBytes,
        digest,
        wireAmount,
        read === REDEEM_FLOW_READS
      );
      if (envelope !== undefined) return envelope;
      await sleep(this.options.rebuildDelayMs ?? FLOW_REBUILD_DELAY_MS);
    }
    throw new Error("unreachable: the last flow read returns or throws");
  }

  /**
   * One read of the redeem flow and the command built from it. Returns undefined when the served
   * LockedLedger cannot cover the release and this is not the last read; the last read throws.
   */
  private async redeemEnvelopeFromFlow(
    executor: string,
    recipient: string,
    vaaBytes: Uint8Array,
    digest: string,
    wireAmount: CantonTrimmedAmount,
    lastRead: boolean
  ): Promise<CantonCommandEnvelope | undefined> {
    // The service's mint flow answers 404 when the recipient has no DepositPreapproval, so the
    // flow name follows the summary's mode; the anchored manager must agree.
    const mode = (await this.managerSummary()).tokenConfig;
    const flowName: NttFlowName = mode === "LockUnlock" ? "release" : "mint";
    let flow: VerifiedNttFlow;
    try {
      flow = await this.verifiedFlow(
        flowName,
        { digest, recipient },
        { recipient }
      );
    } catch (err) {
      if (err instanceof NttFlowServiceError && err.notFound)
        throw new Error(
          `${err.message}; nothing has been submitted, the VAA stays redeemable` +
            (flowName === "mint"
              ? " (a burning-mode recipient needs a DepositPreapproval)"
              : "")
        );
      throw err;
    }
    const { manager } = flow;
    if (manager.tokenConfig !== mode)
      throw new NttAnchorError(
        `the ${flowName} flow serves a ${manager.tokenConfig} manager`
      );
    if (manager.paused) throw new NttAnchorError("the NttManager is paused");
    const shared = {
      executor,
      coreStateCid: roleContractId(flow, FLOW_ROLES.CoreState),
      replayNodeCid: roleContractId(flow, FLOW_ROLES.coveringReplayNode),
      vaaBytes,
      recipient,
    };
    const commandId = redeemCommandId(digest);
    if (manager.tokenConfig === "LockUnlock") {
      // Release checks the ledger balance and its custody pot before it moves anything. The flow
      // service caches its reads, so right after a lock it can serve the ledger from before it;
      // the ledger would then refuse the release with an assertion, not as a stale contract, and
      // no signer rebuilds on that. Wait out the cache and read the flow once more instead.
      const ledger = flowLockedLedger(flow);
      const release =
        untrimAmount(wireAmount, manager.tokenDecimals) *
        10n ** BigInt(DAML_DECIMAL_SCALE - manager.tokenDecimals);
      if (release === 0n)
        throw new NttAnchorError(
          `the VAA amount is zero at this token's ${manager.tokenDecimals} decimals`
        );
      if (ledger.balance < release || ledger.custodyHoldingCid === undefined) {
        if (!lastRead) return undefined;
        throw new NttAnchorError(
          (ledger.custodyHoldingCid === undefined
            ? "the LockedLedger names no custody pot"
            : `the LockedLedger balance ${ledger.balance} does not cover the release of ${release} (units of 10^-10)`) +
            "; the VAA stays redeemable"
        );
      }
      const preapproval = flowNeedsTransferPreapproval(flow)
        ? roleContractId(flow, FLOW_ROLES.recipientTransferPreapproval)
        : optionalRoleContractId(flow, FLOW_ROLES.recipientTransferPreapproval);
      return this.envelope(
        flow,
        NTT_CHOICES.Release,
        releaseArgument({
          ...shared,
          ledgerCid: roleContractId(flow, FLOW_ROLES.LockedLedger),
          ...(preapproval !== undefined
            ? { transferPreapprovalCid: preapproval }
            : {}),
        }),
        executor,
        commandId
      );
    }
    return this.envelope(
      flow,
      NTT_CHOICES.Mint,
      mintArgument({
        ...shared,
        depositPreapprovalCid: roleContractId(
          flow,
          FLOW_ROLES.DepositPreapproval
        ),
      }),
      executor,
      commandId
    );
  }

  /**
   * One Release (locking) or Mint (burning) command, with the payer as executor. The recipient is
   * `options.recipient`, or the payer when the payer's party hash is the VAA recipient; the VAA
   * holds only the hash. The command id derives from the VAA digest. After a stale-contract
   * rejection a Canton signer rebuilds it from a fresh flow under the same command id.
   */
  async *redeem(
    attestations: Ntt.Attestation[],
    payer?: AccountAddress<C>,
    options: CantonNttRedeemOptions = {}
  ): AsyncGenerator<CantonUnsignedTransaction<N, C>> {
    if (attestations.length !== 1)
      throw new Error(
        `Canton NTT has one transceiver: expected 1 attestation, got ${attestations.length}`
      );
    if (!payer)
      throw new Error(
        "A Canton redeem needs the payer: it is the executor of the command"
      );
    const executor = new CantonAddress(
      payer as unknown as AnyCantonAddress
    ).toParty();
    const vaa = attestations[0]!;
    this.checkInboundVaa(vaa);
    const transfer = vaa.payload.nttManagerPayload.payload;
    const recipient = parseCantonPartyId(options.recipient ?? executor);
    const recipientHash = encoding.hex.encode(partyAddress(recipient));
    const vaaRecipient = encoding.hex.encode(
      transfer.recipientAddress.toUint8Array()
    );
    if (recipientHash !== vaaRecipient)
      throw new Error(
        options.recipient === undefined
          ? `The VAA recipient 0x${vaaRecipient} is not the payer ${executor}; pass the recipient party in options.recipient`
          : `The VAA recipient 0x${vaaRecipient} is not the hash of ${recipient}`
      );
    const vaaBytes = serialize(vaa);
    const digest = vaaDigest(vaa.hash);
    const build = () =>
      this.redeemEnvelope(
        executor,
        recipient,
        vaaBytes,
        digest,
        transfer.trimmedAmount
      );
    const rebuild = async () => {
      await sleep(this.options.rebuildDelayMs ?? FLOW_REBUILD_DELAY_MS);
      return build();
    };
    yield this.unsignedTransaction(await build(), "CantonNtt.redeem", rebuild);
  }
}
