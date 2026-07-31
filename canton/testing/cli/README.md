# ntt-playground

A devnet playground CLI for the Canton NTT contracts in `canton/`. It stands up
a local Canton network, deploys an NTT (a manager, a transceiver, and the token
interface the manager calls to move tokens), controls a 1/1 Wormhole guardian
that signs VAAs (Wormhole's signed cross-chain messages) on demand, and drives
inbound/outbound transfers end to end — all from the command line, with no
second real chain involved (outbound transfers are verified by recomputing and
signing the published message; inbound transfers are signed as if from a
fictitious peer chain).

This is a development/testing tool. It is not part of the production `ntt`
package and is never uploaded to a real network — see
[MainNet path](#mainnet-path) for what porting this to production actually
requires.

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

State (parties, the guardian key, deployment addresses/peers — including each
peer's configured decimals) persists in `playground.state.json` in the
working directory between invocations — every
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
| `init [--guardian-key HEX] [--fee N]` | Bootstrap a fresh `CoreState` + core registries + the NTT root `NttGovernance` with a 1/1 guardian set (propose/accept — see [Genesis](#genesis-and-the-2-of-2-requirement)). Generates a random guardian key unless `--guardian-key` is given. `--fee` self-signs and applies an initial `SetMessageFee` governance VAA. |
| `party list` | List every party the participant knows (`party=... isLocal=...`), annotating the hints of playground-allocated ones (`hint=Alice`). |
| `party allocate --hint HINT` | Allocate a party under a hint. Idempotent: an already-known hint returns its existing party. |
| `deploy --config FILE [--name NAME]` | Deploy an NTT in two steps: `deployRegistry` (gg-submitted) stands up the deployment's mock registry factory contract(s), then `deployNtt` (admin-submitted) resolves them and makes one `RegisterManager` call that atomically registers the `NttManager`, mints its gg-owned transceiver `Emitter`, and claims its replay-trie root — then pre-sets any peers listed in the config. For `amulet` `deployRegistry` is a no-op (the real Splice registry's factory already exists) and `deployNtt` onboards the admin as a validator wallet user first. |
| `emitter register --name NAME --owner HINT` | Register a standalone core-bridge `Emitter` (not tied to an NTT deployment), keyed under a CLI-local name in state. |
| `publish --emitter NAME --payload HEX [--nonce N] [--consistency-level N] [--sign]` | Publish an arbitrary message from a registered emitter via `Emitter.PublishMessage`; `--sign` also signs the resulting VAA with the playground's guardian key. |
| `peer set --deployment NAME --chain N --manager HEX --transceiver HEX --decimals N` | Configure (or replace) a peer for a remote chain. `--decimals` (required, 1-255) is the peer chain token's decimals: outbound transfers to that peer trim to `min(8, tokenDecimals, peerDecimals)` and reject any amount that doesn't round-trip exactly at that precision ("dust the peer cannot represent"). |
| `transfer --deployment NAME --user HINT --chain N --recipient-address HEX --amount N [--sign] [--tap-usd USD] [--no-fund]` | Outbound `NttManager.Transfer`. For `mock`, ensures the sender has a standing pre-approval (`preapprove`) and funds it (`fundUser`) before transferring, unless `--no-fund` (always implied by `--strict-participant-isolation`) is set, in which case the sender's own holdings are enumerated in-script instead — see [Disclosure service](#disclosure-service). Prints the recomputed published message (bit-exact — same encoders the manager used internally); `--sign` also signs the resulting VAA with the playground's guardian key. For `amulet` the sender is onboarded as a real validator wallet user and tapped (`--tap-usd`, default `"100"`, `"0"` skips) before the real transfer-factory is resolved; the receiver is the deployment's admin (custody is admin-owned). Rejected while the deployment is paused ("deployment is paused"), and rejected if `--amount` doesn't round-trip exactly at the destination peer's configured decimals (dust). |
| `fund --deployment NAME --user HINT --amount N` | Mock-kind-only faucet: ensure the recipient's standing pre-approval, then mint via `Playground.Ops:fundUser` (`guardian-governance`-submitted — gg is the mint's sole signatory). The `preapprove`+`fundUser` block that used to be welded into `transfer`, split out as its own operator-side command — see [Disclosure service](#disclosure-service). |
| `receive --deployment NAME --vaa HEX --recipient HINT --pubkey HEX [--executor HINT]` | Relay a signed VAA through `NttManager.Mint`/`Release`, executor-only. A replayed VAA exits non-zero. The recipient must have run `preapprove` first — for both burn-mint and lock-unlock `mock` deployments — else the delivery is rejected and the VAA stays deliverable. Rejected outright for `amulet` (receiving against real Amulet is out of scope), and rejected while the deployment is paused. |
| `preapprove --deployment NAME --user HINT` | Opt a recipient in to inbound deliveries: a standing `DepositPreapproval` (burn-mint `mock`) or `MockTransferPreapproval` (lock-unlock `mock`), created self-signed by the recipient. Idempotent. For `amulet` this is a ledger-surfaced no-op (`preapproved=false`); real Amulet's own `TransferPreapproval` is out of scope here. |
| `preapprove revoke --deployment NAME --user HINT` | Tear down a recipient's standing pre-approval (owner-only `Revoke`, either template). |
| `pause --deployment NAME` | Admin-controlled (submits as whoever CURRENTLY holds the role): halt a deployment's value movement (`Transfer`/`Release`/`Mint`) via `NttManager.SetPaused`. While paused, `transfer` and `receive` both fail with "deployment is paused"; config choices — including `admin accept-gg-vaa`'s underlying `TransferAdmin` — keep working. |
| `unpause --deployment NAME` | Admin-controlled: lift a pause, restoring `transfer`/`receive`. |
| `admin propose-gg --deployment NAME` | Have the deployment's CURRENT admin create a standing `AdminTransferProposal` offering the operational role to `guardianGovernance` — the propose half of the VAA-gated custody opt-in (see [Guardian custody opt-in](#guardian-custody-opt-in)). Prints `managerAddress=` and `factoryEpoch=`, the two fields `guardian sign-governance accept-admin` binds its VAA to. |
| `admin accept-gg-vaa --deployment NAME --vaa HEX --pubkey HEX [--executor HINT]` | Permissionlessly relay a guardian-signed accept-admin VAA through `NttManager.AcceptAdminTransferByVaa`, completing the handoff with no live `gg` signature. On success, updates the state file's recorded admin for this deployment to `gg`. A replayed VAA exits non-zero. Succeeds even while the deployment is paused (an admin/config operation, not a value movement). |
| `guardian sign-transfer --deployment NAME --to-recipient HINT --amount N [--source-chain N] [--sequence N]` | Sign an inbound NTT transfer VAA as if it came from the deployment's configured peer. |
| `guardian sign-vaa --emitter-chain N --emitter HEX --sequence N --payload HEX` | Sign an arbitrary payload as a VAA (not NTT-specific). |
| `guardian sign-governance set-fee --fee N [--apply]` | Sign a Core `SetMessageFee` governance VAA (module `Core`, target chain 72 — `cantonChainId`, the playground's fixed Wormhole chain id, also literally `NttManager.chainId` now, not just a VAA convention); `--apply` also submits it via `SubmitGovernanceVAA`. |
| `guardian sign-governance accept-admin --deployment NAME [--factory-epoch N]` | Sign an NTT `AcceptAdminTransferToGovernance` governance VAA (module `Ntt`, target chain 72 — checked against the target manager's own `chainId`) authorizing `gg`'s acceptance of the named deployment's admin role. `--factory-epoch` must match the deployment's CURRENT factory epoch at relay time (default `0`, the registration epoch); `admin propose-gg`'s output reports the live value to sign against. |
| `guardian verify-vaa --deployment NAME --vaa HEX` | Verify a VAA on-ledger via `CoreState.ParseAndVerifyVAA` (no replay-consume) — cross-checks the CLI's off-chain signature against the live guardian set. |
| `status` | Print the guardian set, message fee, and every deployment (including each deployment's `chainId`, its peers with their `decimals`, and `paused`). |
| `contracts list` | Print every live `CoreState`/`Emitter`/`NttManager` (plus the replay-node count) as JSON. |
| `observe --deployment NAME` | Print one deployment's current outbound sequence, `chainId`, peers (with `decimals`), and `paused` state. |
| `observe stream --deployment NAME [--from-offset N] [--count N] [--timeout DUR] [--print-offset] [--any-emitter]` | LocalNet only. Read the REAL Ledger API v2 update stream as a dedicated `guardian-watcher` reader user (granted only `CanReadAs(guardianObserver)`, never `actAs`) and print every observed `WormholeMessage` as JSON. `--print-offset` prints the current ledger end (`ledgerEnd=<n>`) and exits, for recording a starting point before a transfer. Filters by the deployment's derived transceiver address unless `--any-emitter`. |
| `balance --party HINT --deployment NAME` | Print a party's mock/CIP-56 holdings for a deployment, or (for `amulet`) its real Amulet holdings (`amuletHoldingTotal=...`). Custody is admin-owned, so a deployment's own admin hint (the config's `adminHint`, or `<name>-admin` by default) resolves the custodian's own balance — it's a wallet user's real party for `amulet`, an ordinary CLI-allocated party otherwise. |
| `disclosure serve [--listen ADDR] [--participant ROLE]` | Serve the read-only disclosure service (`GET /v1/healthz`, `GET /v1/disclosures?template=...`, `POST /v1/seam/{transferOut,receive,publish,acceptAdminTransfer}`) fronting one participant (default `--participant guardian-governance`). Only meaningful on `localnet` (the sandbox profile has a single participant, so there is never a cross-participant fetch to serve, and its generic `/v1/disclosures` endpoint 503s there for lack of a JSON Ledger API). Binds `127.0.0.1:7599` by default; a non-loopback `--listen` prints a startup warning rather than silently exposing allow-listed contract payloads — see [Disclosure service](#disclosure-service). |

Every command accepts `--verbose`: each sub-step — network bring-up details,
party allocation/actAs grants, every `dpm script` invocation with its input and
duration, and what the Daml script does on-ledger — is narrated on **stderr**
with a `[v] ` prefix. stdout is unchanged whether or not the flag is set, so
`field=value` output stays machine-parseable. The e2e suite always passes
`--verbose`, so `go test -tags e2e ./e2e -v` shows the full story per step.

Party hints (`--user`, `--recipient`, `--to-recipient`, `--party`, `--executor`)
are display names the CLI allocates fresh Canton parties for on first use and
remembers thereafter (`playground.state.json`'s `users` map) — the same hint
always resolves to the same party across commands. On the `localnet` profile
the hint also decides which participant the party is allocated on and
routed to thereafter (`--topology-config`'s `partyHosting` map — see
[Topology](#topology)); the resolved participant is persisted in state
alongside the party id, so it stays stable even if the config changes later.

### Deploy config

```json
{
  "name": "burnmint",
  "mode": "burn-mint",
  "tokenKind": "mock",
  "decimals": 8,
  "peers": [
    { "chain": 2, "manager": "00..bb", "transceiver": "00..cc", "decimals": 8 }
  ]
}
```

- `mode`: `"burn-mint"` or `"lock-unlock"` (`"amulet"` only supports
  `"lock-unlock"` — Amulet has no `BurnMintFactory`).
- `tokenKind`: `"mock"` (a local mock registry — see
  [Receive: mock-registry pre-approvals](#receive-mock-registry-pre-approvals)
  — `transfer` funds the sender automatically before sending) or `"amulet"`
  (the manager's real production token-standard interfaces against **real
  Canton Coin (Amulet)** on the LocalNet profile — see
  [Real Amulet (`amulet`)](#real-amulet-amulet)).
- `decimals`: Amulet has 10 decimals, so `"amulet"` deploy configs use `10`
  (the NTT wire still trims to 8, per `internal/wire.TrimDecimals`).
- `adminHint`: optional display name for the deployment's admin party
  (default `<name>-admin`) — the lock/unlock custodian and the burn/mint
  instrument-binding admin. For `"amulet"` this party is validator-onboarded
  rather than bare-allocated (see [Real Amulet](#real-amulet-amulet)).
- `peers`: optional; pre-configures peers at deploy time (equivalent to
  `peer set` calls after the fact). Each peer entry requires `decimals`
  (1-255) — the peer chain token's decimals; see the `peer set` row above
  for how it governs outbound trimming.

See `testdata/deploy-burnmint.json`, `testdata/deploy-lockunlock.json`, and
`testdata/deploy-amulet.json`.

### Receive: mock-registry pre-approvals

Daml authority is not transitive through nested exercises, so a factory's
transfer implementation can only create a receiver's holding when the
receiver's own signature (or the factory-signing party's) is reachable
inside the choice body — an executor relaying a VAA has neither. The
playground closes this gap the same way real registries do: a receiver
signs a standing pre-approval once, up front, and the factory delivers
through it. `Playground.MockRegistry` (`canton/test/daml/Playground/`)
supplies the missing piece for `"mock"` lock/unlock: `MockTransferPreapproval`
(signed by its `owner`, observed by `guardianGovernance` so the registry can
resolve it) and `MockPreapprovedTransferFactory`, whose delivery choice is
controlled by `guardianGovernance` — inside it, both the owner's signature
(from the pre-approval) and gg's (the controller) are in scope, closing the
gap. Burn/mint uses the model's own `DepositPreapproval` for the same
purpose. Either way, `preapprove` is the one-time recipient opt-in and
`receive` (any executor) is the permissionless relay — see the Commands
table above.

### Guardian custody opt-in

Handing a deployment's `admin` role to `guardianGovernance` (`gg`) is the
guardian quorum's custody opt-in (see the core NTT README's "Custody follows
the admin"). In production this is completed without any live `gg`
signature — guardians sign an off-chain governance VAA, and any executor
relays it permissionlessly:

```sh
# The deployment's CURRENT admin creates the standing offer.
./ntt-playground admin propose-gg --deployment lockunlock
# managerAddress=... factoryEpoch=0

# The guardian quorum signs an acceptance VAA bound to that (managerAddress,
# factoryEpoch) pair. The epoch stands in for the factory's contract id,
# which on-ledger code cannot compare against VAA bytes; any factory
# rotation after vetting bumps it, so a stale VAA fails closed.
./ntt-playground guardian sign-governance accept-admin \
  --deployment lockunlock --factory-epoch 0
# vaa=... pubkey=...

# For a lock/unlock deployment with a non-empty reserve, gg needs its own
# standing pre-approval first -- the pot's receiver is gg itself.
./ntt-playground preapprove --deployment lockunlock --user GuardianGovernance

# Any executor relays the VAA; no gg signature is submitted here.
./ntt-playground admin accept-gg-vaa --deployment lockunlock \
  --vaa <vaa> --pubkey <pubkey> --executor Erin
```

`admin accept-gg-vaa` updates the state file's recorded admin for the
deployment to `gg` on success, so later commands (`status`, a subsequent
`receive`) see the new admin without a fresh ledger round-trip. Re-submitting
the same VAA fails as a replay (the digest was already consumed in `gg`'s
replay trie), and `receive` against the deployment continues to work
unchanged — the handoff repoints custody only, nothing on the wire.

`admin propose-gg` is single-party (it reads and submits as the deployment's
CURRENT admin alone), so it never needs a `RemoteSeam` — instead the CLI routes
the whole call to that admin's own participant, which is operator's under the
default topology pre-handoff, and `gg`'s own once a prior `accept-gg-vaa` has
completed. `admin accept-gg-vaa` gets the same `RemoteSeam` treatment `transfer`/`receive`
already have (see [Topology](#topology)): on the single-participant `sandbox`
profile it takes the unmodified `remote = None` fast path
`Playground.Ops:acceptAdminTransferByVaa` has always supported; on a
multi-participant `localnet` topology, whenever the relaying `--executor`'s
participant differs from `gg`'s (unconditionally the case there, since gg is
its own participant), the CLI fetches a `RemoteSeam` via
`Playground.Prepare:prepareAcceptAdmin` first — either by running that script
directly against `gg`'s own participant, or, with `--disclosure-service-url`
set, over HTTP against the disclosure service (see
[Disclosure service](#disclosure-service)) — exactly like `transfer`/`receive`
do.

### Real Amulet (`amulet`)

`amulet` drives the manager's real production token-standard interfaces
against the real DSO's live Amulet `InstrumentId` on Splice LocalNet — real
Canton Coin locks/unlocks, not a mock registry. Requires `--profile localnet`
(`deploy`/`transfer` reject it otherwise:
`"amulet requires a profile with real Amulet (localnet)"`).

- **Party model.** Custody is admin-owned — there is no separate custody
  party. `deploy` onboards the deployment's **admin** as a validator wallet
  user (`setupAmuletAdmin`), taps the validator's own wallet (it pays the
  admin's own `TransferPreapproval`'s creation fee), creates that
  preapproval, and resolves the registry's current transfer-factory cid via
  a probe transfer. `transfer --user HINT` onboards the sending user the
  same way and taps it (`--tap-usd`, USD-denominated — the devnet tap
  divides by the open mining round's Amulet price server-side, so don't
  assert the resulting CC amount of a tap itself); every lock's receiver is
  the deployment's admin.
- **Registry resolution happens off-ledger, per call.** `internal/amulet`
  resolves the real transfer-factory cid, its choice context, and its
  disclosed contracts (`AmuletRules`, `TransferPreapproval`, `OpenMiningRound`,
  `ExternalPartyConfigState`, `ExternalPartyAmuletRules`) from the validator's
  scan-proxy immediately before every `transfer`; if the resolved factory cid
  no longer matches the manager's committed one, the CLI submits `SetFactory`
  as admin first, then proceeds.
- **Receiving is out of scope.** `receive --deployment NAME` hard-errors for
  this kind (`"receiving is out of scope for real Amulet (amulet)"`) — there
  is no mock factory to unlock against.
- **Balances.** `balance --party <adminHint> --deployment NAME` prints
  `amuletHoldingTotal=<decimal>`, read directly off the token-standard
  `Holding` interface (`Playground.Query:amuletBalance`), not an HTTP call.
- **Observing the guardian's actual observation.** `observe stream` (see the
  Commands table) proves the guardian genuinely observed the outbound
  transfer — reading the real Ledger API v2 update stream as a reader user
  with only `CanReadAs(guardianObserver)`, not by recomputing the payload a
  second time.

## Topology

On the `localnet` profile the playground runs across **six participants**,
not one — this is what makes Daml's privacy model observable: a party only
sees the contracts it is a stakeholder or disclosure-recipient on, and only
its own home participant can act as it.

| participant | hosts | gRPC ledger | JSON API | validator API |
| --- | --- | --- | --- | --- |
| `app-provider` | Operator, deployment admins (custody-owning; validator-onboarded wallet users for `amulet`), and the default for any unmapped party hint | 3901 | 3975 | 3903 |
| `app-user` | Alice | 2901 | 2975 | 2903 |
| `bob` | Bob | 5901 | 5975 | 5903 |
| `guardian-governance` | GuardianGovernance | 6901 | 6975 | 6903 |
| `guardian-observer` | GuardianObserver | 7901 | 7975 | 7903 |
| `alice-solo` | whichever hint a `--topology-config` explicitly routes there (e.g. `Zoe` in `testdata/topology-alice-solo.json`) — not part of the built-in `partyHosting` default, so no hint lands here unless a config says so | 8901 | 8975 | 8903 |

`app-provider` and `app-user` are participants the Splice LocalNet bundle
already runs; `bob`, `guardian-governance`, `guardian-observer`, and
`alice-solo` are added by a compose override the CLI writes into the
LocalNet directory (alongside the existing Postgres-container override) —
the vendored bundle itself is never edited. Every participant runs the same
unsafe shared-secret HS256 auth as `app-provider` and is reachable through
the same `ledger-api-user` admin user, so `party list`, `balance --party`,
and every other per-party command transparently target the right
participant. `alice-solo` exists to prove a fresh participant hosting no
playground party but the one the test allocates can still transact — see
[Disclosure service](#disclosure-service).

`--topology-config` (see below) controls which party hint lands on which
participant; the table above is the built-in default when no config file
maps a hint. `party list` reflects the real topology: each line is prefixed
`participant=<role>`, and a party's `isLocal=true` only on its home
participant.

The `sandbox` profile is unaffected by any of this — see
[sandbox](#sandbox-default) below.

### `--topology-config`

A root persistent flag, `--topology-config PATH`, points at a JSON file that
controls two things: which participant a party hint is allocated/routed to
(`partyHosting`), and which Daml templates the CLI is allowed to fetch an
explicit disclosure for before a cross-participant submit (`disclose`).

Default path when the flag is omitted: `playground.topology.json` next to
the state file. A missing file is not an error — it resolves to the
built-in `partyHosting` default (the table above) and a built-in `disclose`
default covering every template a `mock` deployment's `transfer`/`receive`/
`admin accept-gg-vaa` path needs (`NttManager`, `NttGovernance`,
`LockedLedger`, `CoreState`, `Emitter`, the covering `ReplayNode`,
`DepositPreapproval`, `MockTransferPreapproval`,
`MockPreapprovedTransferFactory`, `CoinFactory`, `Cip56MockHolding`,
`AdminTransferProposal`) — otherwise the CLI's basic
`deploy`/`transfer`/`receive` flow would fail out of the box on LocalNet with
no config file at all. The "everything fails closed" posture (no disclosures
ever fetched, so any submit that needs to read a contract off another
participant fails on-ledger with `CONTRACT_NOT_FOUND`) is what an *explicit*
config with `"disclose": []` gets you (see
`testdata/topology-empty.json`) — a present file always means exactly what
it says, even if that's less than the built-in default. A file that exists
but is malformed (bad JSON, or an unrecognized top-level key) is a hard
error rather than a silent fallback, so a typo doesn't quietly disable
disclosure.

Shape:

```json
{
  "partyHosting": {
    "Alice": "app-user",
    "Bob": "bob",
    "GuardianGovernance": "guardian-governance",
    "GuardianObserver": "guardian-observer",
    "*": "app-provider"
  },
  "disclose": [
    { "template": "Wormhole.Ntt.Manager:NttManager", "fetchAs": "GuardianGovernance" },
    { "template": "Wormhole.Core.State:CoreState", "fetchAs": "GuardianGovernance" },
    { "template": "Wormhole.Core.State:Emitter", "fetchAs": "GuardianGovernance" },
    { "template": "Token.CIP0056.CoinFactory:CoinFactory", "fetchAs": "GuardianGovernance" }
  ]
}
```

- `partyHosting` maps a party hint to a participant role; `"*"` is the
  fallback for any hint not listed explicitly.
- `disclose` is an allow-list of `Module:Entity`-qualified templates. Only
  templates on this list are fetched (as a disclosure payload, via a
  read-only Daml script run against the data owner's own participant) and
  attached to a cross-participant submit; a template not on the list is
  simply never fetched. `fetchAs` names the party a template's data is
  conceptually read as — today that's `guardianGovernance` for nearly
  everything a `mock` deployment's transfer/receive path touches (it
  co-signs/owns the manager, `CoreState`, the transceiver `Emitter`, the
  replay trie, and the mock registry's factory/preapproval/holding
  templates outright) and `operator` for `NttGovernance`. The field is
  carried through the config rather than driving participant selection
  itself: each `prepare*` script (`Playground.Prepare`) always reads as one
  fixed party for a given entrypoint, so a config only ever names which
  templates to fetch, not where from.

This is a deliberate, user-controlled dial: pulling an entry out of
`disclose` turns a specific cross-participant operation back into a
`CONTRACT_NOT_FOUND` failure without touching any other template's
visibility. See `testdata/topology-empty.json` (nothing disclosed),
`testdata/topology-localnet.json` (the set the e2e suite runs against), and
`testdata/topology-missing-corestate.json` (the localnet set minus one
entry, for exercising per-template granularity) for worked examples.

When a command's actor and the data it needs both resolve to the same
participant, none of this applies — the command takes the same local path
it always has, disclosure-free.

### Genesis and the 2-of-2 requirement

`init` bootstraps `CoreState` **and** the NTT root `NttGovernance` together,
as a two-step propose/accept across two different participants rather than a
single co-signed create: the operator proposes a
`CoreStateBootstrapProposal` on its own participant, and `GuardianGovernance`
accepts it — the step that actually creates both `CoreState` and
`NttGovernance`, in the same ceremony, so gg signs once for both roots — on
its own, separate participant. Because no participant hosts both parties,
this genuinely requires two independent participants to
confirm the transaction, which is the concrete "2-of-2" property the
topology split delivers: an operator acting alone cannot produce a
`CoreState` GuardianGovernance never saw and never authorized, and vice
versa.

## Disclosure service

`ntt-playground disclosure serve` runs a small, stateless, read-only HTTP
service that fronts one participant (default `guardian-governance`) and owns
the disclosure allow-list on the server side, rather than trusting whichever
CLI happens to be asking. It exposes:

- `GET /v1/healthz` — `{"participant":"...","templates":[...],"ledgerEnd":<offset>}`.
- `GET /v1/disclosures?template=<Module:Entity>[&template=...]` — a generic
  ACS export over the JSON Ledger API v2 (`includeCreatedEventBlob: true`),
  returning each allow-listed contract's id, hex-encoded blob, and decoded
  create argument at the offset the read was pinned to. A template not on
  the service's own allow-list is refused with 403, naming the offender,
  rather than silently served.
- `POST /v1/seam/{transferOut|receive|publish|acceptAdminTransfer}` — runs the
  same `Playground.Prepare:prepare*` script `transfer`/`receive`/`publish`/
  `admin accept-gg-vaa` already run locally, and returns the resulting
  `RemoteSeam` JSON verbatim. The service injects its own `discloseTemplates`
  allow-list into the script input server-side, ignoring whatever the client
  sent — the server, not the caller, decides what gets disclosed.

**Why.** Before this service existed, a consumer's CLI had to hold the data
owner's participant credentials (an admin JWT for `guardian-governance`) to
fetch a `RemoteSeam` — it could read gg's entire ACS and submit as gg. With
the service, a consumer holds only its own participant's credentials and can
obtain nothing beyond the allow-listed templates, read-only. This is a
privilege reduction, not new functionality: the same `RemoteSeam`/
`Disclosure` mechanism ([Topology](#topology) above) is retained end to end;
only who holds which credentials to fetch it changes.

**Client side.** Two root persistent flags:

- `--disclosure-service-url URL` — when set, any command that would
  otherwise run a `prepare*` script directly against the data owner's own
  participant instead fetches the `RemoteSeam` over HTTP from the disclosure
  service at `URL`. Empty (the default) is byte-identical to pre-service
  behavior.
- `--strict-participant-isolation` — an opt-in guard: any step that would
  target a participant other than the actor's own fails immediately, naming
  both roles, instead of silently succeeding because the CLI happens to hold
  every participant's credentials. It is the negative control that proves a
  flow genuinely needs `--disclosure-service-url` rather than admin access it
  should not have. Commands that legitimately span participants by design —
  `init`, `deploy`, `fund`, `party` — never set an isolation baseline, so the
  guard can never fire for them regardless of this flag; they are simply
  incompatible with the isolation model, not broken by it.

**`fund`.** `fund --deployment NAME --user HINT --amount N` is the mock
faucet (`preapprove` + `Playground.Ops:fundUser`) split out of `transfer`,
because minting is a `guardian-governance`-side submit — gg is the mint's
sole signatory — that no disclosure service can stand in for (a
`DisclosedContract` grants visibility, never authority). It is correctly an
operator/faucet action, run by whoever legitimately holds gg's credentials.
`transfer --no-fund` (implied automatically by
`--strict-participant-isolation`) skips that block and leaves the transfer's
input holdings empty; `Playground.Ops:transferOut`'s Mock branch then
enumerates the sender's own holdings in-script, which needs no
cross-participant disclosure at all.

**Security posture — stated plainly.** This is a harness-grade
implementation, not a production one: no authentication, no TLS. It binds
`127.0.0.1` by default; passing `--listen` with a non-loopback address prints
a warning at startup instead of silently exposing allow-listed contract
payloads to anything that can reach it. It is still a strict improvement over
today's baseline (the CLI holding the fronted participant's admin JWT
outright), but a production deployment needs, at minimum: mTLS or OAuth
client-credentials auth, per-consumer (not global) allow-lists, rate
limiting, and an audit log of `(consumer, template, contract id, offset)`.
Do not point `--listen` at anything but loopback outside a test/harness
context.

```sh
# operator side, holding guardian-governance's own credentials:
./ntt-playground --profile localnet disclosure serve
# disclosure serve: fronting participant=guardian-governance templates=11
# disclosure serve: unauthenticated harness-grade service; keep it loopback-bound unless you know why
# disclosure serve: listening on http://127.0.0.1:7599

# consumer side, holding only its own participant's credentials:
./ntt-playground --profile localnet \
  --disclosure-service-url http://127.0.0.1:7599 \
  --strict-participant-isolation \
  transfer --deployment burnmint --user Zoe --chain 2 \
  --recipient-address 00..ee --amount 500000 --sign
```

## Profiles

### sandbox (default)

A bare `dpm sandbox`, no authentication, no Docker. This is the CI-viable path
and what `go test -tags e2e ./e2e` exercises by default.

The [Topology](#topology) split and `--topology-config` are LocalNet-only
concepts. `dpm sandbox` is a single in-process participant — every party is
always co-located with every actor, so there is never a cross-participant
submit to gate, and `--topology-config`/disclosure has no effect here: every
command takes the same local, disclosure-free path it always has.

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

`LOCALNET_DIR` is optional. If it is unset, the CLI looks for the extracted bundle at, in
order:

1. `canton/testing/.localnet/splice-node/docker-compose/localnet` (repo-local cache; gitignored)
2. `$HOME/.cache/ntt-playground/splice-node/docker-compose/localnet`
3. `$HOME/splice-node/docker-compose/localnet`

and uses the first one that contains a `compose.yaml`. Extracting the tarball to one of those
paths means every subsequent shell picks it up with no export needed. Set `LOCALNET_DIR`
explicitly to override discovery or to point at a bundle elsewhere.

The CLI applies a compose override on top of the bundle
(`wormhole-postgres-override.yaml`, written into `LOCALNET_DIR` automatically):
the bundle hardcodes `container_name: postgres` (colliding with any other
container of that name, even a stopped one) and publishes the database on host
port 5432 (colliding with a locally running PostgreSQL). The override renames
the container to `splice-localnet-postgres` and publishes host port 15432
instead; set `LOCALNET_POSTGRES_CONTAINER_NAME` / `LOCALNET_POSTGRES_HOST_PORT`
to change either. In-network connectivity is unaffected — services resolve the
database by compose service name. Requires docker compose >= 2.24 (`!override`).

See [Topology](#topology) above for the six-participant layout this profile
runs. Auth is LocalNet's documented "unsafe" shared-secret HS256 mode (never
valid against a real participant); the CLI mints tokens for
`ledger-api-user` automatically, on every participant.

Amulet (Canton Coin) does not implement `BurnMintFactory`, so only the
lock/unlock path is exercisable against it directly — see
[Real Amulet (`amulet`)](#real-amulet-amulet) for the real (not mock)
registry client this profile enables.

**Verification status:** the sandbox profile runs in CI on every push and pull
request (the `playground CLI (go vet/test + sandbox e2e)` job in
[`.github/workflows/canton.yml`](../../../.github/workflows/canton.yml)). The
LocalNet profile's distinguishing surface — unsafe-JWT auth, DAR vetting
against a real DSO topology, `CanActAs` grants, real Amulet (tap,
`TransferPreapproval`, the transfer-instruction registry), and the real Ledger
API v2 update stream — cannot run in that job, so it is covered separately by
the `playground CLI (LocalNet e2e)` job in the same workflow (weekly schedule
plus manual `workflow_dispatch`, not a PR gate, because the stack is too heavy
to run on every push). Run it locally with
`NTT_PLAYGROUND_PROFILE=localnet go test -tags e2e ./e2e -v`.

LocalNet-only behaviors to be aware of, none of which the auth-less sandbox can
show:

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
- **Never persist contract ids.** `NttManager` is consumed only by its admin
  config choices (`SetPeer`, `SetFactory`, the rare `TransferAdmin`), so its
  cid stays stable across ordinary transfer/receive traffic; `CoreState`,
  `Emitter`, `LockedLedger`, and the covering replay-trie node all still
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
  `toAmuletSeamJSON` (`transfer.go`) re-encodes before handing the token
  arguments to `Playground.Ops:transferOut`.

## MainNet path

Documented, not implemented — this CLI is devnet-only throughout.

- **Profile → real validator.** A production profile would point at your own
  participant's ledger endpoint, use OAuth client-credentials JWTs instead of
  the unsafe HS256 mode, and skip `allocateParty` (parties pre-exist, onboarded
  through the validator).
- **Genesis/governance.** `init`'s propose/accept pair
  (`Playground.Init:proposeGenesis`/`acceptGenesis`) stands in for a real
  ceremony: `guardianGovernance` becomes an external/threshold party via
  topology transactions, the guardian set is installed at genesis co-signed
  by operator + guardianGovernance, and evolves only through real
  guardian-set-upgrade governance VAAs (`SubmitGovernanceVAA`). This CLI's
  1/1 signer (`internal/guardian`) never becomes production code — a real
  guardian set is k-of-n and the keys never touch a CLI.
- **DARs.** Only the production `ntt` DAR is ever uploaded to a real network —
  never `ntt-test` (it drags in `daml-script`). Playground ops would need to
  be re-expressed against production package-ids, either via the JSON Ledger
  API/gRPC directly or a script-only companion DAR with zero templates.
- **Fees/token.** The fee instrument becomes Canton Coin; a real `Allocation`
  is drawn from the user's wallet per `Transfer`/`PublishMessage`. The manager
  already drives the CIP-0056 `TransferFactory`/`BurnMintFactory` interfaces
  directly against a real registry — only lock/unlock is exercisable against
  Amulet directly (it has no `BurnMintFactory`).
- **Disclosures.** The `queryDisclosure`/`coveringDisclosure` pattern this CLI
  uses (reading as guardianGovernance or operator, whichever party owns the
  data) is fronted, in this CLI, by a harness-grade disclosure service
  (`ntt-playground disclosure serve` — see
  [Disclosure service](#disclosure-service)). A production deployment needs
  the hardening that service deliberately skips: mTLS or OAuth
  client-credentials auth, per-consumer (not global) allow-lists, rate
  limiting, and an audit log, as described in the core bridge's README.

## Follow-ups

Out of scope for this CLI, listed here rather than silently dropped:

- Guardian-set-upgrade governance signing (`guardian sign-governance` covers
  the Core `SetMessageFee` action and NTT's `AcceptAdminTransferToGovernance`
  action only; rotating the guardian set itself is not implemented).
- Demoting `gg` by governance VAA (`admin accept-gg-vaa` only covers an
  ordinary admin handing the role TO `gg`; `gg` handing it back still needs
  the direct `TransferAdmin` path — see the core NTT README's follow-ups).
