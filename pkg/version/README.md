# Version Metadata

This package centralizes version-related metadata used by the operator binary.

Current policy:
- Kubernetes API versioning is independent from operator release versioning.
- The current API surface is `pullsecrets.ognicki.ooo/v1alpha1`.
- The operator release version follows SemVer-style identifiers.
- Build metadata should embed operator version, git commit, and build date.
