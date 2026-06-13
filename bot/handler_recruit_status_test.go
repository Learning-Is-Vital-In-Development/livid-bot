package bot

import (
	"strings"
	"testing"
)

func TestBuildRecruitStatusSummary(t *testing.T) {
	summary := buildRecruitStatusSummary("26-2", []RecruitSignupSummary{
		{Emoji: "1️⃣", StudyName: "Go Concurrency", Count: minMembersToStart},
		{Emoji: "2️⃣", StudyName: "Kubernetes", Count: minMembersToStart - 1},
	})

	for _, want := range []string{
		"📊 26-2 모집 현황",
		"1️⃣ Go Concurrency",
		"신청: 3명",
		"상태: 시작 가능",
		"2️⃣ Kubernetes",
		"신청: 2명",
		"상태: 1명 부족",
	} {
		if !strings.Contains(summary, want) {
			t.Fatalf("expected status summary to contain %q, got: %s", want, summary)
		}
	}
}

func TestBuildRecruitStatusSummaryNoOpenRecruitment(t *testing.T) {
	summary := buildRecruitStatusSummary("26-2", nil)
	if !strings.Contains(summary, "열린 모집이 없습니다") {
		t.Fatalf("expected empty summary, got: %s", summary)
	}
}
