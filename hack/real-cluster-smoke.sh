#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TEST_ID="${PSO_TEST_ID:-$(date +%Y%m%d%H%M%S)}"
OPERATOR_NAMESPACE="${PSO_OPERATOR_NAMESPACE:-pull-secrets}"
OPERATOR_IMAGE="${PSO_IMAGE:-ghcr.io/mwognicki/pull-secrets-operator:dev-alpha1}"
REGISTRY_SERVER="${PSO_TEST_REGISTRY_SERVER:-}"
REGISTRY_USERNAME="${PSO_TEST_REGISTRY_USERNAME:-}"
REGISTRY_PASSWORD="${PSO_TEST_REGISTRY_PASSWORD:-}"
REGISTRY_EMAIL="${PSO_TEST_REGISTRY_EMAIL:-ops@example.com}"
WAIT_TIMEOUT="${PSO_WAIT_TIMEOUT:-180s}"

TEST_PREFIX="psop-e2e-${TEST_ID}"
INCLUDED_NAMESPACE="${TEST_PREFIX}-include"
OVERRIDE_NAMESPACE="${TEST_PREFIX}-override"
EXCLUDED_NAMESPACE="${TEST_PREFIX}-excluded"
INVALID_NAMESPACE="${TEST_PREFIX}-invalid"

EXPECTED_DEFAULT_SECRET_NAME=""
TEMP_DIR="$(mktemp -d)"

cleanup() {
  set +e
  kubectl delete registrypullsecret --ignore-not-found "${TEST_PREFIX}-valid" "${TEST_PREFIX}-invalid" >/dev/null 2>&1
  kubectl delete pullsecretpolicy --ignore-not-found cluster >/dev/null 2>&1
  kubectl -n "${OPERATOR_NAMESPACE}" delete secret --ignore-not-found "${TEST_PREFIX}-credentials" >/dev/null 2>&1
  kubectl delete namespace --ignore-not-found \
    "${INCLUDED_NAMESPACE}" \
    "${OVERRIDE_NAMESPACE}" \
    "${EXCLUDED_NAMESPACE}" \
    "${INVALID_NAMESPACE}" >/dev/null 2>&1
  rm -rf "${TEMP_DIR}"
}

require_command() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "missing required command: $1" >&2
    exit 1
  fi
}

require_env() {
  if [[ -z "$2" ]]; then
    echo "missing required environment variable: $1" >&2
    exit 1
  fi
}

assert_secret_exists() {
  local namespace="$1"
  local name="$2"

  kubectl -n "${namespace}" get secret "${name}" >/dev/null
}

assert_secret_missing() {
  local namespace="$1"
  local name="$2"

  if kubectl -n "${namespace}" get secret "${name}" >/dev/null 2>&1; then
    echo "secret ${namespace}/${name} exists, but it should not" >&2
    exit 1
  fi
}

assert_condition_reason() {
  local kind="$1"
  local name="$2"
  local type="$3"
  local expected_reason="$4"

  local actual_reason
  actual_reason="$(kubectl get "${kind}" "${name}" -o "jsonpath={.status.conditions[?(@.type=='${type}')].reason}")"
  if [[ "${actual_reason}" != "${expected_reason}" ]]; then
    echo "unexpected ${kind}/${name} condition reason for ${type}: expected ${expected_reason}, got ${actual_reason}" >&2
    exit 1
  fi
}

assert_ready_status() {
  local kind="$1"
  local name="$2"
  local expected_status="$3"

  local actual_status
  actual_status="$(kubectl get "${kind}" "${name}" -o "jsonpath={.status.conditions[?(@.type=='Ready')].status}")"
  if [[ "${actual_status}" != "${expected_status}" ]]; then
    echo "unexpected ${kind}/${name} Ready condition: expected ${expected_status}, got ${actual_status}" >&2
    exit 1
  fi
}

derive_secret_name() {
  local server="$1"
  local host="${server#http://}"
  host="${host#https://}"
  host="${host%%/*}"
  host="${host%%:*}"

  if [[ "${host}" != *.* ]]; then
    printf '%s-pull-secret\n' "${host}"
    return
  fi

  IFS='.' read -r -a parts <<< "${host}"
  unset 'parts[${#parts[@]}-1]'

  if [[ ${#parts[@]} -gt 0 ]]; then
    case "${parts[${#parts[@]}-1]}" in
      gov|net|com|co|org|edu)
        unset 'parts[${#parts[@]}-1]'
        ;;
    esac
  fi

  if [[ ${#parts[@]} -eq 0 ]]; then
    echo "failed to derive secret name from ${server}" >&2
    exit 1
  fi

  printf '%s-pull-secret\n' "${parts[${#parts[@]}-1]}"
}

install_operator() {
  kubectl apply -f "${ROOT_DIR}/config/crd/pullsecrets.ognicki.ooo_pullsecretpolicies.yaml"
  kubectl apply -f "${ROOT_DIR}/config/crd/pullsecrets.ognicki.ooo_registrypullsecrets.yaml"
  kubectl apply -f "${ROOT_DIR}/config/manager/manager.yaml"
  kubectl apply -f "${ROOT_DIR}/config/rbac/manager.yaml"
  kubectl -n "${OPERATOR_NAMESPACE}" set image deployment/pull-secrets-operator-manager manager="${OPERATOR_IMAGE}"
  kubectl -n "${OPERATOR_NAMESPACE}" rollout status deployment/pull-secrets-operator-manager --timeout="${WAIT_TIMEOUT}"
}

create_test_namespaces() {
  kubectl create namespace "${INCLUDED_NAMESPACE}"
  kubectl create namespace "${OVERRIDE_NAMESPACE}"
  kubectl create namespace "${EXCLUDED_NAMESPACE}"
  kubectl create namespace "${INVALID_NAMESPACE}"
}

write_manifests() {
  cat > "${TEMP_DIR}/policy.yaml" <<EOF
apiVersion: pullsecrets.ognicki.ooo/v1alpha1
kind: PullSecretPolicy
metadata:
  name: cluster
spec:
  excludedNamespaces:
    - ${EXCLUDED_NAMESPACE}
EOF

  cat > "${TEMP_DIR}/credentials-secret.yaml" <<EOF
apiVersion: v1
kind: Secret
metadata:
  name: ${TEST_PREFIX}-credentials
  namespace: ${OPERATOR_NAMESPACE}
type: Opaque
stringData:
  server: ${REGISTRY_SERVER}
  username: ${REGISTRY_USERNAME}
  password: ${REGISTRY_PASSWORD}
  email: ${REGISTRY_EMAIL}
EOF

  cat > "${TEMP_DIR}/valid-rps.yaml" <<EOF
apiVersion: pullsecrets.ognicki.ooo/v1alpha1
kind: RegistryPullSecret
metadata:
  name: ${TEST_PREFIX}-valid
spec:
  credentialsSecretRef:
    name: ${TEST_PREFIX}-credentials
    namespace: ${OPERATOR_NAMESPACE}
  namespaces:
    policy: Inclusive
    namespaces:
      - ${INCLUDED_NAMESPACE}
      - ${OVERRIDE_NAMESPACE}
    namespaceOverrides:
      - namespace: ${OVERRIDE_NAMESPACE}
        secretName: ${TEST_PREFIX}-override-secret
EOF

  cat > "${TEMP_DIR}/invalid-rps.yaml" <<EOF
apiVersion: pullsecrets.ognicki.ooo/v1alpha1
kind: RegistryPullSecret
metadata:
  name: ${TEST_PREFIX}-invalid
spec:
  credentialsSecretRef:
    name: ${TEST_PREFIX}-credentials
    namespace: ${OPERATOR_NAMESPACE}
  namespaces:
    policy: Inclusive
    namespaces:
      - ${INCLUDED_NAMESPACE}
      - ${EXCLUDED_NAMESPACE}
EOF
}

run_valid_scenario() {
  kubectl apply -f "${TEMP_DIR}/policy.yaml"
  kubectl apply -f "${TEMP_DIR}/credentials-secret.yaml"
  kubectl apply -f "${TEMP_DIR}/valid-rps.yaml"

  kubectl wait --for=jsonpath='{.status.conditions[?(@.type=="Ready")].status}'=True \
    "registrypullsecret/${TEST_PREFIX}-valid" \
    --timeout="${WAIT_TIMEOUT}"

  assert_secret_exists "${INCLUDED_NAMESPACE}" "${EXPECTED_DEFAULT_SECRET_NAME}"
  assert_secret_exists "${OVERRIDE_NAMESPACE}" "${TEST_PREFIX}-override-secret"
  assert_secret_missing "${EXCLUDED_NAMESPACE}" "${EXPECTED_DEFAULT_SECRET_NAME}"
}

run_invalid_scenario() {
  kubectl apply -f "${TEMP_DIR}/invalid-rps.yaml"

  kubectl wait --for=jsonpath='{.status.conditions[?(@.type=="Ready")].status}'=False \
    "registrypullsecret/${TEST_PREFIX}-invalid" \
    --timeout="${WAIT_TIMEOUT}"

  assert_condition_reason "registrypullsecret" "${TEST_PREFIX}-invalid" "Ready" "ValidationFailed"
  assert_ready_status "pullsecretpolicy" "cluster" "True"
}

main() {
  require_command kubectl
  require_command mktemp
  require_env PSO_TEST_REGISTRY_SERVER "${REGISTRY_SERVER}"
  require_env PSO_TEST_REGISTRY_USERNAME "${REGISTRY_USERNAME}"
  require_env PSO_TEST_REGISTRY_PASSWORD "${REGISTRY_PASSWORD}"

  trap cleanup EXIT

  EXPECTED_DEFAULT_SECRET_NAME="$(derive_secret_name "${REGISTRY_SERVER}")"

  kubectl version --client >/dev/null
  kubectl auth can-i get namespaces >/dev/null

  install_operator
  create_test_namespaces
  write_manifests
  run_valid_scenario
  run_invalid_scenario

  echo "real-cluster smoke test passed for ${TEST_PREFIX}"
}

main "$@"
