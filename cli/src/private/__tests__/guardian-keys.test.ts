import { describe, expect, test } from "bun:test";

import { recoverGuardianPublicKeys } from "../canton/guardian-keys.js";
import { parseTransferVaa } from "../vaa.js";
import { CANTON_TRANSFER_VAA, ETH_TRANSFER_VAA } from "./fixtures/vaas.js";

// The single guardian of the retired 2026-08 private network.
const RETIRED_GUARDIAN_PUBLIC_KEY =
  "042cb0fec84a83ca7b143d369451d73bc1c26080c93ee813142bf0f0351fa76ca711b20b0294c495ecbf04d9daf7c97cf84a5f1262716ca2c06bbf7e0b72059c85";

describe("recoverGuardianPublicKeys", () => {
  test.each([[ETH_TRANSFER_VAA], [CANTON_TRANSFER_VAA]])(
    "one (index, uncompressed key) per signature",
    (vaaHex) => {
      expect(recoverGuardianPublicKeys(parseTransferVaa(vaaHex))).toEqual([
        {
          guardianIndex: 0,
          uncompressedPublicKeyHex: RETIRED_GUARDIAN_PUBLIC_KEY,
        },
      ]);
    }
  );

  test("a signature with a high s rejects", () => {
    const parsed = parseTransferVaa(ETH_TRANSFER_VAA);
    parsed.signatures[0].s = Uint8Array.from(
      Buffer.from("ff".repeat(32), "hex")
    );
    expect(() => recoverGuardianPublicKeys(parsed)).toThrow();
  });
});
