# Native Token Transfers (NTT) on Canton

This directory is the Canton/Daml implementation of Wormhole
[NTT](https://github.com/wormhole-foundation/native-token-transfers), the same
protocol implemented for EVM, Solana, and Sui elsewhere in this repository.

It builds on the Wormhole core bridge. The core itself (the `wormhole-core`
Daml package: `Emitter`, VAA parsing and verification, guardian sets, replay
protection) is not in this repository. It lives in
[`wormholelabs-xyz/wormhole`](https://github.com/wormholelabs-xyz/wormhole)
under `canton/`, is documented in that repo's
[`canton/README.md`](https://github.com/wormholelabs-xyz/wormhole/blob/integration/canton/canton/README.md),
and is consumed here as a pinned, vendored DAR (see
[`dars/README.md`](dars/README.md)). NTT uses two core primitives: publish a
message, and verify-and-consume a VAA. The NTT "Wormhole transceiver" on
Canton is simply a core `Emitter`.

Like the core, NTT uses no contract keys: every contract is resolved
off-ledger and passed as an explicit, disclosed contract id. Background for
the core concepts referenced below (message publishing and fees, the replay
trie, disclosed-cid submission, the trust model) is in the core
`canton/README.md`.

## Packages

| Directory (Daml package) | Contents |
| --- | --- |
| `ntt` (`ntt`) | The protocol. `Manager` (the `NttManager`), `Governance` (the `NttGovernance` bootstrap root and manager registry), `Ledger` (per-deployment `LockedLedger` custody accounting), `Deposit` (the recipient's `DepositPreapproval`), `Payload` (wire codec), `Amount` (`TrimmedAmount` and conversions), `Cip56` (token-standard helpers). Templates only; uploadable to a participant. |
| `test` (`ntt-test`) | The Daml Script test suite (`Test.TestNtt`) plus a copied subset of core test helpers (`Test.TestCore`, `Test.TestReplay`, `Test.MockToken`) so NTT runs end to end under `dpm test`. |
| `dars/` | Vendored DAR binaries: the pinned `wormhole-core` DAR and the CIP-0056 interface DARs. Provenance and sha256 in [`dars/README.md`](dars/README.md). |

The manager drives the CIP-0056 token-standard interfaces (`Holding`,
`TransferFactory`, `BurnMintFactory`) directly, so any conforming token works
without a per-token wrapper contract. (An earlier design had a bespoke
`NttToken` interface with per-token implementations; it was dropped for this
reason.)

## Build & test

Requires the `dpm` toolchain (Daml SDK 3.5.1, declared in every `daml.yaml`)
and a JDK. Both packages build against the vendored core and CIP-0056
interface DARs; core is never rebuilt here.

```sh
# from this directory
dpm build --all        # builds ntt, then test (in dep order)
cd test && dpm test    # runs the Daml Script suite
```

A live-sandbox Go integration harness (the recipient-binding match test) lives
in [`testing/go/`](testing/go/); see its build tag and comments to run it
against a `dpm sandbox`.

## Parties and the trust model

Three parties sign every `NttManager`, and each signature has one job:

- `operator` runs the core bridge instance and ties the deployment to it.
- `admin` is the deployer. It owns the deployment's transceiver `Emitter`,
  maintains the peer table, and scopes the deployment's replay protection
  (VAAs are consumed with `consumer = admin`).
- `guardianGovernance` (`gg`) is the guardians' k-of-n threshold party, the
  same party that anchors the core and receives message fees. It owns the
  lock/unlock reserves and administers burn/mint instruments, so its signature
  is the custody and mint authority.

`gg` acts exactly once: a guardian quorum ceremony (the same external-signing
flow that creates the genesis `CoreState`) creates the `NttGovernance` root.
Everything afterwards runs on inherited authority. `RegisterManager` lends the
root's signatures to create managers, and the manager's choice bodies lend the
manager's signatures to move tokens. Value can only move inside fixed template
code, and every inbound movement first verifies and consumes a VAA, so no
single party can move bridged value, and nobody has to co-sign at transfer
time. That is what keeps both deployment and relaying permissionless.

A rogue movement of the reserve would take either a native spend by the
guardian quorum or a new `gg`-signed contract, which is also a quorum act. The
model ultimately rests on `gg`'s namespace being quorum-governed at the
topology layer, so its threshold cannot be quietly lowered. That is a
deployment requirement; the contracts cannot check it.

## Deployment

Anyone can stand up a deployment: bring a CIP-0056 token and pick a mode,
lock/unlock or burn/mint. The deployer first registers a core `Emitter` (the
transceiver), then exercises `RegisterManager` on the disclosed
`NttGovernance` root. This allocates a stable `managerId` and creates the
manager in one transaction, with no approval step: the caller supplies the
`admin` signature and the root supplies `operator`'s and `gg`'s. For
lock/unlock it also creates the deployment's `LockedLedger` at balance zero.
For burn/mint it checks the mint capability: `gg` must administer the
instrument, and the instrument id must be bound to the registering admin (see
"Instrument binding" below).

Two key-derived identities, both owner-bound (see the core README's message
publishing section):

- transceiver address =
  `keccak256("wormhole:emitter:v1" ‖ operator ‖ admin ‖ emitterId)`. This is
  the VAA emitter other chains register as the peer. It is derived by the
  watcher, not stored, and must be a single shared `Emitter` per deployment,
  since its address is the peer identity.
- manager address =
  `keccak256("wormhole:ntt-manager:v1" ‖ operator ‖ admin ‖ managerId)`,
  computed at registration and stored in `managerAddress`. The distinct domain
  tag keeps an emitter and a manager with the same ordinals from colliding.
  `admin` is in the preimage and co-signs, so a compromised operator cannot
  forge an existing manager's address to consume that deployment's inbound
  VAAs.

Because registration is permissionless, anyone can create junk managers
co-signed by `gg` and the operator. They are inert (their choices cannot move
anything the creator could not already move), but they do occupy the
signatories' participants; that is ordinary spam, bounded by synchronizer
costs rather than by this package.

## Wire codec (`Wormhole.Ntt.Payload`)

Big-endian encoders for the three nested structures (`NativeTokenTransfer`,
prefix `0x994E5454`; `NttManagerMessage`; `WormholeTransceiverMessage`, prefix
`0x9945FF10`). Amounts travel as the NTT `TrimmedAmount`: an integer plus a
scale byte, at most 8 decimals. Round-trips are covered by
`TestNtt:testNttCodec`.

## Send (`NttManager.Transfer`)

`Transfer` is controlled by `user`, the token owner, alone; no admin or
operator involvement. It trims the amount to the wire form, moves the user's
tokens (lock or burn, see "Custody" below), assembles the nested message, and
publishes it through the deployment's transceiver with
`Emitter.PublishMessage`. The guardian watcher therefore observes an ordinary
core message; there is no NTT-specific watcher code.

The choice is nonconsuming. Everything its body exercises runs on the
manager's inherited signatory authority, while the CIP-0056 factory
independently checks that the input holdings belong to `user`, so a caller can
never lock or burn a holding it does not own. The message id is the
transceiver's next sequence number, read from the fetched `Emitter` in the
same transaction that publishes, so it equals the resulting VAA's sequence and
is unique per transceiver.

The sender pays the message fee itself. `PublishMessage` charges the
governance-set `messageFee` to its `payer`, and `Transfer` passes
`payer = user`. As payer, the user co-controls the nested `PublishMessage`,
which is what lets the fee be drawn from the user's own allocation
(`feeAllocation`; pass `None` at fee 0). An NTT send is never fee-exempt
(`TestNtt:testNttSend`, `testNttTransferByDifferentUser`).

The user is not a stakeholder of the manager, the transceiver, the
`CoreState`, the `LockedLedger`, or the registry factory, so a submission
attaches them as explicit disclosures. This is a visibility requirement, not
an authorization one.

## Receive (`NttManager.Release` / `NttManager.Mint`)

Inbound delivery is permissionless: any `executor` (a relayer, the recipient,
anyone) submits with only disclosed contracts. The executor's identity carries
no authority; verification is the gate. Both inbound choices run the same
checks, in order:

1. Pin the disclosed `CoreState` to the deployment's committed
   `guardianGovernance` (via `GetGuardianGovernance`). Pinning the operator
   instead would not help: a compromised operator can co-sign a forged
   `CoreState` under a throwaway gg party, but cannot forge the real `gg`'s
   signature (`TestNtt:testReceivePinsGuardianGovernance`).
2. Verify the VAA's guardian signatures and consume its digest, atomically,
   with the core `VerifyAndConsumeVAA` (the per-consumer replay trie, scoped
   to `admin`, so each deployment consumes a VAA at most once).
3. Check the VAA emitter is the configured peer transceiver for its source
   chain, and that the nested message's source and recipient manager addresses
   match the peer and this deployment.
4. Bind the caller-supplied `recipient` to the VAA's `recipientAddress` (see
   below), so a relayer cannot steer the payout.

`Release` (lock/unlock) then debits the deployment's `LockedLedger`, which
rejects any amount above the balance, and transfers custody holdings from `gg`
to the recipient. `Mint` (burn/mint) mints through the recipient's standing
`DepositPreapproval`. Any submitter can only cause the VAA-bound transfer to
the VAA-bound recipient. The received wire amount is rescaled to the local
token's decimals before paying out, truncating precision the token cannot
represent, so a receive never pays out more than was sent.

**The deposit pre-approval.** A CIP-0056 holding is signed by its owner, and a
Daml choice body carries only the exercised contract's signatories plus the
choice's controllers, never the submission's `actAs` set. A relayer therefore
cannot supply the recipient's signature for a mint, and the recipient may be
offline. The recipient instead signs once, up front: it creates a
`DepositPreapproval` (a single self-signed create, the NTT analog of Canton
Coin's `TransferPreapproval`). `Mint` runs the mint inside that contract's
`Deposit` choice, where the recipient's stored signature and `gg`'s
instrument-admin authority (supplied by the manager) are both in scope. A
missing, revoked, or mismatched pre-approval aborts the whole transaction
before the VAA digest is consumed, so the same VAA is deliverable once the
recipient opts in. `Deposit` is nonconsuming (one approval, unlimited
deliveries) and only the owner can `Revoke`.

**Recipient binding.** A Canton `Party` id has no fixed-size form to put in
the VAA's 32-byte `recipientAddress`, so the sender hashes it:
`recipientAddressFor recipient =
keccak256("wormhole:ntt-recipient:v1" ‖ lp(partyToText recipient))`. The
inbound path recomputes the hash from the caller-supplied `recipient` and
checks it against the VAA. The binding lives inside the guardian-signed VAA,
so no relayer can redirect it. The deliberate tradeoff: a Party id is
permanent, so a VAA is bound to the recipient party as it existed when the
sender hashed it, and a recipient who needs a different party (lost key,
custodial migration) needs a re-send. The encoding is pinned by
`testRecipientAddressForVector`; the matching-recipient path is covered by the
live harness in `testing/go/` (a static fixture cannot name a freshly
allocated Party).

## Token integration and custody

The manager calls the CIP-0056 factories directly: `TransferFactory` for
lock/unlock, `BurnMintFactory` for burn/mint. The registry's factory cid is
never stored on the ledger. Callers resolve the registry's current factory
off-ledger and pass it as `registryCid` on every operation, the same way
`coreStateCid` is passed, so a registry can replace its factory without
stranding anything.

**Custody sits with the guardian quorum.** Bridged value is owned by `gg`, not
by the deployment admin, who could otherwise drain reserves or mint at will.
The manager's choice bodies inherit `gg`'s authority for the custody transfer
or mint, and every inbound choice is VAA-gated, so the reserve moves only
against a valid, replay-protected VAA. This adds no new trusted party: it is
the same guardian quorum that VAA verification already trusts.

**The locked ledger.** All lock/unlock deployments deposit into the same `gg`
pot, and Wormhole attests only that an emitter emitted bytes, not that a
transfer is backed. A deployment that trusts a hostile peer must therefore not
be able to reach other deployments' collateral. Each deployment's
`LockedLedger` is credited on every lock and debited on every release, and
`Debit` rejects amounts above the balance, so a deployment can release at most
what it locked. The cap is sound even though the pot is fungible: credits and
debits are atomic with the corresponding deposit and release, so the sum of
all ledger balances never exceeds `gg`'s physical holdings, and every
deployment's cap stays satisfiable no matter which holdings a release spends.
One consequence of the shared pot: `gg` also custodies message fees, so a
release may physically spend fee holdings of the same instrument. That is
value-neutral (change returns to `gg` and the cap still binds), but worth
knowing when auditing `gg`'s holdings.

**Instrument binding (burn/mint exclusivity).** Burn/mint deployments are
isolated by instrument: a manager only ever mints its own `instrumentId`. That
is only meaningful if an instrument belongs to at most one deployment, and
registration is permissionless, so exclusivity has to be enforced, not
assumed. It cannot be enforced by lookup (no contract keys, so "nobody has
claimed this instrument yet" is not an on-ledger-checkable fact), and a
claimed-instruments set on the root would grow without bound. Instead the
binding lives in the instrument's identity: `RegisterManager` requires
`instrumentId.id == nttInstrumentIdFor admin nonce`, a hash commitment to the
registering admin. A CIP-0056 instrument id is fixed at creation, so whoever
creates the instrument decides, once and forever, which admin can bind a
manager to it. This is the role EVM NTT's `setMinter` pointer plays, attached
to the immutable instrument identity instead of a mutable token field
(`TestNtt:testBurnMintInstrumentIsExclusiveToItsAdmin`). Two consequences: the
admin party must be durable (the binding cannot be re-pointed; recovery from a
lost admin party means a new instrument and a holder migration), and the
instrument must be created with the binding already in its id, which is a
deployment-setup requirement documented rather than enforced here.

**Synchronous settlement.** Lock and release require the registry's
`TransferFactory_Transfer` to settle in the same transaction; a `Pending` or
`Failed` result aborts, so a two-step registry flow can never leave the bridge
thinking value moved when it did not. Delivering to an arbitrary receiver (the
custody target on lock, the recipient on unlock) still depends on
registry-level receiver pre-approval (for example Amulet's own
`TransferPreapproval`), which is outside NTT's control.

## Contention and contract churn

The manager itself is consumed only by `SetPeer`, so its cid is stable between
peer changes and disclosures of it stay valid. What serializes is:

- Sends, on the transceiver `Emitter` (consumed by every publish). Inherent to
  Wormhole sequence numbering.
- Lock-mode value movement, on the `LockedLedger` (both `Transfer` and
  `Release` touch it). Burn/mint deployments have no ledger, so their sends
  contend only on the emitter and their mints only on the replay trie.
- VAA consumption, on the covering replay-trie node.

A submission that loses a race re-resolves the fresh cid and retries, the same
fail-closed pattern the core publish path uses.

## Follow-ups

- Never-opted-in recipients: a pending+claim fallback (mint a pending
  instruction the recipient later claims) would let delivery proceed before
  opt-in. Deliberately out of scope for now.
- Foreign-admin lock/unlock against a real registry: the tests collapse the
  reserve owner (`gg`) and the lock instrument's admin onto one party because
  the mock factory cannot model a real registry's receiver pre-approval. A
  real lock/unlock deployment bridges a token whose admin is not `gg` and
  relies on that registry's `TransferPreapproval`; exercising that end to end
  is future work.
- Inbound rate limits (EVM NTT parity): a governance-set cap on inbound
  release/mint rate would bound the damage from a compromised peer beyond the
  `LockedLedger` cap. Not required for isolation, so deferred.
- Sharding the `LockedLedger` if lock-mode send/release contention ever
  matters in practice.
- Hint-free verification (persist guardian pubkeys) so the inbound path needs
  no `pubKeys` hints. Core-wide, tracked in the core repo.
- The permanent recipient-address binding (see "Recipient binding").
