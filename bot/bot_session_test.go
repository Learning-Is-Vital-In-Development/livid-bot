package bot

import (
	"testing"

	"github.com/bwmarrin/discordgo"
)

func TestConfigureDiscordSessionDoesNotRequestVoiceStateIntent(t *testing.T) {
	session := &discordgo.Session{}

	configureDiscordSession(session)

	if session.Identify.Intents&discordgo.IntentsGuildVoiceStates != 0 {
		t.Fatalf("did not expect guild voice state intent, got %v", session.Identify.Intents)
	}
}
