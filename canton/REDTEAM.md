# NTT on Canton: red-team results

This document records the attacks in `test/daml/Test/TestRedTeam.daml`.
Rows O1 and O2 are in `test/daml/Test/TestNttOperatorIndependence.daml`.
Each test runs one or more attacks and asserts that each attack fails.
A failing test is a finding.

## Harness

The harness signs VAAs in the script with a generated secp256k1 guardian key.
Thus an inbound transfer can name a recipient party that the script allocates.
Lock/unlock deployments bridge a `Coin` of a foreign issuer.
The issuer's `CoinFactory` is the registry, so the registry checks each input holding.
Burn/mint deployments use the canonical gg `CoinFactory`.
The `NttGovernance` root has a separate custody party, `nttCustody`.

## Results

| ID  | Attack                                                                                                                                                          | Test                                                                 | Result               |
| --- | --------------------------------------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------- | -------------------- |
| A3  | Replay a consumed VAA on `Release` and `Mint`: same manager, other executor, other manager, other replay trie, other mode                                       | `testRedTeamReplayRejected`                                          | Held                 |
| A4  | Commit a non-canonical factory through `SetFactoryByVaa` and `RegisterManagerByVaa`                                                                             | `testRedTeamGovernanceForgeriesRejected`                             | Held                 |
| A5  | Lock or burn a forged holding, a holding of another owner, a holding of another instrument, one coin two times                                                  | `testRedTeamLyingHoldingsRejected`                                   | Held                 |
| A10 | Release more than the ledger balance, from a sibling pot, after `ConsolidateCustody`, after a drain                                                             | `testRedTeamLedgerOverdrawRejected`                                  | Held                 |
| A11 | Wrong target chain, zero amount, dust, decimals overflow, maximum amount, wrong emitter, wrong source manager, wrong guardian set, foreign key, wrong recipient | `testRedTeamInboundEdgeCasesRejected`                                | Held                 |
| A12 | Governance VAAs with a wrong emitter chain, emitter address, guardian set, epoch, manager, chain, or action; replay; stale epoch after a rotation               | `testRedTeamGovernanceForgeriesRejected`                             | Held                 |
| A13 | A relayer or a stranger moves the admin role, a peer, the ledger, or custody; gg alone sets a peer                                                              | `testRedTeamUnauthorizedMovesRejected`                               | Held                 |
| N1  | Registry code mints a gg-administered instrument after the quorum accepts custody                                                                               | `testRedTeamQuorumCustodyRegistryCannotMintBridgedInstrument`        | Fixed (F1)           |
| N1a | Under quorum custody, registry code forges a transfer from another transceiver or sets a peer on another gg-adminned manager                                    | `testRedTeamQuorumCustodyRegistryCannotUseGovernanceAuthority`       | Fixed (F1)           |
| N1b | A root, manager, registration, or hand-off makes gg a custodian, `nttCustody` an admin, or `gg` the `nttCustody` of a manager                                   | `testRedTeamReserveCustodianIsNeverGuardianGovernance`               | Held                 |
| N1c | A hand-off moves a pooled sibling reserve; a registry takes or delivers an incorrect hand-off amount                                                            | `testRedTeamAdminTransferMovesOnlyTheLedgerBalance`                  | Fixed (F1 follow-up) |
| N2  | A third-party relayer delivers a release on a production registry                                                                                               | `testRedTeamThirdPartyRelayerReleasesOnCoinRegistry`                 | Fixed (F2)           |
| N2a | Registry takes an incorrect custodian outflow, misroutes change, or answers Pending; duplicate pot                                                              | `testRedTeamThirdPartyReleaseRejectsInexactCustodianOutflow`         | Held                 |
| O1  | A former operator creates or archives an NTT root, a manager, or a ledger that names the real `gg`                                                              | `TestNttOperatorIndependence:testFormerOperatorHasNoNttPower`        | Held                 |
| O2  | Redeem one inbound VAA two times: same deployment, a foreign trie, a sibling deployment, an admin-claimed root                                                  | `TestNttOperatorIndependence:testInboundVaaRedeemsOncePerDeployment` | Held                 |
| N2b | Registry reports a bad deposit or change: wrong owner or instrument, locked, unsigned, duplicate, or an input                                                   | `testRedTeamRegistryMisreportedHoldingsRejected`                     | Held                 |
| N3  | Outbound wire fields one step outside the range, 63-character or 65-character addresses, and boundary values                                                    | `testRedTeamTransferWireFieldBounds`                                 | Fixed (F3)           |
| N4  | A third-party relayer releases Canton Coin from an aged custody pot                                                                                             | LocalNet, `NTT_RELEASE_AFTER_ROUNDS=5 ntt_lock_unlock.sh`            | Held                 |
| N5  | The DSO expires an idle custody pot of Canton Coin as dust                                                                                                      | Splice 0.10.2 source                                                 | Open (F4)            |
| N6  | A lock/unlock admin alone publishes on its gg-owned `Emitter`, sets a burn/mint factory, hands the admin role to gg, or registers a burn/mint deployment        | `testRedTeamLockUnlockAdminCannotUseGgAuthority`                     | Held                 |

A4 note: the harness cannot make a `CoinFactory` from a different package version.
The test package can depend on one `token-cip0056` DAR only.

A4 note: the canonical-factory pin uses the package name, not the package ID.
`requireCanonicalFactory` accepts a `CoinFactory` of each vetted `token-cip0056` version.
An interface choice runs the code of the vetted version that the submitter prefers.
Thus each new `token-cip0056` version that a gg host vets is a decision about the gg mint authority.

## F1: quorum custody gives gg authority to registry code

Severity: Critical.
Status: fixed in PR #75.
The attack needs the quorum to accept custody of a lock/unlock deployment.
Before the fix, the registry can mint each burn/mint instrument with no limit.
It can also publish each payload from each NTT transceiver.
A forged `PublishMessage` on a gg-owned transceiver gives a VAA with no lock or burn behind it.
The peer chain then mints or releases value that no deposit backs.
It can also change each manager whose admin is gg.

### Cause

- `TransferFactory_Transfer` has the controller `transfer.sender`.
- Before the fix, the custodian after `AcceptAdminTransferByVaa` is gg.
- `ConsolidateCustody`, `Release`, `TransferAdmin`, and the lock merge in `Transfer` send with `sender = admin`, which is gg.
- Thus the body of the registry choice has the authority of gg.
- The canonical `CoinFactory` has gg as its only signatory. `BurnMintFactory_BurnMint` needs only gg and the extra actors.
- `Emitter.PublishMessage` needs the owner and the payer. The owner of each NTT transceiver is gg. The registry admin is the payer.
- `SetPeer` needs the admin and the payer. The admin of each quorum-held manager is gg.
- `TransferPreapproval_Receive` in `token-cip0056` needs only the instrument admin, which is gg.
- `validateSettledHolding` accepts a holding that the instrument admin signs alone (`Cip56.daml`). Thus the registry admin can make holdings with `owner = gg` without gg.
- `ConsolidateCustody` is permissionless. Thus the registry admin can start the merge at any time.
- The registry reads the contract ids of its targets from `extraArgs`, which the caller of `ConsolidateCustody` supplies.

### Reproduction (before the fix)

1. Register a burn/mint deployment C with the canonical gg `CoinFactory`.
2. As registry admin R, create `SenderAuthorityRegistryFactory`. This registry mints C's instrument with the authority of the sender.
3. As R, register a lock/unlock deployment with this factory.
4. As R, create an `AdminTransferProposal` to gg.
5. As a relayer, submit `AcceptAdminTransferByVaa` with a valid `AcceptAdminToGovernance` VAA.
6. As R, create two `RegistryHolding` contracts with `owner = gg`.
7. As R, submit `ConsolidateCustody` with these two holdings.
8. Result: R owns 1000000.0 of C's bridged instrument.

`testRedTeamQuorumCustodyRegistryCannotUseGovernanceAuthority` uses the same steps with `SenderAuthorityGovernanceRegistryFactory`.
Before the fix, both of its attacks succeed (PR #75, first commit):

- The registry publishes a forged `NativeTokenTransfer` of 10000000.0 tokens from the transceiver of a different gg-adminned deployment.
- The registry sets a peer on that deployment's manager.

### Fix

gg never owns a lock/unlock reserve.

1. The `NttGovernance` root has a second quorum party, `nttCustody`, as a signatory.
2. Each `NttManager` inherits the `nttCustody` signature from the root.
3. The new manager field `reserveCustodian` names the reserve owner. It is `admin` while `admin` is not gg. It is `nttCustody` while `admin` is gg.
4. Each custody transfer sends with `sender = reserveCustodian`.
5. `TransferAdmin` to gg moves the ledger balance to `nttCustody`. A demotion moves the ledger balance from `nttCustody` to the new admin.
6. The template precondition of `NttManager` rejects gg as `reserveCustodian` and rejects `nttCustody` as `admin`. Registration and `TransferAdmin` assert the same rules.

After the fix, the registry body has the authority of the registry admin and `nttCustody`.
Each attack above fails with an authorization error that names gg.

### Follow-up: a hand-off moves only the ledger balance

Severity: High.
Status: fixed in PR #75.

`ConsolidateCustody` is permissionless.
It can merge the reserves of two deployments with the same custodian into the pot of one deployment.
All quorum-held deployments have the same custodian, `nttCustody`.
Before the fix, `TransferAdmin` moved the full pot to the new custodian.
Thus a demotion of one deployment moved the reserves of the other quorum-held deployments to the new admin.

After the fix, `TransferAdmin` moves exactly the ledger balance.
The rest of the pot stays with the old custodian.
The hand-off uses `SenderOutflowAndReceiverInflowEqualAmount`.
The registry must take exactly the balance from the old custodian.
The registry must also deliver exactly the balance to the new custodian.
Thus a registry fee cannot spend the reserves of other deployments.

`testRedTeamAdminTransferMovesOnlyTheLedgerBalance` runs three cases: a quorum demotion, a quorum promotion, and a succession between admins.
In each case, the other deployment releases its full balance after the hand-off.

### Residual risk

Status: accepted.

A registry that the quorum vetted runs with the authority of `nttCustody`.
It can move each holding of `nttCustody`.
This includes the reserves of other quorum-held deployments, also in other instruments.
The `LockedLedger` cap does not stop it.

Rule for the guardian quorum:

1. Examine the transfer registry before you sign an `AcceptAdminToGovernance` VAA.
2. Examine each new version of that registry before a guardian participant vets it.
3. Examine the registry again before you sign a `SetFactory` VAA for it.
4. Do not vet a registry version that you did not examine.

`README.md`, section "Custody party", lists what the registry cannot do.

The fee registry runs with the authority of `feeCustody`, not `gg`.
Core 0.8.3 (`wormholelabs-xyz/wormhole` PR #81) moved fee custody from `gg` to `feeCustody`.
Since core 0.9.0, the fee instrument admin is not `gg` and not `feeCustody`.

## Replay scope of a deployment

Each inbound VAA path consumes the digest in the `gg` replay trie under the
namespace that the manager stores. These paths are `Release`, `Mint`,
`AcceptAdminTransferByVaa`, `SetPeerByVaa` and `SetFactoryByVaa`. The caller
supplies the replay node. The core checks its consumer, its namespace and its
`gg` against the manager. `RegisterManager` and `RegisterManagerByVaa`
compute the namespace from the instrument, the registering admin and the
`nextId` of the root. The caller cannot choose it. `nextId` increases by one
for each registration, so each deployment of one root has its own namespace.

The core has no rule of one root per namespace. A second root in the
namespace of a deployment needs `gg` authority. NTT claims a root only in a
new registration. Thus a second trie is possible only in these cases:

- A second `NttGovernance` root for the same `gg` starts at a used `nextId`.
  The guardians must sign one root for each `gg`. A second root has another
  `rootIdentity`, so it rejects each `RegisterManagerByVaa` VAA of the first
  root. A lock/unlock `RegisterManager` needs no VAA, so this rule stays.
- Code outside `ntt` that runs with `gg` authority claims a root. The
  canonical `CoinFactory` package does not depend on `wormhole-core`. Each
  `gg` host vets each new `token-cip0056` version.

## F2: a third-party relayer cannot deliver a release

Severity: Medium.
Status: fixed in PR #74 (branch commit `17bcd79`).
Before the fix, value stayed safe, but an inbound unlock needed the recipient to submit.

### Cause

- `executeSyncTransfer` fetches each receiver holding to check the delivered amount (`ntt/daml/Wormhole/Ntt/Cip56.daml:586`).
- A fetch needs an authorizer that is a stakeholder of the contract.
- In `Release`, the authorizers are the manager signatories and the executor.
- A production holding has the issuer and the recipient as stakeholders. Neither is an authorizer.
- The test mock signs holdings with gg. Thus the existing tests do not show the failure.

### Reproduction

1. Register a lock/unlock deployment on the issuer `CoinFactory`.
2. Lock 1.0 from user A.
3. Sign a transfer VAA for 0.4 to recipient B.
4. As relayer X, submit `Release` with B's `TransferPreapproval` in `extraArgs`.
5. Result: the ledger rejects the fetch of B's new holding with an authorization failure.
6. The same release succeeds when B is the executor.

### Fix

`executeSyncTransfer` takes a `SettledAmountCheck`.
Each call site selects the side whose holdings the transaction can fetch.

- `Release` uses `SenderOutflowEqualsAmount`.
- The lock deposit and the lock merge in `Transfer` use `ReceiverHoldingsSumToAmount`.
- `ConsolidateCustody` and `TransferAdmin` use `ReceiverHoldingsSumToAmount`.
- In these four transfers, the receiver is the custodian or the new admin. Thus the receiver is an authorizer, and these checks did not change.

`SenderOutflowEqualsAmount` does these steps:

1. It rejects an input list that contains a holding two times.
2. Before the registry call, it fetches each input holding. Each input must have `owner = sender`, the transfer instrument, no lock, and the instrument admin as a signatory.
3. It calls the registry. A `Pending` or `Failed` result aborts the transaction.
4. It rejects a change list that contains a holding two times, or that contains an input holding.
5. It fetches each change holding. Each change holding must pass the same four checks as an input.
6. It asserts that the input total, less the change total, is equal to the release amount.

`ReceiverHoldingsSumToAmount` applies the same four checks to each receiver holding, with `owner = receiver`.
It also rejects a receiver list that contains a holding two times.
`Transfer` also validates the first lock deposit as a custody holding before the ledger records it as the pot.

The custodian is a signatory of the manager. Thus the custodian can fetch the inputs and the change.

### Why the check is sound

- The ledger debit is equal to the release amount.
- The check proves that the custodian gave up exactly the release amount, if each change holding is new.
- Thus the custody pot and the ledger balance decrease by the same value.
- A registry that takes more from the custodian, for example a fee, fails the check. Thus a release cannot erode the reserve below the ledger balance.
- A registry that takes less from the custodian also fails the check.
- A registry that pays the change to a different party fails the ownership check.
- Synchronous settlement does not change. `Pending` and `Failed` results abort, so the bridge cannot debit the ledger when no value moved.
- The check does not prove that the recipient received the amount. The registry owns the instrument and controls its holdings. The old receiver check also trusted holdings that the registry made.

Residual: the check cannot detect change that existed before the transaction.
A holding has no marker of the transaction that created it.
A registry can report an earlier custodian holding as change, for example the pot of a sibling deployment.
Then the check passes, but the custodian gave up more than the release amount, and the ledger hint points to the earlier holding.
Only the registry can do this, because the instrument admin must sign the change.
The registry is the instrument admin, so it can already move each holding of its instrument.

## F3: outbound transfer accepts fields that the guardians cannot deliver

Severity: Low.
Status: fixed in PR #74 (branch commit `0c48f58`).
Before the fix, a wrong client input burned or locked value, and no peer could redeem it.

### Cause

`Transfer` (`Manager.daml:799`) does not check these fields:

- `consistencyLevel` must be a uint8. `SetPeer` and registration check this; `Transfer` does not.
- `nonce` must be a uint32.
- `recipientAddress` must be 32 bytes. `leftPadTo` keeps an address that is longer than 32 bytes (`Manager.daml:901`).
- `recipientAddress` must not be zero. The reference `transfer` rejects a zero recipient.

### Reproduction

Submit `Transfer` with each value below. Each transfer locks the value and publishes a message.

- `consistencyLevel = 256` or `-1`
- `nonce = 4294967296` or `-1`
- `recipientAddress = "01" <> 32 bytes`
- `recipientAddress = 32 zero bytes`

### Fix

`Transfer` asserts these conditions before it moves value:

1. `isUint8 consistencyLevel`
2. `isUint32 nonce`
3. `checkHexValueIsWellFormed recipientAddress`. `recipientAddress` is a `UniversalAddress`: exactly 64 lowercase hex characters.
4. `not (isZeroAddress recipientAddress)`

`SetPeer` and registration use the same helpers.
A failed assertion aborts the transaction. Thus no value moves and the manager publishes no message.

### Follow-up: half bytes

The first fix asserted `byteLength recipientAddress == 32`.
`byteLength` divides the character count by 2 and drops the remainder.
Thus a 65-character address passed, and `leftPadTo 32` kept it.
65 zero characters also passed `isZeroAddress`.
The check now asserts exactly 64 characters.

The same follow-up applies these checks to the payload codec:

- `runGet` rejects a buffer that is not whole bytes of lowercase hex.
- Each byte field has a constrained type. Its parser rejects a value that is not whole bytes of lowercase hex.
- Each 32-byte field has a type of exactly 32 bytes, so the codec writes it without padding.

## N4: a release from an aged Canton Coin pot

Result: held. The F2 check accepts each release from an aged custody pot.

### Question

Canton Coin (Amulet) has a holding fee for each round.
If a transfer charges this fee to old inputs, the custodian gives up more than the release amount.
Then `SenderOutflowEqualsAmount` rejects each release from an aged pot.

### Facts from the Splice 0.10.2 source

The source is `canton-network/splice` at tag `0.10.2`, directory `daml/splice-amulet/daml/Splice`.

- The `HoldingV1` view of an `Amulet` reports `initialAmount` (`Amulet.daml:193`).
- A transfer adds `initialAmount` for each input amulet (`AmuletRules.daml:1415`).
- The holding fee applies only in `Amulet_Expire` (`AmuletRules.daml:1421`).
- `summarizeTransfer` requires that the holding fees are zero (`AmuletRules.daml:1617`).
- Each output fee is zero (`AmuletRules.daml:1609`).
- The change is the input total less the outputs (`AmuletRules.daml:1638`).
- `validTransferConfig` requires a zero create fee, transfer fee, and lock holder fee (`AmuletConfig.daml:192-196`).
- The direct path and the `TransferPreapproval_SendV2` path use the same transfer code (`ExternalPartyAmuletRules.daml:447`, `:487`).
- Release 0.4.17 removed the holding fees from transfers, as CIP-78 specifies.
- Release 0.5.16 stopped all fees in `AmuletConfig`, as CIP-107 specifies.

Thus the input view total less the change is equal to the release amount for each pot age.

### Live result

LocalNet ran Splice 0.10.2 with ntt package `8b9c3ee1`.
The open round was 21.
The rate per round of the pot was 0.00380518 CC.

1. The locker locked 5.0 CC. The new pot was 5.0 CC from round 0.
2. A relayer on `sv` released 2.0 CC through the recipient `TransferPreapproval`.
3. The change was 3.0 CC from round 0. The recipient balance went from 2000.0 to 2002.0.
4. The ledger balance went from 5.0 to 3.0. `Release` succeeded.

The holding fee that the pot accrued by round 21 is 0.0799 CC.
The transfer did not take this fee.

NOTE: Token-standard transfers use the round of `ExternalPartyConfigState`.
The DSO updates this round one time each 24 hours.
On this LocalNet, the round was 0 for each transfer.

## F4: an idle custody pot of Canton Coin can expire as dust

Severity: Low.
Status: open. Operations can prevent it.
The attack needs a deployment with a small pot and no lock or release for a long time.

### Cause

- `Amulet_ExpireV2` has the controller `dso` (`Amulet.daml:173-187`).
- The choice archives an amulet when the accrued holding fee is equal to or more than `initialAmount` (`Expiry.daml:124-149`).
- The pot is an amulet that the custodian owns. Thus the DSO can archive the pot.
- After this, the custody reserve is less than the ledger balance.
- `Release` cannot fetch the pot. The custodian must restore the reserve.

The pot expires at `createdAt + ceil(initialAmount / ratePerRound)`.
On LocalNet, the 3.0 CC pot expires at round 789.

### Mitigation

Each lock merge and each release makes a new pot from the current round.
Thus only an idle pot ages.

1. Monitor the expiry round of each pot.
2. Before that round, run `ConsolidateCustody` with the pot and one more custodian holding.
3. The merge makes a new pot from the current round.

A call with only the pot does not refresh it, because that call does not make a transfer.

## Observations with no finding

- `ConsolidateCustody` is permissionless. A caller can merge the pot of a sibling deployment with the same admin into one pot. The ledger caps still hold. The sibling hint then points to an archived holding, and the sibling must use explicit holdings or a new consolidation.
- An inbound amount that overflows the 64-bit `Int` after rescale makes the VAA fail. Examples are wire decimals 0 with a very large amount, or wire decimals 255. A correct peer does not send these values.
- A fetch of a holding that the caller does not own fails at the ledger before the NTT custodian check. The result is the same rejection.
