---
title: Referenced credentials
description: Store registry credentials in a Kubernetes Secret and reference it from a RegistryPullSecret.
sidebar:
  order: 2
---

Referenced credentials keep sensitive data in a standard Kubernetes `Secret` and point to it from the `RegistryPullSecret`. This lets you control access with standard Kubernetes RBAC and rotate credentials without touching the `RegistryPullSecret` object.

## Step 1 — Create the credentials Secret

The Secret must be of type `Opaque` and use specific data keys.

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: ghcr-credentials
  namespace: pull-secrets
type: Opaque
stringData:
  server: ghcr.io
  username: your-username
  password: your-token
  email: ops@example.com   # optional
```

```bash
kubectl apply -f ghcr-credentials.yaml
```

### Required and optional data keys

| Key | Required | Description |
|---|---|---|
| `server` | Yes | Registry hostname |
| `username` | Yes | Registry username |
| `password` | Yes | Registry password or access token |
| `email` | No | Email address associated with the registry account |
| `auth` | No | Pre-encoded `base64(username:password)`. Included as-is in the rendered `dockerconfigjson` if present |

## Step 2 — Reference the Secret

Point `spec.credentialsSecretRef` at the Secret you created.

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
```

```bash
kubectl apply -f registrypullsecret.yaml
```

The `namespace` field in `credentialsSecretRef` is required — the Secret can live in any namespace.

## Credential rotation

Update the source Secret. The operator watches it and reconciles immediately, propagating the new credentials to all managed replica secrets.

```bash
kubectl -n pull-secrets create secret generic ghcr-credentials \
  --from-literal=server=ghcr.io \
  --from-literal=username=your-username \
  --from-literal=password=new-token \
  --dry-run=client -o yaml | kubectl apply -f -
```

## Mutually exclusive with inline credentials

You must use exactly one of `spec.credentials` or `spec.credentialsSecretRef`. Using both, or neither, results in a `ValidationFailed` condition on the `RegistryPullSecret`.

## Recommended placement

Keeping the credentials Secret in the same namespace as the operator (`pull-secrets` by default) makes the trust boundary clear: RBAC on that namespace controls who can read or modify the actual passwords, while `RegistryPullSecret` objects themselves can safely be read by a broader audience.
