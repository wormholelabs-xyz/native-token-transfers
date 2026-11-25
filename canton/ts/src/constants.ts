import type { Network } from "@wormhole-foundation/sdk-base";
import type { CorePackageIds } from "@wormhole-foundation/sdk-canton";

/** Package name of the NTT Daml package */
export const NTT_PACKAGE_NAME = "ntt";

/** Package id of ntt 0.7.0 (Transfer takes expectedManagerAddress; party tag "wormhole:v1") */
export const NTT_PACKAGE_ID_0_7_0 =
  "43e1b2060c420d9bbd78ebeaeca1c30b58ee967fd40b5aacd58088d6f239a63d";

/** The pinned ntt package of a network: the current id, and a retiring one during an upgrade */
export type NttPackageIds = CorePackageIds;

/**
 * Pinned ntt packages per network. Testnet and Mainnet are added once ntt 0.7.0 is deployed
 * there; until then the caller passes them in its config.
 */
export const nttPackageIds: { readonly [N in Network]?: NttPackageIds } = {
  Devnet: { current: NTT_PACKAGE_ID_0_7_0 },
};

/** The pinned ntt package ids of a network, current first. Throws if the network has none. */
export function pinnedNttPackageIds(
  network: Network,
  override?: NttPackageIds
): string[] {
  const ids = override ?? nttPackageIds[network];
  if (!ids) throw new Error(`No ntt package id is known for Canton ${network}`);
  return ids.retiring ? [ids.current, ids.retiring] : [ids.current];
}

/**
 * Base URL of the NTT flow service (canton/disclosure) per network. The service is public and the
 * same for every user of a network. A network is added once its service is live; until then the
 * caller passes a URL (contracts.ntt.canton.flowServiceUrl) or a flow source of its own.
 */
export const nttFlowServiceUrls: { readonly [N in Network]?: string } = {};

/** Timeout of a request to the flow service */
export const FLOW_REQUEST_TIMEOUT_MS = 15_000;
/** Size cap of a flow service response (blobs and ids, plus decoded payloads) */
export const FLOW_RESPONSE_MAX_BYTES = 10 * 1024 * 1024;
/** Entries one flow response may hold before the client refuses */
export const FLOW_DISCLOSURES_MAX = 32;
/** Managers one /v1/managers response may hold before the client refuses */
export const MANAGERS_MAX = 512;

/**
 * Wait before a rebuild reads a flow again after a stale-contract rejection. The flow service
 * caches answers, failures included, for 5 seconds by default (plan risk NR5).
 */
export const FLOW_REBUILD_DELAY_MS = 5_500;

/** Canton finality is immediate; the guardian watcher accepts 0 */
export const TRANSFER_CONSISTENCY_LEVEL = 0;

/** NTT trims wire amounts to at most 8 decimals */
export const NTT_WIRE_DECIMALS_MAX = 8;

/** Command id prefixes, shared with the ntt CLI so a retry across both is deduplicated */
export const TRANSFER_COMMAND_ID_PREFIX = "ntt-private-send-";
export const REDEEM_COMMAND_ID_PREFIX = "ntt-private-redeem-";
/** Command id prefix of an AdminTransferProposal create (setOwner) */
export const PROPOSE_ADMIN_COMMAND_ID_PREFIX = "ntt-admin-propose-";
