# canton/disclosure

This directory holds the two disclosure parts. The `ntt-disclosure` Daml
package defines the disclosure set for each NTT flow. The `service/` Go
module serves the sets' createdEventBlobs over HTTP.

## Daml library

The module header of `daml/Wormhole/Ntt/Disclosure.daml` lists each flow and
its disclosure set. Read that table for the current set contents.

## Service

The service exposes two endpoints:

- `GET /v1/healthz` — a liveness check.
- `GET /v1/disclosures?template=Module:Entity` — the active contracts for
  an allow-listed template, with each contract's createdEventBlob. The
  service reads these contracts from the JSON Ledger API v2.

A template not on the allow-list gets a 403 response. The allow-list uses
package-qualified names, for example `#ntt:Wormhole.Ntt.Manager:NttManager`.
A `template` query value can give just the `Module:Entity` tail; the service
matches it against the allow-list and forwards the qualified name upstream.

The service re-reads `--access-token-file` on each upstream request. An
operator can rotate the token file's contents without a restart.

Build and run the service:

1. Go to the service directory. Run `cd canton/disclosure/service`.
2. Build the binary. Run `go build -o disclosure-service .`.
3. Start the service. Run `./disclosure-service --json-api <url> --party
   <reading-party> [--access-token-file <jwt-file>]`.

## Deployment posture

By design, the service does not authenticate requests. Bind it to loopback. As an
alternative, place it behind the deployment layer's network boundary. This
boundary can be a sidecar, a mesh, or a TLS-terminating proxy.
`--access-token-file` holds its only upstream credential.
