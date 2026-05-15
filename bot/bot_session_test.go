package bot

import (
	"testing"

	"github.com/bwmarrin/discordgo"
)

func TestConfigureDiscordSessionUsesSynchronousEventDispatch(t *testing.T) {
	session := &discordgo.Session{}

	configureDiscordSession(session)

	if !session.SyncEvents {
		t.Fatal("expected SyncEvents to preserve gateway event order for voice session logging")
	}
	if session.Identify.Intents&discordgo.IntentsGuildVoiceStates == 0 {
		t.Fatalf("expected guild voice state intent, got %v", session.Identify.Intents)
	}
}
