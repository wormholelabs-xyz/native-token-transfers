# canton/testing/cli

Go packages and standalone binaries supporting the Canton NTT test harnesses. The full
playground CLI (network bring-up, party allocation, transfer/receive flows, and the
sandbox/LocalNet e2e suite) lives on `canton/playground-merged`; this branch carries only the
pieces ported for standalone use.

## disclosure-service

`cmd/disclosure-service` is a harness-grade, unauthenticated HTTP sidecar that fronts one
Canton participant. It lets a consumer read allow-listed active contracts, or run a
prepare-seam Daml Script, without holding that participant's own admin credentials. It has no
TLS, no auth beyond an optional bearer token file, and binds to loopback by default -- it is
not a production service.

Endpoints:

- `GET /v1/healthz` -- participant name, configured template allow-list, and (when an ACS
  backend is wired) the current ledger end.
- `GET /v1/disclosures?template=<name>` -- active contracts for one or more allow-listed
  templates, read via the JSON Ledger API v2. 503s if no ACS backend is configured
  (`--acs-party` unset, or the target participant has no JSON API endpoint).
- `POST /v1/seam/{name}` -- runs one of a fixed set of `Playground.Prepare` Daml Scripts
  (`transferOut`, `receive`, `publish`, `acceptAdminTransfer`, `deployNtt`) via `dpm script`
  and returns its JSON output. The server's own template allow-list is always injected into the
  script input, overriding anything the caller sent.

### Build and run

```sh
cd canton/testing/cli
go build ./cmd/disclosure-service
./disclosure-service --profile sandbox
```

Common flags:

- `--listen` (default `127.0.0.1:7599`) -- bind address; a non-loopback value prints a warning.
- `--participant` (default `guardian-governance`) -- which participant this instance fronts.
- `--profile` (`sandbox` or `localnet`) -- network profile, per `internal/profile`.
- `--topology-config` -- path to the disclosure allow-list config; a missing file falls back to
  built-in defaults (see `internal/disclosure.Config`).
- `--dar` -- path to the compiled test DAR; defaults to discovering `canton/` by walking up for
  `multi-package.yaml` and resolving `test/.daml/dist/ntt-test-0.1.0.dar`.
- `--dpm-path` -- path to the `dpm` binary; defaults to `PATH` or `~/.dpm/bin`.
- `--access-token-file` -- path to a file holding a bearer JWT. This is the service's only auth
  input (it mints nothing); required whenever the selected profile requires auth.
- `--acs-party` -- reading party for `GET /v1/disclosures`; ACS stays disabled without it.
- `--verbose` -- narrate each `dpm script` invocation to stderr.
