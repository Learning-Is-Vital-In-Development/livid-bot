package bot

import (
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"
	"livid-bot/study"
)

func TestParseStudiesFilters(t *testing.T) {
	options := []*discordgo.ApplicationCommandInteractionDataOption{
		{
			Name:  "branch",
			Type:  discordgo.ApplicationCommandOptionString,
			Value: "26-2",
		},
	}

	branch, status := parseStudiesFilters(options)
	if branch != "26-2" {
		t.Fatalf("expected branch 26-2, got %q", branch)
	}
	if status != "active" {
		t.Fatalf("expected default status active, got %q", status)
	}
}

func TestBuildStudiesResponse(t *testing.T) {
	studies := []study.Study{
		{Branch: "26-2", Name: "algo", Status: "active", ChannelID: "111"},
		{Branch: "26-2", Name: "backend", Status: "active", ChannelID: "222"},
	}

	message := buildStudiesResponse("26-2", "active", studies)

	if !strings.Contains(message, "- [26-2] algo (active) <#111>") {
		t.Fatalf("expected formatted first row, got: %s", message)
	}
	if !strings.Contains(message, "- [26-2] backend (active) <#222>") {
		t.Fatalf("expected formatted second row, got: %s", message)
	}
}
