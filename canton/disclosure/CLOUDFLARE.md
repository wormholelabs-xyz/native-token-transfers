# Example edge: Cloudflare

This document gives one example edge for the disclosure service on a
separate gateway host. The operator selects the edge. Read the section
"Deployment on a separate gateway" in `README.md` first.

In this example, a `cloudflared` tunnel connects the gateway host to
Cloudflare. The tunnel uses outbound connections only. Thus the gateway host
needs no inbound port.

Cloudflare is the edge proxy. It ends TLS, applies the WAF rules, and
applies the rate limits. The flow routes and the CIP-0056 registry routes
stay public. Relayers and wallets do not authenticate.

```
relayer -> Cloudflare (WAF, rate limits) -> cloudflared tunnel (outbound)
        -> 127.0.0.1:7599 disclosure-service -> participant JSON Ledger API
```

## 1. Start the service on loopback

1. Keep the default `--listen 127.0.0.1:7599`.
2. Do not set `--behind-edge-proxy`. `cloudflared` connects to the loopback
   address.
3. Start the service with the upstream TLS and auth flags from `README.md`.
4. Run `curl http://127.0.0.1:7599/v1/readyz` on the gateway host.
5. Make sure that the response is `{"status":"ready"}`.

## 2. Close the inbound ports

1. Block all inbound traffic to the gateway host in its firewall.
2. Allow outbound traffic to Cloudflare for the tunnel.
3. Allow outbound traffic to the participant JSON Ledger API.
4. Allow outbound traffic to the OAuth2 token endpoint, if you use one.

## 3. Create the tunnel

1. Install `cloudflared` on the gateway host.
2. Run `cloudflared tunnel login`.
3. Run `cloudflared tunnel create disclosure-gateway`.
4. Write the tunnel ID that the command shows.
5. Write `/etc/cloudflared/config.yml`:

```yaml
tunnel: <TUNNEL_ID>
credentials-file: /etc/cloudflared/<TUNNEL_ID>.json

ingress:
  - hostname: canton-disclosure.example.com
    path: ^/v1(/.*)?$
    service: http://127.0.0.1:7599
  - hostname: canton-disclosure.example.com
    path: ^/registry/.*$
    service: http://127.0.0.1:7599
  - service: http_status:404
```

6. Run `cloudflared tunnel ingress validate`.
7. Run `cloudflared tunnel route dns disclosure-gateway canton-disclosure.example.com`.
8. Install `cloudflared` as a system service.
9. Run `cloudflared tunnel info disclosure-gateway`.
10. Make sure that the tunnel shows active connections.

The catch-all rule returns HTTP 404 for each request that the first two rules
do not match. Remove the `/registry/` rule when the service runs without
`--registry-instruments-file`. Keep the credentials file secret. It authenticates the
tunnel.

## 4. Keep the flow and registry routes public

The flow routes are public, as the Splice Scan routes are. Relayers do not
authenticate. CIP-0056 requires registry routes without authentication. Do
not put Cloudflare Access in front of `/v1` or `/registry/`. TLS, the WAF
rules, and the rate limits are the edge controls.

## 5. Add WAF custom rules

Add these custom rules for the hostname. Set the action to Block. Each rule
has a name and an expression.

```
# Methods other than GET on /v1
http.host eq "canton-disclosure.example.com" && starts_with(http.request.uri.path, "/v1") && http.request.method ne "GET"

# Methods other than GET and POST on /registry/
http.host eq "canton-disclosure.example.com" && starts_with(http.request.uri.path, "/registry/") && !(http.request.method in {"GET" "POST"})

# Paths outside /v1 and /registry/
http.host eq "canton-disclosure.example.com" && !starts_with(http.request.uri.path, "/v1") && !starts_with(http.request.uri.path, "/registry/")

# Long query strings
http.host eq "canton-disclosure.example.com" && len(http.request.uri.query) gt 1024

# Large request bodies
http.host eq "canton-disclosure.example.com" && http.request.body.size gt 65536
```

The service accepts GET on `/v1`. It accepts GET on the registry metadata
routes and POST on the other registry routes. It returns 404 for a registry
route with the other method. It serves paths under `/v1` and
`/registry/` only. It rejects a query string that is longer than 1024 bytes.
It rejects a registry request body that is larger than 64 KiB.

## 6. Add rate limiting rules

Add a rate limiting rule for `/v1/*` and `/registry/*`:

| Setting | Value |
|---|---|
| Expression | `http.host eq "canton-disclosure.example.com" and (starts_with(http.request.uri.path, "/v1/") or starts_with(http.request.uri.path, "/registry/"))` |
| Characteristics | IP |
| Requests | Select a value from the expected relayer volume |
| Period | 10 seconds on the Free plan; longer periods on paid plans |
| Action | Block |
| Duration | 10 seconds on the Free plan; longer durations on paid plans |

Relayers are programs. A program cannot complete a challenge. Thus use the
Block action, not a challenge action.

The edge limits do not replace the limits of the service. The flags
`--max-upstream-concurrency` and `--cache-ttl` limit the load on the
participant.

## 7. Check the setup

1. Run `curl https://canton-disclosure.example.com/v1`.
2. Make sure that the response is the endpoint list.
3. Run `curl https://canton-disclosure.example.com/v1/readyz`.
4. Make sure that the response is `{"status":"ready"}`.
5. Run `curl -X POST https://canton-disclosure.example.com/v1/healthz`.
6. Make sure that Cloudflare blocks the request.
7. Run `curl https://canton-disclosure.example.com/other`.
8. Make sure that Cloudflare blocks the request.
9. Run `curl https://canton-disclosure.example.com/registry/metadata/v1/info`.
10. Make sure that `adminId` is the `gg` party.
11. From a host outside the operator network, try to connect to port 7599 of
    the gateway host.
12. Make sure that the connection fails.
