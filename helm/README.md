# Helm

This directory contains the hand-written Helm chart for installing the operator.

Current chart:
- `pull-secrets-operator`

The chart installs:
- CRDs for `PullSecretPolicy` and `RegistryPullSecret`
- service account and cluster-wide RBAC
- the operator deployment in the chosen Helm release namespace
