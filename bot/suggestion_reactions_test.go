package bot

import (
	"testing"

	"github.com/bwmarrin/discordgo"
)

func TestFilterSuggestionReactionUsersDropsBotEmptyAndDuplicates(t *testing.T) {
	participants, ids := filterSuggestionReactionUsers([]*discordgo.User{
		nil,
		{ID: "bot"},
		{ID: "bot-flag", Bot: true},
		{ID: "user-1", Username: "one"},
		{ID: "user-1", Username: "one duplicate"},
		{ID: ""},
		{ID: "user-2", Username: "two"},
	}, "bot")

	if len(participants) != 2 || len(ids) != 2 {
		t.Fatalf("expected two participants, got participants=%d ids=%d", len(participants), len(ids))
	}
	if ids[0] != "user-1" || ids[1] != "user-2" {
		t.Fatalf("unexpected ids: %v", ids)
	}
}
