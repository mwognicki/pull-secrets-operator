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
- `Exclusive` namespace policy behavior
- inline credentials mode
- prompt reconciliation after updating an existing `RegistryPullSecret`
- removal of obsolete managed secrets after changing target namespaces or secret names
- non-destructive behavior when deleting a `RegistryPullSecret`
- duplicate namespace validation in explicit namespace lists
- duplicate namespace validation in namespace overrides
- wildcard namespace rejection
- invalid explicit target secret name rejection
- collision rejection for a foreign unmanaged target `Secret`
- behavior when managed replica `Secret` objects are manually modified
- behavior when managed replica `Secret` objects are manually deleted
- multi-registry coverage using additional registry providers

It does not yet cover:
- multiple registry providers in one run
- mutation or deletion scenarios after initial sync
- invalid Kubernetes namespace-name rejection
- `PullSecretPolicy` validity edge cases
- automatic pull-request execution from GitHub Actions
- PR-scoped image build and test wiring inside CI

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
export PSO_TEST_REGISTRY_SERVER_2='ghcr.io'
export PSO_TEST_REGISTRY_USERNAME_2='your-second-user'
export PSO_TEST_REGISTRY_PASSWORD_2='your-second-password-or-token'
export PSO_TEST_REGISTRY_EMAIL_2='ops@example.com'
export PSO_TEST_REGISTRY_SERVER_3='registry.gitlab.com'
export PSO_TEST_REGISTRY_USERNAME_3='your-third-user'
export PSO_TEST_REGISTRY_PASSWORD_3='your-third-password-or-token'
export PSO_TEST_REGISTRY_EMAIL_3='ops@example.com'
export PSO_IMAGE='ghcr.io/mwognicki/pull-secrets-operator:v0.1.0-beta.1'
export PSO_OPERATOR_NAMESPACE='pull-secrets'
export PSO_WAIT_TIMEOUT='180s'
export PSO_TEST_ID='manualrun01'
export PSO_SMOKE_USE_CACHE='true'
export PSO_SMOKE_FORCE_RERUN='false'
```

### Local Passed-Scenario Cache

The script can reuse previously passed scenario results for the same local input hash.

Cache behavior:

- only passed scenarios are cached
- failed scenarios are never cached
- a minimal environment health check still runs every time
- if every scenario already has a cached pass for the current input hash, the script exits early without reinstalling the operator or rerunning cluster assertions

The cache key currently includes:

- the current contents of the relevant API, controller, sync, manifest, and smoke-script files
- the selected `PSO_IMAGE`

The cache intentionally does not include kubeconfig or other environment-specific secrets.

The local cache directory is:

```text
.smoke-cache/real-cluster
```

Useful controls:

- `PSO_SMOKE_USE_CACHE=true` enables pass-cache reuse
- `PSO_SMOKE_USE_CACHE=false` disables cache usage
- `PSO_SMOKE_FORCE_RERUN=true` ignores cached passes and reruns every scenario

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

The repository now also includes a manual GitHub workflow at [.github/workflows/real-cluster-smoke.yaml](/Users/marek/Work/Ognicki/pull-secrets-operator/.github/workflows/real-cluster-smoke.yaml).

That workflow currently runs on `workflow_dispatch` and expects these GitHub Secrets:

- `TS_OAUTH_CLIENT_ID`
- `TS_OAUTH_SECRET`
- `TS_TAGS`
- `KUBECONFIG_CONTENT`
- `PSO_TEST_REGISTRY_SERVER`
- `PSO_TEST_REGISTRY_USERNAME`
- `PSO_TEST_REGISTRY_PASSWORD`
- `PSO_TEST_REGISTRY_EMAIL`
- `PSO_TEST_REGISTRY_SERVER_2`
- `PSO_TEST_REGISTRY_USERNAME_2`
- `PSO_TEST_REGISTRY_PASSWORD_2`
- `PSO_TEST_REGISTRY_EMAIL_2`
- `PSO_TEST_REGISTRY_SERVER_3`
- `PSO_TEST_REGISTRY_USERNAME_3`
- `PSO_TEST_REGISTRY_PASSWORD_3`
- `PSO_TEST_REGISTRY_EMAIL_3`

It provides:
- cluster connectivity through Tailscale
- kubeconfig material from GitHub Secrets
- test registry credentials from GitHub Secrets
- a selectable image tag input, currently defaulting to `v0.1.0-beta.1`
- an ephemeral Tailscale node for the workflow run through `tailscale/github-action@v4`
- restore/save transport for `.smoke-cache/real-cluster`, with actual pass reuse still decided by the smoke script's own input-hash logic

It does not yet run automatically for pull requests.

## Manual GitHub Workflow

The `Real Cluster Smoke Tests` workflow is the first CI wrapper around the same smoke harness used locally.

Its execution flow is:

1. check out the repository
2. install `kubectl`
3. connect the runner to the Tailnet through Tailscale
4. write kubeconfig from a GitHub Secret
5. wait for Kubernetes API reachability
6. run [hack/real-cluster-smoke.sh](/Users/marek/Work/Ognicki/pull-secrets-operator/hack/real-cluster-smoke.sh)

### Required Secrets

| Secret | Purpose |
| --- | --- |
| `TS_OAUTH_CLIENT_ID` | Tailscale OAuth client ID for the GitHub Action. |
| `TS_OAUTH_SECRET` | Tailscale OAuth client secret for the GitHub Action. |
| `TS_TAGS` | Tailscale tags applied to the ephemeral workflow node. |
| `KUBECONFIG_CONTENT` | Kubeconfig content used by `kubectl` during the smoke run. |
| `PSO_TEST_REGISTRY_SERVER` | Registry server used by the smoke test credentials Secret. |
| `PSO_TEST_REGISTRY_USERNAME` | Registry username used by the smoke test credentials Secret. |
| `PSO_TEST_REGISTRY_PASSWORD` | Registry password or token used by the smoke test credentials Secret. |
| `PSO_TEST_REGISTRY_EMAIL` | Optional registry email used by the smoke test credentials Secret. |
| `PSO_TEST_REGISTRY_SERVER_2` | Second registry server used by the multi-registry smoke scenario. |
| `PSO_TEST_REGISTRY_USERNAME_2` | Second registry username used by the multi-registry smoke scenario. |
| `PSO_TEST_REGISTRY_PASSWORD_2` | Second registry password or token used by the multi-registry smoke scenario. |
| `PSO_TEST_REGISTRY_EMAIL_2` | Optional second registry email used by the multi-registry smoke scenario. |
| `PSO_TEST_REGISTRY_SERVER_3` | Third registry server used by the multi-registry smoke scenario. |
| `PSO_TEST_REGISTRY_USERNAME_3` | Third registry username used by the multi-registry smoke scenario. |
| `PSO_TEST_REGISTRY_PASSWORD_3` | Third registry password or token used by the multi-registry smoke scenario. |
| `PSO_TEST_REGISTRY_EMAIL_3` | Optional third registry email used by the multi-registry smoke scenario. |

### Manual Trigger

Run the workflow from the GitHub Actions UI and provide the `image_tag` input.

Example:

```text
v0.1.0-beta.1
```

The workflow then tests:

```text
ghcr.io/mwognicki/pull-secrets-operator:<image_tag>
```

### Current Limitations

- it is manual-only for now
- it assumes the selected image tag already exists in GHCR
- it does not yet build the PR image inside the same workflow
- it does not yet report results back to a pull request as a constraint check
