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
	ID          string    `json:"id"`
	ProjectID   string    `json:"project_id"`
	Name        string    `json:"name"`
	Status      string    `json:"status"`
	Progress    int       `json:"progress"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	ParentRunID string    `json:"parent_run_id,omitempty"`
	// EngineRunID links the control-plane run to the execution engine's run
	// (Prefect flow run id locally, KFP run id on Kubernetes).
	EngineRunID string         `json:"engine_run_id,omitempty"`
	Steps       []PipelineStep `json:"steps"`
	Logs        []RunLog       `json:"logs,omitempty"`
}

// UpdateRunStepRequest is sent by the executing pipeline itself (through the
// SDK step reporter) at every step transition.
type UpdateRunStepRequest struct {
	Step    string `json:"step"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

type PipelineStep struct {
	Name      string   `json:"name"`
	Status    string   `json:"status"`
	Image     string   `json:"image"`
	DependsOn []string `json:"depends_on,omitempty"`
	Progress  int      `json:"progress"`
}

type RunLog struct {
	Timestamp time.Time `json:"timestamp"`
	Step      string    `json:"step"`
	Level     string    `json:"level"`
	Message   string    `json:"message"`
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
	ID               string             `json:"id"`
	ProjectID        string             `json:"project_id"`
	Name             string             `json:"name"`
	Version          string             `json:"version"`
	Stage            string             `json:"stage"`
	ArtifactURI      string             `json:"artifact_uri"`
	Metrics          map[string]float64 `json:"metrics"`
	CreatedAt        time.Time          `json:"created_at"`
	GateStatus       string             `json:"gate_status"`
	DeploymentStatus string             `json:"deployment_status"`
	CanaryWeight     int                `json:"canary_weight"`
	EndpointURL      string             `json:"endpoint_url,omitempty"`
	PreviousStage    string             `json:"previous_stage,omitempty"`
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

type DeployModelRequest struct {
	CanaryWeight int `json:"canary_weight"`
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
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Type      string     `json:"type"`
	Endpoint  string     `json:"endpoint"`
	SecretRef string     `json:"secret_ref"`
	Status    string     `json:"status"`
	CreatedAt time.Time  `json:"created_at"`
	CheckedAt *time.Time `json:"checked_at,omitempty"`
	Message   string     `json:"message,omitempty"`
}

type CreateConnectionRequest struct {
	Name      string `json:"name"`
	Type      string `json:"type"`
	Endpoint  string `json:"endpoint"`
	SecretRef string `json:"secret_ref"`
}

type ReadinessItem struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	Status      string `json:"status"`
	Description string `json:"description"`
	Action      string `json:"action,omitempty"`
}

type Readiness struct {
	Percent int             `json:"percent"`
	Ready   bool            `json:"ready"`
	Items   []ReadinessItem `json:"items"`
}

type AgentSession struct {
	ID           string    `json:"id"`
	AgentID      string    `json:"agent_id"`
	UserID       string    `json:"user_id"`
	Status       string    `json:"status"`
	CurrentNode  string    `json:"current_node"`
	Turns        int       `json:"turns"`
	InputTokens  int       `json:"input_tokens"`
	OutputTokens int       `json:"output_tokens"`
	CostUSD      float64   `json:"cost_usd"`
	StartedAt    time.Time `json:"started_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type AgentTrace struct {
	ID         string         `json:"id"`
	AgentID    string         `json:"agent_id"`
	SessionID  string         `json:"session_id"`
	Name       string         `json:"name"`
	Status     string         `json:"status"`
	DurationMS int64          `json:"duration_ms"`
	Tokens     int            `json:"tokens"`
	Metadata   map[string]any `json:"metadata,omitempty"`
	CreatedAt  time.Time      `json:"created_at"`
}

type RecordTraceRequest struct {
	AgentID      string         `json:"agent_id"`
	SessionID    string         `json:"session_id"`
	UserID       string         `json:"user_id"`
	Name         string         `json:"name"`
	Status       string         `json:"status"`
	CurrentNode  string         `json:"current_node"`
	DurationMS   int64          `json:"duration_ms"`
	InputTokens  int            `json:"input_tokens"`
	OutputTokens int            `json:"output_tokens"`
	CostUSD      float64        `json:"cost_usd"`
	Metadata     map[string]any `json:"metadata,omitempty"`
}

// FeatureView is the control-plane record of a feature store view: what the
// catalog shows and what materialization jobs report against. The data itself
// lives in the online (Redis/Feast) and offline (Parquet on S3) stores.
type FeatureView struct {
	ID                string         `json:"id"`
	Name              string         `json:"name"`
	Entity            string         `json:"entity"`
	Fields            []FeatureField `json:"fields"`
	Tags              []string       `json:"tags"`
	Source            string         `json:"source"`
	TTLSeconds        int            `json:"ttl_seconds"`
	Status            string         `json:"status"`
	OnlineEntityCount int            `json:"online_entity_count"`
	MaterializedAt    *time.Time     `json:"materialized_at,omitempty"`
	CreatedAt         time.Time      `json:"created_at"`
}

type FeatureField struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type ApplyFeatureViewRequest struct {
	Name       string         `json:"name"`
	Entity     string         `json:"entity"`
	Fields     []FeatureField `json:"fields"`
	Tags       []string       `json:"tags"`
	Source     string         `json:"source"`
	TTLSeconds int            `json:"ttl_seconds"`
}

type MaterializationReport struct {
	EntityCount int `json:"entity_count"`
}

type InvokeAgentRequest struct {
	Message   string `json:"message"`
	SessionID string `json:"session_id,omitempty"`
	UserID    string `json:"user_id,omitempty"`
}

type AgentUsage struct {
	Sessions     int     `json:"sessions"`
	Active       int     `json:"active"`
	InputTokens  int     `json:"input_tokens"`
	OutputTokens int     `json:"output_tokens"`
	CostUSD      float64 `json:"cost_usd"`
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
