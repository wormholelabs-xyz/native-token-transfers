// Pure functions over a parsed active-contracts response: recursive entry
// collection, template-suffix finders, and coin selection. Contract ids
// rotate on every consuming choice, so callers must re-run these against a
// fresh snapshot for every operation — never cache a resolved cid.
import { partyId, type CantonChainConfig, type CantonConnection } from "../config.js";
import type { CantonClient } from "./client.js";

export interface AcsEntry {
  templateId: string;
  contractId: string;
  createArgument: Record<string, unknown>;
}

/**
 * Recursively walks a parsed active-contracts response, collecting every
 * object with templateId/contractId/createArgument — tolerant of whatever
 * wrapper shape the JSON Ledger API nests entries in.
 */
export function collectEntries(acs: unknown): AcsEntry[] {
  const results: AcsEntry[] = [];
  walk(acs, results);
  return results;
}

function walk(node: unknown, out: AcsEntry[]): void {
  if (node === null || typeof node !== "object") return;
  if (Array.isArray(node)) {
    for (const item of node) walk(item, out);
    return;
  }
  const obj = node as Record<string, unknown>;
  if (
    typeof obj.templateId === "string" &&
    typeof obj.contractId === "string" &&
    typeof obj.createArgument === "object" &&
    obj.createArgument !== null &&
    !Array.isArray(obj.createArgument)
  ) {
    out.push({
      templateId: obj.templateId,
      contractId: obj.contractId,
      createArgument: obj.createArgument as Record<string, unknown>,
    });
  }
  for (const value of Object.values(obj)) walk(value, out);
}

function bySuffix(entries: AcsEntry[], suffix: string): AcsEntry[] {
  return entries.filter((e) => e.templateId.endsWith(suffix));
}

export function findCoreState(entries: AcsEntry[]): AcsEntry | undefined {
  return bySuffix(entries, ":Wormhole.Core.State:CoreState")[0];
}

export function findNttManager(
  entries: AcsEntry[],
  managerAddress: string
): AcsEntry | undefined {
  return bySuffix(entries, ":Wormhole.Ntt.Manager:NttManager").find(
    (e) => e.createArgument["managerAddress"] === managerAddress
  );
}

export function findEmitter(
  entries: AcsEntry[],
  owner: string,
  emitterId: string
): AcsEntry | undefined {
  return bySuffix(entries, ":Wormhole.Core.State:Emitter").find(
    (e) =>
      e.createArgument["owner"] === owner &&
      e.createArgument["emitterId"] === emitterId
  );
}

export function findLockedLedger(
  entries: AcsEntry[],
  managerAddress: string
): AcsEntry | undefined {
  return bySuffix(entries, ":Wormhole.Ntt.Ledger:LockedLedger").find(
    (e) => e.createArgument["managerAddress"] === managerAddress
  );
}

export function findDepositPreapproval(
  entries: AcsEntry[],
  owner: string,
  instrumentId: string
): AcsEntry | undefined {
  return bySuffix(entries, ":Wormhole.Ntt.Deposit:DepositPreapproval").find(
    (e) => {
      const inst = e.createArgument["instrumentId"] as
        | { id?: string }
        | undefined;
      return e.createArgument["owner"] === owner && inst?.id === instrumentId;
    }
  );
}

export function findReplayNode(
  entries: AcsEntry[],
  namespace: string
): AcsEntry | undefined {
  return bySuffix(entries, ":Wormhole.Core.Replay:ReplayNode").find(
    (e) => e.createArgument["namespace"] === namespace
  );
}

export function findCoins(
  entries: AcsEntry[],
  owner: string,
  instrumentId: string
): AcsEntry[] {
  return bySuffix(entries, ":Token.CIP0056.Coin:Coin").filter((e) => {
    const inst = e.createArgument["instrumentId"] as
      | { id?: string }
      | undefined;
    return e.createArgument["owner"] === owner && inst?.id === instrumentId;
  });
}

/**
 * Picks the first holding (in ACS order) with amount >= rawAmount, excluding
 * the custody holding. Not the smallest-sufficient holding — matches
 * ntt-lib.sh's `jq ... | [0]` semantics exactly.
 *
 * Coin.amount on the ledger is a DAML Decimal string ("5.0000000000"), so the
 * comparison scales both sides to raw units at `decimals`.
 */
export function selectCoin(
  coins: AcsEntry[],
  rawAmount: bigint,
  decimals: number,
  custodyCid: string | undefined
): AcsEntry {
  const candidates = coins.filter((c) => c.contractId !== custodyCid);
  const match = candidates.find(
    (c) => decimalToRaw(String(c.createArgument["amount"]), decimals) >= rawAmount
  );
  if (!match) {
    const available = candidates
      .map((c) => `${c.contractId.slice(0, 16)}...=${c.createArgument["amount"]}`)
      .join(", ");
    throw new Error(
      `no single holding with amount >= ${rawAmount} raw (excluding custody); ` +
        `available: ${available || "(none)"}`
    );
  }
  return match;
}

/** DAML Decimal string -> raw bigint at `decimals`; truncates excess fraction digits. */
function decimalToRaw(value: string, decimals: number): bigint {
  const [whole, frac = ""] = value.split(".");
  const fracPadded = frac.slice(0, decimals).padEnd(decimals, "0");
  return BigInt(whole + fracPadded);
}

export interface ResolvedCantonState {
  coreStateCid: string;
  managerCid: string;
  // ACS-sourced templateId (package id + module + entity) for the found
  // NttManager — upgrade-safe exercise commands must use this, not a
  // hardcoded package id.
  managerTemplateId: string;
  namespace: string;
  transceiverEmitterId: string;
  emitterCid: string;
  ledgerCid?: string;
  custodyCid?: string;
  lockedBalance?: string;
  depositPreapprovalCid?: string;
  replayNodeCid: string;
  coins: AcsEntry[];
}

/**
 * ledgerEnd -> activeContracts -> finders, in one shot. Never cache the
 * result across operations: cids rotate on every consuming choice.
 */
export async function snapshot(
  client: CantonClient,
  canton: CantonConnection,
  chain: CantonChainConfig,
  opts: { recipientParty?: string } = {}
): Promise<ResolvedCantonState> {
  const user = partyId(canton, canton.parties.user);
  const operator = partyId(canton, canton.parties.operator);
  const governance = partyId(canton, canton.parties.governance);
  const parties = [user, operator, governance];

  const offset = await client.ledgerEnd();
  const raw = await client.activeContracts(parties, offset);
  const entries = collectEntries(raw);

  const coreState = findCoreState(entries);
  if (!coreState) {
    throw new Error(`no CoreState visible to [${parties.join(", ")}]`);
  }

  const manager = findNttManager(entries, chain.manager);
  if (!manager) {
    throw new Error(
      `no NttManager with managerAddress ${chain.manager} visible to [${parties.join(", ")}]`
    );
  }
  const namespace = String(manager.createArgument["namespace"] ?? "");
  const transceiverEmitterId = String(
    manager.createArgument["transceiverEmitterId"] ?? ""
  );

  const emitter = findEmitter(entries, governance, transceiverEmitterId);
  if (!emitter) {
    throw new Error(
      `no Emitter with owner ${governance} and emitterId ${transceiverEmitterId} visible to [${parties.join(", ")}]`
    );
  }

  const replayNode = findReplayNode(entries, namespace);
  if (!replayNode) {
    throw new Error(
      `no ReplayNode with namespace ${namespace} visible to [${parties.join(", ")}]`
    );
  }

  const lockedLedger = findLockedLedger(entries, chain.manager);
  const recipientParty = opts.recipientParty ?? user;
  const depositPreapproval = findDepositPreapproval(
    entries,
    recipientParty,
    chain.instrument
  );
  const coins = findCoins(entries, user, chain.instrument);

  return {
    coreStateCid: coreState.contractId,
    managerCid: manager.contractId,
    managerTemplateId: manager.templateId,
    namespace,
    transceiverEmitterId,
    emitterCid: emitter.contractId,
    ledgerCid: lockedLedger?.contractId,
    custodyCid: lockedLedger
      ? String(lockedLedger.createArgument["custodyHoldingCid"] ?? "")
      : undefined,
    lockedBalance: lockedLedger
      ? String(lockedLedger.createArgument["balance"] ?? "0")
      : undefined,
    depositPreapprovalCid: depositPreapproval?.contractId,
    replayNodeCid: replayNode.contractId,
    coins,
  };
}
