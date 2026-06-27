package store

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/mlaiops/platform/pkg/api"
)

var ErrNotFound = errors.New("resource not found")

type Store struct {
	mu       sync.RWMutex
	projects []api.Project
	runs     []api.PipelineRun
}

func New() *Store {
	now := time.Now().UTC()
	return &Store{
		projects: []api.Project{{
			ID: "prj-demo", Name: "Fraud detection starter", Description: "A guided tabular ML project",
			Template: "tabular-classification", Namespace: "team-demo", Status: "ready", CreatedAt: now.Add(-2 * time.Hour),
		}},
		runs: []api.PipelineRun{{
			ID: "run-demo", ProjectID: "prj-demo", Name: "training-pipeline",
			Status: "succeeded", Progress: 100, CreatedAt: now.Add(-45 * time.Minute),
		}},
	}
}

func (s *Store) Projects() []api.Project {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]api.Project(nil), s.projects...)
}

func (s *Store) CreateProject(req api.CreateProjectRequest) (api.Project, error) {
	name := strings.TrimSpace(req.Name)
	if len(name) < 3 {
		return api.Project{}, errors.New("name must contain at least 3 characters")
	}
	if req.Template == "" {
		req.Template = "tabular-classification"
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	id := fmt.Sprintf("prj-%d", time.Now().UnixNano())
	project := api.Project{
		ID: id, Name: name, Description: strings.TrimSpace(req.Description), Template: req.Template,
		Namespace: slug(name), Status: "ready", CreatedAt: time.Now().UTC(),
	}
	s.projects = append([]api.Project{project}, s.projects...)
	return project, nil
}

func (s *Store) Runs() []api.PipelineRun {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]api.PipelineRun(nil), s.runs...)
}

func (s *Store) SubmitPipeline(req api.SubmitPipelineRequest) (api.PipelineRun, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	found := false
	for _, project := range s.projects {
		if project.ID == req.ProjectID {
			found = true
			break
		}
	}
	if !found {
		return api.PipelineRun{}, ErrNotFound
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = "training-pipeline"
	}
	run := api.PipelineRun{
		ID: fmt.Sprintf("run-%d", time.Now().UnixNano()), ProjectID: req.ProjectID,
		Name: name, Status: "queued", Progress: 0, CreatedAt: time.Now().UTC(),
	}
	s.runs = append([]api.PipelineRun{run}, s.runs...)
	return run, nil
}

func slug(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var result strings.Builder
	lastDash := false
	for _, char := range value {
		valid := char >= 'a' && char <= 'z' || char >= '0' && char <= '9'
		if valid {
			result.WriteRune(char)
			lastDash = false
		} else if !lastDash && result.Len() > 0 {
			result.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(result.String(), "-")
}
