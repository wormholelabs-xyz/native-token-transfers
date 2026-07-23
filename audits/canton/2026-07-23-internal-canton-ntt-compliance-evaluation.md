# NTT Canton Implementation — Compliance Evaluation

Date: 2026-07-23. Audited commit: `c41edbf6cb97499fd39933a542c798a0981492f4`
("canton: enforce reserve coverage on admin handoff and exact registry delivery"),
the tip of branch `canton/ntt-cip56-rework` at the time of evaluation. All file and
line references are as of that commit.

Scope: functional wire/mesh compatibility with
the NTT standard, the 29 security invariants in `docs/INVARIANTS.md`, and access
control. Method: six parallel deep-readers mapped the Canton code, the EVM/Solana/Sui
reference, the wire formats and their test vectors, and the mesh-integration surface;
each load-bearing claim was then adversarially re-verified against the actual code
(13 verifiers, each trying to refute its finding), and the wire codec was checked
empirically by porting the reference deserialization vectors into Daml
(`canton/test/daml/Test/TestNttVectors.daml`, 66/66 tests pass).

## Bottom line

The implementation is a faithful, security-sound port of NTT to Canton's model. The
wire codec is byte-identical to the reference for the canonical cross-runtime message,
so a Canton deployment can join an existing EVM/Solana NTT mesh at the protocol level.
No fund-theft, inflation, forgery, or double-spend path was found. The gaps are three
substantive ones (all medium) and a set of low/cosmetic divergences; none is a
correctness hole, but two affect real deployments.

The three that matter:

1. **No Wormhole chain id for Canton** — a launch-coordination prerequisite, not a code
   defect. Nothing in the contracts hardcodes a chain id; the tooling and guardian
   watcher need an assignment before a real mesh connection is possible.
2. **Outbound amount trimming ignores the peer's decimals** — sends to a peer whose
   token has fewer than 8 decimals silently lose the sender's dust at the destination.
   Deflationary only (never inflation or theft), but real value loss.
3. **No pause / emergency-halt, and burn/mint mint has no cap** — the reference treats
   pause as a core safety feature; Canton has no first-class halt, and a compromised
   peer chain plus a guardian-signed VAA can mint unboundedly in burn/mint mode.

Everything else is either subsumed by another mechanism, an intended design tradeoff, or
informational.

---

## 1. Functional compatibility and mesh integration

### Wire format: compatible (empirically verified)

Canton encodes the three nested NTT structures exactly as the spec and the reference
implementations do:

- `NativeTokenTransfer`: `0x994E5454 ‖ decimals(1) ‖ amount(8) ‖ sourceToken(32) ‖
  recipientAddress(32) ‖ recipientChain(2) [‖ additionalPayloadLen(2) ‖ payload]`
- `NttManagerMessage`: `id(32) ‖ sender(32) ‖ payloadLen(2) ‖ payload`
- `WormholeTransceiverMessage`: `0x9945FF10 ‖ sourceManager(32) ‖ recipientManager(32)
  ‖ mgrPayloadLen(2) ‖ mgrPayload ‖ xcvrPayloadLen(2) ‖ xcvrPayload`

The ported test `testVectorTransceiverMessage1` proves Canton's encoder produces
byte-for-byte the shared 217-byte `evm/test/payloads/transceiver_message_1.txt` vector
(the one EVM, Solana, and the TS SDK all consume) and its decoder round-trips it. The
optional `additionalPayload` extension and the non-canonical explicit-zero-length form
both behave exactly as the reference: accepted on decode, never emitted on encode.

Two encoder/decoder divergences, both benign (details in §5): decoders tolerate trailing
bytes where EVM enforces exact length (not reachable as an exploit because the VAA is
guardian-signed and replay keys on the VAA hash), and the `sender` field carries the
manager address rather than the originating user.

### The transceiver is a core Emitter

The "Wormhole transceiver" on Canton is a core Wormhole `Emitter`. A send is an ordinary
`Emitter.PublishMessage`; the guardian watcher observes a normal core message, so there
is no NTT-specific watcher code. Inbound, Canton consumes standard VAAs via the core
`VerifyAndConsumeVAA`. This is the cleanest possible fit with the existing guardian
infrastructure.

### How to connect Canton to an existing mesh

Per peer chain, four data cross the boundary:

- **On the EVM peer** (`onlyOwner`): `NttManager.setPeer(cantonChainId,
  cantonManagerAddress, cantonTokenDecimals, inboundLimit)` and
  `WormholeTransceiver.setWormholePeer(cantonChainId, cantonTransceiverAddress)`.
- **On the Canton side** (`controller admin`): `SetPeer{chainId, Peer{managerAddress,
  transceiverAddress}}` — one choice registers both the peer manager and peer
  transceiver (EVM splits these across two contracts).

Canton's two 32-byte identities are keccak hashes bound to the stable `namespace` party
(so they survive admin handoff):

- `managerAddress = keccak256("wormhole:ntt-manager:v1" ‖ operator ‖ namespace ‖ managerId)`
  — stored on-ledger, this is the peer address EVM registers and checks as `sourceManager`.
- `transceiverAddress = keccak256("wormhole:emitter:v1" ‖ operator ‖ namespace ‖ emitterId)`
  — derived off-ledger by the watcher, this is the VAA `emitterAddress` EVM checks.

### What does not fit the standard model

- **No assigned Wormhole chain id** (Finding F3, medium). Grep of `sdk/` and `cli/` finds
  zero Canton references; `@wormhole-foundation/sdk-base` has no Canton entry; tests use
  `72` as a placeholder. Without an assignment, guardians cannot stamp `emitterChain`,
  peers cannot be registered, and the SDK cannot represent the chain. The contracts
  themselves are chain-id-agnostic (no hardcoded id), so no contract change is needed once
  an id is assigned — only test vectors that bake in `72` would need regeneration.
- **No SDK / CLI / Connect / executor support.** No Canton platform module exists;
  the CLI switches cover only Evm/Solana/Sui. Canton's submission model (Ledger API
  `actAs` parties + explicit disclosed contracts) matches none of the existing signer
  abstractions. Inbound delivery is permissionless but operationally heavy: a relayer
  needs an off-ledger disclosure index (manager, CoreState, replay node; ledger + custody
  pot for lock/unlock; deposit pre-approval + factory for burn/mint).
- **Single transceiver vs manager-global threshold** (F4, low). Canton has exactly one
  transceiver (implicit 1-of-1). Because the EVM threshold is manager-global, an existing
  multi-transceiver EVM deployment cannot add Canton without dropping global threshold to
  1 — a real deployment constraint, liveness only, no theft. Canton's attestation security
  equals the guardian set, the same trust as a threshold-1 Wormhole-only EVM deployment.
- **No `TransceiverInit` / `TransceiverRegistration` broadcasts** (F5, low). Canton emits
  neither. These are consumed only by off-chain NTT Global Accountant bookkeeping; the
  standard treats them as opt-in (Solana/Sui do not emit them automatically). Transfers
  work without them; only accountant discovery is affected, and the namespace can publish
  the payloads manually via its Emitter if needed.
- **Recipient binding is a one-way hash of the party id.** A Canton `Party` has no
  fixed-size form, so the sender hashes it into the 32-byte `recipientAddress`. The binding
  is permanent: a recipient who loses the party (key loss, custodial migration) needs a
  re-send. Mint additionally requires a standing self-signed `DepositPreapproval`.
- **`additionalPayload` is always empty outbound and ignored inbound** — no integrator
  additional-payload hook.

---

## 2. Security invariants (INV-001 .. INV-029)

### Supply conservation and double-spend

- **INV-001 (supply conservation)** — Held, with one caveat. Lock/unlock is capped by the
  per-deployment `LockedLedger` (`Debit` rejects amounts above balance), so a deployment
  can release at most what it locked. Burn/mint burns exactly the wire amount and mints the
  received amount. Caveat: the decimals divergence (F2) makes cross-decimal transfers
  strictly deflationary — the destination can credit less than the source removed — never
  inflationary.
- **INV-002 (no double-spend)** — Held. Each VAA digest is consumed atomically in the
  replay trie. Scope is per-namespace, not per-manager (F8); adversarial analysis confirms
  no cross-manager double-spend, because the digest is bound to a unique `recipientManager`
  and a mismatch aborts and rolls back the consumption in the same Daml transaction.

### Message integrity

- **INV-003 (authentication)** — Held. Every inbound movement first pins the `CoreState`
  to the committed `guardianGovernance` (a compromised operator cannot forge the real gg's
  signature), then verifies guardian signatures via the core.
- **INV-004/005 (ordering / sequence reuse)** — Held via the Emitter sequence (message id =
  the transceiver's next sequence, unique per transceiver).
- **INV-006 (hash integrity)** — Held. Replay keys on the VAA hash over the exact signed
  bytes. Decoder trailing-byte tolerance (F7) does not weaken this: appending bytes breaks
  the guardian signature, and Canton re-derives no digest from decoded content.
- **INV-019 (chain id validation)** — Outbound half held (peer lookup by `recipientChain`).
  Inbound half absent (F1): `verifyInboundTransfer` never checks `recipientChain`. Verified
  low: the deployment-unique `managerAddress` equality check subsumes what `toChain` does on
  EVM (where identical contract addresses recur across chains). Defense-in-depth gap, not a
  reachable exploit.
- **INV-028/029 (payload length limits)** — Encoders contain no explicit uint16/uint8 length
  assertions; outbound size safety rests on the core Emitter's byte cap. Not reachable in
  practice (Canton emits fixed-shape messages); worth an assertion for parity.

### Access control and authorization (see also §3)

- **INV-007 (owner-only admin)** — Held. Config choices (`SetPeer`, `SetFactory`,
  `TransferAdmin`) are `controller admin`. Ledger `Credit`/`Debit`/`SetCustodyHint` require
  the full signatory set, reachable only through the manager's own choice bodies.
- **INV-008 (transceiver authorization)** — Held structurally: the single transceiver is
  fixed at registration and validated at send time (`owner == namespace`, operator,
  emitterId); inbound checks `emitterAddress == peer.transceiverAddress`.

### Rate limiting, thresholds, pause, upgrade

- **INV-009/010/011/026 (rate limiting)** — Absent (F13). Verified acceptable to defer: the
  EVM reference ships a `NttManagerNoRateLimiting` variant and treats limits as optional.
  README documents inbound rate limits as a follow-up.
- **INV-012/022/023/024/025 (multi-transceiver thresholds)** — Not applicable / structurally
  satisfied (F4). Canton has one non-disableable transceiver; effective threshold is fixed
  at 1. The register/enable/threshold machinery does not exist and is not needed.
- **INV-014/015 (pause)** — Absent (F13, the material half). All three references enforce
  pause; EVM keeps `whenNotPaused` even in the no-rate-limit variant. Canton has no
  first-class halt and no unilateral gg halt (archiving the manager needs all four
  signatories). A partial, reversible substitute exists — the admin can overwrite a chain's
  peer to a sentinel, blocking that chain's inbound and outbound — but it is per-chain, has
  no separate fast pauser role, and is undocumented as an emergency procedure.
- **INV-016/017 (upgrade)** — Daml/Canton upgrade model differs from the EVM proxy model;
  not evaluated in depth here. Contracts are replaced by governance, not upgraded in place.

### Decimals, peers, timing

- **INV-018 (decimals consistency)** — Partially held. Local decimals bounded 0..10
  (substrate-forced: Daml Decimal has 10 fractional digits); inbound rescale truncates and
  never overpays. The outbound peer-decimals gap (F2) is the divergence.
- **INV-021 (peer not on same chain)** — Unenforced (F9). Admin-only, so misconfiguration
  not attack; a same-chain peer yields only a supply-neutral burn-then-mint loop. Note
  Solana/Sui also lack this guard; it is EVM-specific hardening.
- **INV-027 (redemption controls)** — Held. Approval (VAA verification + peer + recipient
  binding) precedes any payout; replay state prevents duplicate redemption.

---

## 3. Access control

The four-party signatory model (`operator`, `namespace`, `admin`, `guardianGovernance`)
is coherent and each signature has a distinct, documented job. Verified sound:

- **The custody model is access-control-safe** (F12). `Release` accepts caller-supplied
  input holdings and `ConsolidateCustody` is permissionless, but the payout amount and
  recipient are fixed by the guardian-signed VAA, the transfer sender is hardcoded to
  `admin`, exact delivery is asserted, and every spent holding is validated as the current
  admin's own, of this instrument, unlocked. An executor cannot spend a third party's
  holdings or redirect value. Within the shared gg pool, the per-deployment `LockedLedger`
  cap keeps a hostile-peer deployment away from the others' collateral. Residual griefing
  (staling a custody hint) is bounded and permissionlessly repairable.
- **Burn/mint mint authority** is the guardian quorum itself — `DepositPreapproval.Deposit`
  is controlled by `instrumentId.admin` (= gg). The "arbitrary mint" surface is the
  intended burn/mint trust model, not a relayer-reachable bug: gg is the token's mint
  authority and a k-of-n quorum party.
- **Instrument binding** (burn/mint exclusivity) is enforced by a hash commitment in the
  instrument id (`instrumentId.id == nttInstrumentIdFor namespace nonce`), the Canton analog
  of EVM's `setMinter`. Tested for exclusivity.
- **Admin handoff** (`TransferAdmin`, usually propose-accept) is dual-authorized, moves the
  reserve with the role, and enforces a coverage check; the deployment identity (addresses,
  replay scope, ledger) stays with the stable namespace, so nothing changes on the wire.

One doc-vs-code discrepancy to fix (not a code bug): the README says replay is scoped to
`admin` and the burn/mint instrument is bound to `admin`; the code correctly uses
`namespace` (the stable identity) in both places. The code is the safer, self-consistent
choice; the README paragraphs are stale from the cip56 rework.

---

## 4. Findings, ranked

| ID | Finding | Severity | Class | Invariants |
|----|---------|----------|-------|------------|
| F3 | No assigned Wormhole chain id (launch prerequisite) | medium | missing-feature | INV-019 |
| F2 | Outbound trim ignores peer decimals → value loss to <8-dec peers | medium | spec-divergence | INV-001, INV-018 |
| F13 | No pause/halt; burn/mint mint uncapped | medium | spec-divergence | INV-014/015 (+009/010/011/026) |
| F1 | Inbound `recipientChain` not checked (subsumed by managerAddress) | low | spec-divergence | INV-019 |
| F4 | Single transceiver ⇒ EVM peers need global threshold=1 | low | intended-design | INV-012/022-025 |
| F5 | No TransceiverInit/Registration broadcasts (accountant only) | low | missing-feature | — |
| F6 | `sender` field = manager address, not user | low | spec-divergence | — |
| F7 | Decoders tolerate trailing bytes (not exploitable) | low | spec-divergence | INV-006 |
| F9 | `SetPeer` lacks EVM guards (admin-only, misconfig only) | low | spec-divergence | INV-021 |
| F10 | Outbound `sourceToken` caller-supplied, unchecked (metadata) | low | spec-divergence | — |
| F8 | Replay scope per-namespace not per-manager (sound) | info | intended-design | — |
| F11 | Decimals bounded 0..10; 0-dec can't be an EVM peer | info | intended-design | INV-018 |
| F12 | Custody override / permissionless consolidate (sound) | info | intended-design | — |

Uint64 asymmetry (not a numbered finding, folded into wire compat): Daml's signed 64-bit
`Int` cannot represent wire amounts in `[2^63, 2^64)`; such an inbound transfer aborts
(fail-closed, never mis-paid) but is a wire-compatibility asymmetry. A peer must not send
amounts at that scale to Canton.

---

## 5. Tests added

`canton/test/daml/Test/TestNttVectors.daml` (new, isolated, 6 scripts, all pass):

- `testVectorTransceiverMessage1` — byte-exact encode + full decode round-trip against the
  canonical cross-runtime vector. The core wire-compatibility proof.
- `testVectorAdditionalPayload` — the 32-byte `additionalPayload` extension, byte-exact.
- `testVectorNonCanonicalEmptyPayload` — the explicit-zero-length form is accepted on
  decode and re-encodes to the canonical (shorter) form, matching the reference.
- `testTrimVectorsMatching` — TrimmedAmount cases where Canton matches the reference.
- `testTrimVectorsPeerDecimalDivergence` — pins the F2 divergence: Canton emits
  `{158434, dec 6}` where the reference emits `{158, dec 3}`, and a 3-decimal peer then
  drops `0.000434`.
- `testAmountUint64Bound` — pins the signed-64-bit `Int` limit.

---

## 6. Recommendations

Ordered by value. None is required for correctness; the first three close the material gaps.

1. **Chain id (F3):** obtain a Wormhole chain id assignment, add the Canton entry to
   `sdk-base` and platform support to `sdk/`/`cli/`, and regenerate the vectors that bake in
   `72`. Optionally add a local `chainId` field to `NttManager` so the inbound path can also
   enforce the INV-019 `toChain` check explicitly.
2. **Peer decimals (F2):** add a `decimals` field to `Peer` (validated non-zero in
   `SetPeer`, mirroring EVM), trim outbound to `min(8, tokenDecimals, peerDecimals)`, and
   reject transfers whose amount does not round-trip (the `TransferAmountHasDust` analog).
   Removes silent sender value loss and restores burn/mint conservation across decimals.
3. **Pause (F13):** add a first-class `paused` flag on `NttManager`, checked in
   `Transfer`/`Release`/`Mint`, togglable by admin (and settable by a designated pauser or
   gg, mirroring EVM's owner/pauser split). Separately, prioritize the deferred inbound cap
   for burn/mint mode, whose mint is otherwise unbounded against a compromised peer.
4. **Low-cost hygiene:** exact-length assertion in the decoders (F7); `SetPeer` guards for
   chain-id range, same-chain, and zero addresses (F9); set `sender` to the originating user
   and `sourceToken` to a committed identifier (F6, F10); consider excluding 0 from the
   decimals bound (F11).
5. **Docs:** fix the stale README paragraphs that say `admin` where the code uses
   `namespace` (replay scope and instrument binding); document the single-transceiver /
   threshold-1 deployment constraint (F4) and the accountant-broadcast absence (F5).
