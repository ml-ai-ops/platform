package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func createTestProject(t *testing.T, server http.Handler) string {
	t.Helper()
	create := httptest.NewRequest(http.MethodPost, "/api/v1/projects", strings.NewReader(`{"name":"Engine project"}`))
	created := httptest.NewRecorder()
	server.ServeHTTP(created, create)
	if created.Code != http.StatusCreated {
		t.Fatalf("project create failed: %d", created.Code)
	}
	return strings.Split(strings.Split(created.Body.String(), `"id":"`)[1], `"`)[0]
}

func TestSubmitPipelineCreatesPrefectFlowRun(t *testing.T) {
	prefect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/api/deployments/name/"):
			_, _ = w.Write([]byte(`{"id":"dep-1"}`))
		case r.URL.Path == "/api/deployments/dep-1/create_flow_run":
			_, _ = w.Write([]byte(`{"id":"fr-77"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer prefect.Close()
	t.Setenv("PREFECT_API_URL", prefect.URL)

	server := testServer()
	projectID := createTestProject(t, server)
	submit := httptest.NewRequest(http.MethodPost, "/api/v1/pipelines/submit", strings.NewReader(`{"project_id":"`+projectID+`","name":"training-pipeline"}`))
	submitted := httptest.NewRecorder()
	server.ServeHTTP(submitted, submit)
	if submitted.Code != http.StatusAccepted || !strings.Contains(submitted.Body.String(), `"engine_run_id":"fr-77"`) {
		t.Fatalf("engine run not linked: %d %s", submitted.Code, submitted.Body.String())
	}
}

func TestSubmitPipelineFailsClosedWhenEngineRejects(t *testing.T) {
	prefect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer prefect.Close()
	t.Setenv("PREFECT_API_URL", prefect.URL)

	server := testServer()
	projectID := createTestProject(t, server)
	submit := httptest.NewRequest(http.MethodPost, "/api/v1/pipelines/submit", strings.NewReader(`{"project_id":"`+projectID+`","name":"training-pipeline"}`))
	submitted := httptest.NewRecorder()
	server.ServeHTTP(submitted, submit)
	if submitted.Code != http.StatusAccepted || !strings.Contains(submitted.Body.String(), `"status":"failed"`) {
		t.Fatalf("engine rejection must fail the run visibly: %d %s", submitted.Code, submitted.Body.String())
	}
}

func TestStepReportEndpoint(t *testing.T) {
	server := testServer()
	projectID := createTestProject(t, server)
	submit := httptest.NewRequest(http.MethodPost, "/api/v1/pipelines/submit", strings.NewReader(`{"project_id":"`+projectID+`"}`))
	submitted := httptest.NewRecorder()
	server.ServeHTTP(submitted, submit)
	runID := strings.Split(strings.Split(submitted.Body.String(), `"id":"`)[1], `"`)[0]

	report := httptest.NewRequest(http.MethodPost, "/api/v1/pipelines/runs/"+runID+"/steps", strings.NewReader(`{"step":"validate-data","status":"running","message":"loading rows"}`))
	reported := httptest.NewRecorder()
	server.ServeHTTP(reported, report)
	if reported.Code != http.StatusOK || !strings.Contains(reported.Body.String(), `"status":"running"`) {
		t.Fatalf("step report failed: %d %s", reported.Code, reported.Body.String())
	}
}
