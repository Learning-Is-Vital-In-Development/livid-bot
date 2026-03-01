package bot

import (
	"context"
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
	"livid-bot/db"
	"livid-bot/study"
)

func newStudiesHandler(studyRepo *db.StudyRepository) func(s *discordgo.Session, i *discordgo.InteractionCreate) {
	return func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		branch, status := parseStudiesFilters(i.ApplicationCommandData().Options)
		logCommand(i, "start", "studies requested branch=%q status=%q", branch, status)

		if branch != "" && !isValidBranch(branch) {
			respondError(s, i, "Invalid branch format. Use YY-Q with Q in 1~4 (e.g. 26-2).")
			return
		}

		if status != "active" && status != "archived" {
			respondError(s, i, "Invalid status. Use one of: active, archived.")
			return
		}

		ctx := context.Background()
		studies, err := studyRepo.FindByFilters(ctx, branch, status)
		if err != nil {
			logCommand(i, "error", "failed to load studies branch=%q status=%q err=%v", branch, status, err)
			respondError(s, i, "Failed to load studies.")
			return
		}

		content := buildStudiesResponse(branch, status, studies)
		if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: content,
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		}); err != nil {
			logCommand(i, "error", "failed to respond studies command: %v", err)
			return
		}
		logCommand(i, "success", "studies returned count=%d branch=%q status=%q", len(studies), branch, status)
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

func buildStudiesResponse(branch, status string, studies []study.Study) string {
	if len(studies) == 0 {
		return "No studies found for the provided filters."
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("Studies (status=%s", status))
	if branch != "" {
		b.WriteString(fmt.Sprintf(", branch=%s", branch))
	}
	b.WriteString("):\n")

	for _, st := range studies {
		b.WriteString(fmt.Sprintf("- [%s] %s (%s) <#%s>\n", st.Branch, st.Name, st.Status, st.ChannelID))
	}

	return truncateForDiscord(strings.TrimSuffix(b.String(), "\n"), discordMessageLimit)
}
