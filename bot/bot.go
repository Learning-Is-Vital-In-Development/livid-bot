package bot

import (
	"fmt"
	"os"
	"os/signal"

	"github.com/bwmarrin/discordgo"
	"livid-bot/db"
)

type Config struct {
	BotToken      string
	ApplicationID string
	GuildID       string
	StudyRepo     *db.StudyRepository
	MemberRepo    *db.MemberRepository
	RecruitRepo   *db.RecruitRepository
}

func Run(cfg Config) {
	discord, err := discordgo.New("Bot " + cfg.BotToken)
	checkNilErr(err)

	discord.Identify.Intents = discordgo.IntentsGuilds |
		discordgo.IntentsGuildMessages |
		discordgo.IntentsGuildMessageReactions

	commandHandlers := map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate){
		"hello":  handleHello,
		"submit": handleSubmit,
	}

	discord.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		if h, ok := commandHandlers[i.ApplicationCommandData().Name]; ok {
			h(s, i)
		}
	})

	registeredCommands := make([]*discordgo.ApplicationCommand, len(commands))
	for i, command := range commands {
		cmd, err := discord.ApplicationCommandCreate(cfg.ApplicationID, cfg.GuildID, command)
		checkNilErr(err)
		registeredCommands[i] = cmd
	}

	err = discord.Open()
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	defer discord.Close()

	fmt.Println("Bot running.... Press CTRL + C to exit")
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c
}
