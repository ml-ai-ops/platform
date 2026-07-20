package integrations

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
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
	case "user_access":
		plural, kind = "nexusworkspaces", "NexusWorkspace"
		name = "workspace-" + command.ID
		compute, _ := resource["compute"].(map[string]any)
		storage, _ := resource["storage"].(map[string]any)
		spec = map[string]any{
			"subject": resource["subject"], "services": resource["services"], "disabled": resource["disabled"],
			"compute":   map[string]any{"vcpus": integer(compute["vcpus"]), "memoryGB": integer(compute["memory_gb"]), "gpus": integer(compute["gpus"]), "gpuType": compute["gpu_type"], "maxVMs": integer(compute["max_vms"])},
			"storageGB": integer(storage["size_gb"]),
		}
	default:
		return fmt.Errorf("unsupported lifecycle kind %q", command.Kind)
	}
	object := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "mlaiops.io/v1alpha1", "kind": kind,
		"metadata": map[string]any{"name": dnsName(name, command.ID), "namespace": d.namespace, "labels": map[string]any{"mlaiops.io/tenant": command.Tenant, "mlaiops.io/resource-id": command.ID}},
		"spec":     spec,
	}}
	gvr := schema.GroupVersionResource{Group: "mlaiops.io", Version: "v1alpha1", Resource: plural}
	resources := d.client.Resource(gvr).Namespace(d.namespace)
	if command.Kind == "user_access" && command.Action == "access.deleted" {
		err := resources.Delete(ctx, object.GetName(), metav1.DeleteOptions{})
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}
	existing, err := resources.Get(ctx, object.GetName(), metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		_, err = resources.Create(ctx, object, metav1.CreateOptions{})
		return err
	}
	if err != nil {
		return err
	}
	object.SetResourceVersion(existing.GetResourceVersion())
	_, err = resources.Update(ctx, object, metav1.UpdateOptions{})
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

func integer(value any) int64 {
	switch number := value.(type) {
	case float64:
		return int64(number)
	case int64:
		return number
	case int:
		return int64(number)
	default:
		return 0
	}
}

func dnsName(name, fallback string) string {
	if name == "" {
		name = fallback
	}
	raw := strings.ToLower(name)
	value := strings.Trim(regexp.MustCompile(`[^a-z0-9]+`).ReplaceAllString(raw, "-"), "-")
	if value == "" {
		value = "resource"
	}
	if len(value) <= 63 {
		return value
	}
	digest := fmt.Sprintf("%x", sha256.Sum256([]byte(raw)))[:8]
	return strings.Trim(value[:54], "-") + "-" + digest
}
