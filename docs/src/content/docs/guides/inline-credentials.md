---
title: Inline credentials
description: Embed registry credentials directly in a RegistryPullSecret resource.
sidebar:
  order: 1
---

Inline credentials are the simplest way to provide registry credentials. All fields are embedded directly in the `RegistryPullSecret` spec — no separate `Secret` object required.

:::caution
`RegistryPullSecret` is a cluster-scoped resource. Its contents are visible to anyone with `get` or `list` access to `RegistryPullSecret` objects. If your credentials are sensitive, prefer [referenced credentials](/guides/referenced-credentials/) instead.
:::

## Example

```yaml
apiVersion: pullsecrets.ognicki.ooo/v1alpha1
kind: RegistryPullSecret
metadata:
  name: my-registry
spec:
  credentials:
    server: registry.example.com
    username: ci-robot
    password: s3cr3t
    email: ci@example.com   # optional
  namespaces:
    policy: Exclusive
    namespaces:
      - kube-system
```

## Fields

| Field | Required | Description |
|---|---|---|
| `credentials.server` | Yes | Registry hostname (with or without scheme). Example: `ghcr.io` |
| `credentials.username` | Yes | Registry username |
| `credentials.password` | Yes | Registry password or token |
| `credentials.email` | No | Email address associated with the registry account |
| `credentials.auth` | No | Pre-encoded `base64(username:password)` string. Included in the rendered `dockerconfigjson` if provided |

## Mutually exclusive with `credentialsSecretRef`

You must use exactly one of `spec.credentials` or `spec.credentialsSecretRef`. Setting both, or neither, is a validation error that sets `Ready=False` with reason `ValidationFailed`.

## Updating credentials

Edit the `RegistryPullSecret` resource directly. The operator reconciles the change immediately, updating all managed replica secrets.

```bash
kubectl edit registrypullsecret my-registry
```

Or apply a new manifest:

```bash
kubectl apply -f my-registry.yaml
```
