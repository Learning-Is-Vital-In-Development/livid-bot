package bot

import (
	"context"
	"strings"
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

func TestBuildSuggestionOpenedNoticeIncludesCreatedChannelLink(t *testing.T) {
	notice := buildSuggestionOpenedNotice("study-channel")

	if !strings.Contains(notice, "<#study-channel>") {
		t.Fatalf("expected created study channel link in notice, got %q", notice)
	}
}

func TestSendSuggestionOpenedNoticeRepliesToOriginalSuggestion(t *testing.T) {
	client := &fakeSuggestionOpenedNoticeClient{}

	err := sendSuggestionOpenedNotice(context.Background(), client, "guild-1", "suggestion-thread", "starter-message", "study-channel")
	if err != nil {
		t.Fatalf("send notice: %v", err)
	}

	if client.channelID != "suggestion-thread" {
		t.Fatalf("expected notice in original suggestion channel, got %s", client.channelID)
	}
	if client.data == nil {
		t.Fatal("expected message data")
	}
	if !strings.Contains(client.data.Content, "<#study-channel>") {
		t.Fatalf("expected study channel link in notice, got %q", client.data.Content)
	}
	if client.data.AllowedMentions == nil {
		t.Fatal("expected mentions to be suppressed")
	}
	if client.data.Reference == nil {
		t.Fatal("expected notice to reference original suggestion message")
	}
	if client.data.Reference.GuildID != "guild-1" || client.data.Reference.ChannelID != "suggestion-thread" || client.data.Reference.MessageID != "starter-message" {
		t.Fatalf("unexpected reference: %+v", client.data.Reference)
	}
	if client.data.Reference.FailIfNotExists == nil || *client.data.Reference.FailIfNotExists {
		t.Fatalf("expected soft message reference, got %+v", client.data.Reference.FailIfNotExists)
	}
}

type fakeSuggestionOpenedNoticeClient struct {
	channelID string
	data      *discordgo.MessageSend
}

func (f *fakeSuggestionOpenedNoticeClient) ChannelMessageSendComplex(channelID string, data *discordgo.MessageSend, options ...discordgo.RequestOption) (*discordgo.Message, error) {
	f.channelID = channelID
	f.data = data
	return &discordgo.Message{ID: "notice-message"}, nil
}
