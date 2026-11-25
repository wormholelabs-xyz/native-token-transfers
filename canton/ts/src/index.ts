// NTT protocol for Canton (ntt 0.7.0).
import {
  protocolIsRegistered,
  registerProtocol,
} from "@wormhole-foundation/sdk-definitions";
import { register as registerDefinitions } from "@wormhole-foundation/sdk-definitions-ntt";
import { _platform } from "@wormhole-foundation/sdk-canton";
import "@wormhole-foundation/sdk-canton-core";
import { CantonNtt } from "./ntt.js";

/** Explicitly register the Canton NTT protocol. Idempotent: safe to call more than once. */
export function register(): void {
  registerDefinitions();
  if (!protocolIsRegistered(_platform, "Ntt"))
    registerProtocol(_platform, "Ntt", CantonNtt);
}

export * from "./amounts.js";
export * from "./anchor.js";
export * from "./commandIds.js";
export * from "./constants.js";
export * from "./flowSource.js";
export * from "./ntt.js";
export * from "./nttContracts.js";
export * from "./transceiver.js";
