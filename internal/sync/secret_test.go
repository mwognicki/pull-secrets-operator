package sync

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	pullsecretsv1alpha1 "github.com/mwognicki/pull-secrets-operator/api/pullsecrets/v1alpha1"
	"github.com/mwognicki/pull-secrets-operator/pkg/metadata"
)

func TestDockerConfigJSON(t *testing.T) {
	t.Parallel()

	rendered, err := DockerConfigJSON(pullsecretsv1alpha1.RegistryCredentials{
		Server:   "ghcr.io",
		Username: "octocat",
		Password: "s3cret",
		Email:    "ops@example.com",
	})
	if err != nil {
		t.Fatalf("DockerConfigJSON() error = %v", err)
	}

	var payload map[string]map[string]map[string]string
	if err := json.Unmarshal(rendered, &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	entry := payload["auths"]["ghcr.io"]
	if entry["username"] != "octocat" {
		t.Fatalf("username = %q, want %q", entry["username"], "octocat")
	}
	if entry["password"] != "s3cret" {
		t.Fatalf("password = %q, want %q", entry["password"], "s3cret")
	}
	if entry["auth"] != base64.StdEncoding.EncodeToString([]byte("octocat:s3cret")) {
		t.Fatalf("auth = %q, want base64 user:pass", entry["auth"])
	}
}

func TestDockerConfigJSONWithExplicitAuth(t *testing.T) {
	t.Parallel()

	rendered, err := DockerConfigJSON(pullsecretsv1alpha1.RegistryCredentials{
		Server:   "ghcr.io",
		Username: "octocat",
		Password: "s3cret",
		Auth:     "preset-auth",
	})
	if err != nil {
		t.Fatalf("DockerConfigJSON() error = %v", err)
	}

	var payload map[string]map[string]map[string]string
	if err := json.Unmarshal(rendered, &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if payload["auths"]["ghcr.io"]["auth"] != "preset-auth" {
		t.Fatalf("auth = %q, want preset-auth", payload["auths"]["ghcr.io"]["auth"])
	}
}

func TestDockerConfigJSONErrors(t *testing.T) {
	t.Parallel()

	tests := []pullsecretsv1alpha1.RegistryCredentials{
		{Username: "octocat", Password: "s3cret"},
		{Server: "ghcr.io", Password: "s3cret"},
		{Server: "ghcr.io", Username: "octocat"},
	}

	for _, credentials := range tests {
		credentials := credentials
		t.Run(credentials.Server+credentials.Username+credentials.Password, func(t *testing.T) {
			t.Parallel()

			if _, err := DockerConfigJSON(credentials); err == nil {
				t.Fatalf("DockerConfigJSON(%#v) error = nil, want error", credentials)
			}
		})
	}
}

func TestDockerConfigJSONMarshalError(t *testing.T) {
	previous := dockerConfigJSONMarshal
	dockerConfigJSONMarshal = func(any) ([]byte, error) {
		return nil, fmt.Errorf("marshal failed")
	}
	t.Cleanup(func() {
		dockerConfigJSONMarshal = previous
	})

	_, err := DockerConfigJSON(pullsecretsv1alpha1.RegistryCredentials{
		Server:   "ghcr.io",
		Username: "octocat",
		Password: "s3cret",
	})
	if err == nil {
		t.Fatal("DockerConfigJSON() error = nil, want marshal error")
	}
}

func TestBuildPullSecret(t *testing.T) {
	t.Parallel()

	secret := BuildPullSecret(
		pullsecretsv1alpha1.RegistryPullSecret{
			ObjectMeta: metav1.ObjectMeta{Name: "ghcr"},
		},
		pullsecretsv1alpha1.RegistryCredentials{
			Server: "ghcr.io",
		},
		NamespacePlan{Namespace: "team-a", SecretName: "ghcr-pull-secret"},
		[]byte(`{"auths":{"ghcr.io":{"auth":"abc"}}}`),
	)

	if secret.Name != "ghcr-pull-secret" || secret.Namespace != "team-a" {
		t.Fatalf("secret metadata = %s/%s, want team-a/ghcr-pull-secret", secret.Namespace, secret.Name)
	}
	if secret.Type != corev1.SecretTypeDockerConfigJson {
		t.Fatalf("secret type = %q, want %q", secret.Type, corev1.SecretTypeDockerConfigJson)
	}
	if got := secret.Labels[metadata.RegistryPullSecretNameLabelKey]; got != "ghcr" {
		t.Fatalf("registry source label = %q, want %q", got, "ghcr")
	}
}

func TestSecretNeedsApply(t *testing.T) {
	t.Parallel()

	desired := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ghcr-pull-secret",
			Namespace: "team-a",
			Labels: map[string]string{
				metadata.ManagedByLabelKey:              metadata.ManagedByLabelValue,
				metadata.RegistryPullSecretNameLabelKey: "ghcr",
			},
		},
		Type: corev1.SecretTypeDockerConfigJson,
		Data: map[string][]byte{
			dockerConfigJSONKey: []byte(`{}`),
		},
	}

	if !SecretNeedsApply(nil, desired) {
		t.Fatalf("SecretNeedsApply(nil, desired) = false, want true")
	}

	existingSame := desired.DeepCopy()
	if SecretNeedsApply(existingSame, desired) {
		t.Fatalf("SecretNeedsApply(existingSame, desired) = true, want false")
	}

	existingDifferent := desired.DeepCopy()
	existingDifferent.Data[dockerConfigJSONKey] = []byte(`{"changed":true}`)
	if !SecretNeedsApply(existingDifferent, desired) {
		t.Fatalf("SecretNeedsApply(existingDifferent, desired) = false, want true")
	}

	existingDifferentType := desired.DeepCopy()
	existingDifferentType.Type = corev1.SecretTypeOpaque
	if !SecretNeedsApply(existingDifferentType, desired) {
		t.Fatalf("SecretNeedsApply(existingDifferentType, desired) = false, want true")
	}

	existingDifferentLabels := desired.DeepCopy()
	existingDifferentLabels.Labels["extra"] = "value"
	if !SecretNeedsApply(existingDifferentLabels, desired) {
		t.Fatalf("SecretNeedsApply(existingDifferentLabels, desired) = false, want true")
	}
}

func TestDesiredSecrets(t *testing.T) {
	t.Parallel()

	registryPullSecret := pullsecretsv1alpha1.RegistryPullSecret{
		ObjectMeta: metav1.ObjectMeta{Name: "ghcr"},
		Spec: pullsecretsv1alpha1.RegistryPullSecretSpec{
			Credentials: &pullsecretsv1alpha1.RegistryCredentials{
				Server:   "ghcr.io",
				Username: "octocat",
				Password: "s3cret",
			},
			Namespaces: pullsecretsv1alpha1.NamespaceSelection{
				Policy:     pullsecretsv1alpha1.NamespaceSelectionPolicyInclusive,
				Namespaces: []string{"team-a", "team-b"},
				NamespaceOverrides: []pullsecretsv1alpha1.NamespaceTargetOverride{
					{Namespace: "team-b", SecretName: "team-b-ghcr"},
				},
			},
		},
	}

	existing := map[string]*corev1.Secret{
		"team-a/ghcr-pull-secret": {
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
				dockerConfigJSONKey: mustDockerConfigJSON(t, pullsecretsv1alpha1.RegistryCredentials{
					Server:   "ghcr.io",
					Username: "octocat",
					Password: "s3cret",
				}),
			},
		},
	}

	got, err := DesiredSecrets(
		registryPullSecret,
		*registryPullSecret.Spec.Credentials,
		pullsecretsv1alpha1.PullSecretPolicy{},
		[]string{"team-a", "team-b", "team-c"},
		existing,
	)
	if err != nil {
		t.Fatalf("DesiredSecrets() error = %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("len(DesiredSecrets()) = %d, want 2", len(got))
	}

	if got[0].Secret.Namespace != "team-a" || got[0].Secret.Name != "ghcr-pull-secret" || got[0].NeedsApply {
		t.Fatalf("first desired secret = %#v, want unchanged team-a target", got[0])
	}

	if got[1].Secret.Namespace != "team-b" || got[1].Secret.Name != "team-b-ghcr" || !got[1].NeedsApply {
		t.Fatalf("second desired secret = %#v, want create/update for team-b override", got[1])
	}
}

func TestObsoleteSecrets(t *testing.T) {
	t.Parallel()

	registryPullSecret := pullsecretsv1alpha1.RegistryPullSecret{
		ObjectMeta: metav1.ObjectMeta{Name: "ghcr"},
	}

	desiredSecrets := []DesiredSecret{
		{
			Secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "ghcr-pull-secret",
					Namespace: "team-a",
				},
			},
		},
	}

	existingSecrets := map[string]*corev1.Secret{
		"team-a/ghcr-pull-secret": {
			ObjectMeta: metav1.ObjectMeta{
				Name:      "ghcr-pull-secret",
				Namespace: "team-a",
				Labels: map[string]string{
					metadata.ManagedByLabelKey:              metadata.ManagedByLabelValue,
					metadata.RegistryPullSecretNameLabelKey: "ghcr",
				},
			},
		},
		"team-b/old-ghcr-secret": {
			ObjectMeta: metav1.ObjectMeta{
				Name:      "old-ghcr-secret",
				Namespace: "team-b",
				Labels: map[string]string{
					metadata.ManagedByLabelKey:              metadata.ManagedByLabelValue,
					metadata.RegistryPullSecretNameLabelKey: "ghcr",
				},
			},
		},
		"team-c/foreign-secret": {
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foreign-secret",
				Namespace: "team-c",
				Labels: map[string]string{
					metadata.ManagedByLabelKey:              metadata.ManagedByLabelValue,
					metadata.RegistryPullSecretNameLabelKey: "dockerhub",
				},
			},
		},
		"team-d/unmanaged-secret": {
			ObjectMeta: metav1.ObjectMeta{
				Name:      "unmanaged-secret",
				Namespace: "team-d",
			},
		},
	}

	got := ObsoleteSecrets(registryPullSecret, existingSecrets, desiredSecrets)
	if len(got) != 1 {
		t.Fatalf("len(ObsoleteSecrets()) = %d, want 1", len(got))
	}
	if got[0].Namespace != "team-b" || got[0].Name != "old-ghcr-secret" {
		t.Fatalf("obsolete Secret = %s/%s, want team-b/old-ghcr-secret", got[0].Namespace, got[0].Name)
	}
}

func TestDesiredSecretsReturnsValidationError(t *testing.T) {
	t.Parallel()

	_, err := DesiredSecrets(
		pullsecretsv1alpha1.RegistryPullSecret{
			ObjectMeta: metav1.ObjectMeta{Name: "ghcr"},
			Spec: pullsecretsv1alpha1.RegistryPullSecretSpec{
				Credentials: &pullsecretsv1alpha1.RegistryCredentials{
					Server:   "ghcr.io",
					Username: "octocat",
					Password: "s3cret",
				},
				Namespaces: pullsecretsv1alpha1.NamespaceSelection{
					Policy:     pullsecretsv1alpha1.NamespaceSelectionPolicyInclusive,
					Namespaces: []string{"team-a", "team-a"},
				},
			},
		},
		pullsecretsv1alpha1.RegistryCredentials{Server: "ghcr.io", Username: "octocat", Password: "s3cret"},
		pullsecretsv1alpha1.PullSecretPolicy{},
		[]string{"team-a"},
		nil,
	)
	if err == nil {
		t.Fatal("DesiredSecrets() error = nil, want validation error")
	}
}

func TestDesiredSecretsReturnsDockerConfigError(t *testing.T) {
	t.Parallel()

	_, err := DesiredSecrets(
		pullsecretsv1alpha1.RegistryPullSecret{
			ObjectMeta: metav1.ObjectMeta{Name: "ghcr"},
			Spec: pullsecretsv1alpha1.RegistryPullSecretSpec{
				Credentials: &pullsecretsv1alpha1.RegistryCredentials{
					Server:   "ghcr.io",
					Username: "octocat",
				},
				Namespaces: pullsecretsv1alpha1.NamespaceSelection{
					Policy:     pullsecretsv1alpha1.NamespaceSelectionPolicyInclusive,
					Namespaces: []string{"team-a"},
				},
			},
		},
		pullsecretsv1alpha1.RegistryCredentials{Server: "ghcr.io", Username: "octocat"},
		pullsecretsv1alpha1.PullSecretPolicy{},
		[]string{"team-a"},
		nil,
	)
	if err == nil {
		t.Fatal("DesiredSecrets() error = nil, want docker config error")
	}
}

func TestDesiredSecretsReturnsEffectiveTargetsErrorBeforeExplicitOverride(t *testing.T) {
	previous := desiredSecretTargets
	desiredSecretTargets = func(
		registryPullSecret pullsecretsv1alpha1.RegistryPullSecret,
		credentials pullsecretsv1alpha1.RegistryCredentials,
		policy pullsecretsv1alpha1.PullSecretPolicy,
		allNamespaces []string,
	) ([]NamespacePlan, error) {
		return nil, fmt.Errorf("target resolution failed")
	}
	t.Cleanup(func() {
		desiredSecretTargets = previous
	})

	_, err := DesiredSecrets(
		pullsecretsv1alpha1.RegistryPullSecret{
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
					TargetSecretName: "valid-target",
				},
			},
		},
		pullsecretsv1alpha1.RegistryCredentials{Server: "ghcr.io", Username: "octocat", Password: "s3cret"},
		pullsecretsv1alpha1.PullSecretPolicy{},
		[]string{"team-a"},
		nil,
	)
	if err == nil {
		t.Fatal("DesiredSecrets() error = nil, want EffectiveTargets error")
	}
}

func TestSecretNeedsApplyDetectsSameLengthDataDifference(t *testing.T) {
	t.Parallel()

	desired := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ghcr-pull-secret",
			Namespace: "team-a",
		},
		Type: corev1.SecretTypeDockerConfigJson,
		Data: map[string][]byte{
			dockerConfigJSONKey: []byte("abcd"),
		},
	}
	existing := desired.DeepCopy()
	existing.Data[dockerConfigJSONKey] = []byte("abce")

	if !SecretNeedsApply(existing, desired) {
		t.Fatal("SecretNeedsApply() = false, want true for same-length byte diff")
	}
}

func mustDockerConfigJSON(t *testing.T, credentials pullsecretsv1alpha1.RegistryCredentials) []byte {
	t.Helper()

	rendered, err := DockerConfigJSON(credentials)
	if err != nil {
		t.Fatalf("DockerConfigJSON() error = %v", err)
	}

	return rendered
}
