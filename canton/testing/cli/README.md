# ntt-playground

A devnet playground CLI for the Canton NTT contracts in `canton/`. It stands up
a local Canton network, deploys an NTT (manager + transceiver + token seam),
controls a 1/1 Wormhole guardian that signs VAAs on demand, and drives
inbound/outbound transfers end to end — all from the command line, with no
second real chain involved (outbound transfers are verified by recomputing and
signing the published message; inbound transfers are signed as if from a
fictitious peer chain).

This is a development/testing tool. It is not part of the production NTT
packages (`ntt-token`, `ntt`, `ntt-cip56`) and is never uploaded to a real
network — see [MainNet path](#mainnet-path) for what porting this to
production actually requires.

## Prerequisites

- `dpm` (Daml SDK 3.5.1 — `curl -fsSL https://get.digitalasset.com/install/install.sh | sh && dpm install 3.5.1`)
- A JDK (Daml Script runs on the JVM)
- Go >= 1.25 (the toolchain directive in `go.mod` triggers an automatic,
  checksummed download via `GOTOOLCHAIN=auto` if your system Go is older)
- Docker, only for the `localnet` profile (~10 GB RAM; see [LocalNet](#localnet))

Build the Daml packages first, from `canton/`:

```sh
dpm build --all
```

The CLI locates the built DAR at `<canton-dir>/test/.daml/dist/ntt-test-0.1.0.dar`
and re-uploads it on every ledger operation (idempotent against an
already-vetted ledger — see [Design notes](#design-notes)).

## Quick start (sandbox profile)

The `sandbox` profile runs a bare `dpm sandbox` — fast, no Docker, and what CI
uses. It is the default; no `--profile` flag needed.

```sh
cd canton/testing/cli
go build -o ntt-playground ./cmd/ntt-playground

./ntt-playground network up                 # starts `dpm sandbox` in the background
./ntt-playground init                       # generates a fresh 1/1 guardian key
./ntt-playground deploy --config testdata/deploy-burnmint.json

# Inbound: sign a transfer as if it arrived from a peer chain, then relay it.
./ntt-playground guardian sign-transfer --deployment burnmint \
  --to-recipient Alice --amount 1000000 --source-chain 2
./ntt-playground receive --deployment burnmint \
  --vaa <vaa-from-above> --recipient Alice --pubkey <pubkey-from-above>
./ntt-playground balance --party Alice --deployment burnmint

# Outbound: transfer, recompute the published message, and sign the resulting VAA.
./ntt-playground transfer --deployment burnmint \
  --user Bob --chain 2 --recipient-address 00..ee --amount 500000 --sign

./ntt-playground status
./ntt-playground network down
```

State (parties, the guardian key, deployment addresses/peers) persists in
`playground.state.json` in the working directory between invocations — every
command after `init` reads and updates it. Never commit this file; it holds a
private key (devnet-only, but still a secret).

## Interactive use

Because state persists and the network keeps running between invocations, the
CLI doubles as an ad-hoc exploration shell against a long-running network —
list party ids, allocate parties, register a standalone core-bridge `Emitter`
(no NTT deployment involved), publish and verify an arbitrary message, inspect
live contracts, and check network health:

```sh
./ntt-playground --profile localnet network up        # once; stays up between commands
./ntt-playground --profile localnet init
./ntt-playground --profile localnet party list        # 1. query party ids present
./ntt-playground --profile localnet party allocate --hint Eve
./ntt-playground --profile localnet emitter register --name oracle --owner oracle-admin   # 2. register an emitter
./ntt-playground --profile localnet publish --emitter oracle --payload deadbeef --sign    # 3. publish arbitrary message
./ntt-playground --profile localnet guardian verify-vaa --vaa <hex>                       #    ...and verify it on-ledger
./ntt-playground --profile localnet contracts list
./ntt-playground --profile localnet network status
```

Every command above also works on the sandbox profile (drop `--profile
localnet`), except that `network status` there reports the sandbox process
instead of per-service compose state.

## Commands

| Command | Purpose |
| --- | --- |
| `network up\|down\|status [--profile sandbox\|localnet]` | Start/stop/check the backing network. `status` on localnet reports each compose service's state/health and exits non-zero when nothing is running. |
| `init [--guardian-key HEX] [--fee N]` | Bootstrap a fresh `CoreState` + registries with a 1/1 guardian set. Generates a random guardian key unless `--guardian-key` is given. `--fee` self-signs and applies an initial `SetMessageFee` governance VAA. |
| `party list` | List every party the participant knows (`party=... isLocal=...`), annotating the hints of playground-allocated ones (`hint=Alice`). |
| `party allocate --hint HINT` | Allocate a party under a hint. Idempotent: an already-known hint returns its existing party. |
| `deploy --config FILE [--name NAME]` | Deploy an NTT (registers the transceiver `Emitter`, stands up the token seam, registers the `NttManager`, claims a replay-trie root, and pre-sets any peers listed in the config). |
| `emitter register --name NAME --owner HINT` | Register a standalone core-bridge `Emitter` (not tied to an NTT deployment), keyed under a CLI-local name in state. |
| `publish --emitter NAME --payload HEX [--nonce N] [--consistency-level N] [--sign]` | Publish an arbitrary message from a registered emitter via `Emitter.PublishMessage`; `--sign` also signs the resulting VAA with the playground's guardian key. |
| `peer set --deployment NAME --chain N --manager HEX --transceiver HEX` | Configure (or replace) a peer for a remote chain. |
| `transfer --deployment NAME --user HINT --chain N --recipient-address HEX --amount N [--sign] [--tap-usd USD]` | Outbound `NttManager.Transfer`. Prints the recomputed published message (bit-exact — same encoders the manager used internally); `--sign` also signs the resulting VAA with the playground's guardian key. For `cip56-custody` the sender is onboarded as a real validator wallet user and tapped (`--tap-usd`, default `"100"`, `"0"` skips) before the real transfer-factory is resolved. |
| `receive --deployment NAME --vaa HEX --recipient HINT --pubkey HEX [--executor HINT]` | Relay a signed VAA through `NttManager.Receive`. A replayed VAA exits non-zero. For owner-signed token kinds the recipient must have run `preapprove` first, else the mint gate rejects the delivery (the VAA stays deliverable). |
| `preapprove --deployment NAME --user HINT` | Opt a recipient in to inbound deposits via `NttToken.PreApproveDeposit` (a standing `DepositPreapproval`). For the admin-signed mock kind this is a ledger-surfaced no-op (`preapproved=false`). |
| `preapprove revoke --deployment NAME --user HINT` | Tear down a recipient's standing deposit pre-approval (owner-only `DepositPreapproval.Revoke`). |
| `guardian sign-transfer --deployment NAME --to-recipient HINT --amount N [--source-chain N] [--sequence N]` | Sign an inbound NTT transfer VAA as if it came from the deployment's configured peer. |
| `guardian sign-vaa --emitter-chain N --emitter HEX --sequence N --payload HEX` | Sign an arbitrary payload as a VAA (not NTT-specific). |
| `guardian sign-governance set-fee --fee N [--apply]` | Sign a Core `SetMessageFee` governance VAA (module `Core`, target chain 72); `--apply` also submits it via `SubmitGovernanceVAA`. |
| `guardian verify-vaa --deployment NAME --vaa HEX` | Verify a VAA on-ledger via `CoreState.ParseAndVerifyVAA` (no replay-consume) — cross-checks the CLI's off-chain signature against the live guardian set. |
| `status` | Print the guardian set, message fee, and every deployment. |
| `contracts list` | Print every live `CoreState`/`Emitter`/`NttManager` (plus the replay-node count) as JSON. |
| `observe --deployment NAME` | Print one deployment's current outbound sequence and peers. |
| `observe stream --deployment NAME [--from-offset N] [--count N] [--timeout DUR] [--print-offset] [--any-emitter]` | LocalNet only. Read the REAL Ledger API v2 update stream as a dedicated `guardian-watcher` reader user (granted only `CanReadAs(guardianObserver)`, never `actAs`) and print every observed `WormholeMessage` as JSON. `--print-offset` prints the current ledger end (`ledgerEnd=<n>`) and exits, for recording a starting point before a transfer. Filters by the deployment's derived transceiver address unless `--any-emitter`. |
| `balance --party HINT --deployment NAME` | Print a party's mock/CIP-56 holdings for a deployment, or (for `cip56-custody`) its real Amulet holdings (`amuletHoldingTotal=...`). A deployment's own `"<name>-custody"` hint resolves directly (it's a wallet user's real party, not one the CLI allocated). |

Every command accepts `--verbose`: each sub-step — network bring-up details,
party allocation/actAs grants, every `dpm script` invocation with its input and
duration, and what the Daml script does on-ledger — is narrated on **stderr**
with a `[v] ` prefix. stdout is unchanged whether or not the flag is set, so
`field=value` output stays machine-parseable. The e2e suite always passes
`--verbose`, so `go test -tags e2e ./e2e -v` shows the full story per step.

Party hints (`--user`, `--recipient`, `--to-recipient`, `--party`, `--executor`)
are display names the CLI allocates fresh Canton parties for on first use and
remembers thereafter (`playground.state.json`'s `users` map) — the same hint
always resolves to the same party across commands.

### Deploy config

```json
{
  "name": "burnmint",
  "mode": "burn-mint",
  "tokenKind": "mock-admin-signed",
  "decimals": 8,
  "peers": [
    { "chain": 2, "manager": "00..bb", "transceiver": "00..cc" }
  ]
}
```

- `mode`: `"burn-mint"` or `"lock-unlock"` (`"cip56-custody"` only supports
  `"lock-unlock"` — Amulet has no `BurnMintFactory`).
- `tokenKind`: `"mock-admin-signed"` (default choice for testing — `LockOrBurn`
  always succeeds regardless of the caller's actual holdings, so `transfer`
  needs no funding step), `"cip56-burn-mint-mock"` or `"cip56-custody-mock"`
  (both drive the real production `Cip56BurnMintToken`/`Cip56CustodyToken`
  implementations over a local mock registry, and DO need `transfer` to fund
  the sender first — the CLI does this automatically), or `"cip56-custody"`
  (the same `Cip56CustodyToken` hook against **real Canton Coin (Amulet)** on
  the LocalNet profile — see [Real Amulet (`cip56-custody`)](#real-amulet-cip56-custody)).
- `decimals`: Amulet has 10 decimals, so `"cip56-custody"` deploy configs use
  `10` (the NTT wire still trims to 8, per `internal/wire.TrimDecimals`).
- `peers`: optional; pre-configures peers at deploy time (equivalent to
  `peer set` calls after the fact).

See `testdata/deploy-burnmint.json`, `testdata/deploy-lockunlock.json`, and
`testdata/deploy-cip56-custody.json`.

### Real Amulet (`cip56-custody`)

`cip56-custody` deploys `Cip56CustodyToken` against the real DSO's live
Amulet `InstrumentId` on Splice LocalNet — real Canton Coin locks/unlocks,
not a mock registry. Requires `--profile localnet` (`deploy` rejects it
otherwise: `"cip56-custody requires a profile with real Amulet (localnet)"`).

- **Party model.** Unlike every other kind, both the custody party and the
  sending user must be **validator wallet users**, not bare script-allocated
  parties — only wallet users can be tapped or hold a `TransferPreapproval`.
  `deploy` onboards the deployment's custody wallet user
  (`<name>-custody`, e.g. `cc-custody-custody`), taps the validator's own
  wallet (it pays the `TransferPreapproval`'s creation fee), and creates that
  preapproval. `transfer --user HINT` onboards the sender the same way and
  taps it (`--tap-usd`, USD-denominated — the devnet tap divides by the open
  mining round's Amulet price server-side, so don't assert the resulting CC
  amount of a tap itself).
- **Registry resolution happens off-ledger, per call.** `internal/amulet`
  resolves the real transfer-factory cid, its choice context, and its
  disclosed contracts (`AmuletRules`, `TransferPreapproval`, `OpenMiningRound`,
  `ExternalPartyConfigState`, `ExternalPartyAmuletRules`) from the validator's
  scan-proxy immediately before every `transfer`, asserting `transferKind ==
  "direct"` (anything else means the receiver's preapproval is missing/expired
  and would settle `Pending`, which the custody hook rejects).
- **Receiving is out of scope.** `receive --deployment NAME` hard-errors for
  this kind (`"receiving network out of scope for real Amulet
  (cip56-custody)"`) — there is no mock factory to unlock against.
- **Balances.** `balance --party <name>-custody --deployment NAME` prints
  `amuletHoldingTotal=<decimal>`, read directly off the token-standard
  `Holding` interface (`Playground.Query:amuletBalance`), not an HTTP call.
- **Observing the guardian's actual observation.** `observe stream` (see the
  Commands table) proves the guardian genuinely observed the outbound
  transfer — reading the real Ledger API v2 update stream as a reader user
  with only `CanReadAs(guardianObserver)`, not by recomputing the payload a
  second time.

## Profiles

### sandbox (default)

A bare `dpm sandbox`, no authentication, no Docker. This is the CI-viable path
and what `go test -tags e2e ./e2e` exercises by default.

### LocalNet

[Splice LocalNet](https://github.com/canton-network/splice) is the official
Canton Network docker-compose stack: a real DSO topology and Canton
Coin/Amulet, as opposed to the sandbox's bare single-participant ledger.

```sh
curl -fsSL -O https://github.com/digital-asset/decentralized-canton-sync/releases/download/v0.6.12/0.6.12_splice-node.tar.gz
tar xzf 0.6.12_splice-node.tar.gz
export LOCALNET_DIR=$PWD/splice-node/docker-compose/localnet
export IMAGE_TAG=0.6.12

./ntt-playground --profile localnet network up     # first boot: DSO bootstrap, 2-6 minutes
./ntt-playground --profile localnet init
# ...same commands as above, with --profile localnet
./ntt-playground --profile localnet network down    # wipes parties/DARs/state
```

The CLI applies a compose override on top of the bundle
(`wormhole-postgres-override.yaml`, written into `LOCALNET_DIR` automatically):
the bundle hardcodes `container_name: postgres` (colliding with any other
container of that name, even a stopped one) and publishes the database on host
port 5432 (colliding with a locally running PostgreSQL). The override renames
the container to `splice-localnet-postgres` and publishes host port 15432
instead; set `LOCALNET_POSTGRES_CONTAINER_NAME` / `LOCALNET_POSTGRES_HOST_PORT`
to change either. In-network connectivity is unaffected — services resolve the
database by compose service name. Requires docker compose >= 2.24 (`!override`).

The `app-provider` participant is used throughout: gRPC Ledger API 3901, JSON
Ledger API v2 3975, validator API 3903. Auth is LocalNet's documented
"unsafe" shared-secret HS256 mode (never valid against a real participant);
the CLI mints tokens for `ledger-api-user` automatically.

Amulet (Canton Coin) does not implement `BurnMintFactory`, so only the
lock/unlock path is exercisable against it directly — see
[Real Amulet (`cip56-custody`)](#real-amulet-cip56-custody) for the real
(not mock) registry client this profile enables.

**Verification status:** the sandbox profile is continuously verified in CI
(the `playground CLI (go vet/test + sandbox e2e)` job in
[`.github/workflows/canton.yml`](../../../.github/workflows/canton.yml)), on
every push and pull request. The LocalNet profile's distinguishing surface —
unsafe-JWT auth, DAR vetting against a real DSO topology, `CanActAs` grants,
real Amulet (tap, `TransferPreapproval`, the transfer-instruction registry),
and the real Ledger API v2 update stream — is exactly what that job can never
exercise, so it is covered separately by the `playground CLI (LocalNet e2e)`
job in the same workflow (weekly schedule plus manual `workflow_dispatch`, not
a PR gate: the stack is too heavy to run on every push). The full e2e suite,
including the real-Amulet `cip56-custody` subtest (deploy against the live
DSO, tap + `TransferPreapproval` + a real 1 CC lock, and a real update-stream
observation whose sequence/payload matched the recomputed ones byte-for-byte),
has already passed end to end against a live Splice LocalNet 0.6.12 stack
(`NTT_PLAYGROUND_PROFILE=localnet go test -tags e2e ./e2e -v`), confirming
auth, DAR vetting, party rights, real-Amulet wallet/registry calls, the WS
update stream, and the JSON encoding assumptions against a real authenticated
participant. The suite has grown since that first run; treat newly added
subtests as unverified against LocalNet until the scheduled job covers them.
LocalNet-only facts discovered and fixed during verification, each impossible
to observe on the auth-less sandbox:

- the validator's `v0` API (scan-proxy, unlike `readyz`) requires a bearer
  token, so the readiness poll and `DSOPartyID` send one;
- allocating a party does not grant the allocating user submission rights, so
  every CLI party allocation is followed by a `CanActAs` grant for
  `ledger-api-user` (scripts take pre-allocated parties as input rather than
  calling `allocateParty` before submitting);
- the `/v2/users/<user>/rights` endpoint requires the user named in the body
  and the rights `oneOf` wrapped in `value`;
- `dpm script`'s `Disclosure.blob` field is hex, not the registry's base64
  `createdEventBlob` — the CLI re-encodes it;
- the documented `daml.ws.auth`/`jwt.token.<jwt>` WebSocket subprotocol pair
  handshakes but fails the actual request with `UNAUTHENTICATED`; a plain
  `Authorization: Bearer` header works;
- tap/onboarding/preapproval/transfer-factory calls can occasionally exceed a
  naive fixed timeout under validator automation load — the CLI retries
  transient transport timeouts, not just HTTP-level 503/429 responses.

## Design notes

- **No Go gRPC client.** Every ledger operation is a parameterized Daml
  Script (`canton/test/daml/Playground/*.daml`) invoked via `dpm script
  --input-file/--output-file`. Disclosures, multi-party `actAs`, and interface
  exercising are already solved in Daml Script; sandbox and LocalNet differ
  only in host/port/auth/upload flags.
- **Never persist contract ids.** `CoreState`, `NttManager`, and `Emitter` all
  churn on every consuming exercise. `playground.state.json` stores only
  stable identities (parties, `managerId`) — every script re-resolves
  contracts from the ACS by identity.
- **Outbound observation without an event stream.** A `WormholeMessage` is a
  choice result, not a template, so nothing can query it after `Transfer`
  returns. `transferOut` instead reads the manager's sequence counter before
  exercising `Transfer` and recomputes the exact published payload with the
  same `Wormhole.Ntt.Payload` encoders the manager uses internally.
- **JSON encoding gotchas**, found by running against a live sandbox rather
  than assumed: a Daml `(Int, Bytes)` tuple (the `pubKeys` hints
  `ParseAndVerifyVAA`/`VerifyAndConsumeVAA`/`Receive` take) serializes as a
  JSON object `{"_1": ..., "_2": ...}`, not a 2-element array
  (`internal/ledger.PubKeyHint`); a Daml `Decimal` serializes as a bare JSON
  number (e.g. `0E-10`), not a quoted string
  (`cmd/ntt-playground.decimalLiteral`); and a Go `nil` slice marshals to
  JSON `null`, which a Daml list-typed field rejects — every list field must
  start non-nil.
- **Hex vs. base64**, found by running against a live LocalNet: `dpm script`'s
  `Disclosure.blob` field is hex, while the Amulet transfer-instruction
  registry's `createdEventBlob` is base64 — same bytes, different rendering.
  `toAmuletSeamJSON` (`transfer.go`) re-encodes before handing the seam to
  `Playground.Ops:transferOut`.

## MainNet path

Documented, not implemented — this CLI is devnet-only throughout.

- **Profile → real validator.** A production profile would point at your own
  participant's ledger endpoint, use OAuth client-credentials JWTs instead of
  the unsafe HS256 mode, and skip `allocateParty` (parties pre-exist, onboarded
  through the validator).
- **Genesis/governance.** `initPlayground` stands in for a real ceremony:
  `guardianGovernance` becomes an external/threshold party via topology
  transactions, the guardian set is installed at genesis co-signed by
  operator + guardianGovernance, and evolves only through real guardian-set-
  upgrade governance VAAs (`SubmitGovernanceVAA`). This CLI's 1/1 signer
  (`internal/guardian`) never becomes production code — a real guardian set
  is k-of-n and the keys never touch a CLI.
- **DARs.** Only the production DARs (`ntt-token`, `ntt`, `ntt-cip56`) are
  ever uploaded to a real network — never `ntt-test` (it drags in
  `daml-script`). Playground ops would need to be re-expressed against
  production package-ids, either via the JSON Ledger API/gRPC directly or a
  script-only companion DAR with zero templates.
- **Fees/token.** The fee instrument becomes Canton Coin; a real `Allocation`
  is drawn from the user's wallet per `Transfer`/`PublishMessage`. The token
  seam becomes a real Amulet (or other CIP-56) registry client. Only
  lock/unlock (`Cip56CustodyToken`) works against Amulet directly — burn/mint
  needs a registry implementing `BurnMintFactory`.
- **Disclosures.** The operator-as-oracle `queryDisclosure`/`coveringNode`
  pattern this CLI uses is replaced by an off-ledger disclosure service over
  an ACS index, as described in the core bridge's README.

## Follow-ups

Out of scope for this CLI, listed here rather than silently dropped:

- Guardian-set-upgrade governance signing (only `SetMessageFee` is
  implemented under `guardian sign-governance`).
