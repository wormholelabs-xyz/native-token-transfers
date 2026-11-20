// Send recipient resolution: every destination parses to a Hex32 through one typed parser.
import { recipientBinding } from "./binding.js";
import type { ChainName, PrivateRegistry } from "./config.js";
import {
  evmAddressToHex32,
  parseCantonPartyId,
  parseEvmAddress,
  parseHex32,
  parseSolanaAddressBytes32,
  type CantonPartyId,
  type Hex32,
} from "./types.js";

export interface RecipientOptions {
  recipient?: string;
  recipientRaw?: string;
  acceptRawRecipient?: boolean;
  acceptUnchecksummedAddress?: boolean;
}

export interface RecipientDefaults {
  ethereumAddress?: string;
  solanaAddress?: string;
}

export interface ResolvedRecipient {
  recipient: Hex32;
  description: string;
  cantonPartyId?: CantonPartyId;
  isRaw: boolean;
}

export function resolveSendRecipient(
  destinationChain: ChainName,
  registry: PrivateRegistry,
  options: RecipientOptions,
  defaults: RecipientDefaults
): ResolvedRecipient {
  if (options.recipientRaw !== undefined) {
    if (!options.acceptRawRecipient) {
      throw new Error(
        "--recipient-raw needs --i-accept-raw-recipient: a wrong raw value strands the funds"
      );
    }
    const recipient = parseHex32(options.recipientRaw.toLowerCase());
    return {
      recipient,
      description: `raw bytes32 0x${recipient}`,
      isRaw: true,
    };
  }

  if (destinationChain === "Canton") {
    const partyText =
      options.recipient ?? registry.canton.participant?.userPartyId;
    if (!partyText)
      throw new Error(
        "no Canton recipient: pass --recipient <hint>::1220<64 hex>"
      );
    const cantonPartyId = parseCantonPartyId(partyText);
    const recipient = recipientBinding(cantonPartyId);
    return {
      recipient,
      description: `Canton party ${cantonPartyId} (binding 0x${recipient})`,
      cantonPartyId,
      isRaw: false,
    };
  }

  if (destinationChain === "Ethereum") {
    const addressText = options.recipient ?? defaults.ethereumAddress;
    if (!addressText)
      throw new Error(
        "no Ethereum recipient: pass --recipient <address> or set ETH_PRIVATE_KEY"
      );
    const address = parseEvmAddress(addressText, {
      acceptUnchecksummed: options.acceptUnchecksummedAddress ?? false,
    });
    return {
      recipient: evmAddressToHex32(address),
      description: `Ethereum address ${address}`,
      isRaw: false,
    };
  }

  const solanaAddress = options.recipient ?? defaults.solanaAddress;
  if (!solanaAddress) {
    throw new Error("no Solana recipient: pass --recipient <address>");
  }
  return {
    recipient: parseSolanaAddressBytes32(solanaAddress),
    description: `Solana address ${solanaAddress}`,
    isRaw: false,
  };
}
