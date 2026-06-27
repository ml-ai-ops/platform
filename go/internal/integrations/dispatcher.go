package integrations

import (
	"context"
	"encoding/json"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

type LifecycleCommand struct {
	ID       string          `json:"id"`
	Kind     string          `json:"kind"`
	Action   string          `json:"action"`
	Resource json.RawMessage `json:"resource"`
	Tenant   string          `json:"tenant"`
}

type Dispatcher struct {
	client    dynamic.Interface
	namespace string
}

func NewDispatcher(client dynamic.Interface, namespace string) *Dispatcher {
	return &Dispatcher{client: client, namespace: namespace}
}

func (d *Dispatcher) Dispatch(ctx context.Context, record KafkaRecord) error {
	var command LifecycleCommand
	if err := json.Unmarshal(record.Value, &command); err != nil {
		return err
	}
	var resource map[string]any
	if err := json.Unmarshal(command.Resource, &resource); err != nil {
		return err
	}
	name, _ := resource["name"].(string)
	if name == "" {
		name = command.ID
	}
	var plural, kind string
	var spec map[string]any
	switch command.Kind {
	case "pipeline_run":
		plural, kind = "nexuspipelineruns", "NexusPipelineRun"
		spec = map[string]any{"pipelineRef": resource["name"], "parameters": map[string]any{"project_id": resource["project_id"]}}
	case "model":
		if command.Action != "model.promoted" {
			return nil
		}
		plural, kind = "nexusmodelpromotions", "NexusModelPromotion"
		spec = map[string]any{"modelName": resource["name"], "version": resource["version"], "targetStage": resource["stage"]}
	case "agent":
		plural, kind = "nexusagents", "NexusAgent"
		spec = map[string]any{
			"version": resource["version"], "image": resource["image"], "graphModule": resource["graph_module"],
			"replicas": map[string]any{"min": resource["replicas"], "max": resource["replicas"]},
			"llm":      map[string]any{"backend": resource["llm_backend"]}, "tools": toolRefs(resource["tools"]),
			"trafficPolicy": map[string]any{"canaryWeight": resource["canary_weight"]},
		}
	case "tool":
		plural, kind = "nexustools", "NexusTool"
		spec = map[string]any{"version": resource["version"], "description": resource["description"], "tags": resource["tags"], "inputSchema": resource["input_schema"]}
	case "connection":
		plural, kind = "nexusconnections", "NexusConnection"
		spec = map[string]any{"type": resource["type"], "endpoint": resource["endpoint"], "secretRef": map[string]any{"name": resource["secret_ref"]}}
	default:
		return fmt.Errorf("unsupported lifecycle kind %q", command.Kind)
	}
	object := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "mlaiops.io/v1alpha1", "kind": kind,
		"metadata": map[string]any{"name": dnsName(name, command.ID), "namespace": d.namespace, "labels": map[string]any{"mlaiops.io/tenant": command.Tenant, "mlaiops.io/resource-id": command.ID}},
		"spec":     spec,
	}}
	gvr := schema.GroupVersionResource{Group: "mlaiops.io", Version: "v1alpha1", Resource: plural}
	_, err := d.client.Resource(gvr).Namespace(d.namespace).Create(ctx, object, metav1.CreateOptions{})
	return err
}

func toolRefs(value any) []any {
	values, _ := value.([]any)
	result := make([]any, 0, len(values))
	for _, item := range values {
		result = append(result, map[string]any{"name": item, "version": "latest"})
	}
	return result
}

func dnsName(name, fallback string) string {
	if name == "" {
		return fallback
	}
	return name
}
