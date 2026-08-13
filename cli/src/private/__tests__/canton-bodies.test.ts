import { describe, expect, test } from "bun:test";
import {
  buildMintBody,
  buildReleaseBody,
  buildTransferBody,
} from "../canton/ntt.js";

const NAMESPACE =
  "122085bf64156420a95663eb8b8cce516858de1d531d95850debe9ec8d15e3f6263e";
const ALICE = `alice::${NAMESPACE}`;
const OPERATOR = `operator::${NAMESPACE}`;
const GOVERNANCE = `guardianGovernance::${NAMESPACE}`;
const GUARDIAN_PUBKEY =
  "042cb0fec84a83ca7b143d369451d73bc1c26080c93ee813142bf0f0351fa76ca711b20b0294c495ecbf04d9daf7c97cf84a5f1262716ca2c06bbf7e0b72059c85";
const NTT_PKG =
  "36a5828273137240590049f754d2946295b8beb2cb00fa390ba7a1c5a34d0ae9";
const TEMPLATE_ID = `${NTT_PKG}:Wormhole.Ntt.Manager:NttManager`;

describe("buildTransferBody", () => {
  // canhub Canton leg: mode locking -> ledgerCid is a contract id, not null.
  test("locking mode carries a ledgerCid", () => {
    const body = buildTransferBody({
      templateId: TEMPLATE_ID,
      managerCid: "mgr-canhub-cid",
      user: ALICE,
      operator: OPERATOR,
      governance: GOVERNANCE,
      transceiverEmitterCid: "em-canhub-cid",
      coreStateCid: "cs-cid",
      ledgerCid: "ledger-canhub-cid",
      recipientChain: 2,
      recipientAddress:
        "000000000000000000000000d10c939ce11814d4c1df1280d091eb3a6f2797ae",
      rawAmount: 12345n,
      inputHoldingCids: ["coin-canhub-cid"],
      commandId: "test-transfer-locking",
    });

    expect(body).toEqual({
      commands: [
        {
          ExerciseCommand: {
            templateId: TEMPLATE_ID,
            contractId: "mgr-canhub-cid",
            choice: "Transfer",
            choiceArgument: {
              user: ALICE,
              transceiverEmitterCid: "em-canhub-cid",
              coreStateCid: "cs-cid",
              ledgerCid: "ledger-canhub-cid",
              recipientChain: "2",
              recipientAddress:
                "000000000000000000000000d10c939ce11814d4c1df1280d091eb3a6f2797ae",
              rawAmount: "12345",
              nonce: "1",
              consistencyLevel: "0",
              inputHoldingCids: ["coin-canhub-cid"],
              feeAllocation: null,
              extraArgs: { context: { values: {} }, meta: { values: {} } },
            },
          },
        },
      ],
      commandId: "test-transfer-locking",
      actAs: [ALICE],
      readAs: [OPERATOR, GOVERNANCE],
    });
  });

  // canburn Canton leg: mode burning -> ledgerCid must be null.
  test("burning mode sets ledgerCid to null", () => {
    const body = buildTransferBody({
      templateId: TEMPLATE_ID,
      managerCid: "mgr-canburn-cid",
      user: ALICE,
      operator: OPERATOR,
      governance: GOVERNANCE,
      transceiverEmitterCid: "em-canburn-cid",
      coreStateCid: "cs-cid",
      ledgerCid: null,
      recipientChain: 1,
      recipientAddress:
        "fc12b0dc42b4d8b280cf13e2ee66fb6b2e6a206b5776b3c8ec17830bd35ba524",
      rawAmount: 500000000n,
      inputHoldingCids: ["coin-canburn-cid"],
      commandId: "test-transfer-burning",
    });

    expect(body).toEqual({
      commands: [
        {
          ExerciseCommand: {
            templateId: TEMPLATE_ID,
            contractId: "mgr-canburn-cid",
            choice: "Transfer",
            choiceArgument: {
              user: ALICE,
              transceiverEmitterCid: "em-canburn-cid",
              coreStateCid: "cs-cid",
              ledgerCid: null,
              recipientChain: "1",
              recipientAddress:
                "fc12b0dc42b4d8b280cf13e2ee66fb6b2e6a206b5776b3c8ec17830bd35ba524",
              rawAmount: "500000000",
              nonce: "1",
              consistencyLevel: "0",
              inputHoldingCids: ["coin-canburn-cid"],
              feeAllocation: null,
              extraArgs: { context: { values: {} }, meta: { values: {} } },
            },
          },
        },
      ],
      commandId: "test-transfer-burning",
      actAs: [ALICE],
      readAs: [OPERATOR, GOVERNANCE],
    });
  });
});

describe("buildReleaseBody", () => {
  // canhub Canton leg (LockUnlock) inbound: Release, vaaBytes normalized to
  // lowercase with any 0x prefix stripped.
  test("golden shape, vaaBytes normalized", () => {
    const body = buildReleaseBody({
      templateId: TEMPLATE_ID,
      managerCid: "mgr-canhub-cid",
      executor: ALICE,
      operator: OPERATOR,
      governance: GOVERNANCE,
      coreStateCid: "cs-cid",
      replayNodeCid: "replay-canhub-cid",
      vaaBytesHex: "0X0100000000010015FF333D",
      guardianPubkey: GUARDIAN_PUBKEY,
      recipient: ALICE,
      ledgerCid: "ledger-canhub-cid",
      commandId: "test-release",
    });

    expect(body).toEqual({
      commands: [
        {
          ExerciseCommand: {
            templateId: TEMPLATE_ID,
            contractId: "mgr-canhub-cid",
            choice: "Release",
            choiceArgument: {
              executor: ALICE,
              coreStateCid: "cs-cid",
              replayNodeCid: "replay-canhub-cid",
              vaaBytes: "0100000000010015ff333d",
              pubKeys: [{ _1: "0", _2: GUARDIAN_PUBKEY }],
              recipient: ALICE,
              ledgerCid: "ledger-canhub-cid",
              inputHoldingCids: [],
              extraArgs: { context: { values: {} }, meta: { values: {} } },
            },
          },
        },
      ],
      commandId: "test-release",
      actAs: [ALICE],
      readAs: [OPERATOR, GOVERNANCE],
    });
  });
});

describe("buildMintBody", () => {
  // canburn Canton leg (BurnMint) inbound: Mint, requires a DepositPreapproval cid.
  test("golden shape, vaaBytes normalized", () => {
    const body = buildMintBody({
      templateId: TEMPLATE_ID,
      managerCid: "mgr-canburn-cid",
      executor: ALICE,
      operator: OPERATOR,
      governance: GOVERNANCE,
      coreStateCid: "cs-cid",
      replayNodeCid: "replay-canburn-cid",
      vaaBytesHex: "0100000000010015FF333D",
      guardianPubkey: GUARDIAN_PUBKEY,
      recipient: ALICE,
      depositPreapprovalCid: "dep-canburn-cid",
      commandId: "test-mint",
    });

    expect(body).toEqual({
      commands: [
        {
          ExerciseCommand: {
            templateId: TEMPLATE_ID,
            contractId: "mgr-canburn-cid",
            choice: "Mint",
            choiceArgument: {
              executor: ALICE,
              coreStateCid: "cs-cid",
              replayNodeCid: "replay-canburn-cid",
              vaaBytes: "0100000000010015ff333d",
              pubKeys: [{ _1: "0", _2: GUARDIAN_PUBKEY }],
              recipient: ALICE,
              depositPreapprovalCid: "dep-canburn-cid",
              extraArgs: { context: { values: {} }, meta: { values: {} } },
            },
          },
        },
      ],
      commandId: "test-mint",
      actAs: [ALICE],
      readAs: [OPERATOR, GOVERNANCE],
    });
  });
});
