// parseTransferVaa / validateAgainstDeployment, offline against the real
// guardian-signed fixtures. Gated like vaa.test.ts: pre-fork, toChain(75)
// throws and the whole suite is skipped rather than failing the run.
import { describe, expect, test } from "bun:test";
import {
  CANTON_VAA,
  ETH_VAA,
  CANTON_TRANSFER_VAA,
  ETH_TRANSFER_VAA,
} from "./fixtures/vaas.js";
import { loadConfig } from "../config.js";
import { recipientBinding } from "../binding.js";
import { parseTransferVaa, validateAgainstDeployment, type ParsedTransferVaa } from "../vaa.js";

const hasCanton = (() => {
  try {
    return require("@wormhole-foundation/sdk").toChain(75) === "Canton";
  } catch {
    return false;
  }
})();

describe.skipIf(!hasCanton)("parseTransferVaa", () => {
  const config = loadConfig();

  test("parses a Canton -> Ethereum transfer VAA (canburn, sequence 3)", () => {
    const parsed = parseTransferVaa(CANTON_TRANSFER_VAA);
    const dep = config.deployments.canburn.chains;

    expect(parsed.sourceChain).toBe("Canton");
    expect(parsed.destinationChain).toBe("Ethereum");
    expect(parsed.sequence).toBe(3n);
    expect(parsed.sourceManager).toBe(dep.Canton.manager);
    expect(parsed.recipientManager).toBe(
      "000000000000000000000000" + dep.Ethereum.manager.slice(2).toLowerCase()
    );
    expect(parsed.trimmedAmount).toEqual({ amount: 100000000n, decimals: 8 });
    // Keeps the original bytes verbatim for on-chain submission.
    expect(parsed.bytes.length * 2).toBe(CANTON_TRANSFER_VAA.length);
  });

  test("parses an Ethereum -> Canton transfer VAA and matches recipientBinding(alice)", () => {
    const parsed = parseTransferVaa(ETH_TRANSFER_VAA);
    const dep = config.deployments.canburn.chains;
    const alice = `${config.canton.parties.user}::${config.canton.namespace}`;

    expect(parsed.sourceChain).toBe("Ethereum");
    expect(parsed.destinationChain).toBe("Canton");
    expect(parsed.recipientManager).toBe(dep.Canton.manager);
    expect(parsed.recipientAddress).toBe(
      recipientBinding(alice).replace(/^0x/, "").toLowerCase()
    );
    expect(parsed.trimmedAmount.amount).toBe(100000000n);
  });

  test("rejects a TransceiverRegistration VAA (Canton-emitted) with a clear error", () => {
    expect(() => parseTransferVaa(CANTON_VAA)).toThrow(
      /Ntt:TransceiverRegistration governance message, not a transfer/
    );
  });

  test("rejects a TransceiverRegistration VAA (Ethereum-emitted) with a clear error", () => {
    expect(() => parseTransferVaa(ETH_VAA)).toThrow(
      /Ntt:TransceiverRegistration governance message, not a transfer/
    );
  });

  test("accepts a 0x-prefixed hex string identically to bare hex", () => {
    const withPrefix = parseTransferVaa("0x" + CANTON_TRANSFER_VAA);
    const bare = parseTransferVaa(CANTON_TRANSFER_VAA);
    expect(withPrefix.sequence).toBe(bare.sequence);
    expect(withPrefix.sourceManager).toBe(bare.sourceManager);
  });
});

describe.skipIf(!hasCanton)("validateAgainstDeployment", () => {
  const config = loadConfig();

  test("passes for the canburn deployment (managers match)", () => {
    const parsed = parseTransferVaa(CANTON_TRANSFER_VAA);
    const result = validateAgainstDeployment(parsed, config, "canburn");
    expect(result).toEqual({ sourceChain: "Canton", destinationChain: "Ethereum" });
  });

  test("fails for canhub (different managers than canburn)", () => {
    const parsed = parseTransferVaa(CANTON_TRANSFER_VAA);
    expect(() => validateAgainstDeployment(parsed, config, "canhub")).toThrow(
      /does not match deployment "canhub"/
    );
  });

  test("refuses a same-chain VAA", () => {
    const parsed = parseTransferVaa(CANTON_TRANSFER_VAA);
    const sameChain: ParsedTransferVaa = { ...parsed, destinationChain: "Canton" };
    expect(() => validateAgainstDeployment(sameChain, config, "canburn")).toThrow(
      /source and destination chain are both "Canton"/
    );
  });
});
