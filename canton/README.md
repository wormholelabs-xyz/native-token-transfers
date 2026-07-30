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
off-ledger and passed as an explicit, disclosed contract id, then validated
on-ledger by its contents against manager-committed state. This is deliberate,
not a platform gap. Contract keys exist from SDK 3.5 (Daml-LF 2.3), but they
are non-unique on Canton 3.x — several live contracts may share a key, negative
lookups are not validated, and key resolution prefers contracts created in the
submitting transaction, so a submitter could shadow the intended contract with
a same-key decoy minted in the same submission. Content validation against
signatory-protected state is immune to that, and works identically for
disclosed contracts. Background for the core concepts referenced below
(message publishing and fees, the replay trie, disclosed-cid submission, the
trust model) is in the core `canton/README.md`.

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
- `admin` is the operational role: it maintains the peer table, rotates the
  committed factory, and owns the lock/unlock reserve. It is transferable
  (`TransferAdmin`, usually via the propose-accept `AdminTransferProposal`);
  handing it to `gg` is the guardian quorum's custody opt-in. In production
  this handoff is completed by relaying a guardian-signed governance VAA
  (`AcceptAdminTransferByVaa`), not by a live `gg` signature — see "Custody
  follows the admin" below.
- `guardianGovernance` (`gg`) is the guardians' k-of-n threshold party, the
  same party that anchors the core and receives message fees. It administers
  burn/mint instruments, owns the deployment's transceiver `Emitter`, is the
  replay-trie consumer for every deployment, and a deployment whose admin role
  was handed to it has a quorum-owned reserve and governance-run admin
  operations.

The deployment's stable identity is no longer a co-signing party. It is a
derived `namespace : Text`, computed once at registration and stored on the
manager:

```
namespace = "ntt:" <> keccak256(tag ‖ lp(instrument.admin) ‖ lp(instrument.id) ‖ lp(registeringAdmin) ‖ managerId)
```

(`lp` is the 4-byte length prefix used throughout this package's address
preimages.) It sub-scopes `gg`'s replay trie — inbound VAAs are consumed with
`consumer = gg, namespace = mgr.namespace` — and never changes, but unlike a
party it needs no ceremony to mint or safeguard: `gg` is already a lifetime
signatory of every manager, so the derived text rides along on `gg`'s
inherited authority. The registering admin in the preimage binds the
deployment's identity to its creator, and `managerId` guarantees exactly one
namespace per deployment (closing the corner case where two deployments would
otherwise share both instrument and registering admin).

`gg` acts exactly once: a guardian quorum ceremony (the same external-signing
flow that creates the genesis `CoreState`) creates the `NttGovernance` root.
Everything afterwards runs on inherited authority. `RegisterManager` lends the
root's signatures to create managers — and, atomically, to mint the
deployment's `gg`-owned transceiver `Emitter` and claim its `gg`-scoped replay
root — and the manager's choice bodies lend the manager's signatures to move
tokens. Value can only move inside fixed template code, and every inbound
movement first verifies and consumes a VAA, so no single signatory can move
bridged value, and nobody has to co-sign at transfer time. That is what keeps
both deployment and relaying permissionless. The custody opt-in is no
exception: `AcceptAdminTransferByVaa` composes `gg`'s side of the handoff from
authority the manager's own signatories already carry (see the module header),
so `gg` never signs live to accept a deployment — genesis remains its only
live signature.

A rogue movement of a `gg`-owned reserve would take either a native spend by
the guardian quorum or a new `gg`-signed contract, which is also a quorum act.
The model ultimately rests on `gg`'s key namespace being quorum-governed at
the topology layer, so its threshold cannot be quietly lowered. That is a
deployment requirement; the contracts cannot check it.

## Deployment

Anyone can stand up a deployment: bring a CIP-0056 token and pick a mode,
lock/unlock or burn/mint. The deployer exercises `RegisterManager` on the
disclosed `NttGovernance` root, passing along the core's `EmitterRegistry` and
`ReplayRootRegistry` anchors (also disclosed). This one transaction:
allocates a stable `managerId`; mints the deployment's transceiver `Emitter`
(owned by `gg`, `gg`'s requester authority inherited from `NttGovernance`);
claims the deployment's replay-trie root (`consumer = gg`, sub-scoped by the
derived `namespace`); and creates the manager — with no separate approval
step. The caller supplies only the `admin` signature; the root supplies
`operator`'s and `gg`'s, and `admin` co-controls (pays) both nested core
onboarding fees so `gg` is never made to fund a deployment's registration. For
lock/unlock it also creates the deployment's `LockedLedger` at balance zero,
with the reserve starting admin-owned (see "Custody follows the admin"
below). For burn/mint it checks the mint capability: `gg` must administer the
instrument, and the instrument id must be bound to the registering admin (see
"Instrument binding" below).

Two key-derived identities, both bound to the REGISTERING admin — present as a
controller at registration, and whose signature co-signs the created manager —
so they survive later admin handoffs (see the core README's message
publishing section):

- transceiver address =
  `keccak256("wormhole:emitter:v1" ‖ operator ‖ gg ‖ emitterId)`. This is the
  VAA emitter other chains register as the peer. It is derived by the
  watcher, not stored, and must be a single shared `Emitter` per deployment,
  since its address is the peer identity. (The preimage's owner ordinal is
  `gg`, not the registering admin — `gg` co-signs every `Emitter` it mints.)
- manager address =
  `keccak256("wormhole:ntt-manager:v1" ‖ operator ‖ registeringAdmin ‖ managerId)`,
  computed once at registration and stored in `managerAddress`. The distinct
  domain tag keeps an emitter and a manager with the same ordinals from
  colliding. The registering admin's signature co-signs the manager at
  creation, so a compromised operator cannot forge an existing manager's
  address to consume that deployment's inbound VAAs; because the address is
  computed once and stored, not recomputed, it is unaffected by any later
  `TransferAdmin`.

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

The same module also codes NTT's own governance packets — `module(32)
"Ntt" ‖ action(1) ‖ chain(2) ‖ <action-specific>`, under the module id `"Ntt"`
so an NTT governance VAA can never be confused with (or replayed as) a core
one. `NttGovernanceAction` is a genuine sum type over the four actions
guardians can sign: accept an admin handoff
(`AcceptAdminToGovernance`, `NttManager.AcceptAdminTransferByVaa`), co-sign a
gg-minted burn/mint registration (`RegisterBurnMintManager`,
`NttGovernance.RegisterManagerByVaa`), vet a factory rotation
(`RotateToCanonicalFactory`, `NttManager.SetFactoryByVaa`), and edit a
gg-adminned deployment's peer table (`SetPeerByGovernance`,
`NttManager.SetPeerByVaa`). Every call site dispatches on the parsed
constructor exhaustively and aborts on the wrong one, even where two actions
share an identical byte layout (accept-admin and rotate both do) — a
guardian-signed VAA for one purpose must never satisfy a different choice
just because its bytes happen to parse.

Two further payloads are published bare (no `0x9945FF10` framing) at the core
bridge and are consumed only by the off-chain NTT Global Accountant — no chain
has a receive path for either: `TransceiverRegistration` (`0x18fc67c2`, 38
bytes), published as part of every `SetPeer`, and `TransceiverInit`
(`0x9c23bd3b`, 70 bytes), published once as part of `RegisterManager`. See
"Accountant broadcasts" below.

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
`CoreState`, the `LockedLedger`, the custody pot holding, or the registry
factory, so a submission attaches them as explicit disclosures. This is a
visibility requirement, not an authorization one.

## Accountant broadcasts (bundled into `SetPeer` / `RegisterManager`)

`TransceiverRegistration` and `TransceiverInit` are no longer published by
dedicated choices; they publish as a bundled effect of the choices that
already change the state each payload attests to. There is no standalone
broadcast choice and no separate re-broadcast path.

- `SetPeer` (`controller admin, payer`, consuming) publishes
  `TransceiverRegistration` (prefix `0x18fc67c2`, 38 bytes: `prefix(4) ‖
  peerChain(2) ‖ peerAddress(32)`) every time it runs — the first peer for a
  chain and every replacement alike — at nonce 0. `payer` is an explicit
  parameter, not hardcoded to `admin`: the direct admin path calls it with
  `payer = admin` (self-funding, unchanged in effect from before `payer`
  existed), while `SetPeerByVaa` (below) calls it with `payer = executor`, so
  the relayer funds the governance path's fee instead of `gg`. `payer` must
  co-control because Daml authorization does not carry a choice's controllers
  past a nested exercise on the same contract; without it the nested
  `PublishMessage` (`controller owner, payer`) would lose the caller's
  authority the moment a wrapper choice like `SetPeerByVaa` calls in here. It
  exercises the deployment's transceiver `Emitter` in the same transaction as
  the peer-table update and returns the resulting `WormholeMessage` alongside
  the new manager cid.
- `RegisterManager` publishes `TransceiverInit` (prefix `0x9c23bd3b`, 70 bytes:
  `prefix(4) ‖ managerAddress(32) ‖ mode(1) ‖ tokenAddress(32) ‖ decimals(1)`)
  exactly once, at registration, also at nonce 0 and `admin`-paid — this
  deployment's manager address, mode (`0` = lock/unlock, `1` = burn/mint),
  token address, and decimals.

Re-announcing a registration means re-exercising `SetPeer` with the same peer
values (see the divergence note below); `RegisterManager`'s init broadcast has
no re-play path at all, since registration itself runs only once per
deployment. An accountant that needs an already-signed VAA again simply
re-fetches it from the guardian API rather than asking the chain to re-emit
it. Neither publish checks `paused` (pause blocks value movement, not
administration), and neither touches the `LockedLedger`, factory, or any
holding.

**Divergence from EVM.** EVM's transceiver peer is immutable once set
(`PeerAlreadySet`), specifically to keep the accountant's bookkeeping simple.
Canton's peer stays replaceable: `SetPeer` always accepts a new value for
`peers[chainId]` and always re-broadcasts. That divergence is not
accounting-neutral. The accountant's peer entries are write-once per
`(emitter, peer chain)`
(`ntt-global-accountant/src/contract.rs` rejects a second registration with
`"peer entry for this chain already exists"`) and `TRANSCEIVER_PEER` is read on
every transfer observation, with a cross-registration check that bails
`"peers are not cross-registered"` if the two sides disagree. So after a peer
rotation W → X, the accountant keeps W forever: inbound transfers from the new
peer X fail `MissingHubRegistration` (X was never recorded), while outbound
transfers to X still resolve — and cross-check — against the stale W entry.
The corridor's accounting for that chain is now permanently wrong, with no
on-chain repair path, since `transceiverEmitterId` is immutable. This does not
argue for reverting to write-once peers on Canton (that would neuter
`SetPeerByVaa`, whose entire purpose is quorum-driven peer edits, and local
routing correctness matters more than accountant bookkeeping) — the honest
framing is that a peer rotation is a **deployment-level event**, exactly as it
is on EVM, where the answer is "deploy a new transceiver": Canton's equivalent
is a new deployment (new manager, new emitter), because the emitter identity
never changes in place
(`TestNtt:testSetPeerReplacementBroadcastsAgain` pins the local-routing half of
this; the accountant-side consequence is operational, not tested here). Note
also that the accountant only processes `TransceiverInit` from lock/unlock
(hub) managers; burn/mint spokes are registered through the hub's own peer
registrations.

**Token address.** A CIP-0056 `InstrumentId` (`admin : Party, id : Text`) has
no native 32-byte form, so `TransceiverInit.tokenAddress` uses
`keccak256("wormhole:ntt-token:v1" ‖ lp(adminText) ‖ lp(id))`, the same
length-prefixed-hash convention as the manager and transceiver address
derivations above. It is computed once at `RegisterManager` and stored on the
manager (`tokenAddress`), not recomputed, so it survives `TransferAdmin`
exactly like `managerAddress`. Pinned by
`TestNttVectors:testVectorTokenAddress`.

**Shared sequence counter.** Both payloads publish through the same
transceiver `Emitter` as `Transfer`, so registration/init messages interleave
with transfer messages in the same sequence space. `RegisterManager`'s
`TransceiverInit` claims sequence 0, so a fresh deployment's emitter sequence
is already at 1 before its first `Transfer`, and every subsequent `SetPeer`
bumps the counter again. Any consumer must discriminate by the 4-byte payload
prefix (`0x9945FF10` transfer, `0x18fc67c2` peer registration, `0x9c23bd3b`
init) and must never assume a sequence number implies a transfer.

**Governance-gated peer edits (`SetPeerByVaa`).** A gg-adminned deployment
(admin handed to `guardianGovernance`, see "Custody follows the admin" below)
can have its peer table edited by guardian quorum vote instead of a live
`admin` action. `SetPeerByVaa` (`controller executor`, nonconsuming; NTT
governance action 4, a 142-byte packet: `module(32) ‖ action(1)=4 ‖ chain(2)
‖ managerAddress(32) ‖ peerEpoch(8) ‖ peerChain(2) ‖ peerManagerAddress(32) ‖
peerTransceiverAddress(32) ‖ peerDecimals(1)`) runs the same verification
ladder as `AcceptAdminTransferByVaa`: the disclosed `CoreState` pinned to this
deployment's own guardian trust anchor and operator; the VAA verified and its
digest consumed into `gg`'s replay trie, sub-scoped by the deployment's
`namespace` (governance and transfer VAAs share one trie, so a governance VAA
can never be replayed as, or by, a transfer one); signed by the CURRENT
guardian set only; and the standard emitter chain/address and target
chain/managerAddress checks. `peerEpoch` mirrors action 1's `factoryEpoch`: it
must equal the manager's current `peerEpoch` (bumped by every `SetPeer`), so a
withheld or superseded peer VAA can never reinstate a since-replaced peer. It
additionally requires `admin == guardianGovernance` before nesting a `SetPeer`
exercise (`payer = executor`, so the relayer self-funds the broadcast fee, not
gg) with the parsed peer values, so a governance-driven edit broadcasts
exactly like an admin-driven one. Any `executor` (a relayer) can submit; the
VAA is the only authority that matters.

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
   with the core `VerifyAndConsumeVAA` (`consumer = gg`, sub-scoped to the
   deployment's derived `namespace`, so each deployment consumes a VAA at most
   once, independently of every other deployment under the same `gg`, and the
   scope is unaffected by an admin handoff).
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
lock/unlock, `BurnMintFactory` for burn/mint.

**The factory is deployment config, not per-call input.** A registry's factory
is a stateless service contract — a reified dictionary — whose choices are all
nonconsuming, so its cid is stable and changes only when the registry itself is
upgraded. The manager therefore commits the factory cid at registration and
uses that committed cid for every lock, burn, unlock, and mint; `admin`
refreshes it with `SetFactory` on the rare occasion of a registry upgrade.

This is what stops an untrusted caller from substituting a look-alike factory.
Because holdings are interface-only, the factory is the sole actuator of any
token movement, and a caller-supplied cid can't be authenticated on-ledger (an
interface view is implementation-controlled and can lie, and there are no
contract keys). If the sender chose the factory, a malicious one could report a
lock or burn that never happened while the manager still emitted a genuine,
value-bearing VAA — bridge inflation. Committing the factory removes caller
choice: the trust collapses to the deployment `admin`, which every peer already
trusts by registering it as a peer (the same trust EVM and Solana NTT place in
a peer's configured token), and whose reach ends at its own reserve (see
below). On a gg-adminned deployment the committed factory is the one the
quorum vetted when it accepted the role, and rotation is automatically a
governance action. The other residual is liveness: if the registry rotates its
factory before the committed cid is refreshed, transfers pause (they never
mis-settle).

**Custody follows the admin.** A deployment registers with its reserve owned
by its own `admin`: fully operational and explicitly custodial — the admin
owns the reserve outright, so users trust it the way they would a custodial
bridge, and wallets should surface who the admin is. The role is handed over
with `TransferAdmin`, normally as propose-accept: the current admin leaves a
standing `AdminTransferProposal`, and the new admin accepts at its own pace,
naming the committed factory it vetted; the pot moves with the role. Handing
the role to `gg` is the guardian quorum's custody opt-in: the reserve becomes
quorum-owned — the old admin can no longer touch it, every inbound movement is
VAA-gated, and all admin operations (peers, factory rotation, a further
handoff) become governance actions. The handoff changes nothing on the wire:
the manager and transceiver addresses are bound to the registering admin (not
the current one), the replay scope is the derived `namespace` under `gg` (both
unaffected by who currently holds `admin`), and the transceiver `Emitter`'s
owner authority is `gg`'s own, inherited from the manager's signatories rather
than from `admin` — so peers, in-flight VAAs, ledger accounting, and outbound
sends are all unaffected by the handoff. The one on-ledger side effect is
bookkeeping: the `LockedLedger` carries the current admin as an observer (pure
visibility — the admin sees its own reserve accounting without a disclosure
round-trip), so the handoff recreates it once to move that observership to the
incoming admin. The same choice serves plain admin succession between ordinary
parties, and lets `gg` hand the role back.

**Governance runs by VAA, not live signature.** The admin handoff above needs
no live `gg` signature: any permissionless relayer submits a guardian-signed
`AcceptAdminToGovernance` VAA against the current admin's standing
`AdminTransferProposal` (`NttManager.AcceptAdminTransferByVaa`), and `gg`'s
authority is inherited from the manager's own signatories rather than
supplied live. The same pattern extends to the two other points where a
gg-minted deployment would otherwise need `gg`'s live signature: standing up
the deployment at all (`NttGovernance.RegisterManagerByVaa`, where the
guardians co-sign the registration alongside the admin instead of `gg`
appearing in the controller list) and rotating the committed factory
(`NttManager.SetFactoryByVaa`). Both are VAA-gated the same way as the admin
handoff — current-guardian-set-only verification, the deployment's own replay
trie, and a factory template-pin to the canonical, gg-administered
`NttCoinFactory` (a VAA cannot name a contract id, so a ByVaa path can only
ever commit the one factory `gg` deployed at genesis) — so a gg-minted
deployment needs no live `gg` signature at any point after the genesis
`NttGovernance` ceremony: register, set peers, and hand admin to `gg` all flow
through signed VAAs. Before that admin handoff runs, the deployment is still
admin-trust — the registering admin controls `SetPeer` regardless of who
authorized the registration — so guardians intending a fully quorum-owned
deployment from genesis should sign the registration and handoff VAAs in the
same ceremony.

**Why the partition is by owner.** Isolation between deployments follows from
ownership, not from accounting or holding topology. Reserves of distinct
admins cannot mix, no matter what any factory reports: a deployment that
commits a dishonest factory can inflate only its own ledger and drain only its
own admin-owned reserve. The `gg` pool is shared only among deployments whose
admin role the quorum accepted, whose factories it vetted. Ownership is also
the one partition a registry cannot churn away: registries reorganize holdings
out from under the bridge (Canton Coin's rounds merge, expire, and recreate
contracts), which erases any partition built out of specific contract ids — a
pinned pot cid, an escrow — but never changes who owns the value. Finally,
because every choice body carries all four signatures, the manager checks that
each custody holding it spends is owned by the deployment's own current
`admin`, so the ambient `gg` authority in another deployment's choices can
never reach the `gg` pool.

**The locked ledger.** Wormhole attests only that an emitter emitted bytes,
not that a transfer is backed. A deployment that trusts a hostile peer must
therefore not be able to release more than it locked: each deployment's
`LockedLedger` is credited on every lock and debited on every release, and
`Debit` rejects amounts above the balance. Within the `gg` pool, where
reserves of one instrument are fungible across gg-adminned deployments, this
cap is what keeps a hostile-peer deployment away from the others' collateral:
credits and debits are atomic with the corresponding deposit and release, so
the sum of those balances never exceeds `gg`'s physical holdings, and every
deployment's cap stays satisfiable no matter which holdings a release spends.
One consequence of the shared pool: `gg` also custodies message fees, so a
release may physically spend fee holdings of the same instrument. That is
value-neutral (change returns to `gg` and the cap still binds), but worth
knowing when auditing `gg`'s holdings.

**The custody pot (single-holding invariant).** Custody holdings are visible
only to their stakeholders (the custodian and the registry admin), so an
executor can only spend what someone discloses to it. To avoid an off-chain
index and coin selection over custody fragments, the manager keeps each
deployment's custody consolidated into one holding, and the ledger carries a
pointer to it (`custodyHoldingCid`): a lock merges the fresh deposit into the
pot with a custodian self-transfer, and a release spends the pot and records
the single change holding as the new pot. The pointer updates in exactly the
transactions the ledger already changes in, so a relayer's disclosure bundle
(manager, ledger, pot) is complete and self-refreshing. The pointer is a hint,
not a trust anchor: every holding is validated at use time as the current
custodian's own, unlocked, of this instrument; `Release` accepts explicit
holding cids as an override (validated the same way); and the permissionless
`ConsolidateCustody` choice re-merges fragments and repoints the hint after a
registry-side reorganization (registries may restructure their holdings
without us, which is why the cid can go stale and why storing it is safe only
as a hint). One open consideration for real registries: transfer fees charged
out of custody inputs would bleed the physical pot below the ledger sum, so a
fee-charging registry needs a per-registry answer (fee waivers, top-ups, or
debiting fees from the ledger).

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
custodian on lock, the recipient on unlock) still depends on registry-level
receiver pre-approval (for example Amulet's own `TransferPreapproval`), which
is outside NTT's control. One practical upside of admin custody: the lock's
receiver is the admin itself, which can self-serve that pre-approval; a
gg-adminned deployment needs it arranged for `gg`, which is part of what the
quorum takes on by accepting the role.

## Contention and contract churn

The manager itself is consumed by its config choices (`SetPeer`, `SetFactory`,
and the rare `TransferAdmin`), so a config edit still leaves the manager cid
stable across ordinary `Transfer` traffic (which is nonconsuming). What
serializes is:

- Publishing, on the transceiver `Emitter` (consumed by every
  `PublishMessage`). This is no longer sends-only: `SetPeer` bundles its
  registration broadcast through the same emitter (see "Accountant
  broadcasts"), so a peer edit now contends with `Transfer` traffic on the
  identical contract, not a separate one. `RegisterManager`'s one-shot init
  broadcast contends the same way, but only once, at registration.
- Lock-mode value movement, on the `LockedLedger` and the custody pot holding
  (both `Transfer` and `Release` touch them, in the same transactions).
  Burn/mint deployments have no ledger, so their sends contend only on the
  emitter and their mints only on the replay trie.
- VAA consumption, on the covering replay-trie node.

A submission that loses a race re-resolves the fresh cid and retries, the same
fail-closed pattern the core publish path uses. This is a liveness cost, not a
correctness one — but it is griefable: a dust-transfer spammer can keep the
emitter churning fast enough that an urgent peer repoint (say, retiring a
compromised peer) keeps losing the race, and the admin is not an `Emitter`
stakeholder, so each retry needs a fresh off-ledger disclosure round-trip.
The runbook for an urgent peer change is therefore **pause first, then
`SetPeer`**: `SetPaused` (`admin`-controlled) takes no `CoreState`/`Emitter` at
all, so it can never lose this race, and once paused `Transfer` aborts
outright, which stops new contention on the emitter before the peer edit is
retried.

## Follow-ups

- Never-opted-in recipients: a pending+claim fallback (mint a pending
  instruction the recipient later claims) would let delivery proceed before
  opt-in. Deliberately out of scope for now.
- Foreign-admin lock/unlock against a real registry: the mock factory cannot
  model a real registry's receiver pre-approval, so the tests make `gg` every
  instrument's admin, run most lock tests with the sender as the deployment
  admin, and hand the admin role to gg first in the different-user lock test.
  A real lock/unlock deployment bridges a token whose admin is not `gg` and
  relies on that registry's `TransferPreapproval`; exercising that end to end
  is future work.
- Driving the VAA-gated governance actions (`AcceptAdminTransferByVaa`,
  `RegisterManagerByVaa`, `SetFactoryByVaa`) from the playground CLI
  (`vaagen`, `ntt-playground` deploy/accept commands) — playground-only
  tooling, tracked separately there.
- A LocalNet integration test driving `RegisterManagerByVaa`/
  `SetFactoryByVaa` end to end against real, freshly allocated Party ids
  (`canton/testing/go`): the registration binding is committed to Party text,
  so the happy path and the non-canonical-factory template-pin are reachable
  only there, not in Daml Script (see `TestNtt:testRegisterManagerByVaaBindingMismatch`).
- Inbound rate limits (EVM NTT parity): a governance-set cap on inbound
  release/mint rate would bound the damage from a compromised peer beyond the
  `LockedLedger` cap. Not required for isolation, so deferred.
- Sharding the `LockedLedger` if lock-mode send/release contention ever
  matters in practice.
- Hint-free verification (persist guardian pubkeys) so the inbound path needs
  no `pubKeys` hints. Core-wide, tracked in the core repo.
- The permanent recipient-address binding (see "Recipient binding").
