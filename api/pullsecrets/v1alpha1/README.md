# API Package

This package contains the initial `pullsecrets.ognicki.ooo/v1alpha1` API scaffolding.

Current resources:
- `PullSecretPolicy` for cluster-wide namespace exclusion rules
- `RegistryPullSecret` for per-registry credentials and namespace targeting

Current API notes:
- `PullSecretPolicy` is intended to behave as a singleton-like resource, conventionally named `cluster`.
- Cluster-wide exclusions always take precedence over per-registry targeting rules.
- Changing cluster-wide exclusions does not retroactively delete or backfill replicated secrets.
- `RegistryPullSecret` changes should be reconciled promptly so explicit spec updates are reflected quickly.
- `RegistryPullSecret.spec.namespaces.targetSecretName` is optional and should be derived from the registry server when omitted.
- Namespace overrides are modeled as a list.
