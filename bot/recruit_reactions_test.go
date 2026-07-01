package bot

import (
	"context"
	"fmt"
	"testing"

	"github.com/bwmarrin/discordgo"
)

func TestFetchAllReactionUsersPaginates(t *testing.T) {
	users := make([]*discordgo.User, 0, 101)
	for idx := 0; idx < 101; idx++ {
		userID := fmt.Sprintf("user-%03d", idx)
		users = append(users, &discordgo.User{ID: userID, Username: userID})
	}

	client := &fakeReactionUserClient{
		reactions: map[string][]*discordgo.User{
			reactionKey("suggestion-channel", "suggestion-message", "🚀"): users,
		},
	}

	fetched, err := fetchAllReactionUsers(context.Background(), client, "suggestion-channel", "suggestion-message", "🚀")
	if err != nil {
		t.Fatalf("fetch reaction users: %v", err)
	}

	if len(fetched) != 101 {
		t.Fatalf("expected 101 users, got %d", len(fetched))
	}
	if fetched[0].ID != "user-000" || fetched[100].ID != "user-100" {
		t.Fatalf("expected paginated users in order, got first=%s last=%s", fetched[0].ID, fetched[100].ID)
	}
	if client.calls != 2 {
		t.Fatalf("expected two paginated reaction API calls, got %d", client.calls)
	}
}

type fakeReactionUserClient struct {
	reactions map[string][]*discordgo.User
	calls     int
}

func (f *fakeReactionUserClient) MessageReactions(channelID, messageID, emojiID string, limit int, beforeID, afterID string, options ...discordgo.RequestOption) ([]*discordgo.User, error) {
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
