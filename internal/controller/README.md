# Controllers

This directory contains the Kubernetes reconcilers for the operator.

## Active Reconcilers

- `RegistryPullSecretReconciler`
  Reconciles per-registry pull-secret intent into replicated `Secret` objects across namespaces.
- `PullSecretPolicyReconciler`
  Reconciles cluster-wide policy status for the singleton-like `PullSecretPolicy`.

## `RegistryPullSecretReconciler`

Current behavior:
- Watches `RegistryPullSecret` resources directly.
- Also watches source credential `Secret` objects referenced through `credentialsSecretRef`.
- Delegates namespace selection, derived secret naming, rendering, and validation to `internal/sync`.
- Creates missing managed replica `Secret`s.
- Updates existing managed replica `Secret`s when the desired content changes.
- Deletes obsolete managed replica `Secret`s when a `RegistryPullSecret` changes target names or namespace selection.

Current lifecycle rules:
- Explicit `RegistryPullSecret` changes should reconcile promptly.
- Deleting a `RegistryPullSecret` is intentionally non-destructive for now.
- No finalizer-based cleanup is used on source resource deletion.
- Drift in already replicated managed `Secret`s is intentionally not watched directly.
- Manual modification or deletion of managed replica `Secret`s is only revisited on a later `RegistryPullSecret` reconciliation opportunity.

Current validation behavior:
- Namespace duplication is rejected.
- Invalid Kubernetes namespace names are rejected.
- Wildcard namespace patterns are rejected.
- Explicitly selected namespaces may not conflict with cluster-wide exclusions.
- Invalid or too-short target `Secret` names are rejected.
- Collisions with existing foreign `Secret`s are rejected.
- Validation failures are surfaced through `RegistryPullSecret.status.conditions`.

Current status behavior:
- `RegistryPullSecret.status` is updated after both successful and failed reconciliations.
- Validation failures use concise condition reasons such as `ValidationFailed`.
- Successful reconciliations report observed generation, desired/applied/deleted secret counts, last sync time, and `Ready=True`.

## `PullSecretPolicyReconciler`

Current behavior:
- Watches `PullSecretPolicy` resources directly.
- Reconciles status only.
- Treats the object named `cluster` as the conventional active singleton.

Current lifecycle rules:
- `PullSecretPolicy` updates affect future reconciliation decisions.
- `PullSecretPolicy` is not treated as a standalone retroactive cleanup or backfill trigger.

Current status behavior:
- Reports whether the object is the active singleton.
- Reports whether the object is valid from the operator perspective.
- Updates concise `Valid` and `Ready` conditions.
- Stores observed generation, excluded namespace count, singleton activity, validity, and last sync time.
