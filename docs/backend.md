# Backend architecture

The repository implements the platform-owned services. Large upstream systems
remain independently deployable dependencies and are reached through their standard APIs.

## Service inventory

| Binary                        | Default port | Responsibility                                            |
| ----------------------------- | -----------: | --------------------------------------------------------- |
| `mlaiops-gateway`           |         8080 | Projects, runs, models, agents, tools, connections, audit |
| `mlaiops-operator`          |         8082 | Deterministic CRD-to-workload reconciliation plans        |
| `mlaiops-feature-gateway`   |         8083 | Feast-compatible online feature retrieval                 |
| `mlaiops-storage-proxy`     |         8084 | Short-lived AWS SigV4 S3 URLs                             |
| `mlaiops-trace-proxy`       |         8081 | OpenAI-compatible reverse proxy and trace emission        |
| `mlaiops-metrics-collector` |         9090 | Prometheus component health + platform metrics  |
| `mlaiops-serving-manager`   |         8085 | Real model serving via mlflow-serve containers (Docker)   |
| `mlaiops`                   |          n/a | Operator and engineer CLI                                 |

Python-owned workload services (each with its own Dockerfile):

| Service | Responsibility |
| --- | --- |
| `agent-runtime` | Serves LangGraph agents over HTTP with Postgres checkpoints and Langfuse tracing |
| `pipeline-runner` | Serves platform flows as Prefect deployments and executes real training runs |
| `realtime-processor` | Kafka consumers for the fraud / call-center / recommendations demos |
| `feature-materializer` | Applies feature definitions and materializes the online store |

All services are built from the root `Dockerfile`:

```bash
docker build --build-arg SERVICE=gateway -t mlaiops/gateway .
docker build --build-arg SERVICE=feature-gateway -t mlaiops/feature-gateway .
```

## Control-plane API

Resource mutations produce durable audit and lifecycle events. Production state is persisted
in PostgreSQL when `DATABASE_URL` is set. Every mutation writes its resource, audit record, and
Kafka outbox entries in one transaction. Local file mode remains available through
`MLAIOPS_DATA_PATH`.

| Resource      | Operations                                                                              |
| ------------- | --------------------------------------------------------------------------------------- |
| Projects      | create, list                                                                            |
| Pipelines     | submit, inspect DAG/logs, cancel, retry, compare parent runs                            |
| Models        | register, evaluate gates, promote, deploy canary, rollback                              |
| Agents        | deploy, list, invoke (runtime proxy), set traffic, sessions, traces, token/cost usage   |
| Feature views | apply (upsert), list, report materialization                                            |
| Storage       | list buckets, list objects by prefix, bounded object preview (proxied to storage-proxy) |
| Prompts       | list Langfuse-managed prompts (reports`configured:false` when Langfuse is absent)     |
| Tools         | register typed schema, list                                                             |
| Connections   | create secret reference, list                                                           |
| Audit         | ordered event list                                                                      |

The component health grid and shared catalog are derived from live state: components turn
healthy only when a checked connection succeeds, and the catalog lists only models, feature
views, agents, and tools that exist.

Set `X-MLAIOps-Actor` on mutation requests for audit attribution. Secrets are represented only
by Kubernetes Secret references; raw credentials are never accepted by the control-plane API.
Connection onboarding performs bounded HTTP health checks and calculates readiness exclusively
from tested infrastructure state.

## Integration contracts

- Kubeflow Pipelines v2: `/apis/v2beta1/runs`
- MLflow: `/api/2.0/mlflow/model-versions/transition-stage`
- Langfuse: `/api/public/ingestion`
- Kafka REST Proxy: `/topics/{topic}`
- S3/MinIO: AWS Signature Version 4 presigned URLs
- Agent serving: OpenAI-compatible LLM calls through the trace proxy

The integration clients fail closed when their base URL is absent or an upstream returns a
non-2xx response.

## Kubernetes resources

CRDs live in `config/crd`, RBAC in `config/rbac`, workload manifests in `config/deploy`, and
default isolation in `config/network`.

The production manifest runs two PostgreSQL-backed gateway replicas. The integration worker
consumes lifecycle topics and creates Nexus CRDs. The leader-elected operator watches those
CRDs, reconciles agent workloads, submits pipeline runs to KFP, transitions MLflow model
stages, and creates KServe `InferenceService` resources.

## Configuration

| Variable                                                           | Component         | Meaning                                                     |
| ------------------------------------------------------------------ | ----------------- | ----------------------------------------------------------- |
| `MLAIOPS_DATA_PATH`                                              | gateway           | Durable control-plane state file                            |
| `DATABASE_URL`                                                   | gateway           | PostgreSQL connection string; enables production repository |
| `OIDC_ISSUER`, `OIDC_AUDIENCE`, `OIDC_JWKS_URL`              | gateway           | JWT verification contract                                   |
| `MLAIOPS_INTERNAL_TOKEN`                                         | feature/storage   | Internal API bearer token                                   |
| `FEAST_URL`                                                      | feature gateway   | Delegate lookups to a running Feast feature server          |
| `REDIS_URL`                                                      | feature gateway   | Direct Redis online store (platform key convention)         |
| `AGENT_RUNTIME_URL`                                              | gateway           | Agent runtime address for`/agents/{id}/invoke`            |
| `STORAGE_PROXY_URL`                                              | gateway           | Storage proxy address for`/api/v1/storage/*`              |
| `LANGFUSE_URL`, `LANGFUSE_PUBLIC_KEY`, `LANGFUSE_SECRET_KEY` | gateway           | Prompt Library proxy                                        |
| `PREFECT_API_URL` | gateway | Real pipeline execution through Prefect |
| `SERVING_MANAGER_URL` | gateway | Real model serving on deploy/rollback |
| `OPENFAAS_URL`, `OPENFAAS_USER`, `OPENFAAS_PASSWORD` | gateway | Serverless function deploy/list/invoke |
| `SERVE_IMAGE`, `PLATFORM_NETWORK` | serving manager | Image and network for model containers |
| `S3_ENDPOINT`                                                    | storage proxy     | MinIO or S3-compatible endpoint                             |
| `S3_REGION`                                                      | storage proxy     | S3 signing region                                           |
| `S3_ACCESS_KEY`, `S3_SECRET_KEY`                               | storage proxy     | Inject from Vault/Kubernetes Secret                         |
| `LLM_UPSTREAM_URL`                                               | trace proxy       | KServe/vLLM or external compatible endpoint                 |
| `TRACE_SINK_URL`, `TRACE_SINK_TOKEN`                           | trace proxy       | Langfuse/event ingestion target                             |
| `MLAIOPS_METRICS_TARGETS`                                        | metrics collector | `name=url` comma-separated health endpoints               |
| `MLAIOPS_URL`                                                    | CLI               | Gateway base URL                                            |

## Local environments

```bash
make local-up          # PostgreSQL, Redis, Kafka, MinIO, MLflow, gateway
make test-integration  # PostgreSQL transaction and outbox verification
make kind-up           # three-node Kind cluster, CRDs and core operators
make test-load         # k6 gateway SLO workload
make test-e2e          # CRD → operator → Deployment/Service Kind test
```

## Remaining environment gates

Before production rollout:

1. Replace the example Dex GitHub organization and hostnames with environment values.
2. Provision Vault roles/policies and referenced Kubernetes Secrets.
3. Install and pin KFP, KServe, Langfuse and the chosen S3 implementation per environment.
4. Run recovery and security tests against the target production cluster and storage classes.
