package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/mlaiops/platform/internal/auth"
	"github.com/mlaiops/platform/internal/integrations"
	"github.com/mlaiops/platform/internal/platform"
	"github.com/mlaiops/platform/internal/store"
	"github.com/mlaiops/platform/pkg/api"
)

type Server struct {
	store  store.Repository
	static fs.FS

	realtimeMu sync.RWMutex
	realtime   map[string]map[string]any
}

func New(data store.Repository, static fs.FS) http.Handler {
	server := &Server{store: data, static: static, realtime: map[string]map[string]any{}}
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
	mux.HandleFunc("POST /api/v1/pipelines/runs/{id}/steps", server.updateRunStep)
	mux.HandleFunc("GET /api/v1/components", server.components)
	mux.HandleFunc("GET /api/v1/catalog", server.catalog)
	mux.HandleFunc("GET /api/v1/features", server.features)
	mux.HandleFunc("POST /api/v1/features", server.applyFeatureView)
	mux.HandleFunc("POST /api/v1/features/{name}/materialized", server.reportMaterialization)
	mux.HandleFunc("GET /api/v1/storage/buckets", server.storageProxy("/buckets"))
	mux.HandleFunc("GET /api/v1/storage/objects", server.storageProxy("/objects"))
	mux.HandleFunc("GET /api/v1/storage/object", server.storageProxy("/object"))
	mux.HandleFunc("GET /api/v1/prompts", server.prompts)
	mux.HandleFunc("GET /api/v1/events", server.events)
	mux.HandleFunc("GET /api/v1/realtime", server.realtimeStats)
	mux.HandleFunc("POST /api/v1/realtime/{demo}", server.reportRealtime)
	mux.HandleFunc("GET /api/v1/functions", server.functions)
	mux.HandleFunc("POST /api/v1/functions", server.deployFunction)
	mux.HandleFunc("POST /api/v1/functions/{name}/invoke", server.invokeFunction)
	mux.HandleFunc("GET /api/v1/models", server.models)
	mux.HandleFunc("POST /api/v1/models", server.registerModel)
	mux.HandleFunc("POST /api/v1/models/{id}/promote", server.promoteModel)
	mux.HandleFunc("POST /api/v1/models/{id}/deploy", server.deployModel)
	mux.HandleFunc("POST /api/v1/models/{id}/rollback", server.rollbackModel)
	mux.HandleFunc("POST /api/v1/models/{id}/predict", server.predictModel)
	mux.HandleFunc("GET /api/v1/agents", server.agents)
	mux.HandleFunc("POST /api/v1/agents", server.deployAgent)
	mux.HandleFunc("PUT /api/v1/agents/{id}/traffic", server.agentTraffic)
	mux.HandleFunc("POST /api/v1/agents/{id}/invoke", server.invokeAgent)
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
	projects, runs, components := s.store.Projects(), s.store.Runs(), platform.Components(s.store.Connections())
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
	if err == nil && item.EngineRunID != "" && os.Getenv("PREFECT_API_URL") != "" {
		// Best effort: the control-plane cancellation is authoritative; the
		// engine cancellation stops the actual execution.
		prefect := integrations.NewPrefect(os.Getenv("PREFECT_API_URL"), "")
		if cancelErr := prefect.CancelFlowRun(r.Context(), item.EngineRunID); cancelErr != nil {
			log.Printf("prefect cancel failed for run %s (%s): %v", item.ID, item.EngineRunID, cancelErr)
		}
	}
	writeMutation(w, item, err, http.StatusOK)
}

// updateRunStep receives step transitions from the executing pipeline itself
// (the SDK step reporter inside each flow).
func (s *Server) updateRunStep(w http.ResponseWriter, r *http.Request) {
	var req api.UpdateRunStepRequest
	if err := decode(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	item, err := s.store.UpdateRunStep(r.PathValue("id"), req, actor(r))
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
	// With an execution engine configured, the run is real: create the Prefect
	// flow run carrying our run id so the flow reports steps back. Fail closed
	// (run marked failed) when the engine rejects the submission.
	if prefectURL := os.Getenv("PREFECT_API_URL"); prefectURL != "" {
		prefect := integrations.NewPrefect(prefectURL, "")
		engineID, submitErr := prefect.CreateFlowRun(r.Context(), run.Name, "mlaiops", run.ID, map[string]any{"run_id": run.ID, "project_id": run.ProjectID})
		if submitErr != nil {
			run, _ = s.store.UpdateRunStep(run.ID, api.UpdateRunStepRequest{Step: "submit-to-engine", Status: "failed", Message: submitErr.Error()}, "system")
			writeJSON(w, http.StatusAccepted, run)
			return
		}
		if linked, linkErr := s.store.SetRunEngine(run.ID, engineID); linkErr == nil {
			run = linked
		}
	}
	writeJSON(w, http.StatusAccepted, run)
}

func (s *Server) components(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, platform.Components(s.store.Connections()))
}

func (s *Server) catalog(w http.ResponseWriter, r *http.Request) {
	items, kind := platform.Catalog(s.store), r.URL.Query().Get("kind")
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

func (s *Server) features(w http.ResponseWriter, _ *http.Request) {
	items := s.store.FeatureViews()
	writeJSON(w, http.StatusOK, api.Page[api.FeatureView]{Items: items, Total: len(items)})
}

func (s *Server) applyFeatureView(w http.ResponseWriter, r *http.Request) {
	var req api.ApplyFeatureViewRequest
	if err := decode(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	item, err := s.store.ApplyFeatureView(req, actor(r))
	writeMutation(w, item, err, http.StatusCreated)
}

func (s *Server) reportMaterialization(w http.ResponseWriter, r *http.Request) {
	var req api.MaterializationReport
	if err := decode(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	item, err := s.store.ReportMaterialization(r.PathValue("name"), req.EntityCount, actor(r))
	writeMutation(w, item, err, http.StatusOK)
}

// storageProxy forwards Storage Explorer reads to the storage proxy, which is
// the sole holder of object-store credentials (PRD section 5.2.5).
func (s *Server) storageProxy(path string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		base := os.Getenv("STORAGE_PROXY_URL")
		if base == "" {
			base = "http://localhost:8084"
		}
		target := strings.TrimRight(base, "/") + path
		if r.URL.RawQuery != "" {
			target += "?" + r.URL.RawQuery
		}
		request, err := http.NewRequestWithContext(r.Context(), http.MethodGet, target, nil)
		if err != nil {
			writeError(w, http.StatusBadGateway, "storage_unreachable", err.Error())
			return
		}
		if token := os.Getenv("MLAIOPS_INTERNAL_TOKEN"); token != "" {
			request.Header.Set("Authorization", "Bearer "+token)
		}
		client := &http.Client{Timeout: 20 * time.Second}
		response, err := client.Do(request)
		if err != nil {
			writeError(w, http.StatusBadGateway, "storage_unreachable", err.Error())
			return
		}
		defer func() { _ = response.Body.Close() }()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(response.StatusCode)
		_, _ = io.Copy(w, response.Body)
	}
}

// prompts proxies Langfuse prompt management for the console's Prompt
// Library. Reports configured=false honestly instead of inventing data.
func (s *Server) prompts(w http.ResponseWriter, r *http.Request) {
	base := os.Getenv("LANGFUSE_URL")
	public := os.Getenv("LANGFUSE_PUBLIC_KEY")
	secret := os.Getenv("LANGFUSE_SECRET_KEY")
	if base == "" || public == "" || secret == "" {
		writeJSON(w, http.StatusOK, map[string]any{"configured": false, "items": []any{}})
		return
	}
	request, err := http.NewRequestWithContext(r.Context(), http.MethodGet, strings.TrimRight(base, "/")+"/api/public/v2/prompts", nil)
	if err != nil {
		writeError(w, http.StatusBadGateway, "langfuse_unreachable", err.Error())
		return
	}
	request.SetBasicAuth(public, secret)
	client := &http.Client{Timeout: 10 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		writeError(w, http.StatusBadGateway, "langfuse_unreachable", err.Error())
		return
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		writeError(w, http.StatusBadGateway, "langfuse_error", response.Status)
		return
	}
	var payload struct {
		Data []map[string]any `json:"data"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadGateway, "langfuse_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"configured": true, "items": payload.Data})
}

// events streams a live state digest as Server-Sent Events so the console
// updates without polling storms. The digest carries enough for the client to
// know *what* changed; panels re-fetch their own data when it does.
func (s *Server) events(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming_unsupported", "response writer cannot stream")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	send := func() bool {
		digest := s.digest()
		raw, err := json.Marshal(digest)
		if err != nil {
			return false
		}
		if _, err := fmt.Fprintf(w, "data: %s\n\n", raw); err != nil {
			return false
		}
		flusher.Flush()
		return true
	}
	if !send() {
		return
	}
	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			if !send() {
				return
			}
		}
	}
}

// digest summarizes mutable state cheaply; identical digests mean no refresh.
func (s *Server) digest() map[string]any {
	runs := s.store.Runs()
	latestRun := ""
	active := 0
	for _, run := range runs {
		if run.UpdatedAt.Format(time.RFC3339Nano) > latestRun {
			latestRun = run.UpdatedAt.Format(time.RFC3339Nano)
		}
		if run.Status == "running" || run.Status == "queued" {
			active++
		}
	}
	sessions := s.store.AgentSessions("")
	latestSession := ""
	for _, session := range sessions {
		if session.UpdatedAt.Format(time.RFC3339Nano) > latestSession {
			latestSession = session.UpdatedAt.Format(time.RFC3339Nano)
		}
	}
	s.realtimeMu.RLock()
	realtimeEvents := 0.0
	for _, stats := range s.realtime {
		if events, ok := stats["events"].(float64); ok {
			realtimeEvents += events
		}
	}
	s.realtimeMu.RUnlock()
	return map[string]any{
		"runs":            len(runs),
		"active_runs":     active,
		"latest_run":      latestRun,
		"sessions":        len(sessions),
		"latest_session":  latestSession,
		"models":          len(s.store.Models()),
		"agents":          len(s.store.Agents()),
		"connections":     len(s.store.Connections()),
		"features":        len(s.store.FeatureViews()),
		"realtime_events": realtimeEvents,
	}
}

// realtimeStats exposes the live stream-processing statistics reported by the
// realtime consumer service. In-memory by design: these are live gauges, not
// durable state.
func (s *Server) realtimeStats(w http.ResponseWriter, _ *http.Request) {
	s.realtimeMu.RLock()
	defer s.realtimeMu.RUnlock()
	writeJSON(w, http.StatusOK, map[string]any{"demos": s.realtime})
}

func (s *Server) reportRealtime(w http.ResponseWriter, r *http.Request) {
	demo := r.PathValue("demo")
	var payload map[string]any
	if err := decode(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	payload["updated_at"] = time.Now().UTC().Format(time.RFC3339)
	s.realtimeMu.Lock()
	s.realtime[demo] = payload
	s.realtimeMu.Unlock()
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// openfaas builds the serverless client from the environment; nil when the
// integration is not configured.
func openfaas() *integrations.OpenFaaS {
	base := os.Getenv("OPENFAAS_URL")
	if base == "" {
		return nil
	}
	client := integrations.NewOpenFaaS(base, os.Getenv("OPENFAAS_USER"), os.Getenv("OPENFAAS_PASSWORD"))
	return &client
}

func (s *Server) functions(w http.ResponseWriter, r *http.Request) {
	client := openfaas()
	if client == nil {
		writeJSON(w, http.StatusOK, map[string]any{"configured": false, "items": []any{}})
		return
	}
	items, err := client.ListFunctions(r.Context())
	if err != nil {
		writeError(w, http.StatusBadGateway, "openfaas_unreachable", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"configured": true, "items": items, "total": len(items)})
}

func (s *Server) deployFunction(w http.ResponseWriter, r *http.Request) {
	client := openfaas()
	if client == nil {
		writeError(w, http.StatusConflict, "not_configured", "OPENFAAS_URL is not configured")
		return
	}
	var req integrations.Function
	if err := decode(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	if err := client.DeployFunction(r.Context(), req); err != nil {
		writeError(w, http.StatusBadGateway, "deploy_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, req)
}

func (s *Server) invokeFunction(w http.ResponseWriter, r *http.Request) {
	client := openfaas()
	if client == nil {
		writeError(w, http.StatusConflict, "not_configured", "OPENFAAS_URL is not configured")
		return
	}
	payload, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	result, err := client.Invoke(r.Context(), r.PathValue("name"), payload)
	if err != nil {
		writeError(w, http.StatusBadGateway, "invoke_failed", err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(result)
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
	if err != nil {
		writeMutation(w, item, err, http.StatusAccepted)
		return
	}
	// With a serving manager configured, deployment is real: an mlflow serve
	// container starts and the live endpoint is recorded. Fail closed when the
	// serving manager rejects it.
	if managerURL := os.Getenv("SERVING_MANAGER_URL"); managerURL != "" {
		endpoint, deployErr := s.requestServing(r, managerURL, item)
		if deployErr != nil {
			item, _ = s.store.SetModelEndpoint(item.ID, "", "failed")
			writeError(w, http.StatusBadGateway, "serving_failed", deployErr.Error())
			return
		}
		if updated, setErr := s.store.SetModelEndpoint(item.ID, endpoint, "serving"); setErr == nil {
			item = updated
		}
	}
	writeJSON(w, http.StatusAccepted, item)
}

func (s *Server) requestServing(r *http.Request, managerURL string, model api.Model) (string, error) {
	body, _ := json.Marshal(map[string]string{"name": model.Name, "artifact_uri": model.ArtifactURI})
	request, err := http.NewRequestWithContext(r.Context(), http.MethodPost, strings.TrimRight(managerURL, "/")+"/deployments", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	request.Header.Set("Content-Type", "application/json")
	if token := os.Getenv("MLAIOPS_INTERNAL_TOKEN"); token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	client := &http.Client{Timeout: 90 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		return "", err
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode >= 300 {
		raw, _ := io.ReadAll(io.LimitReader(response.Body, 2048))
		return "", errors.New("serving manager: " + strings.TrimSpace(string(raw)))
	}
	var payload struct {
		Endpoint string `json:"endpoint"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return "", err
	}
	return payload.Endpoint, nil
}

func (s *Server) rollbackModel(w http.ResponseWriter, r *http.Request) {
	item, err := s.store.RollbackModel(r.PathValue("id"), actor(r))
	if err == nil && os.Getenv("SERVING_MANAGER_URL") != "" {
		// Best effort: remove the serving container for the rolled-back model.
		managerURL := strings.TrimRight(os.Getenv("SERVING_MANAGER_URL"), "/")
		request, buildErr := http.NewRequestWithContext(r.Context(), http.MethodDelete, managerURL+"/deployments/"+url.PathEscape(item.Name), nil)
		if buildErr == nil {
			if token := os.Getenv("MLAIOPS_INTERNAL_TOKEN"); token != "" {
				request.Header.Set("Authorization", "Bearer "+token)
			}
			client := &http.Client{Timeout: 30 * time.Second}
			if response, doErr := client.Do(request); doErr == nil {
				_ = response.Body.Close()
			} else {
				log.Printf("serving rollback undeploy failed for %s: %v", item.Name, doErr)
			}
		}
	}
	writeMutation(w, item, err, http.StatusOK)
}

// predictModel proxies a console test request to the model's live serving
// endpoint (mlflow serve /invocations contract).
func (s *Server) predictModel(w http.ResponseWriter, r *http.Request) {
	var model *api.Model
	for _, item := range s.store.Models() {
		if item.ID == r.PathValue("id") {
			value := item
			model = &value
			break
		}
	}
	if model == nil {
		writeError(w, http.StatusNotFound, "not_found", "model not found")
		return
	}
	if model.EndpointURL == "" || !strings.HasPrefix(model.EndpointURL, "http") {
		writeError(w, http.StatusConflict, "not_serving", "model has no live serving endpoint")
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	request, err := http.NewRequestWithContext(r.Context(), http.MethodPost, strings.TrimRight(model.EndpointURL, "/")+"/invocations", bytes.NewReader(body))
	if err != nil {
		writeError(w, http.StatusBadGateway, "endpoint_unreachable", err.Error())
		return
	}
	request.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 30 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		writeError(w, http.StatusBadGateway, "endpoint_unreachable", err.Error())
		return
	}
	defer func() { _ = response.Body.Close() }()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(response.StatusCode)
	_, _ = io.Copy(w, response.Body)
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

// invokeAgent proxies a test-console or SDK turn to the agent runtime, which
// executes the LangGraph graph and reports the session back through
// POST /api/v1/traces. The runtime address comes from AGENT_RUNTIME_URL.
func (s *Server) invokeAgent(w http.ResponseWriter, r *http.Request) {
	agentID := r.PathValue("id")
	var agent *api.Agent
	for _, item := range s.store.Agents() {
		if item.ID == agentID {
			value := item
			agent = &value
			break
		}
	}
	if agent == nil {
		writeError(w, http.StatusNotFound, "not_found", "agent not found")
		return
	}
	var req api.InvokeAgentRequest
	if err := decode(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	if strings.TrimSpace(req.Message) == "" {
		writeError(w, http.StatusUnprocessableEntity, "validation_error", "message is required")
		return
	}
	runtime := os.Getenv("AGENT_RUNTIME_URL")
	if runtime == "" {
		runtime = "http://localhost:9000"
	}
	body, _ := json.Marshal(req)
	request, err := http.NewRequestWithContext(r.Context(), http.MethodPost, strings.TrimRight(runtime, "/")+"/invoke", bytes.NewReader(body))
	if err != nil {
		writeError(w, http.StatusBadGateway, "runtime_unreachable", err.Error())
		return
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-MLAIOps-Agent-ID", agent.ID)
	request.Header.Set("X-MLAIOps-Agent-Name", agent.Name)
	client := &http.Client{Timeout: 120 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		writeError(w, http.StatusBadGateway, "runtime_unreachable", err.Error())
		return
	}
	defer func() { _ = response.Body.Close() }()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(response.StatusCode)
	_, _ = io.Copy(w, response.Body)
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

// cors reflects the configured origin. MLAIOPS_ALLOWED_ORIGIN pins the
// console origin in public deployments; unset means local development where
// any origin is fine.
func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := os.Getenv("MLAIOPS_ALLOWED_ORIGIN")
		if origin == "" {
			origin = "*"
		}
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if origin != "*" {
			w.Header().Set("Vary", "Origin")
		}
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
