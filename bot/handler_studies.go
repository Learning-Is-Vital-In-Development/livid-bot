package bot

import (
	"context"
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
	"livid-bot/db"
	"livid-bot/study"
)

func newStudiesHandler(studyRepo *db.StudyRepository) func(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate) {
	return func(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate) {
		branch, status := parseStudiesFilters(i.ApplicationCommandData().Options)
		logCommand(ctx, i, "start", "studies requested branch=%q status=%q", branch, status)

		if branch != "" && !isValidBranch(branch) {
			respondError(ctx, s, i, "Invalid branch format. Use YY-Q with Q in 1~4 (e.g. 26-2).")
			return
		}

		if status != "active" && status != "archived" {
			respondError(ctx, s, i, "Invalid status. Use one of: active, archived.")
			return
		}
		if !deferInteractionResponse(ctx, s, i, true) {
			return
		}

		studies, err := studyRepo.FindByFilters(ctx, branch, status)
		if err != nil {
			logCommand(ctx, i, "error", "failed to load studies branch=%q status=%q err=%v", branch, status, err)
			editDeferredError(ctx, s, i, "Failed to load studies.")
			return
		}

		embed := buildStudiesEmbed(branch, status, studies)
		if err := editOriginalInteractionResponseEmbed(ctx, s, i, embed); err != nil {
			logCommand(ctx, i, "error", "failed to respond studies command: %v", err)
			return
		}
		logCommand(ctx, i, "success", "studies returned count=%d branch=%q status=%q", len(studies), branch, status)
	}
}

func parseStudiesFilters(options []*discordgo.ApplicationCommandInteractionDataOption) (branch, status string) {
	status = "active"
	for _, opt := range options {
		switch opt.Name {
		case "branch":
			branch = strings.TrimSpace(opt.StringValue())
		case "status":
			status = strings.TrimSpace(opt.StringValue())
		}
	}
	if status == "" {
		status = "active"
	}
	return branch, status
}

func buildStudiesEmbed(branch, status string, studies []study.Study) *discordgo.MessageEmbed {
	branchLabel := "전체"
	if branch != "" {
		branchLabel = branch
	}

	embed := &discordgo.MessageEmbed{
		Title:       "📚 스터디 목록",
		Description: fmt.Sprintf("상태: **%s**\n분기: **%s**", status, branchLabel),
		Color:       studiesEmbedColor(status),
		Footer:      &discordgo.MessageEmbedFooter{Text: "조회 기준 · /studies"},
	}
	if len(studies) == 0 {
		embed.Description += "\n\n조건에 맞는 스터디가 없습니다."
		return embed
	}

	visibleStudies := studies
	if len(visibleStudies) > discordEmbedMaxFields {
		visibleStudies = studies[:discordEmbedMaxFields-1]
	}

	for _, st := range visibleStudies {
		value := fmt.Sprintf("상태: `%s`", st.Status)
		if st.ChannelID != "" {
			value += fmt.Sprintf("\n채널: <#%s>", st.ChannelID)
		}
		if st.RoleID != "" {
			value += fmt.Sprintf("\n역할: <@&%s>", st.RoleID)
		}
		if st.Description != "" {
			value += fmt.Sprintf("\n설명: %s", st.Description)
		}

		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   truncateForDiscord(fmt.Sprintf("[%s] %s", st.Branch, st.Name), discordEmbedFieldNameLimit),
			Value:  truncateForDiscord(value, discordEmbedFieldValueLimit),
			Inline: false,
		})
	}

	if omitted := len(studies) - len(visibleStudies); omitted > 0 {
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   "…",
			Value:  fmt.Sprintf("%d개 스터디가 더 있습니다.", omitted),
			Inline: false,
		})
	}

	return embed
}

func studiesEmbedColor(status string) int {
	if status == "archived" {
		return discordEmbedColorGray
	}
	return discordEmbedColorBlurple
}
