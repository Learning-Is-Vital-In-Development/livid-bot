package bot

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/bwmarrin/discordgo"
	"livid-bot/study"
)

func TestBuildArchiveAllSummary(t *testing.T) {
	failures := []archiveFailure{
		{studyName: "study-a", reason: "channel move failed"},
		{studyName: "study-b", reason: "db archive failed"},
	}
	warnings := []string{"study-c: role deletion failed"}

	summary := buildArchiveAllSummary(5, 3, failures, warnings)

	if !strings.Contains(summary, "Archived **3/5** studies.") {
		t.Fatalf("unexpected summary header: %s", summary)
	}
	if !strings.Contains(summary, "study-a (channel move failed)") {
		t.Fatalf("expected first failure details in summary: %s", summary)
	}
	if !strings.Contains(summary, "Warnings: study-c: role deletion failed") {
		t.Fatalf("expected warnings in summary: %s", summary)
	}
}

func TestTruncateForDiscord(t *testing.T) {
	message := strings.Repeat("a", 30)
	truncated := truncateForDiscord(message, 10)

	if len([]rune(truncated)) != 10 {
		t.Fatalf("expected truncated rune length 10 but got %d", len([]rune(truncated)))
	}
	if !strings.HasSuffix(truncated, "...") {
		t.Fatalf("expected ellipsis suffix, got: %s", truncated)
	}
}

func TestBuildArchiveAllDryRunSummary(t *testing.T) {
	studyNames := []string{"study-a", "study-b", "study-c"}
	plan := archiveDryRunPlan{
		Assignments:       []string{"archive2", "archive2", "archive3"},
		CategoryUseCounts: map[string]int{"archive2": 2, "archive3": 1},
		CreatedCategories: []string{"archive3"},
	}

	summary := buildArchiveAllDryRunSummary(studyNames, plan)

	if !strings.Contains(summary, "Dry run: **3** active studies would be archived.") {
		t.Fatalf("unexpected dry-run header: %s", summary)
	}
	if !strings.Contains(summary, "Planned categories: archive2 (2), archive3 (1)") {
		t.Fatalf("expected planned categories in summary: %s", summary)
	}
	if !strings.Contains(summary, "Would create: archive3") {
		t.Fatalf("expected created category list in summary: %s", summary)
	}
	if !strings.Contains(summary, "1. study-a -> archive2") {
		t.Fatalf("expected preview mapping in summary: %s", summary)
	}
}

func TestFocusedStringOptionValue(t *testing.T) {
	options := []*discordgo.ApplicationCommandInteractionDataOption{
		{
			Name:    "channel",
			Type:    discordgo.ApplicationCommandOptionString,
			Value:   "1234567890",
			Focused: true,
		},
	}

	got := focusedStringOptionValue(options, "channel")
	if got != "1234567890" {
		t.Fatalf("expected focused value, got %q", got)
	}

	missing := focusedStringOptionValue(options, "name")
	if missing != "" {
		t.Fatalf("expected empty value for missing option, got %q", missing)
	}
}

func TestBuildArchiveStudyAutocompleteChoices(t *testing.T) {
	studies := []study.Study{
		{Name: "Algo", ChannelID: "111"},
		{Name: "Backend", ChannelID: "222"},
		{Name: "Frontend", ChannelID: "333"},
	}

	choices := buildArchiveStudyAutocompleteChoices(studies, "back", 25)
	if len(choices) != 1 {
		t.Fatalf("expected one filtered choice, got %d", len(choices))
	}
	if choices[0].Value != "222" {
		t.Fatalf("expected channel id 222, got %v", choices[0].Value)
	}

	limited := buildArchiveStudyAutocompleteChoices(studies, "", 2)
	if len(limited) != 2 {
		t.Fatalf("expected limited choices count=2, got %d", len(limited))
	}
}

func TestBuildArchiveStudyAutocompleteChoicesNameLimit(t *testing.T) {
	longName := strings.Repeat("a", 200)
	studies := []study.Study{
		{Name: longName, ChannelID: "1234567890"},
	}

	choices := buildArchiveStudyAutocompleteChoices(studies, "", 25)
	if len(choices) != 1 {
		t.Fatalf("expected one choice, got %d", len(choices))
	}

	label := choices[0].Name
	if utf8.RuneCountInString(label) > archiveAutocompleteChoiceNameLimit {
		t.Fatalf("choice label exceeds limit: %d", utf8.RuneCountInString(label))
	}
	if !strings.HasSuffix(label, " (<#1234567890>)") {
		t.Fatalf("choice label suffix missing channel info: %s", label)
	}
}
