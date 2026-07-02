package bot

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/bwmarrin/discordgo"
	"livid-bot/db"
)

const suggestionVoteEmoji = "🚀"

type SuggestionReactionHandler struct {
	suggestionRepo *db.SuggestionRepository
	studyRepo      *db.StudyRepository
	memberRepo     *db.MemberRepository
}

func NewSuggestionReactionHandler(suggestionRepo *db.SuggestionRepository, studyRepo *db.StudyRepository, memberRepo *db.MemberRepository) *SuggestionReactionHandler {
	return &SuggestionReactionHandler{suggestionRepo: suggestionRepo, studyRepo: studyRepo, memberRepo: memberRepo}
}

func (h *SuggestionReactionHandler) OnReactionAdd(s *discordgo.Session, r *discordgo.MessageReactionAdd) {
	if r == nil {
		return
	}
	h.handleReaction(s, r.MessageReaction)
}

func (h *SuggestionReactionHandler) OnReactionRemove(s *discordgo.Session, r *discordgo.MessageReactionRemove) {
	if r == nil {
		return
	}
	h.handleReaction(s, r.MessageReaction)
}

func (h *SuggestionReactionHandler) handleReaction(s *discordgo.Session, reaction *discordgo.MessageReaction) {
	if h == nil || h.suggestionRepo == nil || reaction == nil || reaction.Emoji.Name != suggestionVoteEmoji || reaction.UserID == botUserID(s) {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), reactionTimeout)
	defer cancel()
	ctx, span := startReactionSpan(ctx, "suggestion.reaction", reaction)
	defer span.End()

	suggestion, err := h.suggestionRepo.GetOpenSuggestionByMessageRef(ctx, reaction.ChannelID, reaction.MessageID)
	if err != nil {
		slog.Error("failed to find suggestion for reaction", "channel_id", reaction.ChannelID, "message_id", reaction.MessageID, "error", err)
		return
	}
	if suggestion == nil {
		return
	}

	users, err := fetchAllReactionUsers(ctx, s, reaction.ChannelID, reaction.MessageID, suggestionVoteEmoji)
	if err != nil {
		slog.Error("failed to fetch suggestion reaction users", "suggestion_id", suggestion.ID, "error", err)
		return
	}
	participants, userIDs := filterSuggestionReactionUsers(users, botUserID(s))

	result, err := h.suggestionRepo.SyncVotes(ctx, suggestion.ID, userIDs)
	if err != nil {
		slog.Error("failed to sync suggestion votes", "suggestion_id", suggestion.ID, "error", err)
		return
	}
	if !result.JustConfirmed {
		return
	}

	if err := h.openConfirmedSuggestion(ctx, s, reaction.GuildID, result, participants); err != nil {
		slog.Error("failed to open confirmed suggestion", "suggestion_id", result.SuggestionID, "error", err)
		if markErr := h.suggestionRepo.MarkOpeningFailed(ctx, result.SuggestionID, err.Error()); markErr != nil {
			slog.Error("failed to mark suggestion opening failed", "suggestion_id", result.SuggestionID, "error", markErr)
		}
	}
}

func filterSuggestionReactionUsers(users []*discordgo.User, botUserID string) ([]*discordgo.User, []string) {
	seen := make(map[string]struct{}, len(users))
	participants := make([]*discordgo.User, 0, len(users))
	userIDs := make([]string, 0, len(users))
	for _, user := range users {
		if user == nil || user.ID == "" || user.ID == botUserID || user.Bot {
			continue
		}
		if _, ok := seen[user.ID]; ok {
			continue
		}
		seen[user.ID] = struct{}{}
		participants = append(participants, user)
		userIDs = append(userIDs, user.ID)
	}
	return participants, userIDs
}

func (h *SuggestionReactionHandler) openConfirmedSuggestion(ctx context.Context, s *discordgo.Session, guildID string, result *db.SyncVotesResult, participants []*discordgo.User) error {
	if h.studyRepo == nil || h.memberRepo == nil {
		return fmt.Errorf("study repositories are not configured")
	}
	name := normalizeStudyName(result.SuggestionTitle)
	if name == "" {
		return fmt.Errorf("suggestion title is empty")
	}

	created, err := createStudyResources(ctx, s, h.studyRepo, guildID, "", name, result.SuggestionDescription)
	if err != nil {
		return err
	}
	if err := h.suggestionRepo.MarkOpened(ctx, result.SuggestionID, created.ID); err != nil {
		return fmt.Errorf("mark suggestion opened: %w", err)
	}

	for _, user := range participants {
		if user == nil || user.ID == "" {
			continue
		}
		if err := s.GuildMemberRoleAdd(guildID, user.ID, created.RoleID, discordgo.WithContext(ctx)); err != nil {
			slog.Error("failed to add suggestion study role", "guild_id", guildID, "role_id", created.RoleID, "user_id", user.ID, "error", err)
			continue
		}
		if err := h.memberRepo.AddMember(ctx, created.ID, user.ID, reactionDisplayName(user)); err != nil {
			slog.Error("failed to record suggestion study member", "study_id", created.ID, "user_id", user.ID, "error", err)
		}
	}

	if _, err := s.ChannelMessageSend(created.ChannelID,
		fmt.Sprintf("<@&%s> 스터디가 자동 개설되었습니다.\n\n🚀 기준 인원이 모여 채널이 생성되었습니다. 이후 진행 방식은 이 채널에서 직접 조율해주세요.", created.RoleID),
		discordgo.WithContext(ctx),
	); err != nil {
		slog.Warn("failed to send suggestion study open notice", "channel_id", created.ChannelID, "error", err)
	}
	return nil
}
