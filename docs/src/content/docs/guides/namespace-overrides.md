---
title: Namespace overrides
description: Use a different secret name in specific namespaces while keeping the default everywhere else.
sidebar:
  order: 3
---

By default the operator derives a single secret name from the registry hostname and uses it in every target namespace. Namespace overrides let you use a different name for specific namespaces — useful when a workload expects a particular secret name or when a name collision must be avoided.

## Default name derivation

When `spec.namespaces.targetSecretName` is not set, the operator derives a name from the registry server. For example:

- `ghcr.io` → `ghcr-pull-secret`
- `registry.gitlab.com` → `gitlab-pull-secret`
- `my-registry.internal:5000` → `my-registry-pull-secret`

See [How it works — default secret naming](../../concepts/how-it-works/#default-secret-naming) for the full algorithm.

## Global override

To change the default name for **all** target namespaces, set `spec.namespaces.targetSecretName`:

```yaml
spec:
  namespaces:
    policy: Exclusive
    namespaces:
      - kube-system
    targetSecretName: my-registry-pull-secret
```

Every eligible namespace receives a secret named `my-registry-pull-secret`.

## Per-namespace overrides

To use a different name in **specific** namespaces while keeping the default (or global override) elsewhere, use `spec.namespaces.namespaceOverrides`:

```yaml
spec:
  namespaces:
    policy: Exclusive
    namespaces:
      - kube-system
    namespaceOverrides:
      - namespace: team-a
        secretName: team-a-ghcr
      - namespace: team-b
        secretName: team-b-registry-creds
```

In this example:
- `team-a` gets a secret named `team-a-ghcr`
- `team-b` gets a secret named `team-b-registry-creds`
- All other namespaces get the derived default name (e.g. `ghcr-pull-secret`)

## Combining global and per-namespace overrides

`namespaceOverrides` takes precedence over `targetSecretName`:

```yaml
spec:
  namespaces:
    policy: Exclusive
    namespaces:
      - kube-system
    targetSecretName: our-registry-pull-secret   # used everywhere unless overridden below
    namespaceOverrides:
      - namespace: team-a
        secretName: team-a-special-pull-secret   # only team-a uses this name
```

## Validation constraints

The operator enforces:

- Override namespace names must be valid Kubernetes namespace names. Wildcards are not supported.
- The same namespace may not appear more than once in `namespaceOverrides`.
- Secret names must be valid Kubernetes `Secret` names and contain at least 3 alphanumeric characters.
- A target name (default or override) may not collide with an existing `Secret` that is not already managed by this operator for this registry.

Violations are reported as a `ValidationFailed` condition on the `RegistryPullSecret` and block reconciliation until resolved.

## Changing override names

Updating a `namespaceOverrides` entry causes the operator to:

1. Create (or update) the secret under the new name.
2. Delete the managed secret under the old name.

This happens in a single reconciliation pass. Workloads referencing the old name will see it disappear — update your Pod specs or `imagePullSecrets` references before or alongside the override change.
