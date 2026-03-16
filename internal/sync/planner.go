package sync

import (
	"fmt"
	"net/url"
	"slices"
	"strings"

	pullsecretsv1alpha1 "github.com/mwognicki/pull-secrets-operator/api/pullsecrets/v1alpha1"
)

const defaultDerivedSecretSuffix = "-pull-secret"

// NamespacePlan describes the desired pull secret target for one namespace.
type NamespacePlan struct {
	Namespace  string
	SecretName string
}

// EffectiveTargets resolves the per-namespace target secret names for a registry
// resource across a provided namespace inventory.
func EffectiveTargets(
	registryPullSecret pullsecretsv1alpha1.RegistryPullSecret,
	credentials pullsecretsv1alpha1.RegistryCredentials,
	policy pullsecretsv1alpha1.PullSecretPolicy,
	allNamespaces []string,
) ([]NamespacePlan, error) {
	defaultSecretName, err := DefaultTargetSecretName(credentials.Server)
	if err != nil {
		return nil, err
	}

	if registryPullSecret.Spec.Namespaces.TargetSecretName != "" {
		defaultSecretName = registryPullSecret.Spec.Namespaces.TargetSecretName
	}

	overrides := make(map[string]string, len(registryPullSecret.Spec.Namespaces.NamespaceOverrides))
	for _, override := range registryPullSecret.Spec.Namespaces.NamespaceOverrides {
		overrides[override.Namespace] = override.SecretName
	}

	plans := make([]NamespacePlan, 0, len(allNamespaces))
	for _, namespace := range allNamespaces {
		if !NamespaceSelected(registryPullSecret.Spec.Namespaces, policy.Spec.ExcludedNamespaces, namespace) {
			continue
		}

		secretName := defaultSecretName
		if overrideSecretName, ok := overrides[namespace]; ok {
			secretName = overrideSecretName
		}

		plans = append(plans, NamespacePlan{
			Namespace:  namespace,
			SecretName: secretName,
		})
	}

	return plans, nil
}

// NamespaceSelected reports whether a namespace is eligible after applying both
// the per-registry selection rules and the cluster-wide exclusion policy.
func NamespaceSelected(
	selection pullsecretsv1alpha1.NamespaceSelection,
	globallyExcludedNamespaces []string,
	namespace string,
) bool {
	if slices.Contains(globallyExcludedNamespaces, namespace) {
		return false
	}

	selectedByRegistryRule := slices.Contains(selection.Namespaces, namespace)

	switch selection.Policy {
	case pullsecretsv1alpha1.NamespaceSelectionPolicyInclusive:
		return selectedByRegistryRule
	case pullsecretsv1alpha1.NamespaceSelectionPolicyExclusive:
		return !selectedByRegistryRule
	default:
		return false
	}
}

// DefaultTargetSecretName derives a stable human-friendly secret name from a registry server.
func DefaultTargetSecretName(server string) (string, error) {
	host, err := normalizeRegistryHost(server)
	if err != nil {
		return "", err
	}

	parts := stripKnownTLDSuffix(strings.Split(host, "."))
	candidate := deriveHostLabel(parts)
	if candidate == "" {
		return "", fmt.Errorf("derive secret name from registry server %q: empty candidate", server)
	}

	return candidate + defaultDerivedSecretSuffix, nil
}

func normalizeRegistryHost(server string) (string, error) {
	trimmed := strings.TrimSpace(server)
	if trimmed == "" {
		return "", fmt.Errorf("registry server is empty")
	}

	if !strings.Contains(trimmed, "://") {
		trimmed = "https://" + trimmed
	}

	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", fmt.Errorf("parse registry server %q: %w", server, err)
	}

	host := strings.ToLower(parsed.Hostname())
	if host == "" {
		return "", fmt.Errorf("registry server %q does not include a hostname", server)
	}

	return host, nil
}

func deriveHostLabel(parts []string) string {
	filtered := make([]string, 0, len(parts))
	for _, part := range parts {
		part = sanitizeDNSLabelPart(part)
		if part == "" || ignoredHostTokens[part] {
			continue
		}
		filtered = append(filtered, part)
	}

	for _, part := range filtered {
		if preferredHostTokens[part] {
			return part
		}
	}

	switch {
	case len(filtered) == 0:
		return ""
	case len(filtered) == 1:
		return filtered[0]
	default:
		return filtered[len(filtered)-2]
	}
}

func stripKnownTLDSuffix(parts []string) []string {
	sanitized := make([]string, 0, len(parts))
	for _, part := range parts {
		part = sanitizeDNSLabelPart(part)
		if part == "" {
			continue
		}
		sanitized = append(sanitized, part)
	}

	for _, suffix := range knownDomainSuffixes {
		if len(sanitized) < len(suffix) {
			continue
		}
		if slices.Equal(sanitized[len(sanitized)-len(suffix):], suffix) {
			return sanitized[:len(sanitized)-len(suffix)]
		}
	}

	return sanitized
}

var ignoredHostTokens = map[string]bool{
	"docker":   true,
	"registry": true,
	"www":      true,
}

var preferredHostTokens = map[string]bool{
	"ghcr":        true,
	"oraclecloud": true,
}

var knownDomainSuffixes = [][]string{
	{"co", "uk"},
	{"com"},
	{"pl"},
	{"cloud"},
	{"org"},
	{"net"},
	{"eu"},
	{"io"},
	{"dev"},
	{"space"},
}

func sanitizeDNSLabelPart(part string) string {
	var builder strings.Builder
	builder.Grow(len(part))

	for _, r := range strings.ToLower(part) {
		switch {
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
		case r == '-':
			builder.WriteRune(r)
		}
	}

	return strings.Trim(builder.String(), "-")
}
