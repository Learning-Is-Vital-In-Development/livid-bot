#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "${SCRIPT_DIR}/../../.." && pwd)"
SOURCE_PROJECT_DIR="${SOURCE_PROJECT_DIR:-${PROJECT_DIR}}"
OUTPUT_DIR="${PROJECT_DIR}/deploy/k3s/output"

RELEASE_NAME="${RELEASE_NAME:-livid-bot}"
NAMESPACE="${NAMESPACE:-livid-bot}"
SOURCE_POSTGRES_USER="${SOURCE_POSTGRES_USER:-livid}"
SOURCE_POSTGRES_PASSWORD="${SOURCE_POSTGRES_PASSWORD:-livid}"
SOURCE_POSTGRES_DB="${SOURCE_POSTGRES_DB:-livid}"
TIMESTAMP="$(date +%Y%m%d-%H%M%S)"

PRE_DUMP="${OUTPUT_DIR}/${TIMESTAMP}-pre-cutover.dump"
FINAL_DUMP="${OUTPUT_DIR}/${TIMESTAMP}-final.dump"
HELPER_POD="${RELEASE_NAME}-logs-sync"

mkdir -p "${OUTPUT_DIR}"

target_secret_key() {
  local key="$1"
  kubectl get secret "${RELEASE_NAME}-db-credentials" \
    -n "${NAMESPACE}" \
    -o "jsonpath={.data.${key}}" | python3 -c 'import base64, sys; print(base64.b64decode(sys.stdin.read()).decode(), end="")'
}

cleanup_helper() {
  kubectl delete pod "${HELPER_POD}" -n "${NAMESPACE}" --ignore-not-found >/dev/null 2>&1 || true
}

trap cleanup_helper EXIT

"${SCRIPT_DIR}/apply-secrets.sh"
"${SCRIPT_DIR}/deploy.sh" --set bot.replicas=0

kubectl rollout status statefulset/"${RELEASE_NAME}-db" -n "${NAMESPACE}" --timeout=180s

(
  cd "${SOURCE_PROJECT_DIR}"
  docker compose exec -T \
    -e PGPASSWORD="${SOURCE_POSTGRES_PASSWORD}" \
    db \
    pg_dump -U "${SOURCE_POSTGRES_USER}" -d "${SOURCE_POSTGRES_DB}" -Fc
) > "${PRE_DUMP}"

(
  cd "${SOURCE_PROJECT_DIR}"
  docker compose stop bot
)

(
  cd "${SOURCE_PROJECT_DIR}"
  docker compose exec -T \
    -e PGPASSWORD="${SOURCE_POSTGRES_PASSWORD}" \
    db \
    pg_dump -U "${SOURCE_POSTGRES_USER}" -d "${SOURCE_POSTGRES_DB}" -Fc
) > "${FINAL_DUMP}"

TARGET_POSTGRES_USER="$(target_secret_key POSTGRES_USER)"
TARGET_POSTGRES_PASSWORD="$(target_secret_key POSTGRES_PASSWORD)"
TARGET_POSTGRES_DB="$(target_secret_key POSTGRES_DB)"

kubectl exec -i -n "${NAMESPACE}" "${RELEASE_NAME}-db-0" -- env \
  PGPASSWORD="${TARGET_POSTGRES_PASSWORD}" \
  pg_restore \
    --clean \
    --if-exists \
    --no-owner \
    --no-privileges \
    -U "${TARGET_POSTGRES_USER}" \
    -d "${TARGET_POSTGRES_DB}" < "${FINAL_DUMP}"

kubectl apply -n "${NAMESPACE}" -f - <<EOF
apiVersion: v1
kind: Pod
metadata:
  name: ${HELPER_POD}
spec:
  restartPolicy: Never
  containers:
    - name: sync
      image: busybox:1.36
      command: ["sh", "-ec", "sleep 3600"]
      volumeMounts:
        - name: logs
          mountPath: /logs
  volumes:
    - name: logs
      persistentVolumeClaim:
        claimName: ${RELEASE_NAME}-logs
EOF

kubectl wait --for=condition=Ready pod/"${HELPER_POD}" -n "${NAMESPACE}" --timeout=120s
if [[ -d "${SOURCE_PROJECT_DIR}/logs" ]]; then
  kubectl cp "${SOURCE_PROJECT_DIR}/logs/." "${NAMESPACE}/${HELPER_POD}:/logs"
fi

"${SCRIPT_DIR}/verify-migration.sh"
"${SCRIPT_DIR}/deploy.sh" --set bot.replicas=1
kubectl rollout status deployment/"${RELEASE_NAME}" -n "${NAMESPACE}" --timeout=180s

echo "Cutover completed."
echo "Pre-cutover dump: ${PRE_DUMP}"
echo "Final dump: ${FINAL_DUMP}"
