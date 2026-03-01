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

### `/hello`
- 간단한 응답 확인용 명령어

### `/submit`
- 옵션
  - `screenshot` (attachment, required)
  - `link` (string, required)
- 문제 링크를 마크다운으로 변환해 임베드 메시지로 게시

### `/create-study`
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
- 옵션
  - `channel` (channel, required)
  - `branch` (string, required, autocomplete)
  - `from` (string, required, `YYYY-MM-DD`)
  - `to` (string, required, `YYYY-MM-DD`)
- 동작
  - 선택한 branch의 active 스터디만 모집
  - 모집 메시지에 branch 정보 표시

### `/archive-study`
- 옵션
  - `channel` (string, required, autocomplete)
- 동작
  - 선택한 스터디 채널을 `archiveN` 카테고리로 이동
  - DB 상태를 archived로 변경, 역할 삭제 시도

### `/archive-all`
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
커맨드 라이프사이클 로그를 출력합니다.

예시:
```text
[cmd=create-study stage=start guild=... user=...] create-study requested branch=26-2 name=algo
[cmd=create-study stage=success guild=... user=...] created study branch=26-2 name=algo channel=... role=...
```

## 참고
- 봇 토큰/애플리케이션 정보가 필요한 경우 서버 관리자에게 문의하세요.
