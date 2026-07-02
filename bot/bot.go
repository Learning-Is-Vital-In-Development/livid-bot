package bot

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"livid-bot/db"
)

type Config struct {
	BotToken                        string
	ApplicationID                   string
	GuildID                         string
	SuggestionChannelID             string
	StudyNudgeAnnouncementChannelID string
	StudyRepo                       *db.StudyRepository
	MemberRepo                      *db.MemberRepository
	AuditRepo                       CommandAuditStore
	SuggestionRepo                  *db.SuggestionRepository
}

const (
	interactionTimeout = 10 * time.Second
	reactionTimeout    = 10 * time.Second
	discordHTTPTimeout = 15 * time.Second
)

func Run(cfg Config) error {
	setCommandAuditStore(cfg.AuditRepo)

	discord, err := discordgo.New("Bot " + cfg.BotToken)
	if err != nil {
		return fmt.Errorf("create discord session: %w", err)
	}

	configureDiscordSession(discord)

	suggestionReactionHandler := NewSuggestionReactionHandler(cfg.SuggestionRepo, cfg.StudyRepo, cfg.MemberRepo)

	commandHandlers := map[string]func(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate){
		"help":          handleHelp,
		"archive-study": newArchiveStudyHandler(cfg.StudyRepo),
		"studies":       newStudiesHandler(cfg.StudyRepo),
		"members":       newMembersHandler(cfg.StudyRepo, cfg.MemberRepo),
		"archive-all":   newArchiveAllHandler(cfg.StudyRepo),
		"suggest":       newSuggestHandler(cfg.SuggestionRepo, cfg.SuggestionChannelID),
		"study-nudge":   newStudyNudgeHandler(cfg.SuggestionRepo, cfg.StudyNudgeAnnouncementChannelID),
	}
	autocompleteHandlers := map[string]func(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate){
		"help":          handleHelpAutocomplete,
		"archive-study": newArchiveStudyAutocompleteHandler(cfg.StudyRepo),
		"members":       newMembersAutocompleteHandler(cfg.StudyRepo),
	}

	discord.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		ctx, cancel := context.WithTimeout(context.Background(), interactionTimeout)
		defer cancel()
		ctx, span := startInteractionSpan(ctx, i)
		defer span.End()

		switch i.Type {
		case discordgo.InteractionApplicationCommand:
			commandName := i.ApplicationCommandData().Name
			recordCommandTriggered(ctx, i)
			logCommand(ctx, i, "dispatch", "received application command")
			if h, ok := commandHandlers[commandName]; ok {
				h(ctx, s, i)
			} else {
				logCommand(ctx, i, "error", "no handler registered for command=%s", commandName)
			}
		case discordgo.InteractionApplicationCommandAutocomplete:
			commandName := i.ApplicationCommandData().Name
			logCommand(ctx, i, "dispatch", "received autocomplete interaction")
			if h, ok := autocompleteHandlers[commandName]; ok {
				h(ctx, s, i)
			}
		case discordgo.InteractionModalSubmit:
			customID := i.ModalSubmitData().CustomID
			if strings.HasPrefix(customID, suggestionModalPrefix) {
				newSuggestModalHandler(cfg.SuggestionRepo)(ctx, s, i)
			}
		}
	})

	discord.AddHandler(suggestionReactionHandler.OnReactionAdd)
	discord.AddHandler(suggestionReactionHandler.OnReactionRemove)

	if err := discord.Open(); err != nil {
		return fmt.Errorf("open discord session: %w", err)
	}
	defer func() {
		if err := discord.Close(); err != nil {
			slog.Warn("failed to close discord session", "error", err)
		}
	}()

	if err := syncCommands(discord, cfg.ApplicationID, cfg.GuildID); err != nil {
		return fmt.Errorf("sync commands: %w", err)
	}

	slog.Info("bot running; press CTRL + C to exit")
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(c)
	<-c

	return nil
}

func configureDiscordSession(discord *discordgo.Session) {
	if discord == nil {
		return
	}
	if discord.Client == nil || discord.Client == http.DefaultClient {
		discord.Client = &http.Client{Transport: http.DefaultTransport, Timeout: discordHTTPTimeout}
	}
	if discord.Client.Transport == nil {
		discord.Client.Transport = http.DefaultTransport
	}
	if discord.Client.Timeout == 0 {
		discord.Client.Timeout = discordHTTPTimeout
	}
	discord.Client.Transport = otelhttp.NewTransport(discord.Client.Transport)
	discord.Identify.Intents = discordgo.IntentsGuilds |
		discordgo.IntentsGuildMessages |
		discordgo.IntentsGuildMessageReactions
}
