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

## Splice token-standard interfaces (CIP-0056 / Token Standard V1)

Interface-only packages, frozen at 1.0.0. The package-ids must match what
participants on the Canton Network have vetted. `ntt` drives them all
directly: `holding` and `metadata` (holdings and choice context),
`transfer-instruction` and `burn-mint` (the lock/unlock and burn/mint
factories), and `allocation` (the fee path). All copies must be the same
network-vetted 1.0.0 packages: damlc dedupes transitive DALFs by package-id,
and mixing incompatible copies risks conflicts.

Provenance: `0.6.12_splice-node.tar.gz` from
https://github.com/digital-asset/decentralized-canton-sync/releases/tag/v0.6.12
(path `splice-node/dars/` inside the bundle). These 1.0.0 interface DARs are
immutable and distributed identically across releases (verified byte-for-byte by
sha256 against the copies shipped in cn-quickstart). Built `--target=2.1`; our
packages target LF 2.3, which may data-depend on lower LF 2.x versions.

## `splice-amulet` (real Canton Coin / Amulet templates, `ntt-test` only)

`ntt-test`'s `Playground.Amulet` module needs concrete `TemplateTypeRep`s
(`AmuletRules`, `TransferPreapproval`, `OpenMiningRound`,
`ExternalPartyAmuletRules`, `ExternalPartyConfigState`) to construct `Disclosure`
values for the registry's disclosed contracts when locking/unlocking real
Amulet through a CIP-56 custody token (`Cip56CustodyToken`) on Splice LocalNet.
This is the only reason it is vendored: the playground never mints or burns
Amulet directly, and the production DAR (`ntt`) never depends on it.

Provenance: `0.6.12_splice-node.tar.gz` (same release as the token-standard
interfaces above), path `splice-node/dars/splice-amulet-0.1.22.dar`. Verified
byte-identical to `splice-amulet-current.dar` in the same bundle. Its embedded
interface DALF package-ids (`holding-v1`, `transfer-instruction-v1`,
`metadata-v1`) match the ones already vendored above, so `damlc` dedupes them
cleanly with no conflict.

**Consumed by `ntt-test` only** (a `data-dependencies` entry in
`canton/test/daml.yaml`) — never a dependency of `ntt`.

## sha256

| DAR | consumed by | sha256 |
| --- | --- | --- |
| `wormhole-core-0.3.0.dar` | `ntt`, `ntt-test` | `7587fe8fecd3b61b3b1eddc3558c227fa97eb577302466fdf24c3bb2ed55a090` |
| `splice-api-token-metadata-v1-1.0.0.dar` | `ntt`, `ntt-test` | `455eb160cb5abd4ae9918a6fbb9dad471f721adda39f0e5c76feef08d05637fc` |
| `splice-api-token-holding-v1-1.0.0.dar` | `ntt`, `ntt-test` | `ef75f8eb41a65810221784fdb78bb9dfac7cb22245aba14fa7cb7f69c34e0175` |
| `splice-api-token-allocation-v1-1.0.0.dar` | `ntt`, `ntt-test` | `c3f3b447142577ea4fa7d912ca11cd6821de7588e324e8877425932a02fccaa1` |
| `splice-api-token-burn-mint-v1-1.0.0.dar` | `ntt`, `ntt-test` | `a18e85c4841a278bce000df8329c3f0e2fee3b30b55dd6a31492d10a72b4f9c1` |
| `splice-api-token-transfer-instruction-v1-1.0.0.dar` | `ntt`, `ntt-test` | `e4c73aa7ae73fb2fc330b938ffb99f568792321640ba4b9472902aa8d742c994` |
| `splice-amulet-0.1.22.dar` | `ntt-test` only | `bcfcd6a9250172a8384d1326a2990b374c66a367ca7d4120b243aecfd372761e` |

Verify with `sha256sum <file>` against this table before trusting a copy.
