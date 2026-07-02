package bot

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/bwmarrin/discordgo"
	"livid-bot/study"
)

func TestBuildArchiveStudySuccessMessage(t *testing.T) {
	moved := buildArchiveStudySuccessMessage("study-a", archiveResult{CategoryName: "archive2"})
	if moved != "**study-a** 스터디를 아카이브했습니다 (**archive2**로 이동)." {
		t.Fatalf("unexpected moved message: %s", moved)
	}

	missing := buildArchiveStudySuccessMessage("study-b", archiveResult{Warning: "channel already missing; archived DB row only"})
	if !strings.Contains(missing, "**study-b** 스터디를 아카이브했습니다 (DB 상태만 변경).") {
		t.Fatalf("expected DB-only archive message: %s", missing)
	}
	if !strings.Contains(missing, "주의: channel already missing; archived DB row only.") {
		t.Fatalf("expected missing-channel warning: %s", missing)
	}
}

func TestIsDiscordAPIErrorCode(t *testing.T) {
	restErr := &discordgo.RESTError{
		Response:     &http.Response{Status: "404 Not Found"},
		ResponseBody: []byte(`{"message":"Unknown Channel","code":10003}`),
		Message:      &discordgo.APIErrorMessage{Code: discordgo.ErrCodeUnknownChannel, Message: "Unknown Channel"},
	}
	wrapped := fmt.Errorf("wrapped: %w", restErr)

	if !isDiscordAPIErrorCode(wrapped, discordgo.ErrCodeUnknownChannel) {
		t.Fatal("expected wrapped unknown-channel REST error to match")
	}
	if isDiscordAPIErrorCode(wrapped, discordgo.ErrCodeUnknownRole) {
		t.Fatal("did not expect unknown-channel error to match unknown-role code")
	}
	if isDiscordAPIErrorCode(fmt.Errorf("plain error"), discordgo.ErrCodeUnknownChannel) {
		t.Fatal("did not expect non-REST error to match Discord API code")
	}
}

func TestBuildArchiveAllSummary(t *testing.T) {
	failures := []archiveFailure{
		{studyName: "study-a", reason: "channel move failed"},
		{studyName: "study-b", reason: "db archive failed"},
	}
	warnings := []string{"study-c: role deletion failed"}

	summary := buildArchiveAllSummary(5, 3, failures, warnings)

	if !strings.Contains(summary, "스터디 **3/5**개를 아카이브했습니다.") {
		t.Fatalf("unexpected summary header: %s", summary)
	}
	if !strings.Contains(summary, "study-a (channel move failed)") {
		t.Fatalf("expected first failure details in summary: %s", summary)
	}
	if !strings.Contains(summary, "주의: study-c: role deletion failed") {
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

	if !strings.Contains(summary, "미리보기: 활성 스터디 **3**개가 아카이브될 예정입니다.") {
		t.Fatalf("unexpected dry-run header: %s", summary)
	}
	if !strings.Contains(summary, "예정 카테고리: archive2 (2), archive3 (1)") {
		t.Fatalf("expected planned categories in summary: %s", summary)
	}
	if !strings.Contains(summary, "새로 만들 카테고리: archive3") {
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
