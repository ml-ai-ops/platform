// Package api contains the stable control-plane wire contracts.
package api

import "time"

type Project struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Template    string    `json:"template"`
	Namespace   string    `json:"namespace"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
}

type CreateProjectRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Template    string `json:"template"`
}

type PipelineRun struct {
	ID        string    `json:"id"`
	ProjectID string    `json:"project_id"`
	Name      string    `json:"name"`
	Status    string    `json:"status"`
	Progress  int       `json:"progress"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type SubmitPipelineRequest struct {
	ProjectID string `json:"project_id"`
	Name      string `json:"name"`
}

type Component struct {
	Name        string `json:"name"`
	Category    string `json:"category"`
	Status      string `json:"status"`
	Description string `json:"description"`
	URL         string `json:"url,omitempty"`
}

type Model struct {
	ID          string             `json:"id"`
	ProjectID   string             `json:"project_id"`
	Name        string             `json:"name"`
	Version     string             `json:"version"`
	Stage       string             `json:"stage"`
	ArtifactURI string             `json:"artifact_uri"`
	Metrics     map[string]float64 `json:"metrics"`
	CreatedAt   time.Time          `json:"created_at"`
}

type RegisterModelRequest struct {
	ProjectID   string             `json:"project_id"`
	Name        string             `json:"name"`
	Version     string             `json:"version"`
	ArtifactURI string             `json:"artifact_uri"`
	Metrics     map[string]float64 `json:"metrics"`
}

type PromoteModelRequest struct {
	Stage string `json:"stage"`
}

type Agent struct {
	ID           string    `json:"id"`
	ProjectID    string    `json:"project_id"`
	Name         string    `json:"name"`
	Version      string    `json:"version"`
	Image        string    `json:"image"`
	GraphModule  string    `json:"graph_module"`
	LLMBackend   string    `json:"llm_backend"`
	Status       string    `json:"status"`
	Replicas     int       `json:"replicas"`
	CanaryWeight int       `json:"canary_weight"`
	Tools        []string  `json:"tools"`
	CreatedAt    time.Time `json:"created_at"`
}

type DeployAgentRequest struct {
	ProjectID   string   `json:"project_id"`
	Name        string   `json:"name"`
	Version     string   `json:"version"`
	Image       string   `json:"image"`
	GraphModule string   `json:"graph_module"`
	LLMBackend  string   `json:"llm_backend"`
	Replicas    int      `json:"replicas"`
	Tools       []string `json:"tools"`
}

type TrafficRequest struct {
	CanaryWeight int `json:"canary_weight"`
}

type Tool struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Version     string         `json:"version"`
	Description string         `json:"description"`
	Tags        []string       `json:"tags"`
	InputSchema map[string]any `json:"input_schema"`
	Status      string         `json:"status"`
	CreatedAt   time.Time      `json:"created_at"`
}

type RegisterToolRequest struct {
	Name        string         `json:"name"`
	Version     string         `json:"version"`
	Description string         `json:"description"`
	Tags        []string       `json:"tags"`
	InputSchema map[string]any `json:"input_schema"`
}

type Connection struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Type      string    `json:"type"`
	Endpoint  string    `json:"endpoint"`
	SecretRef string    `json:"secret_ref"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

type CreateConnectionRequest struct {
	Name      string `json:"name"`
	Type      string `json:"type"`
	Endpoint  string `json:"endpoint"`
	SecretRef string `json:"secret_ref"`
}

type AuditEvent struct {
	ID         string         `json:"id"`
	Action     string         `json:"action"`
	Resource   string         `json:"resource"`
	ResourceID string         `json:"resource_id"`
	Actor      string         `json:"actor"`
	Metadata   map[string]any `json:"metadata,omitempty"`
	CreatedAt  time.Time      `json:"created_at"`
}

type CatalogItem struct {
	Name     string   `json:"name"`
	Version  string   `json:"version"`
	Status   string   `json:"status"`
	Kind     string   `json:"kind"`
	Metadata []string `json:"metadata"`
}

type Dashboard struct {
	Projects      int           `json:"projects"`
	ActiveRuns    int           `json:"active_runs"`
	Healthy       int           `json:"healthy_components"`
	Total         int           `json:"total_components"`
	RecentRuns    []PipelineRun `json:"recent_runs"`
	OnboardingPct int           `json:"onboarding_percent"`
}

type APIError struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

type Page[T any] struct {
	Items []T `json:"items"`
	Total int `json:"total"`
}
