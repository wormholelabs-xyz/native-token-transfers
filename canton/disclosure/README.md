# canton/disclosure

This directory holds the `service/` Go module: the production disclosure
service. On Canton, a submitter cannot fetch a contract it is not a
stakeholder of; it must attach the contract as a disclosure. This service
serves those disclosures over HTTP. Each flow's set comes from its choice
body in `canton/ntt/daml/Wormhole/Ntt/Manager.daml`.

Testnet base URL: `https://canton-disclosure.labsapis.com`.

## Endpoints

- `GET /v1/healthz` — liveness: disclosing parties, template count, ledger end.
- `GET /v1/flows/{flow}` — one flow's assembled disclosure set, each contract
  labeled by its role. Parameters per flow:

  | Flow | Params |
  |---|---|
  | `release` | `manager`, `digest`, `recipient` (optional) |
  | `mint` | `manager`, `digest`, `recipient` |
  | `set-peer` | `manager`, `digest` |
  | `accept-admin` | `manager`, `digest` |
  | `transfer` | `manager` |
  | `register` | `gg` (optional), `by-vaa` (bool, optional) |
  | `consolidate` | `manager` |

- `GET /v1/disclosures?template=Module:Entity` — raw blobs for one
  allow-listed template. Repeat `template=` for several. With no parameter,
  it returns every allow-listed template.

`manager` and `digest` are hex strings, 64 characters each: the manager
address and the VAA hash. A flow response carries a `disclosures` array
(role, templateId, contractId, createdEventBlob, synchronizerId) and a
`missing` array. `missing` names owner-only set members (proposals,
preapprovals) the disclosing parties cannot see; the client supplies those.
Attach each `disclosures` entry to the command submission as a
`DisclosedContract`, and read each role's `contractId` for the choice
arguments.

## Examples

```
# One flow's whole set.
curl 'https://canton-disclosure.labsapis.com/v1/flows/release?manager=<hex64>&digest=<hex64>'

# One template, by its Module:Entity tail.
curl 'https://canton-disclosure.labsapis.com/v1/disclosures?template=Token.CIP0056.CoinFactory:CoinFactory'

# Several templates: repeat the parameter.
curl 'https://canton-disclosure.labsapis.com/v1/disclosures?template=Wormhole.Ntt.Manager:NttManager&template=Wormhole.Core.State:CoreState'
```

A `template` value needs both segments, joined by `:`; `Module` is the Daml
module, `Entity` the template name. A package-qualified value
(`#token-cip0056:Token.CIP0056.CoinFactory:CoinFactory`) hits the same
allow-list entry; URL-encode its `#` as `%23`. A template outside the
allow-list gets a 403.

## Flags

- `--json-api` (required) — base URL of the JSON Ledger API v2.
- `--disclosing-party` (required) — comma-separated parties as which the
  service reads the ledger. The token must grant readAs for each, and one
  participant must host them all. One service replaces one service per party.
- `--access-token-file` — bearer token for the upstream; re-read on every
  request, so an operator rotates it without a restart.
- `--allow-list` — JSON array of package-qualified names, overriding the
  built-in list.
- `--max-contracts-per-template`, `--max-upstream-concurrency`, `--cache-ttl`
  — load bounds: per-template result cap, in-flight upstream query cap, and
  a short read cache.
- `--listen` — bind address; defaults to loopback.

## Build and run

1. Run `cd canton/disclosure/service && go build -o disclosure-service .`.
2. Run `./disclosure-service --json-api <url> --disclosing-party <p1,p2,...>
   [--access-token-file <jwt-file>]`.

Container: `docker build -t disclosure-service canton/disclosure/service`,
then run it with `--listen 0.0.0.0:7599` and the token file mounted
read-only; the loopback default is unreachable through a published port.

## Deployment posture

By design, the service does not authenticate requests. Bind it to loopback,
or place it behind the deployment layer's network boundary (a tunnel, a
mesh, or a TLS-terminating proxy) with access control at that boundary.
`--access-token-file` holds its only upstream credential.
