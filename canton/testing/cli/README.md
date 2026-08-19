# canton/testing/cli

This module contains Go packages and standalone binaries for the Canton NTT test harnesses.
The full playground CLI lives on the `canton/playground-merged` branch. That branch has the
network bring-up, the party allocation, the transfer flows, and the e2e suite. This branch
carries only the parts that the disclosure service needs.

## disclosure-service

`cmd/disclosure-service` is an unauthenticated HTTP sidecar for test harnesses. It fronts one
Canton participant. Through it, a consumer can read allow-listed active contracts and run
prepare-seam Daml Scripts. The consumer does not need the participant's admin credentials.

The service is not a production service. It has no TLS. Its only auth input is an optional
bearer token file. It binds to loopback by default.

Endpoints:

- `GET /v1/healthz` — returns the participant name and the template allow-list. When the ACS
  backend is on, the response also contains the current ledger end.
- `GET /v1/disclosures?template=<name>` — returns active contracts for allow-listed templates.
  The service reads them from the JSON Ledger API v2. The service returns 503 when the ACS
  backend is off (no `--acs-party`, or the participant has no JSON API endpoint).
- `POST /v1/seam/{name}` — runs one `Playground.Prepare` Daml Script with `dpm script` and
  returns the script's JSON output. The seam names are `transferOut`, `receive`, `publish`,
  `acceptAdminTransfer`, and `deployNtt`. The server always replaces the caller's template
  list with its own allow-list.

### Build and run

1. Go to the module directory: `cd canton/testing/cli`.
2. Build the binary: `go build ./cmd/disclosure-service`.
3. Start the service: `./disclosure-service --profile sandbox`.

Flags:

- `--listen` (default `127.0.0.1:7599`) — the bind address. The service prints a warning for
  a non-loopback value.
- `--participant` (default `guardian-governance`) — the participant that this instance fronts.
- `--profile` (`sandbox` or `localnet`) — the network profile. See `internal/profile`.
- `--topology-config` — the path to the disclosure allow-list config. When the file is
  missing, the service uses the built-in defaults. See `internal/disclosure.Config`.
- `--dar` — the path to the compiled test DAR. By default, the service finds `canton/` with a
  walk up to `multi-package.yaml`, then uses `test/.daml/dist/ntt-test-0.1.0.dar`.
- `--dpm-path` — the path to the `dpm` binary. The default search is `PATH`, then
  `~/.dpm/bin`.
- `--access-token-file` — the path to a file that contains a bearer JWT. This is the only
  auth input. The service mints no tokens. The flag is mandatory when the profile requires
  auth.
- `--acs-party` — the reading party for `GET /v1/disclosures`. Without it, the ACS backend
  stays disabled.
- `--verbose` — writes each `dpm script` invocation to stderr.
