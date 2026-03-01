package bot

import (
	"context"
	"fmt"
	"log"

	"github.com/bwmarrin/discordgo"
	"livid-bot/db"
)

func newArchiveStudyHandler(studyRepo *db.StudyRepository) func(s *discordgo.Session, i *discordgo.InteractionCreate) {
	return func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		options := i.ApplicationCommandData().Options
		optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
		for _, opt := range options {
			optionMap[opt.Name] = opt
		}

		name := optionMap["name"].StringValue()
		ctx := context.Background()

		// Find study to get role ID before archiving
		st, err := studyRepo.FindByName(ctx, name)
		if err != nil {
			log.Printf("Failed to find study %q: %v", name, err)
			respondError(s, i, fmt.Sprintf("Study %q not found.", name))
			return
		}

		if st.Status != "active" {
			respondError(s, i, fmt.Sprintf("Study %q is already archived.", name))
			return
		}

		// Archive in DB
		if err := studyRepo.Archive(ctx, name); err != nil {
			log.Printf("Failed to archive study %q: %v", name, err)
			respondError(s, i, "Failed to archive study.")
			return
		}

		// Delete Discord role
		if err := s.GuildRoleDelete(i.GuildID, st.RoleID); err != nil {
			log.Printf("Failed to delete role %s for study %q: %v", st.RoleID, name, err)
		}

		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: fmt.Sprintf("Study **%s** has been archived. Role deleted.", name),
			},
		})
	}
}

func newArchiveAllHandler(studyRepo *db.StudyRepository) func(s *discordgo.Session, i *discordgo.InteractionCreate) {
	return func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		ctx := context.Background()

		// Find all active studies to delete their roles
		studies, err := studyRepo.FindAllActive(ctx)
		if err != nil {
			log.Printf("Failed to find active studies: %v", err)
			respondError(s, i, "Failed to load active studies.")
			return
		}

		if len(studies) == 0 {
			respondError(s, i, "No active studies to archive.")
			return
		}

		// Delete each role from Discord
		for _, st := range studies {
			if err := s.GuildRoleDelete(i.GuildID, st.RoleID); err != nil {
				log.Printf("Failed to delete role %s for study %q: %v", st.RoleID, st.Name, err)
			}
		}

		// Archive all in DB
		count, err := studyRepo.ArchiveAll(ctx)
		if err != nil {
			log.Printf("Failed to archive all studies: %v", err)
			respondError(s, i, "Failed to archive studies.")
			return
		}

		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: fmt.Sprintf("Archived **%d** studies. All roles deleted.", count),
			},
		})
	}
}
