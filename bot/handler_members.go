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
			respondError(ctx, s, i, "필수 옵션이 없습니다: channel")
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
			editDeferredError(ctx, s, i, "선택한 채널의 스터디를 찾을 수 없습니다.")
			return
		}

		members, err := memberRepo.FindActiveByStudyID(ctx, st.ID)
		if err != nil {
			slog.Error("failed to find members for study", "study_id", st.ID, "study_name", st.Name, "error", err)
			editDeferredError(ctx, s, i, "스터디 멤버 목록을 불러오지 못했습니다.")
			return
		}

		content := buildMembersMessage(st.Name, members)
		if err := editMembersInteractionResponse(ctx, s, i, content, memberMentionIDs(members)); err != nil {
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

func editMembersInteractionResponse(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate, content string, mentionIDs []string) error {
	_, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Content: &content,
		AllowedMentions: &discordgo.MessageAllowedMentions{
			Users: mentionIDs,
		},
	}, discordgo.WithContext(ctx))
	return err
}

func buildMembersMessage(studyName string, members []study.StudyMember) string {
	if len(members) == 0 {
		return fmt.Sprintf("📚 **%s 멤버**\n등록된 멤버가 없습니다.\n\n조회 기준 · /members", studyName)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "📚 **%s 멤버**\n총 **%d명**\n\n", studyName, len(members))

	for idx, member := range visibleMembers(members) {
		fmt.Fprintf(&b, "%d. %s\n참여일: `%s`\n", idx+1, memberMention(member), member.JoinedAt.Format("2006-01-02"))
	}

	if omitted := len(members) - len(visibleMembers(members)); omitted > 0 {
		fmt.Fprintf(&b, "… %d명이 더 있습니다.\n", omitted)
	}
	b.WriteString("\n조회 기준 · /members")
	return b.String()
}

func visibleMembers(members []study.StudyMember) []study.StudyMember {
	if len(members) > discordEmbedMaxFields {
		return members[:discordEmbedMaxFields-1]
	}
	return members
}

func memberMentionIDs(members []study.StudyMember) []string {
	ids := make([]string, 0, len(members))
	for _, member := range visibleMembers(members) {
		if member.UserID != "" {
			ids = append(ids, member.UserID)
		}
	}
	return ids
}

func memberMention(member study.StudyMember) string {
	if member.UserID != "" {
		return fmt.Sprintf("<@%s>", member.UserID)
	}
	if member.Username != "" {
		return member.Username
	}
	return "알 수 없음"
}
