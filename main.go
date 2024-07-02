package main

import (
	"fmt"
	"livid-bot/bot"
	"os"
)

func main() {
	bot.BotToken = os.Getenv("DISCORD_BOT_TOKEN")
	bot.ApplicationID = os.Getenv("DISCORD_APPLICATION_ID")
	bot.GuildID = os.Getenv("DISCORD_GUILD_ID")

	fmt.Println("Bot token: ", bot.BotToken)
	fmt.Println("Application ID: ", bot.ApplicationID)
	fmt.Println("Guild ID: ", bot.GuildID)

	bot.Run()
}
