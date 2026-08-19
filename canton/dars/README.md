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

Provenance: built from `wormholelabs-xyz/wormhole` branch `integration/canton`
(merged PR #53, version 0.4.0) at commit
`8663c5b7abea81df18a7075a77eaac00a9a065d0` — `Emitter.PublishMessage` returns
the successor `Emitter` contract id alongside the message, the replay trie is
scoped by `(consumer, namespace)` instead of `consumer` alone, and `payer` is
separate from `requester`/`consumer` on `RegisterEmitter`/`ClaimReplayRoot`.

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
from this exact DAR via `fetchFromInterface`, so its package-id is part of the
canonical-factory identity: re-vendoring a different build changes what
counts as "the" factory.

It is developed in `wormholelabs-xyz/CantonExamples` under `tokens/cip0056`
and is consumed as a pinned artifact, exactly as `wormhole-core` above.

Provenance: built from `wormholelabs-xyz/CantonExamples` branch `dev` at
commit `b448589d8742ef5983ec8dc7c6ddd275ceccd1b7` — package `token-cip0056`
version `0.2.0`, sdk-version 3.5.1, LF target 2.3.

## `splice-amulet` (real Canton Coin / Amulet templates, `ntt-test` only)

`ntt-test`'s `Playground.Disclose` module needs concrete `TemplateTypeRep`s
(`AmuletRules`, `Round`, `ExternalPartyAmuletRules`, `ExternalPartyConfigState`)
to construct `Disclosure` values for the registry's disclosed contracts. That
is the only reason we vendor this DAR. The production DAR (`ntt`) never
depends on it.

Provenance: `0.6.12_splice-node.tar.gz`, the same release as the
token-standard interfaces above, path
`splice-node/dars/splice-amulet-0.1.22.dar`. We checked that the file is
byte-identical to `splice-amulet-current.dar` in the same bundle. Its embedded
interface DALF package-ids (`holding-v1`, `transfer-instruction-v1`,
`metadata-v1`) match the ones above, so `damlc` dedupes them with no conflict.

**Only `ntt-test` consumes this DAR** (a `data-dependencies` entry in
`canton/test/daml.yaml`). It is never a dependency of `ntt`.

## sha256

| DAR | consumed by | sha256 |
| --- | --- | --- |
| `wormhole-core-0.4.0.dar` | `ntt`, `ntt-test` | `3eb9ae97cd34d884fa3c2d4242ce7106f7bcb3cc067c615005233c99300bc3fd` |
| `token-cip0056-0.2.0.dar` | `ntt`, `ntt-test` | `fd193587c74f590bd6a1c782e56a351301e596f57ca3c4cb3dd180cfd3a19e02` |
| `splice-api-token-metadata-v1-1.0.0.dar` | `ntt`, `ntt-test` | `455eb160cb5abd4ae9918a6fbb9dad471f721adda39f0e5c76feef08d05637fc` |
| `splice-api-token-holding-v1-1.0.0.dar` | `ntt`, `ntt-test` | `ef75f8eb41a65810221784fdb78bb9dfac7cb22245aba14fa7cb7f69c34e0175` |
| `splice-api-token-allocation-v1-1.0.0.dar` | `ntt`, `ntt-test` | `c3f3b447142577ea4fa7d912ca11cd6821de7588e324e8877425932a02fccaa1` |
| `splice-api-token-allocation-instruction-v1-1.0.0.dar` | `ntt`, `ntt-test` | `e2607ca3a1d735a82d3066b78132aa8f94b1886c99a5f14148742d252c7220a2` |
| `splice-api-token-burn-mint-v1-1.0.0.dar` | `ntt`, `ntt-test` | `a18e85c4841a278bce000df8329c3f0e2fee3b30b55dd6a31492d10a72b4f9c1` |
| `splice-api-token-transfer-instruction-v1-1.0.0.dar` | `ntt`, `ntt-test` | `e4c73aa7ae73fb2fc330b938ffb99f568792321640ba4b9472902aa8d742c994` |
| `splice-amulet-0.1.22.dar` | `ntt-test` only | `bcfcd6a9250172a8384d1326a2990b374c66a367ca7d4120b243aecfd372761e` |

Verify with `sha256sum <file>` against this table before trusting a copy.
