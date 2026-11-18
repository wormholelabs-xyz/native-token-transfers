# Example edge: Cloudflare

This document gives one example edge for the disclosure service on a
separate gateway host. The operator selects the edge. Read the section
"Deployment on a separate gateway" in `README.md` first.

In this example, a `cloudflared` tunnel connects the gateway host to
Cloudflare. The tunnel uses outbound connections only. Thus the gateway host
needs no inbound port.

```
relayer -> Cloudflare (WAF, rate limits) -> cloudflared tunnel (outbound)
        -> 127.0.0.1:7599 disclosure-service -> participant JSON Ledger API
```

## 1. Start the service on loopback

1. Keep the default `--listen 127.0.0.1:7599`.
2. Start the service with the upstream TLS and auth flags from `README.md`.
3. Run `curl http://127.0.0.1:7599/v1/healthz` on the gateway host.
4. Make sure that the response shows a `ledgerEnd` value.

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
  - service: http_status:404
```

6. Run `cloudflared tunnel ingress validate`.
7. Run `cloudflared tunnel route dns disclosure-gateway canton-disclosure.example.com`.
8. Install `cloudflared` as a system service.
9. Run `cloudflared tunnel info disclosure-gateway`.
10. Make sure that the tunnel shows active connections.

The catch-all rule returns HTTP 404 for each request that the first rule
does not match. Keep the credentials file secret. It authenticates the
tunnel.

## 4. Decide on Cloudflare Access

The service is public by design. Relayers do not log in. Thus Cloudflare
Access is optional. Add Access only to limit the service to known clients,
for example with service tokens.

## 5. Add WAF custom rules

Add these custom rules for the hostname. Set the action to Block.

| Rule | Expression |
|---|---|
| Methods other than GET | `http.host eq "canton-disclosure.example.com" and http.request.method ne "GET"` |
| Paths outside `/v1` | `http.host eq "canton-disclosure.example.com" and not starts_with(http.request.uri.path, "/v1")` |

The service accepts GET only. It serves paths under `/v1` only.

## 6. Add rate limiting rules

Add a rate limiting rule for `/v1/*`:

| Setting | Value |
|---|---|
| Expression | `http.host eq "canton-disclosure.example.com" and starts_with(http.request.uri.path, "/v1/")` |
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
3. Run `curl -X POST https://canton-disclosure.example.com/v1/healthz`.
4. Make sure that Cloudflare blocks the request.
5. Run `curl https://canton-disclosure.example.com/other`.
6. Make sure that Cloudflare blocks the request.
7. From a host outside the operator network, try to connect to port 7599 of
   the gateway host.
8. Make sure that the connection fails.
