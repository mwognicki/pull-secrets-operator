# pull-secrets-operator

[![codecov](https://codecov.io/github/mwognicki/pull-secrets-operator/graph/badge.svg?token=57TCBX4OK3)](https://codecov.io/github/mwognicki/pull-secrets-operator)
[![Go Report Card](https://goreportcard.com/badge/github.com/mwognicki/pull-secrets-operator)](https://goreportcard.com/report/github.com/mwognicki/pull-secrets-operator)
![GitHub License](https://img.shields.io/github/license/mwognicki/pull-secrets-operator)
![GitHub last commit (branch)](https://img.shields.io/github/last-commit/mwognicki/pull-secrets-operator/main)
![GitHub Tag](https://img.shields.io/github/v/tag/mwognicki/pull-secrets-operator)
[![Discord](https://img.shields.io/discord/1483122428132589584)](https://discord.com/channels/1483122428132589584)

Kubernetes operator for replicating Docker pull secrets across namespaces.

## Repository Layout

This repository currently contains project scaffolding only. The layout is prepared for:
- a per-registry pull secret API
- a cluster-wide namespace policy API
- controller and replication logic kept separate from API definitions
- the `pullsecrets.ognicki.ooo/v1alpha1` API group
- hand-written Kubernetes manifests under `config/`

```text
.
|-- api/
|   `-- pullsecrets/v1alpha1/
|-- cmd/
|   `-- manager/
|-- config/
|   |-- crd/
|   |-- manager/
|   |-- rbac/
|   `-- samples/
|-- hack/
|-- internal/
|   |-- controller/
|   `-- sync/
`-- pkg/
    `-- metadata/
```

See the README files inside those directories for the intended responsibilities.

## Current API Direction

- API group: `pullsecrets.ognicki.ooo`
- Version: `v1alpha1`
- Cluster-wide resource: `PullSecretPolicy`, conventionally named `cluster`
- Per-registry resource: `RegistryPullSecret`
- Per-registry credentials can come either from inline spec fields or from a referenced Kubernetes `Secret`
- Per-registry default target secret name is optional and should be derived from the registry server when omitted
- Explicit `RegistryPullSecret` changes should be reconciled promptly
- Cluster-wide exclusions override per-registry rules and do not retroactively delete or backfill secrets
- Deleting a `RegistryPullSecret` is intentionally non-destructive for now and leaves already replicated Secrets in place
- Explicit namespace entries and namespace overrides must use valid Kubernetes namespace names, may not be duplicated within their lists, and wildcard namespace patterns are not supported
- Resulting pull secret names must be Kubernetes-compatible and contain at least 3 alphanumeric characters
- Explicitly selected namespaces may not conflict with cluster-wide exclusions, and target names may not collide with existing foreign Secrets
- `RegistryPullSecret.status` reports reconciliation results and secret counts
- invalid reconciliation or policy situations are reflected through concise status conditions and messages
- `PullSecretPolicy.status` reports singleton activity, operator-validity, and excluded namespace counts

## Current Manifests

- Hand-written CRDs are in `config/crd`
- Hand-written RBAC is in `config/rbac`
- Hand-written manager deployment resources are in `config/manager`
- Hand-written sample custom resources are in `config/samples`

## Container Build

- The manager image is built from the hand-written [Dockerfile](./Dockerfile)
- The runtime base image is `almalinux/10-kitten-micro`

## Versioning Policy

- Kubernetes API versioning follows explicit API stability, starting with `pullsecrets.ognicki.ooo/v1alpha1`
- Operator binary and image versioning follows SemVer-style tags, with the current development baseline set to `0.1.0-dev`
- Development images should prefer branch or commit-specific tags over mutable release-like tags
- Stable releases should use immutable SemVer tags such as `v0.1.0`
- The manager binary embeds three build fields: operator version, git commit, and build date
- The manager supports `--version` to print the embedded build metadata

## Remaining Work

- The first local real-cluster smoke test flow is documented in [docs/real-cluster-testing.md](./docs/real-cluster-testing.md)
