# Gateway & control-plane services

## gateway

The heart of Nexus. A single Go binary that serves the **REST API** and the
**embedded web console**, and orchestrates every downstream engine.

| | |
| --- | --- |
| **Image** | Built from root `Dockerfile` with `SERVICE=gateway` |
| **Host port** | `8080` (`GATEWAY_PORT`) |
| **Source** | `go/cmd/gateway`, `go/internal/httpapi` |
| **Health** | `GET /api/v1/health` |
| **Store** | PostgreSQL (`DATABASE_URL`) or local JSON file (`MLAIOPS_DATA_PATH`) |

### Responsibilities

- **Owns state** for projects, pipeline runs, models, agents, tools, connections,
  feature views, and immutable audit events.
- **Transactional writes** — every mutation persists its resource, an audit record,
  and Kafka outbox entries in one transaction (Postgres mode).
- **Orchestration & proxying** — drives Prefect (pipelines), the serving manager
  (endpoints), the agent runtime (agent turns), the storage proxy (browse), the
  feature gateway (lookups), and Langfuse (prompts).
- **Fail-closed** — if a downstream engine rejects a request, the control plane
  reflects it honestly (e.g. a run is marked failed, a deploy returns 502).
- **Authorization** — the RBAC middleware runs on every request in every mode.
- **Live updates** — `GET /api/v1/events` streams a state digest over SSE.
- **Serves the console** — the vanilla-JS UI is embedded via `go:embed`.

### The embedded console

`go/cmd/gateway/web/` holds `index.html`, `app.js`, and `styles.css`, embedded into
the binary. It is dependency-free (no React, no build step). It renders 10 views
(Overview, Projects, Pipelines, Models, Agents, Features, Storage, Real-Time,
Catalog, Platform), a role-aware menu bar, a DAG visualizer, a metric chart, chat
and prediction consoles, a storage browser, and a prompt library — all driven by
the same API documented under [Reference → REST API](../reference/api.md).

### Key environment

See the [gateway section of the configuration reference](../getting-started/configuration.md#gateway-control-plane).
The essentials: `DATABASE_URL`, `PREFECT_API_URL`, `SERVING_MANAGER_URL`,
`AGENT_RUNTIME_URL`, `STORAGE_PROXY_URL`, `LANGFUSE_*`, `MLAIOPS_LOCAL_ROLE`,
`MLAIOPS_INTERNAL_TOKEN`, and (public) `OIDC_*`, `MLAIOPS_ALLOWED_ORIGIN`.

## operator

Kubernetes controller for the **scale path** — leader-elected reconcilers that turn
Nexus CRDs (`NexusAgent`, `NexusPipelineRun`, `NexusTool`, `NexusConnection`,
`NexusModelPromotion`) into workloads. Ships as `mlaiops-operator`. Not part of the
default Compose stack; used only with the optional Kubernetes deployment.

| | |
| --- | --- |
| **Source** | `go/cmd/operator`, `go/internal/operator` |
| **CRDs** | `config/crd/` |
| **RBAC** | `config/rbac/operator.yaml` |

## metrics-collector

Prometheus exposition of platform metrics: pipeline duration, inference
latency/requests, feature-lookup latency, LLM tokens/cost, agent quality, and active
sessions. Ships as `mlaiops-metrics-collector`, default port `9090`.

| | |
| --- | --- |
| **Source** | `go/cmd/metrics-collector`, `go/internal/metrics` |
| **Config** | `MLAIOPS_METRICS_TARGETS` (gateway/services to scrape) |

## integration-worker

Kafka lifecycle worker that translates durable outbox commands into Nexus CRDs on
the Kubernetes path. Ships as `mlaiops-integration-worker`. Part of the scale-path
deployment (`config/deploy/integration-worker.yaml`).

## cli

Single-binary operator/engineer CLI (`mlaiops`). See [CLI reference](../reference/cli.md).
