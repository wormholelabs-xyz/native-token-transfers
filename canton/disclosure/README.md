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

The service is not ready for public mainnet traffic. Public mainnet traffic
waits for one of these changes:

- The index of plan item N73-07.
- Pages from `/v2/state/active-contracts-page` in place of one full list.

The reason is the participant list limit. See [Bounds](#bounds).

## Endpoints

- `GET /v1` — this endpoint list, as JSON.
- `GET /v1/healthz` — liveness. It returns 200 and `{"status":"ok"}` while
  the process serves HTTP. It does not call the upstream.
- `GET /v1/readyz` — readiness. It returns 200 and `{"status":"ready"}` when
  the last upstream ledger-end call succeeded. Else it returns 503. The
  service makes one such call in 2 s or more, with a timeout of 3 s. Each
  other caller in that time gets the last result.

The inbound cap of 64 requests does not apply to `/v1`, `/v1/healthz`, and
`/v1/readyz`. Thus an orchestrator gets a probe result when flow requests
fill the cap.
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
string of 64 characters. `recipient` and `gg` are party ids with the format
`<hint>::1220<fingerprint>`. The hint has 1 to 185 characters from
`[A-Za-z0-9._-]`. The fingerprint has 64 lowercase hex characters. The
service rejects a different value with 400 before it reads the ledger. Give each parameter one time only. The service
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

The service log quotes each value from a client, for example the path.
Thus a client value cannot start a new log line.

When the participant returns 413 for a template list, the flow returns 502
with the fixed message `upstream ledger list limit reached`. The log line
names the template and `http-list-max-elements-limit`. It also holds the
start of the participant response body.

Each response has `Cache-Control: no-store` and
`X-Content-Type-Options: nosniff`.

## Cache

The service keeps each upstream answer for `--cache-ttl`. This includes a
failed read. Thus a template above a bound costs one upstream read in each
cache window. A local `service busy` result is not kept.

The cache key holds the parameter values of the request: `manager`,
`digest`, `recipient`, and `gg`. This has a cost:

- Each new value is a cache miss.
- Each miss reads the full list of one template from the participant.
- Thus one anonymous request of about 200 bytes can cause one full template
  read.
- The upstream cap of 8 protects the participant. Other callers can then
  wait 2 s and get 503.
- The cache holds 512 entries. Many new values push out the useful entries.

The index of plan item N73-07 removes this cost.

## Data that an anonymous caller can get

The routes are public. The owner accepts this exposure by design:

- A `mint` request tells if a known party has a `DepositPreapproval` for a
  deployment. The service returns 404 when it has none. A `release` request
  with `recipient` tells the same for a `TransferPreapproval`. The caller
  must know the party id.
- `/v1/managers` gives the `instrumentId.admin` and `reserveCustodian` party
  ids of each deployment. The probes do not give party ids.
- Each `NttManager` blob gives the admin, `gg`, `nttCustody`, the peers, the
  limits, and the pause state.
- Each `ReplayNode` of a namespace gives the consumed VAA digests.
- The custody pot of a deployment gives the locked amount.
- A pending `AdminTransferProposal` to `gg` is visible.

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
| Contracts in one upstream template list | The participant `http-list-max-elements-limit`. The Canton documentation gives 1024 as the default. Above it, the participant returns 413, and each flow that reads the template returns 502. |
| Inbound requests at one time | 64, without `/v1`, `/v1/healthz`, and `/v1/readyz`; the next request gets 503 |
| Upstream readiness calls | 1 in 2 s, with a 3 s timeout |
| Time for one request | 60 s |
| Bytes of blobs and ids in one flow response | 4 MiB |
| Query string of a flow | 1024 bytes |
| Matching contracts for one selector | 8 |
| Contracts scanned in one upstream response | 100 000 |
| Bytes of one upstream response | 64 MiB |

A stranger can make the first bound fail. For example, a stranger can create
1025 `DepositPreapproval` contracts with `instrumentId.admin = gg`. `gg` is an
observer of each. Each `mint` flow then returns 502. `gg` cannot archive these
contracts. The index of plan item N73-07 removes this failure.

The service can hold 8 upstream responses at one time, each up to 64 MiB.
Set the memory limit of the container to more than 512 MiB.

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

## CI vulnerability scan

The CI job `disclosure service (go)` runs `govulncheck` v1.9.0. The job
compares the module checksum of `golang.org/x/vuln@v1.9.0` with a fixed value
in the workflow. `sum.golang.org` also checks the download. The `go.sum` file
in that module pins the dependencies of `govulncheck`. To change the version,
change the module and the checksum in the workflow together.

## Open items

- The index of plan item N73-07. Until it is in place, the participant list
  limit stops a flow (see [Bounds](#bounds)). This blocks public mainnet
  traffic.
- Live checks that did not run:
  - The list limit and its status code on a participant (plan item N73-28).
  - The `events-by-contract-id` response shape on a live participant.
  - `synchronizerId` after a reassignment. The service reads the
    synchronizer of the create. After a reassignment, a correct set can
    have two synchronizers, and the flow returns 500.
- Lock/unlock flows return 500 when `gg` cannot see the `CoinFactory`. PR
  #76 puts that factory in `missing`.
