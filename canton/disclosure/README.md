# canton/disclosure

This directory holds the `service/` Go module: the production disclosure
service. It serves each NTT flow's createdEventBlobs over HTTP.

The executable specification of the sets lives in the test package:
`canton/test/daml/Wormhole/Ntt/Disclosure.daml`. Its module header lists
each flow and its set, and `Test.TestDisclosure` proves each set sufficient
and minimal against the real NTT choices. The specification runs only in
tests; this service is the production artifact.

## Service

The service exposes three endpoints:

- `GET /v1/healthz` — a liveness check.
- `GET /v1/disclosures?template=Module:Entity` — the active contracts for
  an allow-listed template, with each contract's createdEventBlob. The
  service reads these contracts from the JSON Ledger API v2.
- `GET /v1/flows/{flow}` — one NTT flow's assembled disclosure set, each
  contract labeled by its role. The service selects natively in Go over
  decoded `createArgument` payloads; it runs no Daml interpreter. `flow` and
  its query parameters:

  | Flow | Params |
  |---|---|
  | `release` | `manager`, `digest`, `recipient` (optional) |
  | `mint` | `manager`, `digest`, `recipient` |
  | `set-peer` | `manager`, `digest` |
  | `accept-admin` | `manager`, `digest` |
  | `transfer` | `manager` |
  | `register` | `gg` (optional), `by-vaa` (bool, optional) |
  | `consolidate` | `manager` |

  `manager` and `digest` are hex strings, 64 characters (32 bytes) each.
  The response carries a `disclosures` array (role, templateId, contractId,
  createdEventBlob, synchronizerId) and a `missing` array naming set members
  the disclosing party cannot see (owner-only contracts such as
  `AdminTransferProposal` and `TransferPreapproval`); the client supplies
  those itself. `canton/test/daml/Wormhole/Ntt/Disclosure.daml`'s
  module header is the canonical definition of each flow's set; this
  service mirrors its table.

A template not on the allow-list gets a 403 response. The allow-list uses
package-qualified names, for example `#ntt:Wormhole.Ntt.Manager:NttManager`.
A `template` query value can give just the `Module:Entity` tail; the service
matches it against the allow-list and forwards the qualified name upstream.

The service re-reads `--access-token-file` on each upstream request. An
operator can rotate the token file's contents without a restart.

`--disclosing-party` names the disclosing parties, comma-separated: the
parties as which the service reads the ledger. Every served blob is a
contract at least one of them sees. The access token must grant readAs for
each party, and one participant must host them all. One service with the
full party list replaces one service per party. The disclosing parties are
the source of the disclosures; the submitter that attaches them is a
different party.

Build and run the service:

1. Go to the service directory. Run `cd canton/disclosure/service`.
2. Build the binary. Run `go build -o disclosure-service .`.
3. Start the service. Run `./disclosure-service --json-api <url>
   --disclosing-party <party> [--access-token-file <jwt-file>]`.

## Container

`service/Dockerfile` builds a static binary on a distroless base. Build and
run:

1. Build the image. Run `docker build -t disclosure-service canton/disclosure/service`.
2. Start the container. Run `docker run --rm -p 127.0.0.1:7599:7599
   -v <token-dir>:/secrets:ro disclosure-service --listen 0.0.0.0:7599
   --json-api <url> --disclosing-party <p1,p2,...>
   --access-token-file /secrets/token`.

Inside a container, pass `--listen 0.0.0.0:7599`; the loopback default is
unreachable through a published port. The service re-reads the token file
per request, so an external refresher can rotate the mounted file.

## Deployment posture

By design, the service does not authenticate requests. Bind it to loopback. As an
alternative, place it behind the deployment layer's network boundary. This
boundary can be a sidecar, a mesh, or a TLS-terminating proxy.
`--access-token-file` holds its only upstream credential.
