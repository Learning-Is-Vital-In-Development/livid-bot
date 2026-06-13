package bot

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"livid-bot/db"

	"github.com/bwmarrin/discordgo"
)

const minMembersToStart = 3

func newRecruitCloseAutocompleteHandler(studyRepo *db.StudyRepository) func(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate) {
	return func(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate) {
		query := focusedStringOptionValue(i.ApplicationCommandData().Options, "branch")
		logCommand(ctx, i, "start", "recruit branch autocomplete query=%q", query)

		branches, err := studyRepo.FindDistinctActiveBranches(ctx)
		if err != nil {
			slog.Error("failed to load active branches for recruit autocomplete", "error", err)
			respondAutocomplete(ctx, s, i, nil)
			return
		}

		choices := buildRecruitBranchAutocompleteChoices(branches, query, recruitBranchAutocompleteMaxChoices)
		respondAutocomplete(ctx, s, i, choices)
		logCommand(ctx, i, "success", "recruit branch autocomplete choices=%d", len(choices))
	}
}

func buildStudyStartSummary(started, archived, errors []string) string {
	var b strings.Builder

	if len(started) > 0 {
		fmt.Fprintf(&b, "Started **%d** studies: %s", len(started), strings.Join(started, ", "))
	}

	if len(archived) > 0 {
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "Archived **%d** studies (< %d members): %s",
			len(archived), minMembersToStart, strings.Join(archived, ", "))
	}

	if len(errors) > 0 {
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "Errors: %s", strings.Join(errors, ", "))
	}

	if b.Len() == 0 {
		b.WriteString("No studies were processed.")
	}

	return truncateForDiscord(b.String(), discordMessageLimit)
}
