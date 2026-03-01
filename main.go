package main

import (
	"context"
	"log/slog"
	"os"

	"livid-bot/bot"
	"livid-bot/db"
	"livid-bot/internal/logging"
)

func main() {
	logging.Configure()

	token := requireEnv("DISCORD_BOT_TOKEN")
	appID := requireEnv("DISCORD_APPLICATION_ID")
	guildID := requireEnv("DISCORD_GUILD_ID")
	databaseURL := requireEnv("DATABASE_URL")

	ctx := context.Background()

	pool, err := db.NewPool(ctx, databaseURL)
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := db.Migrate(ctx, pool); err != nil {
		slog.Error("failed to run migrations", "error", err)
		os.Exit(1)
	}

	cfg := bot.Config{
		BotToken:      token,
		ApplicationID: appID,
		GuildID:       guildID,
		StudyRepo:     db.NewStudyRepository(pool),
		MemberRepo:    db.NewMemberRepository(pool),
		RecruitRepo:   db.NewRecruitRepository(pool),
		AuditRepo:     db.NewCommandAuditRepository(pool),
	}

	if err := bot.Run(cfg); err != nil {
		slog.Error("bot exited with error", "error", err)
		os.Exit(1)
	}
}

func requireEnv(key string) string {
	value := os.Getenv(key)
	if value == "" {
		slog.Error("required environment variable is not set", "key", key)
		os.Exit(1)
	}
	return value
}
