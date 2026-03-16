package sync

import (
	"fmt"
	"slices"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/validation"

	pullsecretsv1alpha1 "github.com/mwognicki/pull-secrets-operator/api/pullsecrets/v1alpha1"
	"github.com/mwognicki/pull-secrets-operator/pkg/metadata"
)

// ValidateRegistryPullSecret enforces semantic validation rules that go beyond
// basic CRD admission checks.
func ValidateRegistryPullSecret(
	registryPullSecret pullsecretsv1alpha1.RegistryPullSecret,
	credentials pullsecretsv1alpha1.RegistryCredentials,
	policy pullsecretsv1alpha1.PullSecretPolicy,
	existingSecrets map[string]*corev1.Secret,
) error {
	if err := validateNamespaceSelection(registryPullSecret.Spec.Namespaces); err != nil {
		return err
	}
	if err := validateGloballyExcludedSelection(registryPullSecret.Spec.Namespaces, policy.Spec.ExcludedNamespaces); err != nil {
		return err
	}
	if err := validateDefaultTargetSecretName(registryPullSecret, credentials); err != nil {
		return err
	}

	targets, err := EffectiveTargets(registryPullSecret, credentials, policy, policyAwareNamespaceInventory(registryPullSecret, policy))
	if err != nil {
		return err
	}

	if err := validateTargets(registryPullSecret, targets, existingSecrets); err != nil {
		return err
	}

	return nil
}

func validateDefaultTargetSecretName(
	registryPullSecret pullsecretsv1alpha1.RegistryPullSecret,
	credentials pullsecretsv1alpha1.RegistryCredentials,
) error {
	name := registryPullSecret.Spec.Namespaces.TargetSecretName
	if name == "" {
		derived, err := DefaultTargetSecretName(credentials.Server)
		if err != nil {
			return err
		}
		name = derived
	}

	if err := validatePullSecretName(name); err != nil {
		return fmt.Errorf("default target pull secret name %q is invalid: %w", name, err)
	}

	return nil
}

func validateNamespaceSelection(selection pullsecretsv1alpha1.NamespaceSelection) error {
	switch selection.Policy {
	case pullsecretsv1alpha1.NamespaceSelectionPolicyInclusive, pullsecretsv1alpha1.NamespaceSelectionPolicyExclusive:
	default:
		return fmt.Errorf("namespace selection policy %q is invalid", selection.Policy)
	}

	if selection.TargetSecretName != "" {
		if err := validatePullSecretName(selection.TargetSecretName); err != nil {
			return fmt.Errorf("targetSecretName %q is invalid: %w", selection.TargetSecretName, err)
		}
	}

	seenNamespaces := make(map[string]struct{}, len(selection.Namespaces))
	for _, namespace := range selection.Namespaces {
		if err := validateNamespaceName(namespace); err != nil {
			return fmt.Errorf("namespace %q is invalid: %w", namespace, err)
		}
		if _, ok := seenNamespaces[namespace]; ok {
			return fmt.Errorf("namespace %q is duplicated in namespaces", namespace)
		}
		seenNamespaces[namespace] = struct{}{}
	}

	seenOverrideNamespaces := make(map[string]struct{}, len(selection.NamespaceOverrides))
	for _, override := range selection.NamespaceOverrides {
		if err := validateNamespaceName(override.Namespace); err != nil {
			return fmt.Errorf("namespace override %q is invalid: %w", override.Namespace, err)
		}
		if _, ok := seenOverrideNamespaces[override.Namespace]; ok {
			return fmt.Errorf("namespace override %q is duplicated", override.Namespace)
		}
		seenOverrideNamespaces[override.Namespace] = struct{}{}

		if err := validatePullSecretName(override.SecretName); err != nil {
			return fmt.Errorf("namespace override secretName %q is invalid: %w", override.SecretName, err)
		}
	}

	return nil
}

func validateTargets(
	registryPullSecret pullsecretsv1alpha1.RegistryPullSecret,
	targets []NamespacePlan,
	existingSecrets map[string]*corev1.Secret,
) error {
	for _, target := range targets {
		if err := validatePullSecretName(target.SecretName); err != nil {
			return fmt.Errorf("resulting pull secret name %q for namespace %q is invalid: %w", target.SecretName, target.Namespace, err)
		}

		key := target.Namespace + "/" + target.SecretName
		existing := existingSecrets[key]
		if existing == nil {
			continue
		}
		if existing.Labels[metadata.ManagedByLabelKey] != metadata.ManagedByLabelValue {
			return fmt.Errorf("target Secret %s already exists and is not managed by this operator", key)
		}
		if existing.Labels[metadata.RegistryPullSecretNameLabelKey] != registryPullSecret.Name {
			return fmt.Errorf("target Secret %s is already managed by RegistryPullSecret %q", key, existing.Labels[metadata.RegistryPullSecretNameLabelKey])
		}
	}

	return nil
}

func policyAwareNamespaceInventory(
	registryPullSecret pullsecretsv1alpha1.RegistryPullSecret,
	policy pullsecretsv1alpha1.PullSecretPolicy,
) []string {
	known := make([]string, 0, len(registryPullSecret.Spec.Namespaces.Namespaces)+len(registryPullSecret.Spec.Namespaces.NamespaceOverrides)+len(policy.Spec.ExcludedNamespaces))
	seen := make(map[string]struct{})

	appendNamespace := func(namespace string) {
		if _, ok := seen[namespace]; ok {
			return
		}
		seen[namespace] = struct{}{}
		known = append(known, namespace)
	}

	for _, namespace := range registryPullSecret.Spec.Namespaces.Namespaces {
		appendNamespace(namespace)
	}
	for _, override := range registryPullSecret.Spec.Namespaces.NamespaceOverrides {
		appendNamespace(override.Namespace)
	}
	for _, namespace := range policy.Spec.ExcludedNamespaces {
		appendNamespace(namespace)
	}

	return known
}

func validateNamespaceName(namespace string) error {
	if strings.Contains(namespace, "*") {
		return fmt.Errorf("wildcard namespace patterns are not supported")
	}
	if errs := validation.IsDNS1123Label(namespace); len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "; "))
	}

	return nil
}

func validatePullSecretName(name string) error {
	if errs := validation.IsDNS1123Subdomain(name); len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "; "))
	}

	alnumCount := 0
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z':
			alnumCount++
		case r >= '0' && r <= '9':
			alnumCount++
		}
	}
	if alnumCount < 3 {
		return fmt.Errorf("must contain at least 3 alphanumeric characters")
	}

	return nil
}

func validateGloballyExcludedSelection(
	selection pullsecretsv1alpha1.NamespaceSelection,
	globallyExcludedNamespaces []string,
) error {
	if selection.Policy == pullsecretsv1alpha1.NamespaceSelectionPolicyInclusive {
		for _, namespace := range selection.Namespaces {
			if slices.Contains(globallyExcludedNamespaces, namespace) {
				return fmt.Errorf("namespace %q is explicitly selected but excluded by PullSecretPolicy", namespace)
			}
		}
	}

	for _, override := range selection.NamespaceOverrides {
		if slices.Contains(globallyExcludedNamespaces, override.Namespace) {
			return fmt.Errorf("namespace override %q is excluded by PullSecretPolicy", override.Namespace)
		}
	}

	return nil
}
