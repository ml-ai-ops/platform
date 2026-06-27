package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/mlaiops/platform/pkg/api"
)

var ErrNotFound = errors.New("resource not found")
var ErrConflict = errors.New("resource already exists")

type state struct {
	Projects    []api.Project     `json:"projects"`
	Runs        []api.PipelineRun `json:"runs"`
	Models      []api.Model       `json:"models"`
	Agents      []api.Agent       `json:"agents"`
	Tools       []api.Tool        `json:"tools"`
	Connections []api.Connection  `json:"connections"`
	Audit       []api.AuditEvent  `json:"audit"`
}

type Store struct {
	mu   sync.RWMutex
	path string
	data state
}

func New(path ...string) *Store {
	s := &Store{}
	if len(path) > 0 {
		s.path = path[0]
	}
	if s.path != "" {
		_ = s.load()
	}
	if len(s.data.Projects) == 0 {
		now := time.Now().UTC()
		s.data.Projects = []api.Project{{ID: "prj-demo", Name: "Fraud detection starter", Description: "A guided tabular ML project", Template: "tabular-classification", Namespace: "team-demo", Status: "ready", CreatedAt: now}}
		s.data.Runs = []api.PipelineRun{{ID: "run-demo", ProjectID: "prj-demo", Name: "training-pipeline", Status: "succeeded", Progress: 100, CreatedAt: now, UpdatedAt: now}}
		_ = s.persist()
	}
	return s
}

func (s *Store) Projects() []api.Project {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return clone(s.data.Projects)
}
func (s *Store) Runs() []api.PipelineRun {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return clone(s.data.Runs)
}
func (s *Store) Models() []api.Model { s.mu.RLock(); defer s.mu.RUnlock(); return clone(s.data.Models) }
func (s *Store) Agents() []api.Agent { s.mu.RLock(); defer s.mu.RUnlock(); return clone(s.data.Agents) }
func (s *Store) Tools() []api.Tool   { s.mu.RLock(); defer s.mu.RUnlock(); return clone(s.data.Tools) }
func (s *Store) Connections() []api.Connection {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return clone(s.data.Connections)
}
func (s *Store) Audit() []api.AuditEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return clone(s.data.Audit)
}

func (s *Store) CreateProject(req api.CreateProjectRequest, actor ...string) (api.Project, error) {
	name := strings.TrimSpace(req.Name)
	if len(name) < 3 {
		return api.Project{}, errors.New("name must contain at least 3 characters")
	}
	if req.Template == "" {
		req.Template = "tabular-classification"
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, v := range s.data.Projects {
		if strings.EqualFold(v.Name, name) {
			return api.Project{}, ErrConflict
		}
	}
	p := api.Project{ID: id("prj"), Name: name, Description: strings.TrimSpace(req.Description), Template: req.Template, Namespace: slug(name), Status: "ready", CreatedAt: time.Now().UTC()}
	s.data.Projects = append([]api.Project{p}, s.data.Projects...)
	s.record("project.created", "project", p.ID, first(actor), nil)
	return p, s.persist()
}

func (s *Store) SubmitPipeline(req api.SubmitPipelineRequest, actor ...string) (api.PipelineRun, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !hasProject(s.data.Projects, req.ProjectID) {
		return api.PipelineRun{}, ErrNotFound
	}
	if strings.TrimSpace(req.Name) == "" {
		req.Name = "training-pipeline"
	}
	now := time.Now().UTC()
	run := api.PipelineRun{ID: id("run"), ProjectID: req.ProjectID, Name: strings.TrimSpace(req.Name), Status: "queued", Progress: 0, CreatedAt: now, UpdatedAt: now}
	s.data.Runs = append([]api.PipelineRun{run}, s.data.Runs...)
	s.record("pipeline.submitted", "pipeline_run", run.ID, first(actor), nil)
	return run, s.persist()
}

func (s *Store) RegisterModel(req api.RegisterModelRequest, actor string) (api.Model, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !hasProject(s.data.Projects, req.ProjectID) {
		return api.Model{}, ErrNotFound
	}
	if req.Name == "" || req.Version == "" || req.ArtifactURI == "" {
		return api.Model{}, errors.New("name, version and artifact_uri are required")
	}
	for _, v := range s.data.Models {
		if v.Name == req.Name && v.Version == req.Version {
			return api.Model{}, ErrConflict
		}
	}
	m := api.Model{ID: id("mdl"), ProjectID: req.ProjectID, Name: req.Name, Version: req.Version, Stage: "candidate", ArtifactURI: req.ArtifactURI, Metrics: req.Metrics, CreatedAt: time.Now().UTC()}
	s.data.Models = append([]api.Model{m}, s.data.Models...)
	s.record("model.registered", "model", m.ID, actor, nil)
	return m, s.persist()
}

func (s *Store) PromoteModel(modelID, stage, actor string) (api.Model, error) {
	allowed := map[string]bool{"staging": true, "production": true, "archived": true}
	if !allowed[stage] {
		return api.Model{}, errors.New("stage must be staging, production or archived")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.data.Models {
		if s.data.Models[i].ID == modelID {
			s.data.Models[i].Stage = stage
			s.record("model.promoted", "model", modelID, actor, map[string]any{"stage": stage})
			return s.data.Models[i], s.persist()
		}
	}
	return api.Model{}, ErrNotFound
}

func (s *Store) DeployAgent(req api.DeployAgentRequest, actor string) (api.Agent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !hasProject(s.data.Projects, req.ProjectID) {
		return api.Agent{}, ErrNotFound
	}
	if req.Name == "" || req.Version == "" || req.Image == "" || req.GraphModule == "" {
		return api.Agent{}, errors.New("name, version, image and graph_module are required")
	}
	if req.Replicas < 1 {
		req.Replicas = 1
	}
	a := api.Agent{ID: id("agt"), ProjectID: req.ProjectID, Name: req.Name, Version: req.Version, Image: req.Image, GraphModule: req.GraphModule, LLMBackend: req.LLMBackend, Replicas: req.Replicas, Tools: req.Tools, Status: "pending", CreatedAt: time.Now().UTC()}
	s.data.Agents = append([]api.Agent{a}, s.data.Agents...)
	s.record("agent.deployed", "agent", a.ID, actor, nil)
	return a, s.persist()
}

func (s *Store) SetAgentTraffic(agentID string, weight int, actor string) (api.Agent, error) {
	if weight < 0 || weight > 100 {
		return api.Agent{}, errors.New("canary_weight must be between 0 and 100")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.data.Agents {
		if s.data.Agents[i].ID == agentID {
			s.data.Agents[i].CanaryWeight = weight
			s.record("agent.traffic_updated", "agent", agentID, actor, map[string]any{"canary_weight": weight})
			return s.data.Agents[i], s.persist()
		}
	}
	return api.Agent{}, ErrNotFound
}

func (s *Store) RegisterTool(req api.RegisterToolRequest, actor string) (api.Tool, error) {
	if req.Name == "" || req.Version == "" || len(req.InputSchema) == 0 {
		return api.Tool{}, errors.New("name, version and input_schema are required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, v := range s.data.Tools {
		if v.Name == req.Name && v.Version == req.Version {
			return api.Tool{}, ErrConflict
		}
	}
	t := api.Tool{ID: id("tool"), Name: req.Name, Version: req.Version, Description: req.Description, Tags: req.Tags, InputSchema: req.InputSchema, Status: "ready", CreatedAt: time.Now().UTC()}
	s.data.Tools = append([]api.Tool{t}, s.data.Tools...)
	s.record("tool.registered", "tool", t.ID, actor, nil)
	return t, s.persist()
}

func (s *Store) CreateConnection(req api.CreateConnectionRequest, actor string) (api.Connection, error) {
	if req.Name == "" || req.Type == "" || req.SecretRef == "" {
		return api.Connection{}, errors.New("name, type and secret_ref are required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, v := range s.data.Connections {
		if strings.EqualFold(v.Name, req.Name) {
			return api.Connection{}, ErrConflict
		}
	}
	c := api.Connection{ID: id("conn"), Name: req.Name, Type: req.Type, Endpoint: req.Endpoint, SecretRef: req.SecretRef, Status: "pending", CreatedAt: time.Now().UTC()}
	s.data.Connections = append([]api.Connection{c}, s.data.Connections...)
	s.record("connection.created", "connection", c.ID, actor, nil)
	return c, s.persist()
}

func (s *Store) load() error {
	raw, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, &s.data)
}

func (s *Store) persist() error {
	if s.path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o750); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func (s *Store) record(action, resource, resourceID, actor string, metadata map[string]any) {
	if actor == "" {
		actor = "anonymous"
	}
	event := api.AuditEvent{ID: id("evt"), Action: action, Resource: resource, ResourceID: resourceID, Actor: actor, Metadata: metadata, CreatedAt: time.Now().UTC()}
	s.data.Audit = append([]api.AuditEvent{event}, s.data.Audit...)
}

func clone[T any](values []T) []T { return append([]T(nil), values...) }
func hasProject(values []api.Project, id string) bool {
	for _, v := range values {
		if v.ID == id {
			return true
		}
	}
	return false
}
func first(values []string) string {
	if len(values) > 0 {
		return values[0]
	}
	return ""
}
func id(prefix string) string { return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano()) }
func slug(value string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(strings.TrimSpace(value)) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
			dash = false
		} else if !dash && b.Len() > 0 {
			b.WriteByte('-')
			dash = true
		}
	}
	return strings.Trim(b.String(), "-")
}
