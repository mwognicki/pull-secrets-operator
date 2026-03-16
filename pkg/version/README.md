# Version Metadata

This package centralizes version-related metadata used by the operator binary.

Current policy:
- Kubernetes API versioning is independent from operator release versioning.
- The current API surface is `pullsecrets.ognicki.ooo/v1alpha1`.
- The operator release version follows SemVer-style identifiers.
- Build metadata should embed operator version, git commit, and build date.
- `pkg/version.Version` is intentionally a generic development fallback (`dev`) in source control.
- Release and CI image builds should inject the effective version at build time through linker flags.
- Tag-based image releases are the source of truth for shipped operator versions, for example `v0.1.0-beta.2`.
