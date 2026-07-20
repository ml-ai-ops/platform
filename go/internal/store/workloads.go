package store

import (
	"context"
	"errors"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/ml-ai-ops/platform/pkg/api"
)

var (
	resourceQuantity = regexp.MustCompile(`^[1-9][0-9]*(m|Mi|Gi)?$`)
	functionName     = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$`)
)

// ValidateFunctionRequest applies the portable subset of the OpenFaaS and
// Kubernetes function contracts before anything is sent to the runtime.
func ValidateFunctionRequest(req api.DeployFunctionRequest) error {
	if req.ProjectID == "" || strings.TrimSpace(req.Name) == "" || strings.TrimSpace(req.Image) == "" {
		return errors.New("project_id, name and image are required")
	}
	if !functionName.MatchString(req.Name) {
		return errors.New("function name must be a lowercase DNS label")
	}
	if req.CPU != "" && !resourceQuantity.MatchString(req.CPU) {
		return errors.New("CPU must be a quantity such as 500m or 2")
	}
	if req.Memory != "" && !resourceQuantity.MatchString(req.Memory) {
		return errors.New("memory must be a quantity such as 512Mi or 2Gi")
	}
	mode := req.Annotations["com.nexus.invocation"]
	if mode != "" && mode != "sync" && mode != "async" {
		return errors.New("com.nexus.invocation must be sync or async")
	}
	if schedule := strings.TrimSpace(req.Annotations["schedule"]); schedule != "" {
		if !strings.Contains(req.Annotations["topic"], "cron-function") {
			return errors.New("scheduled functions must subscribe to the cron-function topic")
		}
		for _, expression := range strings.Split(schedule, ";") {
			if len(strings.Fields(expression)) != 5 {
				return errors.New("schedule must contain five-field cron expressions")
			}
		}
	}
	return nil
}

func validateGitRepository(req api.SetProjectRepositoryRequest) (api.GitRepository, error) {
	raw := strings.TrimSpace(req.URL)
	if raw == "" {
		return api.GitRepository{}, errors.New("repository URL is required")
	}
	provider := "git"
	if strings.HasPrefix(raw, "git@") {
		if !strings.Contains(raw, ":") || strings.ContainsAny(raw, "\n\r\t ") {
			return api.GitRepository{}, errors.New("invalid SSH repository URL")
		}
	} else {
		parsed, err := url.Parse(raw)
		if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
			return api.GitRepository{}, errors.New("repository URL must use HTTPS or the git@host:path SSH form")
		}
		if parsed.User != nil {
			return api.GitRepository{}, errors.New("repository credentials must not be embedded in the URL")
		}
	}
	lower := strings.ToLower(raw)
	switch {
	case strings.Contains(lower, "github.com"):
		provider = "github"
	case strings.Contains(lower, "gitlab.com"):
		provider = "gitlab"
	case strings.Contains(lower, "bitbucket.org"):
		provider = "bitbucket"
	}
	branch := strings.TrimSpace(req.DefaultBranch)
	if branch == "" {
		branch = "main"
	}
	if strings.ContainsAny(branch, " ~^:?*[\\") || strings.HasPrefix(branch, "-") || strings.Contains(branch, "..") {
		return api.GitRepository{}, errors.New("invalid default branch")
	}
	return api.GitRepository{URL: raw, Provider: provider, DefaultBranch: branch, LastCommit: strings.TrimSpace(req.LastCommit)}, nil
}

func (s *Store) Project(projectID string) (api.Project, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, project := range s.data.Projects {
		if project.ID == projectID {
			return project, nil
		}
	}
	return api.Project{}, ErrNotFound
}

func (s *Store) SetProjectRepository(projectID string, req api.SetProjectRepositoryRequest, actor string) (api.Project, error) {
	repository, err := validateGitRepository(req)
	if err != nil {
		return api.Project{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.data.Projects {
		if s.data.Projects[i].ID == projectID {
			now := time.Now().UTC()
			repository.SyncedAt = &now
			s.data.Projects[i].Repository = &repository
			s.record("project.repository_updated", "project", projectID, actor, map[string]any{"provider": repository.Provider})
			return s.data.Projects[i], s.persist()
		}
	}
	return api.Project{}, ErrNotFound
}

func (s *Store) PipelineDefinition(definitionID string) (api.PipelineDefinition, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return pipelineDefinitionFrom(s.data.Definitions, definitionID)
}

func pipelineDefinitionFrom(values []api.PipelineDefinition, definitionID string) (api.PipelineDefinition, error) {
	for _, definition := range values {
		if definition.ID == definitionID {
			return definition, nil
		}
	}
	return api.PipelineDefinition{}, ErrNotFound
}

func validatePipelineDefinition(req api.UpsertPipelineDefinitionRequest) (api.UpsertPipelineDefinitionRequest, error) {
	req.Name, req.Version = strings.TrimSpace(req.Name), strings.TrimSpace(req.Version)
	if req.ProjectID == "" || req.Name == "" || req.Version == "" || len(req.Jobs) == 0 {
		return req, errors.New("project_id, name, version and at least one job are required")
	}
	if req.ExecutionMode == "" {
		req.ExecutionMode = "prefect"
	}
	if req.ExecutionMode != "prefect" && req.ExecutionMode != "functions" {
		return req, errors.New("execution_mode must be prefect or functions")
	}
	known := make(map[string]bool, len(req.Jobs))
	for i := range req.Jobs {
		job := &req.Jobs[i]
		job.Name, job.Kind = strings.TrimSpace(job.Name), strings.TrimSpace(job.Kind)
		if job.Name == "" || known[job.Name] {
			return req, errors.New("job names must be non-empty and unique")
		}
		known[job.Name] = true
		if job.Kind != "function" && job.Kind != "container" {
			return req, errors.New("job kind must be function or container")
		}
		if job.Kind == "function" && strings.TrimSpace(job.Function) == "" {
			return req, errors.New("function jobs require function")
		}
		if job.Kind == "container" && strings.TrimSpace(job.Image) == "" {
			return req, errors.New("container jobs require image")
		}
		if req.ExecutionMode == "functions" && job.Kind != "function" {
			return req, errors.New("functions execution mode only accepts function jobs")
		}
		if job.Resources.CPU != "" && !resourceQuantity.MatchString(job.Resources.CPU) {
			return req, errors.New("job CPU must be a Kubernetes quantity such as 500m or 2")
		}
		if job.Resources.Memory != "" && !resourceQuantity.MatchString(job.Resources.Memory) {
			return req, errors.New("job memory must be a quantity such as 512Mi or 2Gi")
		}
		if job.Retries < 0 || job.Resources.GPU < 0 {
			return req, errors.New("job retries and GPU count cannot be negative")
		}
	}
	for _, job := range req.Jobs {
		for _, dependency := range job.DependsOn {
			if dependency == job.Name || !known[dependency] {
				return req, errors.New("job dependencies must reference another job")
			}
		}
	}
	visiting, visited := map[string]bool{}, map[string]bool{}
	byName := map[string]api.PipelineJob{}
	for _, job := range req.Jobs {
		byName[job.Name] = job
	}
	var visit func(string) bool
	visit = func(name string) bool {
		if visiting[name] {
			return false
		}
		if visited[name] {
			return true
		}
		visiting[name] = true
		for _, parent := range byName[name].DependsOn {
			if !visit(parent) {
				return false
			}
		}
		visiting[name], visited[name] = false, true
		return true
	}
	for name := range byName {
		if !visit(name) {
			return req, errors.New("pipeline graph contains a cycle")
		}
	}
	return req, nil
}

func (s *Store) UpsertPipelineDefinition(definitionID string, req api.UpsertPipelineDefinitionRequest, actor string) (api.PipelineDefinition, error) {
	req, err := validatePipelineDefinition(req)
	if err != nil {
		return api.PipelineDefinition{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !hasProject(s.data.Projects, req.ProjectID) {
		return api.PipelineDefinition{}, ErrNotFound
	}
	now := time.Now().UTC()
	for i := range s.data.Definitions {
		if s.data.Definitions[i].ID == definitionID || (definitionID == "" && s.data.Definitions[i].ProjectID == req.ProjectID && s.data.Definitions[i].Name == req.Name && s.data.Definitions[i].Version == req.Version) {
			definition := &s.data.Definitions[i]
			definition.Name, definition.Version, definition.ExecutionMode, definition.Jobs = req.Name, req.Version, req.ExecutionMode, req.Jobs
			definition.RepositoryURL, definition.CommitSHA, definition.UpdatedAt = req.RepositoryURL, req.CommitSHA, now
			s.record("pipeline_definition.updated", "pipeline_definition", definition.ID, actor, nil)
			return *definition, s.persist()
		}
	}
	definition := api.PipelineDefinition{ID: id("pipe"), ProjectID: req.ProjectID, Name: req.Name, Version: req.Version, ExecutionMode: req.ExecutionMode, Jobs: req.Jobs, RepositoryURL: req.RepositoryURL, CommitSHA: req.CommitSHA, CreatedAt: now, UpdatedAt: now}
	s.data.Definitions = append([]api.PipelineDefinition{definition}, s.data.Definitions...)
	s.record("pipeline_definition.created", "pipeline_definition", definition.ID, actor, nil)
	return definition, s.persist()
}

func stepsFromDefinition(definition api.PipelineDefinition) []api.PipelineStep {
	steps := make([]api.PipelineStep, 0, len(definition.Jobs))
	for _, job := range definition.Jobs {
		image := job.Image
		if job.Kind == "function" {
			image = "function://" + job.Function
		}
		steps = append(steps, api.PipelineStep{Name: job.Name, Status: "pending", Image: image, DependsOn: job.DependsOn})
	}
	return steps
}

func resetSteps(previous []api.PipelineStep) []api.PipelineStep {
	steps := make([]api.PipelineStep, len(previous))
	copy(steps, previous)
	for i := range steps {
		steps[i].Status, steps[i].Progress = "pending", 0
	}
	return steps
}

func (s *Store) UpsertFunction(req api.DeployFunctionRequest, owner, actor string) (api.Function, error) {
	if err := ValidateFunctionRequest(req); err != nil {
		return api.Function{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !hasProject(s.data.Projects, req.ProjectID) {
		return api.Function{}, ErrNotFound
	}
	now := time.Now().UTC()
	for i := range s.data.Functions {
		if s.data.Functions[i].Name == req.Name {
			fn := &s.data.Functions[i]
			if fn.ProjectID != req.ProjectID {
				return api.Function{}, ErrConflict
			}
			fn.Image, fn.EnvVars, fn.Labels, fn.Annotations, fn.CPU, fn.Memory, fn.Status, fn.UpdatedAt = req.Image, req.EnvVars, req.Labels, req.Annotations, req.CPU, req.Memory, "deployed", now
			s.record("function.updated", "function", fn.Name, actor, nil)
			return *fn, s.persist()
		}
	}
	fn := api.Function{Name: req.Name, ProjectID: req.ProjectID, Image: req.Image, EnvVars: req.EnvVars, Labels: req.Labels, Annotations: req.Annotations, CPU: req.CPU, Memory: req.Memory, OwnerSubject: owner, Status: "deployed", CreatedAt: now, UpdatedAt: now}
	s.data.Functions = append([]api.Function{fn}, s.data.Functions...)
	s.record("function.deployed", "function", fn.Name, actor, nil)
	return fn, s.persist()
}

func (s *Store) DeleteFunction(name, actor string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.data.Functions {
		if s.data.Functions[i].Name == name {
			s.data.Functions = append(s.data.Functions[:i], s.data.Functions[i+1:]...)
			s.record("function.deleted", "function", name, actor, nil)
			return s.persist()
		}
	}
	return ErrNotFound
}

func (p *Postgres) Project(projectID string) (api.Project, error) {
	return get[api.Project](p, "project", projectID)
}
func (p *Postgres) SetProjectRepository(projectID string, req api.SetProjectRepositoryRequest, actor string) (api.Project, error) {
	project, err := p.Project(projectID)
	if err != nil {
		return project, err
	}
	repository, err := validateGitRepository(req)
	if err != nil {
		return project, err
	}
	now := time.Now().UTC()
	repository.SyncedAt = &now
	project.Repository = &repository
	return project, p.write("project", project.ID, project, "project.repository_updated", actor, map[string]any{"provider": repository.Provider})
}
func (p *Postgres) PipelineDefinition(definitionID string) (api.PipelineDefinition, error) {
	return get[api.PipelineDefinition](p, "pipeline_definition", definitionID)
}
func (p *Postgres) UpsertPipelineDefinition(definitionID string, req api.UpsertPipelineDefinitionRequest, actor string) (api.PipelineDefinition, error) {
	req, err := validatePipelineDefinition(req)
	if err != nil {
		return api.PipelineDefinition{}, err
	}
	if !p.exists("project", req.ProjectID) {
		return api.PipelineDefinition{}, ErrNotFound
	}
	now := time.Now().UTC()
	if definitionID != "" {
		definition, getErr := p.PipelineDefinition(definitionID)
		if getErr != nil {
			return definition, getErr
		}
		definition.Name, definition.Version, definition.ExecutionMode, definition.Jobs, definition.RepositoryURL, definition.CommitSHA, definition.UpdatedAt = req.Name, req.Version, req.ExecutionMode, req.Jobs, req.RepositoryURL, req.CommitSHA, now
		return definition, p.write("pipeline_definition", definition.ID, definition, "pipeline_definition.updated", actor, nil)
	}
	for _, existing := range p.PipelineDefinitions() {
		if existing.ProjectID == req.ProjectID && existing.Name == req.Name && existing.Version == req.Version {
			return p.UpsertPipelineDefinition(existing.ID, req, actor)
		}
	}
	definition := api.PipelineDefinition{ID: id("pipe"), ProjectID: req.ProjectID, Name: req.Name, Version: req.Version, ExecutionMode: req.ExecutionMode, Jobs: req.Jobs, RepositoryURL: req.RepositoryURL, CommitSHA: req.CommitSHA, CreatedAt: now, UpdatedAt: now}
	return definition, p.write("pipeline_definition", definition.ID, definition, "pipeline_definition.created", actor, nil)
}
func (p *Postgres) UpsertFunction(req api.DeployFunctionRequest, owner, actor string) (api.Function, error) {
	if err := ValidateFunctionRequest(req); err != nil {
		return api.Function{}, err
	}
	if !p.exists("project", req.ProjectID) {
		return api.Function{}, ErrNotFound
	}
	now := time.Now().UTC()
	for _, fn := range p.Functions() {
		if fn.Name == req.Name {
			if fn.ProjectID != req.ProjectID {
				return api.Function{}, ErrConflict
			}
			fn.Image, fn.EnvVars, fn.Labels, fn.Annotations, fn.CPU, fn.Memory, fn.Status, fn.UpdatedAt = req.Image, req.EnvVars, req.Labels, req.Annotations, req.CPU, req.Memory, "deployed", now
			return fn, p.write("function", fn.Name, fn, "function.updated", actor, nil)
		}
	}
	fn := api.Function{Name: req.Name, ProjectID: req.ProjectID, Image: req.Image, EnvVars: req.EnvVars, Labels: req.Labels, Annotations: req.Annotations, CPU: req.CPU, Memory: req.Memory, OwnerSubject: owner, Status: "deployed", CreatedAt: now, UpdatedAt: now}
	return fn, p.write("function", fn.Name, fn, "function.deployed", actor, nil)
}
func (p *Postgres) DeleteFunction(name, actor string) error {
	tag, err := p.pool.Exec(context.Background(), `DELETE FROM platform_resources WHERE tenant_id=$1 AND kind='function' AND id=$2`, p.tenant, name)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	event := api.AuditEvent{ID: id("evt"), Action: "function.deleted", Resource: "function", ResourceID: name, Actor: actorOrAnonymous(actor), CreatedAt: time.Now().UTC()}
	_, err = p.pool.Exec(context.Background(), `INSERT INTO audit_events (tenant_id,id,action,resource,resource_id,actor,metadata,created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		p.tenant, event.ID, event.Action, event.Resource, event.ResourceID, event.Actor, event.Metadata, event.CreatedAt)
	return err
}
