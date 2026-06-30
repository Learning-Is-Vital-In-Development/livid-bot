package bot

import (
	"context"
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
	"livid-bot/db"
	"livid-bot/study"
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
		if !deferInteractionResponse(ctx, s, i, false) {
			return
		}

		created, err := createStudyResources(ctx, s, studyRepo, i.GuildID, branch, name, description)
		if err != nil {
			logCommand(ctx, i, "error", "failed to create study branch=%s name=%s err=%v", branch, name, err)
			editDeferredError(ctx, s, i, "Failed to create study.")
			return
		}

		if err := editOriginalInteractionResponse(ctx, s, i, fmt.Sprintf("Study **%s** created in branch **%s**!\nChannel: <#%s>\nRole: <@&%s>",
			name, branch, created.ChannelID, created.RoleID)); err != nil {
			logCommand(ctx, i, "error", "failed to respond create-study success: %v", err)
			return
		}
		logCommand(ctx, i, "success", "created study branch=%s name=%s channel=%s role=%s", branch, name, created.ChannelID, created.RoleID)
	}
}

func createStudyResources(ctx context.Context, s *discordgo.Session, studyRepo *db.StudyRepository, guildID, branch, name, description string) (study.Study, error) {
	categoryID, channels, err := ensureCategoryID(ctx, s, guildID, "active")
	if err != nil {
		return study.Study{}, fmt.Errorf("ensure active category: %w", err)
	}

	channelName := uniqueStudyChannelName(buildStudyChannelName(branch, name), channels)
	channel, err := s.GuildChannelCreateComplex(guildID, discordgo.GuildChannelCreateData{
		Name:     channelName,
		Type:     discordgo.ChannelTypeGuildText,
		ParentID: categoryID,
	}, discordgo.WithContext(ctx))
	if err != nil {
		return study.Study{}, fmt.Errorf("create channel %s: %w", channelName, err)
	}

	role, err := s.GuildRoleCreate(guildID, &discordgo.RoleParams{Name: name, Mentionable: boolPtr(false)}, discordgo.WithContext(ctx))
	if err != nil {
		_, _ = s.ChannelDelete(channel.ID, discordgo.WithContext(ctx))
		return study.Study{}, fmt.Errorf("create role %s: %w", name, err)
	}

	created, err := studyRepo.Create(ctx, branch, name, description, channel.ID, role.ID)
	if err != nil {
		_, _ = s.ChannelDelete(channel.ID, discordgo.WithContext(ctx))
		_ = s.GuildRoleDelete(guildID, role.ID, discordgo.WithContext(ctx))
		return study.Study{}, fmt.Errorf("save study: %w", err)
	}
	return created, nil
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
