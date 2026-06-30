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
		if !deferInteractionResponse(ctx, s, i, false) {
			return
		}

		mappings, err := recruitRepo.FindOpenRecruitMappingsByBranch(ctx, branch)
		if err != nil {
			slog.Error("failed to find open recruit mappings by branch", "branch", branch, "error", err)
			editDeferredError(ctx, s, i, "Failed to load recruitment data.")
			return
		}

		summaries, err := collectRecruitSignupsFromMappings(ctx, s, mappings, botUserID(s))
		if err != nil {
			slog.Error("failed to collect recruit signup reactions", "branch", branch, "error", err)
			editDeferredError(ctx, s, i, "Failed to collect recruitment reactions.")
			return
		}

		if err := editOriginalInteractionResponseEmbed(ctx, s, i, buildRecruitStatusEmbed(branch, summaries)); err != nil {
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

func buildRecruitStatusEmbed(branch string, summaries []RecruitSignupSummary) *discordgo.MessageEmbed {
	embed := &discordgo.MessageEmbed{
		Title:  fmt.Sprintf("📊 %s 모집 현황", branch),
		Color:  recruitStatusEmbedColor(summaries),
		Footer: &discordgo.MessageEmbedFooter{Text: "반응 기준 집계 · /recruit-status"},
	}

	if len(summaries) == 0 {
		embed.Description = "열린 모집이 없습니다."
		return embed
	}

	totalSignups := 0
	readyStudies := 0
	for _, summary := range summaries {
		totalSignups += summary.Count
		if summary.Count >= minMembersToStart {
			readyStudies++
		}
	}
	embed.Description = fmt.Sprintf("최소 시작 인원: **%d명**\n총 신청: **%d명** · 시작 가능: **%d/%d개**",
		minMembersToStart, totalSignups, readyStudies, len(summaries))

	visibleSummaries := summaries
	if len(visibleSummaries) > discordEmbedMaxFields {
		visibleSummaries = summaries[:discordEmbedMaxFields-1]
	}

	for _, summary := range visibleSummaries {
		shortage := minMembersToStart - summary.Count
		status := "✅ 시작 가능"
		if shortage > 0 {
			status = fmt.Sprintf("🟡 %d명 부족", shortage)
			if summary.Count == 0 {
				status = fmt.Sprintf("🔴 %d명 부족", shortage)
			}
		}

		value := fmt.Sprintf("신청: **%d명**\n상태: %s", summary.Count, status)
		if summary.StudyChannelID != "" {
			value += fmt.Sprintf("\n채널: <#%s>", summary.StudyChannelID)
		}

		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   truncateForDiscord(strings.TrimSpace(fmt.Sprintf("%s %s", summary.Emoji, summary.StudyName)), discordEmbedFieldNameLimit),
			Value:  truncateForDiscord(value, discordEmbedFieldValueLimit),
			Inline: false,
		})
	}

	if omitted := len(summaries) - len(visibleSummaries); omitted > 0 {
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   "…",
			Value:  fmt.Sprintf("%d개 스터디가 더 있습니다.", omitted),
			Inline: false,
		})
	}

	return embed
}

func recruitStatusEmbedColor(summaries []RecruitSignupSummary) int {
	if len(summaries) == 0 {
		return discordEmbedColorGray
	}

	allReady := true
	anySignup := false
	for _, summary := range summaries {
		if summary.Count > 0 {
			anySignup = true
		}
		if summary.Count < minMembersToStart {
			allReady = false
		}
	}
	if allReady {
		return discordEmbedColorGreen
	}
	if anySignup {
		return discordEmbedColorYellow
	}
	return discordEmbedColorRed
}
