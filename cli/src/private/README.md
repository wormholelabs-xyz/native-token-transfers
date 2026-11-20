# `ntt private` commands

The `ntt private` commands move NTT transfers between Canton, Ethereum, and Solana.

- `ntt private list` shows the deployments in the registry.
- `ntt private send <amount> --deployment <name> --from <chain> --to <chain>` sends a transfer.
- `ntt private redeem --deployment <name> --vaa <hex>` redeems a guardian-signed VAA.

## Registry

The registry is a JSON file with `schemaVersion` 2. The CLI reads the first registry in this list:

1. The `--deployments` path.
2. The `NTT_PRIVATE_DEPLOYMENTS` path.
3. `private-deployments.json` at the repository root.

The CLI does not read a registry from the working directory.
Each run prints the registry path and the first 16 characters of its SHA-256.
A registry that is not the repository file gives a red warning.

The repository file holds the network, the chain ids and the public disclosure service URL.
It has no deployments. Put the deployments and the participant settings in a local file.

| Field                              | Rule                                                                  |
| ---------------------------------- | --------------------------------------------------------------------- |
| `canton.disclosureServiceUrl`      | `https:` URL. Use `http:` for `localhost` only.                       |
| `canton.participant.jsonApiUrl`    | `https:` URL of your own participant JSON Ledger API.                 |
| `canton.participant.oauthTokenUrl` | Full `https:` token URL of the identity provider.                     |
| `canton.participant.oauthClientId` | The OAuth client id. The secret is `CANTON_OAUTH_CLIENT_SECRET`.      |
| `canton.participant.userPartyId`   | Complete party id `<hint>::1220<64 hex>`. The CLI acts as this party. |
| `chains.Canton.instrument`         | `{ admin, id }` of the token instrument.                              |
| `chains.Canton.reserveCustodian`   | Complete party id of the reserve custodian.                           |
| `chains.Canton.vettedPackageIds`   | `ntt` and `core` package ids that you vetted.                         |
| `chains.Ethereum` addresses        | EIP-55 checksummed.                                                   |
| `chains.Solana` addresses          | Base58 values that decode to exactly 32 bytes.                        |
| `decimals`                         | Integer from 0 to 18.                                                 |
| `chainIds`                         | Integers from 1 to 65535, all different.                              |

The loader rejects an unknown field. Thus a schema 1 file fails.

## Checks before a submit

- The CLI shows the network, the route, the amount and the recipient.
- You type the network name. Then you type the recipient:
  - For a Canton recipient, type the complete party id.
  - For other recipients, type the last 8 hex characters of the recipient bytes32.
- `--yes` skips the typed confirmation. Without `--yes`, a submit from a non-terminal input stops.
- For a Canton recipient, `--yes` also needs `--yes-recipient` with the same complete party id.
- `--recipient-raw` needs `--i-accept-raw-recipient`.
- An Ethereum send checks the RPC chain id, the manager token, the token decimals and the destination peer.
- A Solana send checks the manager mint, the mint decimals and the destination peer.
- The registry chain ids must equal the SDK chain ids. On Testnet, `chainIds.Ethereum` is 10002 (Sepolia).
- `SOLANA_PAYER_PATH` must have mode 600 or 400.
- A Canton operation gets its contracts from the disclosure service flow routes.
- A Canton command acts as the user party only. It uses no `readAs` party.

## Trust in the disclosure service

The CLI trusts the disclosure service to name the correct `NttManager` contract. The reason is that
the user's participant cannot read the `NttManager`. Only its stakeholders can read it.

The CLI compares the `NttManager` role with these values:

- The `/v1/managers` entry of the registry manager address: contract id, token mode, instrument,
  `reserveCustodian` and pause state.
- The vetted `ntt` and `core` package ids of the registry.

`/v1/managers` comes from the same service. Thus a changed service can name a different `NttManager`
contract of the vetted package with the same values. The ledger binds each disclosed blob to its
contract id, but the CLI cannot decode the blob.

Limits:

- The CLI does not check that a Canton recipient party exists. The flow routes need a VAA digest, thus
  no recipient check exists before a send. The typed party id is the control.
- `/v1/managers` gives no token decimals. The CLI does not compare the Canton decimals with the ledger.
- `feeAllocation` is always empty. A core message fee above zero makes each Canton send fail. No funds move.

## Retries

- The CLI signs an Ethereum transaction first. It prints the hash, then it broadcasts.
  After the signature, each error gives the hash and "do not re-run; check the hash".
- The CLI signs a Solana transaction first. It prints the signature, then it sends.
  After the signature, a send error gives "result unknown; do not re-run; check the signature".
  A read error during the wait continues the bounded poll. A poll that ends without an answer gives the same message.
  The CLI reports "expired, safe to retry" only when a second status read after the block-height limit finds no transaction.
  If the sequence read fails after the confirmation, the CLI shows the signature without a sequence.
- A Canton send prints its transfer nonce. To retry the same transfer, use `--transfer-nonce` with that value.
  The command id is then the same, and the participant rejects a duplicate.
- A Canton submit waits up to 120 s. A timeout or another submit error gives "result unknown" with the
  command id and the `--transfer-nonce` value.
- A Canton redeem uses a command id from the VAA digest.
