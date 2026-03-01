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

		name := optionMap["name"].StringValue()
		description := ""
		if opt, ok := optionMap["description"]; ok {
			description = opt.StringValue()
		}

		guildID := i.GuildID

		categoryID, err := ensureCategoryID(s, guildID, "active")
		if err != nil {
			log.Printf("Failed to ensure active category: %v", err)
			respondError(s, i, "Failed to prepare active category.")
			return
		}

		// Create Discord channel
		channel, err := s.GuildChannelCreateComplex(guildID, discordgo.GuildChannelCreateData{
			Name:     name,
			Type:     discordgo.ChannelTypeGuildText,
			ParentID: categoryID,
		})
		if err != nil {
			log.Printf("Failed to create channel: %v", err)
			respondError(s, i, "Failed to create channel.")
			return
		}

		// Create Discord role
		role, err := s.GuildRoleCreate(guildID, &discordgo.RoleParams{
			Name:        name,
			Mentionable: boolPtr(true),
		})
		if err != nil {
			log.Printf("Failed to create role: %v", err)
			// Compensate: delete the channel we just created
			if _, delErr := s.ChannelDelete(channel.ID); delErr != nil {
				log.Printf("Failed to clean up channel %s: %v", channel.ID, delErr)
			}
			respondError(s, i, "Failed to create role.")
			return
		}

		// Save to DB
		ctx := context.Background()
		_, err = studyRepo.Create(ctx, name, description, channel.ID, role.ID)
		if err != nil {
			log.Printf("Failed to save study to DB: %v", err)
			// Compensate: delete channel and role
			if _, delErr := s.ChannelDelete(channel.ID); delErr != nil {
				log.Printf("Failed to clean up channel %s: %v", channel.ID, delErr)
			}
			if delErr := s.GuildRoleDelete(guildID, role.ID); delErr != nil {
				log.Printf("Failed to clean up role %s: %v", role.ID, delErr)
			}
			respondError(s, i, "Failed to save study. Duplicate name?")
			return
		}

		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: fmt.Sprintf("Study **%s** created!\nChannel: <#%s>\nRole: <@&%s>",
					name, channel.ID, role.ID),
			},
		})
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
		return "", err
	}

	return category.ID, nil
}
