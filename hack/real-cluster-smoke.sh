#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
source "${ROOT_DIR}/hack/lib/load-dotenv.sh"
load_default_dotenv_files "${ROOT_DIR}"

TEST_ID="${PSO_TEST_ID:-$(date +%Y%m%d%H%M%S)}"
OPERATOR_NAMESPACE="${PSO_OPERATOR_NAMESPACE:-pull-secrets}"
OPERATOR_IMAGE="${PSO_IMAGE:-ghcr.io/mwognicki/pull-secrets-operator:v0.1.0-beta.1}"
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

COLOR_BOLD='\033[1m'
COLOR_BLUE='\033[34m'
COLOR_GREEN='\033[32m'
COLOR_YELLOW='\033[33m'
COLOR_RESET='\033[0m'

log_info() {
  printf '%b\n' "${COLOR_BOLD}${COLOR_BLUE}[smoke]${COLOR_RESET} $*"
}

log_success() {
  printf '%b\n' "${COLOR_BOLD}${COLOR_GREEN}[smoke]${COLOR_RESET} $*"
}

log_chore() {
  printf '%b\n' "${COLOR_BOLD}${COLOR_YELLOW}Maintenance/chore:${COLOR_RESET} $*"
}

log_test_start() {
  printf '%b\n' "${COLOR_BOLD}${COLOR_BLUE}Test/assertion:${COLOR_RESET} $*"
}

log_test_pass() {
  printf '%b\n' "${COLOR_BOLD}${COLOR_GREEN}Test/assertion passed:${COLOR_RESET} $*"
}

cleanup_test_resources() {
  set +e
  log_chore "Cleaning up test-created custom resources, credentials Secret, and throwaway namespaces"
  kubectl delete registrypullsecret --ignore-not-found "${TEST_PREFIX}-valid" "${TEST_PREFIX}-invalid" >/dev/null 2>&1
  kubectl delete pullsecretpolicy --ignore-not-found cluster >/dev/null 2>&1
  kubectl -n "${OPERATOR_NAMESPACE}" delete secret --ignore-not-found "${TEST_PREFIX}-credentials" >/dev/null 2>&1
  kubectl delete namespace --ignore-not-found \
    "${INCLUDED_NAMESPACE}" \
    "${OVERRIDE_NAMESPACE}" \
    "${EXCLUDED_NAMESPACE}" \
    "${INVALID_NAMESPACE}" >/dev/null 2>&1
}

cleanup_operator_install() {
  set +e
  log_chore "Removing operator installation resources to restore a clean cluster state"
  kubectl delete -f "${ROOT_DIR}/config/manager/manager.yaml" >/dev/null 2>&1
  kubectl delete -f "${ROOT_DIR}/config/rbac/manager.yaml" >/dev/null 2>&1
  kubectl delete -f "${ROOT_DIR}/config/crd/pullsecrets.ognicki.ooo_registrypullsecrets.yaml" >/dev/null 2>&1
  kubectl delete -f "${ROOT_DIR}/config/crd/pullsecrets.ognicki.ooo_pullsecretpolicies.yaml" >/dev/null 2>&1
  kubectl wait --for=delete namespace/"${OPERATOR_NAMESPACE}" --timeout=120s >/dev/null 2>&1
}

cleanup() {
  log_info "Starting smoke-test cleanup"
  cleanup_test_resources
  cleanup_operator_install
  rm -rf "${TEMP_DIR}"
  log_success "Smoke-test cleanup completed"
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
  log_chore "Installing CRDs, manager deployment, and RBAC for image ${OPERATOR_IMAGE}"
  kubectl apply -f "${ROOT_DIR}/config/crd/pullsecrets.ognicki.ooo_pullsecretpolicies.yaml"
  kubectl apply -f "${ROOT_DIR}/config/crd/pullsecrets.ognicki.ooo_registrypullsecrets.yaml"
  kubectl apply -f "${ROOT_DIR}/config/manager/manager.yaml"
  kubectl apply -f "${ROOT_DIR}/config/rbac/manager.yaml"
  kubectl -n "${OPERATOR_NAMESPACE}" set image deployment/pull-secrets-operator-manager manager="${OPERATOR_IMAGE}"
  kubectl -n "${OPERATOR_NAMESPACE}" rollout status deployment/pull-secrets-operator-manager --timeout="${WAIT_TIMEOUT}"
}

create_test_namespaces() {
  log_chore "Creating unique test namespaces for included, overridden, excluded, and invalid scenarios"
  kubectl create namespace "${INCLUDED_NAMESPACE}"
  kubectl create namespace "${OVERRIDE_NAMESPACE}"
  kubectl create namespace "${EXCLUDED_NAMESPACE}"
  kubectl create namespace "${INVALID_NAMESPACE}"
}

write_manifests() {
  log_chore "Rendering temporary manifests for the cluster policy, credentials Secret, and RegistryPullSecret scenarios"
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
  log_test_start "Happy-path replication: included namespaces receive managed pull secrets, overrides rename them, and excluded namespaces stay untouched"
  kubectl apply -f "${TEMP_DIR}/policy.yaml"
  kubectl apply -f "${TEMP_DIR}/credentials-secret.yaml"
  kubectl apply -f "${TEMP_DIR}/valid-rps.yaml"

  kubectl wait --for=jsonpath='{.status.conditions[?(@.type=="Ready")].status}'=True \
    "registrypullsecret/${TEST_PREFIX}-valid" \
    --timeout="${WAIT_TIMEOUT}"

  assert_secret_exists "${INCLUDED_NAMESPACE}" "${EXPECTED_DEFAULT_SECRET_NAME}"
  assert_secret_exists "${OVERRIDE_NAMESPACE}" "${TEST_PREFIX}-override-secret"
  assert_secret_missing "${EXCLUDED_NAMESPACE}" "${EXPECTED_DEFAULT_SECRET_NAME}"
  log_test_pass "Happy-path replication assertions completed successfully"
}

run_invalid_scenario() {
  log_test_start "Validation failure: an explicitly targeted namespace that is excluded cluster-wide must drive Ready=False with ValidationFailed"
  kubectl apply -f "${TEMP_DIR}/invalid-rps.yaml"

  kubectl wait --for=jsonpath='{.status.conditions[?(@.type=="Ready")].status}'=False \
    "registrypullsecret/${TEST_PREFIX}-invalid" \
    --timeout="${WAIT_TIMEOUT}"

  assert_condition_reason "registrypullsecret" "${TEST_PREFIX}-invalid" "Ready" "ValidationFailed"
  assert_ready_status "pullsecretpolicy" "cluster" "True"
  log_test_pass "Validation-failure assertions completed successfully"
}

main() {
  require_command kubectl
  require_command mktemp
  require_env PSO_TEST_REGISTRY_SERVER "${REGISTRY_SERVER}"
  require_env PSO_TEST_REGISTRY_USERNAME "${REGISTRY_USERNAME}"
  require_env PSO_TEST_REGISTRY_PASSWORD "${REGISTRY_PASSWORD}"

  trap cleanup EXIT

  EXPECTED_DEFAULT_SECRET_NAME="$(derive_secret_name "${REGISTRY_SERVER}")"
  log_info "Loaded smoke-test configuration for registry ${REGISTRY_SERVER}"
  log_info "Expected default managed Secret name: ${EXPECTED_DEFAULT_SECRET_NAME}"
  log_info "Test resource prefix: ${TEST_PREFIX}"

  kubectl version --client >/dev/null
  kubectl auth can-i get namespaces --all-namespaces >/dev/null
  log_chore "Performing pre-run reset to avoid interference from previous smoke runs"
  cleanup_operator_install

  install_operator
  create_test_namespaces
  write_manifests
  run_valid_scenario
  run_invalid_scenario

  log_success "Real-cluster smoke test passed for ${TEST_PREFIX}"
}

main "$@"
