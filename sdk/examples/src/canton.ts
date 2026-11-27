// An NTT transfer from Canton to Sepolia and back, through the manual NTT route.
//
// Canton has no executor yet, so both legs are manual: the sender initiates, the guardians sign
// the VAA, and a signer on the destination redeems it. On Canton the signer is a party hosted on
// your own participant (a local party); its JSON Ledger API token must act as that party.
//
// Set in `.env` (see helpers.ts for the EVM key):
//   CANTON_JSON_API_URL         your participant's JSON Ledger API
//   CANTON_TOKEN                a bearer token that acts as CANTON_PARTY (or use the OAuth
//                               client-credentials provider below)
//   CANTON_PARTY                your party id "<hint>::1220<64 hex>"
//   CANTON_GUARDIAN_GOVERNANCE  the core's guardianGovernance party (contracts.coreBridge)
//   CANTON_NTT_FLOW_SERVICE_URL the NTT flow service of the network
// and the deployment values in CANTON_NTT below.
import {
  Network,
  Wormhole,
  amount,
  routes,
  type ChainsConfig,
} from "@wormhole-foundation/sdk";
import evm from "@wormhole-foundation/sdk/platforms/evm";
import {
  CantonPlatform,
  getCantonSigner,
  staticTokenProvider,
} from "@wormhole-foundation/sdk-canton";

// register protocol implementations
import { register as registerDefinitionsNtt } from "@wormhole-foundation/sdk-definitions-ntt";
import { register as registerEvmNtt } from "@wormhole-foundation/sdk-evm-ntt";
import { register as registerCantonNtt } from "@wormhole-foundation/sdk-canton-ntt";

registerDefinitionsNtt();
registerEvmNtt();
registerCantonNtt();

import { NttRoute, nttManualRoute } from "@wormhole-foundation/sdk-route-ntt";
import { getSigner } from "./helpers.js";

function env(key: string): string {
  const value = process.env[key];
  if (!value) throw new Error(`Missing env var ${key}`);
  return value;
}

// The Canton leg of the deployment. The token is the CIP-0056 instrument "<admin>/<id>"; the
// manager and the transceiver are 64-hex addresses. A Canton transfer needs the reserve
// custodian; the flow service URL and the pinned package ids are needed until the SDK has
// defaults for the network.
const CANTON_TOKEN = "<instrument admin party>/<instrument id>";
const CANTON_TOKEN_DECIMALS = 8;
const CANTON_NTT: NttRoute.TokenConfig = {
  chain: "Canton",
  token: CANTON_TOKEN,
  manager: "<64 hex manager address>",
  transceiver: [{ type: "wormhole", address: "<64 hex transceiver address>" }],
  tokenDecimals: CANTON_TOKEN_DECIMALS,
  canton: {
    reserveCustodian: "<reserve custodian party>",
    flowServiceUrl: process.env["CANTON_NTT_FLOW_SERVICE_URL"],
    nttPackageIds: { current: "<64 hex ntt package id>" },
    corePackageIds: { current: "<64 hex wormhole-core package id>" },
    mode: "burning",
  },
};
// The peer deployment on Sepolia
const SEPOLIA_NTT: NttRoute.TokenConfig = {
  chain: "Sepolia",
  token: "<token address>",
  manager: "<NttManager address>",
  transceiver: [{ type: "wormhole", address: "<WormholeTransceiver address>" }],
  tokenDecimals: 18,
};

/**
 * The Canton platform with the settings a generic chain config has no field for. The ledger keeps
 * no decimals for an instrument, so the NTT token's decimals (its manager's tokenDecimals) are
 * given here: without them the platform reports 10, and the route refuses the request.
 */
class Canton<N extends Network> extends CantonPlatform<N> {
  constructor(network: N, chains?: ChainsConfig<N, "Canton">) {
    super(network, chains, {
      tokenProvider: staticTokenProvider(env("CANTON_TOKEN")),
      // or: oauthClientCredentialsTokenProvider({ tokenUrl, clientId, clientSecret })
      instrumentDecimals: { [CANTON_TOKEN]: CANTON_TOKEN_DECIMALS },
    });
  }
}

(async function () {
  (await import("dotenv")).config();

  const wh = new Wormhole("Testnet", [evm.Platform, Canton], {
    chains: {
      Canton: {
        rpc: env("CANTON_JSON_API_URL"),
        contracts: { coreBridge: env("CANTON_GUARDIAN_GOVERNANCE") },
      },
    },
  });
  const canton = wh.getChain("Canton");
  const sepolia = wh.getChain("Sepolia");

  const cantonParty = env("CANTON_PARTY");
  const cantonSigner = await getCantonSigner(
    await canton.getRpc(),
    cantonParty
  );
  const cantonAddress = Wormhole.chainAddress("Canton", cantonParty);
  const sepoliaSigner = await getSigner(sepolia);

  const resolver = wh.resolver([
    nttManualRoute({ tokens: { Example: [CANTON_NTT, SEPOLIA_NTT] } }),
  ]);
  const cantonToken = Wormhole.tokenId("Canton", CANTON_NTT.token);
  const sepoliaToken = Wormhole.tokenId("Sepolia", SEPOLIA_NTT.token);

  // Canton -> Sepolia. The Transfer locks or burns from the party's unlocked holdings and
  // publishes the NTT message; the guardians sign it, and the Sepolia signer redeems it.
  {
    const request = await routes.RouteTransferRequest.create(wh, {
      source: cantonToken,
      destination: sepoliaToken,
    });
    const [route] = await resolver.findRoutes(request);
    if (!route || !routes.isManual(route)) throw new Error("No manual route");
    const validated = await route.validate(request, { amount: "1.5" });
    if (!validated.valid) throw validated.error;
    const quote = await route.quote(request, validated.params);
    if (!quote.success) throw quote.error;
    console.log(
      "Sending",
      amount.display(quote.sourceToken.amount),
      "from Canton"
    );
    const receipt = await route.initiate(
      request,
      cantonSigner,
      quote,
      sepoliaSigner.address
    );
    console.log("Initiated on Canton:", receipt);
    // Reads the message id from the update, waits for the VAA, redeems on Sepolia.
    await routes.checkAndCompleteTransfer(route, receipt, sepoliaSigner.signer);
  }

  // Sepolia -> Canton. The VAA holds only the hash of the recipient party; the route keeps the
  // party from initiate, so any party on your participant can redeem it for the recipient. A
  // burning-mode recipient needs a DepositPreapproval of the deployment first.
  {
    const request = await routes.RouteTransferRequest.create(wh, {
      source: sepoliaToken,
      destination: cantonToken,
    });
    const [route] = await resolver.findRoutes(request);
    if (!route || !routes.isManual(route)) throw new Error("No manual route");
    const validated = await route.validate(request, { amount: "1.5" });
    if (!validated.valid) throw validated.error;
    const quote = await route.quote(request, validated.params);
    if (!quote.success) throw quote.error;
    const receipt = await route.initiate(
      request,
      sepoliaSigner.signer,
      quote,
      cantonAddress
    );
    console.log("Initiated on Sepolia:", receipt);
    // Release (locking) or Mint (burning) as the Canton party. A redeem that is retried after a
    // stale contract keeps its command id, derived from the VAA digest.
    await routes.checkAndCompleteTransfer(route, receipt, cantonSigner);
  }
})();
