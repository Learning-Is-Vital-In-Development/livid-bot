package bot

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"livid-bot/db"
	"livid-bot/study"

	"github.com/bwmarrin/discordgo"
)

const minMembersToStart = 3

func newStudyStartHandler(
	studyRepo *db.StudyRepository,
	memberRepo *db.MemberRepository,
	recruitRepo *db.RecruitRepository,
	reactionHandler *ReactionHandler,
) func(s *discordgo.Session, i *discordgo.InteractionCreate) {
	return func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		options := i.ApplicationCommandData().Options
		optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
		for _, opt := range options {
			optionMap[opt.Name] = opt
		}

		opt, ok := optionMap["branch"]
		if !ok {
			respondError(s, i, "Missing required option: branch.")
			return
		}
		branch := opt.StringValue()
		logCommand(i, "start", "study-start requested branch=%s", branch)

		if !isValidBranch(branch) {
			respondError(s, i, fmt.Sprintf("Invalid branch format: %q. Use YY-Q (e.g. 26-2).", branch))
			return
		}

		ctx := context.Background()

		messageIDs, studyInfos, err := recruitRepo.FindOpenMappingsByBranch(ctx, branch)
		if err != nil {
			slog.Error("failed to find open mappings by branch", "branch", branch, "error", err)
			respondError(s, i, "Failed to load recruitment data.")
			return
		}

		if len(studyInfos) == 0 {
			respondError(s, i, fmt.Sprintf("No open recruitments found for branch %q.", branch))
			return
		}

		reactionHandler.Untrack(messageIDs)

		if _, err := recruitRepo.CloseByBranch(ctx, branch); err != nil {
			slog.Error("failed to close recruit messages by branch", "branch", branch, "error", err)
			respondError(s, i, "Failed to close recruitment.")
			return
		}

		var started []string
		var archived []string
		var errors []string

		for _, info := range studyInfos {
			members, err := memberRepo.FindActiveByStudyID(ctx, info.StudyID)
			if err != nil {
				slog.Error("failed to load members for study", "study_id", info.StudyID, "study_name", info.StudyName, "error", err)
				errors = append(errors, fmt.Sprintf("%s: failed to load members", info.StudyName))
				continue
			}

			if len(members) < minMembersToStart {
				archiveResult, archiveErr := archiveStudy(s, studyRepo, i.GuildID, studyToModel(info))
				if archiveErr != nil {
					slog.Error("failed to auto-archive study", "study_id", info.StudyID, "study_name", info.StudyName, "error", archiveErr)
					errors = append(errors, fmt.Sprintf("%s: archive failed", info.StudyName))
					continue
				}

				if _, msgErr := s.ChannelMessageSend(info.ChannelID,
					fmt.Sprintf("모집 인원이 %d명 미만이어서 스터디가 자동 아카이브되었습니다.", minMembersToStart),
				); msgErr != nil {
					slog.Warn("failed to send archive notice to channel", "channel_id", info.ChannelID, "error", msgErr)
				}

				archiveEntry := info.StudyName
				if archiveResult.Warning != "" {
					archiveEntry += " (" + archiveResult.Warning + ")"
				}
				archived = append(archived, archiveEntry)
			} else {
				if _, msgErr := s.ChannelMessageSend(info.ChannelID,
					fmt.Sprintf("<@&%s> 스터디에 오신 것을 환영합니다! 스터디를 진행해주세요.", info.RoleID),
				); msgErr != nil {
					slog.Warn("failed to send start notice to channel", "channel_id", info.ChannelID, "error", msgErr)
				}

				started = append(started, info.StudyName)
			}
		}

		summary := buildStudyStartSummary(started, archived, errors)
		if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: summary,
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		}); err != nil {
			logCommand(i, "error", "failed to respond study-start summary: %v", err)
			return
		}
		logCommand(i, "success", "study-start completed branch=%s started=%d archived=%d errors=%d",
			branch, len(started), len(archived), len(errors))
	}
}

// studyToModel converts RecruitStudyInfo to Study for archiveStudy.
// Only ID, Name, ChannelID, RoleID, and Status are required by archiveStudy.
func studyToModel(info db.RecruitStudyInfo) study.Study {
	return study.Study{
		ID:        info.StudyID,
		Name:      info.StudyName,
		ChannelID: info.ChannelID,
		RoleID:    info.RoleID,
		Status:    "active",
	}
}

func newStudyStartAutocompleteHandler(studyRepo *db.StudyRepository) func(s *discordgo.Session, i *discordgo.InteractionCreate) {
	return func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		ctx := context.Background()
		query := focusedStringOptionValue(i.ApplicationCommandData().Options, "branch")
		logCommand(i, "start", "study-start branch autocomplete query=%q", query)

		branches, err := studyRepo.FindDistinctActiveBranches(ctx)
		if err != nil {
			slog.Error("failed to load active branches for study-start autocomplete", "error", err)
			respondAutocomplete(s, i, nil)
			return
		}

		choices := buildRecruitBranchAutocompleteChoices(branches, query, recruitBranchAutocompleteMaxChoices)
		respondAutocomplete(s, i, choices)
		logCommand(i, "success", "study-start branch autocomplete choices=%d", len(choices))
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
