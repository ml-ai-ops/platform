//go:build integration

package integration

import (
	"context"
	"os"
	"testing"

	"github.com/mlaiops/platform/internal/store"
	"github.com/mlaiops/platform/pkg/api"
)

func TestPostgresPersistsResourceAuditAndOutboxAtomically(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	repository, err := store.OpenPostgres(context.Background(), databaseURL, "integration-test")
	if err != nil {
		t.Fatal(err)
	}
	defer repository.Close()
	project, err := repository.CreateProject(api.CreateProjectRequest{Name: "Integration project"}, "test-user")
	if err != nil && err != store.ErrConflict {
		t.Fatal(err)
	}
	if project.ID != "" {
		if _, err := repository.SubmitPipeline(api.SubmitPipelineRequest{ProjectID: project.ID, Name: "train"}, "test-user"); err != nil {
			t.Fatal(err)
		}
	}
	if len(repository.Audit()) == 0 {
		t.Fatal("expected durable audit event")
	}
	events, err := repository.PendingOutbox(context.Background(), 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) == 0 {
		t.Fatal("expected transactional outbox events")
	}
}
