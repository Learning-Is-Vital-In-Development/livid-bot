package bot

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/bwmarrin/discordgo"
	"livid-bot/db"
	"livid-bot/study"
)

func newMembersHandler(studyRepo *db.StudyRepository, memberRepo *db.MemberRepository) func(s *discordgo.Session, i *discordgo.InteractionCreate) {
	return func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		options := i.ApplicationCommandData().Options
		optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
		for _, opt := range options {
			optionMap[opt.Name] = opt
		}

		opt, ok := optionMap["channel"]
		if !ok {
			respondError(s, i, "Missing required option: channel.")
			return
		}
		channelID := opt.StringValue()
		logCommand(i, "start", "members requested channel=%s", channelID)
		ctx := context.Background()

		st, err := studyRepo.FindByChannelID(ctx, channelID)
		if err != nil {
			log.Printf("Failed to find study by channel %q: %v", channelID, err)
			respondError(s, i, "No study found for the selected channel.")
			return
		}

		members, err := memberRepo.FindActiveByStudyID(ctx, st.ID)
		if err != nil {
			log.Printf("Failed to find members for study %q: %v", st.Name, err)
			respondError(s, i, "Failed to load study members.")
			return
		}

		content := buildMembersResponse(st.Name, members)
		if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: content,
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		}); err != nil {
			logCommand(i, "error", "failed to respond members command: %v", err)
			return
		}
		logCommand(i, "success", "members returned count=%d study=%s", len(members), st.Name)
	}
}

func newMembersAutocompleteHandler(studyRepo *db.StudyRepository) func(s *discordgo.Session, i *discordgo.InteractionCreate) {
	return func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		ctx := context.Background()
		data := i.ApplicationCommandData()
		query := focusedStringOptionValue(data.Options, "channel")
		logCommand(i, "start", "members autocomplete query=%q", query)

		studies, err := studyRepo.FindAllActive(ctx)
		if err != nil {
			log.Printf("Failed to load active studies for members autocomplete: %v", err)
			respondAutocomplete(s, i, nil)
			return
		}

		choices := buildArchiveStudyAutocompleteChoices(studies, query, archiveAutocompleteMaxChoices)
		respondAutocomplete(s, i, choices)
		logCommand(i, "success", "members autocomplete choices=%d", len(choices))
	}
}

func buildMembersResponse(studyName string, members []study.StudyMember) string {
	if len(members) == 0 {
		return fmt.Sprintf("No members found for study **%s**.", studyName)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "📚 **%s** members (%d)\n", studyName, len(members))

	for _, m := range members {
		joinedDate := m.JoinedAt.Format("2006-01-02")
		fmt.Fprintf(&b, "- <@%s> (joined: %s)\n", m.UserID, joinedDate)
	}

	return truncateForDiscord(strings.TrimSuffix(b.String(), "\n"), discordMessageLimit)
}
