# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Test Commands

```bash
go build ./...              # Build all packages
go test ./...               # Run all tests
go test ./... -cover        # Tests with coverage
go test ./bot -run TestName # Run a single test
go vet ./...                # Static analysis
```

## Local Development

```bash
docker compose up -d db     # Start PostgreSQL (port 15432)
go run main.go              # Run bot (requires env vars below)
```

Required environment variables (see `.env.example`):
- `DISCORD_BOT_TOKEN`, `DISCORD_APPLICATION_ID`, `DISCORD_GUILD_ID`, `DATABASE_URL`

## Architecture

Go module `livid-bot` — a Discord bot for study group lifecycle management (create → recruit → start → archive).

### Package Structure

- **`main.go`** — Entry point. Loads env vars, connects DB, runs migrations, initializes repositories, starts bot.
- **`bot/`** — Discord interaction layer. Slash command definitions (`commands.go`), handler closures (`handler_*.go`), reaction tracking (`reaction.go`), archive category allocation (`archive_category.go`), helpers (`helpers.go`).
- **`db/`** — PostgreSQL data access via `pgx/v5`. Repository pattern (`StudyRepository`, `MemberRepository`, `RecruitRepository`). Embedded SQL migrations auto-applied on startup.
- **`study/`** — Domain types (`Study`, `StudyMember`, `RecruitMessage`, `RecruitMapping`).

### Key Patterns

**Handler pattern**: Each command handler is a closure returned by `newXxxHandler(repos...)`, capturing dependencies. Registered in `bot.go` as `map[string]func`.

**Copy-on-write concurrency**: `ReactionHandler` uses `sync.RWMutex` + copy-on-write for its in-memory `messageID → emoji → roleInfo` map. `Track`/`Untrack` create new maps rather than mutating.

**Archive category allocator**: Manages `archiveN` categories with a 50-channel limit, reservation/commit/release pattern, and read-only permission enforcement.

**Migrations**: Embedded SQL files in `db/migrations/` with `NNN_description.sql` naming. Tracked in `schema_migrations` table. Applied transactionally on startup.

### Conventions

- Branch format: `YY-Q` (e.g., `26-2` for 2026 Q2). Validated by `isValidBranch()`.
- Structured logging: `[cmd=X stage=Y guild=Z user=W] message`.
- Error responses to users are ephemeral. Errors are wrapped with `fmt.Errorf("context: %w", err)`.
- Tests use table-driven style.
- Autocomplete handlers are registered separately from command handlers in `bot.go`.
