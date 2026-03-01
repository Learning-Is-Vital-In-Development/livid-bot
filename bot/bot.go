package bot

import (
	"fmt"
	"log"
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

	// Initialize reaction handler and load existing mappings from DB
	reactionHandler := NewReactionHandler(cfg.MemberRepo)
	if err := reactionHandler.LoadFromDB(cfg.RecruitRepo); err != nil {
		log.Printf("Warning: failed to load reaction mappings: %v", err)
	}

	commandHandlers := map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate){
		"hello":         handleHello,
		"submit":        handleSubmit,
		"create-study":  newCreateStudyHandler(cfg.StudyRepo),
		"recruit":       newRecruitHandler(cfg.StudyRepo, cfg.RecruitRepo, reactionHandler),
		"archive-study": newArchiveStudyHandler(cfg.StudyRepo),
		"archive-all":   newArchiveAllHandler(cfg.StudyRepo),
	}
	autocompleteHandlers := map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate){
		"archive-study": newArchiveStudyAutocompleteHandler(cfg.StudyRepo),
	}

	discord.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		switch i.Type {
		case discordgo.InteractionApplicationCommand:
			commandName := i.ApplicationCommandData().Name
			if h, ok := commandHandlers[commandName]; ok {
				h(s, i)
			}
		case discordgo.InteractionApplicationCommandAutocomplete:
			commandName := i.ApplicationCommandData().Name
			if h, ok := autocompleteHandlers[commandName]; ok {
				h(s, i)
			}
		}
	})

	discord.AddHandler(reactionHandler.OnReactionAdd)
	discord.AddHandler(reactionHandler.OnReactionRemove)

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
