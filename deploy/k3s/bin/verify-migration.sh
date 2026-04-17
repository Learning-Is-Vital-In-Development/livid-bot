#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "${SCRIPT_DIR}/../../.." && pwd)"
SOURCE_PROJECT_DIR="${SOURCE_PROJECT_DIR:-${PROJECT_DIR}}"

RELEASE_NAME="${RELEASE_NAME:-livid-bot}"
NAMESPACE="${NAMESPACE:-livid-bot}"
SOURCE_POSTGRES_USER="${SOURCE_POSTGRES_USER:-livid}"
SOURCE_POSTGRES_PASSWORD="${SOURCE_POSTGRES_PASSWORD:-livid}"
SOURCE_POSTGRES_DB="${SOURCE_POSTGRES_DB:-livid}"
TARGET_POD="${TARGET_POD:-${RELEASE_NAME}-db-0}"

target_secret_key() {
  local key="$1"
  kubectl get secret "${RELEASE_NAME}-db-credentials" \
    -n "${NAMESPACE}" \
    -o "jsonpath={.data.${key}}" | python3 -c 'import base64, sys; print(base64.b64decode(sys.stdin.read()).decode(), end="")'
}

source_psql() {
  local sql="$1"
  (
    cd "${SOURCE_PROJECT_DIR}"
    docker compose exec -T \
      -e PGPASSWORD="${SOURCE_POSTGRES_PASSWORD}" \
      db \
      psql -U "${SOURCE_POSTGRES_USER}" -d "${SOURCE_POSTGRES_DB}" -At -c "${sql}"
  )
}

target_psql() {
  local sql="$1"
  kubectl exec -n "${NAMESPACE}" "${TARGET_POD}" -- env \
    PGPASSWORD="${TARGET_POSTGRES_PASSWORD}" \
    psql -U "${TARGET_POSTGRES_USER}" -d "${TARGET_POSTGRES_DB}" -At -c "${sql}"
}

TARGET_POSTGRES_USER="$(target_secret_key POSTGRES_USER)"
TARGET_POSTGRES_PASSWORD="$(target_secret_key POSTGRES_PASSWORD)"
TARGET_POSTGRES_DB="$(target_secret_key POSTGRES_DB)"

mapfile -t tables < <(source_psql "SELECT tablename FROM pg_tables WHERE schemaname = 'public' ORDER BY tablename;")

if [[ "${#tables[@]}" -eq 0 ]]; then
  echo "No public tables found in source database." >&2
  exit 1
fi

for table in "${tables[@]}"; do
  source_count="$(source_psql "SELECT COUNT(*) FROM \"${table}\";")"
  target_count="$(target_psql "SELECT COUNT(*) FROM \"${table}\";")"
  printf '%-32s source=%s target=%s\n' "${table}" "${source_count}" "${target_count}"
  if [[ "${source_count}" != "${target_count}" ]]; then
    echo "Count mismatch detected for ${table}." >&2
    exit 1
  fi
done

source_migrations="$(source_psql "SELECT filename FROM schema_migrations ORDER BY filename;")"
target_migrations="$(target_psql "SELECT filename FROM schema_migrations ORDER BY filename;")"

if [[ "${source_migrations}" != "${target_migrations}" ]]; then
  echo "schema_migrations mismatch detected." >&2
  exit 1
fi

echo "Migration verification passed."
