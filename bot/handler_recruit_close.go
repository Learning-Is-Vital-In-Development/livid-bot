package bot

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/bwmarrin/discordgo"
	"livid-bot/db"
	"livid-bot/study"
)

func newRecruitCloseHandler(
	studyRepo *db.StudyRepository,
	memberRepo *db.MemberRepository,
	recruitRepo *db.RecruitRepository,
	reactionHandler *ReactionHandler,
) func(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate) {
	return func(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate) {
		branch := recruitBranchFromOptions(i.ApplicationCommandData().Options)
		logCommand(ctx, i, "start", "recruit-close requested branch=%s", branch)

		if !isValidBranch(branch) {
			respondError(ctx, s, i, fmt.Sprintf("Invalid branch format: %q. Use YY-Q (e.g. 26-2).", branch))
			return
		}
		if !deferInteractionResponse(ctx, s, i, true) {
			return
		}

		mappings, err := recruitRepo.FindOpenRecruitMappingsByBranch(ctx, branch)
		if err != nil {
			slog.Error("failed to find open recruit mappings by branch", "branch", branch, "error", err)
			editDeferredError(ctx, s, i, "Failed to load recruitment data.")
			return
		}
		if len(mappings) == 0 {
			editDeferredError(ctx, s, i, fmt.Sprintf("No open recruitments found for branch %q.", branch))
			return
		}

		summaries, err := collectRecruitSignupsFromMappings(ctx, s, mappings, botUserID(s))
		if err != nil {
			slog.Error("failed to collect recruit signup reactions", "branch", branch, "error", err)
			editDeferredError(ctx, s, i, "Failed to collect recruitment reactions.")
			return
		}

		reactionHandler.Untrack(recruitMessageIDsFromMappings(mappings))
		if _, err := recruitRepo.CloseByBranch(ctx, branch); err != nil {
			slog.Error("failed to close recruit messages by branch", "branch", branch, "error", err)
			editDeferredError(ctx, s, i, "Failed to close recruitment.")
			return
		}

		var started []string
		var archived []string
		var errors []string

		for _, summary := range summaries {
			if summary.Count < minMembersToStart {
				archiveResult, archiveErr := archiveStudy(ctx, s, studyRepo, i.GuildID, study.Study{
					ID:        summary.StudyID,
					Name:      summary.StudyName,
					ChannelID: summary.StudyChannelID,
					RoleID:    summary.RoleID,
					Status:    "active",
				})
				if archiveErr != nil {
					slog.Error("failed to auto-archive study", "study_id", summary.StudyID, "study_name", summary.StudyName, "error", archiveErr)
					errors = append(errors, fmt.Sprintf("%s: archive failed", summary.StudyName))
					continue
				}

				if _, msgErr := s.ChannelMessageSend(summary.StudyChannelID,
					fmt.Sprintf("모집 인원이 %d명 미만이어서 스터디가 자동 아카이브되었습니다.", minMembersToStart),
					discordgo.WithContext(ctx),
				); msgErr != nil {
					slog.Warn("failed to send archive notice to channel", "channel_id", summary.StudyChannelID, "error", msgErr)
				}

				archiveEntry := summary.StudyName
				if archiveResult.Warning != "" {
					archiveEntry += " (" + archiveResult.Warning + ")"
				}
				archived = append(archived, archiveEntry)
				continue
			}

			if _, err := s.GuildRoleEdit(i.GuildID, summary.RoleID, &discordgo.RoleParams{Mentionable: boolPtr(true)}, discordgo.WithContext(ctx)); err != nil {
				slog.Warn("failed to make study role mentionable", "guild_id", i.GuildID, "role_id", summary.RoleID, "error", err)
				errors = append(errors, fmt.Sprintf("%s: role mentionable update failed", summary.StudyName))
			}

			for _, user := range summary.Users {
				if user == nil || user.ID == "" {
					continue
				}
				if err := s.GuildMemberRoleAdd(i.GuildID, user.ID, summary.RoleID, discordgo.WithContext(ctx)); err != nil {
					slog.Error("failed to add role to recruited user", "guild_id", i.GuildID, "role_id", summary.RoleID, "user_id", user.ID, "error", err)
					errors = append(errors, fmt.Sprintf("%s: failed to add role to %s", summary.StudyName, user.ID))
					continue
				}
				if err := memberRepo.AddMember(ctx, summary.StudyID, user.ID, recruitReactionDisplayName(user)); err != nil {
					slog.Error("failed to record recruited member", "study_id", summary.StudyID, "user_id", user.ID, "error", err)
					errors = append(errors, fmt.Sprintf("%s: failed to record %s", summary.StudyName, user.ID))
				}
			}

			if _, msgErr := s.ChannelMessageSend(summary.StudyChannelID,
				fmt.Sprintf("<@&%s> 스터디에 오신 것을 환영합니다! 스터디를 진행해주세요.", summary.RoleID),
				discordgo.WithContext(ctx),
			); msgErr != nil {
				slog.Warn("failed to send start notice to channel", "channel_id", summary.StudyChannelID, "error", msgErr)
			}

			started = append(started, summary.StudyName)
		}

		summary := buildStudyStartSummary(started, archived, errors)
		if err := editOriginalInteractionResponse(ctx, s, i, summary); err != nil {
			logCommand(ctx, i, "error", "failed to respond recruit-close summary: %v", err)
			return
		}
		logCommand(ctx, i, "success", "recruit-close completed branch=%s started=%d archived=%d errors=%d",
			branch, len(started), len(archived), len(errors))
	}
}

func recruitMessageIDsFromMappings(mappings []db.OpenRecruitMapping) []string {
	seen := make(map[string]struct{}, len(mappings))
	ids := make([]string, 0, len(mappings))
	for _, mapping := range mappings {
		if mapping.RecruitMessageID == "" {
			continue
		}
		if _, ok := seen[mapping.RecruitMessageID]; ok {
			continue
		}
		seen[mapping.RecruitMessageID] = struct{}{}
		ids = append(ids, mapping.RecruitMessageID)
	}
	return ids
}

func recruitReactionDisplayName(user *discordgo.User) string {
	if user == nil {
		return ""
	}
	if name := user.DisplayName(); name != "" {
		return name
	}
	if user.Username != "" {
		return user.Username
	}
	return user.ID
}
