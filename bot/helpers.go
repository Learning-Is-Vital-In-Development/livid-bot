package bot

import (
	"fmt"
	"log/slog"

	"github.com/bwmarrin/discordgo"
)

func respondError(s *discordgo.Session, i *discordgo.InteractionCreate, message string) {
	logCommand(i, "error", "%s", message)
	if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: message,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	}); err != nil {
		logCommand(i, "error", "failed to respond error message: %v", err)
	}
}

func respondAutocomplete(s *discordgo.Session, i *discordgo.InteractionCreate, choices []*discordgo.ApplicationCommandOptionChoice) {
	if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionApplicationCommandAutocompleteResult,
		Data: &discordgo.InteractionResponseData{
			Choices: choices,
		},
	}); err != nil {
		logCommand(i, "error", "failed to respond autocomplete: %v", err)
	}
}

func boolPtr(v bool) *bool {
	return &v
}

func int64Ptr(v int64) *int64 {
	return &v
}

func logCommand(i *discordgo.InteractionCreate, stage, format string, args ...interface{}) {
	commandName := interactionCommandName(i)
	guildID := "unknown"
	userID := "unknown"
	if i != nil {
		if i.GuildID != "" {
			guildID = i.GuildID
		}
		userID = interactionUserID(i)
	}

	message := "command event"
	if format == "" {
		format = message
	}
	message = fmt.Sprintf(format, args...)

	attrs := []any{
		"cmd", commandName,
		"stage", stage,
		"guild", guildID,
		"user", userID,
	}

	if stage == "error" {
		slog.Error(message, attrs...)
		return
	}
	slog.Info(message, attrs...)
}

func interactionCommandName(i *discordgo.InteractionCreate) string {
	if i == nil {
		return "unknown"
	}
	if i.Type != discordgo.InteractionApplicationCommand && i.Type != discordgo.InteractionApplicationCommandAutocomplete {
		return "unknown"
	}
	data := i.ApplicationCommandData()
	if data.Name == "" {
		return "unknown"
	}
	return data.Name
}

func interactionUserID(i *discordgo.InteractionCreate) string {
	if i == nil {
		return "unknown"
	}
	if i.Member != nil && i.Member.User != nil && i.Member.User.ID != "" {
		return i.Member.User.ID
	}
	if i.User != nil && i.User.ID != "" {
		return i.User.ID
	}
	return "unknown"
}
