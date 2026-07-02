# LIVID Discord Bot

LIVID Discord 서버에서 스터디 제안, 자동 개설, 아카이브를 자동화하는 Go 기반 Discord 봇입니다.

## Legal
- [Terms of Service](./TERMS_OF_SERVICE.md)
- [Privacy Policy](./PRIVACY_POLICY.md)

## 주요 기능
- 스터디 제안: `/suggest`로 제안 등록, 🚀 기준 인원 도달 시 채널/역할 자동 생성
- 제안 리마인드: open 상태 제안을 공지 채널에 알림
- 아카이브: 개별/일괄 아카이브, `archiveN` 카테고리 이동, dry-run 지원
- 멤버 조회: 스터디 역할 기준 active 멤버 목록 표시
- 구조화된 커맨드 로그 출력 (`cmd/stage/guild/user`)

## 환경 변수
실행 전 아래 환경 변수를 설정해야 합니다.

```bash
DISCORD_BOT_TOKEN=<YOUR_BOT_TOKEN>
DISCORD_APPLICATION_ID=<YOUR_APPLICATION_ID>
DISCORD_GUILD_ID=<YOUR_GUILD_ID>
DATABASE_URL=postgres://livid:***@localhost:15432/livid?sslmode=disable
SUGGESTION_CHANNEL_ID=124...                 # optional: 없으면 modal 제출 후 신규-스터디-논의 채널을 이름으로 조회
STUDY_NUDGE_ANNOUNCEMENT_CHANNEL_ID=124...   # optional: 없으면 LIVID 기본 공지 채널 사용
LOG_FORMAT=json   # json | text (default: json)
LOG_LEVEL=info    # debug | info | warn | error (default: info)
LOG_FILE_ENABLED=false
LOG_FILE_PATH=/var/log/livid-bot/bot.log
LOG_FILE_MAX_SIZE_MB=10
LOG_FILE_MAX_BACKUPS=900
LOG_FILE_MAX_AGE_DAYS=730
LOG_FILE_COMPRESS=true
```

예시는 [.env.example](.env.example) 참고.

## 실행 방법

### 1) 로컬 실행
```bash
# DB 실행
docker compose up -d db

# 봇 실행
go run main.go
```

### 2) Docker Compose 전체 실행
```bash
docker compose up --build
```

> 앱 시작 시 DB 마이그레이션이 자동 실행됩니다.

### 3) Kubernetes 배포
운영용 Helm chart와 Argo CD Application은 `~/projects/infrastructure`가 소유합니다.

일반 배포 경로는 `main` push입니다.

```text
livid-bot main push
→ GitHub Actions lint/test
→ GHCR image publish
→ infrastructure repo image tag bump
→ Argo CD auto-sync
```

최초 온보딩/복구 시에는 infrastructure repo에서 Argo CD Application과 runtime secret을 적용합니다.

```bash
cd /Users/haril/projects/infrastructure
mise run cluster:argocd:apply-apps
mise run livid-bot:apply-secrets
```

데이터를 유지한 채 Docker Compose에서 Kubernetes로 넘길 때는
`mise run livid-bot:cutover`를 사용합니다.

## 슬래시 명령어

### `/help`
- 호출자 권한 기준으로 사용 가능한 명령어를 안내
- 옵션
  - `command` (string, optional, autocomplete)
- 동작
  - 옵션 미입력: 사용 가능한 명령 목록을 카드(Embed)로 표시
  - 옵션 입력: 선택한 명령의 상세 정보(설명/권한/옵션)를 카드(Embed)로 표시
  - 응답은 ephemeral(호출자에게만 표시)

### `/members`
- 옵션
  - `channel` (string, required, autocomplete)
- 동작
  - 선택한 스터디의 active 멤버 목록을 표시 (ephemeral)

### `/suggest`
- 옵션
  - `visibility` (string, required: `anonymous` | `public`)
  - `threshold` (integer, optional, 기본 3)
  - `duration_days` (integer, optional, 기본 14, 최대 90)
- 동작
  - 스터디 제안 modal을 표시
  - 신규 스터디 논의 채널에 제안 글을 게시
  - 🚀 기준 인원이 모이면 채널/역할을 자동 생성
  - 같은 제목의 자동 개설 스터디가 이미 있으면 DB 이름에 `-2`, `-3` suffix를 붙여 충돌을 피함

### `/studies`
- 관리자 전용
- 옵션
  - `branch` (string, optional)
  - `status` (string, optional: `active` | `archived`)
- 동작
  - 조건에 맞는 스터디 목록을 표시 (ephemeral)

### `/archive-study`
- 관리자 전용
- 옵션
  - `channel` (string, required, autocomplete)
- 동작
  - 선택한 스터디 채널을 `archiveN` 카테고리로 이동
  - DB 상태를 archived로 변경, 역할 삭제 시도

### `/study-nudge`
- 관리자 전용
- 동작
  - open 상태의 스터디 제안을 공지 채널에 알림

### `/archive-all`
- 관리자 전용
- 옵션
  - `dry-run` (boolean, optional)
- 동작
  - active 스터디 전체 아카이브
  - `dry-run=true`면 실제 변경 없이 배치 결과만 미리보기

## 데이터 모델 변경 사항 (요약)
- `studies` 테이블에 `branch` 컬럼 사용
- 고유성 기준: `name` 단독 → `(branch, name)`
  - 같은 이름의 스터디도 분기가 다르면 생성 가능
  - 자동 개설은 같은 `(branch, name)`이 있으면 suffix를 붙여 저장

## 테스트
```bash
go test ./...
```

커버리지:
```bash
go test ./... -cover
```

GitHub Actions CI:
- `lint`: `gofmt -l .`, `golangci-lint`
- `test`: `go build ./...`, `go test -v ./...`, `go test -race ./db`
- `coverage`: 전체 패키지 coverage 프로파일, 요약, HTML artifact 생성
- `publish-and-update-gitops`: `main` push에서 GHCR image를 publish하고 `songkg7/infrastructure`의 livid-bot image tag를 bump합니다. `INFRA_REPO_TOKEN` secret이 필요합니다.

## 로그
`slog` 기반 구조화 로그를 출력합니다.

- `LOG_FORMAT=json`(기본): JSON 단일 라인 형식
- `LOG_FORMAT=text`: 사람이 읽기 쉬운 key=value 형식
- `LOG_LEVEL`로 최소 출력 레벨 제어
- `LOG_FILE_ENABLED=true`면 로그를 파일에도 동시 저장합니다.
- 파일 저장은 `lumberjack`(size 기반 rotation)으로 처리합니다.
  - `LOG_FILE_MAX_SIZE_MB`: 단일 파일 최대 크기
  - `LOG_FILE_MAX_BACKUPS`: 보관할 백업 파일 수
  - `LOG_FILE_MAX_AGE_DAYS`: 보관 최대 일수
  - `LOG_FILE_COMPRESS=true`: 회전 파일 gzip 압축

Docker Compose 기본 설정(`docker-compose.yml`):
- 파일 로그 경로: `./logs`(호스트) -> `/var/log/livid-bot`(컨테이너)
- 파일 로그 정책: `10MB * 901개(현재+백업)` 약 9GB 수준
- Docker stdout 로그 정책: `json-file`, `20m * 5` (약 100MB)
- 총량은 bot 로그 기준 10GB 이내를 목표로 구성되어 있습니다.

주의:
- 파일 로그의 `MaxAge` 정리는 회전 시점에 적용됩니다.
- `docker compose down`으로 컨테이너를 삭제해도 `./logs`는 유지됩니다.

예시:
```json
{"time":"2026-03-02T10:00:00Z","level":"INFO","msg":"create-study requested branch=26-2 name=algo","cmd":"create-study","stage":"start","guild":"...","user":"..."}
{"time":"2026-03-02T10:00:01Z","level":"INFO","msg":"created study branch=26-2 name=algo channel=... role=...","cmd":"create-study","stage":"success","guild":"...","user":"..."}
```

## Command Audit
- 모든 슬래시 명령(`InteractionApplicationCommand`) 호출은 `command_audit_logs` 테이블에 기록됩니다.
- key는 Discord interaction ID (`i.Interaction.ID`)를 사용합니다.
- 기록 단계: `triggered` -> `success` 또는 `error`
- autocomplete 인터랙션은 audit 대상에서 제외됩니다.
- audit 저장이 실패해도 사용자 명령은 계속 실행됩니다.

## 참고
- 봇 토큰/애플리케이션 정보가 필요한 경우 서버 관리자에게 문의하세요.
