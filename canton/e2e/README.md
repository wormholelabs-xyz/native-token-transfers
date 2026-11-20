# Canton ↔ Ethereum NTT end-to-end test

This test sends an NTT transfer from Ethereum to Canton and back on the Wormhole
devnet. The devnet guardian signs both VAAs. Unlike the Daml Script suite in
`../test`, nothing here signs a VAA by hand or uses a made-up peer.

## Run it

```sh
git submodule update --init --recursive   # evm/lib
tilt up -f Tiltfile.canton-evm -- --canton
```

The `ntt-canton-evm-e2e` resource runs the test once eth-devnet, the guardian and
Canton are ready. It is green when the test passes. Trigger it again to run the
test again. Each run deploys new contracts and parties.

To tear it down, pass the same Tiltfile and flags. A bare `tilt down` loads the
repository's default `Tiltfile` instead:

```sh
tilt down -f Tiltfile.canton-evm -- --canton
```

The wormhole devnet comes from `wormholelabs-xyz/wormhole`, branch
`integration/canton`, checked out into `.wormhole-canton/`. To use a local
wormhole checkout instead, set `NTT_WORMHOLE_DIR`:

```sh
NTT_WORMHOLE_DIR=../wormhole tilt up -f Tiltfile.canton-evm -- --canton
```

`.wormhole-canton/` is then a symlink to that checkout. If `.wormhole-canton/`
is already a git checkout from an earlier run, remove it first.

## What it does

[`run.sh`](run.sh) runs these steps:

1. **EVM deploy.** It deploys `DummyToken` (18 decimals), then runs
   `DeployWormholeNtt.s.sol` for a locking NttManager with a WormholeTransceiver.
2. **Canton deploy.** `Test.TiltE2E:tiltE2eSetup` creates a core instance with
   the devnet guardian, the NTT governance root and a burn/mint manager (8
   decimals) on chain 75. It registers the EVM manager and transceiver as the
   chain 2 peer, and creates the recipient's deposit pre-approval.
3. **EVM peer.** `setPeer` and `setWormholePeer` register the Canton manager and
   transceiver for chain 75.
4. **EVM → Canton.** It locks 2.5 tokens on EVM, fetches the VAA from the
   guardian, and mints with `Test.TiltE2E:tiltE2eReceive`. A second mint with
   the same VAA must fail.
5. **Canton → EVM.** `Test.TiltE2E:tiltE2eSend` burns 1.0 token on Canton. The
   script fetches the VAA and calls `receiveMessage` on the EVM transceiver. A
   fresh EVM address must receive exactly 1.0 token.

The log shows the hash of every EVM transaction (`evm tx`) and the update id of
every Canton transaction (`canton update`), each with what it did. The update ids
come from the participant's JSON Ledger API on `canton:6864`. A wormhole devnet
that doesn't expose it still runs the test, without the update ids.

The Canton steps live in
[`../test/daml/Test/TiltE2E.daml`](../test/daml/Test/TiltE2E.daml). They pass
party ids and addresses between steps as JSON, and look up the live contracts
again in each step.

The EVM account is devnet anvil account 15, which no other devnet test uses.

## Notes

- The wormhole checkout must build the same `wormhole-core` that NTT vendors in
  `../dars`. The devnet bootstrap uploads its own core first, and Canton rejects
  the second one unless it is a valid upgrade (`NOT_VALID_UPGRADE_PACKAGE`). When
  the `integration/canton` tip has moved on, point `NTT_WORMHOLE_DIR` at a
  checkout with a matching `canton/core/daml.yaml` version.

- The test creates its own Canton core instance, separate from the one the
  devnet bootstrap creates. The devnet guardian watches every party, so it
  observes both.
- A Canton participant vets one package per name and version. If you change
  `TiltE2E.daml` (or anything else in `ntt-test`), the new `ntt-test` 0.1.0
  cannot replace the one an earlier run uploaded. The test then fails and says
  so. Restart the `canton` resource for a fresh sandbox.
- `run.sh` also runs outside Tilt. Set `ETH_RPC`, `GUARDIAN_REST`,
  `CANTON_LEDGER_HOST`, `CANTON_JSON_API`, `CORE_BRIDGE`, `EVM_DIR` and
  `NTT_TEST_DAR` to point it at other endpoints and files. An empty
  `CANTON_JSON_API` skips the update ids.
