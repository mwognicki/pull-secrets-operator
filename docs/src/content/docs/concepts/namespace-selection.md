---
title: Namespace selection
description: How pull-secrets-operator decides which namespaces receive a replicated pull secret.
sidebar:
  order: 2
---

Each `RegistryPullSecret` controls which namespaces receive its replicated secret through two complementary mechanisms: a **namespace policy** on the resource itself, and a **cluster-wide exclusion list** on the `PullSecretPolicy` singleton.

## Per-registry policy

The `spec.namespaces.policy` field accepts one of two values.

### Inclusive

Only namespaces explicitly listed in `spec.namespaces.namespaces` are eligible.

```yaml
spec:
  namespaces:
    policy: Inclusive
    namespaces:
      - team-a
      - team-b
      - staging
```

Use `Inclusive` when you want tight control and prefer to opt namespaces in explicitly.

### Exclusive

All namespaces in the cluster are eligible **except** the ones listed in `spec.namespaces.namespaces`.

```yaml
spec:
  namespaces:
    policy: Exclusive
    namespaces:
      - kube-system
      - kube-public
      - kube-node-lease
```

Use `Exclusive` when you want broad coverage and prefer to opt a few namespaces out.

## Cluster-wide exclusions

The `PullSecretPolicy` singleton (name: `cluster`) holds a list of namespaces that the operator will **never** touch, regardless of what any `RegistryPullSecret` says.

```yaml
apiVersion: pullsecrets.ognicki.ooo/v1alpha1
kind: PullSecretPolicy
metadata:
  name: cluster
spec:
  excludedNamespaces:
    - kube-system
    - cert-manager
    - pull-secrets
```

Cluster-wide exclusions always take precedence. A namespace in `excludedNamespaces` is skipped even if a `RegistryPullSecret` explicitly includes it — and doing so is a **validation error** that blocks reconciliation of that resource.

## Precedence rules

The effective set of target namespaces for a `RegistryPullSecret` is determined by applying both layers in order:

1. Start with all namespaces in the cluster.
2. Apply the per-registry `Inclusive` or `Exclusive` policy.
3. Remove any namespace that appears in `PullSecretPolicy.spec.excludedNamespaces`.

The final set is what the operator replicates into.

## Validation constraints

The operator enforces these rules and reports violations as a `ValidationFailed` condition on the `RegistryPullSecret`:

- Namespace names must be valid Kubernetes namespace names. Wildcard patterns (e.g., `team-*`) are not supported.
- The same namespace may not appear more than once in `spec.namespaces.namespaces`.
- A namespace listed in `spec.namespaces.namespaces` may not also be listed in `PullSecretPolicy.spec.excludedNamespaces`.

## Effect of changing exclusions

Exclusion changes take effect on the next reconciliation of each `RegistryPullSecret`:

- **Adding** a namespace to `excludedNamespaces` does **not** retroactively delete already-replicated secrets in that namespace.
- **Removing** a namespace from `excludedNamespaces` does **not** immediately backfill secrets — the backfill happens on the next `RegistryPullSecret` reconciliation.
