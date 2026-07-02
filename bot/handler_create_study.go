package bot

import (
	"context"
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
	"livid-bot/db"
	"livid-bot/study"
)

func createStudyResources(ctx context.Context, s *discordgo.Session, studyRepo *db.StudyRepository, guildID, branch, name, description string) (study.Study, error) {
	categoryID, channels, err := ensureCategoryID(ctx, s, guildID, "active")
	if err != nil {
		return study.Study{}, fmt.Errorf("ensure active category: %w", err)
	}

	name, err = studyRepo.NextAvailableName(ctx, branch, name)
	if err != nil {
		return study.Study{}, fmt.Errorf("choose study name: %w", err)
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
