# Sync Logic

This directory contains the operator's internal planning, rendering, credential resolution, and validation logic.

Keeping this logic separate from reconcilers makes it easier to test the operator behavior deterministically without a running controller manager.

## Current Responsibilities

- resolve effective registry credentials from either:
  - inline `spec.credentials`, or
  - `spec.credentialsSecretRef`
- validate `RegistryPullSecret` and `PullSecretPolicy` semantics beyond basic CRD admission
- determine whether a namespace is eligible after applying:
  - cluster-wide exclusions from `PullSecretPolicy`
  - per-resource `Inclusive` or `Exclusive` namespace selection
- derive a default pull-secret name from the registry server when no explicit name is provided
- apply per-namespace target secret name overrides
- build the effective namespace-to-secret target plan
- render desired `kubernetes.io/dockerconfigjson` `Secret` objects
- compare existing and desired `Secret`s to decide whether create or update is needed
- determine which existing managed replica `Secret`s have become obsolete and should be deleted

## Current Modules

- `credentials.go`
  Resolves effective credentials from inline fields or a referenced Kubernetes `Secret`.
- `planner.go`
  Computes effective namespace targets and derives default pull-secret names from registry hosts.
- `validation.go`
  Enforces semantic validation rules for both custom resources.
- `secret.go`
  Renders docker config payloads, builds desired `Secret` objects, compares them with existing ones, and identifies obsolete managed replicas.

## Current Validation Rules

For `RegistryPullSecret`:
- exactly one of inline credentials or `credentialsSecretRef` must be used
- namespace selection policy must be valid
- wildcard namespace patterns are not supported
- namespace entries may not be duplicated
- namespace override entries may not be duplicated
- explicit namespace names must be valid Kubernetes namespace names
- explicit or derived target secret names must be valid Kubernetes `Secret` names
- resulting secret names must contain at least 3 alphanumeric characters
- explicitly selected namespaces may not conflict with cluster-wide exclusions
- target names may not collide with foreign or differently owned Secrets

For `PullSecretPolicy`:
- excluded namespace names must be valid Kubernetes namespace names
- excluded namespace entries may not be duplicated

Validation errors are wrapped so the controller layer can distinguish semantic validation failures from runtime reconciliation errors.

## Current Planning Behavior

- Cluster-wide exclusions always win over per-resource namespace targeting.
- `Inclusive` means only listed namespaces are eligible.
- `Exclusive` means all namespaces except the listed ones are eligible.
- `targetSecretName` overrides the derived default for all selected namespaces.
- `namespaceOverrides` may override that target name for specific namespaces.
- Default target names are derived from the registry host after removing:
  - scheme
  - port
  - top-level domain
  - common secondary domain tokens such as `co`, `com`, `gov`, `net`, `org`, and `edu`
  - generic host tokens such as `docker`, `registry`, and `www`
- Non-public hostnames such as `localhost` or `some-other-server:2929` are kept as the base name after port stripping.

## Current Secret Rendering Behavior

- Replicated secrets are always rendered as `kubernetes.io/dockerconfigjson`.
- The canonical `.dockerconfigjson` payload is built from the effective credentials.
- Managed secrets are labeled so the controller can identify ownership and obsolete replicas later.

## Current Intentional Limits

- This package does not perform live registry reachability or credential verification.
- Drift in already replicated managed Secrets is not handled here proactively; the package only computes desired state when reconciliation is triggered.
