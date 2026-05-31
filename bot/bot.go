package bot

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/bwmarrin/discordgo"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"livid-bot/db"
)

type Config struct {
	BotToken       string
	ApplicationID  string
	GuildID        string
	StudyRepo      *db.StudyRepository
	MemberRepo     *db.MemberRepository
	RecruitRepo    *db.RecruitRepository
	AuditRepo      CommandAuditStore
	SuggestionRepo *db.SuggestionRepository
	VoiceRepo      VoiceSessionStore
}

func Run(cfg Config) error {
	setCommandAuditStore(cfg.AuditRepo)

	discord, err := discordgo.New("Bot " + cfg.BotToken)
	if err != nil {
		return fmt.Errorf("create discord session: %w", err)
	}

	configureDiscordSession(discord)

	if cfg.VoiceRepo != nil {
		closed, err := cfg.VoiceRepo.CloseOpenSessions(context.Background(), time.Now().UTC(), "bot_restart")
		if err != nil {
			slog.Warn("failed to close stale voice sessions on startup", "error", err)
		} else if closed > 0 {
			slog.Info("closed stale voice sessions on startup", "count", closed)
		}
	}

	// Initialize reaction handler and load existing mappings from DB
	reactionHandler := NewReactionHandler(cfg.MemberRepo)
	if err := reactionHandler.LoadFromDB(cfg.RecruitRepo); err != nil {
		slog.Warn("failed to load reaction mappings", "error", err)
	}

	commandHandlers := map[string]func(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate){
		"help":          handleHelp,
		"create-study":  newCreateStudyHandler(cfg.StudyRepo),
		"recruit":       newRecruitHandler(cfg.StudyRepo, cfg.RecruitRepo, reactionHandler),
		"archive-study": newArchiveStudyHandler(cfg.StudyRepo),
		"studies":       newStudiesHandler(cfg.StudyRepo),
		"members":       newMembersHandler(cfg.StudyRepo, cfg.MemberRepo),
		"archive-all":   newArchiveAllHandler(cfg.StudyRepo),
		"study-start":   newStudyStartHandler(cfg.StudyRepo, cfg.MemberRepo, cfg.RecruitRepo, reactionHandler),
		"suggest-start": newSuggestStartHandler(cfg.SuggestionRepo),
		"suggest":       newSuggestHandler(cfg.SuggestionRepo),
		"vote":          newVoteHandler(cfg.SuggestionRepo),
		"voice-stats":   newVoiceStatsHandler(cfg.VoiceRepo),
	}
	autocompleteHandlers := map[string]func(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate){
		"help":          handleHelpAutocomplete,
		"archive-study": newArchiveStudyAutocompleteHandler(cfg.StudyRepo),
		"members":       newMembersAutocompleteHandler(cfg.StudyRepo),
		"recruit":       newRecruitBranchAutocompleteHandler(cfg.StudyRepo),
		"study-start":   newStudyStartAutocompleteHandler(cfg.StudyRepo),
	}

	discord.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		ctx, span := startInteractionSpan(context.Background(), i)
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
			switch customID {
			case "suggest_modal":
				newSuggestModalHandler(cfg.SuggestionRepo)(ctx, s, i)
			}
		case discordgo.InteractionMessageComponent:
			customID := i.MessageComponentData().CustomID
			switch customID {
			case "vote_select":
				newVoteSelectHandler(cfg.SuggestionRepo)(ctx, s, i)
			}
		}
	})

	discord.AddHandler(reactionHandler.OnReactionAdd)
	discord.AddHandler(reactionHandler.OnReactionRemove)
	discord.AddHandler(newVoiceStateHandler(cfg.VoiceRepo, cfg.GuildID))

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
	signal.Notify(c, os.Interrupt)
	<-c

	return nil
}

func configureDiscordSession(discord *discordgo.Session) {
	if discord == nil {
		return
	}
	// discordgo runs handlers in separate goroutines by default. Voice session
	// logging depends on receiving a user's VoiceStateUpdate events in gateway
	// order, so keep dispatch synchronous and rely on the DB advisory lock for
	// per-user write serialization.
	if discord.Client == nil {
		discord.Client = http.DefaultClient
	}
	if discord.Client.Transport == nil {
		discord.Client.Transport = http.DefaultTransport
	}
	discord.Client.Transport = otelhttp.NewTransport(discord.Client.Transport)
	discord.SyncEvents = true
	discord.Identify.Intents = discordgo.IntentsGuilds |
		discordgo.IntentsGuildMessages |
		discordgo.IntentsGuildMessageReactions |
		discordgo.IntentsGuildVoiceStates
}
