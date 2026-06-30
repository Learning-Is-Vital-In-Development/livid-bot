package bot

import (
	"strings"
	"testing"
	"time"

	"livid-bot/db"
)

func TestBuildStudyNudgeMessagePointsToOriginalSuggestions(t *testing.T) {
	msg := buildStudyNudgeMessage("guild-1", []*db.StudySuggestion{
		{
			Title:     "Go 스터디",
			ChannelID: "thread-1",
			MessageID: "message-1",
			VoteCount: 2,
			Threshold: 3,
			ExpiresAt: time.Date(2026, 7, 14, 23, 59, 59, 0, suggestionDeadlineLocation),
		},
	})

	for _, want := range []string{"@everyone", "Go 스터디", "🚀 2 / 3", "2026-07-14", "https://discord.com/channels/guild-1/thread-1/message-1"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("expected %q in nudge message, got: %s", want, msg)
		}
	}
}
