# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

`pull-secrets-operator` is a Kubernetes operator (API group `pullsecrets.ognicki.ooo/v1alpha1`) that replicates Docker pull secrets across namespaces. It manages two cluster-scoped custom resources:

- **`RegistryPullSecret`** – per-registry source of truth; drives secret replication
- **`PullSecretPolicy`** – singleton (name: `cluster`) for cluster-wide namespace exclusions

## Commands

```bash
# Run all tests with coverage
go test -coverprofile=coverage.txt ./...

# Run tests for a single package
go test ./internal/sync/...

# Run a single test by name
go test ./internal/sync/... -run TestValidateRegistryPullSecret

# Build the manager binary
go build ./cmd/manager/

# Build the container image (multi-stage, requires Docker BuildKit)
docker build \
  --build-arg VERSION=0.1.0-dev \
  --build-arg GIT_COMMIT=$(git rev-parse --short HEAD) \
  --build-arg BUILD_DATE=$(date -u +%Y-%m-%dT%H:%M:%SZ) \
  -t pull-secrets-operator:dev .

# Run the real-cluster smoke tests (requires kubeconfig and test registry creds)
hack/real-cluster-smoke.sh
```

## Architecture

### Package Responsibilities

**`internal/sync/`** – Pure business logic, no Kubernetes API calls. All functions are exported and tested independently without a running cluster.

- `credentials.go` – resolves credentials from either inline `spec.credentials` or a referenced `Secret` (exactly one source required)
- `planner.go` – computes namespace targets after applying cluster-wide exclusions and per-registry inclusive/exclusive policy; derives the default secret name from the registry hostname by stripping scheme, port, TLD, and ignored tokens (`docker`, `registry`, `www`)
- `secret.go` – renders `kubernetes.io/dockerconfigjson` Secrets, compares with existing to set `NeedsApply`, identifies obsolete managed replicas for deletion
- `validation.go` – semantic validation beyond CRD admission (no duplicate namespaces, no wildcard names, no collision with cluster-wide exclusions or foreign Secrets, secret name ≥ 3 alphanumeric chars); wraps errors so controllers can distinguish `IsValidationError`

**`internal/controller/`** – Reconcilers only; orchestrate sync functions and make API calls.

- `RegistryPullSecretReconciler` – main loop: fetches policy + namespaces + existing secrets → calls `sync.DesiredSecrets` + `sync.ObsoleteSecrets` → applies/deletes → updates status with counts and `Ready` condition (`Synced` / `SyncFailed` / `ValidationFailed`)
- `PullSecretPolicyReconciler` – status-only; validates policy and sets `Valid`/`Ready` conditions without triggering cascading reconciliations

**`api/pullsecrets/v1alpha1/`** – Type definitions and DeepCopy helpers. The `PullSecretPolicySingletonName` constant (`"cluster"`) is the expected name for the singleton resource.

**`pkg/metadata/`** – Label constants used to identify managed Secrets:
- `app.kubernetes.io/managed-by: pull-secrets-operator`
- `pullsecrets.ognicki.ooo/registry-pull-secret: <source-name>`
- `pullsecrets.ognicki.ooo/registry-server: <server>`

**`pkg/version/`** – Embeds `VERSION`, `GIT_COMMIT`, `BUILD_DATE` at build time via ldflags.

### Key Behavioral Rules

- **Non-destructive deletion:** Deleting a `RegistryPullSecret` leaves replicated Secrets in place. No finalizers are used.
- **Cluster-wide exclusions always win:** Even explicitly selected namespaces are skipped if in `PullSecretPolicy.spec.excludedNamespaces`.
- **Drift tolerance:** Manual edits to managed replica Secrets are only corrected on the next `RegistryPullSecret` reconciliation. Managed replicas are not watched directly.
- **Credential Secret watch:** Only source credential Secrets (via `credentialsSecretRef`) trigger reconciliation, not replicated Secrets.
- **`PullSecretPolicy` is optional:** If absent, the reconciler proceeds with an empty policy (no exclusions).

### Secret Naming

`DefaultTargetSecretName` in `planner.go` derives a name from the registry hostname:
1. Normalizes (adds `https://` if no scheme, lowercases)
2. Strips TLD and common secondary domains (`com`, `net`, `co`, etc.)
3. Strips ignored tokens (`docker`, `registry`, `www`)
4. Picks the last remaining label
5. Appends `-pull-secret` suffix

Override with `spec.namespaces.targetSecretName` (global) or `spec.namespaces.namespaceOverrides[].secretName` (per-namespace).

## Config / Deployment

- `config/crd/` – hand-written CRDs (do not use controller-gen)
- `config/rbac/` – hand-written ClusterRole + ClusterRoleBinding
- `config/manager/` – hand-written Deployment
- `config/samples/` – example custom resources
- `helm/pull-secrets-operator/` – Helm chart (v0.1.0), installs CRDs + RBAC + Deployment

Default operator namespace: `pull-secrets`. Default image: `ghcr.io/mwognicki/pull-secrets-operator:latest`.

## CI

- **`pr-constraint-checks.yaml`** – Go tests + Codecov, SonarQube gate, Docker build + Trivy CVE scan (CRITICAL/HIGH), PR comment summary
- **`real-cluster-smoke.yaml`** – manual `workflow_dispatch`; connects via Tailscale, runs `hack/real-cluster-smoke.sh` against a real cluster; results cached in `.smoke-cache/real-cluster`
- **`release-image-tag.yaml`** – builds and pushes image on Git tag push

Smoke test environment variables: `PSO_TEST_REGISTRY_*`, `PSO_IMAGE`, `PSO_OPERATOR_NAMESPACE`, `PSO_WAIT_TIMEOUT`.
