# Vendored packages

## wormhole-foundation-sdk-base-6.1.4-canton.tgz

`@wormhole-foundation/sdk-base` 6.1.4 plus the private Canton chain (id 75, platform
`Canton`, hex address format, 10 native decimals, instant finality). Forced onto every
consumer via the flat `overrides` entry in the root `package.json`.

- Source: `wormholelabs-xyz/wormhole-sdk-ts`, branch `canton-6.1.4`, commit `b8bfa7a1`
  (based on upstream tag `6.1.4`).
- sha256: `3d100c687dba03cbb28a151a51ab88291fe57b661da0e3e02aeaa632f93dfc68`

Rebuild:

```
git clone -b canton-6.1.4 https://github.com/wormholelabs-xyz/wormhole-sdk-ts.git
cd wormhole-sdk-ts && npm ci
npm test -w core/base && npm run build -w core/base
cd core/base && npm pkg set version=6.1.4 && npm pack
```

In-repo package versions on the branch stay at the upstream placeholder; the version
is stamped only at pack time (a committed bump desyncs the monorepo lockfile).

Delivery is a flat `overrides` entry in the root `package.json`, honored directly by
`bun install` — no npm, no install.sh patch step needed. This repo uses Bun
workspaces; run `bun install` (no network for this dependency edge — the tarball is
committed in this directory).

SDK upgrades: rebase `canton-6.1.4` onto the new upstream tag, re-pack, replace the
tarball, update the override entry (filename/version) and this file.

There is exactly one `@wormhole-foundation/sdk-base` resolved across the workspace —
the hoisted, Canton-aware copy. There is no nested Canton-less duplicate to route
around.
