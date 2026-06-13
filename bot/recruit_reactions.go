package bot

import (
	"context"
	"fmt"

	"github.com/bwmarrin/discordgo"
	"livid-bot/db"
)

const discordReactionPageLimit = 100

type recruitReactionClient interface {
	MessageReactions(channelID, messageID, emojiID string, limit int, beforeID, afterID string, options ...discordgo.RequestOption) ([]*discordgo.User, error)
}

type RecruitSignupSummary struct {
	RecruitMessageID string
	RecruitChannelID string
	StudyID          int64
	StudyName        string
	StudyChannelID   string
	RoleID           string
	Emoji            string
	Users            []*discordgo.User
	Count            int
}

func collectRecruitSignupsFromMappings(
	ctx context.Context,
	client recruitReactionClient,
	mappings []db.OpenRecruitMapping,
	botUserID string,
) ([]RecruitSignupSummary, error) {
	byStudy := make(map[int64]*RecruitSignupSummary)
	seenUsersByStudy := make(map[int64]map[string]struct{})
	order := make([]int64, 0, len(mappings))

	for _, mapping := range mappings {
		summary, ok := byStudy[mapping.StudyID]
		if !ok {
			byStudy[mapping.StudyID] = &RecruitSignupSummary{
				RecruitMessageID: mapping.RecruitMessageID,
				RecruitChannelID: mapping.RecruitChannelID,
				StudyID:          mapping.StudyID,
				StudyName:        mapping.StudyName,
				StudyChannelID:   mapping.StudyChannelID,
				RoleID:           mapping.RoleID,
				Emoji:            mapping.Emoji,
			}
			seenUsersByStudy[mapping.StudyID] = make(map[string]struct{})
			order = append(order, mapping.StudyID)
			summary = byStudy[mapping.StudyID]
		}

		users, err := fetchAllReactionUsers(ctx, client, mapping.RecruitChannelID, mapping.RecruitMessageID, mapping.Emoji)
		if err != nil {
			return nil, fmt.Errorf("fetch reaction users for study %d emoji %s: %w", mapping.StudyID, mapping.Emoji, err)
		}

		seenUsers := seenUsersByStudy[mapping.StudyID]
		for _, user := range users {
			if user == nil || user.ID == "" || user.ID == botUserID {
				continue
			}
			if _, exists := seenUsers[user.ID]; exists {
				continue
			}
			seenUsers[user.ID] = struct{}{}
			summary.Users = append(summary.Users, user)
		}
	}

	summaries := make([]RecruitSignupSummary, 0, len(order))
	for _, studyID := range order {
		summary := byStudy[studyID]
		summary.Count = len(summary.Users)
		summaries = append(summaries, *summary)
	}
	return summaries, nil
}

func fetchAllReactionUsers(ctx context.Context, client recruitReactionClient, channelID, messageID, emoji string) ([]*discordgo.User, error) {
	var allUsers []*discordgo.User
	afterID := ""

	for {
		users, err := client.MessageReactions(
			channelID,
			messageID,
			emoji,
			discordReactionPageLimit,
			"",
			afterID,
			discordgo.WithContext(ctx),
		)
		if err != nil {
			return nil, err
		}
		if len(users) == 0 {
			return allUsers, nil
		}

		allUsers = append(allUsers, users...)
		if len(users) < discordReactionPageLimit {
			return allUsers, nil
		}

		last := users[len(users)-1]
		if last == nil || last.ID == "" || last.ID == afterID {
			return allUsers, nil
		}
		afterID = last.ID
	}
}
