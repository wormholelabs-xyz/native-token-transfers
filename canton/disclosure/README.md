# canton/disclosure

This directory holds the `service/` Go module: the production disclosure
service. On Canton, a submitter cannot fetch a contract that it is not a
stakeholder of. The submitter must attach the contract as a disclosure. This
service serves those disclosures over HTTP, one flow set at a time. Each
flow set comes from its choice body in
`canton/ntt/daml/Wormhole/Ntt/Manager.daml`.

Mainnet base URL: `https://canton-disclosure.labsapis.com`.

## Deployment

The flow routes and the CIP-0056 registry routes are public, as the Splice
Scan routes are. Clients do not authenticate. CIP-0056 requires registry
routes without authentication. Each response holds only the contracts and
data that a flow or a registry choice needs.

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

The reason is the participant list limit. See [Bounds](#bounds). This
applies to the flow routes and to the CIP-0056 registry routes.

## Endpoints

- `GET /v1` — the list of the endpoints that the service serves, as JSON.
  The list has the CIP-0056 registry routes only when the registry API is on.
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

  | Flow           | Parameters                                  |
  | -------------- | ------------------------------------------- |
  | `release`      | `manager`, `digest`, `recipient` (optional) |
  | `mint`         | `manager`, `digest`, `recipient`            |
  | `set-peer`     | `manager`, `digest`                         |
  | `accept-admin` | `manager`, `digest`                         |
  | `transfer`     | `manager`                                   |
  | `register`     | `gg` (optional), `by-vaa` (bool, optional)  |
  | `consolidate`  | `manager`                                   |

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

The service keeps a read by contract ID that finds no contract for 1 s or
less. These entries use 128 of the 512 entries or fewer. When 128 are in
use, the service does not keep the next one.

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

| Role flag                     | Party                       | Templates                                                                                                                                                                                                         |
| ----------------------------- | --------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `--guardian-governance-party` | `gg` (`guardianGovernance`) | `NttManager`, `LockedLedger`, `CoreState`, `Emitter`, `ReplayNode`, `NttGovernance`, `EmitterRegistry`, `ReplayRootRegistry`, `CoinFactory`, `DepositPreapproval`, `AdminTransferProposal`, `TransferPreapproval` |
| `--ntt-custody-party`         | `nttCustody`                | The custody pots, and `TransferPreapproval`                                                                                                                                                                       |

The registry API reads `NttManager`, `CoinFactory`, `Coin`,
`TransferPreapproval`, `CoinTransferInstruction` and `CoinAllocation` as `gg`
only. `gg` is a stakeholder of each of these templates.

The two parties must be different. `gg` signs `ReplayRootRegistry` (core
0.9.0), so the service serves the `register` flow with the `gg` reader. The
`gg` parameter of `register` must be the `--guardian-governance-party` value.

The `reserveCustodian` of a deployment owns its lock/unlock reserve. It is
the admin, or `nttCustody` when the admin is `gg`. The service cannot see
the reserve of an admin that is a third party. The response then names that
contract in `missing`.

## Bounds

| Bound                                                         | Value                                                                                                                                                                                                                                                                                                              |
| ------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| Contracts in one upstream template list                       | The participant `http-list-max-elements-limit`. The Canton documentation gives 1024 as the default. Above it, the participant returns 413, and each flow that reads the template returns 502. This is the first bound for the registry reads of `Coin` and `TransferPreapproval`. See "Registry reads at a bound". |
| Time to read the headers and the body of one request          | 10 s                                                                                                                                                                                                                                                                                                               |
| Inbound requests at one time                                  | 64, without `/v1`, `/v1/healthz`, and `/v1/readyz`; the next request gets 503                                                                                                                                                                                                                                      |
| Upstream readiness calls                                      | 1 in 2 s, with a 3 s timeout                                                                                                                                                                                                                                                                                       |
| Time for one request                                          | 60 s                                                                                                                                                                                                                                                                                                               |
| Bytes of blobs and ids in one flow response or choice context | 4 MiB                                                                                                                                                                                                                                                                                                              |
| Query string of a flow or a registry route                    | 1024 bytes                                                                                                                                                                                                                                                                                                         |
| Request body of a registry route                              | 64 KiB                                                                                                                                                                                                                                                                                                             |
| Matching contracts for one selector                           | 8                                                                                                                                                                                                                                                                                                                  |
| Instruments in one registry page                              | 100                                                                                                                                                                                                                                                                                                                |
| Instruments with a `Coin` total                               | 1000                                                                                                                                                                                                                                                                                                               |
| Contracts scanned in one upstream response                    | 100 000                                                                                                                                                                                                                                                                                                            |
| Bytes of one upstream response                                | 64 MiB                                                                                                                                                                                                                                                                                                             |

The list limit is a participant setting. Each deployment sets it on its own
participant. The service cannot read the value. Record the value of each
participant in the deployment notes.

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
- `--guardian-governance-party`, `--ntt-custody-party` (required) — the
  reader parties. The token must grant `readAs` for each. One participant
  must host both of them.
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
  `NttManager` cap of `/v1/managers` and of the registry instrument list, the
  upstream query cap, and a short read cache.
  `--cache-ttl 0` turns the cache off. With `--registry-instruments-file`,
  the minimum is 1 s. Without the cache, each instruments request reads the
  full `Coin` list.
- `--listen` — bind address. The default is `127.0.0.1:7599`.
- `--registry-instruments-file` — serve the CIP-0056 registry API for the
  instruments that `--guardian-governance-party` administers. See "CIP-0056
  registry API".

## Build and run

1. Run `cd canton/disclosure/service && go build -o disclosure-service .`.
2. Run `./disclosure-service --json-api <url>
--guardian-governance-party <gg-party> --ntt-custody-party
<ntt-custody-party> [--access-token-file <jwt-file>]`.

Container: run `docker build -t disclosure-service canton/disclosure/service`.
Then run the container with `--listen 0.0.0.0:7599
--behind-edge-proxy`, behind the edge proxy. Mount the
token file read-only. The loopback default is not reachable through a
published port.

### Secret files

The container runs as UID 65532 and GID 65532, the distroless `nonroot`
user. The service reads three secret files: the token file, the client
secret file, and the client key file. Each secret file must be a regular
file. The service accepts these modes: 0400, 0440, 0600, and 0640. The
service refuses a secret file with the group write or the group execute
permission. The service refuses a secret file with a permission for other
users.

Outside Kubernetes, set the owner of each secret file to UID 65532. Set the
mode of each secret file to 0400.

On Kubernetes, a Secret volume has no setting for the file owner. Thus UID
65532 must read each secret file through the group read permission. Use this
procedure:

1. Set `fsGroup` to 65532 in the pod `securityContext`.
2. Set `defaultMode` to 0440 in the Secret volume.
3. Mount the Secret volume read-only.

The kubelet sets the group of each secret file to 65532. The mode of each
secret file is 0440. A Secret change replaces the files with the same group
and mode.

```yaml
spec:
  securityContext:
    runAsUser: 65532
    runAsGroup: 65532
    runAsNonRoot: true
    fsGroup: 65532
  containers:
    - name: disclosure-service
      image: <disclosure-service image, pinned by digest>
      args: ["--access-token-file", "/etc/disclosure/token"]
      volumeMounts:
        - name: disclosure-secrets
          mountPath: /etc/disclosure
          readOnly: true
  volumes:
    - name: disclosure-secrets
      secret:
        secretName: disclosure-secrets
        defaultMode: 0440
```

YAML reads `0440` as an octal number. In JSON, write the decimal value 288.

Alternative: an init container copies each secret file to a memory
`emptyDir`. The copy has the owner 65532 and the mode 0400. The init container
runs as UID 0. Thus set `runAsNonRoot` on the service container only. The init
container copies the files one time, at pod start. Restart the pod after a
Secret change.

```yaml
spec:
  initContainers:
    - name: copy-secrets
      image: <shell image with install, pinned by digest>
      securityContext:
        runAsUser: 0
      command:
        [
          "sh",
          "-c",
          "install -o 65532 -g 65532 -m 0400 /secrets/* /etc/disclosure/",
        ]
      volumeMounts:
        - name: disclosure-secrets-source
          mountPath: /secrets
          readOnly: true
        - name: disclosure-secrets
          mountPath: /etc/disclosure
  containers:
    - name: disclosure-service
      image: <disclosure-service image, pinned by digest>
      securityContext:
        runAsUser: 65532
        runAsGroup: 65532
        runAsNonRoot: true
      args: ["--access-token-file", "/etc/disclosure/token"]
      volumeMounts:
        - name: disclosure-secrets
          mountPath: /etc/disclosure
          readOnly: true
  volumes:
    - name: disclosure-secrets-source
      secret:
        secretName: disclosure-secrets
    - name: disclosure-secrets
      emptyDir:
        medium: Memory
```

## CIP-0056 registry API

The service can also serve the CIP-0056 off-ledger registry API, version 1.
It serves the API for the burn/mint instruments that `gg` administers.
Canton wallets use this API to show, send and receive the tokens that NTT
mints on Canton. In this section, the Daml files are in
`tokens/cip0056/daml/Token/CIP0056/` of the CantonExamples repository. The
line numbers are for commit `b448589`. The sources in the vendored
`canton/dars/token-cip0056-0.2.0.dar` are the same as the sources at that
commit.

### Endpoints

| Endpoint                                                                                                  | Response                                                                      |
| --------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------- |
| `GET /registry/metadata/v1/info`                                                                          | `adminId` is `gg`. `supportedApis` lists the four v1 APIs below.              |
| `GET /registry/metadata/v1/instruments`                                                                   | The instruments, in instrument ID order. Parameters: `pageSize`, `pageToken`. |
| `GET /registry/metadata/v1/instruments/{instrumentId}`                                                    | One instrument.                                                               |
| `POST /registry/transfer-instruction/v1/transfer-factory`                                                 | The `CoinFactory`, the `transferKind` and the choice context.                 |
| `POST /registry/transfer-instruction/v1/{transferInstructionId}/choice-contexts/{accept,reject,withdraw}` | The choice context of a `CoinTransferInstruction`.                            |
| `POST /registry/allocation-instruction/v1/allocation-factory`                                             | The `CoinFactory` and the choice context.                                     |
| `POST /registry/allocations/v1/{allocationId}/choice-contexts/{execute-transfer,withdraw,cancel}`         | The choice context of a `CoinAllocation`.                                     |

The responses agree with the OpenAPI specifications in `service/openapi/`.
The Go tests validate each response against these files. The registry
admin is the `--guardian-governance-party` value.

### Instruments

The instrument set is the instruments of the `NttManager` contracts with these
properties:

- `tokenConfig` is `BurnMint`.
- `instrumentId.admin` is `gg`.
- `guardianGovernance` is `gg`.

The metadata of an instrument comes from these sources:

| Field             | Source                                                                                                                                                                                                                                                           |
| ----------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `id`              | `NttManager.instrumentId.id`                                                                                                                                                                                                                                     |
| `name`, `symbol`  | The instruments file                                                                                                                                                                                                                                             |
| `decimals`        | `NttManager.tokenDecimals`                                                                                                                                                                                                                                       |
| `totalSupply`     | The sum of the active `Coin` amounts of the instrument, as `gg` reads them. Daml `Decimal` text with 10 fraction digits. The read keeps only the sums, and it requests no `createdEventBlob`. The service omits the field when the `Coin` read stops at a bound. |
| `totalSupplyAsOf` | The service clock at the start of the `Coin` read. A response from the cache keeps the time of its read. The service omits the field together with `totalSupply`.                                                                                                |

`totalSupply` does not include value in transit. A `CoinTransferInstruction`
and a `CoinAllocation` hold their amount directly. The choice that makes them
archives the input `Coin` contracts (`CoinTransfer.daml:51-53`, `143`,
`CoinAllocation.daml:10-11`). Thus `totalSupply` goes down by each pending
amount until the receiver accepts, or the sender gets the value back. A
holder can lower the shown value with an offer of its balance. Wallets must
show `totalSupply` as the supply in `Coin` contracts only.

Two `NttManager` contracts can have the same instrument. Then their
`tokenDecimals` and their factory must be equal, or the request fails with
HTTP 500.

`pageSize` is an integer, with the default 25 and the range 1 to 100. A value
outside the range moves to the nearest limit. `pageToken` is the
`nextPageToken` of the previous page. It is the last instrument ID of that
page in unpadded base64url, 683 characters or less. Thus it needs no
percent-encoding, and the query of a page stays below 1024 bytes. The next
page starts at the first ID after it. Give each parameter one time only. The service rejects
a repeated parameter, an unknown parameter, and a `pageSize` that is not an
integer. The other registry routes accept no query parameter.

### Reads

The registry API uses the same read rules as the flow routes:

- The service checks the query, the path and the body before it calls the
  upstream.
- It reads `NttManager` once for each request, with exact filters on
  `tokenConfig`, `instrumentId.admin` and `guardianGovernance`. The filters
  apply before the `--max-contracts-per-template` cap.
- It reads the receiver `TransferPreapproval` with exact filters on `owner`,
  `instrumentId.admin` and `instrumentId.id`. The cap is 8 matches. Above
  the cap, the transfer is an `offer`.
- It reads the committed `CoinFactory`, a `CoinTransferInstruction` and a
  `CoinAllocation` by contract ID.
- It checks the package name and the template of each contract that it
  reads. A contract ID of a different template or package gets 404.
- All contracts of one choice context are on one synchronizer.
- `GET /registry/metadata/v1/instruments/{instrumentId}` returns 404 for an
  instrument with no entry in the instruments file, before an upstream call.

### Instruments file

`--registry-instruments-file` names a JSON object. Each key is an instrument
ID. Each value has the fields `name` and `symbol`:

```json
{
  "wormhole-ntt:93f7db4b180e39ef5cae293224382ee567ffbf02fca6a62c612d72739e24dcfb": {
    "name": "Wormhole Bridged Test",
    "symbol": "WBT"
  }
}
```

The service reads the file at startup. These conditions stop the service:

- The file is not a JSON object, or it has no entries.
- Two entries have the same instrument ID.
- An entry has the same field two times.
- An entry has a field other than `name` and `symbol`.
- An entry has no `name` or no `symbol`, or one of them is empty.
- The file is larger than 1 MiB.

An instrument with no entry in the file is not in the metadata responses.
The service logs it one time. The transfer and allocation endpoints serve it.

The ledger user of the service needs `readAs` for `gg` only for the registry
API.

### Choice contexts and disclosures

Each context comes from the choice body. A context contains each contract
that the choice fetches and that the controller cannot see.

| Endpoint                                 | Choice body                                                                             | `choiceContextData`                                | `disclosedContracts`                 |
| ---------------------------------------- | --------------------------------------------------------------------------------------- | -------------------------------------------------- | ------------------------------------ |
| `transfer-factory`, `self`               | `CoinFactory.daml:56-77`, `CoinTransfer.daml:159-162`                                   | `{"values": {}}`                                   | `CoinFactory`                        |
| `transfer-factory`, `direct`             | `CoinTransfer.daml:163-167`, `TransferPreapproval_Receive` at `CoinTransfer.daml:39-49` | `token-cip0056.transferPreapprovalCid` (see below) | `CoinFactory`, `TransferPreapproval` |
| `transfer-factory`, `offer`              | `CoinTransfer.daml:168`, `CoinFactory.daml:69-77`                                       | `{"values": {}}`                                   | `CoinFactory`                        |
| `accept`, `reject`, `withdraw`           | `CoinTransfer.daml:77-88`                                                               | `{"values": {}}`                                   | None                                 |
| `allocation-factory`                     | `CoinFactory.daml:87-104`                                                               | `{"values": {}}`                                   | `CoinFactory`                        |
| `execute-transfer`, `withdraw`, `cancel` | `CoinAllocation.daml:22-36`                                                             | `{"values": {}}`                                   | None                                 |

The `CoinFactory` has the signatory `gg` only (`CoinFactory.daml:26`). Thus
the sender cannot see it. The service returns the `CoinFactory` that the
`NttManager` of the instrument commits.

`transferKind` has these values:

- `self`: the sender and the receiver are the same party. `transferCore`
  completes the transfer with the authority of the sender.
- `direct`: the receiver has an active `TransferPreapproval` for the
  instrument. The preapproval has the signatory `owner` and the observer
  `instrumentId.admin` (`CoinTransfer.daml:33-34`). Thus `gg` sees it, and the
  sender does not. With two to eight preapprovals, the service selects the
  lowest contract ID.
- `offer`: all other transfers. The choice creates a `CoinTransferInstruction`.
  The receiver accepts it or rejects it. The service also returns `offer` in
  these conditions, and writes one log line:
  - The receiver has more than eight preapprovals for the instrument.
  - The preapproval is on a different synchronizer than the `CoinFactory`.
  - The `TransferPreapproval` read stops at a bound. See "Registry reads at
    a bound".

An `offer` in place of a `direct` is safe. With no preapproval value,
`lookupTransferPreapprovalCid` returns `None` (`CoinTransfer.daml:20-24`).
Then `transferCore` returns `CorePendingNeeded` (`CoinTransfer.daml:168`),
and the factory creates a `CoinTransferInstruction` for the receiver
(`CoinFactory.daml:69-77`). The receiver gets the value when it accepts.

For `direct`, `choiceContextData` is this value. `lookupTransferPreapprovalCid`
(`CoinTransfer.daml:17-24`) reads it:

```json
{
  "values": {
    "token-cip0056.transferPreapprovalCid": {
      "tag": "AV_ContractId",
      "value": "<preapproval contract ID>"
    }
  }
}
```

The wallet gives `choiceContextData` as `extraArgs.context` of the choice.

The choices on a `CoinTransferInstruction` and a `CoinAllocation` only create
`Coin` contracts. Each controller is a stakeholder of the contract
(`CoinTransfer.daml:63-64`, `CoinAllocation.daml:16-17`). Thus these contexts
are empty. The service makes sure that the contract exists and that `gg`
administers its instrument.

### Errors

Each error response is the `ErrorResponse` of the specification:
`{"error": "<text>"}`. The `X-Request-Id` header has the request ID. A 5xx
`error` is a fixed text that ends with the request ID. The service log has
the detail for that request ID. The response does not contain upstream text.

| Status        | Cause                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                    |
| ------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| 400           | The query has a repeated parameter, an unknown parameter, or a parameter of the wrong type. `pageToken` is not a `nextPageToken` of this service. The body is not a JSON object, or it is larger than 64 KiB. A field has the wrong type, or a required field is missing. The value of `expectedAdmin`, `instrumentId.admin`, `sender`, or `receiver` is not a party id `<hint>::1220<64 lowercase hex>`. `expectedAdmin` is not `instrumentId.admin`. A contract ID is not hex. The amount is not a positive `Decimal`. |
| 404           | The instrument is not in the instrument set. The instrument admin is not `gg`. The contract ID is not an active `CoinTransferInstruction` or `CoinAllocation` of `token-cip0056` for a `gg` instrument. The registry API is off.                                                                                                                                                                                                                                                                                         |
| 414           | The query string is larger than 1024 bytes.                                                                                                                                                                                                                                                                                                                                                                                                                                                                              |
| 500           | The ledger data breaks an invariant, for example two `tokenDecimals` values for one instrument, or a choice context on two synchronizers.                                                                                                                                                                                                                                                                                                                                                                                |
| 502, 503, 504 | The upstream participant failed, a read passed a bound, the service shed the load, or the upstream timed out.                                                                                                                                                                                                                                                                                                                                                                                                            |

A 400 text repeats a client value of 32 bytes or less, in quotes. A body
fault has a fixed text. All causes of a 404 for a contract ID have the same
text. Thus a caller cannot learn if `gg` sees a contract with that ID. The
log has the cause.

When the participant returns 413 for the `NttManager` list, the registry
returns 502 with the fixed text `upstream ledger list limit reached`. The log line
names the template. Each registry response has `Cache-Control: no-store` and
`X-Content-Type-Options: nosniff`.

The service reads only the `JsActiveContract` entries of the active contract
set. A receiver `TransferPreapproval` in a reassignment thus gives an
`offer`.

The specification declares 200, 404 and 500 for the metadata routes. It
declares 200, 400 and 404 for the other routes. The other statuses in the
table above are not in the specification.

### Registry reads at a bound

The registry reads the full `Coin` list and the full `TransferPreapproval`
list of `gg`. Each read can stop at one of these bounds:

- The participant list limit. The participant returns 413.
- 100 000 contracts scanned in one response.
- 64 MiB in one response.
- 1000 instruments with a `Coin` total (the `Coin` read only).
- 8 matching preapprovals (the `TransferPreapproval` read only).

When the `Coin` read stops at a bound, the metadata routes return 200. The
service omits `totalSupply` and `totalSupplyAsOf`. Both fields are optional
in the specification. When the `TransferPreapproval` read stops at a bound,
`transfer-factory` returns 200 with `offer` and the `CoinFactory` only. In
each case, the service writes one log line for each request.

The cache keeps the failed read for `--cache-ttl`. Thus a repeat request in
the same cache window does not read the participant again.

These results occur in normal use. For example, NTT mints to more than 1024
holders, or a stranger makes 1025 preapprovals with `instrumentId.admin =
gg`. Any other upstream fault gives 502.

### Debug fields

With `excludeDebugFields` false (the default), each disclosed contract also
has these fields:

- `debugPackageName`: the `packageName` of the created event.
- `debugPayload`: the create argument.
- `debugCreatedAt`: the `createdAt` of the created event.

With `excludeDebugFields` true, the service omits them.

### Discovery

CIP-0056 has this discovery model:

- Each node of a decentralized registry serves the API under its own URL
  prefix.
- A wallet reads the CNS entry of the admin party. The metadata key
  `splice.lfdecentralizedtrust.org/registryUrls` has the URL prefixes,
  separated by commas.

The Canton documentation states that this discovery is not implemented yet.
Until then, each wallet keeps its own map from the admin party to the
registry URL. A CNS entry for `gg` needs a later `gg` ceremony.

### Trust in a registry node

Each guardian gateway can serve the API, because each one reads as `gg`. A
wallet can use the response of any node. A bad response cannot change what a
transaction does. It can only make the transaction fail, or change a direct
transfer into an offer:

- The participant checks each `createdEventBlob` against its contract ID.
  A node cannot change the contents of a disclosed contract.
- The `CoinFactory` code checks `expectedAdmin`
  (`CoinFactory.daml:57-58`) and the instrument admin
  (`CoinTransfer.daml:155-156`).
- With no preapproval value, or a value with a different tag,
  `lookupTransferPreapprovalCid` returns `None`. The transfer then becomes an
  offer (`CoinTransfer.daml:20-24`, `168`).
- A preapproval ID of an archived contract, or of a different template, makes
  the exercise fail.
- A preapproval of a different owner or a different instrument fails the
  asserts of `TransferPreapproval_Receive` (`CoinTransfer.daml:46-48`).

WARNING: The interface choice runs the code of the factory contract. A node
can return a different contract as the factory. The wallet must make sure
that `templateId` of the factory is the `CoinFactory` of `token-cip0056`. The
participant of the wallet runs only the packages that it vetted.

### Specification files

`service/openapi/` holds the four v1 specifications, with no changes. The
source is `https://docs.canton.network/openapi/splice/token-standard/`,
downloaded on 2026-11-18.

| File                             | Spec version | SHA-256                                                            |
| -------------------------------- | ------------ | ------------------------------------------------------------------ |
| `token-metadata-v1.yaml`         | 1.2.0        | `43ca08f0706b433ca20cee1a355da6d4ad00ea8c089b8982fb446627b7a2e135` |
| `transfer-instruction-v1.yaml`   | 1.1.0        | `6e8d4abbb912cd2fa890b7dfcdc940a9ceadd60467049921a1e1151b5363c46f` |
| `allocation-instruction-v1.yaml` | 1.0.0        | `c82cf1b7d661666f676a05fcd4e130562df03f701eca755fbc50d364822ec1c4` |
| `allocation-v1.yaml`             | 1.1.0        | `b28762a1d8f94a300904f20ed9c3f42babad4120b112593a17167f544f24fdd3` |

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

Do not give this user `CanActAs` for a party. Do not give it an admin right.
The service reads the ledger only. It calls these endpoints:

- `GET /v2/state/ledger-end`.
- `POST /v2/state/active-contracts`.
- `POST /v2/events/events-by-contract-id`, for the custody pot and the
  committed factory.

The service reads each template as `gg` or `nttCustody`. The section
"Reader parties" gives the table.

### Edge

The flow routes and the registry routes are public. Clients do not
authenticate. The routes serve only the contracts and data that a flow or a
registry choice needs.

Put an edge proxy in front of the service. The edge proxy ends TLS, applies
a rate limit for each client, and applies a WAF. You choose the edge.
`CLOUDFLARE.md` gives one example setup.

Set `--behind-edge-proxy` only when the service listens on an address that
is not loopback. A tunnel agent on the gateway host, such as `cloudflared`,
connects to the loopback address. Then keep the loopback default and do not
set the flag.

### Flags for the gateway

| Flag                          | Value on the gateway                                                                                                            |
| ----------------------------- | ------------------------------------------------------------------------------------------------------------------------------- |
| `--listen`                    | The address that the edge connects to. With a local tunnel, keep the loopback default.                                          |
| `--behind-edge-proxy`         | Set it only with a `--listen` address that is not loopback.                                                                     |
| `--json-api`                  | The `https://` URL of the participant JSON Ledger API.                                                                          |
| `--json-api-ca-file`          | The PEM CA bundle of the participant server certificate. It replaces the system roots.                                          |
| `--json-api-client-cert-file` | The PEM client certificate for mutual TLS.                                                                                      |
| `--json-api-client-key-file`  | The PEM private key of that certificate.                                                                                        |
| `--oauth-token-url`           | The token endpoint of the identity provider. It must use `https`. Plain `http` works only to a loopback IP address, for a test. |
| `--oauth-ca-file`             | Optional. The PEM CA bundle of the token endpoint certificate. It replaces the system roots.                                    |
| `--oauth-client-id`           | The client ID of the gateway ledger user.                                                                                       |
| `--oauth-client-secret-file`  | The file that holds the client secret.                                                                                          |
| `--oauth-audience`            | The audience of the participant Ledger API.                                                                                     |
| `--oauth-scope`               | Optional. The scope that the identity provider needs.                                                                           |

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
at the time of the failure. It is 1 second and doubles after each failure, to
30 seconds or less. If a refresh fails, the service uses the old token until
half the margin before its expiry. It logs each failed refresh.

If the participant returns HTTP 401, the service discards the token. Then it
gets a new token and sends the request one more time. If the participant
also returns HTTP 401 for the new token, the service counts a failed token
request and waits before the next token request, as for a token endpoint
failure.

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
credentials give read access as `gg` and `nttCustody`. Thus the attacker can
read the bridge state that these parties see. The attacker can also
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
4. Vet no package with code that runs with `gg` authority and can claim a
   `gg` replay root. NTT reserves the replay namespace prefix `ntt:`.

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
