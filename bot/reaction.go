package bot

import (
	"context"
	"log/slog"
	"sync"

	"github.com/bwmarrin/discordgo"
	"livid-bot/db"
)

type emojiStudyInfo struct {
	RoleID  string
	StudyID int64
}

type ReactionHandler struct {
	mu         sync.RWMutex
	mappings   map[string]map[string]emojiStudyInfo // messageID -> emoji -> info
	memberRepo *db.MemberRepository
}

func NewReactionHandler(memberRepo *db.MemberRepository) *ReactionHandler {
	return &ReactionHandler{
		mappings:   make(map[string]map[string]emojiStudyInfo),
		memberRepo: memberRepo,
	}
}

func (h *ReactionHandler) LoadFromDB(recruitRepo *db.RecruitRepository) error {
	ctx := context.Background()
	dbMappings, err := recruitRepo.LoadAllMappings(ctx)
	if err != nil {
		return err
	}

	newMap := make(map[string]map[string]emojiStudyInfo)
	for _, m := range dbMappings {
		if _, ok := newMap[m.MessageID]; !ok {
			newMap[m.MessageID] = make(map[string]emojiStudyInfo)
		}
		newMap[m.MessageID][m.Emoji] = emojiStudyInfo{
			RoleID:  m.RoleID,
			StudyID: m.StudyID,
		}
	}

	h.mu.Lock()
	h.mappings = newMap
	h.mu.Unlock()

	slog.Info("loaded reaction mappings from DB", "count", len(dbMappings))
	return nil
}

// Track adds a new message's emoji-role mappings (copy-on-write).
func (h *ReactionHandler) Track(messageID string, emojiMap map[string]emojiStudyInfo) {
	h.mu.Lock()
	defer h.mu.Unlock()

	newMappings := make(map[string]map[string]emojiStudyInfo, len(h.mappings)+1)
	for k, v := range h.mappings {
		newMappings[k] = v
	}
	newMappings[messageID] = emojiMap
	h.mappings = newMappings
}

// Untrack removes message mappings for the given IDs (copy-on-write).
func (h *ReactionHandler) Untrack(messageIDs []string) {
	if len(messageIDs) == 0 {
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	newMappings := make(map[string]map[string]emojiStudyInfo, len(h.mappings))
	removeSet := make(map[string]struct{}, len(messageIDs))
	for _, id := range messageIDs {
		removeSet[id] = struct{}{}
	}
	for k, v := range h.mappings {
		if _, skip := removeSet[k]; !skip {
			newMappings[k] = v
		}
	}
	h.mappings = newMappings
}

func (h *ReactionHandler) OnReactionAdd(s *discordgo.Session, r *discordgo.MessageReactionAdd) {
	// Ignore bot's own reactions
	if r.UserID == s.State.User.ID {
		return
	}

	info, ok := h.lookup(r.MessageID, r.Emoji.Name)
	if !ok {
		return
	}

	if err := s.GuildMemberRoleAdd(r.GuildID, r.UserID, info.RoleID); err != nil {
		slog.Error("failed to add role to user", "guild_id", r.GuildID, "role_id", info.RoleID, "user_id", r.UserID, "error", err)
		return
	}

	// Get username for DB record
	member, err := s.GuildMember(r.GuildID, r.UserID)
	if err != nil {
		slog.Error("failed to get member info", "guild_id", r.GuildID, "user_id", r.UserID, "error", err)
		return
	}

	username := member.User.Username
	if member.Nick != "" {
		username = member.Nick
	}

	ctx := context.Background()
	if err := h.memberRepo.AddMember(ctx, info.StudyID, r.UserID, username); err != nil {
		slog.Error("failed to record member join", "study_id", info.StudyID, "user_id", r.UserID, "error", err)
	}

	slog.Info("user joined study", "username", username, "user_id", r.UserID, "study_id", info.StudyID)
}

func (h *ReactionHandler) OnReactionRemove(s *discordgo.Session, r *discordgo.MessageReactionRemove) {
	if r.UserID == s.State.User.ID {
		return
	}

	info, ok := h.lookup(r.MessageID, r.Emoji.Name)
	if !ok {
		return
	}

	if err := s.GuildMemberRoleRemove(r.GuildID, r.UserID, info.RoleID); err != nil {
		slog.Error("failed to remove role from user", "guild_id", r.GuildID, "role_id", info.RoleID, "user_id", r.UserID, "error", err)
		return
	}

	ctx := context.Background()
	if err := h.memberRepo.RemoveMember(ctx, info.StudyID, r.UserID); err != nil {
		slog.Error("failed to record member leave", "study_id", info.StudyID, "user_id", r.UserID, "error", err)
	}

	slog.Info("user left study", "user_id", r.UserID, "study_id", info.StudyID)
}

func (h *ReactionHandler) lookup(messageID, emoji string) (emojiStudyInfo, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	emojiMap, ok := h.mappings[messageID]
	if !ok {
		return emojiStudyInfo{}, false
	}
	info, ok := emojiMap[emoji]
	return info, ok
}
