package integrations

import (
	"context"
	"encoding/json"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
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
