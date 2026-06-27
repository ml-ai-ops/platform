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
