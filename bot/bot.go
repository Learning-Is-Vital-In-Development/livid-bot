package bot

import (
	"fmt"
	"log/slog"
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
	AuditRepo     CommandAuditStore
}

func Run(cfg Config) error {
	setCommandAuditStore(cfg.AuditRepo)

	discord, err := discordgo.New("Bot " + cfg.BotToken)
	if err != nil {
		return fmt.Errorf("create discord session: %w", err)
	}

	discord.Identify.Intents = discordgo.IntentsGuilds |
		discordgo.IntentsGuildMessages |
		discordgo.IntentsGuildMessageReactions

	// Initialize reaction handler and load existing mappings from DB
	reactionHandler := NewReactionHandler(cfg.MemberRepo)
	if err := reactionHandler.LoadFromDB(cfg.RecruitRepo); err != nil {
		slog.Warn("failed to load reaction mappings", "error", err)
	}

	commandHandlers := map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate){
		"help":          handleHelp,
		"create-study":  newCreateStudyHandler(cfg.StudyRepo),
		"recruit":       newRecruitHandler(cfg.StudyRepo, cfg.RecruitRepo, reactionHandler),
		"archive-study": newArchiveStudyHandler(cfg.StudyRepo),
		"studies":       newStudiesHandler(cfg.StudyRepo),
		"members":       newMembersHandler(cfg.StudyRepo, cfg.MemberRepo),
		"archive-all":   newArchiveAllHandler(cfg.StudyRepo),
		"study-start":   newStudyStartHandler(cfg.StudyRepo, cfg.MemberRepo, cfg.RecruitRepo, reactionHandler),
	}
	autocompleteHandlers := map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate){
		"archive-study": newArchiveStudyAutocompleteHandler(cfg.StudyRepo),
		"members":       newMembersAutocompleteHandler(cfg.StudyRepo),
		"recruit":       newRecruitBranchAutocompleteHandler(cfg.StudyRepo),
		"study-start":   newStudyStartAutocompleteHandler(cfg.StudyRepo),
	}

	discord.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		switch i.Type {
		case discordgo.InteractionApplicationCommand:
			commandName := i.ApplicationCommandData().Name
			recordCommandTriggered(i)
			logCommand(i, "dispatch", "received application command")
			if h, ok := commandHandlers[commandName]; ok {
				h(s, i)
			} else {
				logCommand(i, "error", "no handler registered for command=%s", commandName)
			}
		case discordgo.InteractionApplicationCommandAutocomplete:
			commandName := i.ApplicationCommandData().Name
			logCommand(i, "dispatch", "received autocomplete interaction")
			if h, ok := autocompleteHandlers[commandName]; ok {
				h(s, i)
			}
		}
	})

	discord.AddHandler(reactionHandler.OnReactionAdd)
	discord.AddHandler(reactionHandler.OnReactionRemove)

	if err := discord.Open(); err != nil {
		return fmt.Errorf("open discord session: %w", err)
	}
	defer func() {
		if err := discord.Close(); err != nil {
			slog.Warn("failed to close discord session", "error", err)
		}
	}()

	for _, command := range commands {
		if _, err := discord.ApplicationCommandCreate(cfg.ApplicationID, cfg.GuildID, command); err != nil {
			return fmt.Errorf("register command %q: %w", command.Name, err)
		}
	}

	slog.Info("bot running; press CTRL + C to exit")
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c

	return nil
}
