# CRD Configuration

This directory contains hand-written installation manifests centered around the operator CRDs.

Current manifests:
- `crds.yaml` as a one-shot install bundle for CRDs, RBAC, namespace, and manager deployment
- `pullsecrets.ognicki.ooo_pullsecretpolicies.yaml`
- `pullsecrets.ognicki.ooo_registrypullsecrets.yaml`

Both custom resources are currently cluster-scoped.
