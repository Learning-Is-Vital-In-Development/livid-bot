# livid-bot on k3s

`livid-bot`의 Docker Compose 런타임을 k3s로 옮기기 위한 Helm chart와
데이터 보존형 컷오버 스크립트 모음이다.

## Layout

- `charts/livid-bot`: PostgreSQL + Discord bot Helm chart
- `bin/apply-secrets.sh`: `.env` 기반 Kubernetes Secret 반영
- `bin/deploy.sh`: Helm 배포
- `bin/verify-migration.sh`: Docker 원본 DB와 k3s DB row count 비교
- `bin/cutover.sh`: dump/restore + 로그 PVC 이관 + bot scale-up

## Runtime Contract

- Namespace: `livid-bot`
- Helm release: `livid-bot`
- Secret names:
  - `livid-bot-db-credentials`
  - `livid-bot-bot-secrets`
- PVC names:
  - `pgdata-livid-bot-db-0`
  - `livid-bot-logs`

## First Deploy

Prerequisite: `quant` 저장소의 `deploy/k3s/setup/04-bootstrap-cluster.sh`를 먼저 실행해
`livid-bot` namespace와 `livid-bot-app` ServiceAccount를 만들어 둔다.

```bash
cd /Users/haril/projects/livid-bot
./deploy/k3s/bin/apply-secrets.sh
./deploy/k3s/bin/deploy.sh
```

## Data-Preserving Cutover

컷오버는 짧은 점검창을 전제로 한다. `bot` 컨테이너를 멈춘 뒤 최종
`pg_dump`를 수행하고, 복구 가능한 Docker 원본을 그대로 남겨 둔다.

```bash
cd /Users/haril/projects/livid-bot
./deploy/k3s/bin/cutover.sh
```

worktree에서 실행할 때는 원본 Docker Compose 체크아웃 경로를 지정한다.

```bash
SOURCE_PROJECT_DIR=/Users/haril/projects/livid-bot ./deploy/k3s/bin/cutover.sh
```

실행 내용:

1. Secrets 적용
2. Helm 배포 (`bot.replicas=0`)
3. Docker 원본 DB pre-cutover dump
4. Docker bot 정지 후 final dump
5. k3s PostgreSQL restore
6. `./logs`를 PVC로 복사
7. row count / `schema_migrations` 검증
8. `bot.replicas=1`로 scale-up

## Rollback

원본 Docker DB 볼륨과 `./logs`는 컷오버 후에도 유지된다.

```bash
cd /Users/haril/projects/livid-bot
helm uninstall livid-bot -n livid-bot
docker compose up -d db bot
```
