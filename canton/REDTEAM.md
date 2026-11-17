# NTT on Canton: red-team results

This document records the attacks in `test/daml/Test/TestRedTeam.daml`.
Each test runs one or more attacks and asserts that each attack fails.
A failing test is a finding.

## Harness

The harness signs VAAs in the script with a generated secp256k1 guardian key.
Thus an inbound transfer can name a recipient party that the script allocates.
Lock/unlock deployments bridge a `Coin` of a foreign issuer.
The issuer's `CoinFactory` is the registry, so the registry checks each input holding.
Burn/mint deployments use the canonical gg `CoinFactory`.

## Results

| ID | Attack | Test | Result |
|----|--------|------|--------|
| A3 | Replay a consumed VAA on `Release` and `Mint`: same manager, other executor, other manager, other replay trie, other mode | `testRedTeamReplayRejected` | Held |
| A4 | Commit a non-canonical factory through `SetFactoryByVaa` and `RegisterManagerByVaa` | `testRedTeamGovernanceForgeriesRejected` | Held |
| A5 | Lock or burn a forged holding, a holding of another owner, a holding of another instrument, one coin two times | `testRedTeamLyingHoldingsRejected` | Held |
| A10 | Release more than the ledger balance, from a sibling pot, after `ConsolidateCustody`, after a drain | `testRedTeamLedgerOverdrawRejected` | Held |
| A11 | Wrong target chain, zero amount, dust, decimals overflow, maximum amount, wrong emitter, wrong source manager, wrong guardian set, foreign key, wrong recipient | `testRedTeamInboundEdgeCasesRejected` | Held |
| A12 | Governance VAAs with a wrong emitter chain, emitter address, guardian set, epoch, manager, chain, or action; replay; stale epoch after a rotation | `testRedTeamGovernanceForgeriesRejected` | Held |
| A13 | Operator, relayer, gg alone, or a stranger moves the admin role, a peer, the ledger, or custody | `testRedTeamUnauthorizedMovesRejected` | Held |
| N1 | Registry code uses gg authority after the quorum accepts custody | `testRedTeamQuorumCustodyRegistryCannotMintBridgedInstrument` | Broken (F1) |
| N2 | A third-party relayer delivers a release on a production registry | `testRedTeamThirdPartyRelayerReleasesOnCoinRegistry` | Broken (F2) |
| N3 | Outbound transfer with fields that the guardians cannot deliver | `testRedTeamTransferRejectsUndeliverableMessage` | Broken (F3) |

A4 note: the harness cannot make a `CoinFactory` from a different package version.
The test package can depend on one `token-cip0056` DAR only.

## F1: quorum custody gives gg authority to registry code

Severity: High.
The attack needs the quorum to accept custody of a lock/unlock deployment.
After that, the registry can mint each burn/mint instrument with no limit.

### Cause

- `TransferFactory_Transfer` has the controller `transfer.sender`.
- After `AcceptAdminTransferByVaa`, the custodian is gg.
- `ConsolidateCustody`, `Release`, `TransferAdmin`, and the lock merge in `Transfer` send with `sender = admin`, which is gg (`ntt/daml/Wormhole/Ntt/Manager.daml:598`, `:858`, `:964`, `:1015`).
- Thus the body of the registry choice has the authority of gg.
- The canonical `CoinFactory` has gg as its only signatory. `BurnMintFactory_BurnMint` needs only gg and the extra actors.
- `TransferPreapproval_Receive` in `token-cip0056` needs only the instrument admin, which is gg.
- `validateCustodyHolding` accepts a holding that the instrument admin signs alone (`Manager.daml:345`). Thus the registry admin can make holdings with `owner = gg` without gg.
- `ConsolidateCustody` is permissionless. Thus the registry admin can start the merge at any time.

The `SetFactory` comment states that the lock/unlock factory never wields gg (`Manager.daml:528`).
This statement is false after the quorum accepts custody.

### Reproduction

1. Register a burn/mint deployment C with the canonical gg `CoinFactory`.
2. As registry admin R, create `SenderAuthorityRegistryFactory`. This registry mints C's instrument when the sender is gg.
3. As R, register a lock/unlock deployment with this factory.
4. As R, create an `AdminTransferProposal` to gg.
5. As a relayer, submit `AcceptAdminTransferByVaa` with a valid `AcceptAdminToGovernance` VAA.
6. As R, create two `RegistryHolding` contracts with `owner = gg`.
7. As R, submit `ConsolidateCustody` with these two holdings.
8. Result: R owns 1000000.0 of C's bridged instrument.

The same authority also lets the registry move each holding that gg owns.
This includes other gg custody pots and collected message fees.

### Proposed fix

Do not use gg as the CIP-56 sender for third-party registry code.
Possible changes:

1. Give quorum custody to a dedicated custody party, separate from the gg mint authority.
2. Make the mint authority a separate party, or make `CoinFactory` co-signed by the operator.
3. Accept quorum custody only for registry templates on an allowlist, as `requireCanonicalFactory` does for burn/mint.

Also correct the `SetFactory` comment at `Manager.daml:528`.

## F2: a third-party relayer cannot deliver a release

Severity: Medium.
Value stays safe. Inbound unlocks to ordinary recipients need the recipient to submit.

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

### Proposed fix

Check conservation on the sender side, which the custodian authority can fetch.
Compare the input total, less the change total, with the release amount.
Alternatively, require a recipient consent contract that the recipient signs, as `DepositPreapproval` does for `Mint`.

## F3: outbound transfer accepts fields that the guardians cannot deliver

Severity: Low.
A wrong client input burns or locks value, and no peer can redeem it.

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

### Proposed fix

Add these assertions at the start of `Transfer`:

1. `0 <= consistencyLevel <= 255`
2. `0 <= nonce <= 4294967295`
3. `byteLength recipientAddress == 32` and `isLowerHex recipientAddress`
4. `not (isZeroAddress recipientAddress)`

## Observations with no finding

- `ConsolidateCustody` is permissionless. A caller can merge the pot of a sibling deployment with the same admin into one pot. The ledger caps still hold. The sibling hint then points to an archived holding, and the sibling must use explicit holdings or a new consolidation.
- An inbound amount that overflows the 64-bit `Int` after rescale makes the VAA fail. Examples are wire decimals 0 with a very large amount, or wire decimals 255. A correct peer does not send these values.
- A fetch of a holding that the caller does not own fails at the ledger before the NTT custodian check. The result is the same rejection.
