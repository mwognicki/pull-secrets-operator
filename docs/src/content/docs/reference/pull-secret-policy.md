---
title: PullSecretPolicy
description: Full API reference for the PullSecretPolicy custom resource.
sidebar:
  order: 2
---

`PullSecretPolicy` is a cluster-scoped singleton resource that defines cluster-wide namespace exclusion rules. The operator recognizes only one instance, which **must be named `cluster`**.

```
apiVersion: pullsecrets.ognicki.ooo/v1alpha1
kind: PullSecretPolicy
```

## Singleton convention

Only the `PullSecretPolicy` named `cluster` is treated as the active policy. Additional objects of this kind are ignored by the operator. The `status.activeSingleton` field reflects whether the object is the recognized singleton.

## Spec

| Field | Type | Required | Description |
|---|---|---|---|
| `spec.excludedNamespaces` | string[] | No | Namespaces the operator must never replicate into. Applies across all `RegistryPullSecret` resources |

## Status

| Field | Type | Description |
|---|---|---|
| `observedGeneration` | integer | Generation of the resource that produced this status |
| `excludedNamespaceCount` | integer | Number of namespaces currently in `spec.excludedNamespaces` |
| `activeSingleton` | boolean | `true` when this object is named `cluster` and recognized as the active singleton |
| `valid` | boolean | `true` when the policy passes validation |
| `lastSyncTime` | timestamp | Time of the last reconciliation of this object |
| `conditions` | Condition[] | See [Status and conditions](../status-and-conditions/) |

## Validation rules

- Excluded namespace names must be valid Kubernetes namespace names. Wildcards are not supported.
- The same namespace may not appear more than once in `spec.excludedNamespaces`.

Violations are reported through the `Valid` condition.

## Precedence

Cluster-wide exclusions always override per-registry rules. A namespace in `excludedNamespaces` is skipped even if a `RegistryPullSecret` explicitly includes it with `Inclusive` policy. Attempting to do so is itself a validation error on the `RegistryPullSecret`.

## Effect timing

Exclusion changes take effect on the **next reconciliation** of each `RegistryPullSecret`:

- Adding a namespace to `excludedNamespaces` does **not** delete already-replicated secrets in that namespace.
- Removing a namespace from `excludedNamespaces` does **not** immediately backfill secrets — the backfill happens during the next `RegistryPullSecret` reconciliation.

## Example

```yaml
apiVersion: pullsecrets.ognicki.ooo/v1alpha1
kind: PullSecretPolicy
metadata:
  name: cluster
spec:
  excludedNamespaces:
    - kube-system
    - kube-public
    - kube-node-lease
    - cert-manager
    - pull-secrets
```
