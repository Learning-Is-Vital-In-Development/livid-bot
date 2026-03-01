package bot

import (
	"strings"
	"testing"
	"time"

	"livid-bot/study"
)

func TestBuildMembersResponse(t *testing.T) {
	joinedAt := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)

	longName := strings.Repeat("스터디", 50)
	manyMembers := make([]study.StudyMember, 100)
	for i := range manyMembers {
		manyMembers[i] = study.StudyMember{
			StudyID:  1,
			UserID:   "111222333444555666",
			Username: "user",
			JoinedAt: joinedAt,
		}
	}

	cases := []struct {
		name      string
		studyName string
		members   []study.StudyMember
		contains  []string
		exact     string
		maxRunes  int
		hasSuffix string
	}{
		{
			name:      "two members",
			studyName: "알고리즘",
			members: []study.StudyMember{
				{StudyID: 1, UserID: "111", Username: "alice", JoinedAt: joinedAt},
				{StudyID: 1, UserID: "222", Username: "bob", JoinedAt: joinedAt.AddDate(0, 0, 5)},
			},
			contains: []string{
				"📚 **알고리즘** members (2)",
				"<@111>",
				"(joined: 2026-03-01)",
				"<@222>",
				"(joined: 2026-03-06)",
			},
		},
		{
			name:      "empty",
			studyName: "알고리즘",
			members:   nil,
			exact:     "No members found for study **알고리즘**.",
		},
		{
			name:      "truncation",
			studyName: longName,
			members:   manyMembers,
			maxRunes:  discordMessageLimit,
			hasSuffix: "...",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := buildMembersResponse(tc.studyName, tc.members)

			if tc.exact != "" && result != tc.exact {
				t.Fatalf("expected %q, got %q", tc.exact, result)
			}
			for _, s := range tc.contains {
				if !strings.Contains(result, s) {
					t.Fatalf("expected result to contain %q, got: %s", s, result)
				}
			}
			if tc.maxRunes > 0 && len([]rune(result)) > tc.maxRunes {
				t.Fatalf("response exceeds limit: %d runes", len([]rune(result)))
			}
			if tc.hasSuffix != "" && !strings.HasSuffix(result, tc.hasSuffix) {
				t.Fatalf("expected suffix %q, got: %s", tc.hasSuffix, result[len(result)-20:])
			}
		})
	}
}
