package db

import (
	"context"
	"testing"
	"time"

	"livid-bot/study"
)

func TestMigrateAddsRecruitScheduleColumns(t *testing.T) {
	tdb := newTestDatabase(t)
	ctx := context.Background()

	var count int
	err := tdb.Pool.QueryRow(ctx,
		`SELECT COUNT(*)
		 FROM information_schema.columns
		 WHERE table_schema = 'public'
		   AND table_name = 'recruit_messages'
		   AND column_name IN ('branch', 'opens_at', 'closes_at')`,
	).Scan(&count)
	if err != nil {
		t.Fatalf("query recruit schedule columns: %v", err)
	}
	if count != 3 {
		t.Fatalf("expected recruit_messages branch/opens_at/closes_at columns, got %d", count)
	}
}

func TestSaveRecruitMessageStoresBranchDatesAndOpenMappings(t *testing.T) {
	tdb := newTestDatabase(t)
	ctx := context.Background()
	repo := NewRecruitRepository(tdb.Pool)

	var studyID int64
	err := tdb.Pool.QueryRow(ctx,
		`INSERT INTO studies (branch, name, description, channel_id, role_id, status)
		 VALUES ($1, $2, $3, $4, $5, 'active')
		 RETURNING id`,
		"26-2", "Go", "", "study-channel", "role-go",
	).Scan(&studyID)
	if err != nil {
		t.Fatalf("insert study: %v", err)
	}

	opensAt := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	closesAt := time.Date(2026, 7, 7, 23, 59, 59, 0, time.UTC)
	if err := repo.SaveRecruitMessage(ctx, SaveRecruitMessageParams{
		MessageID: "recruit-message",
		ChannelID: "recruit-channel",
		Branch:    "26-2",
		OpensAt:   opensAt,
		ClosesAt:  closesAt,
		Mappings: []study.RecruitMapping{
			{Emoji: "1️⃣", StudyID: studyID, RoleID: "role-go"},
		},
	}); err != nil {
		t.Fatalf("save recruit message: %v", err)
	}

	mappings, err := repo.FindOpenRecruitMappingsByBranch(ctx, "26-2")
	if err != nil {
		t.Fatalf("find open recruit mappings: %v", err)
	}
	if len(mappings) != 1 {
		t.Fatalf("expected one open mapping, got %d", len(mappings))
	}
	mapping := mappings[0]
	if mapping.RecruitMessageID != "recruit-message" || mapping.RecruitChannelID != "recruit-channel" {
		t.Fatalf("unexpected recruit message refs: %+v", mapping)
	}
	if mapping.Branch != "26-2" || !mapping.OpensAt.Equal(opensAt) || !mapping.ClosesAt.Equal(closesAt) {
		t.Fatalf("unexpected recruit schedule fields: %+v", mapping)
	}
	if mapping.Emoji != "1️⃣" || mapping.StudyID != studyID || mapping.StudyName != "Go" || mapping.StudyChannelID != "study-channel" || mapping.RoleID != "role-go" {
		t.Fatalf("unexpected study mapping: %+v", mapping)
	}
}
