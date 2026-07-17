# Native Token Transfers (NTT) on Canton

This directory is the **Canton/Daml** implementation of Wormhole
[NTT](https://github.com/wormhole-foundation/native-token-transfers) — the same
NTT protocol implemented for EVM, Solana, and Sui elsewhere in this repository,
ported to the Canton Network.

It is an **uploadable package set built on top of the Wormhole core bridge**. The
core bridge itself (the `wormhole-core` Daml package: `Emitter`, VAA
parse/verify, guardian-set and replay machinery) is **not** in this repository —
it lives in
[`wormholelabs-xyz/wormhole`](https://github.com/wormholelabs-xyz/wormhole)
under `canton/`, and its design is documented in that repo's
[`canton/README.md`](https://github.com/wormholelabs-xyz/wormhole/blob/integration/canton/canton/README.md).
The core is consumed here **as a pinned, vendored DAR** (see
[`dars/README.md`](dars/README.md)) and used as two primitives: **publish** and
**verify**. On Canton the NTT "Wormhole transceiver" *is* a core `Emitter`.

Like the core, NTT uses **no contract keys**: every contract is resolved by an
explicit, disclosed contract-id (see "No contract keys / disclosed cids" in the
core README).

> For core-bridge background referenced below (message publishing & fees, the
> replay trie, disclosed-cid submission, on-chain signature verification, the
> trust model), see the corresponding sections of the
> [core `canton/README.md`](https://github.com/wormholelabs-xyz/wormhole/blob/integration/canton/canton/README.md).

## Packages

| Directory (Daml package) | Contents |
| --- | --- |
| `ntt-token` (`ntt-token`) | The `NttToken` interface (token effect seam) + `TokenMode`, `TrimmedAmount`. Interface-only, per Daml's upgradeability rule. Depends on the CIP-0056 `holding`/`metadata` interfaces (the seam carries `[ContractId Holding]` + `ExtraArgs`). |
| `ntt` (`ntt`) | `Payload` (wire codec) and `Manager` (`NttManagerRegistry`, `NttManager`, deploy/send/receive/peers). |
| `ntt-cip56` (`ntt-cip56`) | Real CIP-0056 implementations of `NttToken` — `Cip56CustodyToken` (lock/unlock via `TransferFactory`) and `Cip56BurnMintToken` (burn/mint via `BurnMintFactory`). Works for Canton Coin (Amulet) or any conforming token. |
| `test` (`ntt-test`) | Daml Script test suite (`Test.TestNtt`) plus a minimal copied subset of core test helpers (`Test.TestCore`, `Test.TestReplay`, `Test.MockToken`) needed to exercise NTT end to end under `dpm test`. |
| `dars/` | Vendored DAR binaries: the pinned `wormhole-core` DAR + the CIP-0056 interface DARs. Provenance and sha256 in [`dars/README.md`](dars/README.md). |

## Build & test

Requires the `dpm` toolchain (Daml SDK 3.5.1, declared in every `daml.yaml`) and
a JDK. All four packages build against the vendored core DAR — **core is never
rebuilt here**.

```sh
# from this directory
dpm build --all        # builds ntt-token, ntt, ntt-cip56, then test (in dep order)
cd test && dpm test    # runs the Daml Script suite
```

A live-sandbox Go integration harness (the recipient-binding match test) lives
in [`testing/go/`](testing/go/); see its build tag and comments to run it
against a `dpm sandbox`.

## Deployment — permissionless and crankless

Anyone stands up a deployment (bring your own token, pick a mode), mirroring core
`RegisterEmitter`. The deployer (`admin`) first registers a core `Emitter` (the
transceiver), then exercises `RegisterManager` directly on the disclosed
`NttManagerRegistry`, which allocates a stable `managerId` and creates the
`NttManager`. `operator`'s co-signature is **inherited from the registry
signatory** — no approval crank — and `operator` and `admin` both co-sign the
manager (mirroring `Emitter`); neither has power over transfer contents.

**Two key-derived identities**, both owner-bound (see the core README's message
publishing section):
- **transceiver address** = `keccak256("wormhole:emitter:v1" ‖ operator ‖ admin ‖ emitterId)`
  — the VAA emitter other chains register; derived by the watcher, not stored. It
  must be a **single shared** `Emitter` per deployment (its address is the peer
  identity), so it cannot be per-user.
- **manager address** = `keccak256("wormhole:ntt-manager:v1" ‖ operator ‖ admin ‖ managerId)`,
  computed on-ledger at registration and stored in `managerAddress`. The distinct
  domain tag keeps an emitter and a manager with the same ordinals from colliding;
  `admin` is in the preimage and co-signs, so a compromised operator cannot forge
  an existing manager's address to consume that peer's inbound VAAs.

## Wire codec (`Wormhole.Ntt.Payload`)

Big-endian encoders for the three nested structures (`NativeTokenTransfer` prefix
`0x994E5454`; `NttManagerMessage`; `WormholeTransceiverMessage` prefix
`0x9945FF10`), built on `Bytes.daml`. Amounts use the NTT **TrimmedAmount** (≤ 8
decimals + a scale byte). Round-trips are tested in `TestNtt:testNttCodec`.

## Send (`NttManager.Transfer`, controller `user`) — the sender self-pays

Trims the amount, drives the token seam's `LockOrBurn` (attributed to `user`, the
actual caller), assembles the nested message, and publishes it via
`Emitter.PublishMessage` — so the guardian watcher observes an **ordinary core
message** (no NTT-specific watcher code). The transceiver `Emitter` and the
`CoreState` are caller-supplied disclosed cids; the transceiver is fetched and
bound to the deployment before publishing.

`user` is the **sole** controller of `Transfer` — no admin/operator sign-off.
Exercising `Transfer` consumes `NttManager` (signatories `operator, admin`), so
the nested `PublishMessage` (controller = the Emitter's `owner`, i.e. `admin`)
runs on inherited signatory authority. `LockOrBurn`'s controller is `manager,
sender`; since `user` (the `Transfer` controller) is passed as `sender`, its
authority reaches the token seam — so an owner-signed holding can actually be
spent/burned, not just an admin-signed mock. The CIP-0056 factory separately
asserts the input holding's owner matches `user`, so a caller can never lock/burn
a holding it doesn't control.

**The sender pays the message fee itself.** `PublishMessage` charges the
governance-set `messageFee` to its `payer`, and `Transfer` passes `payer = user`,
whose authority reaches the fee's `Allocation_ExecuteTransfer` because as payer it
**co-controls** `PublishMessage`. The user attaches a fee allocation from its own
wallet (empty at fee 0) — the same fee every core publish pays, so an NTT send is
never fee-exempt (`TestNtt:testNttSend`, `testNttTransferByDifferentUser`).
Without this `payer` co-controller seam the fee could not be drawn from the user,
since the transceiver is a shared, admin-owned emitter.

The one non-fee requirement is **visibility, not authorization**: `user` is not a
stakeholder of `NttManager`, the transceiver `Emitter`, the token, or the
`CoreState`, so a submission attaches those as explicit disclosures (in Daml
Script, `submitWithDisclosures`). The manager and its transceiver churn their cids
on every send, so a sender re-resolves the fresh cids per send — the same
fail-closed retry the core publish path uses.

## Receive (`NttManager.Receive`, controller `executor`) — permissionless

Any `executor` — a relayer, the recipient, anyone — relays an inbound VAA on the
recipient's behalf. The choice is `nonconsuming`, so its body inherits
`NttManager`'s `operator, admin` authority regardless of who submits; the
executor's identity carries **no authority**, only the disclosures it attaches
(the `CoreState`, the covering `ReplayNode`, the token). It verifies + claims the
VAA atomically via the core `VerifyAndConsumeVAA` — the per-consumer replay trie,
scoped to `admin`, so each deployment claims a VAA at most once — then enforces
the peer, decodes, binds the recipient, and mints/unlocks.
(`TestNtt:testNttReceiveByExecutorRecipientMismatchFails` drives a real receive by
a third-party relayer, all the way to the recipient gate.)

**Guardian trust root — pinned to `guardianGovernance`, not the operator.** The
disclosed `CoreState` is authenticated by checking its `guardianGovernance`
against the anchor `admin` committed at deployment (via `GetGuardianGovernance`).
A `CoreState` requires only *some* `(operator, gg)` signature pair, so pinning the
operator would not stop a compromised operator co-signing a forged `CoreState`
(throwaway gg party, attacker guardian set); pinning `guardianGovernance` — the
guardians' k-of-n external threshold party, unforgeable by a single hot operator
key — closes that (`TestNtt:testNttReceivePinsGuardianGovernance`). This mirrors
EVM NTT verifying against an immutable core address, and soundly replaces the old
key-maintainer trust (a manager keyed to `guardianGovernance`).

**Recipient binding.** A Canton `Party` id has no fixed-size form to bind against
the VAA's `Bytes32` `recipientAddress`, so `Receive` checks
`recipientAddressFor recipient == ntt.recipientAddress`, where `recipientAddressFor`
is `keccak256("wormhole:ntt-recipient:v1" ‖ lp(partyToText recipient))`. A sender
computes the same hash off-chain over the recipient's exact Party-id string; since
it lives inside the VAA it is covered by the guardian signature, so no relayer can
steer the mint elsewhere. **Tradeoff, deliberate:** a Party id is permanent, so a
VAA is bound to the recipient party as it existed when the sender hashed it; a
recipient needing a *different* party (lost key, custodial migration) requires a
re-send. Pinned by `testRecipientAddressForVector`; the matching-recipient path
(no static fixture can match a nondeterministic Party fingerprint) is covered by a
live two-step harness (`testing/go/ntt_recipient_match_integration_test.go`).

## Token seam (`NttToken`) and modes

`NttManager` never references a concrete token; it holds a `ContractId NttToken`
and calls `LockOrBurn` / `MintOrUnlock`, isolating the token detail the way
`VAA.daml` isolates crypto. `TokenMode` selects lock/unlock or burn/mint. The seam
choices carry the CIP-0056 runtime handles (`[ContractId Holding]`, `ExtraArgs`)
the manager threads through. Real implementations live in `ntt-cip56`
(`Cip56CustodyToken`, `Cip56BurnMintToken`); a stdlib `MockToken` in the test
package exercises the whole protocol under `dpm test`. **Submission caveat:** a
production send/receive is app-orchestrated — the caller submits with the token
holder's authority and the registry's disclosed contracts (`extraArgs`). For a
third-party-executor receive against a real (owner-signed) token, the recipient
additionally needs a CIP-0056 transfer pre-approval, or must submit itself; the
mock (admin-signed holdings) proves the on-ledger path without it. `Cip56CustodyToken`'s
lock/unlock also asserts the registry's `TransferFactory_Transfer` settled
synchronously (`TransferInstructionResult_Completed`); a `Pending`/`Failed`
result aborts instead of letting the seam continue with tokens not actually
moved.

## Follow-ups

- **Third-party-executor + owner-signed token**: needs a recipient pre-approval
  (above) for a fully hands-off relay; the on-ledger permissionless path is done.
- **Send contention / message id**: `Transfer` is consuming (per-manager
  `outboundSequence`), so concurrent sends serialize on the manager cid. Sourcing
  the NTT message id from the transceiver `Emitter`'s sequence would let `Transfer`
  be nonconsuming, removing that contention point.
- **Hint-free verification** (persist guardian pubkeys) so `Receive` needs no
  `pubKeys` hints — core-wide, orthogonal to NTT; tracked in the core repo.
- **Recipient-address change**: the permanent Party-id binding above.
