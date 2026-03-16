---
title: Status and conditions
description: Reference for all status fields and conditions reported by pull-secrets-operator.
sidebar:
  order: 3
---

Both `RegistryPullSecret` and `PullSecretPolicy` report reconciliation results and validity through standard Kubernetes status conditions.

## RegistryPullSecret status

### Status fields

| Field | Type | Description |
|---|---|---|
| `observedGeneration` | integer | The resource generation that produced this status update |
| `desiredSecretCount` | integer | Number of namespaces the operator computed as targets in the last reconciliation |
| `appliedSecretCount` | integer | Number of secrets created or updated in the last reconciliation |
| `deletedSecretCount` | integer | Number of obsolete managed secrets deleted in the last reconciliation |
| `lastSyncTime` | timestamp | RFC3339 timestamp of the last reconciliation attempt |
| `conditions` | Condition[] | Standard Kubernetes conditions. See below |

### Conditions

#### `Ready`

Reports whether the last reconciliation completed successfully.

| `status` | `reason` | Meaning |
|---|---|---|
| `True` | `Synced` | Reconciliation completed. All desired secrets are in place |
| `False` | `SyncFailed` | A runtime error occurred during reconciliation. The message field contains details |
| `False` | `ValidationFailed` | The resource failed semantic validation. The message field describes the violation |

### Example — healthy

```yaml
status:
  observedGeneration: 3
  desiredSecretCount: 7
  appliedSecretCount: 2
  deletedSecretCount: 0
  lastSyncTime: "2025-03-01T10:00:00Z"
  conditions:
    - type: Ready
      status: "True"
      reason: Synced
      message: RegistryPullSecret reconciled successfully
      lastTransitionTime: "2025-03-01T10:00:00Z"
      observedGeneration: 3
```

### Example — validation failure

```yaml
status:
  observedGeneration: 4
  desiredSecretCount: 0
  appliedSecretCount: 0
  deletedSecretCount: 0
  lastSyncTime: "2025-03-01T10:05:00Z"
  conditions:
    - type: Ready
      status: "False"
      reason: ValidationFailed
      message: "namespace \"staging\" is explicitly included but also excluded by PullSecretPolicy"
      lastTransitionTime: "2025-03-01T10:05:00Z"
      observedGeneration: 4
```

---

## PullSecretPolicy status

### Status fields

| Field | Type | Description |
|---|---|---|
| `observedGeneration` | integer | The resource generation that produced this status update |
| `excludedNamespaceCount` | integer | Number of namespaces currently in `spec.excludedNamespaces` |
| `activeSingleton` | boolean | `true` when this object is named `cluster` and recognized as the active policy |
| `valid` | boolean | `true` when the policy passes all validation rules |
| `lastSyncTime` | timestamp | RFC3339 timestamp of the last reconciliation of this object |
| `conditions` | Condition[] | Standard Kubernetes conditions. See below |

### Conditions

#### `Valid`

Reports whether the `PullSecretPolicy` passes semantic validation.

| `status` | `reason` | Meaning |
|---|---|---|
| `True` | `Valid` | The policy is valid |
| `False` | `ValidationFailed` | A validation rule was violated. The message field describes the violation |

#### `Ready`

Reports overall readiness of the policy object.

| `status` | `reason` | Meaning |
|---|---|---|
| `True` | `Ready` | The policy is active and valid |
| `False` | `NotSingleton` | This object is not named `cluster` and is therefore not recognized as the active policy |
| `False` | `ValidationFailed` | The policy failed validation |

### Example — healthy

```yaml
status:
  observedGeneration: 1
  excludedNamespaceCount: 4
  activeSingleton: true
  valid: true
  lastSyncTime: "2025-03-01T09:00:00Z"
  conditions:
    - type: Valid
      status: "True"
      reason: Valid
      message: PullSecretPolicy is valid
    - type: Ready
      status: "True"
      reason: Ready
      message: PullSecretPolicy is active
```

---

## Checking status with kubectl

```bash
# Check RegistryPullSecret conditions
kubectl get registrypullsecret ghcr -o jsonpath='{.status.conditions}' | jq

# Watch for Ready=True
kubectl wait registrypullsecret/ghcr --for=condition=Ready --timeout=60s

# Check PullSecretPolicy
kubectl get pullsecretpolicy cluster -o yaml
```
