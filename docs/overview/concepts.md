# Core concepts

The vocabulary you'll see throughout the console, the API, and this documentation.

## Control plane vs data plane

- The **control plane** is the Go gateway: it owns *state and orchestration*
  (projects, runs, models, agents, connections, audit) and drives downstream
  engines.
- The **data plane** is the set of stores that hold *durable data and events*:
  Postgres, Redis, Kafka, MinIO, and MLflow.

## Resources

These are the first-class objects the control plane manages. Each has list/create
endpoints and appears in the console.

| Resource | What it is |
| --- | --- |
| **Project** | A workspace with a namespace and a starting template (tabular classification, forecasting, RAG/agent, or blank). Everything else belongs to a project. |
| **Pipeline run** | One execution of a pipeline through the engine. Has a status, a progress percentage, an ordered set of **steps** (a DAG), and logs. |
| **Model** | A registered model version with metrics, a **quality gate** status, a **stage** (candidate/production), and — once deployed — a live **endpoint URL** and deployment status. |
| **Agent** | A deployed LangGraph agent: a graph module, an LLM backend, a tool list, a canary weight, and status. Produces sessions and traces. |
| **Agent session** | One conversation/thread with an agent: turns, current node, token usage, and cost. |
| **Tool** | A registered capability an agent can call (name, description, schema). |
| **Feature view** | A named group of features for an entity, with a schema, TTL, and online materialization status. |
| **Connection** | A registered external service (MLflow, S3, Kafka, Langfuse, Prefect, Redis, Kubernetes) that the control plane actively health-checks. Stores only a secret *reference*, never raw credentials. |
| **Audit event** | An immutable record of a mutation — who did what, when. |
| **Function** | A serverless function (OpenFaaS) deployable and invocable when configured. |

## Connections and health

A **connection** registers an external service so the gateway can verify it is
reachable. The check runs **from inside the gateway container**, so connection
endpoints use in-stack hostnames (`http://mlflow:5000/health`). Any HTTP response
below `500` counts as healthy — a `401` from a Kubernetes API server is a healthy
signal. Nothing shows green by wishful thinking.

**Onboarding readiness** (the ring on Overview and the checklist on Platform) is
derived from connections plus having at least one project. See
[Connecting services](../connecting-services.md).

## The quality gate and promotion

A model has a **gate status** (`passed` / `pending`). Promotion moves a model
between **stages** (typically `candidate` → `production`). Deployment starts a live
serving container; **canary weight** controls traffic splitting; **rollback**
removes the serving container and reverts. This is the classical-ML lifecycle made
real — a deployed model answers live predictions from the console's test console.

## Agents, sessions, and tracing

An **agent** is a LangGraph graph served by the shared agent-runtime. When you invoke
it, the runtime maintains a **session** (state checkpointed in Postgres, keyed per
agent) and records **turns**. Every LLM call egresses through the **trace-proxy**,
which captures it to Kafka; **Langfuse** provides the observability UI. Token counts
come from the model's usage metadata (measured, never estimated); **cost** is
computed from configured per-1k rates.

## The feature store: online vs offline

- **Online** features are served from **Redis** through the feature-gateway for
  low-latency reads (real-time scoring).
- **Offline** features are materialized as **Parquet snapshots** in MinIO for batch
  use.

The **feature-materializer** applies feature definitions and populates the online
store, reporting entity counts back to the control plane.

## Real-time processing

The **realtime-processor** is a Kafka consumer service that demonstrates three
patterns: **fraud detection**, **call-center analysis**, and **personalized
recommendations**. Each reads an input topic, enriches the event with online
features, scores it with a model or agent, and publishes to an output topic. The
console's Real-Time panel shows live throughput, latency, and recent scored events.

## Serverless

**Serverless** deployment uses **OpenFaaS / faasd** — single-node, Docker-native,
open source. Agents and model endpoints can be deployed as functions with
scale-to-zero. This is a VM-level install (not part of the default Compose stack)
and is the one capability the demo smoke marks as *skipped* locally by design.

!!! info "Serverless with OpenFaaS"
    Serverless is delivered with OpenFaaS/faasd — fully open source, Docker-native
    on a single node, and carrying to OpenFaaS-on-Kubernetes at scale.

## Trace-proxy egress

All agent LLM calls are pointed at the **trace-proxy** (`OPENAI_BASE_URL` →
`http://trace-proxy:8081/v1`). The proxy forwards to the configured upstream
provider and publishes every call to the `mlaiops.llm.traces` Kafka topic. This
gives complete, centralized capture of LLM usage without touching agent code — and
provider API keys never appear in traces, logs, or the control-plane store.

## Roles (RBAC)

Every API request is authorized against a **role**: `admin`/`operator` (full
control), `engineer` (ML lifecycle, not platform connections), `viewer`
(read-only), and `service` (internal reporting only, via the internal token). See
[RBAC & security](../reference/rbac.md).
