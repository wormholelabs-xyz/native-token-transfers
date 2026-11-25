import { describe, expect, test } from "bun:test";
import { keccak256 } from "ethers";

import { partyAddressFor } from "../recipient.js";
import { requireDeployment, validateRegistry } from "../config.js";
import { checkTransferVaaMatchesDeployment } from "../transfer-route.js";
import { parseCantonPartyId } from "../types.js";
import { parseTransferVaa, vaaDigestHex32 } from "../vaa.js";
import {
  CANTON_TRANSFER_VAA,
  CANTON_VAA,
  ETH_TRANSFER_VAA,
  ETH_VAA,
} from "./fixtures/vaas.js";
import {
  RETIRED_CANTON_NAMESPACE,
  retiredCanburnRegistryJson,
} from "./fixtures/registry.js";

const registry = validateRegistry(retiredCanburnRegistryJson());
const canburn = requireDeployment(registry, "canburn");

function flipHexCharacter(hexText: string, characterIndex: number): string {
  const flipped = hexText[characterIndex] === "0" ? "1" : "0";
  return (
    hexText.slice(0, characterIndex) +
    flipped +
    hexText.slice(characterIndex + 1)
  );
}

describe("parseTransferVaa", () => {
  test("Canton -> Ethereum transfer (sequence 3)", () => {
    const parsed = parseTransferVaa(CANTON_TRANSFER_VAA);
    expect(parsed.emitterChainId as number).toBe(75);
    expect(parsed.recipientChainId as number).toBe(2);
    expect(parsed.sequence).toBe(3n);
    expect(parsed.signatures.length).toBe(1);
    expect(parsed.trimmedAmount).toEqual({ amount: 100000000n, decimals: 8 });
    expect(parsed.bytes.length * 2).toBe(CANTON_TRANSFER_VAA.length);
  });

  test("Ethereum -> Canton transfer targets alice by the ntt 0.6.x party address", () => {
    // The fixture predates ntt 0.7.0: its recipient is alice's address under
    // the old tag "wormhole:ntt-recipient:v1", so it does not match
    // partyAddressFor(alice) and does not redeem under 0.7.0.
    const parsed = parseTransferVaa(ETH_TRANSFER_VAA);
    const alice = parseCantonPartyId(`alice::${RETIRED_CANTON_NAMESPACE}`);
    expect(parsed.recipientAddress as string).toBe(
      "b57bfe41337237d696834be81e491c78099971fc2bc1ad6eedd032583350a516"
    );
    expect(parsed.recipientAddress).not.toBe(partyAddressFor(alice));
  });

  test("digest is keccak256(keccak256(body))", () => {
    const vaaBytes = Buffer.from(ETH_TRANSFER_VAA, "hex");
    const body = vaaBytes.subarray(6 + 66);
    expect(vaaDigestHex32(vaaBytes) as string).toBe(
      keccak256(keccak256(body)).slice(2)
    );
  });

  test.each([
    ["TransceiverRegistration (Canton)", CANTON_VAA, /TransceiverRegistration/],
    ["TransceiverRegistration (Ethereum)", ETH_VAA, /TransceiverRegistration/],
    ["truncated", ETH_TRANSFER_VAA.slice(0, -10), /truncated|trailing/],
    ["version 2", "02" + ETH_TRANSFER_VAA.slice(2), /version/],
    ["trailing byte", ETH_TRANSFER_VAA + "00", /trailing/],
    ["odd hex", ETH_TRANSFER_VAA + "0", /hex/],
  ])("rejects %s", (_caseName, vaaHex, expectedError) => {
    expect(() => parseTransferVaa(vaaHex)).toThrow(expectedError);
  });
});

describe("checkTransferVaaMatchesDeployment", () => {
  test.each([[CANTON_TRANSFER_VAA], [ETH_TRANSFER_VAA]])(
    "accepts the matching deployment",
    (vaaHex) => {
      expect(() =>
        checkTransferVaaMatchesDeployment(
          parseTransferVaa(vaaHex),
          registry,
          canburn
        )
      ).not.toThrow();
    }
  );

  test.each([
    ["Canton emitter", "Canton", "emitter"],
    ["Ethereum emitter", "Ethereum", "emitter"],
  ])("rejects a changed %s byte", (_caseName, chain, field) => {
    const changed = retiredCanburnRegistryJson();
    changed.deployments.canburn.chains[chain][field] = flipHexCharacter(
      changed.deployments.canburn.chains[chain][field],
      63
    );
    const changedRegistry = validateRegistry(changed);
    const vaaHex = chain === "Canton" ? CANTON_TRANSFER_VAA : ETH_TRANSFER_VAA;
    expect(() =>
      checkTransferVaaMatchesDeployment(
        parseTransferVaa(vaaHex),
        changedRegistry,
        requireDeployment(changedRegistry, "canburn")
      )
    ).toThrow(/emitter/);
  });

  test("rejects a changed Canton manager", () => {
    const changed = retiredCanburnRegistryJson();
    changed.deployments.canburn.chains.Canton.manager = flipHexCharacter(
      changed.deployments.canburn.chains.Canton.manager,
      0
    );
    const changedRegistry = validateRegistry(changed);
    expect(() =>
      checkTransferVaaMatchesDeployment(
        parseTransferVaa(ETH_TRANSFER_VAA),
        changedRegistry,
        requireDeployment(changedRegistry, "canburn")
      )
    ).toThrow(/manager/);
  });
});
