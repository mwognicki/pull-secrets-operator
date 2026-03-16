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
		{
			name:   "gov se suffix is stripped",
			server: "registry.govservice.gov.se",
			want:   "govservice-pull-secret",
		},
		{
			name:   "gov si suffix is stripped",
			server: "registry.ministry.gov.si",
			want:   "ministry-pull-secret",
		},
		{
			name:   "org fi suffix is stripped",
			server: "docker.foundation.org.fi",
			want:   "foundation-pull-secret",
		},
		{
			name:   "com ge suffix is stripped",
			server: "registry.example.com.ge",
			want:   "example-pull-secret",
		},
		{
			name:   "net dk suffix is stripped",
			server: "registry.company.net.dk",
			want:   "company-pull-secret",
		},
		{
			name:   "co in suffix is stripped",
			server: "registry.widgets.co.in",
			want:   "widgets-pull-secret",
		},
		{
			name:   "localhost stays unchanged apart from suffix",
			server: "localhost",
			want:   "localhost-pull-secret",
		},
		{
			name:   "single label host with port stays unchanged apart from suffix",
			server: "some-other-server:2929",
			want:   "some-other-server-pull-secret",
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

func TestDefaultTargetSecretNameErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		server string
	}{
		{name: "empty server", server: ""},
		{name: "invalid url", server: "https://%%%"},
		{name: "missing hostname", server: "https://:5000"},
		{name: "ignored only host parts", server: "registry.io"},
		{name: "single label sanitizes empty", server: "___:5000"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if _, err := DefaultTargetSecretName(tt.server); err == nil {
				t.Fatalf("DefaultTargetSecretName(%q) error = nil, want error", tt.server)
			}
		})
	}
}

func TestDeriveHostLabel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		parts []string
		want  string
	}{
		{name: "single significant token kept", parts: []string{"mirror", "ghcr"}, want: "ghcr"},
		{name: "single filtered token kept", parts: []string{"docker", "private-host"}, want: "private-host"},
		{name: "all ignored tokens removed", parts: []string{"docker", "registry", "www"}, want: ""},
		{name: "multiple tokens choose final significant token", parts: []string{"alpha", "beta", "gamma"}, want: "gamma"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := deriveHostLabel(tt.parts); got != tt.want {
				t.Fatalf("deriveHostLabel() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestStripKnownTLDSuffix(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		parts []string
		want  []string
	}{
		{name: "empty stays empty", parts: nil, want: []string{}},
		{name: "single label becomes empty", parts: []string{"localhost"}, want: []string{}},
		{name: "secondary token removed after tld", parts: []string{"example", "com", "br"}, want: []string{"example"}},
		{name: "regular tld only removed", parts: []string{"example", "cloud"}, want: []string{"example"}},
		{name: "sanitization happens before stripping", parts: []string{"Example", "COM", "BR"}, want: []string{"example"}},
		{name: "empty sanitized parts are skipped", parts: []string{"***", "example", "com"}, want: []string{"example"}},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := stripKnownTLDSuffix(tt.parts)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("stripKnownTLDSuffix() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestSanitizeDNSLabelPart(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"Local_Host":     "localhost",
		"--mixed-123--":  "mixed-123",
		"spaces are bad": "spacesarebad",
	}

	for input, want := range tests {
		input := input
		want := want
		t.Run(input, func(t *testing.T) {
			t.Parallel()

			if got := sanitizeDNSLabelPart(input); got != want {
				t.Fatalf("sanitizeDNSLabelPart() = %q, want %q", got, want)
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

func TestEffectiveTargetsReturnsDefaultNameErrorBeforeExplicitOverride(t *testing.T) {
	t.Parallel()

	_, err := EffectiveTargets(
		pullsecretsv1alpha1.RegistryPullSecret{
			Spec: pullsecretsv1alpha1.RegistryPullSecretSpec{
				Credentials: &pullsecretsv1alpha1.RegistryCredentials{Server: "___"},
				Namespaces: pullsecretsv1alpha1.NamespaceSelection{
					Policy:           pullsecretsv1alpha1.NamespaceSelectionPolicyInclusive,
					Namespaces:       []string{"team-a"},
					TargetSecretName: "shared-secret",
				},
			},
		},
		pullsecretsv1alpha1.RegistryCredentials{Server: "___"},
		pullsecretsv1alpha1.PullSecretPolicy{},
		[]string{"team-a"},
	)
	if err == nil {
		t.Fatal("EffectiveTargets() error = nil, want derivation error")
	}
}
