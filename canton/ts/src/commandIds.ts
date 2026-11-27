// Command ids of NTT transactions. They are derived from the intent, not from contract ids, so a
// rebuilt or retried submission of the same transfer or redeem is deduplicated by the participant.
// The admin commands that exercise a choice take the base SDK's content-derived id (it includes
// the manager contract id); the proposal create has its own below.
// The formulas are those of the ntt CLI before it moved onto this SDK, so a transfer started by
// an older CLI and retried by this SDK is deduplicated too.
import { encoding } from "@wormhole-foundation/sdk-base";
import { keccak256 } from "@wormhole-foundation/sdk-definitions";
import { canonicalJson } from "@wormhole-foundation/sdk-canton";
import {
  PROPOSE_ADMIN_COMMAND_ID_PREFIX,
  REDEEM_COMMAND_ID_PREFIX,
  TRANSFER_COMMAND_ID_PREFIX,
} from "./constants.js";

const HEX_32 = /^[0-9a-f]{64}$/;

function word(value: bigint): Uint8Array {
  if (value < 0n || value >= 2n ** 256n)
    throw new Error(`${value} does not fit 256 bits`);
  return encoding.bignum.toBytes(value, 32);
}

function bytes32(hex: string, what: string): Uint8Array {
  if (!HEX_32.test(hex))
    throw new Error(`${what} must be 64 lowercase hex characters`);
  return encoding.hex.decode(hex);
}

export interface TransferCommandIdFields {
  user: string;
  /** Lowercase hex */
  manager: string;
  rawAmount: bigint;
  recipientChainId: number;
  /** Lowercase hex */
  recipient: string;
  nonce: number;
}

/**
 * "ntt-private-send-" ‖ keccak256(abi.encode(string user, bytes32 manager, uint256 rawAmount,
 * uint16 recipientChainId, bytes32 recipient, uint32 nonce)), as hex.
 */
export function transferCommandId(fields: TransferCommandIdFields): string {
  if (
    !Number.isInteger(fields.recipientChainId) ||
    fields.recipientChainId < 0 ||
    fields.recipientChainId > 0xffff
  )
    throw new Error(
      `recipientChainId must be a uint16, got ${fields.recipientChainId}`
    );
  if (
    !Number.isInteger(fields.nonce) ||
    fields.nonce < 0 ||
    fields.nonce > 0xffffffff
  )
    throw new Error(`nonce must be a uint32, got ${fields.nonce}`);
  const user = encoding.bytes.encode(fields.user);
  const paddedLength = Math.ceil(user.length / 32) * 32;
  const userTail = new Uint8Array(paddedLength);
  userTail.set(user);
  const encoded = encoding.bytes.concat(
    word(6n * 32n), // offset of the string, after the six head words
    bytes32(fields.manager, "manager"),
    word(fields.rawAmount),
    word(BigInt(fields.recipientChainId)),
    bytes32(fields.recipient, "recipient"),
    word(BigInt(fields.nonce)),
    word(BigInt(user.length)),
    userTail
  );
  return `${TRANSFER_COMMAND_ID_PREFIX}${encoding.hex.encode(keccak256(encoded))}`;
}

/** "ntt-private-redeem-" ‖ the VAA digest */
export function redeemCommandId(digest: string): string {
  return `${REDEEM_COMMAND_ID_PREFIX}${bytes32Hex(digest, "digest")}`;
}

function bytes32Hex(hex: string, what: string): string {
  bytes32(hex, what);
  return hex;
}

/** keccak256(keccak256(body)): the VAA digest the guardians sign and the core replay trie keys */
export function vaaDigest(vaaHash: Uint8Array): string {
  if (vaaHash.length !== 32)
    throw new Error(`A VAA hash has 32 bytes, got ${vaaHash.length}`);
  return encoding.hex.encode(keccak256(vaaHash));
}

/**
 * The command id of an AdminTransferProposal create: "ntt-admin-propose-" ‖ keccak256 of the
 * proposal and the manager contract it is made against. A create names no contract, so a command
 * id derived from its content alone would deduplicate a second, legitimate proposal of the same
 * pair (after the role went away and came back); the manager contract id changes with every admin
 * change, and a retry of one build keeps it.
 */
export function proposeAdminCommandId(fields: {
  admin: string;
  newAdmin: string;
  /** Lowercase hex */
  manager: string;
  managerContractId: string;
}): string {
  const content = canonicalJson({
    ...fields,
    manager: bytes32Hex(fields.manager, "manager"),
  });
  return `${PROPOSE_ADMIN_COMMAND_ID_PREFIX}${encoding.hex.encode(
    keccak256(encoding.bytes.encode(content))
  )}`;
}
