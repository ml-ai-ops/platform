package store

import (
	"path/filepath"
	"testing"

	"github.com/mlaiops/platform/pkg/api"
)

func TestStorePersistsResourcesAndAudit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "platform.json")
	first := New(path)
	project, err := first.CreateProject(api.CreateProjectRequest{Name: "Churn risk"}, "test-user")
	if err != nil {
		t.Fatal(err)
	}
	model, err := first.RegisterModel(api.RegisterModelRequest{
		ProjectID: project.ID, Name: "churn", Version: "1", ArtifactURI: "s3://models/churn/1",
	}, "test-user")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := first.PromoteModel(model.ID, "production", "reviewer"); err != nil {
		t.Fatal(err)
	}

	reloaded := New(path)
	if got := reloaded.Models(); len(got) != 1 || got[0].Stage != "production" {
		t.Fatalf("unexpected persisted models: %#v", got)
	}
	if got := reloaded.Audit(); len(got) != 3 || got[0].Actor != "reviewer" {
		t.Fatalf("unexpected audit events: %#v", got)
	}
}

func TestStoreValidatesAgentTraffic(t *testing.T) {
	s := New()
	agent, err := s.DeployAgent(api.DeployAgentRequest{
		ProjectID: "prj-demo", Name: "support", Version: "1", Image: "example/agent:1",
		GraphModule: "agents.support:graph",
	}, "test-user")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.SetAgentTraffic(agent.ID, 101, "test-user"); err == nil {
		t.Fatal("expected canary validation error")
	}
}
