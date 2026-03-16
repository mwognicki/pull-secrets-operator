package sync

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"

	pullsecretsv1alpha1 "github.com/mwognicki/pull-secrets-operator/api/pullsecrets/v1alpha1"
)

const (
	credentialServerKey   = "server"
	credentialUsernameKey = "username"
	credentialPasswordKey = "password"
	credentialEmailKey    = "email"
	credentialAuthKey     = "auth"
)

// ResolveRegistryCredentials returns the effective registry credentials from either
// inline spec fields or a referenced Kubernetes Secret.
func ResolveRegistryCredentials(
	spec pullsecretsv1alpha1.RegistryPullSecretSpec,
	sourceSecret *corev1.Secret,
) (pullsecretsv1alpha1.RegistryCredentials, error) {
	hasInline := spec.Credentials != nil
	hasSecretRef := spec.CredentialsSecretRef != nil

	switch {
	case hasInline && hasSecretRef:
		return pullsecretsv1alpha1.RegistryCredentials{}, fmt.Errorf("exactly one of credentials or credentialsSecretRef must be set")
	case !hasInline && !hasSecretRef:
		return pullsecretsv1alpha1.RegistryCredentials{}, fmt.Errorf("exactly one of credentials or credentialsSecretRef must be set")
	case hasInline:
		return *spec.Credentials, nil
	default:
		if sourceSecret == nil {
			return pullsecretsv1alpha1.RegistryCredentials{}, fmt.Errorf("credentialsSecretRef is set but source Secret was not provided")
		}
		return credentialsFromSecret(sourceSecret)
	}
}

func credentialsFromSecret(secret *corev1.Secret) (pullsecretsv1alpha1.RegistryCredentials, error) {
	server, err := requiredSecretString(secret, credentialServerKey)
	if err != nil {
		return pullsecretsv1alpha1.RegistryCredentials{}, err
	}
	username, err := requiredSecretString(secret, credentialUsernameKey)
	if err != nil {
		return pullsecretsv1alpha1.RegistryCredentials{}, err
	}
	password, err := requiredSecretString(secret, credentialPasswordKey)
	if err != nil {
		return pullsecretsv1alpha1.RegistryCredentials{}, err
	}

	return pullsecretsv1alpha1.RegistryCredentials{
		Server:   server,
		Username: username,
		Password: password,
		Email:    optionalSecretString(secret, credentialEmailKey),
		Auth:     optionalSecretString(secret, credentialAuthKey),
	}, nil
}

func requiredSecretString(secret *corev1.Secret, key string) (string, error) {
	value := optionalSecretString(secret, key)
	if value == "" {
		return "", fmt.Errorf("source Secret %s/%s is missing required key %q", secret.Namespace, secret.Name, key)
	}
	return value, nil
}

func optionalSecretString(secret *corev1.Secret, key string) string {
	if secret == nil || secret.Data == nil {
		return ""
	}
	return strings.TrimSpace(string(secret.Data[key]))
}
