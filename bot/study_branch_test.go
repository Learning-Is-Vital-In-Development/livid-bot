package bot

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/bwmarrin/discordgo"
)

func TestIsValidBranch(t *testing.T) {
	testCases := []struct {
		branch string
		valid  bool
	}{
		{branch: "26-1", valid: true},
		{branch: "26-4", valid: true},
		{branch: "26-0", valid: false},
		{branch: "26-5", valid: false},
		{branch: "2026-1", valid: false},
		{branch: "aa-1", valid: false},
		{branch: "26-a", valid: false},
	}

	for _, tc := range testCases {
		if isValidBranch(tc.branch) != tc.valid {
			t.Fatalf("branch=%q expected valid=%v", tc.branch, tc.valid)
		}
	}
}

func TestNormalizeStudyName(t *testing.T) {
	testCases := []struct {
		name     string
		expected string
	}{
		{name: "26-2-algo", expected: "algo"},
		{name: "26-3- backend", expected: "backend"},
		{name: "algo", expected: "algo"},
		{name: "  algo  ", expected: "algo"},
		{name: "26-4-", expected: ""},
	}

	for _, tc := range testCases {
		if actual := normalizeStudyName(tc.name); actual != tc.expected {
			t.Fatalf("name=%q expected=%q got=%q", tc.name, tc.expected, actual)
		}
	}
}

func TestBuildStudyChannelName(t *testing.T) {
	testCases := []struct {
		branch   string
		name     string
		expected string
	}{
		{branch: "26-2", name: "algo", expected: "26-2-algo"},
		{branch: "26-1", name: "System Design", expected: "26-1-system-design"},
		{branch: "26-3", name: "C++", expected: "26-3-c"},
		{branch: "26-2", name: "네트워크", expected: "26-2-네트워크"},
		{branch: "26-2", name: "자바 스터디", expected: "26-2-자바-스터디"},
		{branch: "26-2", name: "Go 언어", expected: "26-2-go-언어"},
		{branch: "", name: "Go 언어", expected: "go-언어"},
		{branch: "", name: "🔥🔥🔥", expected: "study"},
	}

	for _, tc := range testCases {
		actual := buildStudyChannelName(tc.branch, tc.name)
		if actual != tc.expected {
			t.Fatalf("branch=%q name=%q expected=%q got=%q", tc.branch, tc.name, tc.expected, actual)
		}
	}
}

func TestSanitizeChannelName_TruncatesAt100(t *testing.T) {
	long := "26-2-" + strings.Repeat("a", 200)
	result := sanitizeChannelName(long)
	if len(result) > 100 {
		t.Fatalf("expected max 100 chars, got %d", len(result))
	}
}

func TestUniqueStudyChannelNameAddsSuffix(t *testing.T) {
	channels := []*discordgo.Channel{
		{Name: "26-2-go", Type: discordgo.ChannelTypeGuildText},
		{Name: "26-2-go-2", Type: discordgo.ChannelTypeGuildText},
	}

	got := uniqueStudyChannelName("26-2-go", channels)
	if got != "26-2-go-3" {
		t.Fatalf("expected suffix fallback, got %q", got)
	}
}

func TestSanitizeChannelName_TruncatesUnicodeSafely(t *testing.T) {
	long := strings.Repeat("가", 120)
	result := sanitizeChannelName(long)
	if !utf8.ValidString(result) {
		t.Fatalf("expected valid UTF-8 after truncation, got %q", result)
	}
	if runeCount := utf8.RuneCountInString(result); runeCount > 100 {
		t.Fatalf("expected max 100 runes, got %d", runeCount)
	}
}
