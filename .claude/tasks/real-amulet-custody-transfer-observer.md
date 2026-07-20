# Real Canton Coin (Amulet) CIP-56 custody transfer + real stream observer

Status: PLANNING COMPLETE — all research resolved; ready for implementation. No code changes yet.

## Goal

On Splice **LocalNet** (real DSO + real Amulet/Canton Coin), stand up a CIP-56
**custody** NTT deployment backed by *real* Canton Coin, perform a full outbound NTT
`transfer` of **1 CC** to another network, and prove the guardian **observation
actually occurs** by streaming it off a **real Ledger API v2 update stream** as the
`guardianObserver` party — not by recomputing the payload.

The receiving network is out of scope; we stop at the observed `WormholeMessage`.

## Hard constraints (established during research)

1. **Custody / lock-unlock only.** Amulet does **not** implement `BurnMintFactory`
   (`canton/testing/cli/README.md:194-197,274-275`). The token kind must be CIP-56
   custody (`Cip56CustodyToken`, `canton/ntt-cip56/daml/Wormhole/Ntt/TokenCip56.daml`).
2. **LocalNet required.** Bundle extracted at
   `/Users/smurf/WormholeLabs/Canton/splice-node/docker-compose/localnet`
   (from `0.6.12_splice-node.tar.gz`; the full tar with `splice-node/dars/` is at
   `/Users/smurf/WormholeLabs/Canton/0.6.12_splice-node.tar.gz`). `LOCALNET_DIR` must be
   exported at run time; the test boots the stack via the CLI's `network up`
   (`docker compose up -d --wait`).
3. **`ntt-test` is uploadable to LocalNet** (devnet), so new on-ledger mutations are
   Daml Script functions in `Playground.*`. Only MainNet forbids `ntt-test`.
4. **The custody Daml needs no changes for lock/unlock.** `Cip56CustodyToken` takes the
   transfer-factory cid fresh per call (`registryCid`), calls the real
   `TransferFactory_Transfer`, and aborts unless settlement is synchronous `Completed`.
5. **The CLI's ledger chokepoint stays `dpm script`** (`internal/ledger/script.go:87`)
   for everything that *submits*. The one new non-script surface is the **read-only**
   observer stream (JSON Ledger API v2), plus the existing JSON-API admin calls
   (`/v2/users`, `/v2/packages`) already in `internal/network`.

## What already exists (reduces work)

- `Cip56CustodyToken` production-shaped custody hook — `TokenCip56.daml` (no changes).
- LocalNet lifecycle + DSO party lookup — `internal/network/localnet.go` (`Up()`,
  `waitReady`, `DSOPartyID()` at `:297`, `MintUnsafeToken`, `UploadDAR`,
  `GrantLedgerAPIUserRights`). App-provider participant: gRPC 3901, **JSON API v2
  3975**, validator API **3903**; unsafe HS256 auth, secret `"unsafe"`, aud
  `https://canton.network.global`.
- Daml-Script shell-out chokepoint — `internal/ledger/script.go:87` (`Runner.Run`).
- `AmuletAvailable` profile flag — `internal/profile/profile.go:41` (true for LocalNet,
  currently read by nothing).
- The mock custody path as a template — `Test/TestNtt.daml:200` (`mkCustodyToken`),
  `Playground/Deploy.daml:46-51`, `Playground/Ops.daml` (`resolveTokenSeam:152`,
  `fundUser:104`, `transferOut:194`).
- The wire contract for the observer — vendored `wormhole-core-0.2.0`
  (`Wormhole.Core.State`): `Emitter` is `signatory operator, owner` with
  `observer guardianObserver`; `PublishMessage` is a **consuming** choice whose result
  is `WormholeMessage {registrar, owner, emitterId, sequence, nonce, consistencyLevel,
  payload}` (source: `wormholelabs-xyz/wormhole` tag `canton-core-v0.2.0`,
  `canton/core/daml/Wormhole/Core/State.daml:487-575`). `payload` is `Bytes = Text`
  (hex). Emitter address = `keccak256("wormhole:emitter:v1" ‖ lp(registrar) ‖
  lp(owner) ‖ be8(emitterId))`, already mirrored in Go:
  `internal/wire/ntt.go:189` `DerivedAddress` + `EmitterAddressTag`.
- `guardianObserver` party allocated at `init` and persisted
  (`internal/state/state.go:64`, `Playground/Init.daml:31`).

## What is missing (the actual work)

A. No real-Amulet token kind; deploy hard-wires mock factories
   (`Playground/Types.daml:38-48`, `Playground/Deploy.daml:46-51`).
B. `AmuletAvailable` gates nothing.
C. No off-ledger transfer-factory cid + choice-context + disclosures resolver.
D. `ExtraArgs` is always `emptyExtraArgs`; real Amulet needs the registry's
   `ChoiceContext` + disclosed contracts (AmuletRules, OpenMiningRound,
   TransferPreapproval, factory).
E. No real Amulet funding (tap) or wallet-user onboarding.
F. No real `TransferPreapproval` for the custody party.
G. No real `Holding` enumeration for Amulet.
H. No real stream observer.

## Resolved findings (was: Open questions) — with evidence

All Splice findings verified against the **actual 0.6.12 tag** of
`github.com/hyperledger-labs/splice` (now `canton-network/splice`) and the LocalNet
bundle's static files; Canton findings against the **release-line-3.5** specs —
Splice 0.6.12 pins **Canton 3.5.8** (`nix/canton-sources.json@0.6.12`), so LocalNet's
JSON Ledger API has the 3.5 shape.

### 1. Canton Coin `InstrumentId`

`InstrumentId { admin = <DSO party>, id = "Amulet" }`. Defined in
`daml/splice-amulet/daml/Splice/Amulet/TokenApiUtils.daml:27-38` (tag 0.6.12):
`amuletInstrumentIdName = "Amulet"`. `TransferFactory_Transfer` rejects any other
instrument (`ExternalPartyAmuletRules.daml:360-364`). DSO party comes from
`GET http://localhost:3903/api/validator/v0/scan-proxy/dso-party-id` →
`{"dso_party_id": "DSO::1220..."}` (already implemented:
`internal/network/localnet.go:297 DSOPartyID`).

### 2. Tap (devnet mint)

- `POST http://localhost:3903/api/validator/v0/wallet/tap`
- Auth: `Authorization: Bearer <HS256 unsafe JWT>` with **`sub` = the wallet user**
  whose primary party receives the mint, `aud = https://canton.network.global`.
- Body (`TapRequest`): `{"amount": "<decimal string>", "command_id": "<optional>"}`.
- Response (`TapResponse`): `{"contract_id": "<amulet cid>"}`.
- **The amount is denominated in USD**, not CC: the handler divides by the current
  open round's `amuletPrice` (`apps/wallet/.../HttpWalletHandler.scala` ~line 670)
  before exercising `AmuletRules_DevNet_Tap` (devnet-only, single-tap cap 1e8).
  LocalNet leaves `SPLICE_APP_SV_INITIAL_AMULET_PRICE` unset (bundle
  `conf/splice/sv/app.conf:141-148`) — do not hard-assume a price; tap a generous
  USD amount and assert holdings cover 1 CC (see Phase 2).
- Evidence: `apps/wallet/src/main/openapi/wallet-internal.yaml` @0.6.12 (path line 42,
  `TapRequest`/`TapResponse` lines 1409-1426);
  `daml/splice-amulet/daml/Splice/AmuletRules.daml:356-392`.

### 3. TransferPreapproval creation

- `POST http://localhost:3903/api/validator/v0/wallet/transfer-preapproval` — **no
  request body**; same wallet-user JWT auth as tap. The receiver is the
  **authenticated user's primary party**; provider = the validator operator party;
  the validator's automation converts the created `TransferPreapprovalProposal` into
  the `TransferPreapproval` and the **validator pays the creation fee** (handler
  blocks/retries; `HttpWalletHandler.scala:718-805`).
- Response: `{"transfer_preapproval_contract_id": "<cid>"}`. `409` = already exists
  (same shape — treat as success); `429` = in flight (retry with backoff).
- Consequence: **the custody party must be a validator wallet user's primary party**
  (not a bare script-allocated party). See "Party & user model" below.
- LocalNet pauses `ExpireTransferPreapprovalsTrigger`
  (bundle `conf/splice/sv/app.conf` automation.paused-triggers), so the preapproval
  never expires mid-test.
- Evidence: `wallet-internal.yaml` @0.6.12 lines 527-552.

### 4. Transfer-factory registry endpoint

- Canonical (authenticated, via the validator's scan-proxy — any valid JWT):
  `POST http://localhost:3903/api/validator/v0/scan-proxy/registry/transfer-instruction/v1/transfer-factory`
  (`ValidatorApp.scala:987-1078` mounts the generated resource under
  `api/validator/v0/scan-proxy` with `AuthenticationOnlyAuthExtractor`).
  Also served unauthenticated by scan at
  `http://scan.localhost:4000/registry/transfer-instruction/v1/transfer-factory`
  (nginx `conf/nginx/sv.conf:40-42` → `splice:5012/registry`; scan's 5012 is NOT
  published directly to the host). **Use the scan-proxy** — the CLI already talks to
  :3903 with bearer tokens.
- Request (`GetFactoryRequest`):
  `{"choiceArguments": {...}, "excludeDebugFields": true}` where `choiceArguments`
  is the **Daml-JSON encoding of the full `TransferFactory_Transfer` arguments**
  with `extraArgs.context` and `extraArgs.meta` set to the empty object.
- Response (`TransferFactoryWithChoiceContext`):
  ```json
  {
    "factoryId": "<cid>",
    "transferKind": "self" | "direct" | "offer",
    "choiceContext": {
      "choiceContextData": { ...opaque Daml-JSON ChoiceContext... },
      "disclosedContracts": [
        {"templateId": "pkgid:Module:Entity", "contractId": "...",
         "createdEventBlob": "<base64>", "synchronizerId": "..."}
      ]
    }
  }
  ```
- Scan chooses `transferKind` (`HttpTokenStandardTransferInstructionHandler.scala:82-96`):
  `self` if receiver == sender, **`direct` if the receiver has a `TransferPreapproval`**
  (the preapproval rides in `choiceContextData` under key `"transfer-preapproval"` and
  in `disclosedContracts`, alongside AmuletRules + OpenMiningRound), else `offer`.
  **Assert `transferKind == "direct"` in Go before submitting** — anything else would
  settle `Pending` and the on-ledger custody hook would abort.
- Evidence: `token-standard/splice-api-token-transfer-instruction-v1/openapi/transfer-instruction-v1.yaml`
  @0.6.12; `apps/validator/src/main/openapi/scan-proxy.yaml` @0.6.12.

### 5. Synchronous `Completed` settlement — guaranteed by preapproval

From `daml/splice-amulet/daml/Splice/ExternalPartyAmuletRules.daml`
`amulet_transferFactoryV1_transferImpl` (lines 340-445, @0.6.12): when the
`transfer-preapproval` cid is present in `extraArgs.context`, the factory exercises
`TransferPreapproval_SendV2` **in the same transaction** and returns
`TransferInstructionResult_Completed` — exactly what `Cip56CustodyToken.lockOrBurnImpl`
requires. Without a preapproval it returns `Pending` (two-step offer/accept) and the
hook aborts — that is the desired failure mode, and the Phase-0 test pins it.

Mining rounds: rounds 0 and 1 are created at DSO bootstrap with `opensAt = now`
(`AmuletRules.daml` `AmuletRules_Bootstrap_Rounds`), so an open round exists
essentially as soon as `network up`'s readiness gate passes; the only transient
failure mode is scan/validator stores not yet ingested ("no open mining round") —
**retry tap/transfer-factory calls with backoff for up to ~2 min**. Default
`initialTickDuration` is 10 min (`SpliceUtil.scala:174`) and round 0 stays open
~24 h, so context staleness within a test run is a non-issue; still re-fetch the
context on a submit failure and retry once (cheap insurance).

### 6. Holdings enumeration

Not an HTTP problem at all: the sender/custody parties see their own Amulet contracts
on their ACS, and `ntt-test` already data-depends on `splice-api-token-holding-v1`.
**Enumerate in Daml Script** with `queryInterface @Holding party`, filtering
`instrumentId == amuletInstrument && lock == None && owner == party`. (The wallet
endpoints `GET /v0/wallet/balance` and `GET /v0/wallet/amulets` exist but are not
needed.) Token-standard docs recommend the equivalent Ledger-API interface query
(`docs/src/app_dev/token_standard/index.rst:168-198`).

### 7. Ledger API v2 update stream — JSON API is sufficient (no gRPC needed)

- **`GET /v2/updates` over WebSocket** on the app-provider JSON API
  (`ws://localhost:3975/v2/updates`) is the real stream. The HTTP
  `POST /v2/updates` twin is a bounded **blocking list call** (same request/response
  shapes, `limit` + `stream_idle_timeout_ms` query params) — it is the documented
  fallback if WS handshake trouble appears, with zero new dependencies.
  `/v2/updates/flats|trees` are deprecated ("removed in 3.5.0, use v2/updates").
- WS auth: two `Sec-WebSocket-Protocol` subprotocols: `daml.ws.auth` and
  `jwt.token.<jwt>` (verbatim from the Canton 3.5 JSON API docs; openapi
  `securitySchemes` names `Sec-WebSocket-Protocol`).
- Request (`GetUpdatesRequest`, one JSON frame after connect; 3.5 requires
  `beginExclusive`):
  ```json
  {
    "beginExclusive": <int64 offset>,
    "updateFormat": {
      "includeTransactions": {
        "eventFormat": {
          "filtersByParty": {
            "<guardianObserver party>": {
              "cumulative": [
                {"identifierFilter": {"TemplateFilter": {"value": {
                  "templateId": "#wormhole-core:Wormhole.Core.State:Emitter",
                  "includeCreatedEventBlob": false}}}}
              ]
            }
          },
          "verbose": false
        },
        "transactionShape": "TRANSACTION_SHAPE_LEDGER_EFFECTS"
      }
    }
  }
  ```
  Package-name format (`#wormhole-core:...`) is accepted for `templateId` — no
  package-id computation needed. Per the `EventFormat` spec, under LEDGER_EFFECTS
  **template filters match exercised events** whose witnesses include the party
  ("for ledger-effects create and exercise events are returned, for which the
  witnesses include at least one of the listed parties and match the per-party
  filter") — and `guardianObserver` is a contract observer of `Emitter`, hence a
  witness of the consuming `PublishMessage` exercise.
- Response frames: `{"update": {"Transaction": JsTransaction}}` (or
  `OffsetCheckpoint`/`Reassignment`/`TopologyTransaction` — skip). `JsTransaction`:
  `{updateId, commandId, workflowId, effectiveAt, events[], offset, synchronizerId,
  recordTime, ...}`; `events[]` items are `{"ExercisedEvent": {...}}` /
  `{"CreatedEvent": {...}}`. `ExercisedEvent` fields (verbatim): `offset`, `nodeId`,
  `contractId`, `templateId`, `interfaceId?`, `choice`, `choiceArgument`,
  `actingParties[]`, `consuming`, `witnessParties[]`, `lastDescendantNodeId`,
  **`exerciseResult`**, `packageName`, `implementedInterfaces[]?`, `acsDelta` (3.5).
  Match `choice == "PublishMessage" && consuming == true`; decode `exerciseResult`
  as the `WormholeMessage` record. **Int64 rendering caveat:** decode
  `sequence`/`emitterId`/`nonce` via `json.Number` (accept number or string).
- Start offset: `GET /v2/state/ledger-end` → `{"offset": <int64>}`; feed as
  `beginExclusive`. A fresh LocalNet is unpruned, so replaying from an
  earlier recorded offset is also valid.
- Evidence: canton repo `release-line-3.5`
  `community/ledger/ledger-json-api/src/test/resources/json-api-docs/{openapi,asyncapi}.yaml`;
  `com/daml/ledger/api/v2/transaction_filter.proto`; archived 3.5 JSON-API docs
  (docs.digitalasset.com restructure 404s the live URLs).
- **gRPC fallback (not planned, documented only):** official Go bindings exist at
  `github.com/digital-asset/dazl-client/v8` v8.9.0
  (`.../go/api/com/daml/ledger/api/v2` includes `UpdateService/GetUpdates` +
  `TransactionShape`), pinning `google.golang.org/grpc v1.73.0`,
  `google.golang.org/protobuf v1.36.6`. Only if the JSON path fails live.

### 8. Party & user model (who is who)

| Role | Identity | Why |
| --- | --- | --- |
| operator / guardianGovernance / guardianObserver | script-allocated (existing `init`) | unchanged |
| deployment admin | script-allocated (existing `deploy`) | signs `Cip56CustodyToken`; unchanged |
| **sender** (`transfer --user`) | **validator wallet user** (hint = wallet user id, lowercase) | only wallet users can be tapped |
| **custody** | **validator wallet user** `<deployment>-custody` | only wallet users can hold a `TransferPreapproval` (endpoint targets the authenticated user's primary party) |
| observer stream reader | new ledger user `guardian-watcher` with **only `CanReadAs(guardianObserver)`** | proves observation with observer rights only |

Wallet-user onboarding (**resolved**, `validator-internal.yaml` @0.6.12 line 29,
`HttpValidatorHandler.scala:35-62` → `ValidatorUtil.onboard`):
`POST http://localhost:3903/api/validator/v0/register` with the **new user's own
JWT** (`MintUnsafeToken(sub = <user id>)`; ledger-user name = the JWT `sub`; no
operator involvement, no onboarding secret — the "onboarding secrets" in
`env/splice.env` onboard the validator NODE to the SV, not users). Body: empty
(`RegistrationRequest` is a nullable object — send `{}`). Response:
`{"party_id": "<party>"}`. Idempotent (`getOrAllocateParty` reuses the existing
primary party). Party hint is the sanitized user name
(`SpliceLedgerConnection.sanitizeUserIdToPartyString`, keeps `[a-zA-Z0-9:- ]`), so
user `cc-sender` → party `cc-sender::<fingerprint>`. Admin alternative (not needed):
`POST /v0/admin/users` `{"name", "party_id"?}` with the operator's JWT. Onboarding
allocates the party, creates the ledger user with that primary party, grants the
**validator operator** actAs, and creates the wallet install contracts —
`ledger-api-user` (the `dpm script` user) still needs its own `CanActAs` on the new
party via `network.GrantLedgerAPIUserRights` (existing) before any script submits
as it.
Belt-and-braces: tap the **validator's own wallet** (user `app-provider`,
pre-onboarded — `AUTH_APP_PROVIDER_WALLET_ADMIN_USER_NAME=app-provider` in
`env/app-provider-auth-on.env`) before requesting the preapproval, since the
validator pays the preapproval creation fee.

### 9. Disclosures + choice context through `dpm script`

The registry's `disclosedContracts` must reach the submission. Daml Script 3.5.1's
`Disclosure` is `{templateId : TemplateTypeRep, contractId : ContractId (), blob :
Text}` (`Daml.Script.Internal.Questions.Commands:91`); `Daml.Script` exports the
**type but not the constructor**. Two equivalent construction routes, in preference
order (pick whichever compiles — 5-minute check):

1. `import Daml.Script.Internal.Questions.Commands (Disclosure (..))` — works iff
   daml-script exposes that module to dependents.
2. **Record-update trick** (no constructor needed): take any `queryDisclosure`-obtained
   seed (e.g. the token hook's own disclosure, already in scope in `transferOut`) and
   `seed with templateId = ..., contractId = coerceContractId cid, blob = b` —
   record update needs only the `HasField` instances, which are always exported.

Either way a `TemplateTypeRep` per disclosed contract is required, so **vendor
`splice-amulet-0.1.22.dar`** (see supply-chain section) as a data-dependency of
`ntt-test` only, and map the registry's `templateId` text by `Module:Entity` suffix:

| registry templateId suffix | vendored type for `templateTypeRep @T` |
| --- | --- |
| `Splice.AmuletRules:AmuletRules` | `Splice.AmuletRules.AmuletRules` |
| `Splice.AmuletRules:TransferPreapproval` | `Splice.AmuletRules.TransferPreapproval` |
| `Splice.Round:OpenMiningRound` | `Splice.Round.OpenMiningRound` |
| `Splice.ExternalPartyAmuletRules:ExternalPartyAmuletRules` (the factory) | `Splice.ExternalPartyAmuletRules.ExternalPartyAmuletRules` |
| `Splice.Amulet:FeaturedAppRight` (maybe) | `Splice.Amulet.FeaturedAppRight` |

Abort with the unmapped templateId text on anything else, so a live run reveals any
missing row loudly. Verified compatible: `splice-amulet-0.1.22.dar` embeds the SAME
interface package-ids the repo already vendors (`holding-v1 718a0f77…`,
`transfer-instruction-v1 55ba4deb…`, `metadata-v1 4ded6b66…` — checked against the
bundle), so damlc dedupes cleanly; LocalNet runs `LATEST_PACKAGES_ONLY=true` so live
contracts are created from 0.1.22 (== `splice-amulet-current.dar`, sha-identical).
Canton 3.x package-name upgrade resolution tolerates minor version drift anyway.

`choiceContextData` is the Daml-JSON encoding of
`Splice.Api.Token.MetadataV1.ChoiceContext` — type the script-input field as
`ChoiceContext` and let `dpm script`'s decoder consume the registry JSON verbatim
(both sides speak Daml-JSON; V3 in the verification table).

## Design decisions

- **Sequence-critical flow** (one e2e transfer):
  1. `network up` + `init` (existing).
  2. `deploy --config testdata/deploy-cip56-custody.json` → onboards custody wallet
     user, creates `TransferPreapproval`, deploys `Cip56CustodyToken` with
     `instrumentId = {DSO, "Amulet"}`.
  3. `transfer --deployment cc-custody --user cc-sender --amount 10000000000 --sign`
     → onboards + taps sender, fetches factory + context via scan-proxy, runs
     `Playground.Ops:transferOut` with the real seam.
  4. `observe stream --deployment cc-custody --from-offset <pre-transfer ledger end>`
     → reads the WS stream as `guardian-watcher` (readAs guardianObserver only),
     prints the observed message as JSON.
- **Amounts.** Amulet has 10 decimals → deploy config `decimals: 10`. NTT trims to 8
  wire decimals (`TrimDecimals`), so 1 CC = raw `10_000_000_000` at token scale =
  wire amount `100_000_000` at 8 decimals; `trimmedToDecimal` hands the factory
  exactly `1.0`. Tap `"100"` USD (price-independent cushion; devnet default price
  makes this ≫ 1 CC — do not assert the CC amount of the tap itself).
- **The recompute path stays** — `transferOut`'s output is still the recomputed
  payload; the observer's streamed payload must equal it bit-for-bit (that equality
  is the headline assertion).
- **`observe` grows a `stream` subcommand** (cobra parent keeps its existing leaf
  behavior; `observe stream ...` is matched first as a subcommand). No breakage of
  the existing e2e `observe` subtest.
- **New Go dep for WS**: `github.com/gorilla/websocket v1.5.3` (zero transitive
  deps). If handshake/subprotocol trouble appears live, drop to `POST /v2/updates`
  (blocking list, `endInclusive` = post-transfer ledger end) — same shapes, no dep.

## Plan (phased; each phase ends compilable + tests green)

### Phase 0 — Acceptance test first (per repo convention)

**DONE**: added the e2e subtest (`canton/testing/cli/e2e/playground_e2e_test.go`, after
"contracts list reflects deployments") plus a sandbox-viable gating-error subtest
("cip56-custody is rejected without real Amulet"); added
`canton/testing/cli/testdata/deploy-cip56-custody.json`; added
`canton/testing/cli/internal/observer/observer_test.go` (fixture-driven `DecodeUpdateFrame`
tests: happy path, json.Number-vs-string tolerance, derived-address cross-check against
`internal/wire.DerivedAddress`, non-PublishMessage/non-Transaction frames skipped) and
`canton/testing/cli/internal/amulet/amulet_test.go` (httptest: tap body/response + retry on a
mining-round-lag-shaped 503, preapproval 409-as-success + 429 retry, transfer-factory request
shape + disclosedContracts decode + non-"direct" rejection, wallet-user onboarding
request/response). Verified all four are in the expected red state:
`go vet ./...` fails on `undefined: DecodeUpdateFrame` / `undefined: Client`; `go vet -tags e2e
./e2e/...` fails on `d.CustodyParty undefined` (Phase 1 hasn't landed yet). Also copied
`canton/dars/splice-amulet-0.1.22.dar` in (sha256 verified — see "Verify-live results" above)
ahead of Phase 1 wiring it into `daml.yaml`. Vendored `splice-amulet-0.1.22.dar` and confirmed
its DSO/tap/preapproval/transfer-factory live shapes against the booted LocalNet before writing
any Go/Daml code that depends on them (see "Verify-live results" section above, inserted before
the original "Verify-at-implementation table").

**File: `canton/testing/cli/e2e/playground_e2e_test.go`** — add subtest
`real amulet cip56-custody: transfer 1 CC observed on stream`, placed AFTER
`"contracts list reflects deployments"` (so its manager-count assertion of 3 stays
valid) and before `"network status..."`. Body:

```go
t.Run("real amulet cip56-custody: transfer 1 CC observed on stream", func(t *testing.T) {
    if playgroundProfile != "localnet" { t.Skip("real Amulet requires the localnet profile") }
    // 1. deploy: onboards custody wallet user + TransferPreapproval + real token hook
    out := h.mustRun("deploy", "--config", testdataPath("deploy-cip56-custody.json"))
    s := h.loadState(); d, ok := s.Deployment("cc-custody")
    require.True(t, ok)
    require.Equal(t, "cip56-custody", d.TokenKind)
    require.NotEmpty(t, d.CustodyParty)
    require.Contains(t, d.InstrumentAdmin, "DSO::", "instrument admin must be the real DSO party")

    // 2. record the pre-transfer ledger end (printed by the new offset helper)
    out = h.mustRun("observe", "stream", "--deployment", "cc-custody", "--print-offset")
    fromOffset := extractField(t, out, "ledgerEnd")

    // 3. transfer 1 CC (raw 10^10 at 10 decimals). Auto: onboard+tap sender, factory+context fetch.
    out = h.mustRun("transfer", "--deployment", "cc-custody",
        "--user", "cc-sender", "--chain", "2",
        "--recipient-address", "00..ee", "--amount", "10000000000", "--sign")
    payloadHex := extractField(t, out, "payload")
    seq := extractField(t, out, "sequence")           // recomputed side
    require.True(t, strings.HasPrefix(payloadHex, "9945ff10"))

    // 4. the observation: read the real update stream as guardian-watcher (readAs only)
    out = h.mustRun("observe", "stream", "--deployment", "cc-custody",
        "--from-offset", fromOffset, "--count", "1", "--timeout", "3m")
    var observed []struct {
        EmitterChain     int    `json:"emitterChain"`
        EmitterAddress   string `json:"emitterAddress"`   // derived: keccak(tag‖registrar‖owner‖emitterId)
        Sequence         int    `json:"sequence"`
        Nonce            int    `json:"nonce"`
        ConsistencyLevel int    `json:"consistencyLevel"`
        Payload          string `json:"payload"`
        EffectiveAt      string `json:"effectiveAt"`      // watcher timestamp source
        UpdateID         string `json:"updateId"`
    }
    require.NoError(t, json.Unmarshal([]byte(stripVerbose(out)), &observed))
    require.Len(t, observed, 1)
    m := observed[0]
    require.Equal(t, 72, m.EmitterChain)
    require.Equal(t, d.TransceiverAddress, m.EmitterAddress,
        "derived emitter address must be this deployment's transceiver")
    require.Equal(t, seq, strconv.Itoa(m.Sequence), "streamed sequence == recomputed sequence")
    require.Equal(t, payloadHex, m.Payload, "streamed payload == recomputed payload (bit-exact)")
    require.NotEmpty(t, m.EffectiveAt)
    // decode the NTT payload off the STREAMED bytes: amount 1 CC at wire scale
    w := wire.DecodeWormholeTransceiverMessage(mustHex(t, m.Payload))
    n := wire.DecodeNativeTokenTransfer(wire.DecodeNttManagerMessage(w.ManagerPayload).Payload)
    require.Equal(t, uint64(100_000_000), n.Amount) // 1 CC at 8 wire decimals
    require.Equal(t, uint8(8), n.Decimals)
    require.Equal(t, uint16(2), n.RecipientChain)

    // 5. custody actually holds the locked 1.0 CC
    out = h.mustRun("balance", "--party", "cc-custody-party", "--deployment", "cc-custody")
    require.Equal(t, "1.0000000000", extractField(t, out, "amuletHoldingTotal"))
})
```

(Exact flag/field names may be tuned during implementation; the assertions are the
contract: real deploy identity, stream-vs-recompute equality on sequence + payload,
NTT payload decode of 1 CC, emitter-address derivation, custody holding, and the
skip-gate on non-localnet profiles. `--print-offset` mode only prints
`ledgerEnd=<n>` and exits.)

Also in Phase 0: no harness changes needed — the suite already boots LocalNet when
`NTT_PLAYGROUND_PROFILE=localnet` + `LOCALNET_DIR` are set, and `newHarness` tears
down. The subtest fails at step 1 until Phases 1-4 land.

Go unit tests written in this phase too (they fail/skip until their packages exist):
- `internal/observer/observer_test.go` — decode a canned `JsGetUpdatesResponse`
  fixture (Transaction + ExercisedEvent with `exerciseResult`), assert
  `WormholeMessage` extraction, `json.Number` tolerance, derived address equality
  with `wire.DerivedAddress`.
- `internal/amulet/amulet_test.go` — `httptest.Server` round-trips: tap body/response,
  preapproval 409-as-success, transfer-factory request contains
  `choiceArguments.transfer.instrumentId.id == "Amulet"`, response decode incl.
  `disclosedContracts`, `transferKind != "direct"` rejection, retry-on-503.

### Phase 1 — Real `cip56-custody` token kind (Daml deploy path + gating)

**DONE**: implemented all 11 items. Daml: `Cip56Custody` added to `Playground.Types.TokenKind`;
`Playground.Deploy.DeployInput` gained `custody`/`instrumentAdmin : Optional Party` and a new
branch creating a plain `Cip56CustodyToken` (no mock factory); `Playground.Ops.resolveTokenSeam`
gained a `Cip56Custody` case (`registryCid = None`, token disclosure only) and `receiveVaa` now
guards this kind with an explicit `abort` before any submit (needed `import DA.Action (when)` --
`when` is not in Daml's default Prelude scope, a build error caught this immediately);
`Playground.Query` gained `amuletBalance` (`queryInterface @Holding`, filtered by
instrument/lock/owner). Go: `deploy.go` requires `prof.AmuletAvailable` and `mode ==
"lock-unlock"` for this kind, resolves the DSO via `LocalNetManager.DSOPartyID`, and runs the
full custody-onboarding sequence (onboard wallet user → grant actAs → tap the validator's own
wallet → create TransferPreapproval) via the Phase-2 `internal/amulet` client BEFORE the deploy
script call; `state.Deployment` gained `CustodyParty`/`CustodyUser`/`InstrumentAdmin`; `balance`
(`query.go`) branches on `TokenKind == "cip56-custody"` to call `amuletBalance` instead of
`balances`, and a new `resolvePartyOrCustody` helper resolves a deployment's own
`"<name>-custody"` hint directly from state (it's a wallet user's real party, never one
allocated by `allocatePlaygroundParty`) instead of trying to allocate a fresh one.
`canton/dars/README.md` and `canton/test/daml.yaml` updated per the supply-chain section.

Verified: `dpm build --all` green (only an expected "unused dependency" warning for
`splice-amulet` until Phase 3's `Playground.Amulet` module uses it); `dpm test --all` in
`canton/test` green (all pre-existing Daml unit tests pass unchanged); the FULL sandbox e2e
suite (`go test -tags e2e ./e2e -run TestPlaygroundE2E -v -timeout 20m`, no `LOCALNET_DIR`)
passes end to end (574s) — every pre-existing subtest green, the new real-Amulet subtest
self-skips ("real Amulet requires the localnet profile"), and the new Phase-1 gating subtest
("cip56-custody is rejected without real Amulet") passes. `go build ./...` and `go vet
./internal/...` clean except the still-expected `internal/observer` red state (Phase 4).

1. **`canton/dars/splice-amulet-0.1.22.dar`** (new, vendored binary): extract from
   `/Users/smurf/WormholeLabs/Canton/0.6.12_splice-node.tar.gz` path
   `splice-node/dars/splice-amulet-0.1.22.dar`. sha256
   `bcfcd6a9250172a8384d1326a2990b374c66a367ca7d4120b243aecfd372761e` (verified;
   byte-identical to `splice-amulet-current.dar` in the same bundle).
2. **`canton/dars/README.md`**: provenance row + rationale ("consumed by `ntt-test`
   only, for `TemplateTypeRep`s of disclosed Amulet contracts; never a dependency of
   production DARs").
3. **`canton/test/daml.yaml`**: add `- ../dars/splice-amulet-0.1.22.dar` to
   `data-dependencies`.
4. **`canton/test/daml/Playground/Types.daml`**: add `Cip56Custody` to `TokenKind`;
   `parseTokenKind "cip56-custody" = Cip56Custody`.
5. **`canton/test/daml/Playground/Deploy.daml`**:
   - `DeployInput` gains `custody : Optional Party`, `instrumentAdmin : Optional Party`
     (Go always includes the fields; `null` for mock kinds).
   - New branch:
     ```daml
     Cip56Custody -> do
       let custody = fromSomeNote "playground: cip56-custody needs a custody party" input.custody
           dso     = fromSomeNote "playground: cip56-custody needs the DSO party" input.instrumentAdmin
       cid <- submit admin do
         createCmd Cip56CustodyToken with
           admin, manager = input.operator, custody
           instrumentId = InstrumentId with admin = dso, id = "Amulet"
       pure (toInterfaceContractId @NttToken cid)
     ```
     No mock factory. (`Cip56CustodyToken` is `signatory admin` — a plain create.)
6. **`canton/test/daml/Playground/Ops.daml`**: `resolveTokenSeam` gets a
   `Cip56Custody` case returning `(tokenCid, None, [dTok])` (token disclosure only —
   the real factory cid/disclosures arrive per-call in Phase 3). `receiveVaa` with
   this kind therefore aborts on the missing registry cid — acceptable: receiving is
   out of scope; make the error text explicit
   (`"cip56-custody receive/unlock not supported by the playground"` via a guard in
   `receiveVaa` before submit).
7. **`canton/test/daml/Playground/Query.daml`**: add
   `amuletBalance : AmuletBalanceInput -> Script AmuletBalanceOutput` —
   `queryInterface @Holding input.owner`, sum unlocked amounts for
   `InstrumentId input.instrumentAdmin "Amulet"`. Records:
   `AmuletBalanceInput {owner : Party, instrumentAdmin : Party}`,
   `AmuletBalanceOutput {amuletHoldingTotal : Decimal}`.
8. **Go — `cmd/ntt-playground/deploy.go`**:
   - `deployInput` gains `Custody *string \`json:"custody"\`` and
     `InstrumentAdmin *string \`json:"instrumentAdmin"\`` (pointers so mock kinds
     marshal `null`).
   - `tokenKind == "cip56-custody"`: require `prof.AmuletAvailable`
     (error: `"cip56-custody requires a profile with real Amulet (localnet)"` —
     **this makes `AmuletAvailable` live**); resolve DSO via
     `network.LocalNetManager.DSOPartyID` (construct with `ComposeDir` from the
     existing network-cmd plumbing in `network.go`, or lift `DSOPartyID` to take just
     the base URL); onboard the custody wallet user + preapproval via the Phase-2
     `internal/amulet` client (deploy step order: onboard → grant actAs → tap
     `app-provider` wallet → create preapproval → run deploy script); persist
     `CustodyParty`, `CustodyUser`, `InstrumentAdmin` on the state deployment.
   - Reject `mode != "lock-unlock"` for this kind (burn/mint against Amulet is
     impossible).
9. **`internal/state/state.go`**: `Deployment` gains
   `CustodyParty string \`json:"custodyParty,omitempty"\``,
   `CustodyUser string \`json:"custodyUser,omitempty"\``,
   `InstrumentAdmin string \`json:"instrumentAdmin,omitempty"\``.
10. **`canton/testing/cli/testdata/deploy-cip56-custody.json`** (new):
    ```json
    {
      "name": "cc-custody",
      "mode": "lock-unlock",
      "tokenKind": "cip56-custody",
      "decimals": 10,
      "peers": [ {"chain": 2, "manager": "00..bb", "transceiver": "00..cc"} ]
    }
    ```
    (32-byte hex strings as in the existing testdata files.)
11. **`cmd/ntt-playground/query.go`** (`balance`): for `cip56-custody`, call
    `Playground.Query:amuletBalance` with the deployment's `InstrumentAdmin`; print
    `amuletHoldingTotal=<decimal>`. Party flag accepts the custody hint
    (`<name>-custody` resolves from state's `CustodyParty`).

Deliverable: `dpm build --all` green; sandbox e2e unchanged (new kind rejected there
with the clear gating error — add a small sandbox subtest asserting exactly that
error, which is CI-viable coverage of `AmuletAvailable`).

### Phase 2 — Go Amulet client (`internal/amulet`, new package)

**DONE** (implemented before Phase 1's Go wiring, since `deploy.go`'s custody onboarding
depends on this client): `canton/testing/cli/internal/amulet/amulet.go` implements `Client`
with `OnboardWalletUser`, `Tap` (retries the round-lag failure mode), `CreateTransferPreapproval`
(409-as-success, 429 retry), and `GetTransferFactory` (fails fast on non-"direct"
`transferKind`, captures `choiceContextData` as verbatim `json.RawMessage`). All seven
`internal/amulet/amulet_test.go` cases pass. Also added `network.CreateLedgerUser` to
`internal/network/localnet.go` (`POST /v2/users`, `CanReadAs`/`CanActAs` rights, 409/already-
exists idempotent) — its `CanReadAs` wrapper shape was smoke-tested live against the booted
LocalNet (200 OK), confirming V4 in full (see "Verify-live results" above).

**File: `canton/testing/cli/internal/amulet/amulet.go`** — thin authenticated
`net/http` client; base URL = the validator (`http://localhost:3903`, override via
existing `ValidatorBaseURL` conventions); tokens minted per-user via
`network.MintUnsafeToken`. `Logf` narration like the other internal packages.

```go
type Client struct { ValidatorBaseURL string; Logf func(string, ...any) }

// OnboardWalletUser onboards user (POST /api/validator/v0/register, the new user's
// own JWT: MintUnsafeToken(sub=user), empty body {}) and returns response party_id.
// Idempotent (re-registering returns the existing primary party).
func (c *Client) OnboardWalletUser(ctx context.Context, user string) (party string, err error)

// Tap mints amulet to user's primary party. usdAmount is USD (divided by the open
// round's amulet price server-side). Retries "no open mining round" for up to 2 min.
func (c *Client) Tap(ctx context.Context, user, usdAmount string) (contractID string, err error)

// CreateTransferPreapproval creates (or finds, on 409) the user's TransferPreapproval;
// retries 429 with backoff.
func (c *Client) CreateTransferPreapproval(ctx context.Context, user string) (cid string, err error)

type DisclosedContract struct {
    TemplateID       string `json:"templateId"`
    ContractID       string `json:"contractId"`
    CreatedEventBlob string `json:"createdEventBlob"`
    SynchronizerID   string `json:"synchronizerId"`
}
type TransferFactory struct {
    FactoryID          string
    TransferKind       string            // must be "direct" for the custody lock
    ChoiceContextData  json.RawMessage   // passed VERBATIM into the script input
    DisclosedContracts []DisclosedContract
}
// GetTransferFactory posts the Daml-JSON TransferFactory_Transfer arguments to
// /api/validator/v0/scan-proxy/registry/transfer-instruction/v1/transfer-factory.
func (c *Client) GetTransferFactory(ctx context.Context, user string, args TransferArgs) (TransferFactory, error)

// TransferArgs builds choiceArguments: expectedAdmin(dso), transfer{sender, receiver,
// amount (decimal string), instrumentId{admin: dso, id: "Amulet"}, requestedAt,
// executeBefore (RFC3339, now/now+1h), inputHoldingCids: [], meta:{values:{}}},
// extraArgs{context:{values:{}}, meta:{values:{}}}. [shape: V2]
```

`GetTransferFactory` fails fast with the response body if `transferKind != "direct"`
("custody receiver has no TransferPreapproval — deploy creates it; re-run deploy or
check validator automation").

**File: `internal/network/localnet.go`** — add
`CreateLedgerUser(ctx, jsonAPIBaseURL, adminToken, userID string, readAs, actAs []string) error`
(`POST /v2/users`, treat already-exists as success; rights use the same
`{"kind": {"CanReadAs": {"value": {"party": ...}}}}` wrapper convention already
proven for `CanActAs` in `GrantLedgerAPIUserRights` — [shape: V4]).

Deliverable: unit tests from Phase 0 (`internal/amulet/amulet_test.go`) green;
`go vet`/`go test ./...` green.

### Phase 3 — Wire real inputs through the transfer path

**DONE**: added `canton/test/daml/Playground/Amulet.daml` (`DisclosedContractIn`, `AmuletSeam`,
`toDisclosure` -- route 2 from findings §9, the record-update trick on a `queryDisclosure`-
obtained seed, confirmed compiling with no constructor import needed (V6 resolved: route 2
works); `amuletHoldings`). `toDisclosure` needed `{-# LANGUAGE AllowAmbiguousTypes #-}` plus a
`HasTemplateTypeRep t` constraint on its local `mkDisclosure` helper -- `templateTypeRep`'s
type variable appears only in the constraint, not the return type, which GHC's ambiguity
check rejects by default. `Playground.Ops.transferOut` now branches on `Cip56Custody`:
`registryCid = Some seam.factoryCid`, `extraArgs` built from `seam.choiceContext`, disclosures
= token disclosure `::` mapped registry disclosures, `inputHoldingCids` resolved in-script via
`amuletHoldings` when Go sends none. `transfer.go` branches on `TokenKind == "cip56-custody"`:
onboards the sender as a wallet user (cached in `state.Users` like every other hint), taps it
(new `--tap-usd` flag, default "100", "0" skips), resolves the real transfer-factory per call,
and sends the seam through untouched (`ChoiceContext` as `json.RawMessage`). `receive.go` hard-
errors for this kind ("receiving network out of scope for real Amulet (cip56-custody)").

Verified: `dpm build --all` and `dpm test --all` green (V3 also confirmed by inspecting the
vendored `splice-api-token-metadata-v1` DAR's source -- `ChoiceContext = {values : TextMap
AnyValue}` with `AV_ContractId`/etc. variant tags -- which matches the live-captured
`choiceContextData` shape exactly, byte for byte); `go build ./...`/`go vet` clean except the
still-expected `internal/observer` red state; the FULL sandbox e2e suite re-run fresh
(`-count=1`, forcing an actual CLI rebuild+rerun rather than Go's test cache) still passes
end to end (594s), confirming the `transfer.go` rewrite didn't disturb any mock-kind path.

**Manual live smoke test against LocalNet (init → deploy → transfer → balance), before writing
Phase 4**: full round-trip succeeded end to end -- `init`, `deploy --config
deploy-cip56-custody.json` (onboards custody, taps app-provider, creates the preapproval,
deploys the token), `transfer --deployment cc-custody --user cc-sender --chain 2 --amount
10000000000 --sign` (onboards+taps sender, resolves the real transfer-factory, locks 1 CC), and
`balance --party cc-custody-custody --deployment cc-custody` → `amuletHoldingTotal=1.0000000000`
exactly. The recomputed payload decodes to amount `0x5f5e100` (100,000,000 = 1 CC at 8 wire
decimals), prefix `9945ff10`, recipient chain `0002`. Two real deviations surfaced and were
fixed here (not just theoretical -- both reproduced live, not caught by inspection):

- **V5 was WRONG, not "expected identical"**: `dpm script`'s `Disclosure.blob` field is HEX, not
  base64. Passing the registry's base64 `createdEventBlob` straight through failed with
  `com.digitalasset.daml.lf.script.converter.ConverterException: cannot parse HexString ...`.
  Fixed in `toAmuletSeamJSON` (`transfer.go`): base64-decode then hex-encode before handing the
  blob to the script. `internal/amulet` itself is unaffected -- it still returns the registry's
  base64 form verbatim; the re-encoding is a `cmd/ntt-playground`-level concern (the Daml-Script
  boundary), keeping the package layering clean.
- **Tap/onboard/preapproval/transfer-factory calls can exceed a 30s per-attempt timeout on live
  LocalNet** (observed both a plain `curl` tap taking 16.5s and the CLI's own tap hitting
  "context deadline exceeded" at 30s twice in a row) -- these are network/automation latency
  blips, not the documented "no open mining round"/429 failure shapes the original retry logic
  covered. Added `isTransientTransportError` (checks `context.DeadlineExceeded` /
  `net.Error.Timeout()`) to `internal/amulet`, wired into `Tap`, `CreateTransferPreapproval`,
  `OnboardWalletUser`, and `GetTransferFactory`'s retry loops (all bumped to a 45s per-attempt
  timeout), so a slow-but-eventually-successful attempt retries instead of failing the whole
  call. All `internal/amulet` unit tests still pass unchanged after this fix.

The live `Playground.Ops:transferOut` script call itself took 1m25s end to end (five disclosed
contracts, a large `AmuletRules` blob) -- well within `dpm script`'s own timeout, no separate
fix needed, but worth noting for anyone tuning a tighter external timeout around it.

1. **`canton/test/daml/Playground/Amulet.daml`** (new module, script-side seam):
   ```daml
   data DisclosedContractIn = DisclosedContractIn with
       templateId : Text          -- "pkgid:Module:Entity" (registry format)
       contractId : ContractId () -- decodes from the JSON contract-id string
       blob       : Text          -- createdEventBlob (base64) [shape: V5]
     deriving (Eq, Show)

   data AmuletSeam = AmuletSeam with
       factoryCid    : ContractId ()   -- registry factoryId
       choiceContext : ChoiceContext   -- decoded verbatim from choiceContextData [V3]
       disclosed     : [DisclosedContractIn]
     deriving (Eq, Show)

   -- Disclosure construction: route 1 (constructor import) or route 2 (record
   -- update on a seed) — see "Resolved findings §9". Maps templateId suffix →
   -- templateTypeRep of the vendored splice-amulet types; aborts on unknown.
   toDisclosure : Disclosure -> DisclosedContractIn -> Disclosure

   -- Unlocked Amulet holdings owned by `owner` for {dso, "Amulet"}.
   amuletHoldings : Party -> Party -> Script [ContractId Holding]
   ```
2. **`canton/test/daml/Playground/Ops.daml`**:
   - `TransferOutInput` gains `amulet : Optional AmuletSeam` (Go sends `null` for
     mock kinds).
   - `transferOut`, when `parseTokenKind input.tokenKind == Cip56Custody`:
     - require `Some seam = input.amulet`;
     - `registryCid = Some (coerceContractId seam.factoryCid)`;
     - `extraArgs = ExtraArgs with context = seam.choiceContext, meta = emptyMetadata`;
     - `inputHoldingCids`: if `input.inputHoldingCids == []`, resolve in-script via
       `amuletHoldings input.user dso` (dso read off the token contract's
       `instrumentId.admin` — query `@Cip56CustodyToken input.admin`);
     - disclosures: token disclosure (from `resolveTokenSeam`) ++
       `map (toDisclosure seed) seam.disclosed` (seed = the token disclosure);
     - everything else (peer lookup, recompute, submit-as-user) unchanged.
   - Mock kinds: `amulet` must be `None`; behavior byte-identical to today.
3. **Go — `cmd/ntt-playground/transfer.go`**:
   - `transferOutInput` gains `Amulet *amuletSeamJSON \`json:"amulet"\`` mirroring
     `AmuletSeam` (`choiceContext` as `json.RawMessage` pass-through).
   - Branch on `d.TokenKind`:
     - `"cip56-custody"` (requires `prof.AmuletAvailable`): resolve sender as wallet
       user (hint = user id; `amulet.OnboardWalletUser` → party; then
       `grantActAs(party)`; persist in `s.Users[hint]`); `Tap(sender, tapUSD)` (new
       flag `--tap-usd`, default `"100"`, `"0"` skips); `GetTransferFactory` with
       `TransferArgs{sender, receiver: d.CustodyParty, amount: formatDecimal(amount,
       10), dso: d.InstrumentAdmin}`; assert `transferKind == "direct"`; build the
       seam; call `transferOut` with `InputHoldingCids: []` (in-script resolution).
     - existing kinds: unchanged (`fundUser` path).
   - On the script failing with a stale-context error, re-fetch the factory context
     once and retry (log it).
4. **`cmd/ntt-playground/receive.go`**: hard error for `"cip56-custody"`
   ("receiving network out of scope for real Amulet").

Deliverable: `transfer --deployment cc-custody ... --amount 10000000000` locks
1 CC sender→custody on live LocalNet; `balance` shows `amuletHoldingTotal=1.0…` for
custody; recompute output printed as today. (Manually verifiable mid-plan; the
Phase-0 subtest still red only on the `observe stream` step.)

### Phase 4 — Real stream observer (`observe stream`)

1. **`go.mod`**: add `github.com/gorilla/websocket v1.5.3` (see supply-chain).
2. **File: `canton/testing/cli/internal/observer/observer.go`** (new package):
   ```go
   type Config struct {
       JSONAPIBaseURL string        // http://localhost:3975 (profile.JSONAPIBaseURL)
       Token          string        // JWT for the reading user (guardian-watcher)
       ObserverParty  string        // guardianObserver party id (the filter party)
       BeginExclusive int64
       Logf           func(string, ...any)
   }
   type Observed struct {
       Registrar, Owner string
       EmitterID        int64
       Sequence         int64
       Nonce            int64
       ConsistencyLevel int64
       Payload          string // hex
       EffectiveAt      string // JsTransaction.effectiveAt (watcher timestamp)
       UpdateID         string
       Offset           int64
       EmitterAddress   string // hex(wire.DerivedAddress(EmitterAddressTag, Registrar, Owner, EmitterID))
   }
   func LedgerEnd(ctx context.Context, baseURL, token string) (int64, error) // GET /v2/state/ledger-end
   // Stream dials ws(s)://.../v2/updates with subprotocols
   // "daml.ws.auth", "jwt.token."+token, sends GetUpdatesRequest (shape in findings §7,
   // TemplateFilter "#wormhole-core:Wormhole.Core.State:Emitter", LEDGER_EFFECTS),
   // and invokes yield for every consuming PublishMessage ExercisedEvent until ctx
   // is done or yield returns false.
   func Stream(ctx context.Context, cfg Config, yield func(Observed) bool) error
   ```
   Decode ints via `json.Number`. `exerciseResult` decodes as
   `{registrar, owner, emitterId, sequence, nonce, consistencyLevel, payload}`
   (`WormholeMessage`; `payload` is a hex string — `Bytes = Text` in wormhole-core).
   Skip non-`Transaction` frames (`OffsetCheckpoint` etc.).
3. **File: `cmd/ntt-playground/observe_stream.go`** (new): `observe stream`
   subcommand attached to the existing `newObserveCmd` parent in `query.go`
   (convert `observe` from leaf to parent-with-Run — cobra runs the parent `RunE`
   when no subcommand matches, so `observe --deployment X` keeps working).
   Flags: `--deployment` (required), `--from-offset` (int64; default: current ledger
   end), `--count` (default 1), `--timeout` (default 3m), `--print-offset` (print
   `ledgerEnd=<n>` and exit), `--any-emitter` (skip the address filter).
   Behavior:
   - error unless `prof.JSONAPIBaseURL != ""` ("observe stream requires the localnet
     profile");
   - ensure the reader user exists: `network.CreateLedgerUser(ctx, base, adminToken,
     "guardian-watcher", readAs=[s.GuardianObserver], actAs=nil)`; mint
     `MintUnsafeToken("guardian-watcher", 1h)` — **the stream is read with
     CanReadAs(guardianObserver) only** (verbose-log this claim);
   - stream until `--count` messages whose derived `EmitterAddress` equals the
     deployment's `TransceiverAddress` (unless `--any-emitter`);
   - stdout: a JSON array of `Observed` (camelCase fields incl. `emitterChain: 72`
     constant), machine-parseable (verbose narration stays on stderr).
4. **`internal/observer/observer_test.go`** (from Phase 0) green: fixture-driven
   decoding + address derivation; no network needed.

### Phase 5 — Green end-to-end + docs

- Run `NTT_PLAYGROUND_PROFILE=localnet LOCALNET_DIR=… go test -tags e2e ./e2e -v
  -timeout 30m`: the Phase-0 subtest passes; all pre-existing subtests stay green
  (mock kinds untouched; the managers-count subtest runs before the new deploy).
- Sandbox CI run (`go test -tags e2e ./e2e`) green: new subtest skips; the Phase-1
  gating subtest (cip56-custody rejected on sandbox) passes.
- `go vet ./... && go test ./...` green.
- **`canton/testing/cli/README.md`**: document the new token kind (deploy-config
  table), the `observe stream` command (Commands table), the wallet-user model
  (tap is USD-denominated; custody preapproval), and delete the two completed
  follow-ups (`:284-287`), leaving the governance-signing one. Update the
  "Verification status" paragraph. No `.claude` references.
- Update this plan file with per-phase completion notes as work lands.

## Supply-chain section (NEW dependencies, exact pins)

| Dependency | Kind | Pin | Provenance / audit |
| --- | --- | --- | --- |
| `github.com/gorilla/websocket` | Go module (direct) | `v1.5.3` (exact; no `^`/`~` semantics in Go anyway — go.sum freezes it) | Well-known publisher (gorilla toolkit, maintained again since 2023, >20k importers). Steps: `go get github.com/gorilla/websocket@v1.5.3` → verify `go.sum` diff is exactly the two expected lines → `go mod verify` → `govulncheck ./...` (`golang.org/x/vuln/cmd/govulncheck@latest` run, not installed as a dep) → commit `go.mod`+`go.sum` in the same commit as the code using it. Check for a newer *patch* at implementation time; pin whatever exact version is chosen. |
| `splice-amulet-0.1.22.dar` | vendored DAR (data-dependency of `ntt-test` ONLY) | sha256 `bcfcd6a9250172a8384d1326a2990b374c66a367ca7d4120b243aecfd372761e` | Extracted from the already-trusted `0.6.12_splice-node.tar.gz` (same provenance as the six DARs in `canton/dars/README.md`; official release `digital-asset/decentralized-canton-sync` v0.6.12, path `splice-node/dars/`). Byte-identical to `splice-amulet-current.dar` in the bundle (verified). Never rebuilt from source. Add the sha256 row to `canton/dars/README.md`. Embedded interface DALF package-ids verified identical to the repo's existing vendored interface DARs (`holding-v1 718a0f77…`, `transfer-instruction-v1 55ba4deb…`, `metadata-v1 4ded6b66…`), so no package conflicts. **Must never become a dependency of `ntt`, `ntt-token`, or `ntt-cip56`.** |

Nothing else is new. Explicitly NOT added: any gRPC/proto stack
(`dazl-client`/`grpc`/`protobuf` — only the documented fallback), any Daml SDK
change (stays 3.5.1). All `go test`/`go build` runs use the existing committed
`go.sum` (Go's default is frozen/verified resolution; never run `go mod tidy`
without reviewing the diff).

Cannot be pinned (and doesn't need to be): the LocalNet docker images are already
pinned by `IMAGE_TAG=0.6.12` per the README's LocalNet instructions.

## Verify-live results (recorded live against the booted LocalNet, before writing the Go/Daml
## code that depends on them)

- **DAR sha256**: confirmed exactly `bcfcd6a9250172a8384d1326a2990b374c66a367ca7d4120b243aecfd372761e`
  for `splice-node/dars/splice-amulet-0.1.22.dar` extracted from the tarball (the earlier
  `tar -xzf ... -O` probe silently produced empty output due to a shell quirk; extracting to
  disk and hashing confirmed the match). Byte-identical to `splice-amulet-current.dar` in the
  same bundle, as the plan asserts. DAR copied to `canton/dars/splice-amulet-0.1.22.dar`.
- **V1 (wallet-user onboarding)**: confirmed live. `POST /v0/register` with the new user's own
  JWT and empty body `{}` returns `{"party_id": "..."}}`. **Deviation from plan**: the FIRST
  call for a brand-new user timed out client-side (>10s, the validator's automation is slow the
  very first time); a retry with a 60s client timeout returned 200 with the same party id
  (idempotent, per the plan's claim). `internal/amulet.OnboardWalletUser` must use a generous
  timeout (60s+) rather than assuming a fast response.
- **V2 (transfer-factory choiceArguments shape)**: confirmed live with the exact candidate body
  the plan proposed (RFC3339 `.000Z` timestamps, `inputHoldingCids: []`, `meta: {"values": {}}`,
  `extraArgs: {"context": {"values": {}}, "meta": {"values": {}}}`) — HTTP 200,
  `"transferKind":"direct"` once the receiver had a `TransferPreapproval`.
  **Deviation from plan**: the response's `disclosedContracts` includes a FIFTH contract not in
  the plan's mapping table: `Splice.ExternalPartyConfigState:ExternalPartyConfigState` (registry
  templateId suffix `Splice.ExternalPartyConfigState:ExternalPartyConfigState`, choiceContextData
  key `"external-party-config-state"`). The plan's speculative
  `Splice.Amulet:FeaturedAppRight ("maybe")` row did NOT appear and is dropped. Confirmed the
  type exists in the vendored `splice-amulet-0.1.22.dar` (`Splice/ExternalPartyConfigState.daml`,
  `signatory dso`) — the DAR ships full Daml source, not just DALFs, so this was verified by
  inspection, not guesswork. Updated mapping table (Playground.Amulet's `toDisclosure`) must
  include this fifth row or the disclosure-mapping abort will trip on every live transfer.
- **V3 (ChoiceContext Daml-JSON decode)**: deferred to Phase 3 implementation (needs the Daml
  module to exist); the raw `choiceContextData` shape captured live is
  `{"values": {"transfer-preapproval": {"tag": "AV_ContractId", "value": "<cid>"}, "open-round":
  {...}, "external-party-config-state": {...}, "amulet-rules": {...}}}` — an `AnyValue` variant
  map exactly as `Splice.Api.Token.MetadataV1.ChoiceContext` models it.
- **V4 (`POST /v2/users` shape)**: confirmed live via `GET /v2/users` (list) — user objects carry
  `id`, `primaryParty`, `isDeactivated`, `metadata`, `identityProviderId`,
  `primaryPartyAuthentication`. Rights-grant wrapper convention (`{"kind": {"CanActAs":
  {"value": {...}}}}`) was already proven in `localnet.go`. **Now also confirmed for
  `CanReadAs`** in Phase 2: `POST /v2/users` with body `{"user": {"id": "...", "isDeactivated":
  false}, "rights": [{"kind": {"CanReadAs": {"value": {"party": "<party>"}}}}]}` returned HTTP
  200 against the live 0.6.12 stack — the wrapper convention is uniform across right kinds.
- **V7/V8 (update-stream shapes)**: confirmed live via the `POST /v2/updates` blocking-list
  fallback (chose this over a raw WS probe since no `wscat`/`websocat`/python `websockets` was
  available in this sandbox; the blocking POST uses the identical request/response shapes per
  the plan, so this fully exercises the decode path before Phase 4's WS client is written).
  `GET /v2/state/ledger-end` returns exactly `{"offset": <int>}`. **Deviation from plan**: the
  top-level `update` sum type IS `value`-wrapped — `{"update": {"Transaction": {"value":
  <JsTransaction>}}}` — the plan's doc comment showed it unwrapped
  (`{"update": {"Transaction": JsTransaction}}`). Individual `events[]` items are NOT further
  `value`-wrapped: `{"CreatedEvent": {offset, nodeId, contractId, ...}}` and
  `{"ExercisedEvent": {offset, nodeId, contractId, templateId, interfaceId, choice,
  choiceArgument, actingParties, consuming, witnessParties, lastDescendantNodeId,
  exerciseResult, packageName, implementedInterfaces, acsDelta}}` are flat, confirming the
  plan's per-field list exactly. `internal/observer`'s `Transaction`/`Update` Go structs must
  unwrap one extra `{"value": ...}` layer at the Update-kind level only.
  Also confirmed: a bare empty-party-filter (`filtersForAnyParty`) request against
  `ledger-api-user`'s token was rejected with 403 `PERMISSION_DENIED` — reinforcing the plan's
  design that the stream must always filter by the actual `guardianObserver` party (never an
  any-party filter), read through the dedicated `guardian-watcher` reader user.

## Verify-at-implementation table (explicitly unverified shapes — confirm live, do not guess)

| # | What | How to confirm (exact command) |
| --- | --- | --- |
| V1 | ~~Wallet-user onboarding~~ **RESOLVED during planning** — `POST /v0/register`, new user's own JWT, empty body, response `{"party_id"}`, idempotent, no secret (see findings §8) | Nothing to confirm; optional smoke: `curl -s -X POST -H "Authorization: Bearer $TOKEN" -d '{}' localhost:3903/api/validator/v0/register` |
| V2 | Exact Daml-JSON accepted by `getTransferFactory.choiceArguments` (timestamp format, `inputHoldingCids: []` tolerated, TextMap `{values:{}}` vs `{}`) | `curl` the scan-proxy endpoint with a candidate body; on 400 the error names the offending field. Reference: token-standard CLI `token-standard/cli/src/commands/` @0.6.12 builds this exact body in TS. |
| V3 | `choiceContextData` decodes as `Splice.Api.Token.MetadataV1.ChoiceContext` through `dpm script --input-file` (AnyValue variant encoding) | Feed a captured response through a trivial echo script (`Playground.Amulet:debugDecodeContext`) before wiring transferOut; compare against daml-script's JSON codec. If it mismatches: fall back to passing the two known cids (`amulet-rules`, `open-round`, `transfer-preapproval` keys) as typed fields and building the `ChoiceContext` in-script from them. |
| V4 | `POST /v2/users` body shape (`{"user": {"id": ...}, "rights": [...]}`) and the `CanReadAs` wrapper | `curl localhost:3975/v2/users` with the admin token; the 3.5 openapi (`json-api-docs/openapi.yaml`, canton release-line-3.5) documents `CreateUserRequest`. The `value`-wrapper convention is already proven for `CanActAs` (localnet.go:386-396). |
| V5 | `Disclosure.blob` (daml-script) == registry `createdEventBlob` (both base64 of the created-event blob) | In a script: `queryDisclosure` any contract, print `.blob`; compare format to a registry response's `createdEventBlob`. Both feed `DisclosedContract.created_event_blob`; expected identical. |
| V6 | `Disclosure` construction route: does `import Daml.Script.Internal.Questions.Commands (Disclosure(..))` compile under 3.5.1? | One-line import + `dpm build`. If not: use the record-update route (findings §9), which needs no constructor. |
| V7 | WS handshake specifics against LocalNet 3975 (subprotocol echo, first-frame shape) | `wscat` (or a 20-line Go probe) with subprotocols `daml.ws.auth`, `jwt.token.<jwt>`, send the GetUpdatesRequest from findings §7, observe a transfer. Fallback: `POST /v2/updates` blocking list with `endInclusive` (no new dep). |
| V8 | `sequence`/`emitterId` numeric rendering in `exerciseResult` (number vs string) | Same probe as V7; `json.Number` handles both regardless. |

## Risks / notes

- **Preapproval automation timing**: the wallet endpoint blocks while validator
  automation converts the proposal; the validator pays the fee. Mitigations baked
  into the plan: tap the `app-provider` wallet first; retry 429; generous deploy
  timeout on LocalNet.
- **Round ingestion lag right after `network up`**: tap/transfer-factory return
  "no open mining round"-style errors until scan/validator stores catch up —
  bounded retry (≤2 min) in `internal/amulet`.
- **`transferKind != "direct"`** means the custody preapproval is missing/expired →
  fail fast in Go with a pointed error (never submit; the on-ledger abort in
  `Cip56CustodyToken` is the backstop, pinned by unit tests already).
- **Amulet transfer fees** are deducted from the sender's inputs — never assert the
  sender's post-balance exactly; assert the custody side (exactly 1.0) and the wire
  amount (exactly 10^8).
- **`observe` command restructure** (leaf → parent with subcommand): keep
  `observe --deployment X` behavior identical (existing e2e subtest is the guard).
- **Keep production DARs untouched**: no change under `canton/ntt*`;
  `splice-amulet` is data-depended by `ntt-test` only.
- The e2e "contracts list" manager-count assertion stays at 3 because the new
  deploy subtest runs after it; if anyone reorders, bump the count per profile.

## New/changed test paths

- `canton/testing/cli/e2e/playground_e2e_test.go` — new LocalNet-gated subtest
  (Phase 0) + sandbox gating-error subtest (Phase 1).
- `canton/testing/cli/internal/observer/observer_test.go` — new (fixture decode,
  address derivation, json.Number tolerance).
- `canton/testing/cli/internal/amulet/amulet_test.go` — new (httptest shapes,
  409/429/retry, transferKind guard).
- `canton/testing/cli/testdata/deploy-cip56-custody.json` — new deploy config.
- Daml: `dpm test` coverage for `parseTokenKind "cip56-custody"` + the deploy
  branch shape where testable without a live DSO (`Test/TestNtt.daml` can create a
  `Cip56CustodyToken` with a fake DSO party to pin the deploy record wiring —
  the real-Amulet arc itself is LocalNet-only and covered by the e2e subtest).
