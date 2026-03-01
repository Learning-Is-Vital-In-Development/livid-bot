package bot

import (
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"
)

func TestBuildHelpResponseNonAdmin(t *testing.T) {
	content, visibleCount := buildHelpResponse(commands, 0, true)

	if visibleCount != 2 {
		t.Fatalf("expected 2 visible commands (/help, /members), got %d", visibleCount)
	}
	if !strings.Contains(content, "/help - Show available commands") {
		t.Fatalf("expected /help to be visible, got: %s", content)
	}
	if !strings.Contains(content, "/members - List active members of a study") {
		t.Fatalf("expected /members to be visible, got: %s", content)
	}
	if strings.Contains(content, "/create-study") {
		t.Fatalf("did not expect admin command for non-admin user, got: %s", content)
	}
}

func TestBuildHelpResponseAdmin(t *testing.T) {
	content, visibleCount := buildHelpResponse(commands, discordgo.PermissionAdministrator, true)

	if visibleCount != len(commands) {
		t.Fatalf("expected all commands to be visible for admin, got %d/%d", visibleCount, len(commands))
	}
	if !strings.Contains(content, "/archive-all - Archive all active studies") {
		t.Fatalf("expected /archive-all to be visible for admin, got: %s", content)
	}
}

func TestBuildHelpResponseNoMemberContext(t *testing.T) {
	content, visibleCount := buildHelpResponse(commands, discordgo.PermissionAdministrator, false)

	if visibleCount != 2 {
		t.Fatalf("expected only unrestricted commands when member context is missing, got %d", visibleCount)
	}
	if strings.Contains(content, "/recruit - Post a recruitment message for active studies") {
		t.Fatalf("did not expect admin command without member context, got: %s", content)
	}
}

func TestBuildHelpResponseOptionFormatting(t *testing.T) {
	customCommands := []*discordgo.ApplicationCommand{
		{
			Name:        "help",
			Description: "Show available commands",
		},
		{
			Name:        "sample",
			Description: "Sample command",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:         discordgo.ApplicationCommandOptionString,
					Name:         "query",
					Required:     true,
					Autocomplete: true,
				},
				{
					Type:     discordgo.ApplicationCommandOptionBoolean,
					Name:     "dry-run",
					Required: false,
				},
			},
		},
	}

	content, _ := buildHelpResponse(customCommands, 0, true)

	if !strings.Contains(content, "`query` (string, required, autocomplete)") {
		t.Fatalf("expected formatted required+autocomplete option, got: %s", content)
	}
	if !strings.Contains(content, "`dry-run` (boolean, optional)") {
		t.Fatalf("expected formatted optional option, got: %s", content)
	}
}

func TestBuildHelpResponseTruncation(t *testing.T) {
	longDesc := strings.Repeat("description ", 400)
	customCommands := []*discordgo.ApplicationCommand{
		{
			Name:        "help",
			Description: longDesc,
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:     discordgo.ApplicationCommandOptionString,
					Name:     "very-long-option-name",
					Required: true,
				},
			},
		},
	}

	content, _ := buildHelpResponse(customCommands, 0, true)

	if len([]rune(content)) > discordMessageLimit {
		t.Fatalf("response exceeds limit: %d runes", len([]rune(content)))
	}
	if !strings.HasSuffix(content, "...") {
		t.Fatalf("expected truncated response to end with ellipsis, got: %s", content)
	}
}

func TestNewHelpResponseDataEphemeral(t *testing.T) {
	data := newHelpResponseData("content")
	if data.Flags != discordgo.MessageFlagsEphemeral {
		t.Fatalf("expected ephemeral flag, got %d", data.Flags)
	}
}
