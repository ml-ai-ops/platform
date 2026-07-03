package operator

import (
	"context"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	mlaiopsv1 "github.com/ml-ai-ops/platform/pkg/kube/v1alpha1"
)

func TestAgentControllerCreatesDeploymentAndService(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = appsv1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)
	_ = mlaiopsv1.AddToScheme(scheme)
	agent := &mlaiopsv1.NexusAgent{
		TypeMeta:   metav1.TypeMeta{APIVersion: "mlaiops.io/v1alpha1", Kind: "NexusAgent"},
		ObjectMeta: metav1.ObjectMeta{Name: "support", Namespace: "team-a"},
		Spec:       mlaiopsv1.NexusAgentSpec{Version: "1.2", Image: "registry/support:1.2", GraphModule: "agents.support:graph", Replicas: mlaiopsv1.ReplicaSpec{Min: 2, Max: 5}},
	}
	client := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(agent).WithObjects(agent).Build()
	reconciler := &AgentReconciler{Client: client}
	request := ctrl.Request{NamespacedName: types.NamespacedName{Name: agent.Name, Namespace: agent.Namespace}}
	if _, err := reconciler.Reconcile(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	if _, err := reconciler.Reconcile(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	var deployment appsv1.Deployment
	if err := client.Get(context.Background(), types.NamespacedName{Name: "support-1-2", Namespace: "team-a"}, &deployment); err != nil {
		t.Fatal(err)
	}
	if deployment.Spec.Replicas == nil || *deployment.Spec.Replicas != 2 || len(deployment.Spec.Template.Spec.Containers) != 2 {
		t.Fatalf("unexpected deployment: %#v", deployment.Spec)
	}
	var service corev1.Service
	if err := client.Get(context.Background(), types.NamespacedName{Name: "support-1-2", Namespace: "team-a"}, &service); err != nil {
		t.Fatal(err)
	}
}
