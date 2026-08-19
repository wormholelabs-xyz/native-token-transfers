// Gated on the Canton SDK fork (Part A): pre-fork, toChain(75) throws and this
// whole suite is skipped rather than failing the run. Post-fork it exercises
// real deserialization of chain-75 VAAs, the canary for "exactly one sdk-base
// instance is resolved."
//
// SDK 6.x payload registration is not a module side effect anymore — register()
// explicitly before any deserialize() call.
import { describe, expect, test } from "bun:test";
import { register } from "@wormhole-foundation/sdk-definitions-ntt";
import {
  CANTON_VAA,
  ETH_VAA,
  CANTON_TRANSFER_VAA,
  ETH_TRANSFER_VAA,
} from "./fixtures/vaas.js";
import { loadConfig } from "../config.js";
import { recipientBinding } from "../binding.js";

register();

const hasCanton = (() => {
  try {
    return require("@wormhole-foundation/sdk").toChain(75) === "Canton";
  } catch {
    return false;
  }
})();

describe.skipIf(!hasCanton)(
  "VAA deserialization (requires the Canton SDK fork)",
  () => {
    // Deviation from the original plan: both fixture VAAs turned out, on
    // independent decode, to be TransceiverRegistration governance messages
    // (sequence 1 — each transceiver's own peer-registration, not a user
    // transfer), not "Ntt:WormholeTransfer" payloads. Asserting a
    // WormholeTransfer shape against this data would be fabricated, so this
    // suite verifies the payload type that is actually present. Cross-checks
    // against the committed registry's `emitter` fields, which is exactly
    // what a TransceiverRegistration message carries (chain + transceiver).

    test("Canton-emitted VAA registers the canburn Ethereum transceiver", async () => {
      const { deserialize } = await import("@wormhole-foundation/sdk");
      const config = loadConfig();

      const vaa = deserialize("Ntt:TransceiverRegistration", CANTON_VAA);
      // String(): emitterChain is typed as a template-literal union; coerce for comparison
      expect(String(vaa.emitterChain)).toBe("Canton");
      expect(vaa.sequence).toBe(1n);
      expect(vaa.emitterAddress.toString().replace(/^0x/, "")).toBe(
        config.deployments.canburn.chains.Canton.emitter
      );
      expect(vaa.payload.chain).toBe("Ethereum");
      expect(vaa.payload.transceiver.toString().replace(/^0x/, "")).toBe(
        config.deployments.canburn.chains.Ethereum.emitter
      );
    });

    test("Ethereum-emitted VAA registers the canburn Canton transceiver", async () => {
      const { deserialize } = await import("@wormhole-foundation/sdk");
      const config = loadConfig();

      const vaa = deserialize("Ntt:TransceiverRegistration", ETH_VAA);
      expect(vaa.emitterChain).toBe("Ethereum");
      expect(vaa.sequence).toBe(1n);
      expect(vaa.emitterAddress.toString().replace(/^0x/, "")).toBe(
        config.deployments.canburn.chains.Ethereum.emitter
      );
      expect(String(vaa.payload.chain)).toBe("Canton");
      expect(vaa.payload.transceiver.toString().replace(/^0x/, "")).toBe(
        config.deployments.canburn.chains.Canton.emitter
      );
    });

    test("Canton -> Ethereum transfer VAA carries the canburn managers", async () => {
      const { deserialize } = await import("@wormhole-foundation/sdk");
      const config = loadConfig();
      const dep = config.deployments.canburn.chains;

      const vaa = deserialize("Ntt:WormholeTransfer", CANTON_TRANSFER_VAA);
      expect(String(vaa.emitterChain)).toBe("Canton");
      expect(vaa.sequence).toBe(3n);
      expect(vaa.payload.sourceNttManager.toString().replace(/^0x/, "")).toBe(
        dep.Canton.manager
      );
      expect(vaa.payload.recipientNttManager.toString()).toBe(
        "0x000000000000000000000000" + dep.Ethereum.manager.slice(2).toLowerCase()
      );
      const inner = vaa.payload.nttManagerPayload.payload;
      expect(String(inner.recipientChain)).toBe("Ethereum");
      expect(inner.trimmedAmount.decimals).toBe(8);
      expect(inner.trimmedAmount.amount).toBe(100000000n);
    });

    test("Ethereum -> Canton transfer VAA targets recipientBinding(alice)", async () => {
      const { deserialize } = await import("@wormhole-foundation/sdk");
      const config = loadConfig();
      const dep = config.deployments.canburn.chains;

      const vaa = deserialize("Ntt:WormholeTransfer", ETH_TRANSFER_VAA);
      expect(String(vaa.emitterChain)).toBe("Ethereum");
      expect(vaa.payload.recipientNttManager.toString().replace(/^0x/, "")).toBe(
        dep.Canton.manager
      );
      const inner = vaa.payload.nttManagerPayload.payload;
      expect(String(inner.recipientChain)).toBe("Canton");
      const alice = `${config.canton.parties.user}::${config.canton.namespace}`;
      expect(inner.recipientAddress.toString()).toBe(recipientBinding(alice));
      expect(inner.trimmedAmount.amount).toBe(100000000n);
    });
  }
);
