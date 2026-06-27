# Nexus: ml-ai-ops-platform

Nexus is a self-hosted control plane for classical ML, data-centric AI, and agentic AI
workloads. Its architecture follows [`mlops-platform-prd.md`](mlops-platform-prd.md): Go owns
infrastructure services and Python owns the ML-facing SDK and workload primitives.

## Backend implemented

- Durable control-plane API for projects, pipelines, models, agents, tools, connections, and
  immutable audit events
- Versioned Kubernetes CRDs, RBAC, network isolation, and agent reconciliation plans
- Online feature gateway with a Feast-compatible request shape
- S3/MinIO proxy generating bounded AWS SigV4 URLs
- OpenAI-compatible LLM reverse proxy with asynchronous trace emission
- Prometheus component-health collector
- Standard API clients for KFP, MLflow, Langfuse, and Kafka REST Proxy
- Typed Python SDK, pipeline compiler, tool registry, and tracing primitive
- Single-binary CLI for common platform operations
- Container and Kubernetes deployment assets

The web workspace is intentionally lightweight; backend contracts and operational safety are
the current priority.

## Quick start

Requires Go 1.23+ and Python 3.11+.

```bash
make install
make run
```

Open <http://localhost:8080>, or use the SDK:

```python
from mlaiops_sdk import MLAIOpsClient

with MLAIOpsClient(actor="engineer@example.com") as client:
    project = client.create_project("churn", template="tabular-classification")
    run = client.submit_pipeline(project.id)
```

## Build and verify

```bash
make verify
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
          └── Langfuse         (LLM traces and prompts)

  Kubernetes CRDs ──► operator reconciliation ──► workloads and traffic
```

See [Backend architecture](docs/backend.md) for service ports, configuration, integration
contracts, deployment resources, and explicit production gates.

## Important scope boundary

This repository implements the platform-owned integration and control services. It does not
fork or vendor Kafka, MinIO, KFP/Argo, MLflow, Feast, KServe, Redis, PostgreSQL, or Langfuse.
Operators install those upstream systems and connect them through standard APIs.

The architecture exclusion policy from the PRD is enforced in CI.
