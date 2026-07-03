# Go packages

The Go module is `github.com/ml-ai-ops/platform` (Go 1.23). Its only third-party
dependency is `github.com/jackc/pgx/v5` (Postgres). Commands live under `go/cmd/`,
reusable logic under `go/internal/`, and public API types under `go/pkg/`.

## Commands (`go/cmd`)

Each builds to a `mlaiops-<name>` binary (`make build`) and to a container image via
the root `Dockerfile` (`--build-arg SERVICE=<name>`).

| Command | Binary | Role |
| --- | --- | --- |
| `gateway` | `mlaiops-gateway` | REST API + embedded console |
| `operator` | `mlaiops-operator` | Kubernetes reconcilers (scale path) |
| `integration-worker` | `mlaiops-integration-worker` | Kafka → CRD lifecycle worker (scale path) |
| `trace-proxy` | `mlaiops-trace-proxy` | LLM egress + trace capture |
| `feature-gateway` | `mlaiops-feature-gateway` | Online feature retrieval |
| `storage-proxy` | `mlaiops-storage-proxy` | SigV4 S3 URLs + browse |
| `metrics-collector` | `mlaiops-metrics-collector` | Prometheus platform metrics |
| `serving-manager` | `mlaiops-serving-manager` | Model-serving container control |
| `cli` | `mlaiops` | Operator/engineer CLI |

## Internal packages (`go/internal`)

### `auth`

Authentication and authorization. OIDC/JWKS verification (`Verifier`), the RBAC
model (`Allowed`, `Permissions`, the role constants), the auth middleware, and the
`RBAC` middleware that runs on every request. Roles: `admin`, `operator`,
`engineer`, `viewer`, `service`. See [RBAC & security](../reference/rbac.md).

### `httpapi`

The gateway's HTTP layer. `server.go` defines the router (stdlib `net/http`
ServeMux), all handlers, and the JSON helpers (`writeJSON`, `writeMutation`,
`decode`). `openapi.go` serves the OpenAPI document. This is where every
`/api/v1/*` endpoint is wired.

### `store`

Persistence. `repository.go` is the interface contract; `store.go` is the local
file implementation; `postgres.go` is the PostgreSQL implementation with the
transactional **outbox** (`outbox.go`) — every mutation writes resource + audit +
outbox atomically. Handles run-step transitions, per-agent session scoping, model
endpoints, and feature views.

### `integrations`

HTTP clients for external systems: MLflow, KFP, Langfuse, Prefect (`client.go`),
the Kafka REST producer/consumer (`kafka_consumer.go`), and OpenFaaS. `dispatcher.go`
routes durable commands. `NewPrefect` normalizes the API base URL so paths don't
double-prefix `/api`.

### `serving`

Model serving over the Docker Engine API (`manager.go`). `Manager.Deploy` does a
replace → create → start with labels and restart policy; supports an `APIVersion`
override for older daemons. Backs the serving-manager command.

### `storage`

Object-store access. `presign.go` builds AWS SigV4 URLs (with a verified,
signature-correct path); `browse.go` lists buckets/objects and previews objects.
Backs the storage-proxy command.

### `feature`

Online feature retrieval. `redis.go` (online store), `feast.go` (Feast-compatible
request shape), `service.go` (HTTP handlers). Backs the feature-gateway command.

### `traceproxy`

The OpenAI-compatible reverse proxy (`proxy.go`) that forwards LLM calls upstream
and asynchronously emits each call to the Kafka traces topic. Backs the trace-proxy
command.

### `metrics`

`collector.go` (component health scraping) and `platform.go` (`PlatformCollector`,
the PRD §8.2.1 metric set: pipeline outcomes, agent usage, real-time stats).
Prometheus exposition. Backs the metrics-collector command.

### `operator`

Kubernetes controllers for the scale path: `agent_controller.go`,
`lifecycle_controllers.go`, and deterministic reconciliation plans (`reconciler.go`)
for the Nexus CRDs.

### `platform`

Live derivation of the component-health grid (`Components`) and the shared
**catalog** (`Catalog`) from real connections, models, features, agents, and tools —
no hardcoded data.

## Public API types (`go/pkg`)

| Package | Contents |
| --- | --- |
| `pkg/api` | Request/response types shared by the gateway, CLI, and clients (`Project`, `PipelineRun`, `Model`, `Agent`, `Connection`, request bodies, `Page[T]`, `APIError`) |
| `pkg/kube/v1alpha1` | Typed Kubernetes CRD definitions (scale path) |

## Testing

Every package has table-driven `_test.go` files. The CI `go` job runs `gofmt -l`
(must be empty), `go vet`, `go test -race`, the tagged integration suite against a
real Postgres, `go build ./cmd/...`, and the banned-tech scan. Run locally with
`make test-go` and `make test-integration`.
