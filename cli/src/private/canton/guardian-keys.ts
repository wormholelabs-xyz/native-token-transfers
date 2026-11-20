// One (guardian index, public key) pair per VAA signature, for the Daml pubKeys argument.
// Core VAA.daml verifyOne binds each key to the guardian address with keccak256(key)[12..].
import { Signature, SigningKey } from "ethers";

import { bytesToHex } from "../binding.js";
import type { ParsedTransferVaa } from "../vaa.js";

const ETHEREUM_RECOVERY_ID_OFFSET = 27;
const UNCOMPRESSED_PUBLIC_KEY_HEX_LENGTH = 130;

export interface GuardianPublicKey {
  guardianIndex: number;
  uncompressedPublicKeyHex: string;
}

export function recoverGuardianPublicKeys(
  parsedVaa: ParsedTransferVaa
): GuardianPublicKey[] {
  if (parsedVaa.signatures.length === 0)
    throw new Error("VAA has no signatures");
  return parsedVaa.signatures.map((guardianSignature) => {
    const signature = Signature.from({
      r: `0x${bytesToHex(guardianSignature.r)}`,
      s: `0x${bytesToHex(guardianSignature.s)}`,
      v: ETHEREUM_RECOVERY_ID_OFFSET + guardianSignature.recoveryId,
    });
    const uncompressedPublicKeyHex = SigningKey.recoverPublicKey(
      `0x${parsedVaa.digest}`,
      signature
    ).slice(2);
    if (
      uncompressedPublicKeyHex.length !== UNCOMPRESSED_PUBLIC_KEY_HEX_LENGTH ||
      !uncompressedPublicKeyHex.startsWith("04")
    ) {
      throw new Error(
        "recovered guardian key is not a 65-byte uncompressed key"
      );
    }
    return {
      guardianIndex: guardianSignature.guardianIndex,
      uncompressedPublicKeyHex,
    };
  });
}
