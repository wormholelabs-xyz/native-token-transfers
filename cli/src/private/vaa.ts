// Strict parser for a guardian-signed VAA that carries an NTT transfer
// (WormholeTransceiver -> NttManagerMessage -> NativeTokenTransfer), per
// wormhole sdk/vaa/structs.go and evm/src/libraries/TransceiverStructs.sol.
import { keccak256 } from "ethers";

import { bytesToHex, hexToBytes } from "./hex.js";
import {
  hex32FromBytes,
  parseVaaDigestHex32,
  parseWormholeChainId,
  type Hex32,
  type VaaDigestHex32,
  type WormholeChainId,
} from "./types.js";

const VAA_VERSION = 1;
const VAA_HEADER_FIXED_BYTES = 6; // version u8, guardianSetIndex u32, signatureCount u8
const VAA_SIGNATURE_BYTES = 66; // guardianIndex u8, r 32, s 32, v u8
const VAA_BODY_FIXED_BYTES = 51; // timestamp u32, nonce u32, chain u16, emitter 32, sequence u64, consistency u8
// node/pkg/common MaxGuardianCount.
export const GUARDIAN_COUNT_MAX = 19;
export const VAA_BYTES_MAX = 64 * 1024;

const WORMHOLE_TRANSCEIVER_PREFIX = "9945ff10";
const NATIVE_TOKEN_TRANSFER_PREFIX = "994e5454";
const TRANSCEIVER_REGISTRATION_PREFIX = "18fc67c2";

export interface GuardianSignature {
  guardianIndex: number;
  r: Uint8Array;
  s: Uint8Array;
  recoveryId: number;
}

export interface ParsedTransferVaa {
  /** The guardian-signed bytes, submitted unchanged. */
  bytes: Uint8Array;
  guardianSetIndex: number;
  signatures: GuardianSignature[];
  digest: VaaDigestHex32;
  emitterChainId: WormholeChainId;
  emitterAddress: Hex32;
  sequence: bigint;
  sourceManager: Hex32;
  recipientManager: Hex32;
  recipientAddress: Hex32;
  recipientChainId: WormholeChainId;
  trimmedAmount: { amount: bigint; decimals: number };
}

class ByteReader {
  private offset = 0;
  constructor(
    private readonly bytes: Uint8Array,
    private readonly context: string
  ) {}

  take(length: number): Uint8Array {
    if (this.offset + length > this.bytes.length) {
      throw new Error(
        `${this.context}: truncated (needs ${length} bytes at offset ${this.offset})`
      );
    }
    const slice = this.bytes.slice(this.offset, this.offset + length);
    this.offset += length;
    return slice;
  }

  u8(): number {
    return this.take(1)[0];
  }

  u16(): number {
    const slice = this.take(2);
    return (slice[0] << 8) | slice[1];
  }

  u32(): number {
    return new DataView(this.take(4).buffer).getUint32(0, false);
  }

  u64(): bigint {
    return new DataView(this.take(8).buffer).getBigUint64(0, false);
  }

  remaining(): number {
    return this.bytes.length - this.offset;
  }

  checkFullyConsumed(): void {
    if (this.remaining() !== 0) {
      throw new Error(
        `${this.context}: ${this.remaining()} unexpected trailing bytes`
      );
    }
  }
}

/** keccak256(keccak256(body)): the digest the guardians sign (core VAA.daml digestOf). */
export function vaaDigestHex32(vaaBytes: Uint8Array): VaaDigestHex32 {
  const reader = new ByteReader(vaaBytes, "VAA");
  reader.take(5);
  const signatureCount = reader.u8();
  const bodyOffset =
    VAA_HEADER_FIXED_BYTES + signatureCount * VAA_SIGNATURE_BYTES;
  if (vaaBytes.length < bodyOffset + VAA_BODY_FIXED_BYTES)
    throw new Error("VAA: truncated body");
  return parseVaaDigestHex32(keccak256(keccak256(vaaBytes.slice(bodyOffset))));
}

function parseNttTransferPayload(payload: Uint8Array) {
  const payloadPrefix = bytesToHex(payload.slice(0, 4));
  if (payloadPrefix === TRANSCEIVER_REGISTRATION_PREFIX) {
    throw new Error(
      "this VAA is an NTT TransceiverRegistration governance message, not a transfer"
    );
  }
  if (payloadPrefix !== WORMHOLE_TRANSCEIVER_PREFIX) {
    throw new Error(
      `VAA payload prefix 0x${payloadPrefix} is not the WormholeTransceiver prefix`
    );
  }
  const transceiverMessage = new ByteReader(
    payload,
    "WormholeTransceiver message"
  );
  transceiverMessage.take(4);
  const sourceManager = hex32FromBytes(transceiverMessage.take(32));
  const recipientManager = hex32FromBytes(transceiverMessage.take(32));
  const managerPayload = transceiverMessage.take(transceiverMessage.u16());
  transceiverMessage.take(transceiverMessage.u16());
  transceiverMessage.checkFullyConsumed();

  const managerMessage = new ByteReader(managerPayload, "NttManagerMessage");
  managerMessage.take(32); // id
  managerMessage.take(32); // sender
  const transferPayload = managerMessage.take(managerMessage.u16());
  managerMessage.checkFullyConsumed();

  const transfer = new ByteReader(transferPayload, "NativeTokenTransfer");
  const transferPrefix = bytesToHex(transfer.take(4));
  if (transferPrefix !== NATIVE_TOKEN_TRANSFER_PREFIX) {
    throw new Error(
      `NttManagerMessage payload prefix 0x${transferPrefix} is not NativeTokenTransfer`
    );
  }
  const decimals = transfer.u8();
  const amount = transfer.u64();
  transfer.take(32); // source token
  const recipientAddress = hex32FromBytes(transfer.take(32));
  const recipientChainId = parseWormholeChainId(transfer.u16());
  if (transfer.remaining() > 0) transfer.take(transfer.u16()); // additional payload
  transfer.checkFullyConsumed();

  if (amount === 0n) throw new Error("NativeTokenTransfer amount is zero");
  return {
    sourceManager,
    recipientManager,
    recipientAddress,
    recipientChainId,
    trimmedAmount: { amount, decimals },
  };
}

export function parseTransferVaa(vaaHex: string): ParsedTransferVaa {
  const bytes = hexToBytes(vaaHex.trim());
  if (bytes.length > VAA_BYTES_MAX)
    throw new Error(`VAA is longer than ${VAA_BYTES_MAX} bytes`);
  const reader = new ByteReader(bytes, "VAA");
  const version = reader.u8();
  if (version !== VAA_VERSION)
    throw new Error(`VAA version must be ${VAA_VERSION}, got ${version}`);
  const guardianSetIndex = reader.u32();
  const signatureCount = reader.u8();
  if (signatureCount < 1) throw new Error("VAA has no signatures");
  if (signatureCount > GUARDIAN_COUNT_MAX) {
    throw new Error(
      `VAA has ${signatureCount} signatures; the maximum is ${GUARDIAN_COUNT_MAX}`
    );
  }
  const signatures: GuardianSignature[] = [];
  for (
    let signatureNumber = 0;
    signatureNumber < signatureCount;
    signatureNumber++
  ) {
    const guardianIndex = reader.u8();
    const r = reader.take(32);
    const s = reader.take(32);
    const recoveryId = reader.u8();
    const previous = signatures[signatures.length - 1];
    if (previous && guardianIndex <= previous.guardianIndex) {
      throw new Error("VAA guardian indices are not strictly increasing");
    }
    if (recoveryId > 1)
      throw new Error(
        `VAA signature recovery id must be 0 or 1, got ${recoveryId}`
      );
    signatures.push({ guardianIndex, r, s, recoveryId });
  }
  reader.u32(); // timestamp
  reader.u32(); // nonce
  const emitterChainId = parseWormholeChainId(reader.u16());
  const emitterAddress = hex32FromBytes(reader.take(32));
  const sequence = reader.u64();
  reader.u8(); // consistency level
  const payload = reader.take(reader.remaining());

  return {
    bytes,
    guardianSetIndex,
    signatures,
    digest: vaaDigestHex32(bytes),
    emitterChainId,
    emitterAddress,
    sequence,
    ...parseNttTransferPayload(payload),
  };
}
