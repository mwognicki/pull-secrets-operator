package metadata

const (
	// ManagedByLabelKey identifies the managing operator.
	ManagedByLabelKey = "app.kubernetes.io/managed-by"
	// ManagedByLabelValue is the canonical manager label value for this operator.
	ManagedByLabelValue = "pull-secrets-operator"

	// RegistryPullSecretNameLabelKey links a replicated Secret back to its source RegistryPullSecret.
	RegistryPullSecretNameLabelKey = "pullsecrets.ognicki.ooo/registry-pull-secret"
	// RegistryServerLabelKey records the registry server associated with the replicated Secret.
	RegistryServerLabelKey = "pullsecrets.ognicki.ooo/registry-server"
)
