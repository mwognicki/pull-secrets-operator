---
title: Installation
description: Install pull-secrets-operator into your Kubernetes cluster using Helm or raw manifests.
sidebar:
  order: 1
---

## Requirements

- Kubernetes 1.24 or later
- Cluster-admin access (the operator needs cluster-scoped RBAC)

## Helm

The Helm chart installs the CRDs, RBAC, and the operator deployment in a single command.

```bash
helm install pull-secrets-operator oci://ghcr.io/mwognicki/charts/pull-secrets-operator \
  --namespace pull-secrets \
  --create-namespace
```

To install a specific version:

```bash
helm install pull-secrets-operator oci://ghcr.io/mwognicki/charts/pull-secrets-operator \
  --version 0.1.0 \
  --namespace pull-secrets \
  --create-namespace
```

### Chart values

| Value | Default | Description |
|---|---|---|
| `image.repository` | `ghcr.io/mwognicki/pull-secrets-operator` | Operator image repository |
| `image.tag` | chart `appVersion` | Image tag |
| `image.pullPolicy` | `IfNotPresent` | Image pull policy |
| `replicaCount` | `1` | Number of operator replicas |
| `leaderElection.enabled` | `true` | Enable leader election (required for `replicaCount > 1`) |

## Raw manifests

If you prefer to manage the install yourself, apply the hand-written manifests from the repository in order:

```bash
# 1. Install CRDs
kubectl apply -f config/crd/

# 2. Create the operator namespace
kubectl create namespace pull-secrets

# 3. Apply RBAC
kubectl apply -f config/rbac/

# 4. Deploy the operator
kubectl apply -f config/manager/
```

## Verify the installation

```bash
kubectl -n pull-secrets rollout status deployment/pull-secrets-operator-manager
```

Once the rollout completes, the operator is ready to reconcile `RegistryPullSecret` resources.

## Uninstall

### Helm

```bash
helm uninstall pull-secrets-operator --namespace pull-secrets
kubectl delete namespace pull-secrets
```

:::note
Uninstalling the operator does **not** delete already-replicated pull secrets from your workload namespaces. You will need to clean those up manually if desired.
:::

### Raw manifests

```bash
kubectl delete -f config/manager/
kubectl delete -f config/rbac/
kubectl delete -f config/crd/
kubectl delete namespace pull-secrets
```
