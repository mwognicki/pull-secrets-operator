# Manager Configuration

This directory contains hand-written Kubernetes resources required to run the operator manager.

Current manifest:
- `manager.yaml` with the operator namespace and a single manager Deployment
- the default manager image is `ghcr.io/mwognicki/pull-secrets-operator:latest`

The corresponding container build is defined in the repository root `Dockerfile`.
