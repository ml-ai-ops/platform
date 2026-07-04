package httpapi

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/ml-ai-ops/platform/internal/store"
)

func testServer() http.Handler {
	static, _ := fs.Sub(fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("ok")}}, ".")
	return New(store.New(), static)
}

func TestHealth(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	response := httptest.NewRecorder()
	testServer().ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"status":"ok"`) {
		t.Fatalf("unexpected response: %d %s", response.Code, response.Body.String())
	}
}

func TestLandingAndConsoleRoutes(t *testing.T) {
	static, _ := fs.Sub(fstest.MapFS{
		"index.html":   &fstest.MapFile{Data: []byte("<h1>Nexus landing</h1>")},
		"console.html": &fstest.MapFile{Data: []byte("<h1>Nexus console</h1>")},
	}, ".")
	server := New(store.New(), static)
	for _, test := range []struct {
		path, expected string
	}{
		{"/", "Nexus landing"},
		{"/console.html", "Nexus console"},
	} {
		response := httptest.NewRecorder()
		server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, test.path, nil))
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), test.expected) {
			t.Fatalf("GET %s: %d %s", test.path, response.Code, response.Body.String())
		}
	}
}

func TestCreateProjectAndSubmitPipeline(t *testing.T) {
	server := testServer()
	create := httptest.NewRequest(http.MethodPost, "/api/v1/projects", strings.NewReader(`{"name":"Churn model","template":"tabular-classification"}`))
	created := httptest.NewRecorder()
	server.ServeHTTP(created, create)
	if created.Code != http.StatusCreated || !strings.Contains(created.Body.String(), `"namespace":"churn-model"`) {
		t.Fatalf("unexpected create response: %d %s", created.Code, created.Body.String())
	}
	projectID := strings.Split(strings.Split(created.Body.String(), `"id":"`)[1], `"`)[0]
	submit := httptest.NewRequest(http.MethodPost, "/api/v1/pipelines/submit", strings.NewReader(`{"project_id":"`+projectID+`","name":"train"}`))
	submitted := httptest.NewRecorder()
	server.ServeHTTP(submitted, submit)
	if submitted.Code != http.StatusAccepted || !strings.Contains(submitted.Body.String(), `"status":"queued"`) {
		t.Fatalf("unexpected submit response: %d %s", submitted.Code, submitted.Body.String())
	}
}

func TestCreateProjectRejectsUnknownFields(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/api/v1/projects", strings.NewReader(`{"name":"Valid name","secret":"nope"}`))
	response := httptest.NewRecorder()
	testServer().ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", response.Code)
	}
}
