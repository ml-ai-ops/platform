# REST API reference

The gateway exposes a JSON REST API under `/api/v1`. A searchable human-facing
reference is served at `GET /api-docs.html`; the machine-readable OpenAPI
document remains available at `GET /api/openapi.json`. Every request is authorized by
[RBAC](rbac.md).

- **Base URL (local):** `http://localhost:8080`
- **Content type:** `application/json`
- **Auth:** none locally (role from `MLAIOPS_LOCAL_ROLE`); `Authorization: Bearer
  <token>` in public (OIDC) mode, or the internal token for services.
- **Errors:** non-2xx responses return `{"error": "<code>", "message": "<detail>"}`.
- **Lists:** most list endpoints return `{"items": [...], "total": N}`.

## Identity & health

| Method | Path | Role | Purpose |
| --- | --- | --- | --- |
| `GET` | `/api/v1/health` | public | Liveness (`{"status":"ok",...}`) |
| `GET` | `/api/v1/me` | any | Caller identity, roles, and effective permissions |
| `GET` | `/api/v1/dashboard` | viewer+ | Workspace summary (counts + recent runs) |
| `GET` | `/api/v1/onboarding/readiness` | viewer+ | Onboarding readiness score |
| `GET` | `/api/openapi.json` | public | OpenAPI document |

## Projects

| Method | Path | Role | Purpose |
| --- | --- | --- | --- |
| `GET` | `/api/v1/projects` | viewer+ | List projects |
| `POST` | `/api/v1/projects` | engineer+ | Create a project (`name`, `description`, `template`) |

## Pipelines

| Method | Path | Role | Purpose |
| --- | --- | --- | --- |
| `GET` | `/api/v1/pipelines/runs` | viewer+ | List runs |
| `POST` | `/api/v1/pipelines/submit` | engineer+ | Submit a run (`project_id`, `name`) |
| `GET` | `/api/v1/pipelines/runs/{id}` | viewer+ | Run detail: steps (DAG) + logs |
| `POST` | `/api/v1/pipelines/runs/{id}/cancel` | engineer+ | Cancel (propagates to the engine) |
| `POST` | `/api/v1/pipelines/runs/{id}/retry` | engineer+ | Retry |
| `POST` | `/api/v1/pipelines/runs/{id}/steps` | service | Step transition (from the executing flow) |

## Models

| Method | Path | Role | Purpose |
| --- | --- | --- | --- |
| `GET` | `/api/v1/models` | viewer+ | List models |
| `POST` | `/api/v1/models` | engineer+ | Register (`project_id`, `name`, `version`, `artifact_uri`, `metrics`) |
| `POST` | `/api/v1/models/{id}/promote` | engineer+ | Promote (`stage`) |
| `POST` | `/api/v1/models/{id}/deploy` | engineer+ | Deploy → live endpoint (`canary_weight`) |
| `POST` | `/api/v1/models/{id}/rollback` | engineer+ | Undeploy / roll back |
| `POST` | `/api/v1/models/{id}/predict` | engineer+ | Proxy a prediction to the live endpoint |

## Agents

| Method | Path | Role | Purpose |
| --- | --- | --- | --- |
| `GET` | `/api/v1/agents` | viewer+ | List agents |
| `POST` | `/api/v1/agents` | engineer+ | Deploy an agent |
| `PUT` | `/api/v1/agents/{id}/traffic` | engineer+ | Set canary weight |
| `POST` | `/api/v1/agents/{id}/invoke` | engineer+ | Run one turn (proxied to the runtime) |
| `GET` | `/api/v1/agents/{id}/sessions` | viewer+ | List sessions |
| `GET` | `/api/v1/agents/{id}/traces` | viewer+ | List traces |
| `GET` | `/api/v1/agents/{id}/usage` | viewer+ | Aggregated tokens/cost/active sessions |
| `POST` | `/api/v1/traces` | service | Record a session/trace (from the runtime) |

## Tools

| Method | Path | Role | Purpose |
| --- | --- | --- | --- |
| `GET` | `/api/v1/tools` | viewer+ | List tools |
| `POST` | `/api/v1/tools` | engineer+ | Register a tool |

## Features

| Method | Path | Role | Purpose |
| --- | --- | --- | --- |
| `GET` | `/api/v1/features` | viewer+ | List feature views |
| `POST` | `/api/v1/features` | engineer+ / service | Apply a feature view |
| `POST` | `/api/v1/features/{name}/materialized` | service | Report a materialization (entity count) |

## Storage

| Method | Path | Role | Purpose |
| --- | --- | --- | --- |
| `GET` | `/api/v1/storage/buckets` | viewer+ | List buckets |
| `GET` | `/api/v1/storage/objects` | viewer+ | List objects (`bucket`, `prefix`) |
| `GET` | `/api/v1/storage/object` | viewer+ | Bounded object preview (`bucket`, `key`) |

## Connections

| Method | Path | Role | Purpose |
| --- | --- | --- | --- |
| `GET` | `/api/v1/connections` | viewer+ | List connections |
| `POST` | `/api/v1/connections` | admin/operator | Create a secret-backed connection |
| `POST` | `/api/v1/connections/{id}/test` | admin/operator | Actively health-check a connection |

## Components, catalog, prompts

| Method | Path | Role | Purpose |
| --- | --- | --- | --- |
| `GET` | `/api/v1/components` | viewer+ | Live component-health grid |
| `GET` | `/api/v1/catalog` | viewer+ | Shared catalog (`kind` filter: model/feature/agent/tool) |
| `GET` | `/api/v1/prompts` | viewer+ | Langfuse prompt library (proxy) |

## Real-time & serverless

| Method | Path | Role | Purpose |
| --- | --- | --- | --- |
| `GET` | `/api/v1/realtime` | viewer+ | Live stream-demo statistics |
| `POST` | `/api/v1/realtime/{demo}` | service | Report stream stats (from the processor) |
| `GET` | `/api/v1/functions` | viewer+ | List serverless functions |
| `POST` | `/api/v1/functions` | engineer+ | Deploy a function |
| `POST` | `/api/v1/functions/{name}/invoke` | engineer+ | Invoke a function |

## Events & audit

| Method | Path | Role | Purpose |
| --- | --- | --- | --- |
| `GET` | `/api/v1/events` | viewer+ | SSE stream of the state digest (live updates) |
| `GET` | `/api/v1/audit` | viewer+ | Immutable audit log |

!!! note "Role column"
    "viewer+" means viewer and above. "engineer+" means engineer, operator, admin.
    "service" means the internal token identity (used by in-platform reporters).
    Connections are admin/operator only. The exact matrix is in [RBAC](rbac.md).

## Attribution

Mutations are attributed to an actor. In OIDC mode the actor is the token's
email/subject. Otherwise the `X-MLAIOps-Actor` header is honored (used by internal
services like `pipeline-engine`, `materializer`, `realtime-processor`).
