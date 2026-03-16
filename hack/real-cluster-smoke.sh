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
REGISTRY_SERVER_2="${PSO_TEST_REGISTRY_SERVER_2:-}"
REGISTRY_USERNAME_2="${PSO_TEST_REGISTRY_USERNAME_2:-}"
REGISTRY_PASSWORD_2="${PSO_TEST_REGISTRY_PASSWORD_2:-}"
REGISTRY_EMAIL_2="${PSO_TEST_REGISTRY_EMAIL_2:-ops@example.com}"
REGISTRY_SERVER_3="${PSO_TEST_REGISTRY_SERVER_3:-}"
REGISTRY_USERNAME_3="${PSO_TEST_REGISTRY_USERNAME_3:-}"
REGISTRY_PASSWORD_3="${PSO_TEST_REGISTRY_PASSWORD_3:-}"
REGISTRY_EMAIL_3="${PSO_TEST_REGISTRY_EMAIL_3:-ops@example.com}"
WAIT_TIMEOUT="${PSO_WAIT_TIMEOUT:-180s}"
USE_CACHE="${PSO_SMOKE_USE_CACHE:-true}"
FORCE_RERUN="${PSO_SMOKE_FORCE_RERUN:-false}"

TEST_PREFIX="psop-e2e-${TEST_ID}"
INCLUDED_NAMESPACE="${TEST_PREFIX}-include"
OVERRIDE_NAMESPACE="${TEST_PREFIX}-override"
EXCLUDED_NAMESPACE="${TEST_PREFIX}-excluded"
INVALID_NAMESPACE="${TEST_PREFIX}-invalid"
EXCLUSIVE_ALLOWED_NAMESPACE="${TEST_PREFIX}-exclusive-allow"
EXCLUSIVE_BLOCKED_NAMESPACE="${TEST_PREFIX}-exclusive-block"
INLINE_NAMESPACE="${TEST_PREFIX}-inline"
UPDATE_OLD_NAMESPACE="${TEST_PREFIX}-update-old"
UPDATE_NEW_NAMESPACE="${TEST_PREFIX}-update-new"
COLLISION_NAMESPACE="${TEST_PREFIX}-collision"
DRIFT_NAMESPACE="${TEST_PREFIX}-drift"
MULTI_REGISTRY_NAMESPACE="${TEST_PREFIX}-multi"

EXPECTED_DEFAULT_SECRET_NAME=""
EXPECTED_DEFAULT_SECRET_NAME_2=""
EXPECTED_DEFAULT_SECRET_NAME_3=""
TEMP_DIR=""
CACHE_DIR="${ROOT_DIR}/.smoke-cache/real-cluster"
CACHE_INPUT_HASH=""

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

log_test_cached() {
  printf '%b\n' "${COLOR_BOLD}${COLOR_YELLOW}[CACHED]${COLOR_RESET} $* OK"
}

namespace_allowed_by_policy() {
  local namespace="$1"

  case "${namespace}" in
    "${INCLUDED_NAMESPACE}" | \
    "${OVERRIDE_NAMESPACE}" | \
    "${EXCLUSIVE_ALLOWED_NAMESPACE}" | \
    "${EXCLUSIVE_BLOCKED_NAMESPACE}" | \
    "${INLINE_NAMESPACE}" | \
    "${UPDATE_OLD_NAMESPACE}" | \
    "${UPDATE_NEW_NAMESPACE}" | \
    "${COLLISION_NAMESPACE}" | \
    "${DRIFT_NAMESPACE}" | \
    "${MULTI_REGISTRY_NAMESPACE}")
      return 0
      ;;
  esac

  return 1
}

cleanup_test_resources() {
  log_chore "Cleaning up test-created custom resources, credentials Secret, and throwaway namespaces"
  kubectl delete registrypullsecret --ignore-not-found \
    "${TEST_PREFIX}-valid" \
    "${TEST_PREFIX}-invalid" \
    "${TEST_PREFIX}-exclusive" \
    "${TEST_PREFIX}-inline" \
    "${TEST_PREFIX}-update" \
    "${TEST_PREFIX}-multi-2" \
    "${TEST_PREFIX}-multi-3" >/dev/null 2>&1 || true
  kubectl delete pullsecretpolicy --ignore-not-found cluster >/dev/null 2>&1 || true
  kubectl -n "${OPERATOR_NAMESPACE}" delete secret --ignore-not-found \
    "${TEST_PREFIX}-credentials" \
    "${TEST_PREFIX}-credentials-2" \
    "${TEST_PREFIX}-credentials-3" >/dev/null 2>&1 || true
  kubectl delete namespace --ignore-not-found \
    "${INCLUDED_NAMESPACE}" \
    "${OVERRIDE_NAMESPACE}" \
    "${EXCLUDED_NAMESPACE}" \
    "${INVALID_NAMESPACE}" \
    "${EXCLUSIVE_ALLOWED_NAMESPACE}" \
    "${EXCLUSIVE_BLOCKED_NAMESPACE}" \
    "${INLINE_NAMESPACE}" \
    "${UPDATE_OLD_NAMESPACE}" \
    "${UPDATE_NEW_NAMESPACE}" \
    "${COLLISION_NAMESPACE}" \
    "${DRIFT_NAMESPACE}" \
    "${MULTI_REGISTRY_NAMESPACE}" >/dev/null 2>&1 || true
}

cleanup_operator_install() {
  log_chore "Removing operator installation resources to restore a clean cluster state"
  kubectl delete -f "${ROOT_DIR}/config/manager/manager.yaml" >/dev/null 2>&1 || true
  kubectl delete -f "${ROOT_DIR}/config/rbac/manager.yaml" >/dev/null 2>&1 || true
  kubectl delete -f "${ROOT_DIR}/config/crd/pullsecrets.ognicki.ooo_registrypullsecrets.yaml" >/dev/null 2>&1 || true
  kubectl delete -f "${ROOT_DIR}/config/crd/pullsecrets.ognicki.ooo_pullsecretpolicies.yaml" >/dev/null 2>&1 || true
  kubectl wait --for=delete namespace/"${OPERATOR_NAMESPACE}" --timeout=120s >/dev/null 2>&1 || true
}

cleanup() {
  log_info "Starting smoke-test cleanup"
  cleanup_test_resources
  cleanup_operator_install
  if [[ -n "${TEMP_DIR}" ]]; then
    rm -rf "${TEMP_DIR}"
  fi
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

compute_cache_input_hash() {
  local combined_hashes=""
  while IFS= read -r file; do
    combined_hashes="${combined_hashes}$(LC_ALL=C shasum -a 256 "${file}")"$'\n'
  done < <(
    find \
      "${ROOT_DIR}/api/pullsecrets/v1alpha1" \
      "${ROOT_DIR}/cmd/manager" \
      "${ROOT_DIR}/config/crd" \
      "${ROOT_DIR}/config/manager" \
      "${ROOT_DIR}/config/rbac" \
      "${ROOT_DIR}/hack" \
      "${ROOT_DIR}/internal/controller" \
      "${ROOT_DIR}/internal/sync" \
      "${ROOT_DIR}/pkg/metadata" \
      -type f | sort
  )

  combined_hashes="${combined_hashes}image ${OPERATOR_IMAGE}"$'\n'
  LC_ALL=C printf '%s' "${combined_hashes}" | LC_ALL=C shasum -a 256 | awk '{print $1}'
}

scenario_cache_file() {
  local scenario_key="$1"
  printf '%s/%s-%s.pass\n' "${CACHE_DIR}" "${scenario_key}" "${CACHE_INPUT_HASH}"
}

scenario_is_cached() {
  local scenario_key="$1"

  if [[ "${USE_CACHE}" != "true" || "${FORCE_RERUN}" == "true" ]]; then
    return 1
  fi

  [[ -f "$(scenario_cache_file "${scenario_key}")" ]]
}

record_scenario_pass() {
  local scenario_key="$1"

  mkdir -p "${CACHE_DIR}"
  : > "$(scenario_cache_file "${scenario_key}")"
}

run_scenario() {
  local scenario_key="$1"
  local scenario_summary="$2"
  local scenario_function="$3"

  if scenario_is_cached "${scenario_key}"; then
    log_test_cached "${scenario_summary} (reused cached passing result)"
    return 0
  fi

  "${scenario_function}"
  record_scenario_pass "${scenario_key}"
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
  log_chore "Creating unique test namespaces for included, excluded, inline, exclusive, update, and invalid scenarios"
  kubectl create namespace "${INCLUDED_NAMESPACE}"
  kubectl create namespace "${OVERRIDE_NAMESPACE}"
  kubectl create namespace "${EXCLUDED_NAMESPACE}"
  kubectl create namespace "${INVALID_NAMESPACE}"
  kubectl create namespace "${EXCLUSIVE_ALLOWED_NAMESPACE}"
  kubectl create namespace "${EXCLUSIVE_BLOCKED_NAMESPACE}"
  kubectl create namespace "${INLINE_NAMESPACE}"
  kubectl create namespace "${UPDATE_OLD_NAMESPACE}"
  kubectl create namespace "${UPDATE_NEW_NAMESPACE}"
  kubectl create namespace "${COLLISION_NAMESPACE}"
  kubectl create namespace "${DRIFT_NAMESPACE}"
  kubectl create namespace "${MULTI_REGISTRY_NAMESPACE}"
}

write_manifests() {
  log_chore "Rendering temporary manifests for the cluster policy, credentials Secret, and RegistryPullSecret scenarios"
  local policy_exclusions=""
  while IFS= read -r namespace; do
    if ! namespace_allowed_by_policy "${namespace}"; then
      policy_exclusions="${policy_exclusions}"$'\n'"    - ${namespace}"
    fi
  done < <(kubectl get namespaces -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}')

  cat > "${TEMP_DIR}/policy.yaml" <<EOF
apiVersion: pullsecrets.ognicki.ooo/v1alpha1
kind: PullSecretPolicy
metadata:
  name: cluster
spec:
  excludedNamespaces:${policy_exclusions}
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

  cat > "${TEMP_DIR}/exclusive-rps.yaml" <<EOF
apiVersion: pullsecrets.ognicki.ooo/v1alpha1
kind: RegistryPullSecret
metadata:
  name: ${TEST_PREFIX}-exclusive
spec:
  credentialsSecretRef:
    name: ${TEST_PREFIX}-credentials
    namespace: ${OPERATOR_NAMESPACE}
  namespaces:
    policy: Exclusive
    namespaces:
      - ${EXCLUSIVE_BLOCKED_NAMESPACE}
      - ${INCLUDED_NAMESPACE}
      - ${OVERRIDE_NAMESPACE}
      - ${INLINE_NAMESPACE}
      - ${UPDATE_OLD_NAMESPACE}
      - ${UPDATE_NEW_NAMESPACE}
      - ${INVALID_NAMESPACE}
      - ${COLLISION_NAMESPACE}
      - ${DRIFT_NAMESPACE}
      - ${MULTI_REGISTRY_NAMESPACE}
EOF

  cat > "${TEMP_DIR}/inline-rps.yaml" <<EOF
apiVersion: pullsecrets.ognicki.ooo/v1alpha1
kind: RegistryPullSecret
metadata:
  name: ${TEST_PREFIX}-inline
spec:
  credentials:
    server: ${REGISTRY_SERVER}
    username: ${REGISTRY_USERNAME}
    password: ${REGISTRY_PASSWORD}
    email: ${REGISTRY_EMAIL}
  namespaces:
    policy: Inclusive
    namespaces:
      - ${INLINE_NAMESPACE}
EOF

  cat > "${TEMP_DIR}/update-rps-initial.yaml" <<EOF
apiVersion: pullsecrets.ognicki.ooo/v1alpha1
kind: RegistryPullSecret
metadata:
  name: ${TEST_PREFIX}-update
spec:
  credentialsSecretRef:
    name: ${TEST_PREFIX}-credentials
    namespace: ${OPERATOR_NAMESPACE}
  namespaces:
    policy: Inclusive
    namespaces:
      - ${UPDATE_OLD_NAMESPACE}
EOF

  cat > "${TEMP_DIR}/update-rps-updated.yaml" <<EOF
apiVersion: pullsecrets.ognicki.ooo/v1alpha1
kind: RegistryPullSecret
metadata:
  name: ${TEST_PREFIX}-update
spec:
  credentialsSecretRef:
    name: ${TEST_PREFIX}-credentials
    namespace: ${OPERATOR_NAMESPACE}
  namespaces:
    policy: Inclusive
    namespaces:
      - ${UPDATE_NEW_NAMESPACE}
    namespaceOverrides:
      - namespace: ${UPDATE_NEW_NAMESPACE}
        secretName: ${TEST_PREFIX}-updated-secret
EOF

  cat > "${TEMP_DIR}/duplicate-namespaces-rps.yaml" <<EOF
apiVersion: pullsecrets.ognicki.ooo/v1alpha1
kind: RegistryPullSecret
metadata:
  name: ${TEST_PREFIX}-duplicate-namespaces
spec:
  credentialsSecretRef:
    name: ${TEST_PREFIX}-credentials
    namespace: ${OPERATOR_NAMESPACE}
  namespaces:
    policy: Inclusive
    namespaces:
      - ${INCLUDED_NAMESPACE}
      - ${INCLUDED_NAMESPACE}
EOF

  cat > "${TEMP_DIR}/duplicate-overrides-rps.yaml" <<EOF
apiVersion: pullsecrets.ognicki.ooo/v1alpha1
kind: RegistryPullSecret
metadata:
  name: ${TEST_PREFIX}-duplicate-overrides
spec:
  credentialsSecretRef:
    name: ${TEST_PREFIX}-credentials
    namespace: ${OPERATOR_NAMESPACE}
  namespaces:
    policy: Inclusive
    namespaces:
      - ${OVERRIDE_NAMESPACE}
    namespaceOverrides:
      - namespace: ${OVERRIDE_NAMESPACE}
        secretName: ${TEST_PREFIX}-override-one
      - namespace: ${OVERRIDE_NAMESPACE}
        secretName: ${TEST_PREFIX}-override-two
EOF

  cat > "${TEMP_DIR}/wildcard-rps.yaml" <<EOF
apiVersion: pullsecrets.ognicki.ooo/v1alpha1
kind: RegistryPullSecret
metadata:
  name: ${TEST_PREFIX}-wildcard
spec:
  credentialsSecretRef:
    name: ${TEST_PREFIX}-credentials
    namespace: ${OPERATOR_NAMESPACE}
  namespaces:
    policy: Inclusive
    namespaces:
      - ${TEST_PREFIX}-*
EOF

  cat > "${TEMP_DIR}/short-secret-name-rps.yaml" <<EOF
apiVersion: pullsecrets.ognicki.ooo/v1alpha1
kind: RegistryPullSecret
metadata:
  name: ${TEST_PREFIX}-short-secret-name
spec:
  credentialsSecretRef:
    name: ${TEST_PREFIX}-credentials
    namespace: ${OPERATOR_NAMESPACE}
  namespaces:
    policy: Inclusive
    namespaces:
      - ${INLINE_NAMESPACE}
    namespaceOverrides:
      - namespace: ${INLINE_NAMESPACE}
        secretName: ab
EOF

  cat > "${TEMP_DIR}/collision-secret.yaml" <<EOF
apiVersion: v1
kind: Secret
metadata:
  name: ${EXPECTED_DEFAULT_SECRET_NAME}
  namespace: ${COLLISION_NAMESPACE}
type: Opaque
stringData:
  note: foreign-secret
EOF

  cat > "${TEMP_DIR}/collision-rps.yaml" <<EOF
apiVersion: pullsecrets.ognicki.ooo/v1alpha1
kind: RegistryPullSecret
metadata:
  name: ${TEST_PREFIX}-collision
spec:
  credentialsSecretRef:
    name: ${TEST_PREFIX}-credentials
    namespace: ${OPERATOR_NAMESPACE}
  namespaces:
    policy: Inclusive
    namespaces:
      - ${COLLISION_NAMESPACE}
EOF

  cat > "${TEMP_DIR}/drift-rps.yaml" <<EOF
apiVersion: pullsecrets.ognicki.ooo/v1alpha1
kind: RegistryPullSecret
metadata:
  name: ${TEST_PREFIX}-drift
spec:
  credentialsSecretRef:
    name: ${TEST_PREFIX}-credentials
    namespace: ${OPERATOR_NAMESPACE}
  namespaces:
    policy: Inclusive
    namespaces:
      - ${DRIFT_NAMESPACE}
EOF

  cat > "${TEMP_DIR}/credentials-secret-2.yaml" <<EOF
apiVersion: v1
kind: Secret
metadata:
  name: ${TEST_PREFIX}-credentials-2
  namespace: ${OPERATOR_NAMESPACE}
type: Opaque
stringData:
  server: ${REGISTRY_SERVER_2}
  username: ${REGISTRY_USERNAME_2}
  password: ${REGISTRY_PASSWORD_2}
  email: ${REGISTRY_EMAIL_2}
EOF

  cat > "${TEMP_DIR}/credentials-secret-3.yaml" <<EOF
apiVersion: v1
kind: Secret
metadata:
  name: ${TEST_PREFIX}-credentials-3
  namespace: ${OPERATOR_NAMESPACE}
type: Opaque
stringData:
  server: ${REGISTRY_SERVER_3}
  username: ${REGISTRY_USERNAME_3}
  password: ${REGISTRY_PASSWORD_3}
  email: ${REGISTRY_EMAIL_3}
EOF

  cat > "${TEMP_DIR}/multi-rps-2.yaml" <<EOF
apiVersion: pullsecrets.ognicki.ooo/v1alpha1
kind: RegistryPullSecret
metadata:
  name: ${TEST_PREFIX}-multi-2
spec:
  credentialsSecretRef:
    name: ${TEST_PREFIX}-credentials-2
    namespace: ${OPERATOR_NAMESPACE}
  namespaces:
    policy: Inclusive
    namespaces:
      - ${MULTI_REGISTRY_NAMESPACE}
EOF

  cat > "${TEMP_DIR}/multi-rps-3.yaml" <<EOF
apiVersion: pullsecrets.ognicki.ooo/v1alpha1
kind: RegistryPullSecret
metadata:
  name: ${TEST_PREFIX}-multi-3
spec:
  credentialsSecretRef:
    name: ${TEST_PREFIX}-credentials-3
    namespace: ${OPERATOR_NAMESPACE}
  namespaces:
    policy: Inclusive
    namespaces:
      - ${MULTI_REGISTRY_NAMESPACE}
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

run_exclusive_scenario() {
  log_test_start "Exclusive policy: namespaces named in the local exclusion list must stay untouched while other namespaces still receive the managed pull secret"
  kubectl apply -f "${TEMP_DIR}/exclusive-rps.yaml"

  kubectl wait --for=jsonpath='{.status.conditions[?(@.type=="Ready")].status}'=True \
    "registrypullsecret/${TEST_PREFIX}-exclusive" \
    --timeout="${WAIT_TIMEOUT}"

  assert_secret_exists "${EXCLUSIVE_ALLOWED_NAMESPACE}" "${EXPECTED_DEFAULT_SECRET_NAME}"
  assert_secret_missing "${EXCLUSIVE_BLOCKED_NAMESPACE}" "${EXPECTED_DEFAULT_SECRET_NAME}"
  assert_secret_missing "${EXCLUDED_NAMESPACE}" "${EXPECTED_DEFAULT_SECRET_NAME}"
  log_test_pass "Exclusive policy assertions completed successfully"
}

run_inline_credentials_scenario() {
  log_test_start "Inline credentials mode: a RegistryPullSecret with in-spec credentials must reconcile successfully and create the managed pull secret"
  kubectl apply -f "${TEMP_DIR}/inline-rps.yaml"

  kubectl wait --for=jsonpath='{.status.conditions[?(@.type=="Ready")].status}'=True \
    "registrypullsecret/${TEST_PREFIX}-inline" \
    --timeout="${WAIT_TIMEOUT}"

  assert_secret_exists "${INLINE_NAMESPACE}" "${EXPECTED_DEFAULT_SECRET_NAME}"
  log_test_pass "Inline credentials assertions completed successfully"
}

run_update_and_cleanup_scenario() {
  log_test_start "Update and cleanup: changing a RegistryPullSecret must create the new target secret and remove the obsolete managed secret"
  kubectl apply -f "${TEMP_DIR}/update-rps-initial.yaml"

  kubectl wait --for=jsonpath='{.status.conditions[?(@.type=="Ready")].status}'=True \
    "registrypullsecret/${TEST_PREFIX}-update" \
    --timeout="${WAIT_TIMEOUT}"

  assert_secret_exists "${UPDATE_OLD_NAMESPACE}" "${EXPECTED_DEFAULT_SECRET_NAME}"

  kubectl apply -f "${TEMP_DIR}/update-rps-updated.yaml"

  kubectl wait --for=jsonpath='{.status.conditions[?(@.type=="Ready")].status}'=True \
    "registrypullsecret/${TEST_PREFIX}-update" \
    --timeout="${WAIT_TIMEOUT}"

  assert_secret_exists "${UPDATE_NEW_NAMESPACE}" "${TEST_PREFIX}-updated-secret"
  assert_secret_missing "${UPDATE_OLD_NAMESPACE}" "${EXPECTED_DEFAULT_SECRET_NAME}"
  log_test_pass "Update and cleanup assertions completed successfully"
}

run_non_destructive_delete_scenario() {
  log_test_start "Source deletion: deleting a RegistryPullSecret must leave the already managed target secret in place"
  kubectl delete registrypullsecret "${TEST_PREFIX}-update"
  sleep 5
  assert_secret_exists "${UPDATE_NEW_NAMESPACE}" "${TEST_PREFIX}-updated-secret"
  log_test_pass "Non-destructive source deletion assertions completed successfully"
}

run_duplicate_namespaces_validation_scenario() {
  log_test_start "Validation failure: duplicated namespace entries in the explicit namespace list must be rejected"
  kubectl apply -f "${TEMP_DIR}/duplicate-namespaces-rps.yaml"

  kubectl wait --for=jsonpath='{.status.conditions[?(@.type=="Ready")].status}'=False \
    "registrypullsecret/${TEST_PREFIX}-duplicate-namespaces" \
    --timeout="${WAIT_TIMEOUT}"

  assert_condition_reason "registrypullsecret" "${TEST_PREFIX}-duplicate-namespaces" "Ready" "ValidationFailed"
  log_test_pass "Duplicate namespace validation assertions completed successfully"
}

run_duplicate_overrides_validation_scenario() {
  log_test_start "Validation failure: duplicated namespace override entries must be rejected"
  kubectl apply -f "${TEMP_DIR}/duplicate-overrides-rps.yaml"

  kubectl wait --for=jsonpath='{.status.conditions[?(@.type=="Ready")].status}'=False \
    "registrypullsecret/${TEST_PREFIX}-duplicate-overrides" \
    --timeout="${WAIT_TIMEOUT}"

  assert_condition_reason "registrypullsecret" "${TEST_PREFIX}-duplicate-overrides" "Ready" "ValidationFailed"
  log_test_pass "Duplicate override validation assertions completed successfully"
}

run_wildcard_validation_scenario() {
  log_test_start "Validation failure: wildcard namespace patterns must be rejected"
  kubectl apply -f "${TEMP_DIR}/wildcard-rps.yaml"

  kubectl wait --for=jsonpath='{.status.conditions[?(@.type=="Ready")].status}'=False \
    "registrypullsecret/${TEST_PREFIX}-wildcard" \
    --timeout="${WAIT_TIMEOUT}"

  assert_condition_reason "registrypullsecret" "${TEST_PREFIX}-wildcard" "Ready" "ValidationFailed"
  log_test_pass "Wildcard namespace validation assertions completed successfully"
}

run_short_secret_name_validation_scenario() {
  log_test_start "Validation failure: an explicitly configured target pull-secret name must satisfy the minimum naming rules"
  kubectl apply -f "${TEMP_DIR}/short-secret-name-rps.yaml"

  kubectl wait --for=jsonpath='{.status.conditions[?(@.type=="Ready")].status}'=False \
    "registrypullsecret/${TEST_PREFIX}-short-secret-name" \
    --timeout="${WAIT_TIMEOUT}"

  assert_condition_reason "registrypullsecret" "${TEST_PREFIX}-short-secret-name" "Ready" "ValidationFailed"
  log_test_pass "Short secret-name validation assertions completed successfully"
}

run_collision_validation_scenario() {
  log_test_start "Validation failure: a target secret collision with a foreign unmanaged Secret must be rejected"
  kubectl create -f "${TEMP_DIR}/collision-secret.yaml"
  kubectl apply -f "${TEMP_DIR}/collision-rps.yaml"

  kubectl wait --for=jsonpath='{.status.conditions[?(@.type=="Ready")].status}'=False \
    "registrypullsecret/${TEST_PREFIX}-collision" \
    --timeout="${WAIT_TIMEOUT}"

  assert_condition_reason "registrypullsecret" "${TEST_PREFIX}-collision" "Ready" "ValidationFailed"
  log_test_pass "Foreign secret collision validation assertions completed successfully"
}

run_modified_replica_secret_scenario() {
  log_test_start "Replica drift on modification: manually changing a managed replica Secret must not trigger immediate re-synchronization"
  kubectl apply -f "${TEMP_DIR}/drift-rps.yaml"

  kubectl wait --for=jsonpath='{.status.conditions[?(@.type=="Ready")].status}'=True \
    "registrypullsecret/${TEST_PREFIX}-drift" \
    --timeout="${WAIT_TIMEOUT}"

  kubectl -n "${DRIFT_NAMESPACE}" patch secret "${EXPECTED_DEFAULT_SECRET_NAME}" \
    --type merge \
    -p '{"metadata":{"annotations":{"smoke.ognicki.ooo/drift":"modified"}}}'

  sleep 5

  local annotation_value
  annotation_value="$(kubectl -n "${DRIFT_NAMESPACE}" get secret "${EXPECTED_DEFAULT_SECRET_NAME}" -o "jsonpath={.metadata.annotations.smoke\\.ognicki\\.ooo/drift}")"
  if [[ "${annotation_value}" != "modified" ]]; then
    echo "managed replica secret modification was overwritten unexpectedly" >&2
    exit 1
  fi

  log_test_pass "Replica modification drift assertions completed successfully"
}

run_deleted_replica_secret_scenario() {
  log_test_start "Replica drift on deletion: manually deleting a managed replica Secret must not trigger immediate recreation"
  kubectl -n "${DRIFT_NAMESPACE}" delete secret "${EXPECTED_DEFAULT_SECRET_NAME}"

  sleep 5

  assert_secret_missing "${DRIFT_NAMESPACE}" "${EXPECTED_DEFAULT_SECRET_NAME}"
  log_test_pass "Replica deletion drift assertions completed successfully"
}

run_multi_registry_scenario() {
  log_test_start "Multi-registry replication: additional registry credential sources must create distinct managed pull secrets in the same namespace"
  kubectl apply -f "${TEMP_DIR}/credentials-secret-2.yaml"
  kubectl apply -f "${TEMP_DIR}/credentials-secret-3.yaml"
  kubectl apply -f "${TEMP_DIR}/multi-rps-2.yaml"
  kubectl apply -f "${TEMP_DIR}/multi-rps-3.yaml"

  kubectl wait --for=jsonpath='{.status.conditions[?(@.type=="Ready")].status}'=True \
    "registrypullsecret/${TEST_PREFIX}-multi-2" \
    --timeout="${WAIT_TIMEOUT}"
  kubectl wait --for=jsonpath='{.status.conditions[?(@.type=="Ready")].status}'=True \
    "registrypullsecret/${TEST_PREFIX}-multi-3" \
    --timeout="${WAIT_TIMEOUT}"

  assert_secret_exists "${MULTI_REGISTRY_NAMESPACE}" "${EXPECTED_DEFAULT_SECRET_NAME_2}"
  assert_secret_exists "${MULTI_REGISTRY_NAMESPACE}" "${EXPECTED_DEFAULT_SECRET_NAME_3}"
  log_test_pass "Multi-registry assertions completed successfully"
}

main() {
  require_command kubectl
  require_command mktemp
  require_command shasum
  require_env PSO_TEST_REGISTRY_SERVER "${REGISTRY_SERVER}"
  require_env PSO_TEST_REGISTRY_USERNAME "${REGISTRY_USERNAME}"
  require_env PSO_TEST_REGISTRY_PASSWORD "${REGISTRY_PASSWORD}"
  require_env PSO_TEST_REGISTRY_SERVER_2 "${REGISTRY_SERVER_2}"
  require_env PSO_TEST_REGISTRY_USERNAME_2 "${REGISTRY_USERNAME_2}"
  require_env PSO_TEST_REGISTRY_PASSWORD_2 "${REGISTRY_PASSWORD_2}"
  require_env PSO_TEST_REGISTRY_SERVER_3 "${REGISTRY_SERVER_3}"
  require_env PSO_TEST_REGISTRY_USERNAME_3 "${REGISTRY_USERNAME_3}"
  require_env PSO_TEST_REGISTRY_PASSWORD_3 "${REGISTRY_PASSWORD_3}"

  EXPECTED_DEFAULT_SECRET_NAME="$(derive_secret_name "${REGISTRY_SERVER}")"
  EXPECTED_DEFAULT_SECRET_NAME_2="$(derive_secret_name "${REGISTRY_SERVER_2}")"
  EXPECTED_DEFAULT_SECRET_NAME_3="$(derive_secret_name "${REGISTRY_SERVER_3}")"
  CACHE_INPUT_HASH="$(compute_cache_input_hash)"
  log_info "Loaded smoke-test configuration for registry ${REGISTRY_SERVER}"
  log_info "Loaded additional registry configuration for ${REGISTRY_SERVER_2} and ${REGISTRY_SERVER_3}"
  log_info "Expected default managed Secret name: ${EXPECTED_DEFAULT_SECRET_NAME}"
  log_info "Expected additional managed Secret names: ${EXPECTED_DEFAULT_SECRET_NAME_2}, ${EXPECTED_DEFAULT_SECRET_NAME_3}"
  log_info "Test resource prefix: ${TEST_PREFIX}"
  log_info "Smoke cache enabled: ${USE_CACHE}"

  kubectl version --client >/dev/null
  log_chore "Checking Kubernetes API connectivity with kubectl cluster-info"
  kubectl cluster-info >/dev/null
  log_success "Kubernetes API connectivity check: accessible"
  kubectl auth can-i get namespaces --all-namespaces >/dev/null

  if scenario_is_cached "valid" &&
    scenario_is_cached "invalid" &&
    scenario_is_cached "exclusive" &&
    scenario_is_cached "inline" &&
    scenario_is_cached "update" &&
    scenario_is_cached "non-destructive-delete" &&
    scenario_is_cached "duplicate-namespaces" &&
    scenario_is_cached "duplicate-overrides" &&
    scenario_is_cached "wildcard" &&
    scenario_is_cached "short-secret-name" &&
    scenario_is_cached "collision" &&
    scenario_is_cached "modified-replica-secret" &&
    scenario_is_cached "deleted-replica-secret" &&
    scenario_is_cached "multi-registry"; then
    log_test_cached "Happy-path replication scenario"
    log_test_cached "Cluster-excluded namespace validation scenario"
    log_test_cached "Exclusive policy scenario"
    log_test_cached "Inline credentials scenario"
    log_test_cached "Update and cleanup scenario"
    log_test_cached "Non-destructive source deletion scenario"
    log_test_cached "Duplicate namespace validation scenario"
    log_test_cached "Duplicate override validation scenario"
    log_test_cached "Wildcard namespace validation scenario"
    log_test_cached "Short secret-name validation scenario"
    log_test_cached "Foreign secret collision validation scenario"
    log_test_cached "Replica modification drift scenario"
    log_test_cached "Replica deletion drift scenario"
    log_test_cached "Multi-registry scenario"
    log_success "All real-cluster smoke scenarios already have cached passing results for the current input hash"
    exit 0
  fi

  TEMP_DIR="$(mktemp -d)"
  trap cleanup EXIT
  log_chore "Performing pre-run reset to avoid interference from previous smoke runs"
  cleanup_operator_install

  install_operator
  create_test_namespaces
  write_manifests
  run_scenario "valid" "Happy-path replication scenario" run_valid_scenario
  run_scenario "invalid" "Cluster-excluded namespace validation scenario" run_invalid_scenario
  run_scenario "exclusive" "Exclusive policy scenario" run_exclusive_scenario
  run_scenario "inline" "Inline credentials scenario" run_inline_credentials_scenario
  run_scenario "update" "Update and cleanup scenario" run_update_and_cleanup_scenario
  run_scenario "non-destructive-delete" "Non-destructive source deletion scenario" run_non_destructive_delete_scenario
  run_scenario "duplicate-namespaces" "Duplicate namespace validation scenario" run_duplicate_namespaces_validation_scenario
  run_scenario "duplicate-overrides" "Duplicate override validation scenario" run_duplicate_overrides_validation_scenario
  run_scenario "wildcard" "Wildcard namespace validation scenario" run_wildcard_validation_scenario
  run_scenario "short-secret-name" "Short secret-name validation scenario" run_short_secret_name_validation_scenario
  run_scenario "collision" "Foreign secret collision validation scenario" run_collision_validation_scenario
  run_scenario "modified-replica-secret" "Replica modification drift scenario" run_modified_replica_secret_scenario
  run_scenario "deleted-replica-secret" "Replica deletion drift scenario" run_deleted_replica_secret_scenario
  run_scenario "multi-registry" "Multi-registry scenario" run_multi_registry_scenario

  log_success "Real-cluster smoke test passed for ${TEST_PREFIX}"
}

main "$@"
