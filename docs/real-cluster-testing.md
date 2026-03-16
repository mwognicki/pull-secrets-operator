# Real Cluster Testing

This document describes the first local iteration of real Kubernetes cluster testing for the operator.

For a concise coverage summary, see [docs/real-cluster-tested-scope.md](/Users/marek/Work/Ognicki/pull-secrets-operator/docs/real-cluster-tested-scope.md).

## Current Scope

The current smoke test is intentionally small and is meant to run from a developer machine against a real cluster.

It covers:
- operator installation from the hand-written manifests
- manager rollout using a configurable image
- successful replication into included namespaces
- per-namespace secret name override behavior
- cluster-wide namespace exclusion taking precedence
- validation failure when a `RegistryPullSecret` explicitly targets a cluster-excluded namespace

It does not yet cover:
- multiple registry providers in one run
- mutation or deletion scenarios after initial sync
- automatic execution from GitHub Actions
- Tailscale or kubeconfig bootstrapping inside CI

## Script

Use:

```bash
hack/real-cluster-smoke.sh
```

The script automatically loads repository-root `.env` and `.env.local` files before reading its `PSO_*` settings.

Required environment variables:

```bash
export PSO_TEST_REGISTRY_SERVER='ghcr.io'
export PSO_TEST_REGISTRY_USERNAME='your-user'
export PSO_TEST_REGISTRY_PASSWORD='your-password-or-token'
```

Optional environment variables:

```bash
export PSO_TEST_REGISTRY_EMAIL='ops@example.com'
export PSO_IMAGE='ghcr.io/mwognicki/pull-secrets-operator:v0.1.0-beta.1'
export PSO_OPERATOR_NAMESPACE='pull-secrets'
export PSO_WAIT_TIMEOUT='180s'
export PSO_TEST_ID='manualrun01'
```

## Resource Isolation

Each run creates unique namespaces using the prefix:

```text
psop-e2e-<test-id>-...
```

The script also creates:
- a temporary credentials `Secret` in the operator namespace
- one valid `RegistryPullSecret`
- one intentionally invalid `RegistryPullSecret`
- the singleton `PullSecretPolicy` named `cluster`

Cleanup is attempted automatically when the script exits.
The script also performs a pre-run reset of the operator installation so repeated runs start from a clean slate.

That reset currently removes:
- the operator deployment resources from `config/manager`
- the service account and RBAC from `config/rbac`
- both CRDs from `config/crd`
- the operator namespace `pull-secrets`

## Future CI Direction

This script is intended to become the shared test entrypoint for a later GitHub Actions workflow.

The future CI wrapper will need to provide:
- cluster connectivity through Tailscale
- kubeconfig material from GitHub Secrets
- test registry credentials from GitHub Secrets
- an operator image built from the pull request under test
