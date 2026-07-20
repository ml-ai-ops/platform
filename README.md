# Nexus: ml-ai-ops-platform

Nexus is a self-hosted control plane for classical ML, data-centric AI, and agentic AI
workloads. Go owns
infrastructure services and Python owns the ML-facing SDK and workload primitives.

## Backend implemented

- Durable control-plane API for Git-native projects, reusable pipelines, functions, models, agents, tools, connections, and
  immutable audit events
- Versioned Kubernetes CRDs, RBAC, network isolation, and reconciled per-user Jupyter/IDE workspaces
- Online feature gateway with a Feast-compatible request shape
- S3/MinIO proxy generating bounded AWS SigV4 URLs
- OpenAI-compatible LLM reverse proxy with asynchronous trace emission
- Prometheus component-health collector
- Standard API clients for KFP, MLflow, Langfuse, and Kafka REST Proxy
- PostgreSQL repositories with transactional Kafka outbox delivery
- OIDC/JWKS authentication with tenant and role enforcement
- Leader-elected Kubernetes controllers for agents, pipelines, models and KServe deployment
- Kafka lifecycle worker translating durable commands into Nexus CRDs
- Guided onboarding with active infrastructure checks and readiness scoring
- Container and function DAGs, Dagre visualization, validation, parallel ready jobs, retries, parameters, and Git lineage
- OpenFaaS-backed OCI functions with HTTP, async, Cron, and Kafka trigger configuration
- Admin resource profiles and deny-by-default service/project/storage/function quotas
- Quality-gated model promotion, canary deployment and rollback
- Agent sessions, traces, tools, token usage and cost aggregation
- Dex, Vault, CloudNativePG backup and scoped NetworkPolicy assets
- Typed Python SDK, pipeline compiler, tool registry, and tracing primitive
- Single-binary CLI for common platform operations
- Container and Kubernetes deployment assets

The embedded console exposes these backend contracts directly: administrators provision
users and resources; engineers connect Git, deploy functions, compose flows, run workloads,
and inspect live metadata without bypassing the same authorization boundaries.

## Quick start

Requires Go 1.23+ and Python 3.11+.

```bash
make install
make run
```

Open the landing page at <http://localhost:8080> and the console at
<http://localhost:8080/console.html>, or use the SDK:

```python
from mlaiops_sdk import MLAIOpsClient

with MLAIOpsClient(actor="engineer@example.com") as client:
    project = client.create_project("churn", template="tabular-classification")
    run = client.submit_pipeline(project.id)
```

## Build and verify

```bash
make verify
make test-integration
```

Builds produced in `bin/`:

```text
mlaiops-gateway
mlaiops-operator
mlaiops-trace-proxy
mlaiops-feature-gateway
mlaiops-storage-proxy
mlaiops-metrics-collector
mlaiops-cli
```

## Architecture

```text
Python SDK / CLI / UI
          │ HTTP + JSON
          ▼
  Go control-plane gateway ───────────────► durable audit/state
          │
          ├── KFP / Argo       (pipelines)
          ├── MLflow           (experiments and registry)
          ├── KServe           (model and LLM serving)
          ├── Feast / Redis    (features)
          ├── MinIO / S3       (artifacts)
          ├── Kafka            (events)
          ├── OpenFaaS         (functions and event-driven microservices)
          └── Langfuse         (LLM traces and prompts)

  Access grant ──► NexusWorkspace ──► bounded Jupyter + IDE + persistent disk
  Lifecycle CRDs ──► operator reconciliation ──► workloads and traffic
```

## Documentation

Full documentation lives in [`docs/`](docs/index.md) and builds into a browsable site
with MkDocs Material — architecture, every service and module, installation,
configuration reference, the REST API, RBAC, and operations:

```bash
make docs-install     # pip install mkdocs-material
make docs-serve       # live preview at http://localhost:8000
make docs-build       # strict static build into site/
```

Start at [`docs/index.md`](docs/index.md).

## Important scope boundary

This repository implements the platform-owned integration and control services. It does not
fork or vendor Kafka, MinIO, KFP/Argo, MLflow, Feast, KServe, Redis, PostgreSQL, or Langfuse.
Operators install those upstream systems and connect them through standard APIs.

The architecture exclusion policy is enforced in CI.
