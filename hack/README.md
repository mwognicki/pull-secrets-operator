# Hack Scripts

This directory is reserved for helper scripts used during development, verification, and release workflows.

Current helper scripts:
- `real-cluster-smoke.sh` runs the first local smoke test against a real Kubernetes cluster using unique throwaway namespaces.
- `lib/load-dotenv.sh` provides shared automatic loading of repository-root `.env` files for helper scripts.
