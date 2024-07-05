# LIVID Discord Bot

## Prerequisite

실행을 위해서는 다음과 같은 환경변수가 설정되어 있어야 합니다.

```toml
[env]
DISCORD_BOT_TOKEN="<YOUR_BOT_TOKEN>"
DISCORD_APPLICATION_ID="<YOUR_APPLICATION_ID>"
DISCORD_GUILD_ID="<YOUR_GUILD_ID>" # 실행할 디스코드 서버 ID
```

봇에 기여하기 위해 토큰 정보가 필요할 경우는 관리자에게 문의해주세요.

## Getting Started

```bash
go run main.go
```

or

```bash
# project path
docker build -t livid-bot .
docker run -d --name livid-bot \
  -e DISCORD_BOT_TOKEN=$LIVID_BOT_TOKEN \
  -e DISCORD_GUILD_ID=$LIVID_GUILD_ID \
  -e DISCORD_APPLICATION_ID=$LIVID_APPLICATION_ID \
  livid-bot
```
