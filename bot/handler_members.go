package bot

import (
	"context"
	"fmt"
	"log/slog"

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
		if !deferInteractionResponse(ctx, s, i, true) {
			return
		}

		st, err := studyRepo.FindByChannelID(ctx, channelID)
		if err != nil {
			slog.Error("failed to find study by channel", "channel_id", channelID, "error", err)
			editDeferredError(ctx, s, i, "No study found for the selected channel.")
			return
		}

		members, err := memberRepo.FindActiveByStudyID(ctx, st.ID)
		if err != nil {
			slog.Error("failed to find members for study", "study_id", st.ID, "study_name", st.Name, "error", err)
			editDeferredError(ctx, s, i, "Failed to load study members.")
			return
		}

		embed := buildMembersEmbed(st.Name, members)
		if err := editOriginalInteractionResponseEmbed(ctx, s, i, embed); err != nil {
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

func buildMembersEmbed(studyName string, members []study.StudyMember) *discordgo.MessageEmbed {
	embed := &discordgo.MessageEmbed{
		Title:       fmt.Sprintf("📚 %s 멤버", studyName),
		Description: fmt.Sprintf("총 **%d명**", len(members)),
		Color:       discordEmbedColorBlurple,
		Footer:      &discordgo.MessageEmbedFooter{Text: "조회 기준 · /members"},
	}
	if len(members) == 0 {
		embed.Description = "등록된 멤버가 없습니다."
		return embed
	}

	visibleMembers := members
	if len(visibleMembers) > discordEmbedMaxFields {
		visibleMembers = members[:discordEmbedMaxFields-1]
	}

	for idx, member := range visibleMembers {
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   fmt.Sprintf("%d.", idx+1),
			Value:  fmt.Sprintf("%s\n참여일: `%s`", memberMention(member), member.JoinedAt.Format("2006-01-02")),
			Inline: true,
		})
	}

	if omitted := len(members) - len(visibleMembers); omitted > 0 {
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   "…",
			Value:  fmt.Sprintf("%d명이 더 있습니다.", omitted),
			Inline: true,
		})
	}

	return embed
}

func memberMention(member study.StudyMember) string {
	if member.UserID != "" {
		return fmt.Sprintf("<@%s>", member.UserID)
	}
	if member.Username != "" {
		return member.Username
	}
	return "unknown"
}
