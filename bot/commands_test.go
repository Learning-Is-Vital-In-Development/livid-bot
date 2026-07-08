package bot

import (
	"testing"

	"github.com/bwmarrin/discordgo"
)

func TestCommandsExposeSuggestionFlowOnly(t *testing.T) {
	byName := make(map[string]*discordgo.ApplicationCommand, len(commands))
	for _, cmd := range commands {
		byName[cmd.Name] = cmd
	}

	removed := []string{
		"vote",
		"study-start",
		"create-study",
		"recruit",
		"recruit-status",
		"recruit-close",
		"suggest-start",
	}
	for _, name := range removed {
		if _, ok := byName[name]; ok {
			t.Fatalf("expected /%s command to be removed from the active slash-command surface", name)
		}
	}

	suggestCmd := byName["suggest"]
	if suggestCmd == nil {
		t.Fatal("expected /suggest command to exist")
	}
	if len(suggestCmd.Options) != 3 {
		t.Fatalf("expected /suggest to have visibility, threshold, duration_days options, got %d", len(suggestCmd.Options))
	}
	if suggestCmd.Options[0].Name != "visibility" || !suggestCmd.Options[0].Required {
		t.Fatalf("expected required visibility option, got %+v", suggestCmd.Options[0])
	}
	if suggestCmd.Options[1].Name != "threshold" || suggestCmd.Options[1].Required {
		t.Fatalf("expected optional threshold option, got %+v", suggestCmd.Options[1])
	}
	if suggestCmd.Options[2].Name != "duration_days" || suggestCmd.Options[2].Required {
		t.Fatalf("expected optional duration_days option, got %+v", suggestCmd.Options[2])
	}

	nudgeCmd := byName["study-nudge"]
	if nudgeCmd == nil {
		t.Fatal("expected /study-nudge command to exist")
	}
	assertAdminCommand(t, nudgeCmd)

	inviteCmd := byName["study-invite"]
	if inviteCmd == nil {
		t.Fatal("expected /study-invite command to exist")
	}
	if inviteCmd.DefaultMemberPermissions != nil {
		t.Fatalf("expected /study-invite to rely on handler authorization, got default permissions %v", inviteCmd.DefaultMemberPermissions)
	}
	if len(inviteCmd.Options) != 1 || inviteCmd.Options[0].Name != "user" || inviteCmd.Options[0].Type != discordgo.ApplicationCommandOptionUser || !inviteCmd.Options[0].Required {
		t.Fatalf("expected /study-invite to require one user option, got %+v", inviteCmd.Options)
	}

	for _, name := range []string{"help", "archive-study", "archive-all", "studies", "members"} {
		if byName[name] == nil {
			t.Fatalf("expected /%s command to remain available", name)
		}
	}
}

func assertAdminCommand(t *testing.T, cmd *discordgo.ApplicationCommand) {
	t.Helper()
	if cmd.DefaultMemberPermissions == nil || *cmd.DefaultMemberPermissions != discordgo.PermissionAdministrator {
		t.Fatalf("expected %s to require administrator permissions, got %v", cmd.Name, cmd.DefaultMemberPermissions)
	}
}
