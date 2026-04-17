#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "${SCRIPT_DIR}/../../.." && pwd)"
SOURCE_PROJECT_DIR="${SOURCE_PROJECT_DIR:-${PROJECT_DIR}}"
ENV_FILE="${ENV_FILE:-${SOURCE_PROJECT_DIR}/.env}"

RELEASE_NAME="${RELEASE_NAME:-livid-bot}"
NAMESPACE="${NAMESPACE:-livid-bot}"

if [[ -f "${ENV_FILE}" ]]; then
  set -a
  # shellcheck disable=SC1090
  source "${ENV_FILE}"
  set +a
fi

: "${DISCORD_BOT_TOKEN:?DISCORD_BOT_TOKEN is required}"
: "${DISCORD_APPLICATION_ID:?DISCORD_APPLICATION_ID is required}"
: "${DISCORD_GUILD_ID:?DISCORD_GUILD_ID is required}"

POSTGRES_USER="${POSTGRES_USER:-livid}"
POSTGRES_PASSWORD="${POSTGRES_PASSWORD:-livid}"
POSTGRES_DB="${POSTGRES_DB:-livid}"

kubectl create secret generic "${RELEASE_NAME}-db-credentials" \
  --namespace "${NAMESPACE}" \
  --from-literal=POSTGRES_USER="${POSTGRES_USER}" \
  --from-literal=POSTGRES_PASSWORD="${POSTGRES_PASSWORD}" \
  --from-literal=POSTGRES_DB="${POSTGRES_DB}" \
  --dry-run=client \
  -o yaml | kubectl apply -n "${NAMESPACE}" -f -

kubectl create secret generic "${RELEASE_NAME}-bot-secrets" \
  --namespace "${NAMESPACE}" \
  --from-literal=DISCORD_BOT_TOKEN="${DISCORD_BOT_TOKEN}" \
  --from-literal=DISCORD_APPLICATION_ID="${DISCORD_APPLICATION_ID}" \
  --from-literal=DISCORD_GUILD_ID="${DISCORD_GUILD_ID}" \
  --dry-run=client \
  -o yaml | kubectl apply -n "${NAMESPACE}" -f -
