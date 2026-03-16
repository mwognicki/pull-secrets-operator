# RBAC Configuration

This directory contains hand-written RBAC manifests required by the operator.

Current scope includes permissions for:
- watching custom resources
- listing namespaces
- reconciling replicated secrets across namespaces
- using `leases.coordination.k8s.io` for leader election
