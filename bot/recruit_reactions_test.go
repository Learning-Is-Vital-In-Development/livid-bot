package bot

import (
	"context"
	"fmt"
	"testing"

	"github.com/bwmarrin/discordgo"
	"livid-bot/db"
)

func TestCollectRecruitSignupsFromMappingsPaginatesAndExcludesBot(t *testing.T) {
	const botUserID = "bot-user"

	humans := make([]*discordgo.User, 0, 101)
	for idx := 0; idx < 101; idx++ {
		userID := fmt.Sprintf("user-%03d", idx)
		humans = append(humans, &discordgo.User{ID: userID, Username: userID})
	}

	client := &fakeRecruitReactionClient{
		reactions: map[string][]*discordgo.User{
			reactionKey("recruit-channel", "recruit-message", "1️⃣"): append([]*discordgo.User{{ID: botUserID, Username: "bot"}}, humans...),
		},
	}

	summaries, err := collectRecruitSignupsFromMappings(context.Background(), client, []db.OpenRecruitMapping{
		{
			RecruitChannelID: "recruit-channel",
			RecruitMessageID: "recruit-message",
			StudyID:          10,
			StudyName:        "Go Concurrency",
			StudyChannelID:   "study-channel",
			RoleID:           "role-go",
			Emoji:            "1️⃣",
		},
	}, botUserID)
	if err != nil {
		t.Fatalf("collect signups: %v", err)
	}

	if len(summaries) != 1 {
		t.Fatalf("expected one summary, got %d", len(summaries))
	}
	summary := summaries[0]
	if summary.Count != 101 {
		t.Fatalf("expected 101 human signups, got %d", summary.Count)
	}
	if summary.Users[0].ID != "user-000" || summary.Users[100].ID != "user-100" {
		t.Fatalf("expected paginated human users in order, got first=%s last=%s", summary.Users[0].ID, summary.Users[100].ID)
	}
	if client.calls != 2 {
		t.Fatalf("expected two paginated reaction API calls, got %d", client.calls)
	}
}

func TestCollectRecruitSignupsFromMappingsDeduplicatesUsersPerStudy(t *testing.T) {
	client := &fakeRecruitReactionClient{
		reactions: map[string][]*discordgo.User{
			reactionKey("recruit-channel-1", "recruit-message-1", "1️⃣"): {
				{ID: "user-1", Username: "one"},
				{ID: "user-2", Username: "two"},
			},
			reactionKey("recruit-channel-2", "recruit-message-2", "1️⃣"): {
				{ID: "user-2", Username: "two"},
				{ID: "user-3", Username: "three"},
			},
		},
	}

	summaries, err := collectRecruitSignupsFromMappings(context.Background(), client, []db.OpenRecruitMapping{
		{RecruitChannelID: "recruit-channel-1", RecruitMessageID: "recruit-message-1", StudyID: 10, StudyName: "Go", RoleID: "role-go", Emoji: "1️⃣"},
		{RecruitChannelID: "recruit-channel-2", RecruitMessageID: "recruit-message-2", StudyID: 10, StudyName: "Go", RoleID: "role-go", Emoji: "1️⃣"},
	}, "bot-user")
	if err != nil {
		t.Fatalf("collect signups: %v", err)
	}

	if len(summaries) != 1 {
		t.Fatalf("expected duplicate mappings for same study to be merged, got %d summaries", len(summaries))
	}
	if summaries[0].Count != 3 {
		t.Fatalf("expected deduplicated signup count 3, got %d", summaries[0].Count)
	}
}

type fakeRecruitReactionClient struct {
	reactions map[string][]*discordgo.User
	calls     int
}

func (f *fakeRecruitReactionClient) MessageReactions(channelID, messageID, emojiID string, limit int, beforeID, afterID string, options ...discordgo.RequestOption) ([]*discordgo.User, error) {
	f.calls++
	users := f.reactions[reactionKey(channelID, messageID, emojiID)]
	start := 0
	if afterID != "" {
		for idx, user := range users {
			if user.ID == afterID {
				start = idx + 1
				break
			}
		}
	}
	if limit <= 0 || start >= len(users) {
		return nil, nil
	}
	end := start + limit
	if end > len(users) {
		end = len(users)
	}
	return users[start:end], nil
}

func reactionKey(channelID, messageID, emoji string) string {
	return channelID + "|" + messageID + "|" + emoji
}
