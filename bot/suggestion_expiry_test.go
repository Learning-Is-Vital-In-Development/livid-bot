package bot

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"
	"livid-bot/db"
)

type fakeExpiredSuggestionStore struct {
	suggestions []*db.StudySuggestion
	marked      []int64
	markErr     error
}

func (s *fakeExpiredSuggestionStore) ListExpiredOpenSuggestions(context.Context) ([]*db.StudySuggestion, error) {
	return s.suggestions, nil
}

func (s *fakeExpiredSuggestionStore) MarkExpired(_ context.Context, suggestionID int64) error {
	if s.markErr != nil {
		return s.markErr
	}
	s.marked = append(s.marked, suggestionID)
	return nil
}

type fakeSuggestionExpiryClient struct {
	messages map[string][]string
	err      error
}

func (c *fakeSuggestionExpiryClient) ChannelMessageSend(channelID, content string, _ ...discordgo.RequestOption) (*discordgo.Message, error) {
	if c.err != nil {
		return nil, c.err
	}
	if c.messages == nil {
		c.messages = make(map[string][]string)
	}
	c.messages[channelID] = append(c.messages[channelID], content)
	return &discordgo.Message{ID: "message-1"}, nil
}

func TestRunSuggestionExpiryCheckCommentsAndMarksExpired(t *testing.T) {
	store := &fakeExpiredSuggestionStore{suggestions: []*db.StudySuggestion{{ID: 7, ChannelID: "thread-1"}}}
	client := &fakeSuggestionExpiryClient{}
	var logs bytes.Buffer
	oldLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	defer slog.SetDefault(oldLogger)

	runSuggestionExpiryCheck(context.Background(), client, store)

	if got := len(client.messages["thread-1"]); got != 1 {
		t.Fatalf("expected one expiry message, got %d", got)
	}
	msg := client.messages["thread-1"][0]
	for _, want := range []string{"모집 기간이 종료", "/suggest", "운영진"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("expected %q in expiry message, got %s", want, msg)
		}
	}
	if len(store.marked) != 1 || store.marked[0] != 7 {
		t.Fatalf("expected suggestion 7 marked expired, got %v", store.marked)
	}
	for _, want := range []string{"sent suggestion expiry notice", "suggestion_id=7", "channel_id=thread-1", "message_id=message-1"} {
		if !strings.Contains(logs.String(), want) {
			t.Fatalf("expected %q in success log, got %s", want, logs.String())
		}
	}
}

func TestRunSuggestionExpiryCheckDoesNotMarkWhenCommentFails(t *testing.T) {
	store := &fakeExpiredSuggestionStore{suggestions: []*db.StudySuggestion{{ID: 8, ChannelID: "thread-1"}}}
	client := &fakeSuggestionExpiryClient{err: errors.New("discord down")}

	runSuggestionExpiryCheck(context.Background(), client, store)

	if len(store.marked) != 0 {
		t.Fatalf("expected no expired mark on comment failure, got %v", store.marked)
	}
}
