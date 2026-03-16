package sync

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	pullsecretsv1alpha1 "github.com/mwognicki/pull-secrets-operator/api/pullsecrets/v1alpha1"
	"github.com/mwognicki/pull-secrets-operator/pkg/metadata"
)

func TestValidateRegistryPullSecretRejectsDuplicateNamespaces(t *testing.T) {
	t.Parallel()

	err := ValidateRegistryPullSecret(
		pullsecretsv1alpha1.RegistryPullSecret{
			Spec: pullsecretsv1alpha1.RegistryPullSecretSpec{
				Namespaces: pullsecretsv1alpha1.NamespaceSelection{
					Policy:     pullsecretsv1alpha1.NamespaceSelectionPolicyInclusive,
					Namespaces: []string{"team-a", "team-a"},
				},
			},
		},
		pullsecretsv1alpha1.RegistryCredentials{Server: "ghcr.io"},
		pullsecretsv1alpha1.PullSecretPolicy{},
		nil,
	)
	if err == nil || !strings.Contains(err.Error(), "duplicated in namespaces") {
		t.Fatalf("ValidateRegistryPullSecret() error = %v, want duplicate namespace error", err)
	}
}

func TestValidateRegistryPullSecretAcceptsValidConfiguration(t *testing.T) {
	t.Parallel()

	err := ValidateRegistryPullSecret(
		pullsecretsv1alpha1.RegistryPullSecret{
			ObjectMeta: metav1.ObjectMeta{Name: "ghcr"},
			Spec: pullsecretsv1alpha1.RegistryPullSecretSpec{
				Namespaces: pullsecretsv1alpha1.NamespaceSelection{
					Policy:     pullsecretsv1alpha1.NamespaceSelectionPolicyInclusive,
					Namespaces: []string{"team-a"},
					NamespaceOverrides: []pullsecretsv1alpha1.NamespaceTargetOverride{
						{Namespace: "team-b", SecretName: "team-b-ghcr"},
					},
				},
			},
		},
		pullsecretsv1alpha1.RegistryCredentials{Server: "ghcr.io"},
		pullsecretsv1alpha1.PullSecretPolicy{},
		map[string]*corev1.Secret{
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
		},
	)
	if err != nil {
		t.Fatalf("ValidateRegistryPullSecret() error = %v", err)
	}
}

func TestValidateRegistryPullSecretRejectsUnknownPolicy(t *testing.T) {
	t.Parallel()

	err := ValidateRegistryPullSecret(
		pullsecretsv1alpha1.RegistryPullSecret{
			Spec: pullsecretsv1alpha1.RegistryPullSecretSpec{
				Namespaces: pullsecretsv1alpha1.NamespaceSelection{
					Policy: pullsecretsv1alpha1.NamespaceSelectionPolicy("Broken"),
				},
			},
		},
		pullsecretsv1alpha1.RegistryCredentials{Server: "ghcr.io"},
		pullsecretsv1alpha1.PullSecretPolicy{},
		nil,
	)
	if err == nil || !strings.Contains(err.Error(), "policy") {
		t.Fatalf("ValidateRegistryPullSecret() error = %v, want invalid policy error", err)
	}
}

func TestValidateRegistryPullSecretRejectsDuplicateNamespaceOverrides(t *testing.T) {
	t.Parallel()

	err := ValidateRegistryPullSecret(
		pullsecretsv1alpha1.RegistryPullSecret{
			Spec: pullsecretsv1alpha1.RegistryPullSecretSpec{
				Namespaces: pullsecretsv1alpha1.NamespaceSelection{
					Policy: pullsecretsv1alpha1.NamespaceSelectionPolicyInclusive,
					NamespaceOverrides: []pullsecretsv1alpha1.NamespaceTargetOverride{
						{Namespace: "team-a", SecretName: "team-a-ghcr"},
						{Namespace: "team-a", SecretName: "team-a-ghcr-alt"},
					},
				},
			},
		},
		pullsecretsv1alpha1.RegistryCredentials{Server: "ghcr.io"},
		pullsecretsv1alpha1.PullSecretPolicy{},
		nil,
	)
	if err == nil || !strings.Contains(err.Error(), "namespace override \"team-a\" is duplicated") {
		t.Fatalf("ValidateRegistryPullSecret() error = %v, want duplicate override error", err)
	}
}

func TestValidateRegistryPullSecretRejectsWildcardNamespace(t *testing.T) {
	t.Parallel()

	err := ValidateRegistryPullSecret(
		pullsecretsv1alpha1.RegistryPullSecret{
			Spec: pullsecretsv1alpha1.RegistryPullSecretSpec{
				Namespaces: pullsecretsv1alpha1.NamespaceSelection{
					Policy:     pullsecretsv1alpha1.NamespaceSelectionPolicyInclusive,
					Namespaces: []string{"team-*"},
				},
			},
		},
		pullsecretsv1alpha1.RegistryCredentials{Server: "ghcr.io"},
		pullsecretsv1alpha1.PullSecretPolicy{},
		nil,
	)
	if err == nil || !strings.Contains(err.Error(), "wildcard namespace patterns are not supported") {
		t.Fatalf("ValidateRegistryPullSecret() error = %v, want wildcard error", err)
	}
}

func TestValidateRegistryPullSecretRejectsInvalidNamespaceName(t *testing.T) {
	t.Parallel()

	err := ValidateRegistryPullSecret(
		pullsecretsv1alpha1.RegistryPullSecret{
			Spec: pullsecretsv1alpha1.RegistryPullSecretSpec{
				Namespaces: pullsecretsv1alpha1.NamespaceSelection{
					Policy:     pullsecretsv1alpha1.NamespaceSelectionPolicyInclusive,
					Namespaces: []string{"The quick brown fox!!! :DDDDDD"},
				},
			},
		},
		pullsecretsv1alpha1.RegistryCredentials{Server: "ghcr.io"},
		pullsecretsv1alpha1.PullSecretPolicy{},
		nil,
	)
	if err == nil || !strings.Contains(err.Error(), "is invalid") {
		t.Fatalf("ValidateRegistryPullSecret() error = %v, want invalid namespace error", err)
	}
}

func TestValidateRegistryPullSecretRejectsTooShortTargetSecretName(t *testing.T) {
	t.Parallel()

	err := ValidateRegistryPullSecret(
		pullsecretsv1alpha1.RegistryPullSecret{
			Spec: pullsecretsv1alpha1.RegistryPullSecretSpec{
				Namespaces: pullsecretsv1alpha1.NamespaceSelection{
					Policy:           pullsecretsv1alpha1.NamespaceSelectionPolicyInclusive,
					Namespaces:       []string{"team-a"},
					TargetSecretName: "a-b",
				},
			},
		},
		pullsecretsv1alpha1.RegistryCredentials{Server: "ghcr.io"},
		pullsecretsv1alpha1.PullSecretPolicy{},
		nil,
	)
	if err == nil || !strings.Contains(err.Error(), "at least 3 alphanumeric characters") {
		t.Fatalf("ValidateRegistryPullSecret() error = %v, want short secret name error", err)
	}
}

func TestValidateRegistryPullSecretRejectsInvalidOverrideSecretName(t *testing.T) {
	t.Parallel()

	err := ValidateRegistryPullSecret(
		pullsecretsv1alpha1.RegistryPullSecret{
			Spec: pullsecretsv1alpha1.RegistryPullSecretSpec{
				Namespaces: pullsecretsv1alpha1.NamespaceSelection{
					Policy: pullsecretsv1alpha1.NamespaceSelectionPolicyExclusive,
					NamespaceOverrides: []pullsecretsv1alpha1.NamespaceTargetOverride{
						{Namespace: "team-a", SecretName: "BAD NAME"},
					},
				},
			},
		},
		pullsecretsv1alpha1.RegistryCredentials{Server: "ghcr.io"},
		pullsecretsv1alpha1.PullSecretPolicy{},
		nil,
	)
	if err == nil || !strings.Contains(err.Error(), "namespace override secretName") {
		t.Fatalf("ValidateRegistryPullSecret() error = %v, want invalid override secret name error", err)
	}
}

func TestValidateRegistryPullSecretRejectsInvalidDerivedTargetSecretName(t *testing.T) {
	t.Parallel()

	err := ValidateRegistryPullSecret(
		pullsecretsv1alpha1.RegistryPullSecret{
			Spec: pullsecretsv1alpha1.RegistryPullSecretSpec{
				Namespaces: pullsecretsv1alpha1.NamespaceSelection{
					Policy: pullsecretsv1alpha1.NamespaceSelectionPolicyExclusive,
				},
			},
		},
		pullsecretsv1alpha1.RegistryCredentials{Server: "%%"},
		pullsecretsv1alpha1.PullSecretPolicy{},
		nil,
	)
	if err == nil {
		t.Fatal("ValidateRegistryPullSecret() error = nil, want derived target name error")
	}
}

func TestValidateRegistryPullSecretReturnsEffectiveTargetsError(t *testing.T) {
	t.Parallel()

	err := ValidateRegistryPullSecret(
		pullsecretsv1alpha1.RegistryPullSecret{
			ObjectMeta: metav1.ObjectMeta{Name: "ghcr"},
			Spec: pullsecretsv1alpha1.RegistryPullSecretSpec{
				Namespaces: pullsecretsv1alpha1.NamespaceSelection{
					Policy:           pullsecretsv1alpha1.NamespaceSelectionPolicyInclusive,
					Namespaces:       []string{"team-a"},
					TargetSecretName: "valid-target",
				},
			},
		},
		pullsecretsv1alpha1.RegistryCredentials{Server: "___"},
		pullsecretsv1alpha1.PullSecretPolicy{},
		nil,
	)
	if err == nil {
		t.Fatal("ValidateRegistryPullSecret() error = nil, want EffectiveTargets error")
	}
}

func TestValidateRegistryPullSecretRejectsExcludedInclusiveNamespace(t *testing.T) {
	t.Parallel()

	err := ValidateRegistryPullSecret(
		pullsecretsv1alpha1.RegistryPullSecret{
			Spec: pullsecretsv1alpha1.RegistryPullSecretSpec{
				Namespaces: pullsecretsv1alpha1.NamespaceSelection{
					Policy:     pullsecretsv1alpha1.NamespaceSelectionPolicyInclusive,
					Namespaces: []string{"team-a"},
				},
			},
		},
		pullsecretsv1alpha1.RegistryCredentials{Server: "ghcr.io"},
		pullsecretsv1alpha1.PullSecretPolicy{
			Spec: pullsecretsv1alpha1.PullSecretPolicySpec{ExcludedNamespaces: []string{"team-a"}},
		},
		nil,
	)
	if err == nil || !strings.Contains(err.Error(), "explicitly selected but excluded") {
		t.Fatalf("ValidateRegistryPullSecret() error = %v, want excluded namespace error", err)
	}
}

func TestValidateRegistryPullSecretRejectsExcludedOverrideNamespace(t *testing.T) {
	t.Parallel()

	err := ValidateRegistryPullSecret(
		pullsecretsv1alpha1.RegistryPullSecret{
			Spec: pullsecretsv1alpha1.RegistryPullSecretSpec{
				Namespaces: pullsecretsv1alpha1.NamespaceSelection{
					Policy: pullsecretsv1alpha1.NamespaceSelectionPolicyExclusive,
					NamespaceOverrides: []pullsecretsv1alpha1.NamespaceTargetOverride{
						{Namespace: "team-a", SecretName: "team-a-ghcr"},
					},
				},
			},
		},
		pullsecretsv1alpha1.RegistryCredentials{Server: "ghcr.io"},
		pullsecretsv1alpha1.PullSecretPolicy{
			Spec: pullsecretsv1alpha1.PullSecretPolicySpec{ExcludedNamespaces: []string{"team-a"}},
		},
		nil,
	)
	if err == nil || !strings.Contains(err.Error(), "namespace override \"team-a\" is excluded") {
		t.Fatalf("ValidateRegistryPullSecret() error = %v, want excluded override error", err)
	}
}

func TestValidateRegistryPullSecretRejectsUnmanagedSecretCollision(t *testing.T) {
	t.Parallel()

	err := ValidateRegistryPullSecret(
		pullsecretsv1alpha1.RegistryPullSecret{
			ObjectMeta: metav1.ObjectMeta{Name: "ghcr"},
			Spec: pullsecretsv1alpha1.RegistryPullSecretSpec{
				Namespaces: pullsecretsv1alpha1.NamespaceSelection{
					Policy:     pullsecretsv1alpha1.NamespaceSelectionPolicyInclusive,
					Namespaces: []string{"team-a"},
				},
			},
		},
		pullsecretsv1alpha1.RegistryCredentials{Server: "ghcr.io"},
		pullsecretsv1alpha1.PullSecretPolicy{},
		map[string]*corev1.Secret{
			"team-a/ghcr-pull-secret": {
				ObjectMeta: metav1.ObjectMeta{Name: "ghcr-pull-secret", Namespace: "team-a"},
			},
		},
	)
	if err == nil || !strings.Contains(err.Error(), "is not managed by this operator") {
		t.Fatalf("ValidateRegistryPullSecret() error = %v, want collision error", err)
	}
}

func TestValidateRegistryPullSecretRejectsForeignManagedSecretCollision(t *testing.T) {
	t.Parallel()

	err := ValidateRegistryPullSecret(
		pullsecretsv1alpha1.RegistryPullSecret{
			ObjectMeta: metav1.ObjectMeta{Name: "ghcr"},
			Spec: pullsecretsv1alpha1.RegistryPullSecretSpec{
				Namespaces: pullsecretsv1alpha1.NamespaceSelection{
					Policy:     pullsecretsv1alpha1.NamespaceSelectionPolicyInclusive,
					Namespaces: []string{"team-a"},
				},
			},
		},
		pullsecretsv1alpha1.RegistryCredentials{Server: "ghcr.io"},
		pullsecretsv1alpha1.PullSecretPolicy{},
		map[string]*corev1.Secret{
			"team-a/ghcr-pull-secret": {
				ObjectMeta: metav1.ObjectMeta{
					Name:      "ghcr-pull-secret",
					Namespace: "team-a",
					Labels: map[string]string{
						metadata.ManagedByLabelKey:              metadata.ManagedByLabelValue,
						metadata.RegistryPullSecretNameLabelKey: "dockerhub",
					},
				},
			},
		},
	)
	if err == nil || !strings.Contains(err.Error(), "already managed by RegistryPullSecret") {
		t.Fatalf("ValidateRegistryPullSecret() error = %v, want foreign managed collision error", err)
	}
}

func TestValidateDefaultTargetSecretNameRejectsExplicitInvalidName(t *testing.T) {
	t.Parallel()

	err := validateDefaultTargetSecretName(
		pullsecretsv1alpha1.RegistryPullSecret{
			Spec: pullsecretsv1alpha1.RegistryPullSecretSpec{
				Namespaces: pullsecretsv1alpha1.NamespaceSelection{
					TargetSecretName: "a-b",
				},
			},
		},
		pullsecretsv1alpha1.RegistryCredentials{Server: "ghcr.io"},
	)
	if err == nil {
		t.Fatal("validateDefaultTargetSecretName() error = nil, want invalid explicit name error")
	}
}

func TestPolicyAwareNamespaceInventory(t *testing.T) {
	t.Parallel()

	got := policyAwareNamespaceInventory(
		pullsecretsv1alpha1.RegistryPullSecret{
			Spec: pullsecretsv1alpha1.RegistryPullSecretSpec{
				Namespaces: pullsecretsv1alpha1.NamespaceSelection{
					Namespaces: []string{"team-a", "team-b", "team-a"},
					NamespaceOverrides: []pullsecretsv1alpha1.NamespaceTargetOverride{
						{Namespace: "team-c"},
						{Namespace: "team-b"},
					},
				},
			},
		},
		pullsecretsv1alpha1.PullSecretPolicy{
			Spec: pullsecretsv1alpha1.PullSecretPolicySpec{
				ExcludedNamespaces: []string{"team-d", "team-a"},
			},
		},
	)

	want := []string{"team-a", "team-b", "team-c", "team-d"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("policyAwareNamespaceInventory() = %#v, want %#v", got, want)
	}
}

func TestValidateNamespaceName(t *testing.T) {
	t.Parallel()

	if err := validateNamespaceName("team-a"); err != nil {
		t.Fatalf("validateNamespaceName() error = %v", err)
	}
	if err := validateNamespaceName("bad*name"); err == nil {
		t.Fatal("validateNamespaceName() error = nil, want wildcard error")
	}
	if err := validateNamespaceName("Bad Name"); err == nil {
		t.Fatal("validateNamespaceName() error = nil, want invalid name error")
	}
}

func TestValidatePullSecretName(t *testing.T) {
	t.Parallel()

	if err := validatePullSecretName("good-name-123"); err != nil {
		t.Fatalf("validatePullSecretName() error = %v", err)
	}
	if err := validatePullSecretName("NOPE"); err == nil {
		t.Fatal("validatePullSecretName() error = nil, want syntax error")
	}
	if err := validatePullSecretName("a-b"); err == nil {
		t.Fatal("validatePullSecretName() error = nil, want short-name error")
	}
}

func TestValidateGloballyExcludedSelection(t *testing.T) {
	t.Parallel()

	if err := validateGloballyExcludedSelection(
		pullsecretsv1alpha1.NamespaceSelection{
			Policy:     pullsecretsv1alpha1.NamespaceSelectionPolicyExclusive,
			Namespaces: []string{"team-a"},
		},
		[]string{"team-a"},
	); err != nil {
		t.Fatalf("validateGloballyExcludedSelection() exclusive error = %v", err)
	}

	if err := validateGloballyExcludedSelection(
		pullsecretsv1alpha1.NamespaceSelection{
			Policy: pullsecretsv1alpha1.NamespaceSelectionPolicyInclusive,
			NamespaceOverrides: []pullsecretsv1alpha1.NamespaceTargetOverride{
				{Namespace: "team-a", SecretName: "team-a-ghcr"},
			},
		},
		nil,
	); err != nil {
		t.Fatalf("validateGloballyExcludedSelection() override error = %v", err)
	}
}

func TestValidateNamespaceSelectionRejectsInvalidOverrideNamespace(t *testing.T) {
	t.Parallel()

	err := validateNamespaceSelection(pullsecretsv1alpha1.NamespaceSelection{
		Policy: pullsecretsv1alpha1.NamespaceSelectionPolicyExclusive,
		NamespaceOverrides: []pullsecretsv1alpha1.NamespaceTargetOverride{
			{Namespace: "Bad Name", SecretName: "team-a-ghcr"},
		},
	})
	if err == nil {
		t.Fatal("validateNamespaceSelection() error = nil, want invalid override namespace error")
	}
}

func TestValidateTargetsRejectsInvalidResultingName(t *testing.T) {
	t.Parallel()

	err := validateTargets(
		pullsecretsv1alpha1.RegistryPullSecret{ObjectMeta: metav1.ObjectMeta{Name: "ghcr"}},
		[]NamespacePlan{{Namespace: "team-a", SecretName: "a-b"}},
		nil,
	)
	if err == nil {
		t.Fatal("validateTargets() error = nil, want invalid resulting name error")
	}
}

func TestIsValidationError(t *testing.T) {
	t.Parallel()

	if IsValidationError(nil) {
		t.Fatal("IsValidationError(nil) = true, want false")
	}

	err := ValidateRegistryPullSecret(
		pullsecretsv1alpha1.RegistryPullSecret{
			Spec: pullsecretsv1alpha1.RegistryPullSecretSpec{
				Namespaces: pullsecretsv1alpha1.NamespaceSelection{
					Policy:     pullsecretsv1alpha1.NamespaceSelectionPolicyInclusive,
					Namespaces: []string{"team-a", "team-a"},
				},
			},
		},
		pullsecretsv1alpha1.RegistryCredentials{Server: "ghcr.io"},
		pullsecretsv1alpha1.PullSecretPolicy{},
		nil,
	)
	if !IsValidationError(err) {
		t.Fatalf("IsValidationError(%v) = false, want true", err)
	}
	if IsValidationError(errors.New("plain error")) {
		t.Fatal("IsValidationError(non-validation error) = true, want false")
	}
}

func TestValidatePullSecretPolicy(t *testing.T) {
	t.Parallel()

	if err := ValidatePullSecretPolicy(pullsecretsv1alpha1.PullSecretPolicy{
		Spec: pullsecretsv1alpha1.PullSecretPolicySpec{
			ExcludedNamespaces: []string{"team-a", "team-b"},
		},
	}); err != nil {
		t.Fatalf("ValidatePullSecretPolicy() error = %v", err)
	}

	if err := ValidatePullSecretPolicy(pullsecretsv1alpha1.PullSecretPolicy{
		Spec: pullsecretsv1alpha1.PullSecretPolicySpec{
			ExcludedNamespaces: []string{"team-a", "team-a"},
		},
	}); err == nil || !IsValidationError(err) {
		t.Fatalf("ValidatePullSecretPolicy() duplicate error = %v, want validation error", err)
	}

	if err := ValidatePullSecretPolicy(pullsecretsv1alpha1.PullSecretPolicy{
		Spec: pullsecretsv1alpha1.PullSecretPolicySpec{
			ExcludedNamespaces: []string{"Bad Namespace"},
		},
	}); err == nil || !IsValidationError(err) {
		t.Fatalf("ValidatePullSecretPolicy() invalid namespace error = %v, want validation error", err)
	}
}
