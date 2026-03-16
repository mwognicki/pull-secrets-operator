# Controllers

This directory is reserved for Kubernetes reconcilers.

Controllers here should focus on:
- reconciling per-registry pull secret resources
- reconciling cluster-wide namespace policy resources
- coordinating replication behavior based on both resource types

Behavior notes:
- `RegistryPullSecret` updates should trigger prompt reconciliation so explicit spec changes are synced as soon as possible.
- `PullSecretPolicy` updates should affect future decisions, but should not trigger retroactive cleanup or backfill by themselves.
- `RegistryPullSecret` reconciliation is implemented as the first controller pass and delegates selection/rendering decisions to `internal/sync`.
- `RegistryPullSecret` reconciliation now creates, updates, and deletes Secrets that are managed by the same source resource but no longer desired.
- Deleting a `RegistryPullSecret` is intentionally non-destructive for now; existing managed replica Secrets are left in place and no finalizer-based cleanup is used.
- Drift in managed replica Secrets is intentionally not watched directly; those Secrets are resynchronized only on a later `RegistryPullSecret` reconcile, including after operator restart.
- The controller also performs defensive validation for invalid-but-admitted objects, including namespace duplication, invalid namespace names, wildcard namespace patterns, excluded explicit namespaces, invalid target Secret names, and collisions with foreign Secrets.
- `PullSecretPolicy` is still not treated as a standalone retroactive cleanup trigger by itself.
- `RegistryPullSecret.status` is updated on successful and failed reconciliations.
- `PullSecretPolicy.status` is updated by a dedicated reconciler and reflects whether a given object is the active singleton policy.
