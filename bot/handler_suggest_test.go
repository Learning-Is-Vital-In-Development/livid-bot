package bot

import (
	"context"
	"errors"
	"testing"

	"github.com/bwmarrin/discordgo"
	"livid-bot/db"
)

func TestSuggestModalHandlerDefersBeforePublishingAndEditsOriginal(t *testing.T) {
	order := []string{}
	store := &fakeSuggestStore{
		order:      &order,
		suggestion: &db.StudySuggestion{ID: 42},
	}
	client := &fakeSuggestModalDiscordClient{
		order: &order,
		channels: map[string]*discordgo.Channel{
			"forum": {ID: "forum", Type: discordgo.ChannelTypeGuildForum},
		},
		forumThread: &discordgo.Channel{ID: "thread", LastMessageID: "starter-message"},
	}
	responder := &fakeSuggestResponder{order: &order}

	handler := newSuggestModalHandlerWithDeps(store, responder, client)
	handler(context.Background(), nil, newSuggestModalInteractionForTest("Go 스터디", "동시성 중심"))

	wantOrder := []string{
		"defer",
		"load-channel",
		"create-forum-thread",
		"create-suggestion",
		"add-reaction",
		"edit",
	}
	if len(order) != len(wantOrder) {
		t.Fatalf("expected order %v, got %v", wantOrder, order)
	}
	for idx := range wantOrder {
		if order[idx] != wantOrder[idx] {
			t.Fatalf("expected order %v, got %v", wantOrder, order)
		}
	}
	if store.createdPeriodID != 0 || store.createdMessageID != "starter-message" || store.createdChannelID != "thread" {
		t.Fatalf("expected suggestion to store forum thread starter ref, got period=%d channel=%s message=%s", store.createdPeriodID, store.createdChannelID, store.createdMessageID)
	}
	if client.reactionChannelID != "thread" || client.reactionMessageID != "starter-message" || client.reactionEmoji != "🚀" {
		t.Fatalf("expected reaction on forum starter message, got channel=%s message=%s emoji=%s", client.reactionChannelID, client.reactionMessageID, client.reactionEmoji)
	}
	if responder.editedContent != "제안이 등록되었습니다!" {
		t.Fatalf("expected success edit, got %q", responder.editedContent)
	}
}

func TestSuggestModalHandlerResolvesAutoChannelAfterDeferring(t *testing.T) {
	order := []string{}
	store := &fakeSuggestStore{order: &order, suggestion: &db.StudySuggestion{ID: 42}}
	client := &fakeSuggestModalDiscordClient{
		order: &order,
		guildChannels: []*discordgo.Channel{
			{ID: "forum", Name: suggestionDiscussionChannelName, Type: discordgo.ChannelTypeGuildForum},
		},
		channels: map[string]*discordgo.Channel{
			"forum": {ID: "forum", Type: discordgo.ChannelTypeGuildForum},
		},
		forumThread: &discordgo.Channel{ID: "thread", LastMessageID: "starter-message"},
	}
	responder := &fakeSuggestResponder{order: &order}

	handler := newSuggestModalHandlerWithDeps(store, responder, client)
	handler(context.Background(), nil, newSuggestModalInteractionForTestWithCustomID("Go 스터디", "", "suggest_modal:anonymous:3:14:auto"))

	wantOrder := []string{"defer", "list-channels", "load-channel", "create-forum-thread", "create-suggestion", "add-reaction", "edit"}
	if len(order) != len(wantOrder) {
		t.Fatalf("expected order %v, got %v", wantOrder, order)
	}
	for idx := range wantOrder {
		if order[idx] != wantOrder[idx] {
			t.Fatalf("expected order %v, got %v", wantOrder, order)
		}
	}
}

func TestInteractionCommandNameUsesModalCustomID(t *testing.T) {
	got := interactionCommandName(newSuggestModalInteractionForTest("Go 스터디", ""))
	if got != "suggest_modal:anonymous:3:14:forum" {
		t.Fatalf("expected modal custom ID with options, got %q", got)
	}
}

type fakeSuggestStore struct {
	order      *[]string
	suggestion *db.StudySuggestion

	createErr error

	createdPeriodID  int64
	createdTitle     string
	createdDesc      string
	createdMessageID string
	createdChannelID string
}

func (f *fakeSuggestStore) appendOrder(value string) {
	if f.order != nil {
		*f.order = append(*f.order, value)
	}
}

func (f *fakeSuggestStore) CreateSuggestion(_ context.Context, params db.CreateSuggestionParams) (*db.StudySuggestion, error) {
	f.appendOrder("create-suggestion")
	f.createdPeriodID = params.PeriodID
	f.createdTitle = params.Title
	f.createdDesc = params.Description
	f.createdMessageID = params.MessageID
	f.createdChannelID = params.ChannelID
	if f.createErr != nil {
		return nil, f.createErr
	}
	return f.suggestion, nil
}

type fakeSuggestModalDiscordClient struct {
	order *[]string

	guildChannels []*discordgo.Channel
	channels      map[string]*discordgo.Channel
	forumThread   *discordgo.Channel
	sentMessage   *discordgo.Message

	reactionChannelID string
	reactionMessageID string
	reactionEmoji     string
}

func (f *fakeSuggestModalDiscordClient) appendOrder(value string) {
	if f.order != nil {
		*f.order = append(*f.order, value)
	}
}

func (f *fakeSuggestModalDiscordClient) Channel(channelID string, options ...discordgo.RequestOption) (*discordgo.Channel, error) {
	f.appendOrder("load-channel")
	ch := f.channels[channelID]
	if ch == nil {
		return nil, errors.New("channel not found")
	}
	return ch, nil
}

func (f *fakeSuggestModalDiscordClient) GuildChannels(guildID string, options ...discordgo.RequestOption) ([]*discordgo.Channel, error) {
	f.appendOrder("list-channels")
	return f.guildChannels, nil
}

func (f *fakeSuggestModalDiscordClient) ChannelMessageSendComplex(channelID string, data *discordgo.MessageSend, options ...discordgo.RequestOption) (*discordgo.Message, error) {
	f.appendOrder("send-message")
	if f.sentMessage == nil {
		return nil, errors.New("sent message not configured")
	}
	return f.sentMessage, nil
}

func (f *fakeSuggestModalDiscordClient) ForumThreadStartComplex(channelID string, threadData *discordgo.ThreadStart, messageData *discordgo.MessageSend, options ...discordgo.RequestOption) (*discordgo.Channel, error) {
	f.appendOrder("create-forum-thread")
	if f.forumThread == nil {
		return nil, errors.New("forum thread not configured")
	}
	return f.forumThread, nil
}

func (f *fakeSuggestModalDiscordClient) ChannelMessageDelete(channelID, messageID string, options ...discordgo.RequestOption) error {
	f.appendOrder("delete-message")
	return nil
}

func (f *fakeSuggestModalDiscordClient) MessageReactionAdd(channelID, messageID, emojiID string, options ...discordgo.RequestOption) error {
	f.appendOrder("add-reaction")
	f.reactionChannelID = channelID
	f.reactionMessageID = messageID
	f.reactionEmoji = emojiID
	return nil
}

type fakeSuggestResponder struct {
	order         *[]string
	deferCalls    int
	editCalls     int
	editedContent string
}

func (f *fakeSuggestResponder) deferEphemeral(context.Context, *discordgo.Session, *discordgo.InteractionCreate) error {
	if f.order != nil {
		*f.order = append(*f.order, "defer")
	}
	f.deferCalls++
	return nil
}

func (f *fakeSuggestResponder) editOriginal(_ context.Context, _ *discordgo.Session, _ *discordgo.InteractionCreate, content string) error {
	if f.order != nil {
		*f.order = append(*f.order, "edit")
	}
	f.editCalls++
	f.editedContent = content
	return nil
}

func newSuggestModalInteractionForTest(title, description string) *discordgo.InteractionCreate {
	return newSuggestModalInteractionForTestWithCustomID(title, description, "suggest_modal:anonymous:3:14:forum")
}

func newSuggestModalInteractionForTestWithCustomID(title, description, customID string) *discordgo.InteractionCreate {
	return &discordgo.InteractionCreate{Interaction: &discordgo.Interaction{
		ID:      "interaction-1",
		AppID:   "app-1",
		Token:   "token-1",
		Type:    discordgo.InteractionModalSubmit,
		GuildID: "guild-1",
		Data: discordgo.ModalSubmitInteractionData{
			CustomID: customID,
			Components: []discordgo.MessageComponent{
				&discordgo.ActionsRow{Components: []discordgo.MessageComponent{
					&discordgo.TextInput{CustomID: "title", Value: title},
				}},
				&discordgo.ActionsRow{Components: []discordgo.MessageComponent{
					&discordgo.TextInput{CustomID: "description", Value: description},
				}},
			},
		},
	}}
}
