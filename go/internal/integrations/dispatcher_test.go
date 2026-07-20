package integrations

import (
	"context"
	"encoding/json"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/fake"
)

func TestDispatcherCreatesAgentCRD(t *testing.T) {
	client := fake.NewSimpleDynamicClient(runtime.NewScheme())
	dispatcher := NewDispatcher(client, "team-a")
	resource, _ := json.Marshal(map[string]any{
		"name": "support", "version": "1", "image": "support:1", "graph_module": "agents.support:graph",
		"replicas": 2, "llm_backend": "self-hosted", "tools": []string{"lookup"}, "canary_weight": 10,
	})
	command, _ := json.Marshal(LifecycleCommand{ID: "agt-1", Kind: "agent", Action: "agent.deployed", Resource: resource, Tenant: "team-a"})
	if err := dispatcher.Dispatch(context.Background(), KafkaRecord{Topic: "mlaiops.agent.commands", Value: command}); err != nil {
		t.Fatal(err)
	}
}

func TestDispatcherUpsertsAndDeletesWorkspaceCRD(t *testing.T) {
	client := fake.NewSimpleDynamicClient(runtime.NewScheme())
	dispatcher := NewDispatcher(client, "mlaiops-workspaces")
	resource, _ := json.Marshal(map[string]any{
		"subject": "oidc|user@example.com", "services": []string{"workbench", "ide"}, "disabled": false,
		"compute": map[string]any{"vcpus": 4, "memory_gb": 8, "gpus": 0, "gpu_type": "", "max_vms": 2},
		"storage": map[string]any{"size_gb": 100},
	})
	command, _ := json.Marshal(LifecycleCommand{ID: "oidc|user@example.com", Kind: "user_access", Action: "access.upserted", Resource: resource, Tenant: "team-a"})
	record := KafkaRecord{Topic: "mlaiops.workspace.commands", Value: command}
	if err := dispatcher.Dispatch(context.Background(), record); err != nil {
		t.Fatal(err)
	}
	if err := dispatcher.Dispatch(context.Background(), record); err != nil {
		t.Fatalf("workspace upsert must be idempotent: %v", err)
	}
	gvr := schema.GroupVersionResource{Group: "mlaiops.io", Version: "v1alpha1", Resource: "nexusworkspaces"}
	workspace, err := client.Resource(gvr).Namespace("mlaiops-workspaces").Get(context.Background(), "workspace-oidc-user-example-com", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	memory, _, _ := unstructured.NestedInt64(workspace.Object, "spec", "compute", "memoryGB")
	if memory != 8 {
		t.Fatalf("unexpected workspace compute: %#v", workspace.Object["spec"])
	}
	deleted, _ := json.Marshal(LifecycleCommand{ID: "oidc|user@example.com", Kind: "user_access", Action: "access.deleted", Resource: json.RawMessage(`{"subject":"oidc|user@example.com"}`), Tenant: "team-a"})
	if err := dispatcher.Dispatch(context.Background(), KafkaRecord{Topic: "mlaiops.workspace.commands", Value: deleted}); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Resource(gvr).Namespace("mlaiops-workspaces").Get(context.Background(), "workspace-oidc-user-example-com", metav1.GetOptions{}); err == nil {
		t.Fatal("workspace should be deleted when access is revoked")
	}
}
