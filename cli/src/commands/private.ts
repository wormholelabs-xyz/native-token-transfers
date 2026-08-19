import { registerPrivateCommands } from "../private/commands.js";

/** Thin factory wrapper — subcommands (list/send/redeem) live in private/commands.ts. */
export function createPrivateCommand() {
  return {
    command: "private",
    describe: "Private-network NTT operations (Canton/Ethereum/Solana)",
    builder: registerPrivateCommands,
    handler: () => {}, // yargs handles subcommand dispatch via builder
  };
}
