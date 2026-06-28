package bot

import (
	"strings"
	"testing"
)

func TestBuildRecruitStatusEmbed(t *testing.T) {
	embed := buildRecruitStatusEmbed("26-2", []RecruitSignupSummary{
		{Emoji: "1️⃣", StudyName: "Go Concurrency", StudyChannelID: "111", Count: minMembersToStart},
		{Emoji: "2️⃣", StudyName: "Kubernetes", StudyChannelID: "222", Count: minMembersToStart - 1},
		{Emoji: "3️⃣", StudyName: "No Signups", Count: 0},
	})

	if embed.Title != "📊 26-2 모집 현황" {
		t.Fatalf("unexpected title: %q", embed.Title)
	}
	if embed.Color != discordEmbedColorYellow {
		t.Fatalf("expected yellow status color, got %d", embed.Color)
	}
	for _, want := range []string{
		"최소 시작 인원: **3명**",
		"총 신청: **5명**",
		"시작 가능: **1/3개**",
	} {
		if !strings.Contains(embed.Description, want) {
			t.Fatalf("expected description to contain %q, got: %s", want, embed.Description)
		}
	}
	if len(embed.Fields) != 3 {
		t.Fatalf("expected 3 fields, got %d", len(embed.Fields))
	}
	if embed.Fields[0].Name != "1️⃣ Go Concurrency" {
		t.Fatalf("unexpected first field name: %q", embed.Fields[0].Name)
	}
	if !strings.Contains(embed.Fields[0].Value, "✅ 시작 가능") || !strings.Contains(embed.Fields[0].Value, "채널: <#111>") {
		t.Fatalf("unexpected first field value: %s", embed.Fields[0].Value)
	}
	if !strings.Contains(embed.Fields[1].Value, "🟡 1명 부족") {
		t.Fatalf("unexpected second field value: %s", embed.Fields[1].Value)
	}
	if !strings.Contains(embed.Fields[2].Value, "🔴 3명 부족") {
		t.Fatalf("unexpected third field value: %s", embed.Fields[2].Value)
	}
}

func TestBuildRecruitStatusEmbedNoOpenRecruitment(t *testing.T) {
	embed := buildRecruitStatusEmbed("26-2", nil)
	if embed.Color != discordEmbedColorGray {
		t.Fatalf("expected gray empty status color, got %d", embed.Color)
	}
	if !strings.Contains(embed.Description, "열린 모집이 없습니다") {
		t.Fatalf("expected empty summary, got: %s", embed.Description)
	}
}
