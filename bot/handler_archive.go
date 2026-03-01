package bot

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/bwmarrin/discordgo"
	"livid-bot/db"
)

const discordMessageLimit = 2000

type archiveFailure struct {
	studyName string
	reason    string
}

func newArchiveStudyHandler(studyRepo *db.StudyRepository) func(s *discordgo.Session, i *discordgo.InteractionCreate) {
	return func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		options := i.ApplicationCommandData().Options
		optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
		for _, opt := range options {
			optionMap[opt.Name] = opt
		}

		name := optionMap["name"].StringValue()
		ctx := context.Background()

		st, err := studyRepo.FindByName(ctx, name)
		if err != nil {
			log.Printf("Failed to find study %q: %v", name, err)
			respondError(s, i, fmt.Sprintf("Study %q not found.", name))
			return
		}

		if st.Status != "active" {
			respondError(s, i, fmt.Sprintf("Study %q is already archived.", name))
			return
		}

		channel, err := s.Channel(st.ChannelID)
		if err != nil {
			log.Printf("Failed to load channel %s for study %q: %v", st.ChannelID, name, err)
			respondError(s, i, "Failed to load study channel. Archive aborted.")
			return
		}
		originalParentID := channel.ParentID

		allocator, err := newArchiveCategoryAllocator(s, i.GuildID)
		if err != nil {
			log.Printf("Failed to prepare archive category allocator: %v", err)
			respondError(s, i, "Failed to prepare archive category.")
			return
		}

		targetCategoryID, targetCategoryName, err := allocator.Reserve()
		if err != nil {
			log.Printf("Failed to reserve archive category: %v", err)
			respondError(s, i, "Failed to prepare archive category.")
			return
		}

		if _, err := s.ChannelEdit(st.ChannelID, &discordgo.ChannelEdit{ParentID: targetCategoryID}); err != nil {
			log.Printf("Failed to move channel %s for study %q to %s: %v", st.ChannelID, name, targetCategoryName, err)
			respondError(s, i, "Failed to move study channel to archive category.")
			return
		}

		if err := studyRepo.Archive(ctx, name); err != nil {
			log.Printf("Failed to archive study %q in DB after move: %v", name, err)
			if rollbackErr := rollbackChannelParent(s, st.ChannelID, originalParentID); rollbackErr != nil {
				log.Printf("Failed to rollback channel %s after DB failure for study %q: %v", st.ChannelID, name, rollbackErr)
				respondError(s, i, "Failed to archive study and rollback failed. Please check channel/category state manually.")
				return
			}
			respondError(s, i, "Failed to archive study. Channel move was rolled back.")
			return
		}

		warning := ""
		if err := s.GuildRoleDelete(i.GuildID, st.RoleID); err != nil {
			log.Printf("Failed to delete role %s for study %q: %v", st.RoleID, name, err)
			warning = "\nWarning: Role deletion failed. Please remove it manually if needed."
		}

		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: fmt.Sprintf("Study **%s** has been archived and moved to **%s**.%s", name, targetCategoryName, warning),
			},
		})
	}
}

func newArchiveAllHandler(studyRepo *db.StudyRepository) func(s *discordgo.Session, i *discordgo.InteractionCreate) {
	return func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		ctx := context.Background()
		options := i.ApplicationCommandData().Options
		optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
		for _, opt := range options {
			optionMap[opt.Name] = opt
		}
		dryRun := false
		if opt, ok := optionMap["dry-run"]; ok {
			dryRun = opt.BoolValue()
		}

		studies, err := studyRepo.FindAllActive(ctx)
		if err != nil {
			log.Printf("Failed to find active studies: %v", err)
			respondError(s, i, "Failed to load active studies.")
			return
		}

		if len(studies) == 0 {
			respondError(s, i, "No active studies to archive.")
			return
		}

		allocator, err := newArchiveCategoryAllocator(s, i.GuildID)
		if err != nil {
			log.Printf("Failed to prepare archive category allocator: %v", err)
			respondError(s, i, "Failed to prepare archive category.")
			return
		}

		if dryRun {
			studyNames := make([]string, len(studies))
			for idx, st := range studies {
				studyNames[idx] = st.Name
			}
			plan := allocator.Plan(len(studies))
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: buildArchiveAllDryRunSummary(studyNames, plan),
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			})
			return
		}

		successCount := 0
		failures := make([]archiveFailure, 0)
		warnings := make([]string, 0)

		for _, st := range studies {
			channel, err := s.Channel(st.ChannelID)
			if err != nil {
				log.Printf("Failed to load channel %s for study %q: %v", st.ChannelID, st.Name, err)
				failures = append(failures, archiveFailure{studyName: st.Name, reason: "channel lookup failed"})
				continue
			}
			originalParentID := channel.ParentID

			targetCategoryID, targetCategoryName, err := allocator.Reserve()
			if err != nil {
				log.Printf("Failed to reserve archive category for study %q: %v", st.Name, err)
				failures = append(failures, archiveFailure{studyName: st.Name, reason: "archive category unavailable"})
				continue
			}

			if _, err := s.ChannelEdit(st.ChannelID, &discordgo.ChannelEdit{ParentID: targetCategoryID}); err != nil {
				log.Printf("Failed to move channel %s for study %q to %s: %v", st.ChannelID, st.Name, targetCategoryName, err)
				failures = append(failures, archiveFailure{studyName: st.Name, reason: "channel move failed"})
				continue
			}

			if err := studyRepo.Archive(ctx, st.Name); err != nil {
				log.Printf("Failed to archive study %q in DB after move: %v", st.Name, err)
				if rollbackErr := rollbackChannelParent(s, st.ChannelID, originalParentID); rollbackErr != nil {
					log.Printf("Failed to rollback channel %s after DB failure for study %q: %v", st.ChannelID, st.Name, rollbackErr)
					warnings = append(warnings, fmt.Sprintf("%s: rollback failed", st.Name))
				}
				failures = append(failures, archiveFailure{studyName: st.Name, reason: "db archive failed"})
				continue
			}

			if err := s.GuildRoleDelete(i.GuildID, st.RoleID); err != nil {
				log.Printf("Failed to delete role %s for study %q: %v", st.RoleID, st.Name, err)
				warnings = append(warnings, fmt.Sprintf("%s: role deletion failed", st.Name))
			}

			successCount++
		}

		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: buildArchiveAllSummary(len(studies), successCount, failures, warnings),
			},
		})
	}
}

func rollbackChannelParent(s *discordgo.Session, channelID, parentID string) error {
	_, err := s.ChannelEdit(channelID, &discordgo.ChannelEdit{ParentID: parentID})
	if err != nil {
		return fmt.Errorf("rollback channel parent: %w", err)
	}
	return nil
}

func buildArchiveAllSummary(total, success int, failures []archiveFailure, warnings []string) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Archived **%d/%d** studies.", success, total))

	if len(failures) > 0 {
		parts := make([]string, 0, len(failures))
		for _, failure := range failures {
			parts = append(parts, fmt.Sprintf("%s (%s)", failure.studyName, failure.reason))
		}
		b.WriteString("\nFailed: ")
		b.WriteString(strings.Join(parts, ", "))
	}

	if len(warnings) > 0 {
		b.WriteString("\nWarnings: ")
		b.WriteString(strings.Join(warnings, ", "))
	}

	return truncateForDiscord(b.String(), discordMessageLimit)
}

func buildArchiveAllDryRunSummary(studyNames []string, plan archiveDryRunPlan) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Dry run: **%d** active studies would be archived. No changes were made.", len(studyNames)))

	if len(plan.CategoryUseCounts) > 0 {
		categoryNames := make([]string, 0, len(plan.CategoryUseCounts))
		for categoryName := range plan.CategoryUseCounts {
			categoryNames = append(categoryNames, categoryName)
		}
		sort.Slice(categoryNames, func(i, j int) bool {
			iNum, iOK := parseArchiveCategoryNumber(categoryNames[i])
			jNum, jOK := parseArchiveCategoryNumber(categoryNames[j])
			if iOK && jOK && iNum != jNum {
				return iNum < jNum
			}
			return categoryNames[i] < categoryNames[j]
		})

		parts := make([]string, 0, len(categoryNames))
		for _, categoryName := range categoryNames {
			parts = append(parts, fmt.Sprintf("%s (%d)", categoryName, plan.CategoryUseCounts[categoryName]))
		}
		b.WriteString("\nPlanned categories: ")
		b.WriteString(strings.Join(parts, ", "))
	}

	if len(plan.CreatedCategories) > 0 {
		b.WriteString("\nWould create: ")
		b.WriteString(strings.Join(plan.CreatedCategories, ", "))
	}

	previewLimit := 10
	if len(studyNames) < previewLimit {
		previewLimit = len(studyNames)
	}
	if previewLimit > 0 && len(plan.Assignments) >= previewLimit {
		b.WriteString("\nPreview:\n")
		for idx := 0; idx < previewLimit; idx++ {
			b.WriteString(fmt.Sprintf("%d. %s -> %s\n", idx+1, studyNames[idx], plan.Assignments[idx]))
		}
		if len(studyNames) > previewLimit {
			b.WriteString(fmt.Sprintf("...and %d more", len(studyNames)-previewLimit))
		}
	}

	return truncateForDiscord(b.String(), discordMessageLimit)
}

func truncateForDiscord(message string, maxLength int) string {
	if utf8.RuneCountInString(message) <= maxLength {
		return message
	}
	if maxLength <= 3 {
		return string([]rune(message)[:maxLength])
	}
	return string([]rune(message)[:maxLength-3]) + "..."
}
