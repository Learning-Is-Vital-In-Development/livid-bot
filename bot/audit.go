package bot

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"

	"github.com/bwmarrin/discordgo"
)

const unknownAuditValue = "unknown"

type CommandAuditStore interface {
	RecordTriggered(ctx context.Context, interactionID, commandName, actorUserID, guildID, channelID, optionsJSON string) error
	RecordSuccess(ctx context.Context, interactionID string) error
	RecordError(ctx context.Context, interactionID, errorMessage string) error
}

type noopAuditStore struct{}

func (noopAuditStore) RecordTriggered(context.Context, string, string, string, string, string, string) error {
	return nil
}

func (noopAuditStore) RecordSuccess(context.Context, string) error {
	return nil
}

func (noopAuditStore) RecordError(context.Context, string, string) error {
	return nil
}

var (
	commandAuditStoreMu sync.RWMutex
	commandAuditStore   CommandAuditStore = noopAuditStore{}
)

func setCommandAuditStore(store CommandAuditStore) {
	if store == nil {
		store = noopAuditStore{}
	}

	commandAuditStoreMu.Lock()
	commandAuditStore = store
	commandAuditStoreMu.Unlock()
}

func getCommandAuditStore() CommandAuditStore {
	commandAuditStoreMu.RLock()
	defer commandAuditStoreMu.RUnlock()
	return commandAuditStore
}

func recordCommandTriggered(i *discordgo.InteractionCreate) {
	if !isApplicationCommandInteraction(i) {
		return
	}

	interactionID := interactionID(i)
	if interactionID == "" {
		return
	}

	optionsJSON, err := marshalCommandOptions(i)
	if err != nil {
		slog.Warn("failed to marshal command options for audit", "interaction_id", interactionID, "error", err)
		optionsJSON = "[]"
	}

	err = getCommandAuditStore().RecordTriggered(
		context.Background(),
		interactionID,
		interactionCommandName(i),
		interactionUserID(i),
		interactionGuildID(i),
		interactionChannelID(i),
		optionsJSON,
	)
	if err != nil {
		slog.Warn("failed to write command audit trigger", "interaction_id", interactionID, "error", err)
	}
}

func recordCommandResult(i *discordgo.InteractionCreate, stage, message string) {
	if !isApplicationCommandInteraction(i) {
		return
	}

	interactionID := interactionID(i)
	if interactionID == "" {
		return
	}

	var err error
	switch stage {
	case "success":
		err = getCommandAuditStore().RecordSuccess(context.Background(), interactionID)
	case "error":
		err = getCommandAuditStore().RecordError(context.Background(), interactionID, message)
	default:
		return
	}

	if err != nil {
		slog.Warn("failed to write command audit result", "interaction_id", interactionID, "stage", stage, "error", err)
	}
}

func isApplicationCommandInteraction(i *discordgo.InteractionCreate) bool {
	return i != nil && i.Type == discordgo.InteractionApplicationCommand
}

func interactionID(i *discordgo.InteractionCreate) string {
	if i == nil || i.Interaction == nil || i.ID == "" {
		return ""
	}
	return i.ID
}

func interactionGuildID(i *discordgo.InteractionCreate) string {
	if i == nil || i.GuildID == "" {
		return unknownAuditValue
	}
	return i.GuildID
}

func interactionChannelID(i *discordgo.InteractionCreate) string {
	if i == nil || i.ChannelID == "" {
		return unknownAuditValue
	}
	return i.ChannelID
}

func marshalCommandOptions(i *discordgo.InteractionCreate) (string, error) {
	if i == nil {
		return "[]", nil
	}

	options := i.ApplicationCommandData().Options
	if len(options) == 0 {
		return "[]", nil
	}

	raw, err := json.Marshal(options)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}
