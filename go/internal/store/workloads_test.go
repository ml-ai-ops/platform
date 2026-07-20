package store

import (
	"testing"

	"github.com/ml-ai-ops/platform/pkg/api"
)

func TestNamedResourceProfileAppliesCanonicalLimits(t *testing.T) {
	repository := New()
	access, err := repository.UpsertUserAccess("user-1", api.UpsertUserAccessRequest{
		Role: "user", Services: []string{"projects"},
		Compute: api.ComputeGrant{Profile: "team", VCPUs: 99, MemoryGB: 99},
		Storage: api.StorageGrant{SizeGB: 999},
	}, "admin")
	if err != nil {
		t.Fatal(err)
	}
	if access.Compute.VCPUs != 4 || access.Compute.MemoryGB != 8 || access.Compute.MaxFunctions != 10 || access.Storage.SizeGB != 100 {
		t.Fatalf("team profile was not canonical: %#v", access)
	}
}

func TestProjectRepositoryRejectsEmbeddedCredentials(t *testing.T) {
	repository := New()
	if _, err := repository.SetProjectRepository("prj-demo", api.SetProjectRepositoryRequest{URL: "https://token@github.com/acme/model.git"}, "user"); err == nil {
		t.Fatal("expected embedded credentials to be rejected")
	}
	project, err := repository.SetProjectRepository("prj-demo", api.SetProjectRepositoryRequest{URL: "https://github.com/acme/model.git", DefaultBranch: "develop"}, "user")
	if err != nil {
		t.Fatal(err)
	}
	if project.Repository == nil || project.Repository.Provider != "github" || project.Repository.DefaultBranch != "develop" {
		t.Fatalf("unexpected repository metadata: %#v", project.Repository)
	}
}

func TestPipelineDefinitionCreatesRunFromValidatedDAG(t *testing.T) {
	repository := New()
	definition, err := repository.UpsertPipelineDefinition("", api.UpsertPipelineDefinitionRequest{
		ProjectID: "prj-demo", Name: "feature-train", Version: "1", ExecutionMode: "functions",
		Jobs: []api.PipelineJob{
			{Name: "features", Kind: "function", Function: "features-fn", Resources: api.JobResources{Memory: "512Mi"}},
			{Name: "train", Kind: "function", Function: "train-fn", DependsOn: []string{"features"}, Retries: 2},
		},
	}, "engineer")
	if err != nil {
		t.Fatal(err)
	}
	run, err := repository.SubmitPipeline(api.SubmitPipelineRequest{ProjectID: "prj-demo", DefinitionID: definition.ID, Parameters: map[string]any{"date": "2026-07-20"}}, "engineer")
	if err != nil {
		t.Fatal(err)
	}
	if run.ExecutionMode != "functions" || run.Name != "feature-train" || len(run.Steps) != 2 || run.Steps[1].DependsOn[0] != "features" {
		t.Fatalf("unexpected compiled run: %#v", run)
	}
}

func TestPipelineDefinitionRejectsCycles(t *testing.T) {
	repository := New()
	_, err := repository.UpsertPipelineDefinition("", api.UpsertPipelineDefinitionRequest{
		ProjectID: "prj-demo", Name: "cycle", Version: "1", ExecutionMode: "functions",
		Jobs: []api.PipelineJob{
			{Name: "a", Kind: "function", Function: "a", DependsOn: []string{"b"}},
			{Name: "b", Kind: "function", Function: "b", DependsOn: []string{"a"}},
		},
	}, "engineer")
	if err == nil {
		t.Fatal("expected cyclic definition to be rejected")
	}
}

func TestFunctionRequestValidatesRuntimeAndTriggers(t *testing.T) {
	valid := api.DeployFunctionRequest{ProjectID: "prj-demo", Name: "score-events", Image: "registry/score:1", CPU: "500m", Memory: "1Gi", Annotations: map[string]string{"topic": "cron-function", "schedule": "*/5 * * * *", "com.nexus.invocation": "async"}}
	if err := ValidateFunctionRequest(valid); err != nil {
		t.Fatalf("valid function rejected: %v", err)
	}
	invalid := valid
	invalid.Name = "Score Events"
	if err := ValidateFunctionRequest(invalid); err == nil {
		t.Fatal("invalid DNS label should be rejected")
	}
}
