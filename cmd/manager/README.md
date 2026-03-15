# Manager Command

This directory is reserved for the operator entrypoint.

The manager binary wires together:
- scheme registration
- `RegistryPullSecret` controller setup
- health and readiness endpoints
- leader election and manager options
