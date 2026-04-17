#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "${SCRIPT_DIR}/../../.." && pwd)"

RELEASE_NAME="${RELEASE_NAME:-livid-bot}"
NAMESPACE="${NAMESPACE:-livid-bot}"
SERVICE_ACCOUNT_NAME="${SERVICE_ACCOUNT_NAME:-livid-bot-app}"

if ! kubectl get serviceaccount "${SERVICE_ACCOUNT_NAME}" -n "${NAMESPACE}" >/dev/null 2>&1; then
  echo "Missing ${SERVICE_ACCOUNT_NAME} in namespace ${NAMESPACE}." >&2
  echo "Run quant/deploy/k3s/setup/04-bootstrap-cluster.sh first." >&2
  exit 1
fi

helm upgrade --install \
  "${RELEASE_NAME}" \
  "${PROJECT_DIR}/deploy/k3s/charts/livid-bot" \
  --namespace "${NAMESPACE}" \
  --create-namespace \
  "$@"
