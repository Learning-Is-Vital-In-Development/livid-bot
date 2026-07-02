package bot

import (
	"context"
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
	"livid-bot/db"
)

const defaultStudyNudgeAnnouncementChannelID = "1244507245136314369"

type studyNudgeStore interface {
	ListOpenSuggestionsForNudge(ctx context.Context) ([]*db.StudySuggestion, error)
}

func newStudyNudgeHandler(suggestionRepo studyNudgeStore, announcementChannelID string) func(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate) {
	announcementChannelID = strings.TrimSpace(announcementChannelID)
	if announcementChannelID == "" {
		announcementChannelID = defaultStudyNudgeAnnouncementChannelID
	}

	return func(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate) {
		logCommand(ctx, i, "start", "study-nudge requested")
		if !deferInteractionResponse(ctx, s, i, true) {
			return
		}

		suggestions, err := suggestionRepo.ListOpenSuggestionsForNudge(ctx)
		if err != nil {
			logCommand(ctx, i, "error", "failed to list open suggestions: %v", err)
			editDeferredError(ctx, s, i, "공지할 스터디 제안 조회에 실패했습니다.")
			return
		}
		if len(suggestions) == 0 {
			editDeferredError(ctx, s, i, "공지할 open 상태의 스터디 제안이 없습니다.")
			return
		}

		content := buildStudyNudgeMessage(i.GuildID, suggestions)
		if _, err := s.ChannelMessageSendComplex(announcementChannelID, &discordgo.MessageSend{
			Content: content,
			AllowedMentions: &discordgo.MessageAllowedMentions{
				Parse: []discordgo.AllowedMentionType{discordgo.AllowedMentionTypeEveryone},
			},
		}, discordgo.WithContext(ctx)); err != nil {
			logCommand(ctx, i, "error", "failed to send study nudge: %v", err)
			editDeferredError(ctx, s, i, "공지 채널에 스터디 제안 알림을 보내지 못했습니다.")
			return
		}

		if err := editOriginalInteractionResponse(ctx, s, i, fmt.Sprintf("공지 채널에 open 스터디 제안 %d개를 알렸습니다.", len(suggestions))); err != nil {
			logCommand(ctx, i, "error", "failed to respond study-nudge success: %v", err)
		}
	}
}

func buildStudyNudgeMessage(guildID string, suggestions []*db.StudySuggestion) string {
	var b strings.Builder
	b.WriteString("@everyone\n\n📬 참여를 기다리는 스터디 제안이 있습니다.\n\n")
	b.WriteString("참여하고 싶은 스터디가 있다면 원 제안 글에서 🚀를 눌러주세요.\n")
	b.WriteString("🚀는 실제 참여 의사로 집계됩니다.\n\n")
	for _, suggestion := range suggestions {
		if suggestion == nil {
			continue
		}
		fmt.Fprintf(&b, "- **%s**\n", suggestion.Title)
		fmt.Fprintf(&b, "  - 현재: 🚀 %d / %d\n", suggestion.VoteCount, suggestion.Threshold)
		fmt.Fprintf(&b, "  - 마감: %s\n", suggestionDateLabel(suggestion.ExpiresAt))
		fmt.Fprintf(&b, "  - 제안 보기: https://discord.com/channels/%s/%s/%s\n", guildID, suggestion.ChannelID, suggestion.MessageID)
	}
	return b.String()
}
