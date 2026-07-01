package bot

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"livid-bot/study"
)

func TestBuildMembersMessage(t *testing.T) {
	joinedAt := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)

	cases := []struct {
		name        string
		studyName   string
		members     []study.StudyMember
		contains    []string
		wantOmitted string
	}{
		{
			name:      "two members",
			studyName: "알고리즘",
			members: []study.StudyMember{
				{StudyID: 1, UserID: "111", Username: "alice", JoinedAt: joinedAt},
				{StudyID: 1, UserID: "222", Username: "bob", JoinedAt: joinedAt.AddDate(0, 0, 5)},
			},
			contains: []string{
				"📚 **알고리즘 멤버**",
				"총 **2명**",
				"1. <@111>",
				"참여일: `2026-03-01`",
				"2. <@222>",
				"참여일: `2026-03-06`",
				"조회 기준 · /members",
			},
		},
		{
			name:      "fallback to username without user id",
			studyName: "알고리즘",
			members: []study.StudyMember{
				{StudyID: 1, Username: "alice", JoinedAt: joinedAt},
			},
			contains: []string{
				"1. alice",
				"참여일: `2026-03-01`",
			},
		},
		{
			name:      "empty",
			studyName: "알고리즘",
			contains: []string{
				"📚 **알고리즘 멤버**",
				"등록된 멤버가 없습니다.",
				"조회 기준 · /members",
			},
		},
		{
			name:      "field cap",
			studyName: "알고리즘",
			members: func() []study.StudyMember {
				members := make([]study.StudyMember, 30)
				for i := range members {
					members[i] = study.StudyMember{UserID: "111222333444555666", JoinedAt: joinedAt}
				}
				return members
			}(),
			contains: []string{
				"총 **30명**",
			},
			wantOmitted: "6명이 더 있습니다.",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			message := buildMembersMessage(tc.studyName, tc.members)
			for _, want := range tc.contains {
				if !strings.Contains(message, want) {
					t.Fatalf("expected message to contain %q, got: %s", want, message)
				}
			}
			if tc.wantOmitted != "" && !strings.Contains(message, tc.wantOmitted) {
				t.Fatalf("expected omitted marker %q, got: %s", tc.wantOmitted, message)
			}
		})
	}
}

func TestMemberMentionIDs(t *testing.T) {
	members := []study.StudyMember{
		{UserID: "111"},
		{Username: "missing-id"},
		{UserID: "222"},
	}
	want := []string{"111", "222"}
	if got := memberMentionIDs(members); !reflect.DeepEqual(got, want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
}
