package store

import (
	"testing"

	"github.com/mlaiops/platform/pkg/api"
)

func submitTestRun(t *testing.T, s *Store) api.PipelineRun {
	t.Helper()
	project, err := s.CreateProject(api.CreateProjectRequest{Name: "step test project"}, "tester")
	if err != nil {
		t.Fatal(err)
	}
	run, err := s.SubmitPipeline(api.SubmitPipelineRequest{ProjectID: project.ID, Name: "training-pipeline"}, "tester")
	if err != nil {
		t.Fatal(err)
	}
	return run
}

func TestUpdateRunStepDrivesRunLifecycle(t *testing.T) {
	s := New()
	run := submitTestRun(t, s)

	updated, err := s.UpdateRunStep(run.ID, api.UpdateRunStepRequest{Step: "validate-data", Status: "running"}, "flow")
	if err != nil || updated.Status != "running" {
		t.Fatalf("run should be running: %v %v", updated.Status, err)
	}
	for _, step := range []string{"validate-data", "train-model", "evaluate"} {
		if updated, err = s.UpdateRunStep(run.ID, api.UpdateRunStepRequest{Step: step, Status: "succeeded"}, "flow"); err != nil {
			t.Fatal(err)
		}
	}
	if updated.Status != "succeeded" || updated.Progress != 100 {
		t.Fatalf("run should be complete: status=%s progress=%d", updated.Status, updated.Progress)
	}
	if len(updated.Logs) < 4 {
		t.Fatalf("step transitions must be logged, got %d entries", len(updated.Logs))
	}
}

func TestUpdateRunStepFailureFailsRun(t *testing.T) {
	s := New()
	run := submitTestRun(t, s)
	updated, err := s.UpdateRunStep(run.ID, api.UpdateRunStepRequest{Step: "train-model", Status: "failed", Message: "OOM"}, "flow")
	if err != nil || updated.Status != "failed" {
		t.Fatalf("run should fail with its step: %v %v", updated.Status, err)
	}
	// Terminal runs are not resurrected by late reports.
	late, err := s.UpdateRunStep(run.ID, api.UpdateRunStepRequest{Step: "evaluate", Status: "succeeded"}, "flow")
	if err != nil || late.Status != "failed" {
		t.Fatalf("late report must not resurrect the run: %v %v", late.Status, err)
	}
}

func TestUpdateRunStepAppendsUnknownSteps(t *testing.T) {
	s := New()
	run := submitTestRun(t, s)
	updated, err := s.UpdateRunStep(run.ID, api.UpdateRunStepRequest{Step: "register-model", Status: "succeeded"}, "flow")
	if err != nil {
		t.Fatal(err)
	}
	if len(updated.Steps) != 4 {
		t.Fatalf("dynamic step should be appended, got %d steps", len(updated.Steps))
	}
	if updated.Progress != 25 {
		t.Fatalf("progress must count completed/total: %d", updated.Progress)
	}
}

func TestUpdateRunStepValidation(t *testing.T) {
	s := New()
	run := submitTestRun(t, s)
	if _, err := s.UpdateRunStep(run.ID, api.UpdateRunStepRequest{Step: "x", Status: "sideways"}, "flow"); err == nil {
		t.Fatal("invalid status must be rejected")
	}
	if _, err := s.UpdateRunStep("run-missing", api.UpdateRunStepRequest{Step: "x", Status: "running"}, "flow"); err != ErrNotFound {
		t.Fatalf("expected not found, got %v", err)
	}
}

func TestSetRunEngine(t *testing.T) {
	s := New()
	run := submitTestRun(t, s)
	linked, err := s.SetRunEngine(run.ID, "fr-123")
	if err != nil || linked.EngineRunID != "fr-123" {
		t.Fatalf("engine link failed: %v %v", linked.EngineRunID, err)
	}
}
