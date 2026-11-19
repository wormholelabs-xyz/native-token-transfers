// Pure exercise-body builders for the NttManager choices, golden-testable
// against the exact JSON shapes ntt-lib.sh submits. Numeric choice fields are
// JSON strings on the JSON Ledger API; Optional = null.

function normalizeVaaHex(hex: string): string {
  const stripped = hex.startsWith("0x") || hex.startsWith("0X") ? hex.slice(2) : hex;
  return stripped.toLowerCase();
}

export interface TransferBodyArgs {
  templateId: string;
  managerCid: string;
  user: string; // full party
  operator: string;
  governance: string;
  transceiverEmitterCid: string;
  coreStateCid: string;
  ledgerCid: string | null; // null in burning mode
  recipientChain: number;
  recipientAddress: string; // 32-byte hex, no 0x
  rawAmount: bigint;
  inputHoldingCids: string[];
  commandId: string;
  nonce?: number;
  consistencyLevel?: number;
}

export function buildTransferBody(args: TransferBodyArgs) {
  return {
    commands: [
      {
        ExerciseCommand: {
          templateId: args.templateId,
          contractId: args.managerCid,
          choice: "Transfer",
          choiceArgument: {
            user: args.user,
            transceiverEmitterCid: args.transceiverEmitterCid,
            coreStateCid: args.coreStateCid,
            ledgerCid: args.ledgerCid,
            recipientChain: String(args.recipientChain),
            recipientAddress: args.recipientAddress,
            rawAmount: args.rawAmount.toString(),
            nonce: String(args.nonce ?? 1),
            consistencyLevel: String(args.consistencyLevel ?? 0),
            inputHoldingCids: args.inputHoldingCids,
            feeAllocation: null,
            extraArgs: { context: { values: {} }, meta: { values: {} } },
          },
        },
      },
    ],
    commandId: args.commandId,
    actAs: [args.user],
    readAs: [args.operator, args.governance],
  };
}

export interface ReleaseBodyArgs {
  templateId: string;
  managerCid: string;
  executor: string; // alice
  operator: string;
  governance: string;
  coreStateCid: string;
  replayNodeCid: string;
  vaaBytesHex: string;
  guardianPubkey: string;
  recipient: string; // full party
  ledgerCid: string;
  commandId: string;
}

export function buildReleaseBody(args: ReleaseBodyArgs) {
  return {
    commands: [
      {
        ExerciseCommand: {
          templateId: args.templateId,
          contractId: args.managerCid,
          choice: "Release",
          choiceArgument: {
            executor: args.executor,
            coreStateCid: args.coreStateCid,
            replayNodeCid: args.replayNodeCid,
            vaaBytes: normalizeVaaHex(args.vaaBytesHex),
            pubKeys: [{ _1: "0", _2: args.guardianPubkey }],
            recipient: args.recipient,
            ledgerCid: args.ledgerCid,
            inputHoldingCids: [],
            extraArgs: { context: { values: {} }, meta: { values: {} } },
          },
        },
      },
    ],
    commandId: args.commandId,
    actAs: [args.executor],
    readAs: [args.operator, args.governance],
  };
}

export interface MintBodyArgs {
  templateId: string;
  managerCid: string;
  executor: string;
  operator: string;
  governance: string;
  coreStateCid: string;
  replayNodeCid: string;
  vaaBytesHex: string;
  guardianPubkey: string;
  recipient: string; // full party
  depositPreapprovalCid: string;
  commandId: string;
}

export function buildMintBody(args: MintBodyArgs) {
  return {
    commands: [
      {
        ExerciseCommand: {
          templateId: args.templateId,
          contractId: args.managerCid,
          choice: "Mint",
          choiceArgument: {
            executor: args.executor,
            coreStateCid: args.coreStateCid,
            replayNodeCid: args.replayNodeCid,
            vaaBytes: normalizeVaaHex(args.vaaBytesHex),
            pubKeys: [{ _1: "0", _2: args.guardianPubkey }],
            recipient: args.recipient,
            depositPreapprovalCid: args.depositPreapprovalCid,
            extraArgs: { context: { values: {} }, meta: { values: {} } },
          },
        },
      },
    ],
    commandId: args.commandId,
    actAs: [args.executor],
    readAs: [args.operator, args.governance],
  };
}

/** Outbound sequence from a submit-and-wait-for-transaction-tree response. */
export function extractSequence(txTree: any): string {
  const seq =
    txTree?.transactionTree?.eventsById?.["0"]?.ExercisedTreeEvent?.value
      ?.exerciseResult?.sequence;
  if (seq === undefined || seq === null) {
    throw new Error(
      `no sequence in transaction tree result: ${JSON.stringify(txTree?.cause ?? txTree)}`
    );
  }
  return String(seq);
}

export interface MintedCoin {
  amount: string;
  instrumentId: string;
  owner: string;
}

/** Coins created by an inbound Release/Mint, walked from the transaction tree. */
export function extractMintedCoins(txTree: any): MintedCoin[] {
  const eventsById = txTree?.transactionTree?.eventsById ?? {};
  const results: MintedCoin[] = [];
  for (const event of Object.values(eventsById) as any[]) {
    const created = event?.CreatedTreeEvent?.value;
    if (
      typeof created?.templateId === "string" &&
      created.templateId.endsWith(":Token.CIP0056.Coin:Coin")
    ) {
      results.push({
        amount: created.createArgument.amount,
        instrumentId: created.createArgument.instrumentId?.id,
        owner: created.createArgument.owner,
      });
    }
  }
  return results;
}
