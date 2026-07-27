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

Provenance: built from `wormholelabs-xyz/wormhole` branch `canton/replay-namespace`
(open PR #52, unreleased 0.3.0) at commit `22f9216ea` — scopes the replay trie
by `(consumer, namespace)` instead of `consumer` alone, and separates `payer`
from `requester`/`consumer` on `RegisterEmitter`/`ClaimReplayRoot`.

## Splice token-standard interfaces (CIP-0056 / CIP-0112 — Token Standard V1 + V2)

Interface-only packages. The package-ids must match what participants on the
Canton Network have vetted. `ntt` drives them all directly: `holding`/`holding-v2`
and `metadata` (holdings and choice context), `transfer-instruction`/
`transfer-instruction-v2` (plain transfers, both CIP-0056 and CIP-0112 shapes),
`allocation`/`allocation-v2` + `allocation-instruction-v1`/`-v2` (DvP/atomic
settlement), and `burn-mint` (NTT's own internal, non-standard bridge mint/burn
mechanism — deliberately excluded from the Token Standard family, see
`Wormhole.Ntt.Coin`'s module header). All copies must be the same network-vetted
packages: damlc dedupes transitive DALFs by package-id, and mixing incompatible
copies risks conflicts.

Provenance: `0.6.14_splice-node.tar.gz` from
https://github.com/digital-asset/decentralized-canton-sync/releases/tag/v0.6.14
(path `splice-node/dars/` inside the bundle) — re-pinned from `v0.6.12` on
2026-07-26 to pick up the CIP-0112 v2 interfaces; every v1 DAR already vendored
is confirmed byte-identical between the two releases (same sha256 below), so
this changed no previously-vendored bytes. These interface DARs are immutable
and distributed identically across releases (verified byte-for-byte by sha256
against the copies shipped in cn-quickstart, and across v0.6.11-v0.6.14 for the
v2 files). Built `--target=2.1`; our packages target LF 2.3, which may
data-depend on lower LF 2.x versions.

## sha256

| DAR | consumed by | sha256 |
| --- | --- | --- |
| `wormhole-core-0.3.0.dar` | `ntt`, `ntt-test` | `7587fe8fecd3b61b3b1eddc3558c227fa97eb577302466fdf24c3bb2ed55a090` |
| `splice-api-token-metadata-v1-1.0.0.dar` | `ntt`, `ntt-test` | `455eb160cb5abd4ae9918a6fbb9dad471f721adda39f0e5c76feef08d05637fc` |
| `splice-api-token-holding-v1-1.0.0.dar` | `ntt`, `ntt-test` | `ef75f8eb41a65810221784fdb78bb9dfac7cb22245aba14fa7cb7f69c34e0175` |
| `splice-api-token-holding-v2-1.0.0.dar` | `ntt`, `ntt-test` | `f044b34d7a28c2b6c8a540af720297d55450a889b451dc4eed64a9914f1d6b00` |
| `splice-api-token-allocation-v1-1.0.0.dar` | `ntt`, `ntt-test` | `c3f3b447142577ea4fa7d912ca11cd6821de7588e324e8877425932a02fccaa1` |
| `splice-api-token-allocation-v2-1.0.0.dar` | `ntt`, `ntt-test` | `e349d3a1952cdd52ed55333bef35e9ba40e6e0379521d65556c6aa2f50d9a1fd` |
| `splice-api-token-allocation-instruction-v1-1.0.0.dar` | `ntt`, `ntt-test` | `e2607ca3a1d735a82d3066b78132aa8f94b1886c99a5f14148742d252c7220a2` |
| `splice-api-token-allocation-instruction-v2-1.0.0.dar` | `ntt`, `ntt-test` | `31de6be30385936105b24c8ea544bb25f6eede0d0c008154a11462bb6d14a17b` |
| `splice-api-token-burn-mint-v1-1.0.0.dar` | `ntt`, `ntt-test` | `a18e85c4841a278bce000df8329c3f0e2fee3b30b55dd6a31492d10a72b4f9c1` |
| `splice-api-token-transfer-instruction-v1-1.0.0.dar` | `ntt`, `ntt-test` | `e4c73aa7ae73fb2fc330b938ffb99f568792321640ba4b9472902aa8d742c994` |
| `splice-api-token-transfer-instruction-v2-1.0.0.dar` | `ntt`, `ntt-test` | `720e766456b33abdd26bfc21e54d19413559f0a7ee52000e12d3b6c308199faa` |

Verify with `sha256sum <file>` against this table before trusting a copy.
