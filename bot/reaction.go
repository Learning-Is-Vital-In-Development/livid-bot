package bot

import (
	"context"
	"log"
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

	log.Printf("Loaded %d reaction mappings from DB", len(dbMappings))
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
		log.Printf("Failed to add role %s to user %s: %v", info.RoleID, r.UserID, err)
		return
	}

	// Get username for DB record
	member, err := s.GuildMember(r.GuildID, r.UserID)
	if err != nil {
		log.Printf("Failed to get member info for %s: %v", r.UserID, err)
		return
	}

	username := member.User.Username
	if member.Nick != "" {
		username = member.Nick
	}

	ctx := context.Background()
	if err := h.memberRepo.AddMember(ctx, info.StudyID, r.UserID, username); err != nil {
		log.Printf("Failed to record member join: %v", err)
	}

	log.Printf("User %s (%s) joined study %d", username, r.UserID, info.StudyID)
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
		log.Printf("Failed to remove role %s from user %s: %v", info.RoleID, r.UserID, err)
		return
	}

	ctx := context.Background()
	if err := h.memberRepo.RemoveMember(ctx, info.StudyID, r.UserID); err != nil {
		log.Printf("Failed to record member leave: %v", err)
	}

	log.Printf("User %s left study %d", r.UserID, info.StudyID)
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
