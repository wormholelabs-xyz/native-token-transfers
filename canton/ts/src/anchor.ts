// Trust anchor of the NTT manager (plan 3.5). The user's participant cannot read the NttManager,
// so the flow service names it. Every flow passes these checks before a command is built:
//
//   Layer 1: every disclosed ntt and core contract carries a pinned package id.
//   Layer 2: the flow's manager contract id equals the /v1/managers entry of the configured
//            manager address, with the same mode, instrument and reserve custodian. Same service,
//            so it catches mistakes, not a hostile service.
//   Layer 3: the blob envelope of the NttManager lists the configured guardianGovernance and the
//            configured reserve custodian as signatories, and the create argument read from that
//            blob (not the served payload) has the configured manager address, guardianGovernance
//            and reserve custodian, with reserveCustodian = admin, or nttCustody when gg is admin.
//            The ledger binds the blob to its contract id, so these fields are the ones the
//            command runs against.
//   Layer 4: Transfer carries expectedManagerAddress (nttContracts.ts); the ledger aborts on a
//            mismatch.
//
// nttCustody signs every manager (it is inherited from the NttGovernance root), so a signatory
// check alone does not pin a custodian that is nttCustody: the decoded admin must then be gg.
import type {
  CantonInstrumentId,
  DisclosedContract,
} from "@wormhole-foundation/sdk-canton";
import {
  checkPinnedPackage,
  createArgumentToJson,
  readCreatedEventBlob,
  templateModuleAndEntity,
} from "@wormhole-foundation/sdk-canton";
import type { CoreStateView } from "@wormhole-foundation/sdk-canton-core";
import {
  CORE_STATE_LAYOUT,
  decodeCoreState,
} from "@wormhole-foundation/sdk-canton-core";
import type { NttFlow, NttFlowName, NttManagerSummary } from "./flowSource.js";
import type {
  AdminTransferProposalView,
  LockedLedgerView,
  NttManagerView,
  NttTokenConfig,
} from "./nttContracts.js";
import {
  ADMIN_TRANSFER_PROPOSAL_LAYOUT,
  FLOW_CORE_TEMPLATES,
  FLOW_ROLES,
  LOCKED_LEDGER_LAYOUT,
  NTT_MANAGER_LAYOUT,
  NTT_TEMPLATES,
  TOKEN_CIP0056_TEMPLATES,
  decodeAdminTransferProposal,
  decodeLockedLedger,
  decodeNttManager,
  decodeReplayNode,
  replayNodeRecordsDigest,
} from "./nttContracts.js";

/** What a flow must satisfy */
export interface NttAnchorPolicy {
  /** The configured guardianGovernance party (contracts.coreBridge) */
  guardianGovernance: string;
  /** The configured manager address, lowercase hex */
  managerAddress: string;
  /** The configured reserve custodian. Required for a transfer (layer 3). */
  reserveCustodian?: string;
  /** Pinned ntt package ids */
  nttPackageIds: readonly string[];
  /** Pinned wormhole-core package ids */
  corePackageIds: readonly string[];
  /** The configured mode, when the config has one */
  tokenConfig?: NttTokenConfig;
  /** The configured instrument (contracts.ntt.token), when it is one */
  instrument?: CantonInstrumentId;
}

/** A flow that passed the anchor checks */
export interface VerifiedNttFlow {
  flow: NttFlowName;
  /** The NttManager fields, read from its anchored blob */
  manager: NttManagerView;
  /** The served entries by role, for the roles this client knows */
  roles: ReadonlyMap<string, DisclosedContract>;
  /** The entries to attach to the command, in flow order */
  disclosures: DisclosedContract[];
  /** Roles the service sent that this client does not know; they are not attached */
  ignoredRoles: string[];
  synchronizerId: string;
  /** The /v1/managers entry of the manager */
  summary: NttManagerSummary;
}

/** Why a flow was refused; nothing has been submitted */
export class NttAnchorError extends Error {
  constructor(message: string) {
    super(`${message}; nothing has been submitted`);
    this.name = "NttAnchorError";
  }
}

interface Envelope {
  signatories: string[];
  createArgument: Uint8Array;
}

/** The blob envelope of a disclosed contract; its contract id and template id must match */
function envelopeOf(contract: DisclosedContract): Envelope {
  let envelope;
  try {
    envelope = readCreatedEventBlob(contract.createdEventBlob);
  } catch (err) {
    throw new NttAnchorError(
      `${contract.role} ${contract.contractId}: ${(err as Error).message}`
    );
  }
  if (envelope.contractId !== contract.contractId)
    throw new NttAnchorError(
      `${contract.role}: the blob is for contract ${envelope.contractId}, not ${contract.contractId}`
    );
  if (envelope.templateId !== contract.templateId)
    throw new NttAnchorError(
      `${contract.role}: the blob has template ${envelope.templateId}, not ${contract.templateId}`
    );
  return envelope;
}

function requireSignatory(
  contract: DisclosedContract,
  envelope: Envelope,
  party: string
): void {
  if (!envelope.signatories.includes(party))
    throw new NttAnchorError(
      `${contract.role} ${contract.contractId} is not signed by ${party}`
    );
}

function requireTemplate(
  contract: DisclosedContract,
  moduleAndEntity: string
): void {
  let actual: string;
  try {
    actual = templateModuleAndEntity(contract.templateId);
  } catch (err) {
    throw new NttAnchorError(`${contract.role}: ${(err as Error).message}`);
  }
  if (actual !== moduleAndEntity)
    throw new NttAnchorError(
      `${contract.role} has template ${contract.templateId.slice(0, 200)}, expected ${moduleAndEntity}`
    );
}

function requirePackage(
  contract: DisclosedContract,
  packageIds: readonly string[]
): void {
  try {
    checkPinnedPackage(contract, packageIds);
  } catch (err) {
    throw new NttAnchorError(`layer 1: ${(err as Error).message}`);
  }
}

/** reserveCustodianFor (Manager.daml): nttCustody while gg is admin, else admin */
export function reserveCustodianFor(manager: {
  guardianGovernance: string;
  nttCustody: string;
  admin: string;
}): string {
  return manager.admin === manager.guardianGovernance
    ? manager.nttCustody
    : manager.admin;
}

/**
 * Checks the NttManager entry (layers 1 and 3) and returns its fields, read from the blob. The
 * served payload is not used.
 */
export function checkManagerContract(
  contract: DisclosedContract,
  policy: NttAnchorPolicy
): NttManagerView {
  requireTemplate(contract, NTT_TEMPLATES.NttManager);
  requirePackage(contract, policy.nttPackageIds);
  const envelope = envelopeOf(contract);
  requireSignatory(contract, envelope, policy.guardianGovernance);
  if (policy.reserveCustodian !== undefined)
    requireSignatory(contract, envelope, policy.reserveCustodian);
  let manager: NttManagerView;
  try {
    manager = decodeNttManager(
      createArgumentToJson(envelope.createArgument, NTT_MANAGER_LAYOUT)
    );
  } catch (err) {
    throw new NttAnchorError(
      `NttManager ${contract.contractId}: ${(err as Error).message}`
    );
  }
  // A set: when gg is admin, the manager has two signatories.
  const signatories = [
    ...new Set([manager.admin, manager.guardianGovernance, manager.nttCustody]),
  ].sort();
  if (
    signatories.length !== envelope.signatories.length ||
    signatories.some((party, index) => party !== envelope.signatories[index])
  )
    throw new NttAnchorError(
      `layer 3: the NttManager signatories ${envelope.signatories.join(", ")} are not its admin, guardianGovernance and nttCustody`
    );
  if (manager.guardianGovernance !== policy.guardianGovernance)
    throw new NttAnchorError(
      `layer 3: the NttManager has guardianGovernance ${manager.guardianGovernance}, not ${policy.guardianGovernance}`
    );
  if (manager.managerAddress !== policy.managerAddress)
    throw new NttAnchorError(
      `layer 3: the NttManager has address ${manager.managerAddress}, not the configured ${policy.managerAddress}`
    );
  if (
    manager.reserveCustodian !== reserveCustodianFor(manager) ||
    manager.reserveCustodian === manager.guardianGovernance
  )
    throw new NttAnchorError(
      `layer 3: the NttManager reserveCustodian ${manager.reserveCustodian} is not its admin, or nttCustody under a gg admin`
    );
  if (
    policy.reserveCustodian !== undefined &&
    manager.reserveCustodian !== policy.reserveCustodian
  )
    throw new NttAnchorError(
      `layer 3: the NttManager reserveCustodian ${manager.reserveCustodian} differs from the configured ${policy.reserveCustodian}`
    );
  if (
    policy.tokenConfig !== undefined &&
    manager.tokenConfig !== policy.tokenConfig
  )
    throw new NttAnchorError(
      `the NttManager tokenConfig ${manager.tokenConfig} does not match the configured ${policy.tokenConfig}`
    );
  if (
    policy.instrument !== undefined &&
    (manager.instrumentId.admin !== policy.instrument.admin ||
      manager.instrumentId.id !== policy.instrument.id)
  )
    throw new NttAnchorError(
      "the NttManager instrument differs from the configured token"
    );
  return manager;
}

/** Layer 2: the /v1/managers entry of the manager agrees with the flow */
export function checkManagerSummary(
  managers: readonly NttManagerSummary[],
  managerContract: DisclosedContract,
  manager: NttManagerView
): NttManagerSummary {
  const matches = managers.filter(
    (entry) => entry.managerAddress === manager.managerAddress
  );
  if (matches.length !== 1)
    throw new NttAnchorError(
      `layer 2: /v1/managers lists ${matches.length} managers with address ${manager.managerAddress}`
    );
  const summary = matches[0]!;
  if (summary.contractId !== managerContract.contractId)
    throw new NttAnchorError(
      "layer 2: the NttManager contract id of the flow differs from the /v1/managers entry"
    );
  if (summary.tokenConfig !== manager.tokenConfig)
    throw new NttAnchorError(
      `layer 2: /v1/managers has tokenConfig ${summary.tokenConfig}, the NttManager ${manager.tokenConfig}`
    );
  if (
    summary.instrumentId.admin !== manager.instrumentId.admin ||
    summary.instrumentId.id !== manager.instrumentId.id
  )
    throw new NttAnchorError(
      "layer 2: /v1/managers names another instrument than the NttManager"
    );
  if (summary.reserveCustodian !== manager.reserveCustodian)
    throw new NttAnchorError(
      `layer 2: /v1/managers has reserveCustodian ${summary.reserveCustodian}, the NttManager ${manager.reserveCustodian}`
    );
  return summary;
}

/** The roles a flow must and may hold, and the check of each */
interface RoleSpec {
  required: boolean;
  check: (contract: DisclosedContract) => void;
}

/** Parameters of a flow beyond the policy */
export interface FlowContext {
  /** The redeem recipient (release, mint) */
  recipient?: string;
  /**
   * The roles the caller's command uses, when it uses only part of the flow (an admin command
   * reads the transfer flow for the CoreState and the transceiver Emitter). A role outside this
   * list is still checked when served, but its absence, or the service reporting it missing, is
   * not an error. Default: every role of the flow.
   */
  uses?: readonly string[];
}

function roleSpecs(
  flow: NttFlowName,
  manager: NttManagerView,
  policy: NttAnchorPolicy,
  context: FlowContext
): Map<string, RoleSpec> {
  const gg = policy.guardianGovernance;
  const coreContract =
    (moduleAndEntity: string) => (contract: DisclosedContract) => {
      requireTemplate(contract, moduleAndEntity);
      requirePackage(contract, policy.corePackageIds);
      requireSignatory(contract, envelopeOf(contract), gg);
    };
  const committedFactory =
    (tag: NttManagerView["factory"]["tag"]) =>
    (contract: DisclosedContract) => {
      if (
        manager.factory.tag !== tag ||
        manager.factory.contractId !== contract.contractId
      )
        throw new NttAnchorError(
          `${contract.role} ${contract.contractId} is not the factory the NttManager commits (${manager.factory.contractId})`
        );
      envelopeOf(contract);
    };
  const preapprovalOf = (owner: string) => (contract: DisclosedContract) => {
    requireTemplate(contract, TOKEN_CIP0056_TEMPLATES.TransferPreapproval);
    requireSignatory(contract, envelopeOf(contract), owner);
  };
  const specs = new Map<string, RoleSpec>();
  const add = (
    role: string,
    required: boolean,
    check: (contract: DisclosedContract) => void
  ) => specs.set(role, { required, check });
  // The NttManager is checked on its own (checkManagerContract).
  add(FLOW_ROLES.NttManager, true, () => {});
  add(FLOW_ROLES.CoreState, true, coreContract(FLOW_CORE_TEMPLATES.CoreState));
  const locking = manager.tokenConfig === "LockUnlock";
  const lockingEntries = () => {
    add(FLOW_ROLES.LockedLedger, true, (contract) => {
      requireTemplate(contract, NTT_TEMPLATES.LockedLedger);
      requirePackage(contract, policy.nttPackageIds);
      requireSignatory(contract, envelopeOf(contract), gg);
    });
    add(
      FLOW_ROLES.committedTransferFactory,
      true,
      committedFactory("TransferFactoryCid")
    );
    add(FLOW_ROLES.custodyPotHolding, false, (contract) => {
      requireSignatory(
        contract,
        envelopeOf(contract),
        manager.reserveCustodian
      );
    });
  };
  switch (flow) {
    case "transfer":
      add(
        FLOW_ROLES.transceiverEmitter,
        true,
        coreContract(FLOW_CORE_TEMPLATES.Emitter)
      );
      if (locking) {
        lockingEntries();
        add(
          FLOW_ROLES.custodianTransferPreapproval,
          false,
          preapprovalOf(manager.reserveCustodian)
        );
      } else {
        add(FLOW_ROLES.committedCanonicalCoinFactory, true, (contract) => {
          requireTemplate(contract, TOKEN_CIP0056_TEMPLATES.CoinFactory);
          committedFactory("BurnMintFactoryCid")(contract);
          requireSignatory(contract, envelopeOf(contract), gg);
        });
      }
      break;
    case "set-peer":
      // The governance flow (SetPeerByVaa); this client reads it for the covering ReplayNode.
      add(
        FLOW_ROLES.transceiverEmitter,
        true,
        coreContract(FLOW_CORE_TEMPLATES.Emitter)
      );
      add(
        FLOW_ROLES.coveringReplayNode,
        true,
        coreContract(FLOW_CORE_TEMPLATES.ReplayNode)
      );
      break;
    case "release":
    case "mint": {
      if (context.recipient === undefined)
        throw new Error(`The ${flow} flow needs a recipient`);
      add(
        FLOW_ROLES.coveringReplayNode,
        true,
        coreContract(FLOW_CORE_TEMPLATES.ReplayNode)
      );
      if (flow === "release") {
        lockingEntries();
        add(
          FLOW_ROLES.recipientTransferPreapproval,
          false,
          preapprovalOf(context.recipient)
        );
      } else {
        const recipient = context.recipient;
        add(FLOW_ROLES.DepositPreapproval, true, (contract) => {
          requireTemplate(contract, NTT_TEMPLATES.DepositPreapproval);
          requirePackage(contract, policy.nttPackageIds);
          requireSignatory(contract, envelopeOf(contract), recipient);
        });
        add(FLOW_ROLES.committedCanonicalCoinFactory, true, (contract) => {
          requireTemplate(contract, TOKEN_CIP0056_TEMPLATES.CoinFactory);
          committedFactory("BurnMintFactoryCid")(contract);
          requireSignatory(contract, envelopeOf(contract), gg);
        });
      }
      break;
    }
  }
  return specs;
}

/**
 * True when the committed transfer factory is the token-cip0056 CoinFactory, whose transfers to a
 * third party settle only with the receiver's TransferPreapproval in the choice context.
 */
function needsTransferPreapproval(
  roles: ReadonlyMap<string, DisclosedContract>
): boolean {
  const factory = roles.get(FLOW_ROLES.committedTransferFactory);
  if (!factory) return false;
  try {
    return (
      templateModuleAndEntity(factory.templateId) ===
      TOKEN_CIP0056_TEMPLATES.CoinFactory
    );
  } catch {
    return false;
  }
}

const PREAPPROVAL_ROLES: readonly string[] = [
  FLOW_ROLES.custodianTransferPreapproval,
  FLOW_ROLES.recipientTransferPreapproval,
];

/**
 * Runs the anchor checks on a flow: layers 1 and 3 on the NttManager and on every other entry,
 * layer 2 against /v1/managers. Returns the anchored manager fields and the entries to attach.
 */
export function verifyFlow(
  flow: NttFlow,
  managers: readonly NttManagerSummary[],
  policy: NttAnchorPolicy,
  context: FlowContext = {}
): VerifiedNttFlow {
  const managerEntries = flow.disclosures.filter(
    (entry) => entry.role === FLOW_ROLES.NttManager
  );
  if (managerEntries.length !== 1)
    throw new NttAnchorError(`the ${flow.flow} flow has no NttManager role`);
  const managerContract = managerEntries[0]!;
  const manager = checkManagerContract(managerContract, policy);
  const summary = checkManagerSummary(managers, managerContract, manager);

  const specs = roleSpecs(flow.flow, manager, policy, context);
  const roles = new Map<string, DisclosedContract>();
  const disclosures: DisclosedContract[] = [];
  const ignoredRoles: string[] = [];
  for (const entry of flow.disclosures) {
    const spec = specs.get(entry.role);
    if (!spec) {
      ignoredRoles.push(entry.role);
      continue;
    }
    spec.check(entry);
    roles.set(entry.role, entry);
    disclosures.push(entry);
  }
  const preapprovalNeeded = needsTransferPreapproval(roles);
  const used = (role: string) =>
    role === FLOW_ROLES.NttManager ||
    context.uses === undefined ||
    context.uses.includes(role);
  const missing = flow.missingRoles.filter(
    (role) =>
      used(role) && !(PREAPPROVAL_ROLES.includes(role) && !preapprovalNeeded)
  );
  if (missing.length > 0)
    throw new NttAnchorError(
      `the ${flow.flow} flow cannot supply: ${missing.map((role) => role.slice(0, 64)).join(", ")}`
    );
  for (const [role, spec] of specs)
    if (spec.required && used(role) && !roles.has(role))
      throw new NttAnchorError(`the ${flow.flow} flow has no "${role}" role`);
  const synchronizerId = managerContract.synchronizerId;
  if (disclosures.some((entry) => entry.synchronizerId !== synchronizerId))
    throw new NttAnchorError(
      `the ${flow.flow} flow names more than one synchronizer`
    );
  return {
    flow: flow.flow,
    manager,
    roles,
    disclosures,
    ignoredRoles,
    synchronizerId,
    summary,
  };
}

/** The decoded CoreState of a verified flow, read from its anchored blob */
export function flowCoreState(flow: VerifiedNttFlow): CoreStateView {
  const contract = flow.roles.get(FLOW_ROLES.CoreState);
  if (!contract) throw new Error(`The ${flow.flow} flow has no CoreState`);
  return decodeCoreState(
    createArgumentToJson(
      readCreatedEventBlob(contract.createdEventBlob).createArgument,
      CORE_STATE_LAYOUT
    )
  );
}

/**
 * The decoded LockedLedger of a verified flow, read from its blob (its gg signature and package
 * were checked with the flow). It must be the configured manager's.
 */
export function flowLockedLedger(flow: VerifiedNttFlow): LockedLedgerView {
  const contract = flow.roles.get(FLOW_ROLES.LockedLedger);
  if (!contract)
    throw new NttAnchorError(
      `the ${flow.flow} flow has no "LockedLedger" role`
    );
  const ledger = decodeLockedLedger(
    createArgumentToJson(
      readCreatedEventBlob(contract.createdEventBlob).createArgument,
      LOCKED_LEDGER_LAYOUT
    )
  );
  if (
    ledger.managerAddress !== flow.manager.managerAddress ||
    ledger.guardianGovernance !== flow.manager.guardianGovernance
  )
    throw new NttAnchorError(
      `the LockedLedger ${contract.contractId} is another deployment's`
    );
  return ledger;
}

/** The contract id of a role of a verified flow */
export function roleContractId(flow: VerifiedNttFlow, role: string): string {
  const contract = flow.roles.get(role);
  if (!contract)
    throw new NttAnchorError(`the ${flow.flow} flow has no "${role}" role`);
  return contract.contractId;
}

/** The contract id of an optional role, when the flow has it */
export function optionalRoleContractId(
  flow: VerifiedNttFlow,
  role: string
): string | undefined {
  return flow.roles.get(role)?.contractId;
}

/** True when the flow's committed transfer factory needs a receiver TransferPreapproval */
export function flowNeedsTransferPreapproval(flow: VerifiedNttFlow): boolean {
  return needsTransferPreapproval(flow.roles);
}

/** The checked entries of a verified flow for the given roles, in flow order */
export function flowDisclosures(
  flow: VerifiedNttFlow,
  roles: readonly string[]
): DisclosedContract[] {
  return flow.disclosures.filter((entry) => roles.includes(entry.role));
}

/**
 * Whether the covering ReplayNode of a verified flow records a VAA digest as consumed. The node
 * passed the flow's checks (pinned core package, signed by gg); its create argument is read from
 * the blob, not from the served payload, and must be the node of this deployment's trie: consumer
 * and guardianGovernance gg, namespace the anchored manager's, and a prefix that starts the digest.
 * Any layout difference throws (decodeReplayNode).
 *
 * A true answer is final: a digest never leaves the trie, and gg signs every node. A false answer
 * is only as fresh as the service's view: a stale or withheld node reads as not consumed.
 */
export function flowReplayNodeRecordsDigest(
  flow: VerifiedNttFlow,
  digest: string
): boolean {
  const contract = flow.roles.get(FLOW_ROLES.coveringReplayNode);
  if (!contract)
    throw new NttAnchorError(
      `the ${flow.flow} flow has no "${FLOW_ROLES.coveringReplayNode}" role`
    );
  let node;
  try {
    node = decodeReplayNode(envelopeOf(contract).createArgument);
  } catch (err) {
    if (err instanceof NttAnchorError) throw err;
    throw new NttAnchorError(
      `${contract.role} ${contract.contractId}: ${(err as Error).message}`
    );
  }
  const gg = flow.manager.guardianGovernance;
  if (node.consumer !== gg || node.guardianGovernance !== gg)
    throw new NttAnchorError(
      `the ReplayNode ${contract.contractId} is not in the trie of ${gg}`
    );
  if (node.namespace !== flow.manager.namespace)
    throw new NttAnchorError(
      `the ReplayNode ${contract.contractId} has namespace ${node.namespace.slice(0, 140)}, not the NttManager's ${flow.manager.namespace}`
    );
  try {
    return replayNodeRecordsDigest(node, digest);
  } catch (err) {
    throw new NttAnchorError((err as Error).message);
  }
}

/**
 * Checks an AdminTransferProposal read from a participant's own ACS: the template, a pinned ntt
 * package, and its one signatory (the proposing admin) in the blob envelope. Returns its fields,
 * read from the blob.
 */
export function checkAdminTransferProposal(
  contract: DisclosedContract,
  nttPackageIds: readonly string[]
): AdminTransferProposalView {
  requireTemplate(contract, NTT_TEMPLATES.AdminTransferProposal);
  requirePackage(contract, nttPackageIds);
  const envelope = envelopeOf(contract);
  let proposal: AdminTransferProposalView;
  try {
    proposal = decodeAdminTransferProposal(
      createArgumentToJson(
        envelope.createArgument,
        ADMIN_TRANSFER_PROPOSAL_LAYOUT
      )
    );
  } catch (err) {
    throw new NttAnchorError(
      `AdminTransferProposal ${contract.contractId}: ${(err as Error).message}`
    );
  }
  if (
    envelope.signatories.length !== 1 ||
    envelope.signatories[0] !== proposal.admin
  )
    throw new NttAnchorError(
      `the AdminTransferProposal ${contract.contractId} is not signed by its admin ${proposal.admin} alone`
    );
  return proposal;
}
