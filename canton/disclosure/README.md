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
  other caller in that time gets the last result. The service writes one
  log line for each failed call, not for each 503.

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
`[A-Za-z0-9 :_-]`. This is the Canton party character set. The service
splits the value at the first `::`, as Canton does. The fingerprint has 64
lowercase hex characters. The
service rejects a different value with 400 before it reads the ledger. Give each parameter one time only. The service
rejects a repeated parameter and a parameter that the flow does not use. The
query string is 1024 bytes or less.

A flow response has a `disclosures` array and a `missing` array. Each
`disclosures` entry has a role, `templateId`, `contractId`,
`createdEventBlob`, and `synchronizerId`. All entries have the same
synchronizer. The `missing` array names the set members that no reader party
can see, for example a proposal or a preapproval. The client supplies them.

The service does not serve a template outside a flow set. The service serves
a `Coin` or an `Amulet` only as the custody pot that a `LockedLedger` names.
It reads the pot by its contract id.

### Canton Coin deployments

A lock/unlock deployment of Canton Coin (Amulet) commits the Amulet registry
`TransferFactory`. The reader parties cannot see this DSO contract. Thus the
lock/unlock flows name `committed TransferFactory` in `missing`. Get the
factory and its choice context from the transfer-factory API of the
registry. The custody pot of such a deployment is a `Splice.Amulet:Amulet`.

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
  `--access-token-file` or the `--oauth-*` flags, use `https`. Plain `http`
  works only to a loopback IP address.
- `--guardian-governance-party`, `--ntt-custody-party` (required),
  `--operator-party` — the reader parties. The token must grant `readAs` for
  each. One participant must host all of them.
- `--access-token-file` — bearer token for the upstream, 16 KiB or less. The
  service reads the file again for each request. Thus you can replace the
  token without a restart.
- `--oauth-token-url`, `--oauth-client-id`, `--oauth-client-secret-file`,
  `--oauth-audience`, `--oauth-scope`, `--oauth-ca-file` — OAuth2
  client-credentials grant for the upstream token. See "Deployment on a
  separate gateway".
- `--json-api-ca-file`, `--json-api-client-cert-file`,
  `--json-api-client-key-file` — TLS to the upstream: a private CA and a
  client certificate for mutual TLS. See "Deployment on a separate gateway".
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

The container runs as UID 65532, the distroless `nonroot` user. Set the owner
of the token file, the client secret file, and the client key file to UID
65532. Set the mode of each file to 0400.

## Deployment on a separate gateway

The service can run on a gateway host. The gateway host is not the guardian
node. Only the gateway host has a public IP address.

```
relayer
  -> edge proxy: TLS, rate limit, WAF (for example Cloudflare)
  -> gateway host: disclosure-service
  -> private link or mutual TLS
  -> participant JSON Ledger API: admits the gateway source address only
```

The guardian node needs no public port. A Canton validator connects out to
the sequencers. A validator operator gives the sponsor its egress IP address
for the allowlist. The relayers use the gateway only.

### Ledger user of the gateway

Create one ledger user for the gateway on the participant. Give it these
rights only:

- `CanReadAs` for `gg`.
- `CanReadAs` for `nttCustody`.
- `CanReadAs` for the operator, only to serve the `register` flow.

Do not give this user `CanActAs` for a party. Do not give it an admin right.
The service reads the ledger only. It calls these endpoints:

- `GET /v2/state/ledger-end`.
- `POST /v2/state/active-contracts`.
- `POST /v2/events/events-by-contract-id`, for the custody pot and the
  committed factory.

The service reads `ReplayRootRegistry` as the operator. It reads each other
template as `gg` or `nttCustody`. The section "Reader parties" gives the
table.

### Edge

The flow routes are public. Clients do not authenticate. The routes serve
only the contracts and data that a flow needs.

Put an edge proxy in front of the service. The edge proxy ends TLS, applies
a rate limit for each client, and applies a WAF. You choose the edge.
`CLOUDFLARE.md` gives one example setup.

Set `--behind-edge-proxy` only when the service listens on an address that
is not loopback. A tunnel agent on the gateway host, such as `cloudflared`,
connects to the loopback address. Then keep the loopback default and do not
set the flag.

### Flags for the gateway

| Flag | Value on the gateway |
|---|---|
| `--listen` | The address that the edge connects to. With a local tunnel, keep the loopback default. |
| `--behind-edge-proxy` | Set it only with a `--listen` address that is not loopback. |
| `--json-api` | The `https://` URL of the participant JSON Ledger API. |
| `--json-api-ca-file` | The PEM CA bundle of the participant server certificate. It replaces the system roots. |
| `--json-api-client-cert-file` | The PEM client certificate for mutual TLS. |
| `--json-api-client-key-file` | The PEM private key of that certificate. |
| `--oauth-token-url` | The token endpoint of the identity provider. It must use `https`. Plain `http` works only to a loopback IP address, for a test. |
| `--oauth-ca-file` | Optional. The PEM CA bundle of the token endpoint certificate. It replaces the system roots. |
| `--oauth-client-id` | The client ID of the gateway ledger user. |
| `--oauth-client-secret-file` | The file that holds the client secret. |
| `--oauth-audience` | The audience of the participant Ledger API. |
| `--oauth-scope` | Optional. The scope that the identity provider needs. |

Rules for these flags:

- The `--json-api-*` TLS flags need an `https://` value for `--json-api`.
- `--oauth-ca-file` needs an `https://` value for `--oauth-token-url`.
- Give the client certificate and the client key together.
- The minimum TLS version is 1.2.
- The token request uses the system roots, or the `--oauth-ca-file` bundle.
  The client certificate goes to the participant only.
- The token URL must not contain user information, a query, or a fragment.
- Use the `--oauth-*` flags or `--access-token-file`. Do not use both.
- With no auth flag, the service sends no token. Use this for a local
  development participant only.

The service reads each file at startup. A missing or bad file stops the
service. In OAuth2 mode, the service also gets one token at startup. A
token error stops the service.

The service reads the client secret file again for each token request. Thus
you can replace the secret with no restart. The service reads the client
certificate and key one time, at startup. After you replace them, restart
the service.

The token response must contain `expires_in`. An identity provider that does
not send it fails each token request.

The service keeps the OAuth2 token in memory. It gets a new token before the
old token expires. The margin is 60 seconds or 10% of the token lifetime,
whichever is shorter. One token request takes 10 seconds or less. After a
failed token request, the service waits before the next one. The wait starts
at 1 second and doubles after each failure, to 30 seconds or less.

If the participant returns HTTP 401, the service discards the token. Then it
gets a new token and sends the request one more time.

If the token endpoint fails, the flow request fails with HTTP 502.
`/v1/healthz` still returns 200. `/v1/readyz` returns 503. The service sends
each upstream request with a token.

Example:

```
./disclosure-service \
  --json-api https://participant.internal:7575 \
  --guardian-governance-party '<gg>' \
  --ntt-custody-party '<nttCustody>' \
  --json-api-ca-file /etc/disclosure/participant-ca.pem \
  --json-api-client-cert-file /etc/disclosure/gateway.crt \
  --json-api-client-key-file /etc/disclosure/gateway.key \
  --oauth-token-url https://idp.example/oauth/token \
  --oauth-client-id disclosure-gateway \
  --oauth-client-secret-file /etc/disclosure/client-secret \
  --oauth-audience https://ledger_api.example
```

### What a gateway compromise exposes

An attacker who controls the gateway host gets its credentials. These
credentials give read access as `gg`, `nttCustody`, and the operator, if
you set `--operator-party`. Thus the attacker can read the bridge state that
these parties see. As the operator, the attacker can read each contract
that the operator sees, not only `ReplayRootRegistry`. The attacker can also
change or stop the responses of the gateway. The ledger user has no
`CanActAs` right, so its credentials cannot submit a command.

## Security: authority and the vetting process

This section applies to the token registries that NTT and the core bridge
call. It explains which party authority the registry code gets.

### Registry code runs with the authority of the sender

The CIP-0056 token standard sets these controllers:

- `TransferFactory_Transfer` has the controller `transfer.sender`.
- `Allocation_ExecuteTransfer` has the controllers executor, sender, and
  receiver.

A choice body runs with the authority of its controllers and of the
contract signatories. Thus a registry transfer runs with the authority of the
sender. When `gg` or `nttCustody` is
the sender or the receiver of a registry choice, the registry code runs with
the authority of that party.

### `nttCustody` owns the lock/unlock reserves

`gg` never owns a lock/unlock reserve. In this section, `Manager.daml` is
`canton/ntt/daml/Wormhole/Ntt/Manager.daml`. `reserveCustodianFor`
(`Manager.daml:352`) gives the reserve owner. It is `nttCustody` when `gg`
is the admin. It is the admin in all other cases. The `NttManager`
precondition (`Manager.daml:489-494`) rejects `gg` as `reserveCustodian`.

These custody transfers send with `sender = reserveCustodian`:

- The pot merge in `Transfer` (`Manager.daml:930`).
- `Release` (`Manager.daml:1042`).
- `ConsolidateCustody` (`Manager.daml:1094`).
- `TransferAdmin` (`Manager.daml:659`).

A lock in `Transfer` sends from the user to `reserveCustodian`
(`Manager.daml:911-912`).

A registry that the guardians vetted thus runs with the authority of
`nttCustody`. It can move each holding of `nttCustody`. This includes the
reserves of other quorum-held deployments. The same registry cannot do these
things:

- Publish a message from an NTT transceiver `Emitter`. Its owner is `gg`.
- Mint a `gg`-administered instrument through the canonical `CoinFactory`.
  `BurnMintFactory_BurnMint` needs `gg` as the factory admin
  (`Manager.daml:961-962`).
- Change a manager. `SetPeer` (`Manager.daml:503`), `SetPaused`
  (`Manager.daml:557`), `SetFactory` (`Manager.daml:578`), and
  `TransferAdmin` (`Manager.daml:621`) need the admin, which is `gg`.

Thus the quorum-held lock/unlock reserves are one risk pool. Accept quorum
custody only for a registry that you examined.

### Burn/mint uses the canonical `CoinFactory`

A burn/mint deployment uses the canonical `CoinFactory` of the
`token-cip0056` package. `requireCanonicalFactory` (`Manager.daml:164-173`)
checks this. Its admin must be `gg`. The check uses the package name, not the package ID. Thus it
accepts a `CoinFactory` of each vetted `token-cip0056` version. An interface
choice runs the code of the vetted version that the submitter prefers.

Thus, on each participant that hosts `gg`, the vetting of a new
`token-cip0056` version gives mint authority to that code. Vet a new version
only after a review of its `CoinFactory` and `Coin` code.

### The core fee path runs fee registry code as `feeCustody`

`chargeFee` in `Wormhole.Core.Fees` executes the payer allocation with
`Allocation_ExecuteTransfer`. The receiver is the fee recipient. Core 0.8.1
(`wormholelabs-xyz/wormhole` PR #81) makes the fee recipient the
`feeCustody` party, not `gg`. In production, the fee instrument is Canton
Coin (Amulet). Thus the Amulet registry code runs with the authority of
`feeCustody` and of the payer.

### Package vetting is a participant decision

Each participant vets packages for itself. The participant operator vets a
package with the participant admin API. A DAR upload vets its packages by
default. The `gg` governance ceremony does not control package vetting.

A transaction runs only on packages that each informee participant vetted.
Each guardian participant hosts `gg`. Thus each guardian operator
controls which code runs with the authority of `gg`. All guardians together
are the gate.

Each guardian operator must do these steps:

1. Disable automatic vetting for each registry package that `gg` or
   `nttCustody` uses.
2. Vet a new version of such a package only after a review of its code.
3. Find out if the Splice validator automation vets new Amulet packages
   automatically. Check this on your validator.

### The gateway user must not act as a quorum party

Never give the gateway ledger user `CanActAs` for `gg` or for `nttCustody`.
The gateway is on a public host. Keep its credentials to read access only.
Then a gateway compromise gives no more than a read of bridge state.

## CI vulnerability scan

The CI job `disclosure service (go)` runs `govulncheck` v1.9.0. The job
compares the module checksum of `golang.org/x/vuln@v1.9.0` with a fixed value
in the workflow. The `go.mod` file in that module sets the version of each
dependency. `sum.golang.org` checks each download. To change the version,
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
