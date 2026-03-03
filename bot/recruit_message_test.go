package bot

import (
	"strings"
	"testing"
	"time"

	"livid-bot/study"
)

func TestBuildRecruitMessage(t *testing.T) {
	studies := []study.Study{
		{Name: "algo", Description: "https://example.com/algo"},
		{Name: "go", Description: ""},
	}

	from := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC)

	msg := buildRecruitMessage("26-2", studies, from, to)

	if !strings.Contains(msg, "1️⃣ **algo** — https://example.com/algo") {
		t.Fatalf("expected first study line with description, got: %s", msg)
	}
	if !strings.Contains(msg, "대상 분기: **26-2**") {
		t.Fatalf("expected branch line in message, got: %s", msg)
	}
	if !strings.Contains(msg, "2️⃣ **go**") {
		t.Fatalf("expected second study line, got: %s", msg)
	}
	if strings.Contains(msg, "2️⃣ **go** —") {
		t.Fatalf("did not expect description separator for empty description, got: %s", msg)
	}
	if !strings.Contains(msg, "📅 모집 기간: 2026-03-01 ~ 2026-03-10") {
		t.Fatalf("expected date range in message, got: %s", msg)
	}
}
