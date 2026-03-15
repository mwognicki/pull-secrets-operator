# Repository Guidance

## Project Context
- This repository is a Go project for a Kubernetes operator that replicates Docker pull secrets across namespaces.
- The project target Go version is `1.26.0` (released `2026-02-10`).
- The preferred Kubernetes API group is `pullsecrets.ognicki.ooo`.

## Planned Custom Resources
- One CRD should represent registry pull secret credentials and namespace targeting for a single registry.
- The per-registry resource should support namespace policy modes: inclusive or exclusive.
- The per-registry resource should support per-namespace target secret name overrides.
- A second CRD should define cluster-wide namespace exclusions and always take precedence over per-registry resources.
- Cluster-wide namespace exclusion changes should not retroactively delete existing replicated secrets.
- Removing a namespace from cluster-wide exclusion should not automatically backfill secrets into that namespace.

## Workflow Rules
- Do not generate code, manifests, or configuration that was not explicitly requested or clearly approved by the user.
- Prefer safe scaffolding over speculative implementation.
- Keep changes minimal, practical, and aligned with the current request.
- Do not provide proactive recommendations about next steps unless the user explicitly asks for them.
- Do not try to anticipate the next task, problem, or challenge unless the user specifically asks for that kind of guidance.
- Ask for confirmation or explanation when needed, but avoid over-investigating trivial details.

## Container Images
- For any Docker-based images, use `almalinux/10-kitten-micro` as the desired base image.

## Versioning Policy
- The Kubernetes API should evolve explicitly through versioned packages such as `v1alpha1`, with compatibility decisions reflected in CRD and Go type changes.
- The operator binary and container image should use SemVer-style version identifiers.
- Development builds may use pre-release or branch-oriented tags, but stable release tags should be immutable.
- Build artifacts should embed version, git commit, and build date metadata when possible.
