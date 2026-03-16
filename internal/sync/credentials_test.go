package sync

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	pullsecretsv1alpha1 "github.com/mwognicki/pull-secrets-operator/api/pullsecrets/v1alpha1"
)

func TestResolveRegistryCredentialsInline(t *testing.T) {
	t.Parallel()

	got, err := ResolveRegistryCredentials(pullsecretsv1alpha1.RegistryPullSecretSpec{
		Credentials: &pullsecretsv1alpha1.RegistryCredentials{
			Server:   "ghcr.io",
			Username: "octocat",
			Password: "s3cret",
			Email:    "ops@example.com",
		},
	}, nil)
	if err != nil {
		t.Fatalf("ResolveRegistryCredentials() error = %v", err)
	}
	if got.Server != "ghcr.io" || got.Username != "octocat" || got.Password != "s3cret" {
		t.Fatalf("resolved credentials = %#v", got)
	}
}

func TestResolveRegistryCredentialsFromSecret(t *testing.T) {
	t.Parallel()

	got, err := ResolveRegistryCredentials(
		pullsecretsv1alpha1.RegistryPullSecretSpec{
			CredentialsSecretRef: &pullsecretsv1alpha1.SecretReference{
				Name:      "registry-creds",
				Namespace: "ops",
			},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "registry-creds", Namespace: "ops"},
			Data: map[string][]byte{
				"server":   []byte("ghcr.io"),
				"username": []byte("octocat"),
				"password": []byte("s3cret"),
				"email":    []byte("ops@example.com"),
			},
		},
	)
	if err != nil {
		t.Fatalf("ResolveRegistryCredentials() error = %v", err)
	}
	if got.Server != "ghcr.io" || got.Username != "octocat" || got.Password != "s3cret" {
		t.Fatalf("resolved credentials = %#v", got)
	}
}

func TestResolveRegistryCredentialsRejectsAmbiguousModes(t *testing.T) {
	t.Parallel()

	_, err := ResolveRegistryCredentials(pullsecretsv1alpha1.RegistryPullSecretSpec{
		Credentials: &pullsecretsv1alpha1.RegistryCredentials{
			Server:   "ghcr.io",
			Username: "octocat",
			Password: "s3cret",
		},
		CredentialsSecretRef: &pullsecretsv1alpha1.SecretReference{
			Name:      "registry-creds",
			Namespace: "ops",
		},
	}, nil)
	if err == nil {
		t.Fatalf("expected ambiguity error")
	}
}

func TestResolveRegistryCredentialsRejectsMissingSecretKeys(t *testing.T) {
	t.Parallel()

	_, err := ResolveRegistryCredentials(
		pullsecretsv1alpha1.RegistryPullSecretSpec{
			CredentialsSecretRef: &pullsecretsv1alpha1.SecretReference{
				Name:      "registry-creds",
				Namespace: "ops",
			},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "registry-creds", Namespace: "ops"},
			Data: map[string][]byte{
				"server": []byte("ghcr.io"),
			},
		},
	)
	if err == nil {
		t.Fatalf("expected missing-key error")
	}
}
