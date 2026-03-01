package bot

import (
	"context"
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"
)

type triggeredCall struct {
	interactionID string
	commandName   string
	actorUserID   string
	guildID       string
	channelID     string
	optionsJSON   string
}

type fakeAuditStore struct {
	triggered []triggeredCall
	successes []string
	errors    []struct {
		interactionID string
		message       string
	}
}

func (f *fakeAuditStore) RecordTriggered(
	_ context.Context,
	interactionID, commandName, actorUserID, guildID, channelID, optionsJSON string,
) error {
	f.triggered = append(f.triggered, triggeredCall{
		interactionID: interactionID,
		commandName:   commandName,
		actorUserID:   actorUserID,
		guildID:       guildID,
		channelID:     channelID,
		optionsJSON:   optionsJSON,
	})
	return nil
}

func (f *fakeAuditStore) RecordSuccess(_ context.Context, interactionID string) error {
	f.successes = append(f.successes, interactionID)
	return nil
}

func (f *fakeAuditStore) RecordError(_ context.Context, interactionID, errorMessage string) error {
	f.errors = append(f.errors, struct {
		interactionID string
		message       string
	}{
		interactionID: interactionID,
		message:       errorMessage,
	})
	return nil
}

func TestRecordCommandTriggered(t *testing.T) {
	store := &fakeAuditStore{}
	setCommandAuditStore(store)
	defer setCommandAuditStore(nil)

	i := &discordgo.InteractionCreate{
		Interaction: &discordgo.Interaction{
			ID:        "12345",
			Type:      discordgo.InteractionApplicationCommand,
			GuildID:   "guild-1",
			ChannelID: "channel-1",
			Member: &discordgo.Member{
				User: &discordgo.User{ID: "user-1"},
			},
			Data: discordgo.ApplicationCommandInteractionData{
				Name: "members",
				Options: []*discordgo.ApplicationCommandInteractionDataOption{
					{
						Name:  "channel",
						Type:  discordgo.ApplicationCommandOptionString,
						Value: "channel-1",
					},
				},
			},
		},
	}

	recordCommandTriggered(i)

	if len(store.triggered) != 1 {
		t.Fatalf("expected one trigger record, got %d", len(store.triggered))
	}
	call := store.triggered[0]
	if call.interactionID != "12345" {
		t.Fatalf("expected interaction id 12345, got %q", call.interactionID)
	}
	if call.commandName != "members" {
		t.Fatalf("expected command members, got %q", call.commandName)
	}
	if call.actorUserID != "user-1" {
		t.Fatalf("expected actor user-1, got %q", call.actorUserID)
	}
	if !strings.Contains(call.optionsJSON, `"channel"`) {
		t.Fatalf("expected options json to include option name, got %s", call.optionsJSON)
	}
}

func TestRecordCommandResult(t *testing.T) {
	store := &fakeAuditStore{}
	setCommandAuditStore(store)
	defer setCommandAuditStore(nil)

	i := &discordgo.InteractionCreate{
		Interaction: &discordgo.Interaction{
			ID:      "abc",
			Type:    discordgo.InteractionApplicationCommand,
			GuildID: "guild-1",
			Data: discordgo.ApplicationCommandInteractionData{
				Name: "help",
			},
		},
	}

	recordCommandResult(i, "start", "started")
	recordCommandResult(i, "success", "done")
	recordCommandResult(i, "error", "failed")

	if len(store.successes) != 1 || store.successes[0] != "abc" {
		t.Fatalf("expected one success for interaction abc, got %+v", store.successes)
	}
	if len(store.errors) != 1 || store.errors[0].interactionID != "abc" {
		t.Fatalf("expected one error for interaction abc, got %+v", store.errors)
	}
}

func TestAutocompleteNotAudited(t *testing.T) {
	store := &fakeAuditStore{}
	setCommandAuditStore(store)
	defer setCommandAuditStore(nil)

	i := &discordgo.InteractionCreate{
		Interaction: &discordgo.Interaction{
			ID:      "auto-1",
			Type:    discordgo.InteractionApplicationCommandAutocomplete,
			GuildID: "guild-1",
			Data: discordgo.ApplicationCommandInteractionData{
				Name: "members",
			},
		},
	}

	recordCommandTriggered(i)
	recordCommandResult(i, "success", "done")
	recordCommandResult(i, "error", "failed")

	if len(store.triggered) != 0 || len(store.successes) != 0 || len(store.errors) != 0 {
		t.Fatalf("expected autocomplete to skip audit, got triggered=%d success=%d error=%d", len(store.triggered), len(store.successes), len(store.errors))
	}
}
