package sync

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"maps"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	pullsecretsv1alpha1 "github.com/mwognicki/pull-secrets-operator/api/pullsecrets/v1alpha1"
	"github.com/mwognicki/pull-secrets-operator/pkg/metadata"
)

const dockerConfigJSONKey = ".dockerconfigjson"

// DesiredSecret describes the desired replicated Secret and whether an existing
// Secret needs to be created or updated.
type DesiredSecret struct {
	Secret     *corev1.Secret
	NeedsApply bool
}

var dockerConfigJSONMarshal = json.Marshal
var desiredSecretTargets = EffectiveTargets

// ObsoleteSecrets returns Secrets managed for the given RegistryPullSecret that are
// no longer part of the desired target set and should be deleted on reconciliation.
func ObsoleteSecrets(
	registryPullSecret pullsecretsv1alpha1.RegistryPullSecret,
	existingSecrets map[string]*corev1.Secret,
	desiredSecrets []DesiredSecret,
) []*corev1.Secret {
	desiredKeys := make(map[string]struct{}, len(desiredSecrets))
	for _, desiredSecret := range desiredSecrets {
		desiredKeys[desiredSecret.Secret.Namespace+"/"+desiredSecret.Secret.Name] = struct{}{}
	}

	obsoleteSecrets := make([]*corev1.Secret, 0)
	for key, existingSecret := range existingSecrets {
		if existingSecret.Labels[metadata.ManagedByLabelKey] != metadata.ManagedByLabelValue {
			continue
		}
		if existingSecret.Labels[metadata.RegistryPullSecretNameLabelKey] != registryPullSecret.Name {
			continue
		}
		if _, ok := desiredKeys[key]; ok {
			continue
		}

		obsoleteSecrets = append(obsoleteSecrets, existingSecret.DeepCopy())
	}

	return obsoleteSecrets
}

// DesiredSecrets builds the desired dockerconfigjson Secrets for the provided namespace inventory.
func DesiredSecrets(
	registryPullSecret pullsecretsv1alpha1.RegistryPullSecret,
	credentials pullsecretsv1alpha1.RegistryCredentials,
	policy pullsecretsv1alpha1.PullSecretPolicy,
	allNamespaces []string,
	existingSecrets map[string]*corev1.Secret,
) ([]DesiredSecret, error) {
	if err := ValidateRegistryPullSecret(registryPullSecret, credentials, policy, existingSecrets); err != nil {
		return nil, err
	}

	targets, err := desiredSecretTargets(registryPullSecret, credentials, policy, allNamespaces)
	if err != nil {
		return nil, err
	}

	dockerConfigJSON, err := DockerConfigJSON(credentials)
	if err != nil {
		return nil, err
	}

	desiredSecrets := make([]DesiredSecret, 0, len(targets))
	for _, target := range targets {
		secret := BuildPullSecret(registryPullSecret, credentials, target, dockerConfigJSON)
		key := target.Namespace + "/" + target.SecretName
		desiredSecrets = append(desiredSecrets, DesiredSecret{
			Secret:     secret,
			NeedsApply: SecretNeedsApply(existingSecrets[key], secret),
		})
	}

	return desiredSecrets, nil
}

// BuildPullSecret creates the desired Kubernetes Secret object for one namespace target.
func BuildPullSecret(
	registryPullSecret pullsecretsv1alpha1.RegistryPullSecret,
	credentials pullsecretsv1alpha1.RegistryCredentials,
	target NamespacePlan,
	dockerConfigJSON []byte,
) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      target.SecretName,
			Namespace: target.Namespace,
			Labels: map[string]string{
				metadata.ManagedByLabelKey:              metadata.ManagedByLabelValue,
				metadata.RegistryPullSecretNameLabelKey: registryPullSecret.Name,
				metadata.RegistryServerLabelKey:         credentials.Server,
			},
		},
		Type: corev1.SecretTypeDockerConfigJson,
		Data: map[string][]byte{
			dockerConfigJSONKey: dockerConfigJSON,
		},
	}
}

// SecretNeedsApply reports whether the existing Secret differs from the desired one
// in a way that requires create or update.
func SecretNeedsApply(existing, desired *corev1.Secret) bool {
	if existing == nil {
		return true
	}

	if existing.Type != desired.Type {
		return true
	}

	if !maps.Equal(existing.Labels, desired.Labels) {
		return true
	}

	return !maps.EqualFunc(existing.Data, desired.Data, func(left, right []byte) bool {
		if len(left) != len(right) {
			return false
		}
		for i := range left {
			if left[i] != right[i] {
				return false
			}
		}
		return true
	})
}

// DockerConfigJSON renders the registry credentials into the canonical docker config payload.
func DockerConfigJSON(credentials pullsecretsv1alpha1.RegistryCredentials) ([]byte, error) {
	if credentials.Server == "" {
		return nil, fmt.Errorf("registry credentials server is empty")
	}
	if credentials.Username == "" {
		return nil, fmt.Errorf("registry credentials username is empty")
	}
	if credentials.Password == "" {
		return nil, fmt.Errorf("registry credentials password is empty")
	}

	auth := credentials.Auth
	if auth == "" {
		auth = base64.StdEncoding.EncodeToString([]byte(credentials.Username + ":" + credentials.Password))
	}

	payload := dockerConfigJSON{
		Auths: map[string]dockerAuthEntry{
			credentials.Server: {
				Username: credentials.Username,
				Password: credentials.Password,
				Email:    credentials.Email,
				Auth:     auth,
			},
		},
	}

	rendered, err := dockerConfigJSONMarshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal docker config json: %w", err)
	}

	return rendered, nil
}

type dockerConfigJSON struct {
	Auths map[string]dockerAuthEntry `json:"auths"`
}

type dockerAuthEntry struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Email    string `json:"email,omitempty"`
	Auth     string `json:"auth"`
}
