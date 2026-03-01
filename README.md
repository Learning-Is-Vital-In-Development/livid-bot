# LIVID Discord Bot

LIVID Discord 서버에서 스터디 생성, 모집, 아카이브를 자동화하는 Go 기반 Discord 봇입니다.

## 주요 기능
- 스터디 생성: 분기(`YY-Q`) 기반 채널/역할 자동 생성
- 스터디 모집: 분기 선택 후 해당 분기의 active 스터디만 모집 공지
- 아카이브: 개별/일괄 아카이브, `archiveN` 카테고리 이동, dry-run 지원
- 반응 기반 역할 부여/해제
- 구조화된 커맨드 로그 출력 (`cmd/stage/guild/user`)

## 환경 변수
실행 전 아래 환경 변수를 설정해야 합니다.

```bash
DISCORD_BOT_TOKEN=<YOUR_BOT_TOKEN>
DISCORD_APPLICATION_ID=<YOUR_APPLICATION_ID>
DISCORD_GUILD_ID=<YOUR_GUILD_ID>
DATABASE_URL=postgres://livid:livid@localhost:15432/livid?sslmode=disable
LOG_FORMAT=text   # text | json (default: text)
LOG_LEVEL=info    # debug | info | warn | error (default: info)
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

### `/create-study`
- 관리자 전용
- 옵션
  - `branch` (string, required) 예: `26-2`
  - `name` (string, required)
  - `description` (string, optional)
- 동작
  - 채널명: `<branch>-<name>` 형식으로 생성 (예: `26-2-algo`)
  - 역할명: `<name>` (branch prefix 없음)
  - `name`에 이미 `YY-Q-` prefix가 있으면 자동 제거
- branch 형식 검증: `YY-Q` (`Q`는 `1~4`)

### `/recruit`
- 관리자 전용
- 옵션
  - `channel` (channel, required)
  - `branch` (string, required, autocomplete)
  - `from` (string, required, `YYYY-MM-DD`)
  - `to` (string, required, `YYYY-MM-DD`)
- 동작
  - 선택한 branch의 active 스터디만 모집
  - 모집 메시지에 branch 정보 표시

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

### `/study-start`
- 관리자 전용
- 옵션
  - `branch` (string, required, autocomplete)
- 동작
  - 분기의 모집을 마감하고 멤버 수 기준으로 스터디 시작/아카이브 처리 (ephemeral)

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

## 테스트
```bash
go test ./...
```

커버리지:
```bash
go test ./... -cover
```

## 로그
`slog` 기반 구조화 로그를 출력합니다.

- `LOG_FORMAT=text`(기본): 사람이 읽기 쉬운 key=value 형식
- `LOG_FORMAT=json`: JSON 단일 라인 형식
- `LOG_LEVEL`로 최소 출력 레벨 제어

예시:
```text
time=2026-03-02T10:00:00Z level=INFO msg="create-study requested branch=26-2 name=algo" cmd=create-study stage=start guild=... user=...
time=2026-03-02T10:00:01Z level=INFO msg="created study branch=26-2 name=algo channel=... role=..." cmd=create-study stage=success guild=... user=...
```

## Command Audit
- 모든 슬래시 명령(`InteractionApplicationCommand`) 호출은 `command_audit_logs` 테이블에 기록됩니다.
- key는 Discord interaction ID (`i.Interaction.ID`)를 사용합니다.
- 기록 단계: `triggered` -> `success` 또는 `error`
- autocomplete 인터랙션은 audit 대상에서 제외됩니다.
- audit 저장이 실패해도 사용자 명령은 계속 실행됩니다.

## 참고
- 봇 토큰/애플리케이션 정보가 필요한 경우 서버 관리자에게 문의하세요.
