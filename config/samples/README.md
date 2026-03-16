# Sample Resources

This directory contains hand-written example custom resources demonstrating current API usage.

Current sample manifests:
- `pullsecrets_v1alpha1_pullsecretpolicy.yaml`
- `pullsecrets_v1alpha1_registry_credentials_secret.yaml`
- `pullsecrets_v1alpha1_registrypullsecret_ghcr.yaml`

Example shapes:

```yaml
apiVersion: pullsecrets.ognicki.ooo/v1alpha1
kind: PullSecretPolicy
metadata:
  name: cluster
spec:
  excludedNamespaces:
    - kube-system
    - cert-manager
```

```yaml
apiVersion: pullsecrets.ognicki.ooo/v1alpha1
kind: RegistryPullSecret
metadata:
  name: ghcr
spec:
  credentialsSecretRef:
    name: ghcr-credentials
    namespace: pull-secrets-operator-system
  namespaces:
    policy: Exclusive
    namespaces:
      - kube-system
    namespaceOverrides:
      - namespace: team-a
        secretName: team-a-ghcr
```

Notes:
- exactly one of `credentials` or `credentialsSecretRef` should be set.
- when `credentialsSecretRef` is used, the referenced Secret should contain `server`, `username`, `password`, and optionally `email` and `auth` data keys.
- `targetSecretName` is optional and should be derived from the registry server when omitted.
- explicit `RegistryPullSecret` changes should be reconciled promptly by the operator.
- `PullSecretPolicy` exclusions take precedence over `RegistryPullSecret` targeting.
