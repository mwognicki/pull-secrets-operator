---
title: Quickstart
description: Replicate your first pull secret across namespaces in under five minutes.
sidebar:
  order: 2
---

This guide walks you through replicating a pull secret for `ghcr.io` into all namespaces except `kube-system`, with a custom secret name for one team namespace.

## Before you begin

Make sure the operator is [installed](/getting-started/installation/) and running:

```bash
kubectl -n pull-secrets rollout status deployment/pull-secrets-operator-manager
```

## Step 1 — Exclude system namespaces

Create a `PullSecretPolicy` to keep the operator away from namespaces it should not touch. This resource is a **cluster-wide singleton** and must be named `cluster`.

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

```bash
kubectl apply -f pullsecretpolicy.yaml
```

You can skip this step if you do not need global exclusions.

## Step 2 — Store your registry credentials

Create a standard Kubernetes `Secret` with your registry credentials. The `pull-secrets` namespace is a good place to keep these.

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
  password: your-token-or-password
  email: ops@example.com   # optional
```

```bash
kubectl apply -f ghcr-credentials.yaml
```

## Step 3 — Create a RegistryPullSecret

Define one `RegistryPullSecret` for `ghcr.io`. This example uses the `Exclusive` policy to target every namespace **except** `kube-system` (which is also covered by the `PullSecretPolicy`, so it is excluded either way).

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
    namespaceOverrides:
      - namespace: team-a
        secretName: team-a-ghcr
```

```bash
kubectl apply -f registrypullsecret.yaml
```

## Step 4 — Verify

Check the operator reconciled successfully:

```bash
kubectl get registrypullsecret ghcr -o yaml
```

Look for:

```yaml
status:
  conditions:
    - type: Ready
      status: "True"
      reason: Synced
  desiredSecretCount: 5   # one per eligible namespace
  appliedSecretCount: 5
```

Check that the secret landed in a target namespace:

```bash
kubectl -n default get secret ghcr-pull-secret
```

And the custom name in `team-a`:

```bash
kubectl -n team-a get secret team-a-ghcr
```

## What happens next

- When you add a new namespace, the operator replicates the secret on its next reconciliation.
- When you update `ghcr-credentials`, the operator syncs the change to all managed replicas.
- Removing the `RegistryPullSecret` **does not** delete the replicated secrets — your workloads keep running.
