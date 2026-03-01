package main

import (
	"context"
	"log"
	"os"

	"livid-bot/bot"
	"livid-bot/db"
)

func main() {
	token := requireEnv("DISCORD_BOT_TOKEN")
	appID := requireEnv("DISCORD_APPLICATION_ID")
	guildID := requireEnv("DISCORD_GUILD_ID")
	databaseURL := requireEnv("DATABASE_URL")

	ctx := context.Background()

	pool, err := db.NewPool(ctx, databaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer pool.Close()

	if err := db.Migrate(ctx, pool); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	cfg := bot.Config{
		BotToken:      token,
		ApplicationID: appID,
		GuildID:       guildID,
		StudyRepo:     db.NewStudyRepository(pool),
		MemberRepo:    db.NewMemberRepository(pool),
		RecruitRepo:   db.NewRecruitRepository(pool),
	}

	if err := bot.Run(cfg); err != nil {
		log.Fatalf("Bot exited with error: %v", err)
	}
}

func requireEnv(key string) string {
	value := os.Getenv(key)
	if value == "" {
		log.Fatalf("Required environment variable %s is not set", key)
	}
	return value
}
