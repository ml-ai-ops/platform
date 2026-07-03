package operator

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	mlaiopsv1 "github.com/ml-ai-ops/platform/pkg/kube/v1alpha1"
)

const agentFinalizer = "mlaiops.io/agent-cleanup"

type AgentReconciler struct {
	client.Client
}

func (r *AgentReconciler) Reconcile(ctx context.Context, request ctrl.Request) (ctrl.Result, error) {
	var agent mlaiopsv1.NexusAgent
	if err := r.Get(ctx, request.NamespacedName, &agent); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	if !agent.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(&agent, agentFinalizer) {
			controllerutil.RemoveFinalizer(&agent, agentFinalizer)
			return ctrl.Result{}, r.Update(ctx, &agent)
		}
		return ctrl.Result{}, nil
	}
	if !controllerutil.ContainsFinalizer(&agent, agentFinalizer) {
		controllerutil.AddFinalizer(&agent, agentFinalizer)
		if err := r.Update(ctx, &agent); err != nil {
			return ctrl.Result{}, err
		}
	}
	plan, err := ReconcileAgent(AgentSpec{
		Name: agent.Name, Namespace: agent.Namespace, Version: agent.Spec.Version,
		Image: agent.Spec.Image, GraphModule: agent.Spec.GraphModule,
		MinReplicas: int(agent.Spec.Replicas.Min), MaxReplicas: int(agent.Spec.Replicas.Max),
		LLMBackend: agent.Spec.LLM.Backend, LangfuseProject: agent.Spec.LangfuseProject,
		CanaryWeight: int(agent.Spec.TrafficPolicy.CanaryWeight), StableRef: agent.Spec.TrafficPolicy.StableRef,
	})
	if err != nil {
		return r.fail(ctx, &agent, "InvalidSpec", err)
	}
	deployment := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: plan.Workload.Name, Namespace: agent.Namespace}}
	_, err = controllerutil.CreateOrUpdate(ctx, r.Client, deployment, func() error {
		replicas := int32(plan.Workload.Replicas)
		deployment.Labels = plan.Workload.Labels
		deployment.Spec.Replicas = &replicas
		deployment.Spec.Selector = &metav1.LabelSelector{MatchLabels: map[string]string{"mlaiops.io/agent": agent.Name, "mlaiops.io/version": agent.Spec.Version}}
		deployment.Spec.Template.ObjectMeta.Labels = deployment.Spec.Selector.MatchLabels
		deployment.Spec.Template.Spec.Containers = []corev1.Container{
			{Name: "agent", Image: agent.Spec.Image, Ports: []corev1.ContainerPort{{Name: "http", ContainerPort: 8000}}, Env: envVars(plan.Workload.Containers[0].Env), SecurityContext: hardenedSecurityContext()},
			{Name: "trace-proxy", Image: plan.Workload.Containers[1].Image, Ports: []corev1.ContainerPort{{Name: "proxy", ContainerPort: 8081}}, Env: envVars(plan.Workload.Containers[1].Env), SecurityContext: hardenedSecurityContext()},
		}
		return controllerutil.SetControllerReference(&agent, deployment, r.Scheme())
	})
	if err != nil {
		return ctrl.Result{}, err
	}
	service := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: plan.Workload.Name, Namespace: agent.Namespace}}
	_, err = controllerutil.CreateOrUpdate(ctx, r.Client, service, func() error {
		service.Spec.Selector = deployment.Spec.Selector.MatchLabels
		service.Spec.Ports = []corev1.ServicePort{{Name: "http", Port: 80, TargetPort: intstrFromInt(8000)}}
		return controllerutil.SetControllerReference(&agent, service, r.Scheme())
	})
	if err != nil {
		return ctrl.Result{}, err
	}
	agent.Status.Phase = "Ready"
	agent.Status.ReadyReplicas = deployment.Status.ReadyReplicas
	agent.Status.ObservedGeneration = agent.Generation
	apimeta.SetStatusCondition(&agent.Status.Conditions, metav1.Condition{Type: "Ready", Status: metav1.ConditionTrue, Reason: "WorkloadReconciled", Message: fmt.Sprintf("Deployment %s reconciled", deployment.Name)})
	if err := r.Status().Update(ctx, &agent); err != nil && !apierrors.IsConflict(err) {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *AgentReconciler) fail(ctx context.Context, agent *mlaiopsv1.NexusAgent, reason string, reconcileErr error) (ctrl.Result, error) {
	agent.Status.Phase = "Failed"
	apimeta.SetStatusCondition(&agent.Status.Conditions, metav1.Condition{Type: "Ready", Status: metav1.ConditionFalse, Reason: reason, Message: reconcileErr.Error()})
	_ = r.Status().Update(ctx, agent)
	return ctrl.Result{}, reconcileErr
}

func (r *AgentReconciler) SetupWithManager(manager ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(manager).For(&mlaiopsv1.NexusAgent{}).Owns(&appsv1.Deployment{}).Owns(&corev1.Service{}).Complete(r)
}

func envVars(values map[string]string) []corev1.EnvVar {
	result := make([]corev1.EnvVar, 0, len(values))
	for name, value := range values {
		result = append(result, corev1.EnvVar{Name: name, Value: value})
	}
	return result
}

func hardenedSecurityContext() *corev1.SecurityContext {
	allow := false
	readOnly := true
	nonRoot := true
	return &corev1.SecurityContext{AllowPrivilegeEscalation: &allow, ReadOnlyRootFilesystem: &readOnly, RunAsNonRoot: &nonRoot}
}

func intstrFromInt(value int) intstr.IntOrString { return intstr.FromInt(value) }
