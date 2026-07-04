# Installation

Nexus runs as one Docker Compose stack. The fastest path to a working platform is
`make local-up`.

## Prerequisites

| Requirement | Why | Notes |
| --- | --- | --- |
| **Docker Engine + Compose v2** | Runs the whole stack | Docker Desktop on macOS/Windows, Docker Engine on Linux |
| **~6–8 GB RAM free for Docker** | ~17 services | Raise Docker Desktop's memory if services get OOM-killed |
| **Make** | Convenience targets | Optional — you can call `docker compose` directly |
| **Go 1.23+** | Only for building Go binaries outside Docker or running Go tests | Not needed just to run the stack |
| **Python 3.11+** | Only for the SDK, tests, or docs outside Docker | Not needed just to run the stack |
| **Node.js** | Only for `make verify` (JS syntax check) | Not needed to run the stack |

!!! tip "macOS + iCloud"
    Don't keep the repository under an iCloud-managed folder (`~/Documents`,
    `~/Desktop`). iCloud can evict files and stall `git`. Clone to a plain path like
    `~/dev/mlops`.

## Fastest path: the full stack

```bash
git clone https://github.com/ml-ai-ops/platform.git
cd platform
make local-up
```

`make local-up`:

1. Builds and starts every service defined in `deploy/compose.yaml`.
2. Creates the Kafka topics (`scripts/local-topics.sh`).

The first run builds several images (Go services, MLflow, agent runtime, pipeline
runner, Jupyter workbench), so it takes a few minutes. Subsequent runs are fast.

When it finishes, open the landing page:

<http://localhost:8080>

The operational console is at <http://localhost:8080/console.html>.

### Verify it works

```bash
./scripts/demo-smoke.sh
```

This exercises the whole platform end to end — creates a project, runs a real
pipeline, deploys a model and gets a live prediction, invokes an agent, reads
features from Redis, browses storage, and scores fraud events. Expect:

```
RESULT: 18 passed, 0 failed, 1 skipped
```

The one skip is OpenFaaS (a VM-level install, by design).

### Service URLs

| Service | URL | Credentials (local defaults) |
| --- | --- | --- |
| Landing page | <http://localhost:8080> | none |
| Console + API | <http://localhost:8080/console.html> | none (RBAC role from `MLAIOPS_LOCAL_ROLE`, default admin) |
| Jupyter workbench | <http://localhost:8888> | token `mlaiops-local` |
| MLflow | <http://localhost:15000> | none |
| Prefect | <http://localhost:4200> | none |
| Langfuse | <http://localhost:3000> | `admin@local.dev` / `mlaiops-local-admin` |
| MinIO console | <http://localhost:9001> | `mlaiops` / `mlaiops-local-secret` |

See [Configuration reference](configuration.md) for every port and setting, and
[Connecting all services](../connecting-services.md) for the full connection map.

## Stopping and restarting

```bash
make local-down     # stop the stack (volumes persist)
make local-up       # bring it back
```

Durable state lives in named volumes (`postgres-data`, `minio-data`, `redis-data`,
`prefect-data`, `jupyter-data`), so your projects, models, and notebooks survive
restarts.

## Alternative: build and run the Go control plane only

For control-plane development without the full stack:

```bash
make install        # pip installs the SDK + dev deps
make run            # runs the gateway on :8080 (file-backed store)
```

In this mode there is no execution engine, serving, or agent runtime — the gateway
uses a local JSON file (`MLAIOPS_DATA_PATH`, default `data/platform.json`) and
downstream integrations are simply not configured. Good for API and console work.

## Building the binaries

```bash
make build
```

Produces static binaries in `bin/`:

```
mlaiops-gateway  mlaiops-operator  mlaiops-integration-worker
mlaiops-trace-proxy  mlaiops-feature-gateway  mlaiops-storage-proxy
mlaiops-metrics-collector  mlaiops-serving-manager  mlaiops-cli
```

All services are also built from the root `Dockerfile` with a `SERVICE` build arg:

```bash
docker build --build-arg SERVICE=gateway -t mlaiops/gateway .
```

## Running the test suites

```bash
make verify           # go test + vet + gofmt, ruff, pytest, build, JS check, banned-tech scan
make test-integration # Postgres outbox + pgvector round-trip
make test-e2e         # Kind-based end-to-end (optional scale path)
make test-load        # k6 load test against the gateway
```

## Public / production install

To host on a public VM behind TLS + OIDC, see **[Public hosting](../hosting.md)**.
The short version:

```bash
cp .env.example .env    # fill in domain, OIDC, secrets
make public-up
```

## Documentation site (this site)

```bash
make docs-install       # pip install mkdocs-material
make docs-serve         # live preview at http://localhost:8000
make docs-build         # static build into site/ (strict)
```
