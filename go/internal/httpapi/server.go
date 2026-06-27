package httpapi

import (
	"encoding/json"
	"io/fs"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/mlaiops/platform/internal/platform"
	"github.com/mlaiops/platform/internal/store"
	"github.com/mlaiops/platform/pkg/api"
)

type Server struct {
	store  *store.Store
	static fs.FS
}

func New(data *store.Store, static fs.FS) http.Handler {
	server := &Server{store: data, static: static}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/health", server.health)
	mux.HandleFunc("GET /api/v1/dashboard", server.dashboard)
	mux.HandleFunc("GET /api/v1/projects", server.projects)
	mux.HandleFunc("POST /api/v1/projects", server.createProject)
	mux.HandleFunc("GET /api/v1/pipelines/runs", server.runs)
	mux.HandleFunc("POST /api/v1/pipelines/submit", server.submitPipeline)
	mux.HandleFunc("GET /api/v1/components", server.components)
	mux.HandleFunc("GET /api/v1/catalog", server.catalog)
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
		RecentRuns: runs, OnboardingPct: 75,
	})
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
	project, err := s.store.CreateProject(req)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "validation_error", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, project)
}

func (s *Server) runs(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.store.Runs())
}

func (s *Server) submitPipeline(w http.ResponseWriter, r *http.Request) {
	var req api.SubmitPipelineRequest
	if err := decode(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	run, err := s.store.SubmitPipeline(req)
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

func decode(r *http.Request, target any) error {
	decoder := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<20))
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
