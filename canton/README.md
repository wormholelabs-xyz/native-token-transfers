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

| Directory (Daml package)                 | Contents                                                                                                                                                                                                                                                                                                                                                                              |
| ---------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `ntt` (`ntt`)                            | The protocol. `Manager` (the `NttManager`), `Governance` (the `NttGovernance` bootstrap root and manager registry), `Ledger` (per-deployment `LockedLedger` custody accounting), `Deposit` (the recipient's `DepositPreapproval`), `Payload` (wire codec), `Amount` (`TrimmedAmount` and conversions), `Cip56` (token-standard helpers). Templates only; uploadable to a participant. |
| `genesis` (`wormhole-genesis`)           | `Wormhole.Genesis`: the `WormholeGenesisOffer` template and its `AcceptGenesis` choice. One accept creates the core and NTT roots. Templates only; uploadable to a participant.                                                                                                                                                                                                       |
| `genesis-test` (`wormhole-genesis-test`) | The Daml Script tests of `wormhole-genesis` (`Test.TestAcceptGenesis`).                                                                                                                                                                                                                                                                                                               |
| `disclosure/service`                     | The production disclosure service: a Go binary that serves each flow's disclosure set over HTTP; see [`disclosure/README.md`](disclosure/README.md).                                                                                                                                                                                                                                  |
| `test` (`ntt-test`)                      | The Daml Script test suite (`Test.TestNtt`) plus a copied subset of core test helpers (`Test.TestCore`, `Test.TestReplay`, `Test.MockToken`) so NTT runs end to end under `dpm test`.                                                                                                                                                                                                 |
| `dars/`                                  | Vendored DAR binaries: the pinned `wormhole-core` DAR and the CIP-0056 interface DARs. Provenance and sha256 in [`dars/README.md`](dars/README.md).                                                                                                                                                                                                                                   |

The manager drives the CIP-0056 token-standard interfaces (`Holding`,
`TransferFactory`, `BurnMintFactory`) directly, so any conforming token works
without a per-token wrapper contract. (An earlier design had a bespoke
`NttToken` interface with per-token implementations; it was dropped for this
reason.)

## Build & test

Requires the `dpm` toolchain (Daml SDK 3.7.3, declared in every `daml.yaml`)
and a JDK 21. Each package builds against the vendored core and CIP-0056
interface DARs; core is never rebuilt here.

```sh
# from this directory
dpm build --all                # builds ntt, test, genesis and genesis-test
cd test && dpm test            # runs the NTT Daml Script suite
cd ../genesis-test && dpm test # runs the wormhole-genesis suite
```

A live-sandbox Go integration harness (the recipient-binding match test) lives
in [`testing/go/`](testing/go/); see its build tag and comments to run it
against a `dpm sandbox`.

## Versions and upgrades

For each release, do these steps:

1. Increase the `version:` field in `ntt/daml.yaml`.
2. Build the release DAR.
3. Copy the release DAR into `dars/`.
4. Record its commit, package id, and sha256 in `dars/README.md`.
5. Set the `upgrades:` field in `ntt/daml.yaml` to the new DAR.

With an `upgrades:` field, `damlc` compares each build with the last release.
The build fails when the new package is not upgrade-compatible with it.

One version number identifies one build. Do not use a version number again
for a different build. Participants use the package name and version to put
the upgrades of a package in order.

### Version 0.6.x is not upgrade-compatible

Version 0.6.2 builds against core 0.12.3 (`32abbcd2...`, SDK 3.7.3) and has
the package id
`3aa43a1c498bd8c13467a8d9438d568c1a325bc948119395195bc2fbcb427093`.
`wormhole-genesis` 0.2.2 builds against `ntt` 0.6.2 and core 0.12.3. It has
the package id
`4a69c9c01de4f71eecc442d3aa48f197fec850e49115f91f8ace3340cf99c510`.
Version 0.6.1 built against core 0.12.2 (`e162e0c0...`) and has the package
id `37753fce...`. Version 0.6.0 built against core 0.12.0 (`aec76d21...`) and has the package
id `79a1cc2c...`. `ntt/daml.yaml` has no `upgrades:` field. Use participants
that have no earlier `ntt` package.

Version 0.6.0 changes these values:

- Each byte value has a constrained type with one valid form: lowercase hex
  of an exact width. NTT uses the core types (`UniversalAddress`,
  `EmitterAddress`, `EncodedVaa`, `MessagePayload`, `VaaDigest`, `HexBytes`)
  and its own types (`NttManagerAddress`, `NttMessageId`,
  `NttRegistrationBinding`, `NttGovernanceRootIdentity`). On the Ledger API,
  each value is a variant: `{"tag": "<type>", "value": "<hex>"}`.
- The VAA choices have no `pubKeys` argument. `CoreState` holds each guardian
  public key.
- `NttGovernance` has a `rootIdentity`, and its replay record holds 32
  digests or fewer, with an eviction floor.
- The registration binding of a `RegisterBurnMintManager` VAA holds the root
  identity (tag `wormhole:ntt-registration:v2`).

Version 0.6.1 has the templates and choices of 0.6.0. A re-genesis merges the
NTT replay record with the core function
`tryGovernanceReplayRecordForRegenesisFromRecords`.

Version 0.6.2 adds the argument `claimedRootsNodeCid` to `RegisterManager`
and `RegisterManagerByVaa` (see "Deployment"). `wormhole-genesis` 0.2.2
creates the root of the `gg` claimed-roots tree at a first genesis. Its
`GenesisRoots` has the field `claimedRootsNodeCid`.

### Version 0.5.x is not upgrade-compatible

Version 0.5.0 removes the `operator` field and signatory from `NttGovernance`,
`NttManager` and `LockedLedger`. `gg` is the only NTT anchor. Version 0.5.0
builds against core 0.9.0 (`bba94798...`) and has the package id
`449925fb667ed1169fac61165fa236a03cc798a6b873bbc0c605ea062961d62e`.
An upgrade cannot remove a template field or a signatory. Thus
`ntt/daml.yaml` has no `upgrades:` field. Use participants that have no
earlier `ntt` package.

Version 0.5.0 changes these values:

- The manager address preimage holds `gg`, not the operator.
- The registration binding of a `RegisterBurnMintManager` VAA holds `gg`,
  not the operator.
- The transceiver address preimage holds `gg` as registrar (core 0.9.0).

### Operator independence

`gg` signs every NTT root and every manager. No NTT contract has the operator
as a signatory, a controller or an observer. Thus each NTT function continues
when the operator participant is offline or the operator key is lost.

| Function                                       | Parties that act                        |
| ---------------------------------------------- | --------------------------------------- |
| `RegisterManager`, lock/unlock                 | `admin`                                 |
| `RegisterManagerByVaa`, `gg`-minted burn/mint  | `admin`, with a guardian-signed VAA     |
| `Transfer`                                     | the user                                |
| `Release`, `Mint`, the VAA-gated admin choices | any relayer, with a guardian-signed VAA |

`RegisterManager` of a `gg`-minted burn/mint deployment needs `gg` as a
controller. Use `RegisterManagerByVaa` for it.

One `gg` is one bridge instance. The manager address and the replay namespace
do not hold another instance discriminator. The guardians must sign one
`NttGovernance` root for each `gg`.

The registration binding holds the `rootIdentity` of one `NttGovernance`
root. A first genesis sets it to
`keccak256(tag ‖ lp(utf8(gg)) ‖ uint64(ledger time in microseconds))`. A
re-genesis keeps the identity of the earlier root. A first genesis at another
ledger time makes a root with another identity. That root rejects each
registration VAA of the first root. Each first genesis creates a
claimed-roots tree, so two first geneses of one `gg` make two trees. A
registration claims its (`gg`, namespace) pair in the tree of the node that
the caller gives. Thus a lock/unlock `RegisterManager` registers one time in
each tree. Two first geneses at one ledger time, for example in one
transaction, also make two roots with one identity. Then one registration VAA
also registers one time in each tree. The two managers have one manager
address. This is a residual risk. The disclosure service then finds two
covering nodes and returns 500. One root for each `gg` is a guardian rule. Before the guardians sign a registration VAA, they read the
`rootIdentity` of their root.

NTT must be final at genesis. An upgrade-compatible NTT version needs only
the vetting of the new DAR on each `gg` host. A breaking NTT version needs a
new `NttGovernance` root, and thus one more `gg` transaction. That
transaction must archive the earlier root and create the new root. If the
earlier root stays active, both roots accept one registration VAA. Both
roots compute one namespace. The core rejects the second claim of that pair
in the one claimed-roots tree of the `gg`, with "digest already consumed".
For roots of `ntt` 0.6.2 and core 0.12.3, a re-genesis (`ReGenesisReplacing`,
see "Genesis") is this transaction. It replaces the core roots and the NTT
root together. `EarlierGenesisRoots` names the templates of these package
versions. Thus a breaking `ntt` or core version also needs a new
`wormhole-genesis` version that depends on both package versions.

A breaking core version needs a migration, not only a re-genesis. The
`NttManager` choices take contract ids of core 0.12.3 templates. Thus a
breaking core version also forces a breaking `ntt` version.

A breaking version of `wormhole-core`, `ntt` or `wormhole-genesis` uses a new
package name. Canton rejects some breaking changes under the same package
name, but not all. Core 0.8.3 changed signatories and kept the name. `damlc` gave
only a warning. The re-genesis needs the old and the new core versions vetted
on each `gg` host at the same time. Do not vet a breaking version with a force
flag. The Canton watcher accepts only the package name `wormhole-core`.
Thus a breaking core version also needs a `guardiand` release.

A re-genesis archives the earlier `CoreState`. Then the managers of the
earlier `ntt` version cannot check a VAA or publish a message. Thus each
deployment must move to the new versions. A re-genesis does not carry this
state:

- Core `ReplayNode`. The claimed-roots tree records each (`gg`, namespace)
  pair. The trie of each deployment records each inbound VAA that the
  deployment consumed.
- Core `Emitter`. Each emitter holds its owner, its emitter id and its next
  sequence. The watcher derives the emitter address from the registrar, the
  owner and the emitter id.
- NTT `NttManager`. Each manager holds its namespace, manager id, manager
  address, transceiver emitter id, peers, `peerEpoch`, `factoryEpoch` and
  `paused`.
- NTT `LockedLedger`. Its balance is the release limit of a lock/unlock
  deployment.
- Core `FeeWithdrawalAuthorization`. An active authorization does not move
  to the new core version. Archive it before the unvet. The fee withdrawal
  steps below give the order.

If the new contracts start without the earlier records, these failures are
possible:

- A new trie has no record of the consumed VAAs. Then each earlier inbound
  VAA can mint or release a second time.
- A new claimed-roots tree has no record of the earlier pairs. Then a second
  claim of each earlier pair is possible.
- A new emitter with the earlier emitter address and sequence 0 publishes a
  second message with an earlier message ID. A new emitter with another
  emitter address loses its peers on the other chains.
- A `peerEpoch` or `factoryEpoch` that starts at 0 lets a withheld
  governance VAA succeed. That VAA was never submitted, so the carried trie
  has no record of it. For example, it can set a replaced peer again.
- A paused manager that moves with `paused` false starts transfers, releases
  and mints again. Its admin did not exercise `SetPaused` to start them.
- A lower `LockedLedger` balance stops releases. A higher balance lets one
  deployment release the reserves of other deployments.
- Another namespace, manager address or transceiver emitter id breaks the
  link to the carried trie and to the peers.

Thus the `wormhole-genesis` version for a breaking core version must carry
these records into the new contracts:

- the consumed digests of each trie;
- the consumed pair digests;
- the owner, the emitter id and the sequence of each emitter, with the same
  emitter address formula and tag in the new core version;
- the identity fields, the peers, the two epochs and `paused` of each
  manager;
- the balance of each `LockedLedger`.

A re-genesis does not carry a `FeeWithdrawalAuthorization`. Do not carry it
into the new contracts. The fee withdrawal steps below archive each active
authorization before the unvet.

A re-genesis already carries the governance replay records. It takes them as
parameters and checks them against the earlier roots.
The body of `AcceptGenesis` has the authority of its controllers `gg`,
`feeCustody` and `nttCustody`. It also has the authority of the operator, the
signatory of the `WormholeGenesisOffer`.
Each `NttManager` also has its `admin` as a signatory. Thus the move of a
manager whose admin is not `gg` also needs the signature of that admin. Other
integrators move their own tries and emitters. Until that `wormhole-genesis`
version exists, do not vet a breaking core version on a `gg` host. Do not
sign a re-genesis to it.

For a breaking core version, do the fee withdrawal steps in this order:

1. Commit the re-genesis. It archives the old `CoreState`. Then no
   `TransferFees` VAA can create a `FeeWithdrawalAuthorization` of the old
   core version.
2. For each active `FeeWithdrawalAuthorization` of the old core version, do
   the [fee withdrawal procedure](https://github.com/wormholelabs-xyz/wormhole/blob/integration/canton/docs/canton_guardian_operations.md#76-fee-withdrawal).
   As a minimum, archive each one. The guardian steps are in
   [section 6.8](https://github.com/wormholelabs-xyz/wormhole/blob/integration/canton/docs/canton_guardian_operations.md#68-breaking-core-version-fee-withdrawals-before-the-unvet)
   of the guardian operations doc.
3. Before the unvet, read the active contracts of `gg` on each `gg` host.
   Make sure that no `FeeWithdrawalAuthorization` of the old core version is
   active. If one is active, do step 2 again.
4. Unvet the old core version.

Archive each `FeeWithdrawalAuthorization` of the old core version before the unvet.
An authorization that stays active at the unvet can give a second transfer for one decision.
`gg` can archive it later if each `gg` host vets the old core version again, or if the new core version can use it.
If the guardians also made a new `TransferFees` VAA for that decision, one decision gives two transfers.

The transfer from `feeCustody` does not use the core package. Thus, if the
archive committed, the transfer can complete after the unvet. The guardians
make a new `TransferFees` VAA only if they stop the transfer. The carried
governance replay record rejects the earlier VAA.

Vet no package whose code runs with `gg` authority and can create a
`ReplayNode` or exercise `ClaimReplayRoot` with `consumer = gg`. The
claimed-roots tree binds only `ClaimReplayRoot`. Such code can create a
`ReplayNode` with `consumer = gg` in the namespace of a live deployment. That
opens a second trie, so one inbound VAA can mint or release two times. Such
code can also claim the pair of a future deployment first. Then that
registration fails. NTT reserves the replay namespace prefix `ntt:`.

### Governance VAA replay

`RegisterManagerByVaa` records the digest and the timestamp of each
registration VAA in the `NttGovernance` replay record. The record uses the
core rules (`recordGovernanceDigest`, `checkConsumedGovernanceIsBounded`):

- The record holds 32 digests or fewer.
- On a full record, `recordGovernanceDigest` removes each digest at the
  lowest timestamp. The floor (`evictedGovernanceMaxTimestampSeconds`)
  increases to that timestamp.
- The ledger rejects a registration VAA with a timestamp at or below the
  floor.

After 33 registrations with timestamps at or above its own timestamp, a
pending registration VAA is at or below the floor. Then the ledger rejects
it, and the guardians sign the registration again with a newer timestamp.

A registration has no other replay protection. A second registration with
one VAA makes a second manager for the same instrument. Thus the floor
applies to each registration.

`AcceptAdminTransferByVaa`, `SetPeerByVaa` and `SetFactoryByVaa` do not use
this record. Each of them consumes its VAA in the `gg` replay trie of the
deployment (`VerifyAndConsumeVAA`). The trie keeps each digest, and the
`factoryEpoch` or `peerEpoch` in the VAA must equal the current value.

### Genesis

The operator does the deployment work. The guardians sign one transaction.

1. Upload the `wormhole-core`, `token-cip0056`, `ntt` and `wormhole-genesis`
   DARs to the participant of the operator.
2. On each participant that hosts `gg`, `feeCustody` or `nttCustody`, vet
   the same DARs. The vetting of `wormhole-genesis` on these hosts is the
   security decision: its code runs once with the authority of the three
   parties.
3. Write the `GenesisParameters`. Set `genesisKind` to `FirstGenesis`. Each
   guardian keeps an agreed copy.
4. As the operator, create one `WormholeGenesisOffer` with the parameters.
5. As `gg`, `feeCustody` and `nttCustody`, exercise `AcceptGenesis` on the
   offer. Give the agreed copy as `agreedParameters`.

`AcceptGenesis` rejects parameters that differ from the offer. It also
rejects these parameters:

- `feeCustody` or `nttCustody` is `gg`, or `feeCustody` is `nttCustody`.
- A genesis guardian whose public key does not derive its address. Each
  guardian of `genesisGuardianSet` has an address and a public key.
- The governance chain is not 1 to 65535, or it is 75 (Canton).
- A governance contract that is not a well-formed `EmitterAddress`.
- A replay record with more than 32 digests, or with a digest at or below
  its floor.
- A `FirstGenesis` with an id floor above 0, a consumed digest, or an
  eviction floor.

The ledger accepts each other governance emitter. Before a guardian accepts
the offer, it must check that the governance emitter is the standard Wormhole
governance emitter: chain 1, address
`0000000000000000000000000000000000000000000000000000000000000004`.

It then creates these contracts:

| Contract             | Signatories        |
| -------------------- | ------------------ |
| `CoreState`          | `gg`, `feeCustody` |
| `EmitterRegistry`    | `gg`               |
| `ReplayRootRegistry` | `gg`               |
| `NttGovernance`      | `gg`, `nttCustody` |
| `CoinFactory`        | `gg`               |
| `ReplayNode`         | `gg`               |

A first genesis creates the `CoinFactory` and the `ReplayNode` root of the
claimed-roots tree (`mkInitialClaimedRootsNode gg`). Its namespace is
`wormhole-core:replay-roots`. `GenesisRoots.claimedRootsNodeCid` holds its
contract id. A re-genesis keeps the earlier `CoinFactory` and the
claimed-roots tree. It creates no node of the tree and archives no node.
Thus each pair that a registration claimed before the re-genesis stays
claimed. `GenesisRoots.claimedRootsNodeCid` is `None` after a re-genesis.

The same transaction archives the offer. No created contract has the operator
as a stakeholder. After genesis, each host of `gg`, `feeCustody` and
`nttCustody` can unvet `wormhole-genesis`.

For a re-genesis of a `gg`, set `genesisKind` to `ReGenesisReplacing` with the
contract ids of the earlier `CoreState`, `EmitterRegistry`,
`ReplayRootRegistry`, `NttGovernance` and `CoinFactory`. Set these parameters
from the same contracts:

- `coreGovernanceReplayRecord`: `tryGovernanceReplayRecordForRegenesis` of
  the earlier `CoreState`.
- `nttGovernanceReplayRecord`: `tryGovernanceReplayRecordForRegenesisFromRecords`
  of the record of the earlier `NttGovernance` (`nttGovernanceReplayRecordOf`).
- `genesisGuardianSet`: the index and the current set of the earlier
  `CoreState`, with the public key of each guardian.
- `firstEmitterId`: the `nextId` of the earlier `EmitterRegistry`.
- `firstNttManagerId`: the `nextId` of the earlier `NttGovernance`.

A replay record is the consumed digests (digest to VAA timestamp) and the
eviction floor. For one earlier root, it equals the record of that root.

`AcceptGenesis` fetches the earlier roots. It rejects the re-genesis if a
root has another `gg` or custody party. It also rejects the re-genesis if one
of these values differs from what the earlier roots give: the two first ids
(`firstEmitterId`, `firstNttManagerId`), the two replay records, the guardian
set index and the genesis guardians. It also rejects an earlier `CoinFactory`
with an admin other than `gg`. The ledger compares only these values. Each guardian must
compare each other parameter with the earlier roots: the governance chain,
the governance contract, the fee instrument, each fee, `onboardingFeeAdmin`.
`AcceptGenesis` then archives the four earlier roots and creates the
new roots in the same transaction. The new `CoreState` gets the guardian set
index and the stored sets of the earlier `CoreState`
(`tryGuardianSetsForRegenesis`). The new `NttGovernance` gets the earlier
`rootIdentity`. Thus the new roots reject each earlier governance VAA: a
consumed digest stays in the record or at or below the floor, and an earlier
GuardianSetUpgrade does not match the current index. Each new emitter id is
above the earlier ones. Each new manager id is also above the earlier ones.

The earlier `CoinFactory` stays active and is the factory of the new roots.
Burn/mint managers hold its contract id, and the disclosure service expects
one `CoinFactory` for each `gg`.

A re-genesis cannot change `feeCustody` or `nttCustody`. The core key
precondition applies only to a `CoreState` with no governance history, so
`AcceptGenesis` checks the keys for a re-genesis too. If a guardian of the
earlier current set has no public key, the accept fails. First run
`AttachGuardianPublicKeys` on the earlier `CoreState`. Each guardian must
check that the guardian set index is the current global index.

A governance action or a registration on an earlier root changes its
contract id or its values. Then the accept fails, and the operator makes a
new offer with the current values.

Each replay record holds 32 digests or fewer. The seeded digests are the core
VAA digest (`keccak256(keccak256(body))`). An earlier root of a version with
another digest definition needs a review before a re-genesis.

The ledger cannot see whether a `gg` had roots before a `FirstGenesis`. Each
guardian must check that no `CoreState`, no `EmitterRegistry`, no
`NttGovernance` and no claimed-roots `ReplayNode` of the `gg` is active before
it accepts a `FirstGenesis`.

### Version 0.4.x is not upgrade-compatible

Version 0.4.0 added the `nttCustody` signatory to `NttGovernance` and
`NttManager`. It also added the `reserveCustodian` field to `NttManager`.
An upgrade cannot add a signatory to an existing contract.
Thus `ntt/daml.yaml` has no `upgrades:` field, and `dars/` has no earlier
`ntt` DAR.

An `Optional` field keeps the package upgrade-compatible.
It gives no safe path for an existing deployment:

- An existing root has no `nttCustody` signature. No manager of that root can
  give its reserve to `nttCustody`.
- The reserve of an existing gg-adminned deployment is a gg holding. To move
  it, the registry runs one more transfer with gg as the sender.
- With the value `None`, old contracts keep the state "gg owns the reserve".

No production deployment uses an earlier version. The LocalNet rehearsals
used 0.2.0 and 0.3.0. The earlier versions also include 0.3.1 (core 0.7.0),
0.3.2 (core 0.8.0, package id `f77fe8a8...`) and 0.3.3 (core 0.8.1,
package id `6c5c1733...`). Version 0.4.0 built against core 0.8.0. Version
0.4.1 built against core 0.8.1 (`5b256d13...`) and has the package id
`88e55a615a516145c7b4d8f0f754f0e95adc8a87bf8c4fa41cdf919cdbb4b7b1`. Version
0.3.4 builds against core 0.8.3 (package id `1c555ab2...`). Version 0.4.2
builds against core 0.8.3 (`9d546b11...`) and has the package id
`1a3e3b0e78f9f08adac111bc44110469dc271f61a9c0e5ed2f7b0b20a569feb8`. Canton
checks upgrade compatibility by
package name when it receives a DAR. Thus a participant that has an earlier
`ntt` package rejects the 0.4.x DAR. Use participants that have no earlier
`ntt` package.

To move a deployment from an earlier version, do these steps:

1. Hold a new root ceremony. In one transaction, archive the old
   `NttGovernance` and create the new one. The signatories of both roots
   act. For a 0.4.x root, the operator also acts.
2. In the same transaction, set `nextId` of the new root to the `nextId` of
   the archived root. Thus no manager address and no replay namespace occurs
   two times.
3. In the same transaction, set the `rootIdentity` of the new root as a first
   genesis does. A root before 0.6.0 has no `rootIdentity`, and its digests
   have no timestamps. Set `consumedGovernance` to empty. Set
   `evictedGovernanceMaxTimestampSeconds` to the latest body timestamp of
   the registration VAAs that the guardians signed for the archived root. Thus no earlier registration VAA
   applies again.
4. Register each deployment again on the new root.
5. On each peer chain, register the new manager and transceiver addresses.
6. Move each reserve from its old custodian to the new `reserveCustodian`.

## Parties and the trust model

Three parties sign every `NttManager`, and each signature has one job:

- `admin` is the operational role: it maintains the peer table and rotates
  the committed factory. It is transferable (`TransferAdmin`, usually via the
  propose-accept `AdminTransferProposal`); handing it to `gg` is the guardian
  quorum's custody opt-in.
- `guardianGovernance` (`gg`) is the guardians' k-of-n threshold party, the
  same party that anchors the core and receives message fees. It administers
  burn/mint instruments, owns the deployment's transceiver `Emitter`, is the
  replay-trie consumer for every deployment. When `gg` holds the admin role of
  a deployment, its admin operations are governance actions.
- `nttCustody` owns the lock/unlock reserve of each deployment whose admin
  role is with `gg`. It is a second quorum party (see "Custody party" below).

The manager field `reserveCustodian` names the owner of the lock/unlock
reserve. It is equal to `admin` while `admin` is not `gg`. It is equal to
`nttCustody` while `admin` is `gg`. The template precondition of the manager
rejects `gg` as `reserveCustodian`. It also rejects `nttCustody` as `admin`.
Registration and `TransferAdmin` assert the same two rules.

The transceiver `Emitter` of each deployment has the owner `gg`, also for an
admin-only lock/unlock registration. This owner does not show that the
guardian quorum approved the deployment. Peer chains decide which Canton
deployment to trust.

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

`gg` and `nttCustody` act exactly once: a guardian quorum ceremony (the same
external-signing flow that creates the genesis `CoreState`) creates the
`NttGovernance` root.
Everything afterwards runs on inherited authority. `RegisterManager` lends the
root's signatures to create managers — and, atomically, to mint the
deployment's `gg`-owned transceiver `Emitter` and claim its `gg`-scoped replay
root — and the manager's choice bodies lend the manager's signatures to move
tokens. Value can only move inside fixed template code, and every inbound
movement first checks and consumes a VAA, so no single signatory can move
bridged value, and nobody has to co-sign at transfer time. That is what keeps
both deployment and relaying permissionless.

Before the root ceremony, compare the namespace of `nttCustody` with the
namespace of `gg`. The namespace is the part of the party id after `::`. The
two namespaces must be equal. If they are not equal, stop the ceremony. The
contracts cannot check this rule. IDE ledger party ids have no namespace.
Thus a template precondition with this rule rejects each test party.
`partiesShareNamespace` in `Wormhole.Ntt.Manager` gives the rule.
`testPartyNamespaceFromText` tests it.

A rogue movement of an `nttCustody`-owned reserve takes a native spend by
the guardian quorum, a new `nttCustody`-signed contract (also a quorum act),
or code of a vetted registry (see "Custody party" below).
The model rests on the key namespace of `gg` and `nttCustody`. The guardian
quorum governs this namespace at the topology layer, so nobody can quietly
lower its threshold. The contracts cannot check this deployment condition.

### Custody party

`gg` never owns a lock/unlock reserve. The reason is the CIP-0056 transfer
interface:

- `TransferFactory_Transfer` has the controller `transfer.sender`.
- The reserve owner is the sender of each custody transfer: `Release`, the
  lock merge in `Transfer`, `ConsolidateCustody`, and `TransferAdmin`.
- Thus the registry code of a lock/unlock deployment runs with the authority
  of the reserve owner.

When `gg` owned the reserve, registry code had the authority of `gg`
(`REDTEAM.md`, F1). A registry used it to mint each `gg`-administered
instrument, to publish a payload from each NTT transceiver `Emitter`, and
to change each manager whose admin is `gg`.

`nttCustody` is a second party in the same guardian decentralized namespace
as `gg`. The guardians control both parties with the same threshold. A
rotation of the namespace owner keys applies to both parties. Each party has
its own signing-key update in the same rotation script. A LocalNet rehearsal
on three participants confirmed this procedure (wormhole PR #76). A rotation
is three topology changes. Until all apply, a removed key can still sign for a
party that is not yet updated. Rotate `nttCustody` first, then finish the
rotation at once.

`nttCustody` signs every manager, also burn/mint managers and managers that
an ordinary admin holds. The reasons are:

- The hand-off to `gg` runs with no live signature
  (`AcceptAdminTransferByVaa`). The new manager has `nttCustody` as a
  signatory. Only a signature on the old manager can supply this authority.
- A signature on only the quorum-held managers needs a second source at
  hand-off time, for example a choice on the root that `gg` controls and that
  creates `nttCustody`-signed managers. Each hand-off then needs the root, and
  each hand-off contends with registrations.
- The extra signature gives registry code no authority. Registry code gets
  the authority of the transfer sender only. The sender is `nttCustody` only
  on lock/unlock deployments that the quorum holds.

A malicious or defective registry that the quorum vetted can do these
things with the authority of `nttCustody`:

- Move each holding of `nttCustody`. This includes the reserves of other
  quorum-held deployments, also in other instruments. The `LockedLedger` cap
  does not stop it.
- Make holdings of its own instrument with `nttCustody` as owner.

Thus the quorum-held lock/unlock reserves are one risk pool. The owner
accepts this risk (`REDTEAM.md`, F1, "Residual risk"). The guardian quorum obeys this
rule for each transfer registry:

1. Examine the registry before you sign an `AcceptAdminToGovernance` VAA.
2. Examine each new version of the registry before a guardian participant
   vets it.
3. Examine the registry again before you sign a `SetFactory` VAA for it.
4. Do not vet a registry version that you did not examine.

The same registry cannot do these things:

- Publish a message from an NTT transceiver `Emitter`. The `Emitter` owner is
  `gg`.
- Mint a `gg`-administered instrument through the canonical `CoinFactory`.
- Change a manager: `SetPeer`, `SetFactory`, `SetPaused`, and `TransferAdmin`
  need the admin, which is `gg`.
- Change a `LockedLedger` outside a manager choice. Its choices need `gg`,
  and the `token-cip0056` package does not reference `LockedLedger`.
- Make `nttCustody` the admin of a deployment.

`TestRedTeam:testRedTeamQuorumCustodyRegistryCannotUseGovernanceAuthority`
and `TestRedTeam:testRedTeamQuorumCustodyRegistryCannotMintBridgedInstrument`
show the rejections.

The rule above is for transfer registries only. The fee registry runs with
the authority of `feeCustody`, not `gg`. Core 0.8.3 (`wormholelabs-xyz/wormhole`
PR #81) moved fee custody from `gg` to `feeCustody`. The fee instrument
admin is not `gg` and not `feeCustody` (since core 0.9.0).

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
`gg`'s and `nttCustody`'s, and `admin` co-controls (pays) both nested core
onboarding fees so `gg` is never made to fund a deployment's registration. For
lock/unlock it also creates the deployment's `LockedLedger` at balance zero,
with the reserve starting admin-owned (see "Custody follows the admin"
below). For burn/mint it checks the mint capability: `gg` must administer the
instrument and co-sign the registration, the factory must be the canonical
`CoinFactory` (see "gg is the only minter" below), and the instrument id must
be bound to the registering admin (see "Instrument binding" below).

The deployer also gives `claimedRootsNodeCid` and its disclosure. To find the
node, do these steps:

1. Get the `register` flow from the disclosure service with the `admin`,
   `instrument-admin` and `instrument-id` parameters.
2. Give the `covering claimed-roots ReplayNode` as `claimedRootsNodeCid`.

The service reads the `nextId` of the `NttGovernance` root. It calculates
`nttNamespaceFor instrumentId admin nextId` and gives it as `claimNamespace`,
with `nextId` as `managerId`.

The node covers `claimedRootDigest gg namespace` in the `gg` claimed-roots
tree. `ClaimReplayRoot` records this digest. A second registration of one
pair fails with "digest already consumed". A wrong node fails with the core
text. If a registration changes `nextId` first, get the node again.

Two key-derived identities, both bound to the REGISTERING admin — present as a
controller at registration, and whose signature co-signs the created manager —
so they survive later admin handoffs (see the core README's message
publishing section):

- transceiver address =
  `keccak256("wormhole:emitter:v1" ‖ gg ‖ gg ‖ emitterId)` (registrar `gg`,
  owner `gg`). This is the
  VAA emitter other chains register as the peer. It is derived by the
  watcher, not stored, and must be a single shared `Emitter` per deployment,
  since its address is the peer identity. (The preimage's owner ordinal is
  `gg`, not the registering admin — `gg` co-signs every `Emitter` it mints.)
- manager address =
  `keccak256("wormhole:ntt-manager:v1" ‖ gg ‖ registeringAdmin ‖ managerId)`,
  computed once at registration and stored in `managerAddress`. The distinct
  domain tag keeps an emitter and a manager with the same ordinals from
  colliding. The registering admin's signature co-signs the manager at
  creation, so a party without that signature cannot forge an existing
  manager's address to consume that deployment's inbound VAAs; because the address is
  computed once and stored, not recomputed, it is unaffected by any later
  `TransferAdmin`.

Because registration is permissionless, anyone can create junk managers
co-signed by `gg` and `nttCustody`. They are inert (their choices cannot move
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
deployment's own guardian trust anchor; the VAA verified and its
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
   `guardianGovernance` (via `GetGuardianGovernance`). A forged `CoreState`
   under a throwaway gg party lacks the real `gg`'s signature and fails the
   pin (`TestNtt:testReceivePinsGuardianGovernance`).
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
rejects any amount above the balance, and transfers custody holdings from the
deployment's `reserveCustodian` to the recipient. `Mint` (burn/mint) mints through the recipient's standing
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

**gg is the only minter.** A burn/mint deployment mints only instruments that
`gg` administers, through the canonical `Token.CIP0056.CoinFactory` (vendored
in `dars/`). Every path that commits a burn/mint factory (`RegisterManager`,
`RegisterManagerByVaa`, `SetFactory`, `SetFactoryByVaa`) pins it to that
template with admin `gg` (`requireCanonicalFactory`), and `gg` co-signs each
live-signed path. External minters are not supported. The manager publishes a
value-bearing VAA after each burn, so it must be able to verify the burn. A
factory from another minter is code that NTT cannot verify: it can report a
burn that did not occur. To bridge a token that `gg` does not administer, use
lock/unlock.

The canonical-factory pin uses the package name, not the package ID. Canton
resolves a template by package name across upgrades, so
`requireCanonicalFactory` accepts a `CoinFactory` of each vetted
`token-cip0056` version. An interface choice runs the code of the vetted
version that the submitter prefers. Thus, on each participant that hosts
`gg`, the vetting of a new `token-cip0056` version is a decision about the
`gg` mint authority. Vet a new version only after a review of its
`CoinFactory` and `Coin` code.

**Custody follows the admin.** A deployment registers with its reserve owned
by its own `admin`: fully operational and explicitly custodial — the admin
owns the reserve outright, so users trust it the way they would a custodial
bridge, and wallets must show who the admin is. `TransferAdmin` hands over the
role, normally as propose-accept: the current admin leaves a
standing `AdminTransferProposal`, and the new admin accepts at its own pace,
naming the committed factory it vetted. The ledger balance moves to the new
`reserveCustodian`. The rest of the pot stays with the old custodian, because
the pot can hold the reserves of other deployments of the same custodian
(`ConsolidateCustody`). The registry must take exactly the balance from the
old custodian and deliver exactly the balance to the new custodian. Handing
the role to `gg` is the guardian quorum's custody opt-in: the reserve moves to `nttCustody` — the old admin can no longer touch
it, every inbound movement is VAA-gated, and all admin operations (peers,
factory rotation, a further handoff) become governance actions. A demotion
from `gg` moves the ledger balance from `nttCustody` to the new admin.
The handoff changes nothing on the wire:
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
`CoinFactory` (a VAA cannot name a contract id, so a ByVaa path can only
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
custodians cannot mix, no matter what any factory reports: a deployment that
commits a dishonest factory can inflate only its own ledger and drain only its
own admin-owned reserve. Only deployments whose admin role the quorum
accepted, and whose factories it vetted, share the `nttCustody` pool.
Ownership is also
the one partition a registry cannot churn away: registries reorganize holdings
out from under the bridge (Canton Coin's rounds merge, expire, and recreate
contracts), which erases any partition built out of specific contract ids — a
pinned pot cid, an escrow — but never changes who owns the value. Finally,
because every choice body carries all four signatures, the manager checks
that the deployment's own `reserveCustodian` owns each custody holding that
it spends. Thus the ambient `nttCustody` authority in the choices of another
deployment can never reach the `nttCustody` pool.

**The locked ledger.** Wormhole attests only that an emitter emitted bytes.
It does not attest that value backs a transfer. Thus a deployment that trusts
a hostile peer must not release more than it locked:

- The manager credits the `LockedLedger` of the deployment on each lock.
- The manager debits the `LockedLedger` on each release.
- `Debit` rejects amounts above the balance.

In the `nttCustody` pool, reserves of one instrument are fungible across
gg-adminned deployments. There, this cap keeps a hostile-peer deployment away
from the collateral of the other deployments. Each credit is atomic with its
deposit. Each debit is atomic with its release. Thus the sum of the balances
never exceeds the physical holdings of `nttCustody`. The cap of each
deployment stays satisfiable for each choice of holdings that a release
spends. Message fees go to `feeCustody`, not to `gg` and not to
`nttCustody`. Thus a release cannot spend fee holdings.

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
as a hint). The manager also validates each holding that the registry reports
back. Each receiver holding, release input, and release change holding must
be the expected party's own, of this instrument, unlocked, and signed by the
instrument admin. The first lock deposit, which becomes the pot, gets the
custody validation too.

**Zero sender fees (lock/unlock).** A lock/unlock deployment
needs a registry that charges the sender zero fees on a transfer.
`Release` asserts that the custodian gives up exactly the release amount
(`SenderOutflowEqualsAmount`). A lock admits a sender fee, because its check
reads only the custodian's new holdings. Thus, on a registry that charges
the sender a fee, each lock succeeds and each `Release` aborts. The locked
value then stays in custody with no release path. Canton Coin charges zero
sender fees in Splice 0.10.2. Release 0.4.17 removed holding fees from
transfers (CIP-78). Release 0.5.16 set all `AmuletConfig` fees to zero
(CIP-107). `REDTEAM.md` (N4) records the source references and a LocalNet
release from an aged pot.

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
gg-adminned deployment needs it arranged for `nttCustody`, which is part of
what the quorum takes on by accepting the role.

## Contention and contract churn

The manager itself is consumed by its config choices (`SetPeer`, `SetFactory`,
and the rare `TransferAdmin`), so a config edit still leaves the manager cid
stable across ordinary `Transfer` traffic (which is nonconsuming). What
serializes is:

- Publishing, on the transceiver `Emitter` (consumed by every
  `PublishMessage`, which returns the successor cid to its caller alongside
  the published message). This is no longer sends-only: `SetPeer` bundles its
  registration broadcast through the same emitter (see "Accountant
  broadcasts"), so a peer edit now contends with `Transfer` traffic on the
  identical contract, not a separate one. `RegisterManager`'s one-shot init
  broadcast contends the same way, but only once, at registration, and
  because it forwards the broadcast's returned cid, `RegisterManager` itself
  hands back a live `Emitter` — no re-resolve needed right after registration.
  A later `SetPeer` (or a racing `Transfer`) still churns the same contract,
  so anyone holding an older cid re-resolves before its next publish.
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
- Foreign-admin lock/unlock against a real registry: the `Test.TestNtt` mock
  factory models receiver pre-approval only as a co-signature
  (`preapprovedReceivers`). `Test.TestRedTeam` runs lock, release,
  consolidation, and the custody hand-off against an issuer `CoinFactory`
  with `TransferPreapproval`. A LocalNet run of the `nttCustody` hand-off on
  Canton Coin is future work.
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
- The permanent recipient-address binding (see "Recipient binding").
