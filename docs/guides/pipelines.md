# Pipelines

Pipelines in Nexus **really execute**. Container flows dispatch through Prefect;
function flows invoke deployed OpenFaaS jobs in dependency order. Both report the
same persisted step graph and logs. There is no simulation.

## The moving parts

- **prefect-server** — the engine (UI at <http://localhost:4200>).
- **pipeline-runner** — serves the platform flows as Prefect deployments and runs
  them; pins `mlflow==3.1.1` + `scikit-learn==1.7.0`.
- **gateway** — accepts submissions, creates flow runs, and recomputes run status
  from the steps the flow reports.
- **OpenFaaS** — runs independent function jobs and function-only DAGs, including
  parallel ready jobs and configured retries.

## Submitting a run

=== "Console"

    **Pipelines → ▶ Run pipeline**, pick a project and `training-pipeline`, submit.
    The run appears immediately as `queued`, then the DAG animates as steps run.

=== "SDK"

    ```python
    run = client.submit_pipeline(project.id, name="training-pipeline")
    ```

=== "API"

    ```bash
    curl -s -X POST http://localhost:8080/api/v1/pipelines/submit \
      -H 'Content-Type: application/json' \
      -d '{"project_id":"<id>","name":"training-pipeline"}'
    ```

What happens under the hood:

1. The gateway persists a `queued` run (+ audit + outbox).
2. It creates a **Prefect flow run** carrying the platform run id and project id.
3. If the engine rejects the submission, the run is marked **failed** (fail-closed).

## The training flow

`python/pipelines/training.py` defines `training_pipeline` with four steps:

```
validate  →  train  →  evaluate  →  register
```

- **validate** — builds a deterministic synthetic dataset (fixed `random_state`), so
  every environment produces identical metrics.
- **train** — fits a scikit-learn classifier.
- **evaluate** — computes metrics.
- **register** — logs the run and model to **MLflow** and registers the model version
  with the control plane against the submitting project.

Each step is wrapped in `reported_step`, which posts `running → succeeded/failed` to
`POST /api/v1/pipelines/runs/{id}/steps`. The gateway recomputes status and progress
deterministically and pushes a digest change over SSE so the console's DAG updates.

## Watching a run

=== "Console"

    Click a run row to open its detail sheet: a **DAG** laid out by the pinned
    Dagre library with per-step status dots, plus a **live log tail**. Cancel and
    retry buttons are there (engineer+).

=== "API"

    ```bash
    curl -s http://localhost:8080/api/v1/pipelines/runs/<run-id> | python -m json.tool
    ```

## Cancel & retry

```bash
curl -s -X POST http://localhost:8080/api/v1/pipelines/runs/<id>/cancel -d '{}'
curl -s -X POST http://localhost:8080/api/v1/pipelines/runs/<id>/retry  -d '{}'
```

Cancellation is authoritative in the control plane and also propagates to Prefect to
stop the actual execution.

## Reusable definitions and function flows

Use **Pipelines → Define flow** to save a versioned DAG of container or function
jobs. Function references must already exist in the same project. The gateway
validates dependency references and rejects cycles before persisting the definition.
Submitting with a `definition_id` preserves the definition, parameters, execution
mode, Git repository, and commit lineage on the run.

Ready function jobs execute concurrently, then pass their output to downstream jobs.
See [Distributed workspaces, functions, and Git](distributed-workspaces.md) for a
complete example.

## Writing your own container flow

Add a flow under `python/pipelines/`, wrap each step in `reported_step(run_id,
"step-name")`, and serve it from `python/pipelines/serve.py`. Keep any ML library
pins aligned with the serving image (`deploy/mlflow/Dockerfile`) so trained models
load under serving. The SDK's `pipelines.py` compiler can emit a Prefect flow from a
pipeline definition.

## Kubernetes fidelity path

On the scale path, pipelines run as KFP/Argo workflows and the operator reconciles
`NexusPipelineRun` CRDs. The KFP integration client stays wired for that path; it's
not needed for local or single-VM use.
