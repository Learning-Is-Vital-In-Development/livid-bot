# Livid Discord Bot

## Getting Started

```bash
go run main.go
```

## env 설정

실행을 위해서는 다음과 같은 환경변수가 설정되어 있어야 합니다.

```toml
[env]
DISCORD_BOT_TOKEN="<YOUR_BOT_TOKEN>"
DISCORD_APPLICATION_ID="<YOUR_APPLICATION_ID>"
DISCORD_GUILD_ID="<YOUR_GUILD_ID>" # 실행할 디스코드 서버 ID
```

`BOT_TOKEN`, `APPLICATION_ID` 는 [Discord Developer Portal](https://discord.com/developers/applications) 에서 확인할 수 있습니다.
