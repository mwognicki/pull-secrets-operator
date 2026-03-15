# pull-secrets-operator

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
- Per-registry default target secret name is optional and should be derived from the registry server when omitted
- Explicit `RegistryPullSecret` changes should be reconciled promptly
- Cluster-wide exclusions override per-registry rules and do not retroactively delete or backfill secrets

## Current Manifests

- Hand-written CRDs are in `config/crd`
- Hand-written RBAC is in `config/rbac`
- Hand-written manager deployment resources are in `config/manager`
- Hand-written sample custom resources are in `config/samples`

## Container Build

- The manager image is built from the hand-written [Dockerfile](/Users/marek/Work/Ognicki/pull-secrets-operator/Dockerfile)
- The runtime base image is `almalinux/10-kitten-micro`
