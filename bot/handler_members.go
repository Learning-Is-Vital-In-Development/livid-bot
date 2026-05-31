package bot

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/bwmarrin/discordgo"
	"livid-bot/db"
	"livid-bot/study"
)

func newMembersHandler(studyRepo *db.StudyRepository, memberRepo *db.MemberRepository) func(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate) {
	return func(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate) {
		options := i.ApplicationCommandData().Options
		optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
		for _, opt := range options {
			optionMap[opt.Name] = opt
		}

		opt, ok := optionMap["channel"]
		if !ok {
			respondError(ctx, s, i, "Missing required option: channel.")
			return
		}
		channelID := opt.StringValue()
		logCommand(ctx, i, "start", "members requested channel=%s", channelID)
		st, err := studyRepo.FindByChannelID(ctx, channelID)
		if err != nil {
			slog.Error("failed to find study by channel", "channel_id", channelID, "error", err)
			respondError(ctx, s, i, "No study found for the selected channel.")
			return
		}

		members, err := memberRepo.FindActiveByStudyID(ctx, st.ID)
		if err != nil {
			slog.Error("failed to find members for study", "study_id", st.ID, "study_name", st.Name, "error", err)
			respondError(ctx, s, i, "Failed to load study members.")
			return
		}

		content := buildMembersResponse(st.Name, members)
		if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: content,
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		}, discordgo.WithContext(ctx)); err != nil {
			logCommand(ctx, i, "error", "failed to respond members command: %v", err)
			return
		}
		logCommand(ctx, i, "success", "members returned count=%d study=%s", len(members), st.Name)
	}
}

func newMembersAutocompleteHandler(studyRepo *db.StudyRepository) func(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate) {
	return func(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate) {
		data := i.ApplicationCommandData()
		query := focusedStringOptionValue(data.Options, "channel")
		logCommand(ctx, i, "start", "members autocomplete query=%q", query)

		studies, err := studyRepo.FindAllActive(ctx)
		if err != nil {
			slog.Error("failed to load active studies for members autocomplete", "error", err)
			respondAutocomplete(ctx, s, i, nil)
			return
		}

		choices := buildArchiveStudyAutocompleteChoices(studies, query, archiveAutocompleteMaxChoices)
		respondAutocomplete(ctx, s, i, choices)
		logCommand(ctx, i, "success", "members autocomplete choices=%d", len(choices))
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
