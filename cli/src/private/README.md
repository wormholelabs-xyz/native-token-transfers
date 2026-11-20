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
- `--recipient-raw` needs `--i-accept-raw-recipient`.
- An Ethereum send checks the RPC chain id and the peer manager of the destination chain.
- A Canton operation gets its contracts from the disclosure service flow routes.
  It compares the `NttManager` with `/v1/managers` and the registry before it submits.
- A Canton command acts as the user party only. It uses no `readAs` party.

## Retries

- After an Ethereum broadcast, the CLI does not send again. If the receipt does not come, the CLI shows the hash.
  Examine the hash before you run the command again.
- A Canton send prints its transfer nonce. To retry the same transfer, use `--transfer-nonce` with that value.
  The command id is then the same, and the participant rejects a duplicate.
- A Canton redeem uses a command id from the VAA digest.
