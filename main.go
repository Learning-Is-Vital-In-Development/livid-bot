package main

import (
	"context"
	"log"
	"os"

	"livid-bot/bot"
	"livid-bot/db"
)

func main() {
	token := os.Getenv("DISCORD_BOT_TOKEN")
	appID := os.Getenv("DISCORD_APPLICATION_ID")
	guildID := os.Getenv("DISCORD_GUILD_ID")
	databaseURL := os.Getenv("DATABASE_URL")

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

	bot.Run(cfg)
}
