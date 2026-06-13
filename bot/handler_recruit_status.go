package bot

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/bwmarrin/discordgo"
	"livid-bot/db"
)

func newRecruitStatusHandler(recruitRepo *db.RecruitRepository) func(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate) {
	return func(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate) {
		branch := recruitBranchFromOptions(i.ApplicationCommandData().Options)
		logCommand(ctx, i, "start", "recruit-status requested branch=%s", branch)

		if !isValidBranch(branch) {
			respondError(ctx, s, i, fmt.Sprintf("Invalid branch format: %q. Use YY-Q (e.g. 26-2).", branch))
			return
		}

		mappings, err := recruitRepo.FindOpenRecruitMappingsByBranch(ctx, branch)
		if err != nil {
			slog.Error("failed to find open recruit mappings by branch", "branch", branch, "error", err)
			respondError(ctx, s, i, "Failed to load recruitment data.")
			return
		}

		summaries, err := collectRecruitSignupsFromMappings(ctx, s, mappings, botUserID(s))
		if err != nil {
			slog.Error("failed to collect recruit signup reactions", "branch", branch, "error", err)
			respondError(ctx, s, i, "Failed to collect recruitment reactions.")
			return
		}

		if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: buildRecruitStatusSummary(branch, summaries),
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		}, discordgo.WithContext(ctx)); err != nil {
			logCommand(ctx, i, "error", "failed to respond recruit-status summary: %v", err)
			return
		}
		logCommand(ctx, i, "success", "recruit-status completed branch=%s studies=%d", branch, len(summaries))
	}
}

func recruitBranchFromOptions(options []*discordgo.ApplicationCommandInteractionDataOption) string {
	for _, opt := range options {
		if opt != nil && opt.Name == "branch" {
			return opt.StringValue()
		}
	}
	return ""
}

func botUserID(s *discordgo.Session) string {
	if s == nil || s.State == nil || s.State.User == nil {
		return ""
	}
	return s.State.User.ID
}

func buildRecruitStatusSummary(branch string, summaries []RecruitSignupSummary) string {
	if len(summaries) == 0 {
		return fmt.Sprintf("📊 %s 모집 현황\n\n열린 모집이 없습니다.", branch)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "📊 %s 모집 현황\n", branch)
	fmt.Fprintf(&b, "최소 시작 인원: %d명\n", minMembersToStart)

	for _, summary := range summaries {
		shortage := minMembersToStart - summary.Count
		status := "시작 가능"
		if shortage > 0 {
			status = fmt.Sprintf("%d명 부족", shortage)
		}

		fmt.Fprintf(&b, "\n%s %s\n", summary.Emoji, summary.StudyName)
		fmt.Fprintf(&b, "- 신청: %d명\n", summary.Count)
		fmt.Fprintf(&b, "- 상태: %s\n", status)
	}

	return truncateForDiscord(b.String(), discordMessageLimit)
}
