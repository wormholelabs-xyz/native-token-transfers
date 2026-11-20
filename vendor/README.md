# Vendored packages

## wormhole-foundation-sdk-base-6.1.6-canton.tgz

This tarball is `@wormhole-foundation/sdk-base` 6.1.6 with the Canton chain added.
The Canton values are: chain id 75, platform `Canton`, address format `hex`, 10 native decimals, finality 0, block time 1000 ms.
The upstream SDK does not include Canton.

The root `package.json` `overrides` entry replaces `sdk-base` with this tarball in every workspace.
The root `devDependencies` entry names the same tarball. Bun 1.3.4 does not extract a `file:` tarball that only an override names.
The `bun.lock` entry records its SHA-512 integrity.

### Provenance

| Item                   | Value                                                                                       |
| ---------------------- | ------------------------------------------------------------------------------------------- |
| Source repository      | `wormhole-foundation/wormhole-sdk-ts`                                                       |
| Source tag             | `6.1.6` (commit `2a8ea439ef7c928aa0d0d139ec008c1a11909b2a`)                                 |
| Canton patch           | `vendor/sdk-base-6.1.6-canton.patch` (4 files in `core/base/src/constants/`, 8 added lines) |
| Patch origin           | `wormholelabs-xyz/wormhole-sdk-ts` commit `b8bfa7a1`, applied to tag `6.1.6`                |
| Patch sha256           | `3a9576cd8f8dd557f45cadb94ae8888f4aae1dc56703f53280b68d250a697409`                          |
| Packed version         | `6.1.6`                                                                                     |
| Committed file         | tarball sha256: `2cf60330257c30bff5d66515356bd9478e811b1215aa61d48397b018a80076cc`          |
| Content after `gunzip` | tar sha256: `90a8f70c3a72efce139f5eab0136a01da79a52d8cc4161fbefc7ea05628b0c66`              |
| SHA-512 (base64)       | `2OSen8GRRN9Yi6yvroiRqKxLoaqY4d8eDwMoT4s8bRUnzQDsf2q/uC6jgqLZwudFPA1rJNG2p4HG5U9U6b549g==`  |
| Build tools            | Node.js 24.22.1, npm 11.19.1                                                                |

The tarball and the registry package `sdk-base@6.1.6` have the same 193 file paths.
The executable difference is 9 Canton lines in `chains.js`, `decimals.js`, `finality.js` and `platforms.js`.
The package has no `install`, `preinstall`, `postinstall` or `prepare` script.

The `Vendor provenance` workflow builds the tarball again from the tag and the patch.
It fails when the tar content hash is different from the value in this file.

### Rebuild procedure

Do these steps in a temporary directory outside this repository.

1. Clone the source tag:
   `git clone --depth 1 --branch 6.1.6 https://github.com/wormhole-foundation/wormhole-sdk-ts.git`.
2. Make sure that `git rev-parse HEAD` shows `2a8ea439ef7c928aa0d0d139ec008c1a11909b2a`.
3. Apply the patch: `git apply <this repository>/vendor/sdk-base-6.1.6-canton.patch`.
4. Audit the lockfile: `npm audit --omit=dev`.
5. Install from the lockfile, without lifecycle scripts: `npm ci --ignore-scripts`.
6. Check the registry signatures: `npm audit signatures`.
7. Test the package: `npm test -w core/base`.
8. Build the package: `npm run build -w core/base`.
9. In `core/base`, set the version: `npm pkg set version=6.1.6`.
10. In `core/base`, make the tarball: `npm pack`.
11. Compare the tar content hash with the table above: `gunzip -c <tarball> | shasum -a 256`.

### Install in this repository

Use the frozen lockfile only:

1. Audit the lockfile: `bun audit`.
2. Install: `bun install --frozen-lockfile --ignore-scripts`.

### SDK upgrade

1. Apply the Canton patch to the new upstream tag.
2. Do the rebuild procedure.
3. Replace the tarball and the patch.
4. Update the override and the `devDependencies` entry in the root `package.json`.
5. Update the `bun.lock` entries for the override and for `sdk-base`.
6. Update the values in this file and in the `Vendor provenance` workflow.
