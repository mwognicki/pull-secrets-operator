package sync

import (
	"reflect"
	"testing"

	pullsecretsv1alpha1 "github.com/mwognicki/pull-secrets-operator/api/pullsecrets/v1alpha1"
)

func TestNamespaceSelected(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                      string
		selection                 pullsecretsv1alpha1.NamespaceSelection
		globallyExcludedNamespace []string
		namespace                 string
		want                      bool
	}{
		{
			name: "inclusive policy selects listed namespace",
			selection: pullsecretsv1alpha1.NamespaceSelection{
				Policy:     pullsecretsv1alpha1.NamespaceSelectionPolicyInclusive,
				Namespaces: []string{"team-a", "team-b"},
			},
			namespace: "team-a",
			want:      true,
		},
		{
			name: "inclusive policy skips unlisted namespace",
			selection: pullsecretsv1alpha1.NamespaceSelection{
				Policy:     pullsecretsv1alpha1.NamespaceSelectionPolicyInclusive,
				Namespaces: []string{"team-a", "team-b"},
			},
			namespace: "team-c",
			want:      false,
		},
		{
			name: "exclusive policy skips listed namespace",
			selection: pullsecretsv1alpha1.NamespaceSelection{
				Policy:     pullsecretsv1alpha1.NamespaceSelectionPolicyExclusive,
				Namespaces: []string{"team-a", "team-b"},
			},
			namespace: "team-a",
			want:      false,
		},
		{
			name: "exclusive policy selects unlisted namespace",
			selection: pullsecretsv1alpha1.NamespaceSelection{
				Policy:     pullsecretsv1alpha1.NamespaceSelectionPolicyExclusive,
				Namespaces: []string{"team-a", "team-b"},
			},
			namespace: "team-c",
			want:      true,
		},
		{
			name: "cluster-wide exclusion wins over inclusive selection",
			selection: pullsecretsv1alpha1.NamespaceSelection{
				Policy:     pullsecretsv1alpha1.NamespaceSelectionPolicyInclusive,
				Namespaces: []string{"team-a"},
			},
			globallyExcludedNamespace: []string{"team-a"},
			namespace:                 "team-a",
			want:                      false,
		},
		{
			name: "unknown policy fails closed",
			selection: pullsecretsv1alpha1.NamespaceSelection{
				Policy: pullsecretsv1alpha1.NamespaceSelectionPolicy("Unknown"),
			},
			namespace: "team-a",
			want:      false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := NamespaceSelected(tt.selection, tt.globallyExcludedNamespace, tt.namespace)
			if got != tt.want {
				t.Fatalf("NamespaceSelected() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDefaultTargetSecretName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		server string
		want   string
	}{
		{
			name:   "simple ghcr host",
			server: "ghcr.io",
			want:   "ghcr-pull-secret",
		},
		{
			name:   "host with subdomain",
			server: "docker.toturi.cloud",
			want:   "toturi-pull-secret",
		},
		{
			name:   "oracle cloud host prefers stable vendor token",
			server: "ocir.us-ashburn-1.oci.oraclecloud.com",
			want:   "oraclecloud-pull-secret",
		},
		{
			name:   "scheme and port are ignored",
			server: "https://registry.example.com:5000",
			want:   "example-pull-secret",
		},
		{
			name:   "multi label tld is stripped",
			server: "registry.widgets.co.uk",
			want:   "widgets-pull-secret",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := DefaultTargetSecretName(tt.server)
			if err != nil {
				t.Fatalf("DefaultTargetSecretName() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("DefaultTargetSecretName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestEffectiveTargets(t *testing.T) {
	t.Parallel()

	registryPullSecret := pullsecretsv1alpha1.RegistryPullSecret{
		Spec: pullsecretsv1alpha1.RegistryPullSecretSpec{
			Credentials: &pullsecretsv1alpha1.RegistryCredentials{
				Server: "ghcr.io",
			},
			Namespaces: pullsecretsv1alpha1.NamespaceSelection{
				Policy:     pullsecretsv1alpha1.NamespaceSelectionPolicyExclusive,
				Namespaces: []string{"team-b"},
				NamespaceOverrides: []pullsecretsv1alpha1.NamespaceTargetOverride{
					{
						Namespace:  "team-c",
						SecretName: "custom-team-c",
					},
				},
			},
		},
	}

	policy := pullsecretsv1alpha1.PullSecretPolicy{
		Spec: pullsecretsv1alpha1.PullSecretPolicySpec{
			ExcludedNamespaces: []string{"team-d"},
		},
	}

	got, err := EffectiveTargets(registryPullSecret, *registryPullSecret.Spec.Credentials, policy, []string{"team-a", "team-b", "team-c", "team-d"})
	if err != nil {
		t.Fatalf("EffectiveTargets() error = %v", err)
	}

	want := []NamespacePlan{
		{Namespace: "team-a", SecretName: "ghcr-pull-secret"},
		{Namespace: "team-c", SecretName: "custom-team-c"},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("EffectiveTargets() = %#v, want %#v", got, want)
	}
}

func TestEffectiveTargetsUsesExplicitDefaultSecretName(t *testing.T) {
	t.Parallel()

	registryPullSecret := pullsecretsv1alpha1.RegistryPullSecret{
		Spec: pullsecretsv1alpha1.RegistryPullSecretSpec{
			Credentials: &pullsecretsv1alpha1.RegistryCredentials{
				Server: "docker.toturi.cloud",
			},
			Namespaces: pullsecretsv1alpha1.NamespaceSelection{
				Policy:           pullsecretsv1alpha1.NamespaceSelectionPolicyInclusive,
				Namespaces:       []string{"team-a"},
				TargetSecretName: "shared-secret",
			},
		},
	}

	got, err := EffectiveTargets(registryPullSecret, *registryPullSecret.Spec.Credentials, pullsecretsv1alpha1.PullSecretPolicy{}, []string{"team-a"})
	if err != nil {
		t.Fatalf("EffectiveTargets() error = %v", err)
	}

	want := []NamespacePlan{
		{Namespace: "team-a", SecretName: "shared-secret"},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("EffectiveTargets() = %#v, want %#v", got, want)
	}
}
