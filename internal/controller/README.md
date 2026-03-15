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
- Current scope is create/update of desired Secrets only; cleanup of no-longer-desired Secrets is still intentionally deferred.
