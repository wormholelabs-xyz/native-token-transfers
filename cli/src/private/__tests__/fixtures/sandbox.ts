// A registry over the recorded sandbox run of the Canton NTT SDK (canton/ts/__tests__/fixtures,
// recorded from a local Canton 3.7.3 sandbox with the NTT flow service): its burning and locking
// deployments, each with the Ethereum peer the recorded VAAs come from. Party, contract and
// package ids are from that sandbox only.
import { getAddress } from "ethers";

import { SANDBOX } from "../../../../../canton/ts/__tests__/fixtures/sandbox.js";
import { TEST_CORE_PACKAGE_ID, TEST_NTT_PACKAGE_ID } from "./registry.js";

export { SANDBOX };
export const SETUP = SANDBOX.setup;

export const FLOW_SERVICE_URL = "https://flows.example.test";
export const JSON_API_URL = "https://participant.example.test";

/** The Ethereum peer of the recorded VAAs: manager b1ff5e8b..., transceiver c2aa5e8b... */
const PEER_MANAGER = getAddress("0xb1ff5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f");
const PEER_TRANSCEIVER = getAddress(
  "0xc2aa5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f"
);

function cantonLeg(deployment: "burning" | "locking") {
  const setup = SETUP[deployment];
  return {
    mode: deployment,
    decimals: setup.tokenDecimals,
    manager: setup.managerAddress,
    emitter: setup.transceiverAddress,
    instrument: { admin: setup.instrumentAdmin, id: setup.instrumentId },
    reserveCustodian: setup.reserveCustodian,
    guardianGovernance: SETUP.guardianGovernance,
    vettedPackageIds: { ntt: TEST_NTT_PACKAGE_ID, core: TEST_CORE_PACKAGE_ID },
  };
}

function ethereumLeg() {
  return {
    mode: "locking",
    decimals: 18,
    token: "0x88c4f2412c1f2CA5D459E3355C44F04be7bA8cA8",
    manager: PEER_MANAGER,
    transceiver: PEER_TRANSCEIVER,
    emitter: `${"00".repeat(12)}${PEER_TRANSCEIVER.slice(2).toLowerCase()}`,
  };
}

/** Deployments "sbxburn" (burning) and "sbxlock" (locking); the user is the sandbox user */
export function sandboxRegistryJson(): any {
  return {
    schemaVersion: 2,
    network: "Mainnet",
    chainIds: { Canton: 75, Ethereum: 2, Solana: 1 },
    canton: {
      disclosureServiceUrl: FLOW_SERVICE_URL,
      participant: {
        jsonApiUrl: JSON_API_URL,
        oauthTokenUrl:
          "https://idp.example.test/realms/test/protocol/openid-connect/token",
        oauthClientId: "test-client",
        userPartyId: SETUP.user,
      },
    },
    deployments: {
      sbxburn: {
        chains: { Canton: cantonLeg("burning"), Ethereum: ethereumLeg() },
      },
      sbxlock: {
        chains: { Canton: cantonLeg("locking"), Ethereum: ethereumLeg() },
      },
    },
  };
}
