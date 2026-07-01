package bot

import (
	"strings"
	"testing"
	"time"

	"livid-bot/study"
)

func TestBuildMembersEmbed(t *testing.T) {
	joinedAt := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)

	cases := []struct {
		name        string
		studyName   string
		members     []study.StudyMember
		wantDesc    string
		wantFields  int
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
			wantDesc:   "총 **2명**",
			wantFields: 2,
			contains: []string{
				"<@111>",
				"참여일: `2026-03-01`",
				"<@222>",
				"참여일: `2026-03-06`",
			},
		},
		{
			name:      "fallback to mention without username",
			studyName: "알고리즘",
			members: []study.StudyMember{
				{StudyID: 1, UserID: "111", JoinedAt: joinedAt},
			},
			wantDesc:   "총 **1명**",
			wantFields: 1,
			contains: []string{
				"<@111>",
				"참여일: `2026-03-01`",
			},
		},
		{
			name:       "empty",
			studyName:  "알고리즘",
			members:    nil,
			wantDesc:   "등록된 멤버가 없습니다.",
			wantFields: 0,
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
			wantDesc:    "총 **30명**",
			wantFields:  discordEmbedMaxFields,
			wantOmitted: "6명이 더 있습니다.",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			embed := buildMembersEmbed(tc.studyName, tc.members)

			if embed.Title != "📚 "+tc.studyName+" 멤버" {
				t.Fatalf("unexpected title: %q", embed.Title)
			}
			if embed.Description != tc.wantDesc {
				t.Fatalf("expected description %q, got %q", tc.wantDesc, embed.Description)
			}
			if len(embed.Fields) != tc.wantFields {
				t.Fatalf("expected %d fields, got %d", tc.wantFields, len(embed.Fields))
			}

			combined := embed.Description
			for _, field := range embed.Fields {
				combined += "\n" + field.Name + "\n" + field.Value
			}
			for _, want := range tc.contains {
				if !strings.Contains(combined, want) {
					t.Fatalf("expected embed to contain %q, got: %s", want, combined)
				}
			}
			if tc.wantOmitted != "" && !strings.Contains(combined, tc.wantOmitted) {
				t.Fatalf("expected omitted marker %q, got: %s", tc.wantOmitted, combined)
			}
			if tc.name == "two members" {
				if embed.Fields[0].Name != "1." {
					t.Fatalf("expected mention in field value, got field name %q", embed.Fields[0].Name)
				}
				if !strings.Contains(embed.Fields[0].Value, "<@111>") {
					t.Fatalf("expected first field value to contain mention, got %q", embed.Fields[0].Value)
				}
			}
		})
	}
}
