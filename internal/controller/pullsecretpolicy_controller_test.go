package controller

import (
	"context"
	"testing"

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
	if len(policy.Status.Conditions) != 1 || policy.Status.Conditions[0].Status != metav1.ConditionTrue {
		t.Fatalf("status conditions = %#v, want Ready=True", policy.Status.Conditions)
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
	if len(policy.Status.Conditions) != 1 || policy.Status.Conditions[0].Status != metav1.ConditionFalse {
		t.Fatalf("status conditions = %#v, want Ready=False", policy.Status.Conditions)
	}
}
