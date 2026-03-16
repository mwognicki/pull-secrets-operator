# Real Cluster Tested Scope

This document summarizes what has already been exercised on a real Kubernetes cluster and what still remains unverified there.

## Real Kubernetes Coverage Matrix

The current smoke flow in [hack/real-cluster-smoke.sh](/Users/marek/Work/Ognicki/pull-secrets-operator/hack/real-cluster-smoke.sh) covers part of the operator behavior on a real Kubernetes cluster. The table below tracks both verified and still-unverified scope in one place.

| Scope | Tested On Real Cluster | Test Result | Status Description |
| --- | --- | --- | --- |
| Operator installation from the hand-written CRD, RBAC, and manager manifests | Yes | Passed | Verified by the current real-cluster smoke flow. |
| Manager rollout using `ghcr.io/mwognicki/pull-secrets-operator:v0.1.0-beta.1` | Yes | Passed | Verified by the current real-cluster smoke flow. |
| Successful reconciliation for a valid `RegistryPullSecret` | Yes | Passed | Verified by the current real-cluster smoke flow. |
| Secret replication into an explicitly included namespace | Yes | Passed | Verified by the current real-cluster smoke flow. |
| Per-namespace target secret name override | Yes | Passed | Verified by the current real-cluster smoke flow. |
| Cluster-wide exclusion taking precedence over a `RegistryPullSecret` | Yes | Passed | Verified by the current real-cluster smoke flow. |
| Validation failure when a `RegistryPullSecret` explicitly targets a namespace excluded by `PullSecretPolicy` | Yes | Passed | Verified by the current real-cluster smoke flow. |
| Status behavior for that validation failure, including `Ready=False` with reason `ValidationFailed` | Yes | Passed | Verified by the current real-cluster smoke flow. |
| End-to-end smoke-test cleanup, including operator installation teardown and removal of throwaway namespaces and temporary resources | Yes | Passed | Verified by the current real-cluster smoke flow. |
| `Exclusive` namespace policy behavior as a primary scenario | Yes | Passed | Verified by the current real-cluster smoke flow. |
| Duplicate namespace validation in explicit namespace lists | Yes | Passed | Verified by the current real-cluster smoke flow. |
| Duplicate namespace validation in namespace overrides | Yes | Passed | Verified by the current real-cluster smoke flow. |
| Wildcard namespace rejection | Yes | Passed | Verified by the current real-cluster smoke flow. |
| Invalid Kubernetes namespace-name rejection | No | Not run | No real-cluster scenario exists for this behavior yet. |
| Invalid or too-short resulting pull secret name rejection | Yes | Passed | Verified by the current real-cluster smoke flow. |
| Collisions with existing foreign or unmanaged `Secret` resources | Yes | Passed | Verified by the current real-cluster smoke flow. |
| Inline credential mode | Yes | Passed | Verified by the current real-cluster smoke flow. |
| Additional secret-backed credential variations beyond the current happy path | No | Not run | No real-cluster scenario exists for this behavior yet. |
| Prompt reconciliation after updating an existing `RegistryPullSecret` | Yes | Passed | Verified by the current real-cluster smoke flow. |
| Removal of obsolete managed secrets after changing namespace selection or target secret names | Yes | Passed | Verified by the current real-cluster smoke flow. |
| Non-destructive behavior when deleting a `RegistryPullSecret` | Yes | Passed | Verified by the current real-cluster smoke flow. |
| Behavior when managed replica `Secret` objects are manually modified | Yes | Passed | Verified by the current real-cluster smoke flow. |
| Behavior when managed replica `Secret` objects are manually deleted | Yes | Passed | Verified by the current real-cluster smoke flow. |
| `PullSecretPolicy` validity edge cases, including duplicate excluded namespaces | No | Not run | No real-cluster scenario exists for this behavior yet. |
| `PullSecretPolicy` validity edge cases, including invalid excluded namespace names | No | Not run | No real-cluster scenario exists for this behavior yet. |
| Multi-registry coverage using additional registry providers | No | Not run | No real-cluster scenario exists for this behavior yet. |
| GitHub Actions based real-cluster execution with Tailscale and kubeconfig setup | No | Not run | No CI-backed real-cluster execution exists yet. |
