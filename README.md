# Nexus: ml-ai-ops-platform

Nexus is the first runnable vertical slice of the platform described in
[`mlops-platform-prd.md`](mlops-platform-prd.md). It gives novice ML engineers a guided
golden path while keeping the operational boundaries experienced platform engineers need.

## What works now

- A dependency-light Go control-plane gateway
- A responsive workspace for projects, pipeline runs, shared assets, and component health
- Guided project creation with tabular ML, forecasting, RAG/agent, and expert templates
- Pipeline submission contracts with honest queued/simulated state
- A typed Python SDK suitable for notebooks and training jobs
- OpenAPI discovery at `/api/openapi.json`
- Unit and API tests in both Go and Python

The component page deliberately marks MLflow, Feast, KServe, Kafka, MinIO, and Langfuse as
`not configured`. The MVP defines their integration seams; it does not claim those production
systems exist until they are connected.

## Run it

Requirements: Go 1.22+.

```bash
make run
```

Open <http://localhost:8080>. There are no database or JavaScript build dependencies for the
local experience.

## Test it

```bash
python -m pip install -r requirements.txt
make test
make lint
```

## Use the Python SDK

Install the local package:

```bash
python -m pip install -e ./python
python examples/quickstart.py
```

Or use it directly:

```python
from mlaiops_sdk import MLAIOpsClient

with MLAIOpsClient("http://localhost:8080") as client:
    project = client.create_project("churn", template="tabular-classification")
    run = client.submit_pipeline(project.id)
```

## Architecture

```text
Browser workspace ──HTTP/JSON──> Go gateway ──adapter contracts──> Kubernetes / OSS services
                                      ▲
Python SDK / notebooks ───────────────┘
```

The current store is intentionally in memory so the slice remains instantly runnable. The next
production milestone is a Kubernetes repository and reconciliation layer for
`NexusPipelineRun`, followed by OIDC/RBAC and the MLflow/Argo adapters.

## Repository shape

```text
go/                    Go gateway, API contracts, web workspace, tests
python/mlaiops_sdk/    Typed Python client
python/tests/          SDK tests
examples/              Golden-path examples
Dockerfile             Distroless production image
```

The PRD's exclusion policy is enforced by design: this code has no Iguazio, MLRun, Nuclio, or
V3IO dependencies.
