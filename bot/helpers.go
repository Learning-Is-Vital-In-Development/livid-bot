package bot

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/bwmarrin/discordgo"
)

const (
	discordEmbedColorBlurple = 0x5865F2
	discordEmbedColorGreen   = 0x2ECC71
	discordEmbedColorYellow  = 0xF1C40F
	discordEmbedColorRed     = 0xE74C3C
	discordEmbedColorGray    = 0x95A5A6

	discordEmbedMaxFields       = 25
	discordEmbedFieldNameLimit  = 256
	discordEmbedFieldValueLimit = 1024
)

func respondError(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate, message string) {
	logCommand(ctx, i, "error", "%s", message)
	if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: message,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	}, discordgo.WithContext(ctx)); err != nil {
		logCommand(ctx, i, "error", "failed to respond error message: %v", err)
	}
}

func deferInteractionResponse(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate, ephemeral bool) bool {
	data := &discordgo.InteractionResponseData{
		AllowedMentions: &discordgo.MessageAllowedMentions{},
	}
	if ephemeral {
		data.Flags = discordgo.MessageFlagsEphemeral
	}

	if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: data,
	}, discordgo.WithContext(ctx)); err != nil {
		logCommand(ctx, i, "error", "failed to defer interaction response: %v", err)
		return false
	}
	return true
}

func editOriginalInteractionResponse(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate, content string) error {
	_, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Content:         &content,
		AllowedMentions: &discordgo.MessageAllowedMentions{},
	}, discordgo.WithContext(ctx))
	return err
}

func editOriginalInteractionResponseEmbed(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate, embed *discordgo.MessageEmbed) error {
	content := ""
	embeds := []*discordgo.MessageEmbed{}
	if embed != nil {
		embeds = append(embeds, embed)
	}
	_, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Content:         &content,
		Embeds:          &embeds,
		AllowedMentions: &discordgo.MessageAllowedMentions{},
	}, discordgo.WithContext(ctx))
	return err
}

func editDeferredError(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate, message string) {
	logCommand(ctx, i, "error", "%s", message)
	if err := editOriginalInteractionResponse(ctx, s, i, message); err != nil {
		logCommand(ctx, i, "error", "failed to edit deferred error response: %v", err)
	}
}

func respondAutocomplete(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate, choices []*discordgo.ApplicationCommandOptionChoice) {
	if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionApplicationCommandAutocompleteResult,
		Data: &discordgo.InteractionResponseData{
			Choices: choices,
		},
	}, discordgo.WithContext(ctx)); err != nil {
		logCommand(ctx, i, "error", "failed to respond autocomplete: %v", err)
	}
}

func boolPtr(v bool) *bool {
	return &v
}

func int64Ptr(v int64) *int64 {
	return &v
}

func float64Ptr(v float64) *float64 {
	return &v
}

func logCommand(ctx context.Context, i *discordgo.InteractionCreate, stage, format string, args ...interface{}) {
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
	attrs = append(attrs, traceLogAttrs(ctx)...)

	if stage == "error" {
		slog.Error(message, attrs...)
		recordCommandResult(ctx, i, stage, message)
		return
	}
	slog.Info(message, attrs...)
	recordCommandResult(ctx, i, stage, message)
}

func interactionCommandName(i *discordgo.InteractionCreate) string {
	if i == nil {
		return "unknown"
	}
	if i.Type == discordgo.InteractionModalSubmit {
		data := i.ModalSubmitData()
		if data.CustomID != "" {
			return data.CustomID
		}
		return "modal_submit"
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
