package controller

import (
	"context"
	"testing"

	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	pullsecretsv1alpha1 "github.com/mwognicki/pull-secrets-operator/api/pullsecrets/v1alpha1"
)

func TestPullSecretPolicyReconcileUpdatesSingletonStatus(t *testing.T) {
	t.Parallel()

	scheme := newTestScheme(t)
	reconciler := &PullSecretPolicyReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&pullsecretsv1alpha1.PullSecretPolicy{}).WithObjects(
			&pullsecretsv1alpha1.PullSecretPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: pullsecretsv1alpha1.PullSecretPolicySingletonName, Generation: 3},
				Spec: pullsecretsv1alpha1.PullSecretPolicySpec{
					ExcludedNamespaces: []string{"kube-system", "cert-manager"},
				},
			},
		).Build(),
	}

	_, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: pullsecretsv1alpha1.PullSecretPolicySingletonName},
	})
	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	var policy pullsecretsv1alpha1.PullSecretPolicy
	if err := reconciler.Get(context.Background(), types.NamespacedName{Name: pullsecretsv1alpha1.PullSecretPolicySingletonName}, &policy); err != nil {
		t.Fatalf("get PullSecretPolicy error = %v", err)
	}

	if policy.Status.ObservedGeneration != 3 {
		t.Fatalf("observedGeneration = %d, want 3", policy.Status.ObservedGeneration)
	}
	if policy.Status.ExcludedNamespaceCount != 2 {
		t.Fatalf("excludedNamespaceCount = %d, want 2", policy.Status.ExcludedNamespaceCount)
	}
	if !policy.Status.ActiveSingleton {
		t.Fatalf("activeSingleton = false, want true")
	}
	if !policy.Status.Valid {
		t.Fatalf("valid = false, want true")
	}
	if cond := apimeta.FindStatusCondition(policy.Status.Conditions, "Ready"); cond == nil || cond.Status != metav1.ConditionTrue {
		t.Fatalf("Ready condition = %#v, want true", policy.Status.Conditions)
	}
	if cond := apimeta.FindStatusCondition(policy.Status.Conditions, "Valid"); cond == nil || cond.Status != metav1.ConditionTrue {
		t.Fatalf("Valid condition = %#v, want true", policy.Status.Conditions)
	}
}

func TestPullSecretPolicyReconcileMarksNonSingletonInactive(t *testing.T) {
	t.Parallel()

	scheme := newTestScheme(t)
	reconciler := &PullSecretPolicyReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&pullsecretsv1alpha1.PullSecretPolicy{}).WithObjects(
			&pullsecretsv1alpha1.PullSecretPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "custom-name", Generation: 1},
				Spec: pullsecretsv1alpha1.PullSecretPolicySpec{
					ExcludedNamespaces: []string{"kube-system"},
				},
			},
		).Build(),
	}

	_, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "custom-name"},
	})
	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	var policy pullsecretsv1alpha1.PullSecretPolicy
	if err := reconciler.Get(context.Background(), types.NamespacedName{Name: "custom-name"}, &policy); err != nil {
		t.Fatalf("get PullSecretPolicy error = %v", err)
	}

	if policy.Status.ActiveSingleton {
		t.Fatalf("activeSingleton = true, want false")
	}
	if !policy.Status.Valid {
		t.Fatalf("valid = false, want true")
	}
	if cond := apimeta.FindStatusCondition(policy.Status.Conditions, "Ready"); cond == nil || cond.Status != metav1.ConditionFalse {
		t.Fatalf("Ready condition = %#v, want false", policy.Status.Conditions)
	}
	if cond := apimeta.FindStatusCondition(policy.Status.Conditions, "Valid"); cond == nil || cond.Status != metav1.ConditionTrue {
		t.Fatalf("Valid condition = %#v, want true", policy.Status.Conditions)
	}
}

func TestPullSecretPolicyReconcileMarksInvalidPolicy(t *testing.T) {
	t.Parallel()

	scheme := newTestScheme(t)
	reconciler := &PullSecretPolicyReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&pullsecretsv1alpha1.PullSecretPolicy{}).WithObjects(
			&pullsecretsv1alpha1.PullSecretPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: pullsecretsv1alpha1.PullSecretPolicySingletonName, Generation: 4},
				Spec: pullsecretsv1alpha1.PullSecretPolicySpec{
					ExcludedNamespaces: []string{"bad namespace"},
				},
			},
		).Build(),
	}

	_, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: pullsecretsv1alpha1.PullSecretPolicySingletonName},
	})
	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	var policy pullsecretsv1alpha1.PullSecretPolicy
	if err := reconciler.Get(context.Background(), types.NamespacedName{Name: pullsecretsv1alpha1.PullSecretPolicySingletonName}, &policy); err != nil {
		t.Fatalf("get PullSecretPolicy error = %v", err)
	}

	if !policy.Status.ActiveSingleton {
		t.Fatalf("activeSingleton = false, want true")
	}
	if policy.Status.Valid {
		t.Fatalf("valid = true, want false")
	}
	if cond := apimeta.FindStatusCondition(policy.Status.Conditions, "Valid"); cond == nil || cond.Status != metav1.ConditionFalse {
		t.Fatalf("Valid condition = %#v, want false", policy.Status.Conditions)
	}
	if cond := apimeta.FindStatusCondition(policy.Status.Conditions, "Ready"); cond == nil || cond.Status != metav1.ConditionFalse {
		t.Fatalf("Ready condition = %#v, want false", policy.Status.Conditions)
	} else if cond.Reason != "ValidationFailed" {
		t.Fatalf("Ready reason = %q, want ValidationFailed", cond.Reason)
	}
}
