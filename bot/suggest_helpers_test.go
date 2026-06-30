package bot

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
)

func TestParseSuggestionDeadlineUsesKSTEndOfDay(t *testing.T) {
	now := time.Date(2026, 3, 12, 9, 0, 0, 0, suggestionDeadlineLocation)

	got, err := parseSuggestionDeadline("2026-03-13", now)
	if err != nil {
		t.Fatalf("expected deadline parse to succeed, got error: %v", err)
	}

	if got.Location().String() != suggestionDeadlineLocation.String() {
		t.Fatalf("expected location %q, got %q", suggestionDeadlineLocation, got.Location())
	}
	if got.Hour() != 23 || got.Minute() != 59 || got.Second() != 59 {
		t.Fatalf("expected end-of-day deadline, got %s", got)
	}
	if suggestionDateLabel(got) != "2026-03-13" {
		t.Fatalf("expected formatted label 2026-03-13, got %q", suggestionDateLabel(got))
	}
}

func TestParseSuggestionDeadlineRejectsPastOrElapsedDate(t *testing.T) {
	now := time.Date(2026, 3, 12, 23, 59, 59, 0, suggestionDeadlineLocation)

	if _, err := parseSuggestionDeadline("2026-03-12", now); !errors.Is(err, errSuggestionDeadlinePast) {
		t.Fatalf("expected errSuggestionDeadlinePast for same-day elapsed deadline, got %v", err)
	}
	if _, err := parseSuggestionDeadline("2026-03-11", now); !errors.Is(err, errSuggestionDeadlinePast) {
		t.Fatalf("expected errSuggestionDeadlinePast for past deadline, got %v", err)
	}
}

func TestBuildSuggestionMessage(t *testing.T) {
	expiresAt := time.Date(2026, 3, 26, 23, 59, 59, 0, suggestionDeadlineLocation)
	withDescription := buildSuggestionMessage("Go 스터디", "동시성 중심", suggestionPostOptions{Visibility: suggestionVisibilityAnonymous, Threshold: 3, ExpiresAt: expiresAt})
	if !strings.Contains(withDescription, "**주제**: Go 스터디") {
		t.Fatalf("expected title in message, got: %s", withDescription)
	}
	if !strings.Contains(withDescription, "설명: 동시성 중심") {
		t.Fatalf("expected description in message, got: %s", withDescription)
	}
	if !strings.Contains(withDescription, "🚀 3명 이상") {
		t.Fatalf("expected threshold in suggestion message, got: %s", withDescription)
	}

	withoutDescription := buildSuggestionMessage("Rust 스터디", "", suggestionPostOptions{Visibility: suggestionVisibilityPublic, ProposerUserID: "user-1", Threshold: 5, ExpiresAt: expiresAt})
	if strings.Contains(withoutDescription, "설명:") {
		t.Fatalf("did not expect description line for empty description, got: %s", withoutDescription)
	}
	if !strings.Contains(withoutDescription, "제안자: <@user-1>") {
		t.Fatalf("expected public proposer in suggestion message, got: %s", withoutDescription)
	}
}

func TestFindSuggestionDiscussionChannelUsesFixedDiscussionChannel(t *testing.T) {
	channels := []*discordgo.Channel{
		{ID: "old", Name: "운영진-자유채팅", Type: discordgo.ChannelTypeGuildText},
		{ID: "target", Name: suggestionDiscussionChannelName, Type: discordgo.ChannelTypeGuildForum},
	}

	got := findSuggestionDiscussionChannel(channels)
	if got == nil {
		t.Fatal("expected discussion channel to be found")
	}
	if got.ID != "target" {
		t.Fatalf("expected fixed discussion channel ID target, got %s", got.ID)
	}
}

func TestSuggestModalCustomIDRoundTrip(t *testing.T) {
	opts := suggestionModalOptions{Visibility: suggestionVisibilityPublic, Threshold: 5, DurationDays: 21, ChannelID: "channel-1"}
	got, err := parseSuggestModalCustomID(buildSuggestModalCustomID(opts))
	if err != nil {
		t.Fatalf("parse custom id: %v", err)
	}
	if got != opts {
		t.Fatalf("expected %+v, got %+v", opts, got)
	}
}

func TestPublishSuggestionMessageCreatesForumPostForForumChannel(t *testing.T) {
	client := &fakeSuggestionDiscordClient{
		channels: map[string]*discordgo.Channel{
			"forum": {ID: "forum", Type: discordgo.ChannelTypeGuildForum},
		},
		forumThread: &discordgo.Channel{ID: "thread", LastMessageID: "starter-message"},
	}

	ref, err := publishSuggestionMessage(context.Background(), client, "forum", "Go 스터디", "동시성 중심", suggestionPostOptions{Visibility: suggestionVisibilityAnonymous, Threshold: 3, ExpiresAt: time.Now().Add(24 * time.Hour)})
	if err != nil {
		t.Fatalf("expected forum post publish to succeed, got error: %v", err)
	}

	if client.sentMessageChannelID != "" {
		t.Fatalf("expected not to send a regular channel message, sent to %s", client.sentMessageChannelID)
	}
	if client.forumThreadChannelID != "forum" {
		t.Fatalf("expected forum thread to be created in forum channel, got %s", client.forumThreadChannelID)
	}
	if client.forumThreadName != "Go 스터디" {
		t.Fatalf("expected forum thread name to use suggestion title, got %q", client.forumThreadName)
	}
	if !strings.Contains(client.forumThreadMessageContent, "**주제**: Go 스터디") {
		t.Fatalf("expected forum starter message to contain suggestion content, got %q", client.forumThreadMessageContent)
	}
	if client.forumThreadAllowedMentions == nil {
		t.Fatal("expected forum starter message to suppress mentions")
	}
	if ref.ChannelID != "thread" || ref.MessageID != "starter-message" {
		t.Fatalf("expected returned ref to point at forum thread starter, got channel=%s message=%s", ref.ChannelID, ref.MessageID)
	}
}

func TestPublishSuggestionMessageSendsRegularMessageForTextChannel(t *testing.T) {
	client := &fakeSuggestionDiscordClient{
		channels: map[string]*discordgo.Channel{
			"text": {ID: "text", Type: discordgo.ChannelTypeGuildText},
		},
		sentMessage: &discordgo.Message{ID: "message"},
	}

	ref, err := publishSuggestionMessage(context.Background(), client, "text", "Rust 스터디", "", suggestionPostOptions{Visibility: suggestionVisibilityAnonymous, Threshold: 3, ExpiresAt: time.Now().Add(24 * time.Hour)})
	if err != nil {
		t.Fatalf("expected text channel publish to succeed, got error: %v", err)
	}

	if client.sentMessageChannelID != "text" {
		t.Fatalf("expected regular message to be sent to text channel, got %s", client.sentMessageChannelID)
	}
	if client.sentMessageAllowedMentions == nil {
		t.Fatal("expected regular suggestion message to suppress mentions")
	}
	if client.forumThreadChannelID != "" {
		t.Fatalf("expected not to create forum thread, created in %s", client.forumThreadChannelID)
	}
	if ref.ChannelID != "text" || ref.MessageID != "message" {
		t.Fatalf("expected returned ref to point at text message, got channel=%s message=%s", ref.ChannelID, ref.MessageID)
	}
}

type fakeSuggestionDiscordClient struct {
	channels map[string]*discordgo.Channel

	sentMessageChannelID       string
	sentMessageContent         string
	sentMessageAllowedMentions *discordgo.MessageAllowedMentions
	sentMessage                *discordgo.Message

	forumThreadChannelID       string
	forumThreadName            string
	forumThreadMessageContent  string
	forumThreadAllowedMentions *discordgo.MessageAllowedMentions
	forumThread                *discordgo.Channel
}

func (f *fakeSuggestionDiscordClient) Channel(channelID string, options ...discordgo.RequestOption) (*discordgo.Channel, error) {
	ch := f.channels[channelID]
	if ch == nil {
		return nil, errors.New("channel not found")
	}
	return ch, nil
}

func (f *fakeSuggestionDiscordClient) ChannelMessageSendComplex(channelID string, data *discordgo.MessageSend, options ...discordgo.RequestOption) (*discordgo.Message, error) {
	f.sentMessageChannelID = channelID
	if data != nil {
		f.sentMessageContent = data.Content
		f.sentMessageAllowedMentions = data.AllowedMentions
	}
	if f.sentMessage == nil {
		return nil, errors.New("sent message not configured")
	}
	return f.sentMessage, nil
}

func (f *fakeSuggestionDiscordClient) ForumThreadStartComplex(channelID string, threadData *discordgo.ThreadStart, messageData *discordgo.MessageSend, options ...discordgo.RequestOption) (*discordgo.Channel, error) {
	f.forumThreadChannelID = channelID
	if threadData != nil {
		f.forumThreadName = threadData.Name
	}
	if messageData != nil {
		f.forumThreadMessageContent = messageData.Content
		f.forumThreadAllowedMentions = messageData.AllowedMentions
	}
	if f.forumThread == nil {
		return nil, errors.New("forum thread not configured")
	}
	return f.forumThread, nil
}
