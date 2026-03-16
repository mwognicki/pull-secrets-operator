# Non-CI TODOs

This document collects the remaining non-CI work for the project after the current scaffolding, reconciliation, status reporting, manifests, and constraint checks.

## 1. Validation Hardening

The API currently has baseline schema and controller checks, but validation is still fairly light.

Remaining work:
- tighten CRD validation for names, required combinations, and field formats
- validate namespace override consistency
- validate credential inputs more strictly where appropriate
- add controller-side defensive validation for invalid but admitted objects

Why it matters:
- stronger validation reduces ambiguous reconciliation behavior
- it improves operator ergonomics before the API evolves further

## 2. Status Enrichment

Both current resources now have status reporting, but the status surface is still intentionally minimal.

Remaining work:
- decide whether to expose more detailed reconciliation counters or summaries
- decide whether to record status for skipped namespaces or invalid policy situations
- decide whether status should include user-oriented messages about effective targeting

Why it matters:
- richer status would improve operability and debugging
- the current status model is useful, but still a first pass

## 3. Deployment Usability

The project is installable from hand-written manifests, but deployment ergonomics can still be improved.

Remaining work:
- replace or parameterize the hardcoded `:latest` deployment tag strategy
- define how image versions should be consumed in manifests
- decide whether environment-specific overlays or installation variants are needed

Why it matters:
- it closes the gap between development scaffolding and practical cluster installation
- it supports the versioning policy already defined in the repository

## 4. Installation and Runtime Documentation

The repository documents structure and API intent well, but user-facing operational docs are still missing.

Remaining work:
- write an installation guide for CRDs, RBAC, manager deployment, and sample resources
- document expected reconciliation behavior and cleanup behavior
- document how versioned images and manifests should be consumed
- add local or cluster smoke-test instructions

Why it matters:
- it makes the repository usable by others without reverse-engineering the layout
- it turns the current scaffold into something operationally understandable
