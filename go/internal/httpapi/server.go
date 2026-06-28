package httpapi

import (
	"encoding/json"
	"errors"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/mlaiops/platform/internal/auth"
	"github.com/mlaiops/platform/internal/platform"
	"github.com/mlaiops/platform/internal/store"
	"github.com/mlaiops/platform/pkg/api"
)

type Server struct {
	store  store.Repository
	static fs.FS
}

func New(data store.Repository, static fs.FS) http.Handler {
	server := &Server{store: data, static: static}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/health", server.health)
	mux.HandleFunc("GET /api/v1/dashboard", server.dashboard)
	mux.HandleFunc("GET /api/v1/onboarding/readiness", server.readiness)
	mux.HandleFunc("GET /api/v1/projects", server.projects)
	mux.HandleFunc("POST /api/v1/projects", server.createProject)
	mux.HandleFunc("GET /api/v1/pipelines/runs", server.runs)
	mux.HandleFunc("POST /api/v1/pipelines/submit", server.submitPipeline)
	mux.HandleFunc("GET /api/v1/pipelines/runs/{id}", server.run)
	mux.HandleFunc("POST /api/v1/pipelines/runs/{id}/cancel", server.cancelRun)
	mux.HandleFunc("POST /api/v1/pipelines/runs/{id}/retry", server.retryRun)
	mux.HandleFunc("GET /api/v1/components", server.components)
	mux.HandleFunc("GET /api/v1/catalog", server.catalog)
	mux.HandleFunc("GET /api/v1/models", server.models)
	mux.HandleFunc("POST /api/v1/models", server.registerModel)
	mux.HandleFunc("POST /api/v1/models/{id}/promote", server.promoteModel)
	mux.HandleFunc("POST /api/v1/models/{id}/deploy", server.deployModel)
	mux.HandleFunc("POST /api/v1/models/{id}/rollback", server.rollbackModel)
	mux.HandleFunc("GET /api/v1/agents", server.agents)
	mux.HandleFunc("POST /api/v1/agents", server.deployAgent)
	mux.HandleFunc("PUT /api/v1/agents/{id}/traffic", server.agentTraffic)
	mux.HandleFunc("GET /api/v1/agents/{id}/sessions", server.agentSessions)
	mux.HandleFunc("GET /api/v1/agents/{id}/traces", server.agentTraces)
	mux.HandleFunc("GET /api/v1/agents/{id}/usage", server.agentUsage)
	mux.HandleFunc("POST /api/v1/traces", server.recordTrace)
	mux.HandleFunc("GET /api/v1/tools", server.tools)
	mux.HandleFunc("POST /api/v1/tools", server.registerTool)
	mux.HandleFunc("GET /api/v1/connections", server.connections)
	mux.HandleFunc("POST /api/v1/connections", server.createConnection)
	mux.HandleFunc("POST /api/v1/connections/{id}/test", server.testConnection)
	mux.HandleFunc("GET /api/v1/audit", server.audit)
	mux.HandleFunc("GET /api/openapi.json", server.openapi)
	mux.Handle("/", http.FileServer(http.FS(static)))
	return logging(cors(mux))
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "service": "mlaiops-gateway", "version": "0.1.0"})
}

func (s *Server) dashboard(w http.ResponseWriter, _ *http.Request) {
	projects, runs, components := s.store.Projects(), s.store.Runs(), platform.Components()
	active, healthy := 0, 0
	for _, run := range runs {
		if run.Status == "running" || run.Status == "queued" {
			active++
		}
	}
	for _, component := range components {
		if component.Status == "healthy" {
			healthy++
		}
	}
	if len(runs) > 5 {
		runs = runs[:5]
	}
	writeJSON(w, http.StatusOK, api.Dashboard{
		Projects: len(projects), ActiveRuns: active, Healthy: healthy, Total: len(components),
		RecentRuns: runs, OnboardingPct: readinessFor(s.store).Percent,
	})
}

func (s *Server) readiness(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, readinessFor(s.store))
}

func readinessFor(repository store.Repository) api.Readiness {
	connections := repository.Connections()
	healthy := map[string]bool{}
	for _, connection := range connections {
		if connection.Status == "healthy" {
			healthy[connection.Type] = true
		}
	}
	items := []api.ReadinessItem{
		{Key: "workspace", Label: "Create a workspace", Status: choose(len(repository.Projects()) > 0, "ready", "pending"), Description: "A project namespace and defaults are available.", Action: "projects"},
		{Key: "kubernetes", Label: "Connect Kubernetes", Status: choose(healthy["kubernetes"], "ready", "pending"), Description: "Required to schedule pipelines and serving workloads.", Action: "platform"},
		{Key: "tracking", Label: "Connect MLflow", Status: choose(healthy["mlflow"], "ready", "pending"), Description: "Tracks experiments, artifacts and model versions.", Action: "platform"},
		{Key: "storage", Label: "Connect object storage", Status: choose(healthy["s3"] || healthy["minio"], "ready", "pending"), Description: "Stores datasets, models and pipeline artifacts.", Action: "platform"},
		{Key: "events", Label: "Connect Kafka", Status: choose(healthy["kafka"], "ready", "pending"), Description: "Carries durable lifecycle and audit events.", Action: "platform"},
	}
	ready := 0
	for _, item := range items {
		if item.Status == "ready" {
			ready++
		}
	}
	percent := ready * 100 / len(items)
	return api.Readiness{Percent: percent, Ready: ready == len(items), Items: items}
}
func choose(condition bool, yes, no string) string {
	if condition {
		return yes
	}
	return no
}

func (s *Server) projects(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.store.Projects())
}

func (s *Server) createProject(w http.ResponseWriter, r *http.Request) {
	var req api.CreateProjectRequest
	if err := decode(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	project, err := s.store.CreateProject(req, actor(r))
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "validation_error", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, project)
}

func (s *Server) runs(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.store.Runs())
}
func (s *Server) run(w http.ResponseWriter, r *http.Request) {
	item, err := s.store.Run(r.PathValue("id"))
	writeMutation(w, item, err, http.StatusOK)
}
func (s *Server) cancelRun(w http.ResponseWriter, r *http.Request) {
	item, err := s.store.CancelRun(r.PathValue("id"), actor(r))
	writeMutation(w, item, err, http.StatusOK)
}
func (s *Server) retryRun(w http.ResponseWriter, r *http.Request) {
	item, err := s.store.RetryRun(r.PathValue("id"), actor(r))
	writeMutation(w, item, err, http.StatusAccepted)
}

func (s *Server) submitPipeline(w http.ResponseWriter, r *http.Request) {
	var req api.SubmitPipelineRequest
	if err := decode(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	run, err := s.store.SubmitPipeline(req, actor(r))
	if err != nil {
		status := http.StatusUnprocessableEntity
		if err == store.ErrNotFound {
			status = http.StatusNotFound
		}
		writeError(w, status, "pipeline_submission_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, run)
}

func (s *Server) components(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, platform.Components())
}

func (s *Server) catalog(w http.ResponseWriter, r *http.Request) {
	items, kind := platform.Catalog(), r.URL.Query().Get("kind")
	if kind == "" {
		writeJSON(w, http.StatusOK, items)
		return
	}
	filtered := make([]api.CatalogItem, 0)
	for _, item := range items {
		if item.Kind == kind {
			filtered = append(filtered, item)
		}
	}
	writeJSON(w, http.StatusOK, filtered)
}

func (s *Server) models(w http.ResponseWriter, _ *http.Request) {
	items := s.store.Models()
	writeJSON(w, http.StatusOK, api.Page[api.Model]{Items: items, Total: len(items)})
}

func (s *Server) registerModel(w http.ResponseWriter, r *http.Request) {
	var req api.RegisterModelRequest
	if err := decode(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	item, err := s.store.RegisterModel(req, actor(r))
	writeMutation(w, item, err, http.StatusCreated)
}

func (s *Server) promoteModel(w http.ResponseWriter, r *http.Request) {
	var req api.PromoteModelRequest
	if err := decode(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	item, err := s.store.PromoteModel(r.PathValue("id"), req.Stage, actor(r))
	writeMutation(w, item, err, http.StatusOK)
}
func (s *Server) deployModel(w http.ResponseWriter, r *http.Request) {
	var req api.DeployModelRequest
	if err := decode(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	item, err := s.store.DeployModel(r.PathValue("id"), req.CanaryWeight, actor(r))
	writeMutation(w, item, err, http.StatusAccepted)
}
func (s *Server) rollbackModel(w http.ResponseWriter, r *http.Request) {
	item, err := s.store.RollbackModel(r.PathValue("id"), actor(r))
	writeMutation(w, item, err, http.StatusOK)
}

func (s *Server) agents(w http.ResponseWriter, _ *http.Request) {
	items := s.store.Agents()
	writeJSON(w, http.StatusOK, api.Page[api.Agent]{Items: items, Total: len(items)})
}

func (s *Server) deployAgent(w http.ResponseWriter, r *http.Request) {
	var req api.DeployAgentRequest
	if err := decode(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	item, err := s.store.DeployAgent(req, actor(r))
	writeMutation(w, item, err, http.StatusAccepted)
}

func (s *Server) agentTraffic(w http.ResponseWriter, r *http.Request) {
	var req api.TrafficRequest
	if err := decode(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	item, err := s.store.SetAgentTraffic(r.PathValue("id"), req.CanaryWeight, actor(r))
	writeMutation(w, item, err, http.StatusOK)
}
func (s *Server) agentSessions(w http.ResponseWriter, r *http.Request) {
	items := s.store.AgentSessions(r.PathValue("id"))
	writeJSON(w, http.StatusOK, api.Page[api.AgentSession]{Items: items, Total: len(items)})
}
func (s *Server) agentTraces(w http.ResponseWriter, r *http.Request) {
	items := s.store.AgentTraces(r.PathValue("id"))
	writeJSON(w, http.StatusOK, api.Page[api.AgentTrace]{Items: items, Total: len(items)})
}
func (s *Server) agentUsage(w http.ResponseWriter, r *http.Request) {
	sessions := s.store.AgentSessions(r.PathValue("id"))
	usage := api.AgentUsage{Sessions: len(sessions)}
	for _, session := range sessions {
		if session.Status == "running" {
			usage.Active++
		}
		usage.InputTokens += session.InputTokens
		usage.OutputTokens += session.OutputTokens
		usage.CostUSD += session.CostUSD
	}
	writeJSON(w, http.StatusOK, usage)
}
func (s *Server) recordTrace(w http.ResponseWriter, r *http.Request) {
	var req api.RecordTraceRequest
	if err := decode(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	item, err := s.store.RecordTrace(req)
	writeMutation(w, item, err, http.StatusCreated)
}

func (s *Server) tools(w http.ResponseWriter, _ *http.Request) {
	items := s.store.Tools()
	writeJSON(w, http.StatusOK, api.Page[api.Tool]{Items: items, Total: len(items)})
}

func (s *Server) registerTool(w http.ResponseWriter, r *http.Request) {
	var req api.RegisterToolRequest
	if err := decode(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	item, err := s.store.RegisterTool(req, actor(r))
	writeMutation(w, item, err, http.StatusCreated)
}

func (s *Server) connections(w http.ResponseWriter, _ *http.Request) {
	items := s.store.Connections()
	writeJSON(w, http.StatusOK, api.Page[api.Connection]{Items: items, Total: len(items)})
}

func (s *Server) createConnection(w http.ResponseWriter, r *http.Request) {
	var req api.CreateConnectionRequest
	if err := decode(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	item, err := s.store.CreateConnection(req, actor(r))
	writeMutation(w, item, err, http.StatusCreated)
}
func (s *Server) testConnection(w http.ResponseWriter, r *http.Request) {
	var connection *api.Connection
	for _, item := range s.store.Connections() {
		if item.ID == r.PathValue("id") {
			value := item
			connection = &value
			break
		}
	}
	if connection == nil {
		writeError(w, http.StatusNotFound, "not_found", "connection not found")
		return
	}
	target, err := url.Parse(connection.Endpoint)
	if err != nil || (target.Scheme != "http" && target.Scheme != "https") || target.Host == "" {
		item, updateErr := s.store.UpdateConnectionStatus(connection.ID, "unhealthy", "Endpoint must be an HTTP or HTTPS URL", actor(r))
		if updateErr != nil {
			writeMutation(w, item, updateErr, http.StatusUnprocessableEntity)
			return
		}
		writeJSON(w, http.StatusOK, item)
		return
	}
	client := &http.Client{Timeout: 4 * time.Second}
	request, _ := http.NewRequestWithContext(r.Context(), http.MethodGet, target.String(), nil)
	response, checkErr := client.Do(request)
	status, message := "healthy", "Connection succeeded"
	if checkErr != nil {
		status, message = "unhealthy", checkErr.Error()
	} else {
		_ = response.Body.Close()
		if response.StatusCode >= 500 {
			status, message = "unhealthy", response.Status
		} else {
			message = response.Status
		}
	}
	item, updateErr := s.store.UpdateConnectionStatus(connection.ID, status, message, actor(r))
	writeMutation(w, item, updateErr, http.StatusOK)
}

func (s *Server) audit(w http.ResponseWriter, _ *http.Request) {
	items := s.store.Audit()
	writeJSON(w, http.StatusOK, api.Page[api.AuditEvent]{Items: items, Total: len(items)})
}

func writeMutation[T any](w http.ResponseWriter, value T, err error, success int) {
	if err == nil {
		writeJSON(w, success, value)
		return
	}
	status, code := http.StatusUnprocessableEntity, "validation_error"
	if errors.Is(err, store.ErrNotFound) {
		status, code = http.StatusNotFound, "not_found"
	}
	if errors.Is(err, store.ErrConflict) {
		status, code = http.StatusConflict, "conflict"
	}
	writeError(w, status, code, err.Error())
}

func actor(r *http.Request) string {
	if principal, ok := auth.PrincipalFrom(r.Context()); ok {
		if principal.Email != "" {
			return principal.Email
		}
		return principal.Subject
	}
	if value := strings.TrimSpace(r.Header.Get("X-MLAIOps-Actor")); value != "" {
		return value
	}
	return "anonymous"
}

func decode(r *http.Request, target any) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, api.APIError{Error: code, Message: message})
}

func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		if strings.HasPrefix(r.URL.Path, "/api/") {
			log.Printf("method=%s path=%s duration=%s", r.Method, r.URL.Path, time.Since(start))
		}
	})
}
