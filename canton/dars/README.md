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

Provenance: built from `wormholelabs-xyz/wormhole` PR #81 (branch
`canton/core-fee-custody`) at its head commit
`fd66847d7658f8159d5850aee1a6857171c30fb6` (version 0.8.1), SDK 3.5.1. The
main package id is
`5b256d1377919ed618dffcf3f632dd05e97702ca99e4a3fe5320131b57ec690f`. The
package id identifies the build. A squash merge of PR #81 changes the commit
id, but not the package id.

Version 0.8.1 declares 0.7.0 (`7a22a1c4...`) as its `upgrades:` baseline.
Version 0.8.0 (`03a65dc3...`) is withdrawn. `guardianGovernance` co-signs
`EmitterRegistry` and `Emitter`. `RegisterEmitter` takes
`expectedGuardianGovernance`; `ntt` passes `Some guardianGovernance`.

Version 0.8.1 adds the fee recipient (`feeCustody`) as a signatory of
`CoreState`. The `CoreState` precondition rejects a fee recipient that is
`guardianGovernance` or the operator. When a fee is due, `chargeFee` rejects
a payer that is `guardianGovernance` or the operator. Thus a deployment with
`admin = gg` cannot pay a non-zero fee as `payer = admin`. 0.7.0 contracts
cannot run under 0.8.1, so 0.8.1 starts from a fresh genesis.

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

## Previous `ntt` release (upgrade baseline)

`ntt-0.2.0.dar` is the baseline for the compile-time smart contract upgrade
(SCU) check. `ntt/daml.yaml` references it in its `upgrades:` field. `damlc`
rejects an `ntt` build that is not upgrade-compatible with it.

- Source: this repo, PR #74 (branch commit
  `1db4b3cf1caca05af7028e1585d6a99c859cc910`), `canton/ntt`, built with
  SDK 3.5.1.
- Main package id:
  `8b9c3ee1c93b7039b39421b790c283f507c2b6a01171aa839873f220dc5ff84a`. This
  is the build on the LocalNet rehearsal.

To check the package id, run
`dpm damlc inspect-dar --json dars/ntt-0.2.0.dar` and read
`main_package_id`. The DAR file hash is not reproducible across builds; the
package id is reproducible.

## sha256

| DAR | consumed by | sha256 |
| --- | --- | --- |
| `ntt-0.2.0.dar` | `ntt` (`upgrades:`) | `12910821c7147b3b1702c82d473a027b73e1568b0fbb725477f18b09e08f6aec` |
| `wormhole-core-0.8.1.dar` | `ntt`, `ntt-test` | `32dba2510c7140e5d2660ebc6ffe43386c694c392240780d5fe473e11d9bff80` |
| `token-cip0056-0.2.0.dar` | `ntt`, `ntt-test` | `fd193587c74f590bd6a1c782e56a351301e596f57ca3c4cb3dd180cfd3a19e02` |
| `splice-api-token-metadata-v1-1.0.0.dar` | `ntt`, `ntt-test` | `455eb160cb5abd4ae9918a6fbb9dad471f721adda39f0e5c76feef08d05637fc` |
| `splice-api-token-holding-v1-1.0.0.dar` | `ntt`, `ntt-test` | `ef75f8eb41a65810221784fdb78bb9dfac7cb22245aba14fa7cb7f69c34e0175` |
| `splice-api-token-allocation-v1-1.0.0.dar` | `ntt`, `ntt-test` | `c3f3b447142577ea4fa7d912ca11cd6821de7588e324e8877425932a02fccaa1` |
| `splice-api-token-allocation-instruction-v1-1.0.0.dar` | `ntt`, `ntt-test` | `e2607ca3a1d735a82d3066b78132aa8f94b1886c99a5f14148742d252c7220a2` |
| `splice-api-token-burn-mint-v1-1.0.0.dar` | `ntt`, `ntt-test` | `a18e85c4841a278bce000df8329c3f0e2fee3b30b55dd6a31492d10a72b4f9c1` |
| `splice-api-token-transfer-instruction-v1-1.0.0.dar` | `ntt`, `ntt-test` | `e4c73aa7ae73fb2fc330b938ffb99f568792321640ba4b9472902aa8d742c994` |

Verify with `sha256sum <file>` against this table before trusting a copy.
