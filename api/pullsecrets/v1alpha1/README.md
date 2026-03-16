# API Package

This package contains the current `pullsecrets.ognicki.ooo/v1alpha1` API.

## Resources

- `PullSecretPolicy`
  Cluster-wide namespace exclusion policy.
- `RegistryPullSecret`
  One registry credential source plus namespace targeting rules for replicated Docker pull secrets.

## `PullSecretPolicy`

- Intended to behave as a singleton-like resource.
- The conventional active object name is `cluster`.
- `spec.excludedNamespaces` defines namespaces the operator must ignore globally.
- Cluster-wide exclusions always take precedence over `RegistryPullSecret` targeting.
- Changing exclusions does not retroactively delete already replicated Secrets.
- Removing an exclusion does not automatically backfill Secrets into that namespace.

Current status fields:
- `observedGeneration`
- `excludedNamespaceCount`
- `activeSingleton`
- `valid`
- `lastSyncTime`
- `conditions`

Current conditions:
- `Valid`
- `Ready`

## `RegistryPullSecret`

- Represents one registry and its replication intent.
- Credentials can be provided either:
  - inline via `spec.credentials`, or
  - through `spec.credentialsSecretRef`
- Exactly one credential source should be used.
- A referenced credentials `Secret` should provide:
  - `server`
  - `username`
  - `password`
  - optional `email`
  - optional `auth`

Namespace targeting lives under `spec.namespaces`:
- `policy` is either `Inclusive` or `Exclusive`
- `namespaces` is interpreted according to that policy
- `targetSecretName` is optional
- `namespaceOverrides` is a list of per-namespace secret name overrides

When `targetSecretName` is omitted, the operator derives a stable pull-secret name from the registry server.

## Validation Rules

- Namespace entries and namespace overrides must use valid Kubernetes namespace names.
- Wildcard namespace patterns are intentionally unsupported.
- Namespace entries may not be duplicated within `namespaces`.
- Override namespaces may not be duplicated within `namespaceOverrides`.
- Explicitly selected namespaces may not also be excluded by the active `PullSecretPolicy`.
- Resulting secret names must be valid Kubernetes `Secret` names.
- Resulting secret names must contain at least 3 alphanumeric characters.
- Target names may not collide with existing foreign Secrets.

## Current Lifecycle Behavior

- `RegistryPullSecret` changes should reconcile promptly.
- Changing target names or namespace selection updates desired Secrets and removes obsolete managed Secrets.
- Deleting a `RegistryPullSecret` is intentionally non-destructive for now and leaves already replicated Secrets in place.
- Manual modification or deletion of already replicated managed Secrets is not immediately corrected; those replicas are only revisited on later reconciliation opportunities.

## `RegistryPullSecret` Status

Current status fields:
- `observedGeneration`
- `desiredSecretCount`
- `appliedSecretCount`
- `deletedSecretCount`
- `lastSyncTime`
- `conditions`

Current conditions:
- `Ready`

Validation failures are reported through concise status conditions and messages, typically with reason `ValidationFailed`.
