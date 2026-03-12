package bot

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestParseProposalDeadlineUsesKSTEndOfDay(t *testing.T) {
	now := time.Date(2026, 3, 12, 9, 0, 0, 0, proposalDeadlineLocation)

	got, err := parseProposalDeadline("2026-03-13", now)
	if err != nil {
		t.Fatalf("expected deadline parse to succeed, got error: %v", err)
	}

	if got.Location().String() != proposalDeadlineLocation.String() {
		t.Fatalf("expected location %q, got %q", proposalDeadlineLocation, got.Location())
	}
	if got.Hour() != 23 || got.Minute() != 59 || got.Second() != 59 {
		t.Fatalf("expected end-of-day deadline, got %s", got)
	}
	if proposalDateLabel(got) != "2026-03-13" {
		t.Fatalf("expected formatted label 2026-03-13, got %q", proposalDateLabel(got))
	}
}

func TestParseProposalDeadlineRejectsPastOrElapsedDate(t *testing.T) {
	now := time.Date(2026, 3, 12, 23, 59, 59, 0, proposalDeadlineLocation)

	if _, err := parseProposalDeadline("2026-03-12", now); !errors.Is(err, errProposalDeadlinePast) {
		t.Fatalf("expected errProposalDeadlinePast for same-day elapsed deadline, got %v", err)
	}
	if _, err := parseProposalDeadline("2026-03-11", now); !errors.Is(err, errProposalDeadlinePast) {
		t.Fatalf("expected errProposalDeadlinePast for past deadline, got %v", err)
	}
}

func TestBuildProposalMessage(t *testing.T) {
	withDescription := buildProposalMessage("Go 스터디", "동시성 중심", 2)
	if !strings.Contains(withDescription, "**주제**: Go 스터디") {
		t.Fatalf("expected title in message, got: %s", withDescription)
	}
	if !strings.Contains(withDescription, "설명: 동시성 중심") {
		t.Fatalf("expected description in message, got: %s", withDescription)
	}
	if !strings.Contains(withDescription, "🚀 2표") {
		t.Fatalf("expected vote count in message, got: %s", withDescription)
	}

	withoutDescription := buildProposalMessage("Rust 스터디", "", 0)
	if strings.Contains(withoutDescription, "설명:") {
		t.Fatalf("did not expect description line for empty description, got: %s", withoutDescription)
	}
	if !strings.Contains(withoutDescription, "🚀 0표") {
		t.Fatalf("expected zero vote count in message, got: %s", withoutDescription)
	}
}

func TestUpdateVoteLine(t *testing.T) {
	updated := updateVoteLine("제안 본문\n🚀 1표", 3)
	if !strings.Contains(updated, "🚀 3표") {
		t.Fatalf("expected updated vote line, got: %s", updated)
	}

	appended := updateVoteLine("제안 본문", 1)
	if !strings.HasSuffix(appended, "\n🚀 1표") {
		t.Fatalf("expected vote line to be appended, got: %s", appended)
	}
}
