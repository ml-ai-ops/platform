package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var GroupVersion = schema.GroupVersion{Group: "mlaiops.io", Version: "v1alpha1"}

type ReplicaSpec struct {
	Min int32 `json:"min,omitempty"`
	Max int32 `json:"max,omitempty"`
}

type LLMSpec struct {
	Backend             string `json:"backend,omitempty"`
	InferenceServiceRef string `json:"inferenceServiceRef,omitempty"`
}

type ToolReference struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type TrafficPolicy struct {
	CanaryWeight int32  `json:"canaryWeight,omitempty"`
	StableRef    string `json:"stableRef,omitempty"`
}

type NexusAgentSpec struct {
	Version         string          `json:"version"`
	Image           string          `json:"image"`
	GraphModule     string          `json:"graphModule"`
	Replicas        ReplicaSpec     `json:"replicas,omitempty"`
	LLM             LLMSpec         `json:"llm,omitempty"`
	Tools           []ToolReference `json:"tools,omitempty"`
	LangfuseProject string          `json:"langfuseProject,omitempty"`
	TrafficPolicy   TrafficPolicy   `json:"trafficPolicy,omitempty"`
}

type NexusAgentStatus struct {
	Phase              string             `json:"phase,omitempty"`
	ReadyReplicas      int32              `json:"readyReplicas,omitempty"`
	ObservedGeneration int64              `json:"observedGeneration,omitempty"`
	Conditions         []metav1.Condition `json:"conditions,omitempty"`
}

type NexusAgent struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              NexusAgentSpec   `json:"spec,omitempty"`
	Status            NexusAgentStatus `json:"status,omitempty"`
}

type NexusAgentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NexusAgent `json:"items"`
}

func AddToScheme(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(GroupVersion, &NexusAgent{}, &NexusAgentList{}, &NexusPipelineRun{}, &NexusPipelineRunList{}, &NexusModelPromotion{}, &NexusModelPromotionList{})
	metav1.AddToGroupVersion(scheme, GroupVersion)
	return nil
}

type NexusPipelineRunSpec struct {
	PipelineRef        string            `json:"pipelineRef"`
	Parameters         map[string]string `json:"parameters,omitempty"`
	ServiceAccountName string            `json:"serviceAccountName,omitempty"`
}

type NexusPipelineRunStatus struct {
	Phase       string       `json:"phase,omitempty"`
	WorkflowRef string       `json:"workflowRef,omitempty"`
	StartedAt   *metav1.Time `json:"startedAt,omitempty"`
	FinishedAt  *metav1.Time `json:"finishedAt,omitempty"`
	Message     string       `json:"message,omitempty"`
}

type NexusPipelineRun struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              NexusPipelineRunSpec   `json:"spec,omitempty"`
	Status            NexusPipelineRunStatus `json:"status,omitempty"`
}

type NexusPipelineRunList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NexusPipelineRun `json:"items"`
}

type PromotionGate struct {
	Metric   string  `json:"metric"`
	Operator string  `json:"operator"`
	Value    float64 `json:"value"`
}

type NexusModelPromotionSpec struct {
	ModelName   string          `json:"modelName"`
	Version     string          `json:"version"`
	TargetStage string          `json:"targetStage"`
	Gates       []PromotionGate `json:"gates,omitempty"`
}

type NexusModelPromotionStatus struct {
	Phase               string `json:"phase,omitempty"`
	Message             string `json:"message,omitempty"`
	InferenceServiceRef string `json:"inferenceServiceRef,omitempty"`
}

type NexusModelPromotion struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              NexusModelPromotionSpec   `json:"spec,omitempty"`
	Status            NexusModelPromotionStatus `json:"status,omitempty"`
}

type NexusModelPromotionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NexusModelPromotion `json:"items"`
}

func (in *NexusAgent) DeepCopyObject() runtime.Object {
	if in == nil {
		return nil
	}
	out := new(NexusAgent)
	*out = *in
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec.Tools = append([]ToolReference(nil), in.Spec.Tools...)
	out.Status.Conditions = append([]metav1.Condition(nil), in.Status.Conditions...)
	return out
}

func (in *NexusAgentList) DeepCopyObject() runtime.Object {
	if in == nil {
		return nil
	}
	out := new(NexusAgentList)
	*out = *in
	out.Items = make([]NexusAgent, len(in.Items))
	for i := range in.Items {
		out.Items[i] = *(in.Items[i].DeepCopyObject().(*NexusAgent))
	}
	return out
}

func (in *NexusPipelineRun) DeepCopyObject() runtime.Object {
	if in == nil {
		return nil
	}
	out := new(NexusPipelineRun)
	*out = *in
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec.Parameters = make(map[string]string, len(in.Spec.Parameters))
	for key, value := range in.Spec.Parameters {
		out.Spec.Parameters[key] = value
	}
	if in.Status.StartedAt != nil {
		value := in.Status.StartedAt.DeepCopy()
		out.Status.StartedAt = value
	}
	if in.Status.FinishedAt != nil {
		value := in.Status.FinishedAt.DeepCopy()
		out.Status.FinishedAt = value
	}
	return out
}
func (in *NexusPipelineRunList) DeepCopyObject() runtime.Object {
	if in == nil {
		return nil
	}
	out := new(NexusPipelineRunList)
	*out = *in
	out.Items = make([]NexusPipelineRun, len(in.Items))
	for i := range in.Items {
		out.Items[i] = *(in.Items[i].DeepCopyObject().(*NexusPipelineRun))
	}
	return out
}
func (in *NexusModelPromotion) DeepCopyObject() runtime.Object {
	if in == nil {
		return nil
	}
	out := new(NexusModelPromotion)
	*out = *in
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec.Gates = append([]PromotionGate(nil), in.Spec.Gates...)
	return out
}
func (in *NexusModelPromotionList) DeepCopyObject() runtime.Object {
	if in == nil {
		return nil
	}
	out := new(NexusModelPromotionList)
	*out = *in
	out.Items = make([]NexusModelPromotion, len(in.Items))
	for i := range in.Items {
		out.Items[i] = *(in.Items[i].DeepCopyObject().(*NexusModelPromotion))
	}
	return out
}
