# What is Nexus

Nexus (the `ml-ai-ops-platform`) is a **self-hosted control plane and console for
the entire AI lifecycle**. It unifies three worlds that are usually run on separate
tooling:

1. **Classical MLOps** — data validation, training pipelines, experiment tracking,
   a model registry, quality-gated promotion, and live model serving.
2. **Data-centric AI** — a feature store (online + offline), artifact storage, and
   real-time stream processing.
3. **Agentic AI** — LangGraph agents with persistent session state, long-term
   memory, tool use, LLM cost accounting, and full tracing.

Everything is delivered as **one Docker Compose stack**. The same stack runs on a
developer laptop and on a single public VM — the public deployment simply adds an
authenticated TLS edge in front of the identical services.

## The problem it solves

Teams typically stitch together a dozen tools (an orchestrator, a registry, a
serving system, a feature store, a tracing backend, an agent framework, a secrets
manager, an object store…) and then spend months on the "platform archaeology" of
making them talk to each other. Nexus wires them together for you:

- A **single API and console** covers projects, pipelines, models, agents, tools,
  features, connections, and audit.
- **Real integrations, not mocks** — a stranger can run `make local-up` and every
  capability actually works end to end.
- **Honest infrastructure** — nothing shows as connected unless it passes an active
  health check.

## Capability map

Nexus delivers a comprehensive set of capabilities using only open-source,
non-proprietary tools:

| Capability | How Nexus delivers it |
| --- | --- |
| **AI orchestration** — the full ML + GenAI lifecycle, automated and managed | Go control plane + Prefect pipelines + agent runtime + model registry/serving, one SDK and console |
| **Real-time processing** — fraud, call-center, recommendations | Kafka stream consumers → online feature lookup → live model/agent → result, with three runnable demos |
| **Deployment flexibility** — build once, deploy cloud / on-prem / edge | Fully containerized, config-driven; storage/DB targets swappable via connections; the same images run laptop → VM → edge |
| **Integrated feature store** — real-time + batch | Online store in Redis via the feature gateway; offline Parquet snapshots in object storage |
| **Open-source serverless** — automate AI app deployment | OpenFaaS / faasd (single-node, Docker-native), carrying to OpenFaaS-on-Kubernetes at scale |

## What Nexus is *not*

- **It is not a managed cloud service.** It is software you host. The default target
  is a single VM via Docker Compose; Kubernetes is a documented scale path.
- **It does not require Kubernetes to be useful.** The three Kubernetes-only
  reference tools (KServe/Knative for serving, KFP/Argo for pipelines, Istio for the
  mesh) are fulfilled Compose-natively (mlflow-serve, Prefect, Caddy). The literal
  Kubernetes stack (Kind + manifests) remains available as an optional fidelity path.
- **It does not ship its own LLM by default.** Agents call external provider APIs
  (OpenAI, Anthropic, or any OpenAI-compatible endpoint) through a trace-capturing
  proxy. A `mock` backend lets the whole stack run with zero API keys.

## Two languages, clear ownership

Nexus follows a deliberate split:

- **Go owns infrastructure** — the gateway/control-plane API, the Kubernetes
  operator, the feature gateway, the storage proxy, the trace proxy, the serving
  manager, the metrics collector, and the CLI. Go's strengths (single static
  binaries, concurrency, strong typing) fit long-running services.
- **Python owns ML-facing work** — the typed SDK, the agent runtime, pipeline
  flows, feature definitions, and the real-time demos. Python is where the ML and
  agent ecosystems live (LangGraph, LangChain, MLflow, scikit-learn, Prefect).

The web console is intentionally **vanilla JavaScript embedded in the Go gateway
binary** — no build toolchain, ideal for the single-binary, single-VM story.

## Where to go next

- **[Architecture](architecture.md)** — the full component diagram and data flows.
- **[Core concepts](concepts.md)** — the vocabulary (projects, runs, connections,
  the control plane, the data plane).
- **[Technology & modules used](technology.md)** — the exact stack, versioned.
- **[Installation](../getting-started/installation.md)** — get it running.
