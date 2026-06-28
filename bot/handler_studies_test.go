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

func TestBuildStudiesEmbed(t *testing.T) {
	studies := []study.Study{
		{Branch: "26-2", Name: "algo", Status: "active", ChannelID: "111", RoleID: "999", Description: "algorithm"},
		{Branch: "26-2", Name: "backend", Status: "active", ChannelID: "222"},
	}

	embed := buildStudiesEmbed("26-2", "active", studies)

	if embed.Title != "📚 스터디 목록" {
		t.Fatalf("unexpected title: %q", embed.Title)
	}
	if embed.Color != discordEmbedColorBlurple {
		t.Fatalf("unexpected color: %d", embed.Color)
	}
	if !strings.Contains(embed.Description, "분기: **26-2**") {
		t.Fatalf("unexpected description: %s", embed.Description)
	}
	if len(embed.Fields) != 2 {
		t.Fatalf("expected 2 fields, got %d", len(embed.Fields))
	}
	if embed.Fields[0].Name != "[26-2] algo" {
		t.Fatalf("unexpected first field name: %q", embed.Fields[0].Name)
	}
	for _, want := range []string{"상태: `active`", "채널: <#111>", "역할: <@&999>", "설명: algorithm"} {
		if !strings.Contains(embed.Fields[0].Value, want) {
			t.Fatalf("expected first field to contain %q, got: %s", want, embed.Fields[0].Value)
		}
	}
	if !strings.Contains(embed.Fields[1].Value, "채널: <#222>") {
		t.Fatalf("expected second channel, got: %s", embed.Fields[1].Value)
	}
}

func TestBuildStudiesEmbedEmpty(t *testing.T) {
	embed := buildStudiesEmbed("", "archived", nil)
	if embed.Color != discordEmbedColorGray {
		t.Fatalf("expected archived gray color, got %d", embed.Color)
	}
	if !strings.Contains(embed.Description, "조건에 맞는 스터디가 없습니다") {
		t.Fatalf("unexpected empty description: %s", embed.Description)
	}
}
