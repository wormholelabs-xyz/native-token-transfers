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
| `chains.Canton.guardianGovernance` | Complete party id of the core's `guardianGovernance` party.           |
| `chains.Canton.vettedPackageIds`   | `ntt` and `core` package ids that you vetted.                         |
| `chains.Ethereum` addresses        | EIP-55 checksummed.                                                   |
| `chains.Solana` addresses          | Base58 values that decode to exactly 32 bytes.                        |
| `decimals`                         | Integer from 0 to 18; at most 10 for Canton (a Daml `Decimal`).       |
| `chainIds`                         | Integers from 1 to 65535, all different.                              |

The loader rejects an unknown field. Thus a schema 1 file fails.
A Canton leg without `guardianGovernance` also fails: the SDK checks that this party signed the
`NttManager`.

A local registry with one Canton and Ethereum deployment:

```json
{
  "schemaVersion": 2,
  "network": "Mainnet",
  "chainIds": { "Canton": 75, "Ethereum": 2, "Solana": 1 },
  "canton": {
    "disclosureServiceUrl": "https://canton-disclosure.labsapis.com",
    "participant": {
      "jsonApiUrl": "https://participant.example.com",
      "oauthTokenUrl": "https://idp.example.com/realms/canton/protocol/openid-connect/token",
      "oauthClientId": "ntt-cli",
      "userPartyId": "alice::1220<64 hex>"
    }
  },
  "deployments": {
    "mytoken": {
      "chains": {
        "Canton": {
          "mode": "burning",
          "decimals": 8,
          "manager": "<64 hex>",
          "emitter": "<64 hex>",
          "instrument": {
            "admin": "guardianGovernance::1220<64 hex>",
            "id": "wormhole-ntt:<64 hex>"
          },
          "reserveCustodian": "admin::1220<64 hex>",
          "guardianGovernance": "guardianGovernance::1220<64 hex>",
          "vettedPackageIds": { "ntt": "<64 hex>", "core": "<64 hex>" }
        },
        "Ethereum": {
          "mode": "locking",
          "decimals": 18,
          "token": "0x<EIP-55 address>",
          "manager": "0x<EIP-55 address>",
          "transceiver": "0x<EIP-55 address>",
          "emitter": "000000000000000000000000<transceiver, 40 hex>"
        }
      }
    }
  }
}
```

## Canton through the SDK

The Canton leg runs on the Canton NTT SDK (`@wormhole-foundation/sdk-canton-ntt`, in `canton/ts`).
The CLI maps the registry leg to the SDK config:

| Registry                      | SDK                                                    |
| ----------------------------- | ------------------------------------------------------ |
| `manager`                     | `contracts.ntt.manager`                                |
| `emitter`                     | `contracts.ntt.transceiver.wormhole`                   |
| `instrument`                  | `contracts.ntt.token`, as `"<admin>/<id>"`             |
| `decimals`                    | `contracts.ntt.tokenDecimals`                          |
| `guardianGovernance`          | `contracts.coreBridge`                                 |
| `reserveCustodian`, `mode`    | `contracts.ntt.canton`                                 |
| `vettedPackageIds`            | `contracts.ntt.canton.nttPackageIds`, `corePackageIds` |
| `canton.disclosureServiceUrl` | `contracts.ntt.canton.flowServiceUrl`                  |

The JSON Ledger API client of the SDK gets an OAuth client-credentials token provider. The SDK
builds the one `Transfer`, `Release` or `Mint` command, and its local-party signer submits it.

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
- A Canton command acts as the user party only. It uses no `readAs` party. A Canton redeem acts as
  the recipient party.
- A Canton send refuses, before anything is built: a paused manager, a core message fee above zero,
  a destination chain that is not a peer of the manager, an amount with dust that the peer cannot
  represent, and registry decimals that differ from the manager's `tokenDecimals`.

## Trust in the disclosure service

The user's participant cannot read the `NttManager`. Only its stakeholders can read it. Thus the
disclosure service names it, and the SDK checks each flow before a submit (the anchor checks of
`canton/ts/src/anchor.ts`):

- Each disclosed `templateId` has the vetted `ntt` or `core` package id of the registry.
- The SDK decodes the `NttManager` blob. Its signatories must be its admin, `guardianGovernance` and
  `nttCustody`, and include the registry `guardianGovernance` and `reserveCustodian`. Its address,
  mode, instrument and `reserveCustodian` must equal the registry values.
- The `/v1/managers` entry of the registry manager address must name the same contract, mode,
  instrument and `reserveCustodian`.
- The other disclosed contracts must be signed by `guardianGovernance` (the core contracts, the
  `LockedLedger`), by the recipient or the reserve custodian (a `DepositPreapproval`, a
  `TransferPreapproval`, the custody pot), or be the factory that the `NttManager` commits.

Anyone can register a lock/unlock deployment, and `guardianGovernance` signs every one. The pinned
`reserveCustodian` must also sign the manager, thus the tokens of a send can only go into custody of
that party. A send also passes the registry manager address as the `Transfer` argument
`expectedManagerAddress` (`ntt` 0.7.0 and later). The ledger rejects the send when the named
`NttManager` has another manager address. A redeem has no such argument. `Release` and `Mint` pay only
the recipient that the VAA names, and a different manager rejects the VAA.

A lock or release through the token-cip0056 `CoinFactory` settles only with the receiver's
`TransferPreapproval`. The SDK puts it in the choice's `extraArgs` context: the custodian's for a
send, the recipient's for a release.

Limits:

- The CLI does not check that a Canton recipient party exists. The flow routes need a VAA digest, thus
  no recipient check exists before a send. The typed party id is the control.
- `feeAllocation` is always empty. The SDK refuses a send when the core message fee is above zero.

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
- A Canton submit waits up to 120 s. A timeout or a lost connection gives "result unknown" with the
  command id and the `--transfer-nonce` value. A rejection by the ledger gives "was rejected" with the
  same values.
- A Canton redeem uses a command id from the VAA digest.
- When the ledger reports that a disclosed contract is no longer active, the CLI waits 5.5 s (past the
  disclosure service cache), reads the flow again and resubmits under the same command id. It makes
  three attempts in all, for a send and for a redeem.
- The command ids are the same as before the CLI used the SDK. A retry with an older CLI version is
  deduplicated too.
