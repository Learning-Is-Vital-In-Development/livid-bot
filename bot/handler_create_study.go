package bot

import (
	"context"
	"fmt"
	"log"

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

		// Create Discord channel
		channel, err := s.GuildChannelCreate(guildID, name, discordgo.ChannelTypeGuildText)
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
