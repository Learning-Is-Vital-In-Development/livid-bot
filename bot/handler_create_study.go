package bot

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/bwmarrin/discordgo"
	"livid-bot/db"
)

func newCreateStudyHandler(studyRepo *db.StudyRepository) func(s *discordgo.Session, i *discordgo.InteractionCreate) {
	return func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		options := i.ApplicationCommandData().Options
		optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
		for _, opt := range options {
			optionMap[opt.Name] = opt
		}

		branch := optionMap["branch"].StringValue()
		if !isValidBranch(branch) {
			respondError(s, i, "Invalid branch format. Use YY-Q with Q in 1~4 (e.g. 26-2).")
			return
		}

		name := normalizeStudyName(optionMap["name"].StringValue())
		if name == "" {
			respondError(s, i, "Study name is empty after normalization. Please provide a valid name.")
			return
		}

		description := ""
		if opt, ok := optionMap["description"]; ok {
			description = opt.StringValue()
		}
		logCommand(i, "start", "create-study requested branch=%s name=%s", branch, name)

		guildID := i.GuildID

		categoryID, err := ensureCategoryID(s, guildID, "active")
		if err != nil {
			log.Printf("Failed to ensure active category: %v", err)
			respondError(s, i, "Failed to prepare active category.")
			return
		}

		// Create Discord channel
		channelName := buildStudyChannelName(branch, name)
		channel, err := s.GuildChannelCreateComplex(guildID, discordgo.GuildChannelCreateData{
			Name:     channelName,
			Type:     discordgo.ChannelTypeGuildText,
			ParentID: categoryID,
		})
		if err != nil {
			logCommand(i, "error", "failed to create channel name=%s err=%v", channelName, err)
			respondError(s, i, "Failed to create channel.")
			return
		}

		// Create Discord role
		role, err := s.GuildRoleCreate(guildID, &discordgo.RoleParams{
			Name:        name,
			Mentionable: boolPtr(true),
		})
		if err != nil {
			logCommand(i, "error", "failed to create role name=%s err=%v", name, err)
			// Compensate: delete the channel we just created
			if _, delErr := s.ChannelDelete(channel.ID); delErr != nil {
				logCommand(i, "error", "failed to cleanup channel=%s err=%v", channel.ID, delErr)
			}
			respondError(s, i, "Failed to create role.")
			return
		}

		// Save to DB
		ctx := context.Background()
		_, err = studyRepo.Create(ctx, branch, name, description, channel.ID, role.ID)
		if err != nil {
			logCommand(i, "error", "failed to save study branch=%s name=%s err=%v", branch, name, err)
			// Compensate: delete channel and role
			if _, delErr := s.ChannelDelete(channel.ID); delErr != nil {
				logCommand(i, "error", "failed to cleanup channel=%s err=%v", channel.ID, delErr)
			}
			if delErr := s.GuildRoleDelete(guildID, role.ID); delErr != nil {
				logCommand(i, "error", "failed to cleanup role=%s err=%v", role.ID, delErr)
			}
			respondError(s, i, "Failed to save study. Duplicate name in same branch?")
			return
		}

		if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: fmt.Sprintf("Study **%s** created in branch **%s**!\nChannel: <#%s>\nRole: <@&%s>",
					name, branch, channel.ID, role.ID),
			},
		}); err != nil {
			logCommand(i, "error", "failed to respond create-study success: %v", err)
			return
		}
		logCommand(i, "success", "created study branch=%s name=%s channel=%s role=%s", branch, name, channel.ID, role.ID)
	}
}

func ensureCategoryID(s *discordgo.Session, guildID, categoryName string) (string, error) {
	channels, err := s.GuildChannels(guildID)
	if err != nil {
		return "", err
	}

	for _, ch := range channels {
		if ch.Type == discordgo.ChannelTypeGuildCategory && strings.EqualFold(ch.Name, categoryName) {
			return ch.ID, nil
		}
	}

	category, err := s.GuildChannelCreateComplex(guildID, discordgo.GuildChannelCreateData{
		Name: categoryName,
		Type: discordgo.ChannelTypeGuildCategory,
	})
	if err != nil {
		// Re-fetch in case another goroutine created it concurrently
		channels, refetchErr := s.GuildChannels(guildID)
		if refetchErr != nil {
			return "", err
		}
		for _, ch := range channels {
			if ch.Type == discordgo.ChannelTypeGuildCategory && strings.EqualFold(ch.Name, categoryName) {
				return ch.ID, nil
			}
		}
		return "", err
	}

	return category.ID, nil
}
