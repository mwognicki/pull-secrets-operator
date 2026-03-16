---
title: RegistryPullSecret
description: Full API reference for the RegistryPullSecret custom resource.
sidebar:
  order: 1
---

`RegistryPullSecret` is a cluster-scoped resource that declares the credentials for one container registry and the rules for replicating its pull secret across namespaces.

```
apiVersion: pullsecrets.ognicki.ooo/v1alpha1
kind: RegistryPullSecret
```

## Spec

### `spec.credentials`

Inline credentials. Mutually exclusive with `spec.credentialsSecretRef`.

| Field | Type | Required | Description |
|---|---|---|---|
| `credentials.server` | string | Yes | Registry hostname, with or without scheme. Example: `ghcr.io` |
| `credentials.username` | string | Yes | Registry username |
| `credentials.password` | string | Yes | Registry password or access token |
| `credentials.email` | string | No | Email address for the registry account |
| `credentials.auth` | string | No | Pre-encoded `base64(username:password)`. Passed through as-is into the rendered `dockerconfigjson` |

### `spec.credentialsSecretRef`

Reference to an Opaque `Secret` containing credentials. Mutually exclusive with `spec.credentials`.

| Field | Type | Required | Description |
|---|---|---|---|
| `credentialsSecretRef.name` | string | Yes | Name of the source Secret |
| `credentialsSecretRef.namespace` | string | Yes | Namespace of the source Secret |

The Secret must provide the same keys as the inline fields: `server`, `username`, `password`, and optionally `email` and `auth`.

### `spec.namespaces`

Controls which namespaces receive the replicated secret.

| Field | Type | Required | Description |
|---|---|---|---|
| `namespaces.policy` | `Inclusive` \| `Exclusive` | Yes | How to interpret `namespaces.namespaces` |
| `namespaces.namespaces` | string[] | Yes | Namespace names — included (Inclusive) or excluded (Exclusive) |
| `namespaces.targetSecretName` | string | No | Secret name to use in all target namespaces. Derived from the registry server when omitted |
| `namespaces.namespaceOverrides` | object[] | No | Per-namespace secret name overrides. See below |

#### `namespaces.namespaceOverrides[]`

| Field | Type | Required | Description |
|---|---|---|---|
| `namespace` | string | Yes | Name of the namespace to override |
| `secretName` | string | Yes | Secret name to use in that namespace |

## Status

| Field | Type | Description |
|---|---|---|
| `observedGeneration` | integer | Generation of the resource that produced this status |
| `desiredSecretCount` | integer | Number of namespaces the operator intends to replicate into |
| `appliedSecretCount` | integer | Number of secrets created or updated in the last reconciliation |
| `deletedSecretCount` | integer | Number of obsolete managed secrets deleted in the last reconciliation |
| `lastSyncTime` | timestamp | Time of the last reconciliation attempt |
| `conditions` | Condition[] | See [Status and conditions](../status-and-conditions/) |

## Validation rules

The operator enforces these semantic constraints:

- Exactly one of `credentials` or `credentialsSecretRef` must be set.
- `namespaces.policy` must be `Inclusive` or `Exclusive`.
- Namespace names must be valid Kubernetes namespace names. Wildcard patterns are not supported.
- Namespace entries in `namespaces.namespaces` may not be duplicated.
- Namespace entries in `namespaceOverrides` may not be duplicated.
- A namespace listed in `namespaces.namespaces` (for `Inclusive` policy) may not also appear in `PullSecretPolicy.spec.excludedNamespaces`.
- Target secret names must be valid Kubernetes `Secret` names and contain at least 3 alphanumeric characters.
- Target names may not collide with an existing `Secret` that is not already managed by this operator for this registry.

Validation failures are reported through the `Ready` condition with reason `ValidationFailed`. Reconciliation is blocked until the error is resolved.

## Lifecycle

- Changes reconcile promptly.
- Changing namespace selection or secret names causes the operator to create new secrets and delete the now-obsolete managed secrets.
- Deleting a `RegistryPullSecret` is **non-destructive**: already-replicated secrets are left in place.

## Example

```yaml
apiVersion: pullsecrets.ognicki.ooo/v1alpha1
kind: RegistryPullSecret
metadata:
  name: ghcr
spec:
  credentialsSecretRef:
    name: ghcr-credentials
    namespace: pull-secrets
  namespaces:
    policy: Exclusive
    namespaces:
      - kube-system
    targetSecretName: ghcr-pull-secret
    namespaceOverrides:
      - namespace: team-a
        secretName: team-a-ghcr
```
