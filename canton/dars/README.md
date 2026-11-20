# Vendored Canton DARs

This directory vendors the built DAR binaries the Canton NTT packages
data-depend on. Daml has no git or URL dependency mechanism
(`data-dependencies` in `daml.yaml` only accepts local paths to built `.dar`
files), so cross-repo dependencies are committed here as binaries with
provenance and sha256. These DARs are taken verbatim from a canonical build
and must never be rebuilt from source: a different compiler produces
different package-ids, which would break both on-network vetting (Splice
interfaces) and package-id identity against what is actually deployed
(`wormhole-core`).

## `wormhole-core` (the core bridge primitives)

`ntt` and the `ntt-test` package data-depend on `wormhole-core` (publish via
`Emitter`, parse/verify VAAs, the `GetGuardianGovernance` trust anchor). It is
developed in `wormholelabs-xyz/wormhole` under `canton/core` and is not built
in this repo; it is consumed as a pinned artifact, exactly as the Splice
interfaces below are.

Provenance: built from wormhole integration/canton b2de948da (PR #100 squash),
commit `b2de948da498f6fd741f3c0d9892de29e7de29aa`, version 0.12.0, SDK 3.7.3,
LF target 2.3. The main package id is
`aec76d21ac140c49f91754db4667e0572d24d3b1cacd85b69dc76c92ae38087b`.

Version 0.12.0 has no `upgrades:` baseline. It is not upgrade-compatible with
0.11.1: the `GuardianSet` type and the VAA choice signatures changed. A
contract of an earlier version cannot run under 0.12.0, so 0.12.0 starts from
a fresh genesis. Versions 0.8.0 to 0.8.3 are withdrawn.

Version 0.12.0 has these changes for `ntt`:

- Each byte value has a constrained type with a private constructor
  (`Wormhole.Core.Bytes`). On the Ledger API, each value is a variant
  `{"tag": "<type>", "value": "<lowercase hex>"}`.
- `CoreState` holds the public key of each guardian.
  `ParseAndVerifyVAA`, `VerifyAndConsumeVAA` and `SubmitGovernanceVAA` have
  no `pubKeys` argument.
- `CoreState.consumedGovernance` maps each digest to its VAA timestamp. It
  holds 32 digests or fewer, with an eviction floor.

`guardianGovernance` (`gg`) is the only core anchor. It signs `CoreState`,
`EmitterRegistry`, `ReplayRootRegistry`, each `Emitter` and each
`ReplayNode`. The fee recipient (`feeCustody`) also signs `CoreState`.
`RegisterEmitter` and `ClaimReplayRoot` take `expectedGuardianGovernance`.
`ntt` passes `guardianGovernance`. Any party can submit a governance VAA.
A delegated `onboardingFeeAdmin` sets the registration fee and the claim fee.

The `CoreState` precondition has these rules:

- The fee recipient is not `guardianGovernance`.
- The fee instrument admin is not `guardianGovernance`.
- The fee instrument admin is not the fee recipient.
- The message fee is 0 or more.
- The current guardian set exists and has no expiry. Each other set has a
  lower index and an expiry. `CoreState` stores 4 sets or fewer.
- Each byte value passes the rule of its type.
- The governance replay record holds 32 digests or fewer, each above the
  floor.
- With no governance history, each guardian has a public key that derives its
  address.

When a fee is due, `chargeFee` repeats the rules for the payer, the fee
recipient and the fee instrument admin. The test setups use a placeholder
fee instrument admin that is a separate party.

`ntt` data-depends on `wormhole-core` by package id, so each NTT call into
core runs the code of this exact package id. Do not vendor a core version
older than the one that the guardians observe.

## Splice token-standard interfaces (CIP-0056 — Token Standard V1)

Interface-only packages. The package-ids must match what participants on the
Canton Network have vetted. `ntt` drives them all directly: `holding` and
`metadata` (holdings and choice context), `transfer-instruction` (plain
transfers), `allocation` + `allocation-instruction` (DvP/atomic settlement),
and `burn-mint` (NTT's own internal, non-standard bridge mint/burn mechanism —
deliberately excluded from the Token Standard family, see
`Wormhole.Ntt.CoinFactory`'s module header). All copies must be the same
network-vetted packages: damlc dedupes transitive DALFs by package-id, and
mixing incompatible copies risks conflicts.

Provenance: `0.6.14_splice-node.tar.gz` from
https://github.com/digital-asset/decentralized-canton-sync/releases/tag/v0.6.14
(path `splice-node/dars/` inside the bundle). These interface DARs are
immutable and distributed identically across releases (verified byte-for-byte
by sha256 against the copies shipped in cn-quickstart). Built `--target=2.1`;
our packages target LF 2.3, which may data-depend on lower LF 2.x versions.
The wormhole repository at `b2de948da` vendors the same three core interface
DARs with the same sha256 (from the 0.6.12 bundle).

## `token-cip0056` (the CIP-0056 coin template and factory)

`ntt` data-depends on `token-cip0056` for the `Coin`/`CoinFactory`/
`CoinAllocation`/`CoinTransferInstruction` templates that back the manager's
burn/mint deployments. `requireCanonicalFactory` in `Wormhole.Ntt.Manager`
pins the caller-supplied factory to the `Token.CIP0056.CoinFactory` template
with `fetchFromInterface`.

The pin uses the package name, not the package ID. Canton resolves a template
by package name across upgrades, so the check accepts a `CoinFactory` of each
vetted `token-cip0056` version. An interface choice runs the code of the
vetted version that the submitter prefers. Thus, on each participant that
hosts gg, the vetting of a new `token-cip0056` version is a decision about
the gg mint authority.

It is developed in `wormholelabs-xyz/CantonExamples` under `tokens/cip0056`
and is consumed as a pinned artifact, exactly as `wormhole-core` above.

Provenance: built from `wormholelabs-xyz/CantonExamples` branch `dev` at
commit `b448589d8742ef5983ec8dc7c6ddd275ceccd1b7` — package `token-cip0056`
version `0.2.0`, sdk-version 3.5.1, LF target 2.3.

The SDK 3.7.3 packages use this DAR without a rebuild. LF 2.3 is valid on
protocol versions 36 and 37. A rebuild makes a new package id, and each `gg`
host vets the package id.

## sha256

| DAR                                                    | consumed by  | sha256                                                             |
| ------------------------------------------------------ | ------------ | ------------------------------------------------------------------ |
| `wormhole-core-0.12.0.dar`                             | each package | `5f1c491bfe41f116c272a7e2c97c0db969f2fb1f8019716a23c8defec5c2b663` |
| `token-cip0056-0.2.0.dar`                              | each package | `fd193587c74f590bd6a1c782e56a351301e596f57ca3c4cb3dd180cfd3a19e02` |
| `splice-api-token-metadata-v1-1.0.0.dar`               | each package | `455eb160cb5abd4ae9918a6fbb9dad471f721adda39f0e5c76feef08d05637fc` |
| `splice-api-token-holding-v1-1.0.0.dar`                | each package | `ef75f8eb41a65810221784fdb78bb9dfac7cb22245aba14fa7cb7f69c34e0175` |
| `splice-api-token-allocation-v1-1.0.0.dar`             | each package | `c3f3b447142577ea4fa7d912ca11cd6821de7588e324e8877425932a02fccaa1` |
| `splice-api-token-allocation-instruction-v1-1.0.0.dar` | each package | `e2607ca3a1d735a82d3066b78132aa8f94b1886c99a5f14148742d252c7220a2` |
| `splice-api-token-burn-mint-v1-1.0.0.dar`              | each package | `a18e85c4841a278bce000df8329c3f0e2fee3b30b55dd6a31492d10a72b4f9c1` |
| `splice-api-token-transfer-instruction-v1-1.0.0.dar`   | each package | `e4c73aa7ae73fb2fc330b938ffb99f568792321640ba4b9472902aa8d742c994` |

"Each package" is `ntt`, `ntt-test`, `wormhole-genesis` and
`wormhole-genesis-test`.

Verify with `sha256sum <file>` against this table before trusting a copy.
