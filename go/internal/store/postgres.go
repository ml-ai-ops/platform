package store

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mlaiops/platform/pkg/api"
)

//go:embed migrations/*.sql
var migrations embed.FS

type Postgres struct {
	pool   *pgxpool.Pool
	tenant string
}

type OutboxEvent struct {
	Sequence int64
	ID       string
	Topic    string
	Key      string
	Payload  json.RawMessage
	Attempts int
}

func OpenPostgres(ctx context.Context, databaseURL, tenant string) (*Postgres, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	if tenant == "" {
		tenant = "default"
	}
	repository := &Postgres{pool: pool, tenant: tenant}
	if err := repository.Migrate(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return repository, nil
}

func (p *Postgres) Close()                         { p.pool.Close() }
func (p *Postgres) Ping(ctx context.Context) error { return p.pool.Ping(ctx) }

func (p *Postgres) Migrate(ctx context.Context) error {
	raw, err := migrations.ReadFile("migrations/001_initial.sql")
	if err != nil {
		return err
	}
	_, err = p.pool.Exec(ctx, string(raw))
	return err
}

func (p *Postgres) Projects() []api.Project       { return list[api.Project](p, "project") }
func (p *Postgres) Runs() []api.PipelineRun       { return list[api.PipelineRun](p, "pipeline_run") }
func (p *Postgres) Models() []api.Model           { return list[api.Model](p, "model") }
func (p *Postgres) Agents() []api.Agent           { return list[api.Agent](p, "agent") }
func (p *Postgres) Tools() []api.Tool             { return list[api.Tool](p, "tool") }
func (p *Postgres) Connections() []api.Connection { return list[api.Connection](p, "connection") }

func (p *Postgres) Audit() []api.AuditEvent {
	rows, err := p.pool.Query(context.Background(), `SELECT id, action, resource, resource_id, actor, metadata, created_at FROM audit_events WHERE tenant_id=$1 ORDER BY sequence DESC LIMIT 500`, p.tenant)
	if err != nil {
		return []api.AuditEvent{}
	}
	defer rows.Close()
	result := make([]api.AuditEvent, 0)
	for rows.Next() {
		var event api.AuditEvent
		if rows.Scan(&event.ID, &event.Action, &event.Resource, &event.ResourceID, &event.Actor, &event.Metadata, &event.CreatedAt) == nil {
			result = append(result, event)
		}
	}
	return result
}

func (p *Postgres) CreateProject(req api.CreateProjectRequest, actors ...string) (api.Project, error) {
	if len(strings.TrimSpace(req.Name)) < 3 {
		return api.Project{}, errors.New("name must contain at least 3 characters")
	}
	if req.Template == "" {
		req.Template = "tabular-classification"
	}
	for _, project := range p.Projects() {
		if strings.EqualFold(project.Name, req.Name) {
			return api.Project{}, ErrConflict
		}
	}
	project := api.Project{ID: id("prj"), Name: strings.TrimSpace(req.Name), Description: strings.TrimSpace(req.Description), Template: req.Template, Namespace: slug(req.Name), Status: "ready", CreatedAt: time.Now().UTC()}
	return project, p.write("project", project.ID, project, "project.created", first(actors), nil)
}

func (p *Postgres) SubmitPipeline(req api.SubmitPipelineRequest, actors ...string) (api.PipelineRun, error) {
	if !p.exists("project", req.ProjectID) {
		return api.PipelineRun{}, ErrNotFound
	}
	if strings.TrimSpace(req.Name) == "" {
		req.Name = "training-pipeline"
	}
	now := time.Now().UTC()
	run := api.PipelineRun{ID: id("run"), ProjectID: req.ProjectID, Name: req.Name, Status: "queued", CreatedAt: now, UpdatedAt: now}
	return run, p.write("pipeline_run", run.ID, run, "pipeline.submitted", first(actors), nil)
}

func (p *Postgres) RegisterModel(req api.RegisterModelRequest, actor string) (api.Model, error) {
	if !p.exists("project", req.ProjectID) {
		return api.Model{}, ErrNotFound
	}
	if req.Name == "" || req.Version == "" || req.ArtifactURI == "" {
		return api.Model{}, errors.New("name, version and artifact_uri are required")
	}
	for _, model := range p.Models() {
		if model.Name == req.Name && model.Version == req.Version {
			return api.Model{}, ErrConflict
		}
	}
	model := api.Model{ID: id("mdl"), ProjectID: req.ProjectID, Name: req.Name, Version: req.Version, Stage: "candidate", ArtifactURI: req.ArtifactURI, Metrics: req.Metrics, CreatedAt: time.Now().UTC()}
	return model, p.write("model", model.ID, model, "model.registered", actor, nil)
}

func (p *Postgres) PromoteModel(modelID, stage, actor string) (api.Model, error) {
	if stage != "staging" && stage != "production" && stage != "archived" {
		return api.Model{}, errors.New("stage must be staging, production or archived")
	}
	model, err := get[api.Model](p, "model", modelID)
	if err != nil {
		return model, err
	}
	model.Stage = stage
	return model, p.write("model", model.ID, model, "model.promoted", actor, map[string]any{"stage": stage})
}

func (p *Postgres) DeployAgent(req api.DeployAgentRequest, actor string) (api.Agent, error) {
	if !p.exists("project", req.ProjectID) {
		return api.Agent{}, ErrNotFound
	}
	if req.Name == "" || req.Version == "" || req.Image == "" || req.GraphModule == "" {
		return api.Agent{}, errors.New("name, version, image and graph_module are required")
	}
	if req.Replicas < 1 {
		req.Replicas = 1
	}
	agent := api.Agent{ID: id("agt"), ProjectID: req.ProjectID, Name: req.Name, Version: req.Version, Image: req.Image, GraphModule: req.GraphModule, LLMBackend: req.LLMBackend, Status: "pending", Replicas: req.Replicas, Tools: req.Tools, CreatedAt: time.Now().UTC()}
	return agent, p.write("agent", agent.ID, agent, "agent.deployed", actor, nil)
}

func (p *Postgres) SetAgentTraffic(agentID string, weight int, actor string) (api.Agent, error) {
	if weight < 0 || weight > 100 {
		return api.Agent{}, errors.New("canary_weight must be between 0 and 100")
	}
	agent, err := get[api.Agent](p, "agent", agentID)
	if err != nil {
		return agent, err
	}
	agent.CanaryWeight = weight
	return agent, p.write("agent", agent.ID, agent, "agent.traffic_updated", actor, map[string]any{"canary_weight": weight})
}

func (p *Postgres) RegisterTool(req api.RegisterToolRequest, actor string) (api.Tool, error) {
	if req.Name == "" || req.Version == "" || len(req.InputSchema) == 0 {
		return api.Tool{}, errors.New("name, version and input_schema are required")
	}
	for _, tool := range p.Tools() {
		if tool.Name == req.Name && tool.Version == req.Version {
			return api.Tool{}, ErrConflict
		}
	}
	tool := api.Tool{ID: id("tool"), Name: req.Name, Version: req.Version, Description: req.Description, Tags: req.Tags, InputSchema: req.InputSchema, Status: "ready", CreatedAt: time.Now().UTC()}
	return tool, p.write("tool", tool.ID, tool, "tool.registered", actor, nil)
}

func (p *Postgres) CreateConnection(req api.CreateConnectionRequest, actor string) (api.Connection, error) {
	if req.Name == "" || req.Type == "" || req.SecretRef == "" {
		return api.Connection{}, errors.New("name, type and secret_ref are required")
	}
	connection := api.Connection{ID: id("conn"), Name: req.Name, Type: req.Type, Endpoint: req.Endpoint, SecretRef: req.SecretRef, Status: "pending", CreatedAt: time.Now().UTC()}
	return connection, p.write("connection", connection.ID, connection, "connection.created", actor, nil)
}

func (p *Postgres) write(kind, resourceID string, value any, action, actor string, metadata map[string]any) error {
	ctx := context.Background()
	tx, err := p.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO platform_resources (tenant_id,kind,id,payload) VALUES ($1,$2,$3,$4)
		ON CONFLICT (tenant_id,kind,id) DO UPDATE SET payload=EXCLUDED.payload, updated_at=now()`, p.tenant, kind, resourceID, payload)
	if err != nil {
		return err
	}
	event := api.AuditEvent{ID: id("evt"), Action: action, Resource: kind, ResourceID: resourceID, Actor: actorOrAnonymous(actor), Metadata: metadata, CreatedAt: time.Now().UTC()}
	eventPayload, _ := json.Marshal(event)
	_, err = tx.Exec(ctx, `INSERT INTO audit_events (tenant_id,id,action,resource,resource_id,actor,metadata,created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		p.tenant, event.ID, event.Action, event.Resource, event.ResourceID, event.Actor, event.Metadata, event.CreatedAt)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO outbox_events (tenant_id,id,topic,event_key,payload) VALUES ($1,$2,$3,$4,$5)`,
		p.tenant, id("out"), "mlaiops.audit.operations", resourceID, eventPayload)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (p *Postgres) PendingOutbox(ctx context.Context, limit int) ([]OutboxEvent, error) {
	rows, err := p.pool.Query(ctx, `SELECT sequence,id,topic,event_key,payload,attempts FROM outbox_events WHERE tenant_id=$1 AND published_at IS NULL ORDER BY sequence FOR UPDATE SKIP LOCKED LIMIT $2`, p.tenant, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var events []OutboxEvent
	for rows.Next() {
		var event OutboxEvent
		if err := rows.Scan(&event.Sequence, &event.ID, &event.Topic, &event.Key, &event.Payload, &event.Attempts); err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, rows.Err()
}

func (p *Postgres) MarkPublished(ctx context.Context, sequence int64) error {
	_, err := p.pool.Exec(ctx, `UPDATE outbox_events SET published_at=now(), attempts=attempts+1, last_error=NULL WHERE tenant_id=$1 AND sequence=$2`, p.tenant, sequence)
	return err
}

func (p *Postgres) MarkFailed(ctx context.Context, sequence int64, publishErr error) error {
	_, err := p.pool.Exec(ctx, `UPDATE outbox_events SET attempts=attempts+1,last_error=$3 WHERE tenant_id=$1 AND sequence=$2`, p.tenant, sequence, publishErr.Error())
	return err
}

func list[T any](p *Postgres, kind string) []T {
	rows, err := p.pool.Query(context.Background(), `SELECT payload FROM platform_resources WHERE tenant_id=$1 AND kind=$2 ORDER BY created_at DESC`, p.tenant, kind)
	if err != nil {
		return []T{}
	}
	defer rows.Close()
	result := make([]T, 0)
	for rows.Next() {
		var raw []byte
		var value T
		if rows.Scan(&raw) == nil && json.Unmarshal(raw, &value) == nil {
			result = append(result, value)
		}
	}
	return result
}

func get[T any](p *Postgres, kind, resourceID string) (T, error) {
	var value T
	var raw []byte
	err := p.pool.QueryRow(context.Background(), `SELECT payload FROM platform_resources WHERE tenant_id=$1 AND kind=$2 AND id=$3`, p.tenant, kind, resourceID).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return value, ErrNotFound
	}
	if err != nil {
		return value, err
	}
	return value, json.Unmarshal(raw, &value)
}

func (p *Postgres) exists(kind, resourceID string) bool {
	var exists bool
	_ = p.pool.QueryRow(context.Background(), `SELECT EXISTS(SELECT 1 FROM platform_resources WHERE tenant_id=$1 AND kind=$2 AND id=$3)`, p.tenant, kind, resourceID).Scan(&exists)
	return exists
}

func actorOrAnonymous(actor string) string {
	if actor == "" {
		return "anonymous"
	}
	return actor
}

func (p *Postgres) String() string { return fmt.Sprintf("postgres(tenant=%s)", p.tenant) }
