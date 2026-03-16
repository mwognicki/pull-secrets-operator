package controller

import (
	"context"
	"fmt"

	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	pullsecretsv1alpha1 "github.com/mwognicki/pull-secrets-operator/api/pullsecrets/v1alpha1"
	"github.com/mwognicki/pull-secrets-operator/internal/sync"
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
	validationErr := sync.ValidatePullSecretPolicy(*policy)
	isValid := validationErr == nil

	policy.Status.ObservedGeneration = policy.Generation
	policy.Status.ExcludedNamespaceCount = int32(len(policy.Spec.ExcludedNamespaces))
	policy.Status.ActiveSingleton = policy.Name == pullsecretsv1alpha1.PullSecretPolicySingletonName
	policy.Status.Valid = isValid
	policy.Status.LastSyncTime = &now

	validCondition := metav1.Condition{
		Type:               "Valid",
		ObservedGeneration: policy.Generation,
		LastTransitionTime: now,
	}
	if isValid {
		validCondition.Status = metav1.ConditionTrue
		validCondition.Reason = "Valid"
		validCondition.Message = "PullSecretPolicy is valid from the operator perspective"
	} else {
		validCondition.Status = metav1.ConditionFalse
		validCondition.Reason = "ValidationFailed"
		validCondition.Message = validationErr.Error()
	}
	apimeta.SetStatusCondition(&policy.Status.Conditions, validCondition)

	readyCondition := metav1.Condition{
		Type:               "Ready",
		ObservedGeneration: policy.Generation,
		LastTransitionTime: now,
	}
	switch {
	case !isValid:
		readyCondition.Status = metav1.ConditionFalse
		readyCondition.Reason = "ValidationFailed"
		readyCondition.Message = validationErr.Error()
	case policy.Status.ActiveSingleton:
		readyCondition.Status = metav1.ConditionTrue
		readyCondition.Reason = "SingletonActive"
		readyCondition.Message = "PullSecretPolicy is the active singleton policy"
	default:
		readyCondition.Status = metav1.ConditionFalse
		readyCondition.Reason = "NonSingletonName"
		readyCondition.Message = fmt.Sprintf("PullSecretPolicy must be named %q to be active", pullsecretsv1alpha1.PullSecretPolicySingletonName)
	}
	apimeta.SetStatusCondition(&policy.Status.Conditions, readyCondition)

	if err := r.Status().Update(ctx, policy); err != nil {
		return fmt.Errorf("update PullSecretPolicy status %s: %w", client.ObjectKeyFromObject(policy), err)
	}

	return nil
}
