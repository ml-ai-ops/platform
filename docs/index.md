# Nexus — the ml-ai-ops-platform

Nexus is a **self-hosted platform that brings the whole AI lifecycle together** —
classical ML, data-centric AI, and agentic AI — behind one control plane and one
web console. It runs as a single Docker Compose stack that is **identical on your
laptop and on a public VM**: no service is swapped between local development and
production.

<div class="grid cards" markdown>

- :material-rocket-launch: **[Get started](getting-started/installation.md)**

    One command (`make local-up`) brings up the full stack. Then walk the
    [quickstart](getting-started/quickstart.md).

- :material-sitemap: **[Architecture](overview/architecture.md)**

    How the Go control plane, Python workloads, and the data plane fit together.

- :material-server: **[Services](services/index.md)**

    Every container in the stack, its port, its job, and how it connects.

- :material-package-variant: **[Modules](modules/go-packages.md)**

    The Go packages and Python SDK/workloads that make it all work.

- :material-api: **[REST API](reference/api.md)**

    The full control-plane surface — every endpoint, by resource.

- :material-shield-key: **[RBAC & security](reference/rbac.md)**

    Roles, permissions, tokens, and how authorization is enforced everywhere.

</div>

## What you can do with it

Run `make local-up`, open the landing page at <http://localhost:8080>, then enter
the console at <http://localhost:8080/console.html>. The entire lifecycle works:

- **Create a project** from a template and get a clean workspace.
- **Submit a pipeline** that *really executes* — multi-step containers run through
  a real engine (Prefect) and report every step live as a DAG.
- **Register and promote a model**, deploy it to a **live inference endpoint**, and
  hit it from the built-in test console.
- **Deploy a LangGraph agent** that answers via a real LLM, keeps session state in
  Postgres, retrieves features and long-term memory, and emits full traces.
- **Browse features, artifacts, prompts, and live cost** — all real data.
- **Score events in real time** — fraud detection, call-center analysis, and
  personalized recommendations demos, streaming through Kafka.
- **Develop interactively** in a built-in [Jupyter workbench](guides/workbench.md)
  with a browser terminal, wired into every service.
- **Put the same stack on a public VM** behind TLS + OIDC with one script.

## Design principles

| Principle | What it means in practice |
| --- | --- |
| **Local == deployed** | The public stack is the local stack plus an edge (Caddy + Dex). No image or service is swapped. |
| **Functional, not simulated** | Every tool is really wired: pipelines execute, models serve live, agents call real LLMs, features come from Redis. |
| **Compose-native** | Kubernetes-only tools (KServe, KFP, Istio) are fulfilled by Docker-friendly equivalents (mlflow-serve, Prefect, Caddy). Kubernetes stays a documented scale path. |
| **Honest state** | Nothing is green by wishful thinking — every connection is actively health-checked; costs and tokens are measured, never estimated. |
| **Secure by default in public** | OIDC on every route, RBAC on every request, secrets in `.env`, LLM keys never written to traces or logs. |

## How this documentation is organized

- **[Overview](overview/what-is-nexus.md)** — what Nexus is, its architecture, the
  concepts, and the exact technology used.
- **[Getting started](getting-started/installation.md)** — install, quickstart, and
  the complete configuration reference.
- **[Services](services/index.md)** — a reference for every container in the stack.
- **[Modules](modules/go-packages.md)** — the Go packages and Python modules.
- **[Guides](connecting-services.md)** — task-focused walkthroughs (connecting
  services, the workbench, pipelines, serving, agents, features, real-time).
- **[Reference](reference/api.md)** — the REST API, RBAC, and CLI.
- **[Operations](operations/local.md)** — running locally, public hosting, and
  troubleshooting.

!!! note "Local credentials are development defaults"
    Everything in this documentation that shows a password, key, or token
    (`mlaiops-local`, `pk-lf-local-dev`, etc.) is a **development default**.
    Override every one of them for any public deployment — see
    [Public hosting](hosting.md).
