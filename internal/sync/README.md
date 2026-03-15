# Sync Logic

This directory is reserved for internal replication and policy evaluation logic.

Keeping this logic separate from reconcilers should make it easier to test namespace selection, exclusion handling, and overwrite rules.

Current responsibilities in this package:
- resolve whether a namespace is eligible for replication
- apply `PullSecretPolicy` exclusions before per-registry targeting
- derive a default target secret name from the registry server when none is specified
- resolve per-namespace target secret name overrides
- render desired `kubernetes.io/dockerconfigjson` Secret objects
- compare existing and desired Secrets to decide whether create or update is needed
