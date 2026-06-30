package bot

import (
	"testing"

	"github.com/bwmarrin/discordgo"
)

func TestCommandsUseRecruitCloseAndStatusWithoutVoteOrStudyStart(t *testing.T) {
	byName := make(map[string]*discordgo.ApplicationCommand, len(commands))
	for _, cmd := range commands {
		byName[cmd.Name] = cmd
	}

	if _, ok := byName["vote"]; ok {
		t.Fatal("expected /vote command to be removed")
	}
	if _, ok := byName["study-start"]; ok {
		t.Fatal("expected /study-start command to be renamed to /recruit-close")
	}

	closeCmd := byName["recruit-close"]
	if closeCmd == nil {
		t.Fatal("expected /recruit-close command to exist")
	}
	assertAdminCommand(t, closeCmd)
	assertBranchAutocompleteOption(t, closeCmd)

	statusCmd := byName["recruit-status"]
	if statusCmd == nil {
		t.Fatal("expected /recruit-status command to exist")
	}
	assertAdminCommand(t, statusCmd)
	assertBranchAutocompleteOption(t, statusCmd)

	nudgeCmd := byName["study-nudge"]
	if nudgeCmd == nil {
		t.Fatal("expected /study-nudge command to exist")
	}
	assertAdminCommand(t, nudgeCmd)

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
}

func assertAdminCommand(t *testing.T, cmd *discordgo.ApplicationCommand) {
	t.Helper()
	if cmd.DefaultMemberPermissions == nil || *cmd.DefaultMemberPermissions != discordgo.PermissionAdministrator {
		t.Fatalf("expected %s to require administrator permissions, got %v", cmd.Name, cmd.DefaultMemberPermissions)
	}
}

func assertBranchAutocompleteOption(t *testing.T, cmd *discordgo.ApplicationCommand) {
	t.Helper()
	if len(cmd.Options) != 1 {
		t.Fatalf("expected %s to have exactly one branch option, got %d", cmd.Name, len(cmd.Options))
	}
	opt := cmd.Options[0]
	if opt.Name != "branch" || opt.Type != discordgo.ApplicationCommandOptionString || !opt.Required || !opt.Autocomplete {
		t.Fatalf("expected %s branch option to be required autocomplete string, got %+v", cmd.Name, opt)
	}
}
