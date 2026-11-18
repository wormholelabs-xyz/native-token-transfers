# canton/disclosure

This directory holds the `service/` Go module: the production disclosure
service. On Canton, a submitter cannot fetch a contract that it is not a
stakeholder of. The submitter must attach the contract as a disclosure. This
service serves those disclosures over HTTP, one flow set at a time. Each
flow set comes from its choice body in
`canton/ntt/daml/Wormhole/Ntt/Manager.daml`.

Mainnet base URL: `https://canton-disclosure.labsapis.com`.

## Deployment

The flow routes are public, as the Splice Scan routes are. Clients do not
authenticate. Each response holds only the contracts and data that a flow
needs.

Put an edge proxy in front of the service. You choose the edge. Cloudflare is
one example. The edge proxy does these tasks:

- It ends the TLS connection from each client.
- It applies a rate limit for each client.
- It applies a WAF.

Do these steps:

1. Keep the loopback default of `--listen` when the edge proxy is on the same
   host.
2. Configure TLS, a rate limit, and a WAF at the edge proxy.
3. Set `--behind-edge-proxy` only after you complete step 2.

The service does not start with a `--listen` address that is not a loopback
IP address, unless you set `--behind-edge-proxy`. The name `localhost` is not
a loopback IP address for this check.

## Endpoints

- `GET /v1` — this endpoint list, as JSON.
- `GET /v1/healthz` — liveness. It returns 200 and `{"status":"ok"}`. It does
  not call the upstream.
- `GET /v1/readyz` — readiness. It returns 200 and `{"status":"ready"}` when
  the upstream answers. Else it returns 503.
- `GET /v1/managers` — the visible NTT deployments: `managerAddress`,
  `contractId`, `chainId`, `managerId`, `tokenConfig`, `instrumentId`,
  `reserveCustodian`, `paused`. Use `managerAddress` as the `manager`
  parameter of a flow.
- `GET /v1/flows/{flow}` — the disclosure set of one flow. A role labels each
  contract. The parameters of each flow are:

  | Flow | Parameters |
  |---|---|
  | `release` | `manager`, `digest`, `recipient` (optional) |
  | `mint` | `manager`, `digest`, `recipient` |
  | `set-peer` | `manager`, `digest` |
  | `accept-admin` | `manager`, `digest` |
  | `transfer` | `manager` |
  | `register` | `gg` (optional), `by-vaa` (bool, optional) |
  | `consolidate` | `manager` |

`manager` is the manager address and `digest` is the VAA hash. Each is a hex
string of 64 characters. Give each parameter one time only. The service
rejects a repeated parameter and a parameter that the flow does not use. The
query string is 1024 bytes or less.

A flow response has a `disclosures` array and a `missing` array. Each
`disclosures` entry has a role, `templateId`, `contractId`,
`createdEventBlob`, and `synchronizerId`. All entries have the same
synchronizer. The `missing` array names the set members that no reader party
can see, for example a proposal or a preapproval. The client supplies them.

The service does not serve a template outside a flow set. The service serves
a `Coin` only as the custody pot that a `LockedLedger` names.

## Client checks

Do these checks before you submit a command with a disclosure set:

1. Pin the `ntt` package id that you vetted.
2. Reject each `templateId` that has a different package id.
3. Get the `NttManager` contract id from a second source.
4. Compare that contract id with the `NttManager` entry of the set.
5. Attach each `disclosures` entry to the command as a `DisclosedContract`.
6. Use the `contractId` of each role as the choice argument.

The `ntt` package id changes with each core release. Read the id from the
release table in [`canton/dars/README.md`](../dars/README.md). Do not copy an
id from this file.

## Examples

```
# The disclosure set of one release.
curl 'https://canton-disclosure.labsapis.com/v1/flows/release?manager=<hex64>&digest=<hex64>'

# The visible deployments.
curl 'https://canton-disclosure.labsapis.com/v1/managers'
```

## Errors

A 4xx response has a JSON body with an `error` message about the request.
A 5xx response has a fixed `error` message and a `requestId`. The service
log has the detail for that `requestId`. The response does not contain
upstream text.

## Reader parties

The service reads each template as one role only:

| Role flag | Party | Templates |
|---|---|---|
| `--guardian-governance-party` | `gg` (`guardianGovernance`) | `NttManager`, `LockedLedger`, `CoreState`, `Emitter`, `ReplayNode`, `NttGovernance`, `EmitterRegistry`, `CoinFactory`, `DepositPreapproval`, `AdminTransferProposal`, `TransferPreapproval` |
| `--ntt-custody-party` | `nttCustody` | The custody pots, and `TransferPreapproval` |
| `--operator-party` | operator | `ReplayRootRegistry` (`register` flow only) |

The three parties must be different. Without `--operator-party`, the
`register` flow returns 404. The `gg` parameter of `register` must be the
`--guardian-governance-party` value.

The `reserveCustodian` of a deployment owns its lock/unlock reserve. It is
the admin, or `nttCustody` when the admin is `gg`. The service cannot see
the reserve of an admin that is a third party. The response then names that
contract in `missing`.

## Bounds

| Bound | Value |
|---|---|
| Inbound requests at one time | 64; the next request gets 503 |
| Time for one request | 60 s |
| Bytes of blobs and ids in one flow response | 4 MiB |
| Query string of a flow | 1024 bytes |
| Matching contracts for one selector | 8 |
| Contracts scanned in one upstream response | 100 000 |
| Bytes of one upstream response | 64 MiB |

## Flags

- `--json-api` (required) — base URL of the JSON Ledger API v2. With
  `--access-token-file`, use `https`. Plain `http` works only to a loopback
  IP address.
- `--guardian-governance-party`, `--ntt-custody-party` (required),
  `--operator-party` — the reader parties. The token must grant `readAs` for
  each. One participant must host all of them.
- `--access-token-file` — bearer token for the upstream, 16 KiB or less. The
  service reads the file again for each request. Thus you can replace the
  token without a restart.
- `--behind-edge-proxy` — set it only when an edge proxy with TLS, a rate
  limit, and a WAF is in front. A `--listen` address that is not loopback
  needs it.
- `--max-contracts-per-template` (1 to 10 000), `--max-upstream-concurrency`
  (1 to 64), `--cache-ttl` (0 to 60 s) — load bounds. They are the
  `/v1/managers` cap, the upstream query cap, and a short read cache.
  `--cache-ttl 0` turns the cache off.
- `--listen` — bind address. The default is `127.0.0.1:7599`.

## Build and run

1. Run `cd canton/disclosure/service && go build -o disclosure-service .`.
2. Run `./disclosure-service --json-api <url>
   --guardian-governance-party <gg-party> --ntt-custody-party
   <ntt-custody-party> [--access-token-file <jwt-file>]`.
3. To serve the `register` flow, add `--operator-party <operator-party>`.

Container: run `docker build -t disclosure-service canton/disclosure/service`.
Then run the container with `--listen 0.0.0.0:7599
--behind-edge-proxy`, behind the edge proxy. Mount the
token file read-only. The loopback default is not reachable through a
published port.
