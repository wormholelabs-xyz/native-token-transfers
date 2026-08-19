// Orchestrates Canton-side send/redeem: fresh snapshot -> coin selection /
// preflight -> submit -> report. Thin glue over the pure builders in
// canton/ntt.ts and the ACS finders in canton/acs.ts — no SDK deps here.
import { partyId, type CantonChainConfig, type CantonConnection } from "./config.js";
import type { CantonClient } from "./canton/client.js";
import { snapshot, selectCoin } from "./canton/acs.js";
import {
  buildMintBody,
  buildReleaseBody,
  buildTransferBody,
  extractMintedCoins,
  extractSequence,
  type MintedCoin,
} from "./canton/ntt.js";
import { toRaw } from "./amounts.js";

function commandId(op: string): string {
  return `ntt-private-${op}-${new Date().toISOString()}`;
}

export interface CantonSendResult {
  sequence: string;
  rawAmount: bigint;
}

export interface CantonOpOptions {
  dryRun?: boolean;
}

/**
 * Outbound Transfer: selects a sufficient non-custody holding and submits.
 * In dry-run mode, builds and prints the exact exercise body but never calls
 * submit-and-wait — the ACS snapshot itself is a read, so this still needs
 * live participant access, just no write.
 */
export async function cantonSend(
  client: CantonClient,
  canton: CantonConnection,
  chain: CantonChainConfig,
  amountHuman: string,
  recipientChainId: number,
  recipientAddress32Hex: string,
  opts: CantonOpOptions = {}
): Promise<CantonSendResult | undefined> {
  const rawAmount = toRaw(amountHuman, chain.decimals);
  const state = await snapshot(client, canton, chain);
  const coin = selectCoin(state.coins, rawAmount, chain.decimals, state.custodyCid);

  const user = partyId(canton, canton.parties.user);
  const operator = partyId(canton, canton.parties.operator);
  const governance = partyId(canton, canton.parties.governance);

  const body = buildTransferBody({
    templateId: state.managerTemplateId,
    managerCid: state.managerCid,
    user,
    operator,
    governance,
    transceiverEmitterCid: state.emitterCid,
    coreStateCid: state.coreStateCid,
    ledgerCid: chain.mode === "burning" ? null : state.ledgerCid ?? null,
    recipientChain: recipientChainId,
    recipientAddress: recipientAddress32Hex,
    rawAmount,
    inputHoldingCids: [coin.contractId],
    commandId: commandId("send"),
  });

  if (opts.dryRun) {
    console.log("  [dry-run] would submit:");
    console.log(JSON.stringify(body, null, 2));
    return undefined;
  }

  const result = await client.submitAndWaitForTransactionTree(body);
  return { sequence: extractSequence(result), rawAmount };
}

export interface CantonRedeemResult {
  minted: MintedCoin[];
}

/**
 * Inbound Release (locking mode) or Mint (burning mode). For burning mode,
 * checks for a DepositPreapproval *before* submitting: Mint aborts pre-digest
 * when it's missing, so the VAA stays redeemable, but failing loudly here
 * avoids a wasted round trip and gives an actionable error.
 */
export async function cantonRedeem(
  client: CantonClient,
  canton: CantonConnection,
  chain: CantonChainConfig,
  vaaBytesHex: string,
  recipientPartyHint: string,
  opts: CantonOpOptions = {}
): Promise<CantonRedeemResult | undefined> {
  const recipient = partyId(canton, recipientPartyHint);
  const executor = partyId(canton, canton.parties.user);
  const operator = partyId(canton, canton.parties.operator);
  const governance = partyId(canton, canton.parties.governance);

  const state = await snapshot(client, canton, chain, { recipientParty: recipient });

  let body: unknown;
  if (chain.mode === "burning") {
    if (!state.depositPreapprovalCid) {
      throw new Error(
        `no DepositPreapproval for ${recipient} + ${chain.instrument}; burn/mint inbound ` +
          `redemption needs one — nothing has been submitted, so the VAA stays redeemable ` +
          `once the preapproval exists`
      );
    }
    body = buildMintBody({
      templateId: state.managerTemplateId,
      managerCid: state.managerCid,
      executor,
      operator,
      governance,
      coreStateCid: state.coreStateCid,
      replayNodeCid: state.replayNodeCid,
      vaaBytesHex,
      guardianPubkey: canton.guardianPubkey,
      recipient,
      depositPreapprovalCid: state.depositPreapprovalCid,
      commandId: commandId("mint"),
    });
  } else {
    if (!state.ledgerCid) {
      throw new Error(`no LockedLedger for manager ${chain.manager}; cannot Release`);
    }
    body = buildReleaseBody({
      templateId: state.managerTemplateId,
      managerCid: state.managerCid,
      executor,
      operator,
      governance,
      coreStateCid: state.coreStateCid,
      replayNodeCid: state.replayNodeCid,
      vaaBytesHex,
      guardianPubkey: canton.guardianPubkey,
      recipient,
      ledgerCid: state.ledgerCid,
      commandId: commandId("release"),
    });
  }

  if (opts.dryRun) {
    console.log("  [dry-run] would submit:");
    console.log(JSON.stringify(body, null, 2));
    return undefined;
  }

  const result = await client.submitAndWaitForTransactionTree(body);
  return { minted: extractMintedCoins(result) };
}
