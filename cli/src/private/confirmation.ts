// Typed confirmation before a submit: the network name, then the recipient (full Canton party
// id, or the last 8 hex characters of the recipient bytes32).
import type { NetworkName } from "./config.js";
import type { CantonPartyId, Hex32 } from "./types.js";

const RECIPIENT_SUFFIX_HEX_LENGTH = 8;

export interface SubmitConfirmation {
  network: NetworkName;
  operation: "send" | "redeem";
  deploymentName: string;
  route: string;
  amountText: string;
  recipientDescription: string;
  recipient: Hex32;
  cantonRecipientPartyId?: CantonPartyId;
}

export interface ConfirmationIo {
  assumeYes: boolean;
  stdinIsTty: boolean;
  prompt: (question: string) => Promise<string>;
  /** --yes-recipient: with --yes, must equal the Canton recipient party id. */
  confirmedCantonRecipient?: string;
}

export function confirmationLines(confirmation: SubmitConfirmation): string[] {
  return [
    `network:    ${confirmation.network}`,
    `operation:  ${confirmation.operation} on ${confirmation.deploymentName}, ${confirmation.route}`,
    `amount:     ${confirmation.amountText}`,
    `recipient:  ${confirmation.recipientDescription}`,
    `bytes32:    0x${confirmation.recipient}`,
  ];
}

export async function requireTypedConfirmation(
  confirmation: SubmitConfirmation,
  io: ConfirmationIo
): Promise<void> {
  if (io.assumeYes) {
    if (
      confirmation.cantonRecipientPartyId &&
      io.confirmedCantonRecipient !== confirmation.cantonRecipientPartyId
    ) {
      throw new Error(
        "--yes with a Canton recipient needs --yes-recipient <the same complete party id>; nothing has been submitted"
      );
    }
    return;
  }
  if (!io.stdinIsTty)
    throw new Error(
      "stdin is not a terminal: pass --yes to submit without the typed confirmation"
    );
  const typedNetwork = (
    await io.prompt(
      `type the network name (${confirmation.network}) to continue: `
    )
  ).trim();
  if (typedNetwork !== confirmation.network) {
    throw new Error(
      `typed "${typedNetwork}", expected the network name; nothing has been submitted`
    );
  }
  if (confirmation.cantonRecipientPartyId) {
    const typedParty = (
      await io.prompt("type the full Canton recipient party id: ")
    ).trim();
    if (typedParty !== confirmation.cantonRecipientPartyId) {
      throw new Error(
        "typed party id differs from the recipient; nothing has been submitted"
      );
    }
    return;
  }
  const expectedSuffix = confirmation.recipient.slice(
    -RECIPIENT_SUFFIX_HEX_LENGTH
  );
  const typedSuffix = (
    await io.prompt(
      `type the last ${RECIPIENT_SUFFIX_HEX_LENGTH} hex characters of the recipient bytes32: `
    )
  )
    .trim()
    .toLowerCase();
  if (typedSuffix !== expectedSuffix) {
    throw new Error(
      "typed recipient suffix differs; nothing has been submitted"
    );
  }
}
