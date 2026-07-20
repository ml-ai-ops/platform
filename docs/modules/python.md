# Python SDK & workload modules

Python owns the ML-facing surface: a typed SDK, the agent runtime, pipeline flows,
feature definitions, and the real-time demos. The package is `mlaiops-sdk`
(`python/pyproject.toml`), importable as `mlaiops_sdk`, with optional extras for the
heavier agent/runtime dependencies.

```bash
pip install -e ./python                 # core SDK
pip install -e "./python[agents]"       # + LangGraph/LangChain/Langfuse/psycopg
pip install -e "./python[runtime]"      # + FastAPI/Uvicorn (agent runtime)
```

## `mlaiops_sdk` — the client SDK

A small, typed entry point to the control plane. Core imports are light; heavy agent
dependencies load lazily only when used.

### `client.py` — `MLAIOpsClient`

The typed REST client. Construct with a `base_url` and `actor`, use as a context
manager.

```python
from mlaiops_sdk import MLAIOpsClient

with MLAIOpsClient(base_url="http://localhost:8080", actor="you@example.com") as c:
    project = c.create_project("churn", template="tabular-classification")
    run = c.submit_pipeline(project.id)
    for model in c.list_models():
        c.promote_model(model.id, "production")
        c.deploy_model(model.id)
```

| Area | Methods |
| --- | --- |
| Health | `health()`, `readiness()` |
| Projects | `list_projects()`, `create_project()`, `get_project()`, `connect_repository()` |
| Pipelines | run lifecycle plus `list_pipeline_definitions()` and `create_pipeline_definition()` |
| Functions | `list_functions()`, `deploy_function()`, sync/async invoke, and delete |
| Models | `list_models()`, `register_model()`, `promote_model()`, `deploy_model()`, `rollback_model()` |
| Agents | `list_agents()`, `deploy_agent()`, `invoke_agent()`, `set_agent_traffic()`, `agent_sessions()`, `agent_traces()` |
| Tools | `list_tools()`, `register_tool()` |
| Connections | `list_connections()`, `create_connection()`, `test_connection()` |
| Audit | `audit_events()` |

### `models.py` — typed resources

Pydantic models returned by the client: `Project`, `GitRepository`,
`PipelineDefinition`, `PipelineRun`, `Function`, `Model`, `Agent`,
`AgentSession`, `AgentTrace`, `Tool`, `Connection`, `Readiness`, `AuditEvent`.

### `agents.py` — agent memory & checkpoints

- `get_langgraph_checkpointer()` — an `AsyncPostgresSaver` from
  `MLAIOPS_CHECKPOINT_DSN`/`DATABASE_URL` for durable session state.
- `AgentMemoryClient` — pgvector-backed long-term memory (`remember` / `recall`)
  plus `get_entity_features` via the feature gateway. Uses a deterministic
  `HashingEmbedder` so tests need no network.

### `llm.py` — LLM factory

- `build_chat_model(backend, model, base_url, api_key, temperature)` — returns a
  LangChain chat model for `openai` / `anthropic` / `openai-compatible`, pointing at
  the trace proxy by default.
- `MockChatModel` — a deterministic mock backend (supports scripted tool calls) so
  gate tests run offline and free.

### `tracing.py` — Langfuse integration

- `langfuse_handler()` / `LangfuseContext` — LangChain callback + LangGraph config
  wiring for tracing.
- `record_trace(...)` — posts a session summary to the gateway (`/api/v1/traces`),
  with a gateway-native fallback path.

### `tools.py` — tool registry

- `register_tool(...)` — register a callable as a platform tool.
- `to_langchain_tool()` / `langchain_tools(names)` — convert registered tools to
  LangChain `StructuredTool`s for agents.

### `pipelines.py` — pipeline compiler

Defines container `PipelineStep` and reusable `FunctionStep` jobs. `definition()`
compiles either form to the validated control-plane contract and infers function or
Prefect execution mode.

## `agent_runtime` — the agent service

The FastAPI service that serves LangGraph agents (see
[AI services](../services/ai-observability.md#agent-runtime)).

| Module | Role |
| --- | --- |
| `app.py` | FastAPI app: `POST /invoke`, `POST /stream` (SSE), `GET /healthz`; wires checkpointer + memory + tracing; prefers per-request `X-MLAIOps-Agent-*` headers |
| `graphs.py` | `build_graph(spec, model, checkpointer)` — handles compiled graphs, `StateGraph`, or factory callables |
| `reporting.py` | `Usage` (measured tokens → cost), `usage_from_messages`, `report_session` back to the gateway |
| `__main__.py` | Entrypoint |

## `agents` — demo agent

`agents.customer_support.graph:build` — a real LangGraph `StateGraph`
(reason → tools → respond) with registered tools `feature_store_lookup` (feature
gateway, with a demo fallback) and `kb_search`. This is the agent the stack deploys
by default.

## `pipelines` — flow definitions

| Module | Role |
| --- | --- |
| `training.py` | The `training_pipeline` flow: `validate → train → evaluate → register`; deterministic scikit-learn training; logs to MLflow; registers with the control plane |
| `report.py` | `report_step` / `reported_step` — reports step transitions to the gateway |
| `serve.py` | Serves the flows as Prefect deployments (the pipeline-runner entrypoint) |

## `features` — feature store

| Module | Role |
| --- | --- |
| `definitions.py` | Feature views (`customer_profile`, `transaction_stats_5m`) with entity schemas and deterministic source rows |
| `materialize.py` | `Materializer` — applies definitions, writes the online store, snapshots Parquet to MinIO, reports counts |

## `realtime` — real-time demos

| Module | Role |
| --- | --- |
| `service.py` | The consumer loop: poll Kafka → enrich → score → produce → report stats (resilient to REST timeouts) |
| `demos.py` | `score_fraud`, `analyze_transcript`, `recommend`, `get_features` |
| `kafka.py` | `KafkaRest` producer + `Consumer` (via the REST proxy, httpx only) |
| `produce.py` | CLI to produce demo events (`python -m realtime.produce --demo fraud --count 5`) |

## Tests

`python/tests/` plus per-module tests. The CI `python` job runs `ruff check` and
`pytest -q`. Gate tests use the mock LLM and deterministic embedder — fast, free,
no network. The paid eval lane (LLM-as-judge) runs only when `MLAIOPS_RUN_EVALS=1`
with a real provider key.
