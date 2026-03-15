package controller

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	pullsecretsv1alpha1 "github.com/mwognicki/pull-secrets-operator/api/pullsecrets/v1alpha1"
	"github.com/mwognicki/pull-secrets-operator/internal/sync"
	"github.com/mwognicki/pull-secrets-operator/pkg/metadata"
)

func TestRegistryPullSecretReconcileCreatesMissingSecrets(t *testing.T) {
	t.Parallel()

	scheme := newTestScheme(t)
	reconciler := &RegistryPullSecretReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&pullsecretsv1alpha1.RegistryPullSecret{}).WithObjects(
			&pullsecretsv1alpha1.RegistryPullSecret{
				ObjectMeta: metav1.ObjectMeta{Name: "ghcr"},
				Spec: pullsecretsv1alpha1.RegistryPullSecretSpec{
					Credentials: pullsecretsv1alpha1.RegistryCredentials{
						Server:   "ghcr.io",
						Username: "octocat",
						Password: "s3cret",
					},
					Namespaces: pullsecretsv1alpha1.NamespaceSelection{
						Policy:     pullsecretsv1alpha1.NamespaceSelectionPolicyInclusive,
						Namespaces: []string{"team-a"},
					},
				},
			},
			&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "team-a"}},
		).Build(),
	}

	_, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "ghcr"},
	})
	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	var secret corev1.Secret
	if err := reconciler.Get(context.Background(), types.NamespacedName{Name: "ghcr-pull-secret", Namespace: "team-a"}, &secret); err != nil {
		t.Fatalf("get reconciled Secret error = %v", err)
	}

	if secret.Labels[metadata.RegistryPullSecretNameLabelKey] != "ghcr" {
		t.Fatalf("source label = %q, want %q", secret.Labels[metadata.RegistryPullSecretNameLabelKey], "ghcr")
	}

	var registryPullSecret pullsecretsv1alpha1.RegistryPullSecret
	if err := reconciler.Get(context.Background(), types.NamespacedName{Name: "ghcr"}, &registryPullSecret); err != nil {
		t.Fatalf("get RegistryPullSecret error = %v", err)
	}
	if registryPullSecret.Status.DesiredSecretCount != 1 {
		t.Fatalf("desiredSecretCount = %d, want 1", registryPullSecret.Status.DesiredSecretCount)
	}
	if registryPullSecret.Status.AppliedSecretCount != 1 {
		t.Fatalf("appliedSecretCount = %d, want 1", registryPullSecret.Status.AppliedSecretCount)
	}
	if len(registryPullSecret.Status.Conditions) != 1 || registryPullSecret.Status.Conditions[0].Status != metav1.ConditionTrue {
		t.Fatalf("status conditions = %#v, want Ready=True", registryPullSecret.Status.Conditions)
	}
}

func TestRegistryPullSecretReconcileUpdatesExistingSecret(t *testing.T) {
	t.Parallel()

	scheme := newTestScheme(t)
	existingData := mustDockerConfigJSON(t, pullsecretsv1alpha1.RegistryCredentials{
		Server:   "ghcr.io",
		Username: "octocat",
		Password: "old-secret",
	})

	registryPullSecret := &pullsecretsv1alpha1.RegistryPullSecret{
		ObjectMeta: metav1.ObjectMeta{Name: "ghcr"},
		Spec: pullsecretsv1alpha1.RegistryPullSecretSpec{
			Credentials: pullsecretsv1alpha1.RegistryCredentials{
				Server:   "ghcr.io",
				Username: "octocat",
				Password: "new-secret",
			},
			Namespaces: pullsecretsv1alpha1.NamespaceSelection{
				Policy:     pullsecretsv1alpha1.NamespaceSelectionPolicyInclusive,
				Namespaces: []string{"team-a"},
			},
		},
	}

	reconciler := &RegistryPullSecretReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&pullsecretsv1alpha1.RegistryPullSecret{}).WithObjects(
			registryPullSecret,
			&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "team-a"}},
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "ghcr-pull-secret",
					Namespace: "team-a",
					Labels: map[string]string{
						metadata.ManagedByLabelKey:              metadata.ManagedByLabelValue,
						metadata.RegistryPullSecretNameLabelKey: "ghcr",
						metadata.RegistryServerLabelKey:         "ghcr.io",
					},
				},
				Type: corev1.SecretTypeDockerConfigJson,
				Data: map[string][]byte{
					".dockerconfigjson": existingData,
				},
			},
		).Build(),
	}

	_, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "ghcr"},
	})
	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	var secret corev1.Secret
	if err := reconciler.Get(context.Background(), types.NamespacedName{Name: "ghcr-pull-secret", Namespace: "team-a"}, &secret); err != nil {
		t.Fatalf("get reconciled Secret error = %v", err)
	}

	want := mustDockerConfigJSON(t, registryPullSecret.Spec.Credentials)
	got := secret.Data[".dockerconfigjson"]
	if string(got) != string(want) {
		t.Fatalf("reconciled docker config = %s, want %s", string(got), string(want))
	}
}

func TestRegistryPullSecretReconcileRespectsClusterWideExclusions(t *testing.T) {
	t.Parallel()

	scheme := newTestScheme(t)
	reconciler := &RegistryPullSecretReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&pullsecretsv1alpha1.RegistryPullSecret{}).WithObjects(
			&pullsecretsv1alpha1.PullSecretPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: pullsecretsv1alpha1.PullSecretPolicySingletonName},
				Spec: pullsecretsv1alpha1.PullSecretPolicySpec{
					ExcludedNamespaces: []string{"team-a"},
				},
			},
			&pullsecretsv1alpha1.RegistryPullSecret{
				ObjectMeta: metav1.ObjectMeta{Name: "ghcr"},
				Spec: pullsecretsv1alpha1.RegistryPullSecretSpec{
					Credentials: pullsecretsv1alpha1.RegistryCredentials{
						Server:   "ghcr.io",
						Username: "octocat",
						Password: "s3cret",
					},
					Namespaces: pullsecretsv1alpha1.NamespaceSelection{
						Policy:     pullsecretsv1alpha1.NamespaceSelectionPolicyInclusive,
						Namespaces: []string{"team-a"},
					},
				},
			},
			&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "team-a"}},
		).Build(),
	}

	_, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "ghcr"},
	})
	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	var secret corev1.Secret
	err = reconciler.Get(context.Background(), types.NamespacedName{Name: "ghcr-pull-secret", Namespace: "team-a"}, &secret)
	if !apierrors.IsNotFound(err) {
		t.Fatalf("expected excluded namespace Secret to be absent, got err=%v", err)
	}
}

func TestRegistryPullSecretReconcileDeletesRenamedSecret(t *testing.T) {
	t.Parallel()

	scheme := newTestScheme(t)
	reconciler := &RegistryPullSecretReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&pullsecretsv1alpha1.RegistryPullSecret{}).WithObjects(
			&pullsecretsv1alpha1.RegistryPullSecret{
				ObjectMeta: metav1.ObjectMeta{Name: "ghcr"},
				Spec: pullsecretsv1alpha1.RegistryPullSecretSpec{
					Credentials: pullsecretsv1alpha1.RegistryCredentials{
						Server:   "ghcr.io",
						Username: "octocat",
						Password: "s3cret",
					},
					Namespaces: pullsecretsv1alpha1.NamespaceSelection{
						Policy:           pullsecretsv1alpha1.NamespaceSelectionPolicyInclusive,
						Namespaces:       []string{"team-a"},
						TargetSecretName: "new-ghcr-secret",
					},
				},
			},
			&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "team-a"}},
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "ghcr-pull-secret",
					Namespace: "team-a",
					Labels: map[string]string{
						metadata.ManagedByLabelKey:              metadata.ManagedByLabelValue,
						metadata.RegistryPullSecretNameLabelKey: "ghcr",
						metadata.RegistryServerLabelKey:         "ghcr.io",
					},
				},
				Type: corev1.SecretTypeDockerConfigJson,
				Data: map[string][]byte{
					".dockerconfigjson": mustDockerConfigJSON(t, pullsecretsv1alpha1.RegistryCredentials{
						Server:   "ghcr.io",
						Username: "octocat",
						Password: "s3cret",
					}),
				},
			},
		).Build(),
	}

	_, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "ghcr"},
	})
	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	var newSecret corev1.Secret
	if err := reconciler.Get(context.Background(), types.NamespacedName{Name: "new-ghcr-secret", Namespace: "team-a"}, &newSecret); err != nil {
		t.Fatalf("get new Secret error = %v", err)
	}

	var oldSecret corev1.Secret
	err = reconciler.Get(context.Background(), types.NamespacedName{Name: "ghcr-pull-secret", Namespace: "team-a"}, &oldSecret)
	if !apierrors.IsNotFound(err) {
		t.Fatalf("expected old Secret to be deleted, got err=%v", err)
	}
}

func TestRegistryPullSecretReconcileDeletesRemovedNamespaceSecret(t *testing.T) {
	t.Parallel()

	scheme := newTestScheme(t)
	reconciler := &RegistryPullSecretReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&pullsecretsv1alpha1.RegistryPullSecret{}).WithObjects(
			&pullsecretsv1alpha1.RegistryPullSecret{
				ObjectMeta: metav1.ObjectMeta{Name: "ghcr"},
				Spec: pullsecretsv1alpha1.RegistryPullSecretSpec{
					Credentials: pullsecretsv1alpha1.RegistryCredentials{
						Server:   "ghcr.io",
						Username: "octocat",
						Password: "s3cret",
					},
					Namespaces: pullsecretsv1alpha1.NamespaceSelection{
						Policy:     pullsecretsv1alpha1.NamespaceSelectionPolicyInclusive,
						Namespaces: []string{"team-a"},
					},
				},
			},
			&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "team-a"}},
			&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "team-b"}},
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "ghcr-pull-secret",
					Namespace: "team-b",
					Labels: map[string]string{
						metadata.ManagedByLabelKey:              metadata.ManagedByLabelValue,
						metadata.RegistryPullSecretNameLabelKey: "ghcr",
						metadata.RegistryServerLabelKey:         "ghcr.io",
					},
				},
				Type: corev1.SecretTypeDockerConfigJson,
				Data: map[string][]byte{
					".dockerconfigjson": mustDockerConfigJSON(t, pullsecretsv1alpha1.RegistryCredentials{
						Server:   "ghcr.io",
						Username: "octocat",
						Password: "s3cret",
					}),
				},
			},
		).Build(),
	}

	_, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "ghcr"},
	})
	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	var secret corev1.Secret
	if err := reconciler.Get(context.Background(), types.NamespacedName{Name: "ghcr-pull-secret", Namespace: "team-a"}, &secret); err != nil {
		t.Fatalf("expected desired team-a Secret, got err=%v", err)
	}

	err = reconciler.Get(context.Background(), types.NamespacedName{Name: "ghcr-pull-secret", Namespace: "team-b"}, &secret)
	if !apierrors.IsNotFound(err) {
		t.Fatalf("expected team-b Secret to be deleted, got err=%v", err)
	}
}

func TestRegistryPullSecretReconcileUpdatesFailureStatus(t *testing.T) {
	t.Parallel()

	scheme := newTestScheme(t)
	reconciler := &RegistryPullSecretReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&pullsecretsv1alpha1.RegistryPullSecret{}).WithObjects(
			&pullsecretsv1alpha1.RegistryPullSecret{
				ObjectMeta: metav1.ObjectMeta{Name: "broken", Generation: 7},
				Spec: pullsecretsv1alpha1.RegistryPullSecretSpec{
					Credentials: pullsecretsv1alpha1.RegistryCredentials{
						Server: "",
					},
					Namespaces: pullsecretsv1alpha1.NamespaceSelection{
						Policy:     pullsecretsv1alpha1.NamespaceSelectionPolicyInclusive,
						Namespaces: []string{"team-a"},
					},
				},
			},
			&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "team-a"}},
		).Build(),
	}

	_, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "broken"},
	})
	if err == nil {
		t.Fatalf("expected reconcile error for broken credentials")
	}

	var registryPullSecret pullsecretsv1alpha1.RegistryPullSecret
	if err := reconciler.Get(context.Background(), types.NamespacedName{Name: "broken"}, &registryPullSecret); err != nil {
		t.Fatalf("get RegistryPullSecret error = %v", err)
	}
	if registryPullSecret.Status.ObservedGeneration != 7 {
		t.Fatalf("observedGeneration = %d, want 7", registryPullSecret.Status.ObservedGeneration)
	}
	if len(registryPullSecret.Status.Conditions) != 1 || registryPullSecret.Status.Conditions[0].Status != metav1.ConditionFalse {
		t.Fatalf("status conditions = %#v, want Ready=False", registryPullSecret.Status.Conditions)
	}
}

func newTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()

	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("corev1.AddToScheme() error = %v", err)
	}
	if err := pullsecretsv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("pullsecrets AddToScheme() error = %v", err)
	}

	return scheme
}

func mustDockerConfigJSON(t *testing.T, credentials pullsecretsv1alpha1.RegistryCredentials) []byte {
	t.Helper()

	rendered, err := sync.DockerConfigJSON(credentials)
	if err != nil {
		t.Fatalf("DockerConfigJSON() error = %v", err)
	}

	return rendered
}
