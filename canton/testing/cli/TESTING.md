# Running the ntt-playground tests

The playground has four test tiers. Three run with no Docker; the fourth
(`e2e-localnet`) boots a real Splice LocalNet. A `justfile` wraps each tier so a
run is one command.

Run everything from this directory (`canton/testing/cli`).

## Prerequisites

| Tier | Needs |
|------|-------|
| unit, daml, e2e-sandbox | `dpm` on `PATH`, Go, `just` |
| e2e-localnet | the above **plus** Docker Desktop (>= 24 GB RAM) and `LOCALNET_DIR` |

- `dpm` (Daml toolchain) — the recipes add `~/.dpm/bin` to `PATH` for you.
- `just` — `brew install just`.
- Docker memory: give Docker **>= 24 GB** (Settings -> Resources -> Memory). Below that,
  the Splice stack's Postgres boots unhealthy and the run fails at network-up.
- `LOCALNET_DIR` — an extracted `splice-node/docker-compose/localnet` from a Splice
  release bundle (e.g. `0.6.12_splice-node.tar.gz`):
  ```sh
  export LOCALNET_DIR=/path/to/splice-node/docker-compose/localnet
  ```

## The recipes

```sh
just                 # list all recipes
just unit            # go vet + unit tests            (no Docker)
just daml            # Daml Script tests              (no Docker)
just e2e-sandbox     # full e2e on the sandbox ledger (no Docker, ~6 min)
just check           # unit + daml + e2e-sandbox      (the non-Docker gate)
just e2e-localnet    # full e2e on real LocalNet      (Docker, ~15 min)
```

`e2e-sandbox` runs the whole suite in-process; the LocalNet-only subtests self-skip.
`e2e-localnet` runs the complete suite including the real Canton Coin/Amulet custody
transfer and the Ledger API v2 stream observer.

## Step by step: the LocalNet run

1. Give Docker >= 24 GB and make sure it is running.
2. Point `LOCALNET_DIR` at your extracted bundle:
   ```sh
   export LOCALNET_DIR=/path/to/splice-node/docker-compose/localnet
   ```
3. From `canton/testing/cli`, run:
   ```sh
   just e2e-localnet
   ```
   This builds the CLI, boots LocalNet (`docker compose up --wait`, ~2 min on a warm
   cache), runs all subtests, and tears LocalNet down at the end. Total ~15 min.
4. Success looks like:
   ```
   --- PASS: TestPlaygroundE2E (…s)
   ok  …/canton/testing/cli/e2e  …s
   ```

Manual LocalNet control, if you want to poke around outside a run:

```sh
just localnet-up       # boot the stack
just localnet-status   # list running services
just localnet-down     # tear it down (also cleans up after an interrupted run)
```

## Troubleshooting

| Symptom | Cause | Fix |
|---------|-------|-----|
| `splice-localnet-postgres is unhealthy`, `docker compose up failed` | Docker VM too small / host RAM thrashing | Give Docker >= 24 GB; free host RAM; retry |
| `UNAVAILABLE: Connection reset` / `CoordinatedShutdown` mid-run | ledger dropped under memory pressure | same as above — it is resources, not the tests |
| sandbox run fails ~3 min in, ports `6864-6869` in use | a stale `ntt-playground` sandbox process from a killed run | `pkill -f ntt-playground`, rerun |
| `nginx ... host not found in upstream "ans-web-ui-app-user"` at boot | cosmetic — the web-UI proxy; the CLI uses the gRPC ledger directly | ignore; tests are unaffected |
| `ERROR: export LOCALNET_DIR` | `LOCALNET_DIR` not set | set it (see prerequisites) |

## Coverage

Combined unit + e2e coverage needs instrumentation because the e2e harness rebuilds
its own CLI binary:

```sh
COV=$(mktemp -d)
go test -c -tags e2e -cover -coverpkg=./... -o /tmp/e2e.cover.test ./e2e
( cd e2e && NTT_PLAYGROUND_PROFILE=sandbox GOFLAGS="-cover -coverpkg=./..." \
    GOCOVERDIR="$COV/sandbox" /tmp/e2e.cover.test -test.run TestPlaygroundE2E )
mkdir -p "$COV/unit" && go test ./... -args -test.gocoverdir="$COV/unit"
go tool covdata percent -i="$COV/unit,$COV/sandbox"
```

Add a `$COV/localnet` run (same as sandbox with the localnet env) to cover the
real-Amulet paths. Do not use `go test -cover` for the e2e runs — it hijacks
`GOCOVERDIR` and the instrumented child's data is lost.
