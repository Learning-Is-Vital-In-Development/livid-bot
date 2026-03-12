package bot

import (
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"
)

func TestVisibleCommandsForMemberNonAdmin(t *testing.T) {
	visible := visibleCommandsForMember(commands, 0, true)
	if len(visible) != 5 {
		t.Fatalf("expected 5 visible commands (/help, /members, /suggest-start, /suggest, /vote), got %d", len(visible))
	}
	if visible[0].Name != "help" || visible[1].Name != "members" {
		t.Fatalf("unexpected visible commands: %s, %s", visible[0].Name, visible[1].Name)
	}
	if visible[2].Name != "suggest-start" || visible[3].Name != "suggest" || visible[4].Name != "vote" {
		t.Fatalf("unexpected visible commands: %s, %s, %s", visible[2].Name, visible[3].Name, visible[4].Name)
	}
}

func TestVisibleCommandsForMemberAdmin(t *testing.T) {
	visible := visibleCommandsForMember(commands, discordgo.PermissionAdministrator, true)
	if len(visible) != len(commands) {
		t.Fatalf("expected all commands for admin, got %d/%d", len(visible), len(commands))
	}
}

func TestBuildHelpOverviewEmbed(t *testing.T) {
	embed := buildHelpOverviewEmbed(visibleCommandsForMember(commands, 0, true))
	if embed == nil {
		t.Fatal("expected embed")
	}
	if embed.Title != "도움말" {
		t.Fatalf("expected title 도움말, got %q", embed.Title)
	}
	if !strings.Contains(embed.Description, "`help`") {
		t.Fatalf("expected help in description, got: %s", embed.Description)
	}
	if !strings.Contains(embed.Description, "`members`") {
		t.Fatalf("expected members in description, got: %s", embed.Description)
	}
}

func TestBuildHelpOverviewEmbedTruncation(t *testing.T) {
	longDesc := strings.Repeat("긴설명", 1200)
	cmds := []*discordgo.ApplicationCommand{
		{Name: "a", Description: longDesc},
		{Name: "b", Description: longDesc},
	}

	embed := buildHelpOverviewEmbed(cmds)
	if len([]rune(embed.Description)) > helpEmbedDescriptionLimit {
		t.Fatalf("description exceeds limit: %d", len([]rune(embed.Description)))
	}
}

func TestBuildHelpCommandDetailEmbed(t *testing.T) {
	cmd := &discordgo.ApplicationCommand{
		Name:                     "sample",
		Description:              "샘플 명령어",
		DefaultMemberPermissions: int64Ptr(discordgo.PermissionAdministrator),
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
	}
	embed := buildHelpCommandDetailEmbed(cmd)

	if embed.Title != "sample" {
		t.Fatalf("expected title sample, got %q", embed.Title)
	}
	optionsField := findEmbedField(embed, "옵션")
	if optionsField == nil {
		t.Fatalf("expected options field")
	}
	if !strings.Contains(optionsField.Value, "`query` (문자열, 필수, 자동완성)") {
		t.Fatalf("expected query option in field, got: %s", optionsField.Value)
	}
	if !strings.Contains(optionsField.Value, "`dry-run` (불리언, 선택)") {
		t.Fatalf("expected dry-run option in field, got: %s", optionsField.Value)
	}
	permissionField := findEmbedField(embed, "권한")
	if permissionField == nil || permissionField.Value != "관리자 전용" {
		t.Fatalf("expected admin permission field, got: %+v", permissionField)
	}
}

func TestBuildHelpCommandAutocompleteChoices(t *testing.T) {
	cmds := []*discordgo.ApplicationCommand{
		{
			Name:        "help",
			Description: "사용 가능한 명령어 안내",
		},
		{
			Name:        "members",
			Description: "List active members of a study",
		},
		{
			Name:        "study-start",
			Description: "Close recruitment and start studies for a branch",
		},
	}

	choices := buildHelpCommandAutocompleteChoices(cmds, "study", 25)
	if len(choices) != 1 {
		t.Fatalf("expected one choice, got %d", len(choices))
	}
	if choices[0].Value != "study-start" {
		t.Fatalf("expected study-start value, got %v", choices[0].Value)
	}
	if choices[0].Name != "study-start" {
		t.Fatalf("expected choice label with command name, got %q", choices[0].Name)
	}
}

func TestFindVisibleCommandByName(t *testing.T) {
	cmds := []*discordgo.ApplicationCommand{
		{Name: "members"},
		{Name: "create-study"},
	}

	found := findVisibleCommandByName(cmds, "/create-study")
	if found == nil || found.Name != "create-study" {
		t.Fatalf("expected /create-study to be found, got %+v", found)
	}

	missing := findVisibleCommandByName(cmds, "unknown")
	if missing != nil {
		t.Fatalf("expected missing command to return nil, got %+v", missing)
	}
}

func TestNewHelpResponseDataEphemeral(t *testing.T) {
	embed := &discordgo.MessageEmbed{Title: "도움말"}
	data := newHelpResponseData(embed)
	if data.Flags != discordgo.MessageFlagsEphemeral {
		t.Fatalf("expected ephemeral flag, got %d", data.Flags)
	}
	if len(data.Embeds) != 1 || data.Embeds[0].Title != "도움말" {
		t.Fatalf("expected one embed with title 도움말, got %+v", data.Embeds)
	}
}

func findEmbedField(embed *discordgo.MessageEmbed, name string) *discordgo.MessageEmbedField {
	if embed == nil {
		return nil
	}
	for _, field := range embed.Fields {
		if field.Name == name {
			return field
		}
	}
	return nil
}
