package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// deployTestAgent creates a project and an agent through the API and returns
// the agent id.
func deployTestAgent(t *testing.T, server http.Handler) string {
	t.Helper()
	create := httptest.NewRequest(http.MethodPost, "/api/v1/projects", strings.NewReader(`{"name":"Support workspace","template":"rag-agent"}`))
	created := httptest.NewRecorder()
	server.ServeHTTP(created, create)
	if created.Code != http.StatusCreated {
		t.Fatalf("project create failed: %d %s", created.Code, created.Body.String())
	}
	projectID := strings.Split(strings.Split(created.Body.String(), `"id":"`)[1], `"`)[0]
	deploy := httptest.NewRequest(http.MethodPost, "/api/v1/agents", strings.NewReader(`{"project_id":"`+projectID+`","name":"support","version":"1.0","image":"registry/agents/support:1.0","graph_module":"agents.customer_support.graph:build"}`))
	deployed := httptest.NewRecorder()
	server.ServeHTTP(deployed, deploy)
	if deployed.Code != http.StatusAccepted {
		t.Fatalf("agent deploy failed: %d %s", deployed.Code, deployed.Body.String())
	}
	return strings.Split(strings.Split(deployed.Body.String(), `"id":"`)[1], `"`)[0]
}

func TestInvokeAgentProxiesToRuntime(t *testing.T) {
	var received map[string]any
	runtime := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/invoke" {
			t.Fatalf("unexpected runtime path %s", r.URL.Path)
		}
		if got := r.Header.Get("X-MLAIOps-Agent-Name"); got != "support" {
			t.Fatalf("expected agent name header, got %q", got)
		}
		_ = json.NewDecoder(r.Body).Decode(&received)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"reply":"hello","session_id":"sess-1","input_tokens":3,"output_tokens":2,"cost_usd":0,"duration_ms":5}`))
	}))
	defer runtime.Close()
	t.Setenv("AGENT_RUNTIME_URL", runtime.URL)

	server := testServer()
	agentID := deployTestAgent(t, server)
	invoke := httptest.NewRequest(http.MethodPost, "/api/v1/agents/"+agentID+"/invoke", strings.NewReader(`{"message":"hi","user_id":"u123"}`))
	response := httptest.NewRecorder()
	server.ServeHTTP(response, invoke)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"reply":"hello"`) {
		t.Fatalf("unexpected invoke response: %d %s", response.Code, response.Body.String())
	}
	if received["message"] != "hi" || received["user_id"] != "u123" {
		t.Fatalf("runtime did not receive the payload: %v", received)
	}
}

func TestInvokeAgentUnknownAgent(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/api/v1/agents/agt-missing/invoke", strings.NewReader(`{"message":"hi"}`))
	response := httptest.NewRecorder()
	testServer().ServeHTTP(response, request)
	if response.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d %s", response.Code, response.Body.String())
	}
}

func TestInvokeAgentRequiresMessage(t *testing.T) {
	server := testServer()
	agentID := deployTestAgent(t, server)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/agents/"+agentID+"/invoke", strings.NewReader(`{"message":"  "}`))
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d %s", response.Code, response.Body.String())
	}
}

func TestInvokeAgentRuntimeDown(t *testing.T) {
	t.Setenv("AGENT_RUNTIME_URL", "http://127.0.0.1:1")
	server := testServer()
	agentID := deployTestAgent(t, server)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/agents/"+agentID+"/invoke", strings.NewReader(`{"message":"hi"}`))
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d %s", response.Code, response.Body.String())
	}
}
