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
- Deleting a `RegistryPullSecret` is intentionally non-destructive for now and does not remove already replicated Secrets.
- `RegistryPullSecret` supports either inline credentials or a `credentialsSecretRef`, but not both at the same time.
- `RegistryPullSecret.spec.namespaces.targetSecretName` is optional and should be derived from the registry server when omitted.
- Namespace overrides are modeled as a list.
- Namespace entries and namespace overrides must use valid Kubernetes namespace names and may not be duplicated within their respective lists.
- Wildcard namespace patterns are intentionally unsupported for now.
- Resulting pull secret names must be valid Kubernetes Secret names and contain at least 3 alphanumeric characters.
- An explicitly selected namespace may not also be excluded by `PullSecretPolicy`.
- `RegistryPullSecret.status` reports observed generation, secret counts, last sync time, and a `Ready` condition.
- `PullSecretPolicy.status` reports observed generation, excluded namespace count, singleton activity, last sync time, and a `Ready` condition.
