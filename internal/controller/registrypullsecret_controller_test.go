package controller

import (
	"context"
	"fmt"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	pullsecretsv1alpha1 "github.com/mwognicki/pull-secrets-operator/api/pullsecrets/v1alpha1"
	"github.com/mwognicki/pull-secrets-operator/internal/sync"
	"github.com/mwognicki/pull-secrets-operator/pkg/metadata"
)

func TestRegistryPullSecretReconcileIgnoresMissingResource(t *testing.T) {
	t.Parallel()

	scheme := newTestScheme(t)
	reconciler := &RegistryPullSecretReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).Build(),
	}

	_, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "missing"},
	})
	if err != nil {
		t.Fatalf("Reconcile() error = %v, want nil", err)
	}
}

func TestRegistryPullSecretReconcileReturnsGetError(t *testing.T) {
	t.Parallel()

	scheme := newTestScheme(t)
	base := fake.NewClientBuilder().WithScheme(scheme).Build()
	reconciler := &RegistryPullSecretReconciler{
		Client: testClient{
			Client: base,
			getFunc: func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
				return fmt.Errorf("boom")
			},
		},
	}

	_, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "ghcr"},
	})
	if err == nil || !strings.Contains(err.Error(), "get RegistryPullSecret /ghcr") {
		t.Fatalf("Reconcile() error = %v, want wrapped get error", err)
	}
}

func TestRegistryPullSecretReconcileCreatesMissingSecrets(t *testing.T) {
	t.Parallel()

	scheme := newTestScheme(t)
	reconciler := &RegistryPullSecretReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&pullsecretsv1alpha1.RegistryPullSecret{}).WithObjects(
			&pullsecretsv1alpha1.RegistryPullSecret{
				ObjectMeta: metav1.ObjectMeta{Name: "ghcr"},
				Spec: pullsecretsv1alpha1.RegistryPullSecretSpec{
					Credentials: &pullsecretsv1alpha1.RegistryCredentials{
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
			Credentials: &pullsecretsv1alpha1.RegistryCredentials{
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

	want := mustDockerConfigJSON(t, *registryPullSecret.Spec.Credentials)
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
					Credentials: &pullsecretsv1alpha1.RegistryCredentials{
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
					Credentials: &pullsecretsv1alpha1.RegistryCredentials{
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
					Credentials: &pullsecretsv1alpha1.RegistryCredentials{
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
					Credentials: &pullsecretsv1alpha1.RegistryCredentials{
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

func TestRegistryPullSecretReconcileReturnsStatusUpdateError(t *testing.T) {
	t.Parallel()

	scheme := newTestScheme(t)
	base := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&pullsecretsv1alpha1.RegistryPullSecret{}).WithObjects(
		&pullsecretsv1alpha1.RegistryPullSecret{
			ObjectMeta: metav1.ObjectMeta{Name: "ghcr"},
			Spec: pullsecretsv1alpha1.RegistryPullSecretSpec{
				Credentials: &pullsecretsv1alpha1.RegistryCredentials{
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
	).Build()
	reconciler := &RegistryPullSecretReconciler{
		Client: testClient{
			Client:       base,
			statusWriter: testStatusWriter{SubResourceWriter: base.Status(), updateErr: fmt.Errorf("status update failed")},
		},
	}

	_, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "ghcr"},
	})
	if err == nil || !strings.Contains(err.Error(), "update RegistryPullSecret status /ghcr") {
		t.Fatalf("Reconcile() error = %v, want wrapped status update error", err)
	}
}

func TestRegistryPullSecretReconcileSupportsCredentialsSecretRef(t *testing.T) {
	t.Parallel()

	scheme := newTestScheme(t)
	reconciler := &RegistryPullSecretReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&pullsecretsv1alpha1.RegistryPullSecret{}).WithObjects(
			&pullsecretsv1alpha1.RegistryPullSecret{
				ObjectMeta: metav1.ObjectMeta{Name: "ghcr"},
				Spec: pullsecretsv1alpha1.RegistryPullSecretSpec{
					CredentialsSecretRef: &pullsecretsv1alpha1.SecretReference{
						Name:      "ghcr-creds",
						Namespace: "ops",
					},
					Namespaces: pullsecretsv1alpha1.NamespaceSelection{
						Policy:     pullsecretsv1alpha1.NamespaceSelectionPolicyInclusive,
						Namespaces: []string{"team-a"},
					},
				},
			},
			&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "team-a"}},
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "ghcr-creds", Namespace: "ops"},
				Data: map[string][]byte{
					"server":   []byte("ghcr.io"),
					"username": []byte("octocat"),
					"password": []byte("s3cret"),
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
	if secret.Labels[metadata.RegistryServerLabelKey] != "ghcr.io" {
		t.Fatalf("registry server label = %q, want ghcr.io", secret.Labels[metadata.RegistryServerLabelKey])
	}
}

func TestResolveRegistryCredentialsSupportsInlineCredentials(t *testing.T) {
	t.Parallel()

	scheme := newTestScheme(t)
	reconciler := &RegistryPullSecretReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).Build(),
	}

	credentials, err := reconciler.resolveRegistryCredentials(context.Background(), &pullsecretsv1alpha1.RegistryPullSecret{
		ObjectMeta: metav1.ObjectMeta{Name: "ghcr"},
		Spec: pullsecretsv1alpha1.RegistryPullSecretSpec{
			Credentials: &pullsecretsv1alpha1.RegistryCredentials{
				Server:   "ghcr.io",
				Username: "octocat",
				Password: "s3cret",
			},
			Namespaces: pullsecretsv1alpha1.NamespaceSelection{Policy: pullsecretsv1alpha1.NamespaceSelectionPolicyInclusive},
		},
	})
	if err != nil {
		t.Fatalf("resolveRegistryCredentials() error = %v", err)
	}
	if credentials.Server != "ghcr.io" {
		t.Fatalf("credentials = %#v, want ghcr.io server", credentials)
	}
}

func TestRegistryPullSecretsForSecret(t *testing.T) {
	t.Parallel()

	scheme := newTestScheme(t)
	reconciler := &RegistryPullSecretReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(
			&pullsecretsv1alpha1.RegistryPullSecret{
				ObjectMeta: metav1.ObjectMeta{Name: "ghcr"},
				Spec: pullsecretsv1alpha1.RegistryPullSecretSpec{
					CredentialsSecretRef: &pullsecretsv1alpha1.SecretReference{
						Name:      "ghcr-creds",
						Namespace: "ops",
					},
					Namespaces: pullsecretsv1alpha1.NamespaceSelection{Policy: pullsecretsv1alpha1.NamespaceSelectionPolicyInclusive},
				},
			},
			&pullsecretsv1alpha1.RegistryPullSecret{
				ObjectMeta: metav1.ObjectMeta{Name: "dockerhub"},
				Spec: pullsecretsv1alpha1.RegistryPullSecretSpec{
					CredentialsSecretRef: &pullsecretsv1alpha1.SecretReference{
						Name:      "dockerhub-creds",
						Namespace: "ops",
					},
					Namespaces: pullsecretsv1alpha1.NamespaceSelection{Policy: pullsecretsv1alpha1.NamespaceSelectionPolicyInclusive},
				},
			},
		).Build(),
	}

	requests := reconciler.registryPullSecretsForSecret(context.Background(), &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "ghcr-creds", Namespace: "ops"},
	})

	if len(requests) != 1 || requests[0].Name != "ghcr" {
		t.Fatalf("requests = %#v, want single ghcr request", requests)
	}
}

func TestRegistryPullSecretsForSecretIgnoresOtherObjects(t *testing.T) {
	t.Parallel()

	scheme := newTestScheme(t)
	reconciler := &RegistryPullSecretReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).Build(),
	}

	requests := reconciler.registryPullSecretsForSecret(context.Background(), &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: "team-a"},
	})
	if requests != nil {
		t.Fatalf("requests = %#v, want nil", requests)
	}
}

func TestRegistryPullSecretsForSecretIgnoresManagedReplicaSecret(t *testing.T) {
	t.Parallel()

	scheme := newTestScheme(t)
	reconciler := &RegistryPullSecretReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(
			&pullsecretsv1alpha1.RegistryPullSecret{
				ObjectMeta: metav1.ObjectMeta{Name: "ghcr"},
				Spec: pullsecretsv1alpha1.RegistryPullSecretSpec{
					CredentialsSecretRef: &pullsecretsv1alpha1.SecretReference{
						Name:      "ghcr-creds",
						Namespace: "ops",
					},
					Namespaces: pullsecretsv1alpha1.NamespaceSelection{Policy: pullsecretsv1alpha1.NamespaceSelectionPolicyInclusive},
				},
			},
		).Build(),
	}

	requests := reconciler.registryPullSecretsForSecret(context.Background(), &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ghcr-pull-secret",
			Namespace: "team-a",
			Labels: map[string]string{
				metadata.ManagedByLabelKey:              metadata.ManagedByLabelValue,
				metadata.RegistryPullSecretNameLabelKey: "ghcr",
				metadata.RegistryServerLabelKey:         "ghcr.io",
			},
		},
	})
	if len(requests) != 0 {
		t.Fatalf("requests = %#v, want no requests", requests)
	}
}

func TestRegistryPullSecretsForSecretReturnsEmptyOnListError(t *testing.T) {
	t.Parallel()

	scheme := newTestScheme(t)
	base := fake.NewClientBuilder().WithScheme(scheme).Build()
	reconciler := &RegistryPullSecretReconciler{
		Client: testClient{
			Client: base,
			listFunc: func(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
				return fmt.Errorf("list failed")
			},
		},
	}

	requests := reconciler.registryPullSecretsForSecret(context.Background(), &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "ghcr-creds", Namespace: "ops"},
	})
	if requests != nil {
		t.Fatalf("requests = %#v, want nil on list error", requests)
	}
}

func TestGetPullSecretPolicyReturnsEmptyWhenSingletonMissing(t *testing.T) {
	t.Parallel()

	scheme := newTestScheme(t)
	reconciler := &RegistryPullSecretReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).Build(),
	}

	policy, err := reconciler.getPullSecretPolicy(context.Background())
	if err != nil {
		t.Fatalf("getPullSecretPolicy() error = %v", err)
	}
	if policy.Name != "" {
		t.Fatalf("policy name = %q, want empty", policy.Name)
	}
}

func TestListExistingSecretsIndexesByNamespaceAndName(t *testing.T) {
	t.Parallel()

	scheme := newTestScheme(t)
	reconciler := &RegistryPullSecretReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(
			&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "one", Namespace: "team-a"}},
			&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "two", Namespace: "team-b"}},
		).Build(),
	}

	secrets, err := reconciler.listExistingSecrets(context.Background())
	if err != nil {
		t.Fatalf("listExistingSecrets() error = %v", err)
	}

	if _, ok := secrets["team-a/one"]; !ok {
		t.Fatalf("expected team-a/one key in %#v", secrets)
	}
	if _, ok := secrets["team-b/two"]; !ok {
		t.Fatalf("expected team-b/two key in %#v", secrets)
	}
}

func TestListNamespacesReturnsNames(t *testing.T) {
	t.Parallel()

	scheme := newTestScheme(t)
	reconciler := &RegistryPullSecretReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(
			&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "team-a"}},
			&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "team-b"}},
		).Build(),
	}

	namespaces, err := reconciler.listNamespaces(context.Background())
	if err != nil {
		t.Fatalf("listNamespaces() error = %v", err)
	}
	if len(namespaces) != 2 {
		t.Fatalf("namespaces = %#v, want 2 entries", namespaces)
	}
}

func TestGetPullSecretPolicyReturnsWrappedError(t *testing.T) {
	t.Parallel()

	scheme := newTestScheme(t)
	base := fake.NewClientBuilder().WithScheme(scheme).Build()
	reconciler := &RegistryPullSecretReconciler{
		Client: testClient{
			Client: base,
			getFunc: func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
				if key.Name == pullsecretsv1alpha1.PullSecretPolicySingletonName {
					return fmt.Errorf("policy get failed")
				}
				return base.Get(ctx, key, obj, opts...)
			},
		},
	}

	_, err := reconciler.getPullSecretPolicy(context.Background())
	if err == nil || !strings.Contains(err.Error(), "get PullSecretPolicy") {
		t.Fatalf("getPullSecretPolicy() error = %v, want wrapped error", err)
	}
}

func TestListNamespacesReturnsWrappedError(t *testing.T) {
	t.Parallel()

	scheme := newTestScheme(t)
	base := fake.NewClientBuilder().WithScheme(scheme).Build()
	reconciler := &RegistryPullSecretReconciler{
		Client: testClient{
			Client: base,
			listFunc: func(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
				if _, ok := list.(*corev1.NamespaceList); ok {
					return fmt.Errorf("namespace list failed")
				}
				return base.List(ctx, list, opts...)
			},
		},
	}

	_, err := reconciler.listNamespaces(context.Background())
	if err == nil || !strings.Contains(err.Error(), "list namespaces") {
		t.Fatalf("listNamespaces() error = %v, want wrapped error", err)
	}
}

func TestListExistingSecretsReturnsWrappedError(t *testing.T) {
	t.Parallel()

	scheme := newTestScheme(t)
	base := fake.NewClientBuilder().WithScheme(scheme).Build()
	reconciler := &RegistryPullSecretReconciler{
		Client: testClient{
			Client: base,
			listFunc: func(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
				if _, ok := list.(*corev1.SecretList); ok {
					return fmt.Errorf("secret list failed")
				}
				return base.List(ctx, list, opts...)
			},
		},
	}

	_, err := reconciler.listExistingSecrets(context.Background())
	if err == nil || !strings.Contains(err.Error(), "list secrets") {
		t.Fatalf("listExistingSecrets() error = %v, want wrapped error", err)
	}
}

func TestApplySecretCreatesSecretWhenMissing(t *testing.T) {
	t.Parallel()

	scheme := newTestScheme(t)
	reconciler := &RegistryPullSecretReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).Build(),
	}

	err := reconciler.applySecret(context.Background(), &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "ghcr-pull-secret", Namespace: "team-a"},
		Type:       corev1.SecretTypeDockerConfigJson,
		Data:       map[string][]byte{".dockerconfigjson": []byte("data")},
	})
	if err != nil {
		t.Fatalf("applySecret() error = %v", err)
	}
}

func TestApplySecretReturnsGetError(t *testing.T) {
	t.Parallel()

	scheme := newTestScheme(t)
	base := fake.NewClientBuilder().WithScheme(scheme).Build()
	reconciler := &RegistryPullSecretReconciler{
		Client: testClient{
			Client: base,
			getFunc: func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
				if _, ok := obj.(*corev1.Secret); ok {
					return fmt.Errorf("get failed")
				}
				return base.Get(ctx, key, obj, opts...)
			},
		},
	}

	err := reconciler.applySecret(context.Background(), &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "ghcr-pull-secret", Namespace: "team-a"},
	})
	if err == nil || !strings.Contains(err.Error(), "get Secret team-a/ghcr-pull-secret") {
		t.Fatalf("applySecret() error = %v, want wrapped get error", err)
	}
}

func TestApplySecretReturnsCreateError(t *testing.T) {
	t.Parallel()

	scheme := newTestScheme(t)
	base := fake.NewClientBuilder().WithScheme(scheme).Build()
	reconciler := &RegistryPullSecretReconciler{
		Client: testClient{
			Client: base,
			createFunc: func(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
				return fmt.Errorf("create failed")
			},
		},
	}

	err := reconciler.applySecret(context.Background(), &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "ghcr-pull-secret", Namespace: "team-a"},
	})
	if err == nil || !strings.Contains(err.Error(), "create Secret team-a/ghcr-pull-secret") {
		t.Fatalf("applySecret() error = %v, want wrapped create error", err)
	}
}

func TestApplySecretReturnsUpdateError(t *testing.T) {
	t.Parallel()

	scheme := newTestScheme(t)
	base := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "ghcr-pull-secret", Namespace: "team-a"}},
	).Build()
	reconciler := &RegistryPullSecretReconciler{
		Client: testClient{
			Client: base,
			updateFunc: func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
				return fmt.Errorf("update failed")
			},
		},
	}

	err := reconciler.applySecret(context.Background(), &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "ghcr-pull-secret", Namespace: "team-a"},
	})
	if err == nil || !strings.Contains(err.Error(), "update Secret team-a/ghcr-pull-secret") {
		t.Fatalf("applySecret() error = %v, want wrapped update error", err)
	}
}

func TestDeleteSecretIgnoresNotFound(t *testing.T) {
	t.Parallel()

	scheme := newTestScheme(t)
	base := fake.NewClientBuilder().WithScheme(scheme).Build()
	reconciler := &RegistryPullSecretReconciler{
		Client: testClient{
			Client: base,
			deleteFunc: func(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
				return apierrors.NewNotFound(schema.GroupResource{Group: "", Resource: "secrets"}, obj.GetName())
			},
		},
	}

	if err := reconciler.deleteSecret(context.Background(), &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "ghcr-pull-secret", Namespace: "team-a"},
	}); err != nil {
		t.Fatalf("deleteSecret() error = %v, want nil", err)
	}
}

func TestDeleteSecretReturnsWrappedError(t *testing.T) {
	t.Parallel()

	scheme := newTestScheme(t)
	base := fake.NewClientBuilder().WithScheme(scheme).Build()
	reconciler := &RegistryPullSecretReconciler{
		Client: testClient{
			Client: base,
			deleteFunc: func(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
				return fmt.Errorf("delete failed")
			},
		},
	}

	err := reconciler.deleteSecret(context.Background(), &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "ghcr-pull-secret", Namespace: "team-a"},
	})
	if err == nil || !strings.Contains(err.Error(), "delete Secret team-a/ghcr-pull-secret") {
		t.Fatalf("deleteSecret() error = %v, want wrapped delete error", err)
	}
}

func TestUpdateRegistryPullSecretStatusSuccess(t *testing.T) {
	t.Parallel()

	scheme := newTestScheme(t)
	base := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&pullsecretsv1alpha1.RegistryPullSecret{}).WithObjects(
		&pullsecretsv1alpha1.RegistryPullSecret{
			ObjectMeta: metav1.ObjectMeta{Name: "ghcr", Generation: 5},
		},
	).Build()
	reconciler := &RegistryPullSecretReconciler{Client: base}

	resource := &pullsecretsv1alpha1.RegistryPullSecret{}
	if err := base.Get(context.Background(), types.NamespacedName{Name: "ghcr"}, resource); err != nil {
		t.Fatalf("get RegistryPullSecret error = %v", err)
	}
	err := reconciler.updateRegistryPullSecretStatus(context.Background(), resource, registryPullSecretReconcileStatus{
		desiredSecretCount: 3,
		appliedSecretCount: 2,
		deletedSecretCount: 1,
	}, nil)
	if err != nil {
		t.Fatalf("updateRegistryPullSecretStatus() error = %v", err)
	}

	var updated pullsecretsv1alpha1.RegistryPullSecret
	if err := base.Get(context.Background(), types.NamespacedName{Name: "ghcr"}, &updated); err != nil {
		t.Fatalf("get updated RegistryPullSecret error = %v", err)
	}
	if updated.Status.DesiredSecretCount != 3 || updated.Status.AppliedSecretCount != 2 || updated.Status.DeletedSecretCount != 1 {
		t.Fatalf("status counts = %#v", updated.Status)
	}
	if cond := apimeta.FindStatusCondition(updated.Status.Conditions, "Ready"); cond == nil || cond.Status != metav1.ConditionTrue {
		t.Fatalf("Ready condition = %#v, want true", updated.Status.Conditions)
	}
}

func TestUpdateRegistryPullSecretStatusReturnsWrappedError(t *testing.T) {
	t.Parallel()

	scheme := newTestScheme(t)
	base := fake.NewClientBuilder().WithScheme(scheme).Build()
	reconciler := &RegistryPullSecretReconciler{
		Client: testClient{
			Client:       base,
			statusWriter: testStatusWriter{updateErr: fmt.Errorf("status failed")},
		},
	}

	err := reconciler.updateRegistryPullSecretStatus(context.Background(), &pullsecretsv1alpha1.RegistryPullSecret{
		ObjectMeta: metav1.ObjectMeta{Name: "ghcr"},
	}, registryPullSecretReconcileStatus{}, nil)
	if err == nil || !strings.Contains(err.Error(), "update RegistryPullSecret status /ghcr") {
		t.Fatalf("updateRegistryPullSecretStatus() error = %v, want wrapped status error", err)
	}
}

func TestResolveRegistryCredentialsReturnsMissingSecretError(t *testing.T) {
	t.Parallel()

	scheme := newTestScheme(t)
	reconciler := &RegistryPullSecretReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).Build(),
	}

	_, err := reconciler.resolveRegistryCredentials(context.Background(), &pullsecretsv1alpha1.RegistryPullSecret{
		ObjectMeta: metav1.ObjectMeta{Name: "ghcr"},
		Spec: pullsecretsv1alpha1.RegistryPullSecretSpec{
			CredentialsSecretRef: &pullsecretsv1alpha1.SecretReference{
				Name:      "missing",
				Namespace: "ops",
			},
			Namespaces: pullsecretsv1alpha1.NamespaceSelection{
				Policy: pullsecretsv1alpha1.NamespaceSelectionPolicyInclusive,
			},
		},
	})
	if err == nil {
		t.Fatal("resolveRegistryCredentials() error = nil, want missing Secret error")
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

type testClient struct {
	client.Client
	getFunc      func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error
	listFunc     func(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error
	createFunc   func(ctx context.Context, obj client.Object, opts ...client.CreateOption) error
	updateFunc   func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error
	deleteFunc   func(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error
	statusWriter client.SubResourceWriter
}

func (c testClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	if c.getFunc != nil {
		return c.getFunc(ctx, key, obj, opts...)
	}
	return c.Client.Get(ctx, key, obj, opts...)
}

func (c testClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	if c.listFunc != nil {
		return c.listFunc(ctx, list, opts...)
	}
	return c.Client.List(ctx, list, opts...)
}

func (c testClient) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	if c.createFunc != nil {
		return c.createFunc(ctx, obj, opts...)
	}
	return c.Client.Create(ctx, obj, opts...)
}

func (c testClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	if c.updateFunc != nil {
		return c.updateFunc(ctx, obj, opts...)
	}
	return c.Client.Update(ctx, obj, opts...)
}

func (c testClient) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	if c.deleteFunc != nil {
		return c.deleteFunc(ctx, obj, opts...)
	}
	return c.Client.Delete(ctx, obj, opts...)
}

func (c testClient) Status() client.SubResourceWriter {
	if c.statusWriter != nil {
		return c.statusWriter
	}
	return c.Client.Status()
}

type testStatusWriter struct {
	client.SubResourceWriter
	updateErr error
}

func (w testStatusWriter) Create(ctx context.Context, obj client.Object, subResource client.Object, opts ...client.SubResourceCreateOption) error {
	if w.SubResourceWriter != nil {
		return w.SubResourceWriter.Create(ctx, obj, subResource, opts...)
	}
	return nil
}

func (w testStatusWriter) Update(ctx context.Context, obj client.Object, opts ...client.SubResourceUpdateOption) error {
	if w.updateErr != nil {
		return w.updateErr
	}
	if w.SubResourceWriter != nil {
		return w.SubResourceWriter.Update(ctx, obj, opts...)
	}
	return nil
}

func (w testStatusWriter) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.SubResourcePatchOption) error {
	if w.SubResourceWriter != nil {
		return w.SubResourceWriter.Patch(ctx, obj, patch, opts...)
	}
	return nil
}
