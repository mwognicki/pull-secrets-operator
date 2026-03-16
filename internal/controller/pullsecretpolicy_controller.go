package controller

import (
	"context"
	"fmt"

	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	pullsecretsv1alpha1 "github.com/mwognicki/pull-secrets-operator/api/pullsecrets/v1alpha1"
)

// PullSecretPolicyReconciler reconciles PullSecretPolicy status.
type PullSecretPolicyReconciler struct {
	client.Client
}

func (r *PullSecretPolicyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var policy pullsecretsv1alpha1.PullSecretPolicy
	if err := r.Get(ctx, req.NamespacedName, &policy); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if err := r.updatePullSecretPolicyStatus(ctx, &policy); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *PullSecretPolicyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&pullsecretsv1alpha1.PullSecretPolicy{}).
		Complete(r)
}

func (r *PullSecretPolicyReconciler) updatePullSecretPolicyStatus(
	ctx context.Context,
	policy *pullsecretsv1alpha1.PullSecretPolicy,
) error {
	now := metav1.Now()
	policy.Status.ObservedGeneration = policy.Generation
	policy.Status.ExcludedNamespaceCount = int32(len(policy.Spec.ExcludedNamespaces))
	policy.Status.ActiveSingleton = policy.Name == pullsecretsv1alpha1.PullSecretPolicySingletonName
	policy.Status.LastSyncTime = &now

	condition := metav1.Condition{
		Type:               "Ready",
		ObservedGeneration: policy.Generation,
		LastTransitionTime: now,
	}
	if policy.Status.ActiveSingleton {
		condition.Status = metav1.ConditionTrue
		condition.Reason = "SingletonActive"
		condition.Message = "PullSecretPolicy is the active singleton policy"
	} else {
		condition.Status = metav1.ConditionFalse
		condition.Reason = "NonSingletonName"
		condition.Message = fmt.Sprintf("PullSecretPolicy must be named %q to be active", pullsecretsv1alpha1.PullSecretPolicySingletonName)
	}
	apimeta.SetStatusCondition(&policy.Status.Conditions, condition)

	if err := r.Status().Update(ctx, policy); err != nil {
		return fmt.Errorf("update PullSecretPolicy status %s: %w", client.ObjectKeyFromObject(policy), err)
	}

	return nil
}
