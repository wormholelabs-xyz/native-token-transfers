// Daml-facing definitions of the ntt package (ntt 0.7.0, Wormhole.Ntt.*): template names, flow
// roles, record layouts, payload decoders and choice arguments. Everything that depends on the
// Daml source lives here, so a new ntt version changes this file (plan risk NR1).
import type {
  DamlJson,
  LfType,
  LfValue,
} from "@wormhole-foundation/sdk-canton";
import {
  DAML_DECIMAL_SCALE,
  decodeBytesHex,
  decodeContractId,
  decodeDecimal,
  holdingAmountRaw,
  decodeInt,
  decodeParty,
  decodeText,
  encodeBytes,
  encodeContractId,
  encodeInt,
  encodeParty,
  lfType,
  readValue,
} from "@wormhole-foundation/sdk-canton";
import type { WormholeMessageView } from "@wormhole-foundation/sdk-canton-core";
import { decodePublishMessageResult } from "@wormhole-foundation/sdk-canton-core";

/** "<module>:<entity>" of the ntt templates */
export const NTT_TEMPLATES = {
  NttManager: "Wormhole.Ntt.Manager:NttManager",
  LockedLedger: "Wormhole.Ntt.Ledger:LockedLedger",
  DepositPreapproval: "Wormhole.Ntt.Deposit:DepositPreapproval",
  AdminTransferProposal: "Wormhole.Ntt.Manager:AdminTransferProposal",
} as const;

/** "<module>:<entity>" of the token-cip0056 templates the flows serve */
export const TOKEN_CIP0056_TEMPLATES = {
  CoinFactory: "Token.CIP0056.CoinFactory:CoinFactory",
  TransferPreapproval: "Token.CIP0056.CoinTransfer:TransferPreapproval",
} as const;

/** "<module>:<entity>" of the core templates the flows serve */
export const FLOW_CORE_TEMPLATES = {
  CoreState: "Wormhole.Core.State:CoreState",
  Emitter: "Wormhole.Core.State:Emitter",
  ReplayNode: "Wormhole.Core.Replay:ReplayNode",
} as const;

/** Choices this client exercises: on NttManager, and AcceptAdminTransfer on AdminTransferProposal */
export const NTT_CHOICES = {
  Transfer: "Transfer",
  Release: "Release",
  Mint: "Mint",
  SetPaused: "SetPaused",
  SetPeer: "SetPeer",
  AcceptAdminTransfer: "AcceptAdminTransfer",
} as const;

/** Role labels of the flow service (canton/disclosure/service/flows.go) */
export const FLOW_ROLES = {
  NttManager: "NttManager",
  CoreState: "CoreState",
  transceiverEmitter: "transceiver Emitter",
  coveringReplayNode: "covering ReplayNode",
  LockedLedger: "LockedLedger",
  custodyPotHolding: "custody pot Holding",
  committedTransferFactory: "committed TransferFactory",
  committedCanonicalCoinFactory: "committed canonical CoinFactory",
  recipientTransferPreapproval: "recipient TransferPreapproval",
  custodianTransferPreapproval: "custodian TransferPreapproval",
  DepositPreapproval: "DepositPreapproval",
} as const;

/** The token-cip0056 context key of a receiver's TransferPreapproval (CoinTransfer.daml) */
export const TRANSFER_PREAPPROVAL_CONTEXT_KEY =
  "token-cip0056.transferPreapprovalCid";

// ---------------------------------------------------------------------------------------------
// Record layouts, in the field order of the Daml source (ntt 0.7.0). A create argument read from
// a created event blob has no field labels; these layouts are the only place that knows the order.
// ---------------------------------------------------------------------------------------------

/** Splice.Api.Token.HoldingV1.InstrumentId */
const INSTRUMENT_ID: LfType = lfType.record("InstrumentId", [
  ["admin", lfType.party],
  ["id", lfType.text],
]);

/** Wormhole.Ntt.Manager.Peer */
const PEER: LfType = lfType.record("Peer", [
  ["managerAddress", lfType.bytes("NttManagerAddress")],
  ["transceiverAddress", lfType.bytes("EmitterAddress")],
  ["decimals", lfType.int64],
]);

/** Wormhole.Ntt.Manager.NttManager (ntt 0.7.0) */
export const NTT_MANAGER_LAYOUT: LfType = lfType.record("NttManager", [
  ["namespace", lfType.text],
  ["admin", lfType.party],
  ["guardianGovernance", lfType.party],
  ["nttCustody", lfType.party],
  ["reserveCustodian", lfType.party],
  ["chainId", lfType.int64],
  ["managerId", lfType.int64],
  ["managerAddress", lfType.bytes("NttManagerAddress")],
  ["transceiverEmitterId", lfType.int64],
  ["instrumentId", INSTRUMENT_ID],
  ["tokenConfig", lfType.enum("NttTokenConfig", ["LockUnlock", "BurnMint"])],
  ["tokenDecimals", lfType.int64],
  ["tokenAddress", lfType.bytes("UniversalAddress")],
  ["peers", lfType.map(lfType.int64, PEER)],
  [
    "factory",
    lfType.variant("NttFactory", {
      TransferFactoryCid: lfType.contractId,
      BurnMintFactoryCid: lfType.contractId,
    }),
  ],
  ["paused", lfType.bool],
  ["factoryEpoch", lfType.int64],
  ["peerEpoch", lfType.int64],
]);

/** Wormhole.Ntt.Ledger.LockedLedger (ntt 0.7.0) */
export const LOCKED_LEDGER_LAYOUT: LfType = lfType.record("LockedLedger", [
  ["guardianGovernance", lfType.party],
  ["admin", lfType.party],
  ["managerAddress", lfType.bytes("NttManagerAddress")],
  ["balance", lfType.numeric],
  ["custodyHoldingCid", lfType.optional(lfType.contractId)],
]);

/** Wormhole.Ntt.Manager.AdminTransferProposal (ntt 0.7.0) */
export const ADMIN_TRANSFER_PROPOSAL_LAYOUT: LfType = lfType.record(
  "AdminTransferProposal",
  [
    ["admin", lfType.party],
    ["newAdmin", lfType.party],
    ["managerAddress", lfType.bytes("NttManagerAddress")],
  ]
);

// ---------------------------------------------------------------------------------------------
// Payload decoders. Each accepts the JSON Ledger API form, whether a source served it or
// createArgumentToJson read it from a blob.
// ---------------------------------------------------------------------------------------------

function describe(value: unknown): string {
  const text = JSON.stringify(value);
  return text === undefined ? String(value) : text.slice(0, 80);
}

function objectOf(json: unknown, what: string): Record<string, unknown> {
  if (typeof json !== "object" || json === null || Array.isArray(json))
    throw new Error(`${what}: expected an object, got ${describe(json)}`);
  return json as Record<string, unknown>;
}

function intInRange(
  json: unknown,
  what: string,
  min: bigint,
  max: bigint
): bigint {
  let value: bigint;
  try {
    value = decodeInt(json);
  } catch (err) {
    throw new Error(`${what}: ${(err as Error).message}`);
  }
  if (value < min || value > max)
    throw new Error(`${what}: expected ${min} to ${max}, got ${value}`);
  return value;
}

function hex32(json: unknown, type: string, what: string): string {
  let hex: string;
  try {
    hex = decodeBytesHex(json, type);
  } catch (err) {
    throw new Error(`${what}: ${(err as Error).message}`);
  }
  if (hex.length !== 64)
    throw new Error(`${what}: expected 32 bytes, got ${hex.length / 2}`);
  return hex;
}

export type NttTokenConfig = "LockUnlock" | "BurnMint";

/** A peer deployment as the manager stores it */
export interface NttPeerView {
  /** 32-byte manager address, lowercase hex */
  managerAddress: string;
  /** 32-byte transceiver (emitter) address, lowercase hex */
  transceiverAddress: string;
  decimals: number;
}

/** The NttManager fields this client reads */
export interface NttManagerView {
  namespace: string;
  admin: string;
  guardianGovernance: string;
  nttCustody: string;
  reserveCustodian: string;
  chainId: number;
  managerId: bigint;
  /** Lowercase hex */
  managerAddress: string;
  transceiverEmitterId: bigint;
  instrumentId: { admin: string; id: string };
  tokenConfig: NttTokenConfig;
  tokenDecimals: number;
  /** Lowercase hex */
  tokenAddress: string;
  /** By Wormhole chain id */
  peers: Map<number, NttPeerView>;
  factory: {
    tag: "TransferFactoryCid" | "BurnMintFactoryCid";
    contractId: string;
  };
  paused: boolean;
}

/**
 * The entries of a Daml Map. The JSON Ledger API writes a Map as an array of [key, value] pairs;
 * an object keyed by the encoded key is accepted too (the flow service's own decoder accepts both).
 */
function mapEntries(json: unknown, what: string): Array<[unknown, unknown]> {
  if (Array.isArray(json))
    return json.map((pair, index) => {
      if (!Array.isArray(pair) || pair.length !== 2)
        throw new Error(
          `${what}[${index}]: expected a [key, value] pair, got ${describe(pair)}`
        );
      return [pair[0], pair[1]];
    });
  return Object.entries(objectOf(json, what));
}

function decodePeers(json: unknown): Map<number, NttPeerView> {
  const peers = new Map<number, NttPeerView>();
  for (const [key, value] of mapEntries(json, "NttManager.peers")) {
    const chainId = Number(
      intInRange(
        typeof key === "number" ? String(key) : key,
        "NttManager.peers key",
        1n,
        65535n
      )
    );
    if (peers.has(chainId))
      throw new Error(`NttManager.peers: chain ${chainId} appears twice`);
    const what = `NttManager.peers[${chainId}]`;
    const peer = objectOf(value, what);
    peers.set(chainId, {
      managerAddress: hex32(
        peer["managerAddress"],
        "NttManagerAddress",
        `${what}.managerAddress`
      ),
      transceiverAddress: hex32(
        peer["transceiverAddress"],
        "EmitterAddress",
        `${what}.transceiverAddress`
      ),
      decimals: Number(
        intInRange(peer["decimals"], `${what}.decimals`, 1n, 255n)
      ),
    });
  }
  return peers;
}

/** Decodes an NttManager create argument (JSON Ledger API form) */
export function decodeNttManager(json: unknown): NttManagerView {
  const manager = objectOf(json, "NttManager");
  const tokenConfig = manager["tokenConfig"];
  if (tokenConfig !== "LockUnlock" && tokenConfig !== "BurnMint")
    throw new Error(
      `NttManager.tokenConfig: unknown value ${describe(tokenConfig)}`
    );
  const instrument = objectOf(
    manager["instrumentId"],
    "NttManager.instrumentId"
  );
  const factory = objectOf(manager["factory"], "NttManager.factory");
  const factoryTag = factory["tag"];
  if (
    factoryTag !== "TransferFactoryCid" &&
    factoryTag !== "BurnMintFactoryCid"
  )
    throw new Error(
      `NttManager.factory: unknown constructor ${describe(factoryTag)}`
    );
  if (typeof manager["paused"] !== "boolean")
    throw new Error("NttManager.paused is not a Bool");
  return {
    namespace: decodeText(manager["namespace"]),
    admin: decodeParty(manager["admin"]),
    guardianGovernance: decodeParty(manager["guardianGovernance"]),
    nttCustody: decodeParty(manager["nttCustody"]),
    reserveCustodian: decodeParty(manager["reserveCustodian"]),
    chainId: Number(
      intInRange(manager["chainId"], "NttManager.chainId", 1n, 65535n)
    ),
    managerId: intInRange(
      manager["managerId"],
      "NttManager.managerId",
      0n,
      2n ** 63n - 1n
    ),
    managerAddress: hex32(
      manager["managerAddress"],
      "NttManagerAddress",
      "NttManager.managerAddress"
    ),
    transceiverEmitterId: intInRange(
      manager["transceiverEmitterId"],
      "NttManager.transceiverEmitterId",
      0n,
      2n ** 53n - 1n
    ),
    instrumentId: {
      admin: decodeParty(instrument["admin"]),
      id: decodeText(instrument["id"]),
    },
    tokenConfig,
    tokenDecimals: Number(
      intInRange(manager["tokenDecimals"], "NttManager.tokenDecimals", 0n, 10n)
    ),
    tokenAddress: hex32(
      manager["tokenAddress"],
      "UniversalAddress",
      "NttManager.tokenAddress"
    ),
    peers: decodePeers(manager["peers"]),
    factory: {
      tag: factoryTag,
      contractId: decodeContractId(factory["value"]),
    },
    paused: manager["paused"],
  };
}

/** The LockedLedger fields this client reads */
export interface LockedLedgerView {
  guardianGovernance: string;
  /** Lowercase hex */
  managerAddress: string;
  /** The locked balance in units of 10^-10 (a Daml Decimal) */
  balance: bigint;
  /** The consolidated custody pot, when the ledger names one */
  custodyHoldingCid?: string;
}

/** Decodes a LockedLedger create argument (JSON Ledger API form) */
export function decodeLockedLedger(json: unknown): LockedLedgerView {
  const ledger = objectOf(json, "LockedLedger");
  const balance = decodeDecimal(ledger["balance"]);
  if (balance.startsWith("-"))
    throw new Error(`LockedLedger.balance is negative: ${balance}`);
  const custodyHoldingCid = ledger["custodyHoldingCid"];
  return {
    guardianGovernance: decodeParty(ledger["guardianGovernance"]),
    managerAddress: hex32(
      ledger["managerAddress"],
      "NttManagerAddress",
      "LockedLedger.managerAddress"
    ),
    balance: holdingAmountRaw(balance, DAML_DECIMAL_SCALE),
    ...(custodyHoldingCid !== undefined && custodyHoldingCid !== null
      ? { custodyHoldingCid: decodeContractId(custodyHoldingCid) }
      : {}),
  };
}

/** The AdminTransferProposal fields */
export interface AdminTransferProposalView {
  /** The proposing (then current) admin; the proposal's only signatory */
  admin: string;
  newAdmin: string;
  /** Lowercase hex */
  managerAddress: string;
}

/** Decodes an AdminTransferProposal create argument (JSON Ledger API form) */
export function decodeAdminTransferProposal(
  json: unknown
): AdminTransferProposalView {
  const proposal = objectOf(json, "AdminTransferProposal");
  return {
    admin: decodeParty(proposal["admin"]),
    newAdmin: decodeParty(proposal["newAdmin"]),
    managerAddress: hex32(
      proposal["managerAddress"],
      "NttManagerAddress",
      "AdminTransferProposal.managerAddress"
    ),
  };
}

// ---------------------------------------------------------------------------------------------
// The core's replay trie (Wormhole.Core.Replay:ReplayNode, wormhole-core 0.12.3). NTT consumes
// every inbound VAA digest into the trie of (consumer gg, namespace NttManager.namespace). A node
// holds the hex nibble prefix it covers and the set of consumed digest suffixes below it: a digest
// is consumed when the covering node's prefix starts it and its remaining nibbles are in the set.
// ---------------------------------------------------------------------------------------------

/** The ReplayNode fields */
export interface ReplayNodeView {
  consumer: string;
  guardianGovernance: string;
  namespace: string;
  /** Lowercase hex nibbles, 0 to 63 of them ("" at the root) */
  prefix: string;
  splitThreshold: bigint;
  /** Consumed digest suffixes, lowercase hex, each 64 - prefix.length nibbles */
  consumed: ReadonlySet<string>;
}

const NIBBLES = /^[0-9a-f]*$/;

function replayLayoutError(
  path: string,
  expected: string,
  value: LfValue
): never {
  throw new Error(
    `ReplayNode layout: ${path} is ${value.kind}, expected ${expected}`
  );
}

/** A Wormhole.Core.Bytes nibble type: a single-constructor variant of Text */
function nibbleText(value: LfValue, typeName: string, path: string): string {
  if (value.kind !== "variant")
    replayLayoutError(path, `a ${typeName} variant`, value);
  if (value.constructor !== typeName)
    throw new Error(
      `ReplayNode layout: ${path} has constructor ${value.constructor.slice(0, 40)}, expected ${typeName}`
    );
  if (value.value.kind !== "text")
    replayLayoutError(`${path}.value`, "a Text", value.value);
  if (!NIBBLES.test(value.value.value))
    throw new Error(`ReplayNode layout: ${path} is not lowercase hex nibbles`);
  return value.value.value;
}

/**
 * Decodes the create argument of a ReplayNode read from its created event blob (blob.ts). The
 * layout is Replay.daml's, field by field:
 *
 *   consumer : Party, guardianGovernance : Party, namespace : Text,
 *   prefix : DigestNibblePrefix, splitThreshold : Int, consumed : Set DigestNibbleSuffix
 *
 * A Daml Set is the record `Set { map : Map k () }` (DA.Set.Types); the JSON Ledger API writes it
 * as {"map": [[{"tag": "DigestNibbleSuffix", "value": "<hex>"}, {}], ...]}. Every difference from
 * this layout throws: a missing or extra field, another value kind, a constructor of another
 * name, a set value that is not unit, a repeated suffix, or a suffix of the wrong width. The
 * reader never guesses, so a changed core package fails closed.
 */
export function decodeReplayNode(createArgument: Uint8Array): ReplayNodeView {
  let value: LfValue;
  try {
    value = readValue(createArgument);
  } catch (err) {
    throw new Error(`ReplayNode layout: ${(err as Error).message}`);
  }
  if (value.kind !== "record")
    replayLayoutError("ReplayNode", "a record", value);
  if (value.fields.length !== 6)
    throw new Error(
      `ReplayNode layout: ${value.fields.length} fields, expected 6`
    );
  const [consumer, gg, namespace, prefix, splitThreshold, consumed] =
    value.fields as [LfValue, LfValue, LfValue, LfValue, LfValue, LfValue];
  if (consumer.kind !== "party")
    replayLayoutError("consumer", "a Party", consumer);
  if (gg.kind !== "party")
    replayLayoutError("guardianGovernance", "a Party", gg);
  if (namespace.kind !== "text")
    replayLayoutError("namespace", "a Text", namespace);
  if (splitThreshold.kind !== "int64")
    replayLayoutError("splitThreshold", "an Int", splitThreshold);
  const prefixHex = nibbleText(prefix, "DigestNibblePrefix", "prefix");
  if (prefixHex.length > 63)
    throw new Error(
      `ReplayNode layout: the prefix has ${prefixHex.length} nibbles, at most 63`
    );
  if (consumed.kind !== "record")
    replayLayoutError("consumed", "a Set record", consumed);
  if (consumed.fields.length !== 1)
    throw new Error(
      `ReplayNode layout: the consumed Set has ${consumed.fields.length} fields, expected 1 (map)`
    );
  const map = consumed.fields[0]!;
  if (map.kind !== "map") replayLayoutError("consumed.map", "a Map", map);
  const suffixes = new Set<string>();
  map.entries.forEach(([key, unit], index) => {
    const path = `consumed.map[${index}]`;
    const suffix = nibbleText(key, "DigestNibbleSuffix", `${path}.key`);
    if (unit.kind !== "unit") replayLayoutError(`${path}.value`, "unit", unit);
    if (suffix.length !== 64 - prefixHex.length)
      throw new Error(
        `ReplayNode layout: ${path} has ${suffix.length} nibbles under a ${prefixHex.length}-nibble prefix`
      );
    if (suffixes.has(suffix))
      throw new Error(`ReplayNode layout: ${path} repeats a suffix`);
    suffixes.add(suffix);
  });
  return {
    consumer: consumer.value,
    guardianGovernance: gg.value,
    namespace: namespace.value,
    prefix: prefixHex,
    splitThreshold: splitThreshold.value,
    consumed: suffixes,
  };
}

/**
 * Whether a node records a digest (checkReplayNodeRecordsDigest in Replay.daml). The node must
 * cover the digest: its prefix starts the digest.
 */
export function replayNodeRecordsDigest(
  node: ReplayNodeView,
  digest: string
): boolean {
  if (!/^[0-9a-f]{64}$/.test(digest))
    throw new Error(
      `A VAA digest is 64 lowercase hex nibbles, got "${digest}"`
    );
  if (!digest.startsWith(node.prefix))
    throw new Error(
      `The ReplayNode with prefix "${node.prefix}" does not cover digest ${digest}`
    );
  return node.consumed.has(digest.slice(node.prefix.length));
}

/**
 * Decodes the exercise result of NttManager.Transfer: the WormholeMessage its nested
 * PublishMessage returned.
 */
export function decodeTransferResult(json: unknown): WormholeMessageView {
  return decodePublishMessageResult({ _1: null, _2: json as DamlJson });
}

// ---------------------------------------------------------------------------------------------
// Choice arguments. Fees above zero are not supported yet, so the fee allocation is None.
// ---------------------------------------------------------------------------------------------

/** Splice.Api.Token.MetadataV1.ExtraArgs, with an optional receiver TransferPreapproval context */
export function extraArgs(transferPreapprovalCid?: string): DamlJson {
  const values: { [key: string]: DamlJson } = {};
  if (transferPreapprovalCid !== undefined)
    values[TRANSFER_PREAPPROVAL_CONTEXT_KEY] = {
      tag: "AV_ContractId",
      value: encodeContractId(transferPreapprovalCid),
    };
  return { context: { values }, meta: { values: {} } };
}

export interface TransferArgumentInput {
  user: string;
  transceiverEmitterCid: string;
  coreStateCid: string;
  /** The deployment's LockedLedger; locking mode only */
  ledgerCid?: string;
  recipientChain: number;
  /** 32 bytes, lowercase hex */
  recipientAddress: string;
  rawAmount: bigint;
  nonce: number;
  consistencyLevel: number;
  inputHoldingCids: readonly string[];
  /** The receiver's TransferPreapproval for a token-cip0056 lock */
  transferPreapprovalCid?: string;
  /** The configured manager address (anchor layer 4), lowercase hex */
  expectedManagerAddress: string;
}

/** NttManager.Transfer at fee 0 */
export function transferArgument(input: TransferArgumentInput): DamlJson {
  if (
    !Number.isInteger(input.nonce) ||
    input.nonce < 0 ||
    input.nonce > 0xffffffff
  )
    throw new Error(`nonce must be a uint32, got ${input.nonce}`);
  if (input.rawAmount <= 0n)
    throw new Error(`rawAmount must be positive, got ${input.rawAmount}`);
  return {
    user: encodeParty(input.user),
    transceiverEmitterCid: encodeContractId(input.transceiverEmitterCid),
    coreStateCid: encodeContractId(input.coreStateCid),
    ledgerCid:
      input.ledgerCid === undefined ? null : encodeContractId(input.ledgerCid),
    recipientChain: encodeInt(input.recipientChain),
    recipientAddress: encodeBytes(
      "UniversalAddress",
      input.recipientAddress
    ) as unknown as DamlJson,
    rawAmount: encodeInt(input.rawAmount),
    nonce: encodeInt(input.nonce),
    consistencyLevel: encodeInt(input.consistencyLevel),
    inputHoldingCids: input.inputHoldingCids.map((cid) =>
      encodeContractId(cid)
    ),
    feeAllocation: null,
    extraArgs: extraArgs(input.transferPreapprovalCid),
    expectedManagerAddress: encodeBytes(
      "NttManagerAddress",
      input.expectedManagerAddress
    ) as unknown as DamlJson,
  };
}

export interface InboundArgumentInput {
  executor: string;
  coreStateCid: string;
  replayNodeCid: string;
  vaaBytes: Uint8Array;
  recipient: string;
}

function inboundArgument(input: InboundArgumentInput): {
  [key: string]: DamlJson;
} {
  return {
    executor: encodeParty(input.executor),
    coreStateCid: encodeContractId(input.coreStateCid),
    replayNodeCid: encodeContractId(input.replayNodeCid),
    vaaBytes: encodeBytes("EncodedVaa", input.vaaBytes) as unknown as DamlJson,
    recipient: encodeParty(input.recipient),
  };
}

/** NttManager.Release (locking mode): spends the custody pot the ledger names */
export function releaseArgument(
  input: InboundArgumentInput & {
    ledgerCid: string;
    transferPreapprovalCid?: string;
  }
): DamlJson {
  return {
    ...inboundArgument(input),
    ledgerCid: encodeContractId(input.ledgerCid),
    inputHoldingCids: [],
    extraArgs: extraArgs(input.transferPreapprovalCid),
  };
}

/** NttManager.Mint (burning mode): mints through the recipient's DepositPreapproval */
export function mintArgument(
  input: InboundArgumentInput & { depositPreapprovalCid: string }
): DamlJson {
  return {
    ...inboundArgument(input),
    depositPreapprovalCid: encodeContractId(input.depositPreapprovalCid),
    extraArgs: extraArgs(),
  };
}

/** NttManager.SetPaused */
export function setPausedArgument(newPaused: boolean): DamlJson {
  return { newPaused };
}

export interface SetPeerArgumentInput {
  /** The peer's Wormhole chain id */
  chainId: number;
  peer: NttPeerView;
  transceiverEmitterCid: string;
  coreStateCid: string;
  consistencyLevel: number;
  /** Funds the registration message; the admin on the direct path */
  payer: string;
}

/** NttManager.SetPeer at fee 0 */
export function setPeerArgument(input: SetPeerArgumentInput): DamlJson {
  if (
    !Number.isInteger(input.chainId) ||
    input.chainId < 1 ||
    input.chainId > 65535
  )
    throw new Error(`chainId must be a positive uint16, got ${input.chainId}`);
  if (
    !Number.isInteger(input.peer.decimals) ||
    input.peer.decimals < 1 ||
    input.peer.decimals > 255
  )
    throw new Error(
      `peer decimals must be a positive uint8, got ${input.peer.decimals}`
    );
  return {
    chainId: encodeInt(input.chainId),
    peer: {
      managerAddress: encodeBytes(
        "NttManagerAddress",
        input.peer.managerAddress
      ) as unknown as DamlJson,
      transceiverAddress: encodeBytes(
        "EmitterAddress",
        input.peer.transceiverAddress
      ) as unknown as DamlJson,
      decimals: encodeInt(input.peer.decimals),
    },
    transceiverEmitterCid: encodeContractId(input.transceiverEmitterCid),
    coreStateCid: encodeContractId(input.coreStateCid),
    consistencyLevel: encodeInt(input.consistencyLevel),
    payer: encodeParty(input.payer),
    feeAllocation: null,
  };
}

/** The create argument of an AdminTransferProposal */
export function adminTransferProposalArgument(
  input: AdminTransferProposalView
): DamlJson {
  return {
    admin: encodeParty(input.admin),
    newAdmin: encodeParty(input.newAdmin),
    managerAddress: encodeBytes(
      "NttManagerAddress",
      input.managerAddress
    ) as unknown as DamlJson,
  };
}

export interface AcceptAdminTransferArgumentInput {
  mgrCid: string;
  /** The deployment's LockedLedger; lock/unlock only */
  ledgerCid?: string;
  /** The committed factory the new admin vetted; it must equal the manager's */
  vettedFactory: NttManagerView["factory"];
}

/** AdminTransferProposal.AcceptAdminTransfer, with no balance to move (empty extraArgs) */
export function acceptAdminTransferArgument(
  input: AcceptAdminTransferArgumentInput
): DamlJson {
  return {
    mgrCid: encodeContractId(input.mgrCid),
    ledgerCid:
      input.ledgerCid === undefined ? null : encodeContractId(input.ledgerCid),
    vettedFactory: {
      tag: input.vettedFactory.tag,
      value: encodeContractId(input.vettedFactory.contractId),
    },
    extraArgs: extraArgs(),
  };
}
