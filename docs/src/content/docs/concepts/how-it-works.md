---
title: How it works
description: A conceptual overview of how pull-secrets-operator reconciles RegistryPullSecret resources into Kubernetes Secrets.
sidebar:
  order: 1
---

## Resources

The operator introduces two cluster-scoped custom resources in the `pullsecrets.ognicki.ooo/v1alpha1` API group:

| Resource | Role |
|---|---|
| `RegistryPullSecret` | Declares credentials for one registry and the namespaces it should be replicated into |
| `PullSecretPolicy` | Cluster-wide singleton that defines namespaces the operator must never touch |

Neither resource is namespace-scoped. Both live at the cluster level.

## Reconciliation loop

When a `RegistryPullSecret` is created or updated, the operator:

1. Reads the active `PullSecretPolicy` (named `cluster`). If none exists, it proceeds with no exclusions.
2. Resolves the registry credentials — from inline `spec.credentials` or a referenced Kubernetes `Secret`.
3. Lists all namespaces in the cluster.
4. Computes the **desired state**: which namespaces are eligible, what the secret should be named in each, and what the `dockerconfigjson` payload should contain.
5. Compares desired state against existing `Secret` objects.
6. **Creates or updates** secrets that are missing or out of date.
7. **Deletes** managed secrets that are no longer in the desired set (for example, after a namespace is removed from the selection or a target name changes).
8. Updates the `RegistryPullSecret` status with counts and a `Ready` condition.

## Credential resolution

Credentials can come from two places — exactly one must be used:

- **Inline** (`spec.credentials`): the server, username, password, and optional email/auth fields are embedded directly in the `RegistryPullSecret`.
- **Referenced** (`spec.credentialsSecretRef`): points to an Opaque `Secret` in any namespace. The operator watches that secret and re-reconciles whenever it changes.

## Secret format

Replicated secrets are always of type `kubernetes.io/dockerconfigjson`. The operator renders the canonical `.dockerconfigjson` payload from the resolved credentials and places it in each target namespace under the computed name.

Managed secrets carry labels that the operator uses to track ownership:

```
app.kubernetes.io/managed-by: pull-secrets-operator
pullsecrets.ognicki.ooo/registry-pull-secret: <source-name>
pullsecrets.ognicki.ooo/registry-server: <server>
```

## Default secret naming

When `spec.namespaces.targetSecretName` is not set, the operator derives a name from the registry server hostname:

1. Strips the scheme and port.
2. Removes the TLD and common secondary domain tokens (`com`, `net`, `co`, `org`, `gov`, `edu`).
3. Filters out generic tokens (`docker`, `registry`, `www`).
4. Takes the last meaningful label.
5. Appends `-pull-secret`.

Examples:

| Registry server | Derived name |
|---|---|
| `ghcr.io` | `ghcr-pull-secret` |
| `registry.gitlab.com` | `gitlab-pull-secret` |
| `docker.io` | `docker-pull-secret` |
| `my-registry.internal:5000` | `my-registry-pull-secret` |

## Drift behavior

The operator does **not** continuously watch replicated secrets for drift. If a managed secret is manually edited or deleted, the change is only corrected the next time a reconciliation is triggered — for example, when the `RegistryPullSecret` is updated or the operator restarts.

## Deletion behavior

Deleting a `RegistryPullSecret` is **non-destructive**. The operator does not use finalizers and makes no attempt to clean up already-replicated secrets when the source resource disappears. Your workloads continue to function.

## PullSecretPolicy

The `PullSecretPolicy` is a singleton: only the object named `cluster` is recognized as active. Its only current function is `spec.excludedNamespaces`, which lists namespaces the operator must skip regardless of what any `RegistryPullSecret` says.

Exclusions take effect on the **next reconciliation** of each `RegistryPullSecret`. Adding a namespace to the exclusion list does not retroactively delete already-replicated secrets in that namespace. Removing an exclusion does not immediately backfill secrets — the next `RegistryPullSecret` reconciliation does that.
