package bot

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/bwmarrin/discordgo"
	"livid-bot/db"
	"livid-bot/study"
)

const discordMessageLimit = 2000
const archiveAutocompleteMaxChoices = 25
const archiveAutocompleteChoiceNameLimit = 100

type archiveFailure struct {
	studyName string
	reason    string
}

type archiveResult struct {
	CategoryName string
	Warning      string
}

func archiveStudy(ctx context.Context, s *discordgo.Session, studyRepo *db.StudyRepository, guildID string, st study.Study) (archiveResult, error) {
	channel, err := s.Channel(st.ChannelID, discordgo.WithContext(ctx))
	if err != nil {
		return archiveResult{}, fmt.Errorf("load channel %s for study %q: %w", st.ChannelID, st.Name, err)
	}
	originalParentID := channel.ParentID

	allocator, err := newArchiveCategoryAllocator(ctx, s, guildID)
	if err != nil {
		return archiveResult{}, fmt.Errorf("prepare archive category allocator: %w", err)
	}

	targetCategoryID, targetCategoryName, reservation, err := allocator.Reserve()
	if err != nil {
		return archiveResult{}, fmt.Errorf("reserve archive category: %w", err)
	}

	if _, err := s.ChannelEdit(st.ChannelID, &discordgo.ChannelEdit{ParentID: targetCategoryID}, discordgo.WithContext(ctx)); err != nil {
		return archiveResult{}, fmt.Errorf("move channel %s to %s: %w", st.ChannelID, targetCategoryName, err)
	}
	reservation.Commit()

	if err := studyRepo.ArchiveByID(ctx, st.ID); err != nil {
		if rollbackErr := rollbackChannelParent(ctx, s, st.ChannelID, originalParentID); rollbackErr != nil {
			slog.Error("failed to rollback channel after DB failure", "channel_id", st.ChannelID, "study_name", st.Name, "error", rollbackErr)
			return archiveResult{}, fmt.Errorf("archive study %q in DB (rollback also failed): %w", st.Name, err)
		}
		reservation.Release()
		return archiveResult{}, fmt.Errorf("archive study %q in DB (channel rolled back): %w", st.Name, err)
	}

	warning := ""
	if err := s.GuildRoleDelete(guildID, st.RoleID, discordgo.WithContext(ctx)); err != nil {
		slog.Warn("failed to delete role for archived study", "role_id", st.RoleID, "study_name", st.Name, "error", err)
		warning = "role deletion failed"
	}

	return archiveResult{CategoryName: targetCategoryName, Warning: warning}, nil
}

func newArchiveStudyHandler(studyRepo *db.StudyRepository) func(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate) {
	return func(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate) {
		options := i.ApplicationCommandData().Options
		optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
		for _, opt := range options {
			optionMap[opt.Name] = opt
		}

		channelID := optionMap["channel"].StringValue()
		logCommand(ctx, i, "start", "archive-study requested channel=%s", channelID)
		if !deferInteractionResponse(ctx, s, i, false) {
			return
		}

		st, err := studyRepo.FindByChannelID(ctx, channelID)
		if err != nil {
			slog.Error("failed to find study by channel", "channel_id", channelID, "error", err)
			editDeferredError(ctx, s, i, "No study found for the selected channel.")
			return
		}

		if st.Status != "active" {
			editDeferredError(ctx, s, i, fmt.Sprintf("Study %q is already archived.", st.Name))
			return
		}

		result, err := archiveStudy(ctx, s, studyRepo, i.GuildID, st)
		if err != nil {
			slog.Error("failed to archive study", "study_id", st.ID, "study_name", st.Name, "error", err)
			editDeferredError(ctx, s, i, fmt.Sprintf("Failed to archive study: %v", err))
			return
		}

		warning := ""
		if result.Warning != "" {
			warning = fmt.Sprintf("\nWarning: %s. Please remove it manually if needed.", result.Warning)
		}

		if err := editOriginalInteractionResponse(ctx, s, i, fmt.Sprintf("Study **%s** has been archived and moved to **%s**.%s", st.Name, result.CategoryName, warning)); err != nil {
			logCommand(ctx, i, "error", "failed to respond archive-study success: %v", err)
			return
		}
		logCommand(ctx, i, "success", "archived study id=%d name=%s channel=%s category=%s", st.ID, st.Name, st.ChannelID, result.CategoryName)
	}
}

func newArchiveStudyAutocompleteHandler(studyRepo *db.StudyRepository) func(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate) {
	return func(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate) {
		data := i.ApplicationCommandData()
		query := focusedStringOptionValue(data.Options, "channel")
		logCommand(ctx, i, "start", "archive-study autocomplete query=%q", query)

		studies, err := studyRepo.FindAllActive(ctx)
		if err != nil {
			slog.Error("failed to load active studies for archive autocomplete", "error", err)
			respondAutocomplete(ctx, s, i, nil)
			return
		}

		choices := buildArchiveStudyAutocompleteChoices(studies, query, archiveAutocompleteMaxChoices)
		respondAutocomplete(ctx, s, i, choices)
		logCommand(ctx, i, "success", "archive-study autocomplete choices=%d", len(choices))
	}
}

func newArchiveAllHandler(studyRepo *db.StudyRepository) func(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate) {
	return func(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate) {
		options := i.ApplicationCommandData().Options
		optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
		for _, opt := range options {
			optionMap[opt.Name] = opt
		}
		dryRun := false
		if opt, ok := optionMap["dry-run"]; ok {
			dryRun = opt.BoolValue()
		}
		logCommand(ctx, i, "start", "archive-all requested dry_run=%t", dryRun)
		if !deferInteractionResponse(ctx, s, i, dryRun) {
			return
		}

		studies, err := studyRepo.FindAllActive(ctx)
		if err != nil {
			slog.Error("failed to find active studies", "error", err)
			editDeferredError(ctx, s, i, "Failed to load active studies.")
			return
		}

		if len(studies) == 0 {
			editDeferredError(ctx, s, i, "No active studies to archive.")
			return
		}

		if dryRun {
			allocator, err := newArchiveCategoryAllocator(ctx, s, i.GuildID)
			if err != nil {
				slog.Error("failed to prepare archive category allocator", "error", err)
				editDeferredError(ctx, s, i, "Failed to prepare archive category.")
				return
			}
			studyNames := make([]string, len(studies))
			for idx, st := range studies {
				studyNames[idx] = st.Name
			}
			plan := allocator.Plan(len(studies))
			if err := editOriginalInteractionResponse(ctx, s, i, buildArchiveAllDryRunSummary(studyNames, plan)); err != nil {
				logCommand(ctx, i, "error", "failed to respond archive-all dry-run: %v", err)
				return
			}
			logCommand(ctx, i, "success", "archive-all dry-run studies=%d planned_categories=%d", len(studies), len(plan.CategoryUseCounts))
			return
		}

		successCount := 0
		failures := make([]archiveFailure, 0)
		warnings := make([]string, 0)

		for _, st := range studies {
			result, err := archiveStudy(ctx, s, studyRepo, i.GuildID, st)
			if err != nil {
				slog.Error("failed to archive study", "study_id", st.ID, "study_name", st.Name, "error", err)
				failures = append(failures, archiveFailure{studyName: st.Name, reason: err.Error()})
				continue
			}

			if result.Warning != "" {
				warnings = append(warnings, fmt.Sprintf("%s: %s", st.Name, result.Warning))
			}

			successCount++
		}

		if err := editOriginalInteractionResponse(ctx, s, i, buildArchiveAllSummary(len(studies), successCount, failures, warnings)); err != nil {
			logCommand(ctx, i, "error", "failed to respond archive-all summary: %v", err)
			return
		}
		logCommand(ctx, i, "success", "archive-all completed total=%d success=%d failures=%d warnings=%d", len(studies), successCount, len(failures), len(warnings))
	}
}

func rollbackChannelParent(ctx context.Context, s *discordgo.Session, channelID, parentID string) error {
	_, err := s.ChannelEdit(channelID, &discordgo.ChannelEdit{ParentID: parentID}, discordgo.WithContext(ctx))
	if err != nil {
		return fmt.Errorf("rollback channel parent: %w", err)
	}
	return nil
}

func buildArchiveAllSummary(total, success int, failures []archiveFailure, warnings []string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Archived **%d/%d** studies.", success, total)

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
	fmt.Fprintf(&b, "Dry run: **%d** active studies would be archived. No changes were made.", len(studyNames))

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
			fmt.Fprintf(&b, "%d. %s -> %s\n", idx+1, studyNames[idx], plan.Assignments[idx])
		}
		if len(studyNames) > previewLimit {
			fmt.Fprintf(&b, "...and %d more", len(studyNames)-previewLimit)
		}
	}

	return truncateForDiscord(b.String(), discordMessageLimit)
}

func focusedStringOptionValue(options []*discordgo.ApplicationCommandInteractionDataOption, optionName string) string {
	for _, opt := range options {
		if opt.Name == optionName && opt.Focused {
			return opt.StringValue()
		}
	}
	return ""
}

func buildArchiveStudyAutocompleteChoices(studies []study.Study, query string, limit int) []*discordgo.ApplicationCommandOptionChoice {
	if limit <= 0 {
		return nil
	}

	normalizedQuery := strings.ToLower(strings.TrimSpace(query))
	choices := make([]*discordgo.ApplicationCommandOptionChoice, 0, min(limit, len(studies)))
	for _, st := range studies {
		if normalizedQuery != "" {
			target := strings.ToLower(st.Name + " " + st.ChannelID)
			if !strings.Contains(target, normalizedQuery) {
				continue
			}
		}
		choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
			Name:  buildArchiveAutocompleteChoiceName(st.Name, st.ChannelID),
			Value: st.ChannelID,
		})
		if len(choices) >= limit {
			break
		}
	}
	return choices
}

func buildArchiveAutocompleteChoiceName(studyName, channelID string) string {
	suffix := fmt.Sprintf(" (<#%s>)", channelID)
	maxStudyNameLength := archiveAutocompleteChoiceNameLimit - utf8.RuneCountInString(suffix)
	if maxStudyNameLength <= 0 {
		return truncateForDiscord(suffix, archiveAutocompleteChoiceNameLimit)
	}
	return truncateForDiscord(studyName, maxStudyNameLength) + suffix
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
