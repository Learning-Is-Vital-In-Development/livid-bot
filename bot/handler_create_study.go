package bot

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/bwmarrin/discordgo"
	"livid-bot/db"
)

func newCreateStudyHandler(studyRepo *db.StudyRepository) func(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate) {
	return func(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate) {
		options := i.ApplicationCommandData().Options
		optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
		for _, opt := range options {
			optionMap[opt.Name] = opt
		}

		branch := optionMap["branch"].StringValue()
		if !isValidBranch(branch) {
			respondError(ctx, s, i, "Invalid branch format. Use YY-Q with Q in 1~4 (e.g. 26-2).")
			return
		}

		name := normalizeStudyName(optionMap["name"].StringValue())
		if name == "" {
			respondError(ctx, s, i, "Study name is empty after normalization. Please provide a valid name.")
			return
		}

		description := ""
		if opt, ok := optionMap["description"]; ok {
			description = opt.StringValue()
		}
		logCommand(ctx, i, "start", "create-study requested branch=%s name=%s", branch, name)

		guildID := i.GuildID

		categoryID, channels, err := ensureCategoryID(ctx, s, guildID, "active")
		if err != nil {
			slog.Error("failed to ensure active category", "guild_id", guildID, "error", err)
			respondError(ctx, s, i, "Failed to prepare active category.")
			return
		}

		// Check for duplicate channel name
		channelName := buildStudyChannelName(branch, name)
		for _, ch := range channels {
			if ch.Type == discordgo.ChannelTypeGuildText && ch.Name == channelName {
				logCommand(ctx, i, "duplicate", "channel already exists name=%s channel=%s", channelName, ch.ID)
				respondError(ctx, s, i, fmt.Sprintf("Channel **%s** already exists: <#%s>", channelName, ch.ID))
				return
			}
		}

		// Create Discord channel
		channel, err := s.GuildChannelCreateComplex(guildID, discordgo.GuildChannelCreateData{
			Name:     channelName,
			Type:     discordgo.ChannelTypeGuildText,
			ParentID: categoryID,
		}, discordgo.WithContext(ctx))
		if err != nil {
			logCommand(ctx, i, "error", "failed to create channel name=%s err=%v", channelName, err)
			respondError(ctx, s, i, "Failed to create channel.")
			return
		}

		// Create Discord role
		role, err := s.GuildRoleCreate(guildID, &discordgo.RoleParams{
			Name:        name,
			Mentionable: boolPtr(true),
		}, discordgo.WithContext(ctx))
		if err != nil {
			logCommand(ctx, i, "error", "failed to create role name=%s err=%v", name, err)
			// Compensate: delete the channel we just created
			if _, delErr := s.ChannelDelete(channel.ID, discordgo.WithContext(ctx)); delErr != nil {
				logCommand(ctx, i, "error", "failed to cleanup channel=%s err=%v", channel.ID, delErr)
			}
			respondError(ctx, s, i, "Failed to create role.")
			return
		}

		// Save to DB
		_, err = studyRepo.Create(ctx, branch, name, description, channel.ID, role.ID)
		if err != nil {
			logCommand(ctx, i, "error", "failed to save study branch=%s name=%s err=%v", branch, name, err)
			// Compensate: delete channel and role
			if _, delErr := s.ChannelDelete(channel.ID, discordgo.WithContext(ctx)); delErr != nil {
				logCommand(ctx, i, "error", "failed to cleanup channel=%s err=%v", channel.ID, delErr)
			}
			if delErr := s.GuildRoleDelete(guildID, role.ID, discordgo.WithContext(ctx)); delErr != nil {
				logCommand(ctx, i, "error", "failed to cleanup role=%s err=%v", role.ID, delErr)
			}
			respondError(ctx, s, i, "Failed to save study. Duplicate name in same branch?")
			return
		}

		if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: fmt.Sprintf("Study **%s** created in branch **%s**!\nChannel: <#%s>\nRole: <@&%s>",
					name, branch, channel.ID, role.ID),
			},
		}, discordgo.WithContext(ctx)); err != nil {
			logCommand(ctx, i, "error", "failed to respond create-study success: %v", err)
			return
		}
		logCommand(ctx, i, "success", "created study branch=%s name=%s channel=%s role=%s", branch, name, channel.ID, role.ID)
	}
}

func ensureCategoryID(ctx context.Context, s *discordgo.Session, guildID, categoryName string) (string, []*discordgo.Channel, error) {
	channels, err := s.GuildChannels(guildID, discordgo.WithContext(ctx))
	if err != nil {
		return "", nil, err
	}

	for _, ch := range channels {
		if ch.Type == discordgo.ChannelTypeGuildCategory && strings.EqualFold(ch.Name, categoryName) {
			return ch.ID, channels, nil
		}
	}

	category, err := s.GuildChannelCreateComplex(guildID, discordgo.GuildChannelCreateData{
		Name: categoryName,
		Type: discordgo.ChannelTypeGuildCategory,
	}, discordgo.WithContext(ctx))
	if err != nil {
		// Re-fetch in case another goroutine created it concurrently
		channels, refetchErr := s.GuildChannels(guildID, discordgo.WithContext(ctx))
		if refetchErr != nil {
			return "", nil, err
		}
		for _, ch := range channels {
			if ch.Type == discordgo.ChannelTypeGuildCategory && strings.EqualFold(ch.Name, categoryName) {
				return ch.ID, channels, nil
			}
		}
		return "", nil, err
	}

	return category.ID, channels, nil
}
