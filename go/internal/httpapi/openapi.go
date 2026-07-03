package httpapi

import "net/http"

func (s *Server) openapi(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"openapi": "3.0.3",
		"info":    map[string]any{"title": "ml-ai-ops-platform API", "version": "0.1.0"},
		"paths": map[string]any{
			"/api/v1/health":               map[string]any{"get": map[string]any{"summary": "Gateway health"}},
			"/api/v1/me":                   map[string]any{"get": map[string]any{"summary": "Caller identity, roles and effective permissions"}},
			"/api/v1/dashboard":            map[string]any{"get": map[string]any{"summary": "Workspace summary"}},
			"/api/v1/onboarding/readiness": map[string]any{"get": map[string]any{"summary": "Onboarding readiness"}},
			"/api/v1/projects":             map[string]any{"get": map[string]any{"summary": "List projects"}, "post": map[string]any{"summary": "Create project"}},
			"/api/v1/pipelines/runs":       map[string]any{"get": map[string]any{"summary": "List pipeline runs"}},
			"/api/v1/pipelines/submit":     map[string]any{"post": map[string]any{"summary": "Submit pipeline"}},
			"/api/v1/pipelines/runs/{id}":  map[string]any{"get": map[string]any{"summary": "Pipeline steps and logs"}},
			"/api/v1/components":           map[string]any{"get": map[string]any{"summary": "List platform component health"}},
			"/api/v1/catalog":              map[string]any{"get": map[string]any{"summary": "Browse models, features, agents and tools"}},
			"/api/v1/models":               map[string]any{"get": map[string]any{"summary": "List registered models"}, "post": map[string]any{"summary": "Register a model"}},
			"/api/v1/models/{id}/promote":  map[string]any{"post": map[string]any{"summary": "Promote a model"}},
			"/api/v1/agents":               map[string]any{"get": map[string]any{"summary": "List agents"}, "post": map[string]any{"summary": "Deploy an agent"}},
			"/api/v1/tools":                map[string]any{"get": map[string]any{"summary": "List tools"}, "post": map[string]any{"summary": "Register a tool"}},
			"/api/v1/connections":          map[string]any{"get": map[string]any{"summary": "List connections"}, "post": map[string]any{"summary": "Create a secret-backed connection"}},
			"/api/v1/audit":                map[string]any{"get": map[string]any{"summary": "List audit events"}},
			"/api/v1/traces":               map[string]any{"post": map[string]any{"summary": "Record an agent trace"}},
		},
	})
}
