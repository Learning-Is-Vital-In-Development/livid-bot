package db

import (
	"context"
	"testing"
)

func TestMemberRepositoryAddRemoveAndReAdd(t *testing.T) {
	tdb := newTestDatabase(t)
	ctx := context.Background()
	studyRepo := NewStudyRepository(tdb.Pool)
	memberRepo := NewMemberRepository(tdb.Pool)

	st, err := studyRepo.Create(ctx, "", "분산 시스템", "", "channel-1", "role-1")
	if err != nil {
		t.Fatalf("create study: %v", err)
	}

	if err := memberRepo.AddMember(ctx, st.ID, "user-1", "first-name"); err != nil {
		t.Fatalf("add member: %v", err)
	}
	if err := memberRepo.AddMember(ctx, st.ID, "user-1", "updated-name"); err != nil {
		t.Fatalf("add same member again: %v", err)
	}

	assertActiveMemberCount(t, ctx, tdb, st.ID, "user-1", 1)
	members, err := memberRepo.FindActiveByStudyID(ctx, st.ID)
	if err != nil {
		t.Fatalf("find active members: %v", err)
	}
	if len(members) != 1 || members[0].Username != "updated-name" {
		t.Fatalf("expected one active updated member, got %+v", members)
	}

	if err := memberRepo.RemoveMember(ctx, st.ID, "user-1"); err != nil {
		t.Fatalf("remove member: %v", err)
	}
	assertActiveMemberCount(t, ctx, tdb, st.ID, "user-1", 0)

	if err := memberRepo.AddMember(ctx, st.ID, "user-1", "rejoined-name"); err != nil {
		t.Fatalf("re-add member: %v", err)
	}
	assertActiveMemberCount(t, ctx, tdb, st.ID, "user-1", 1)
}

func assertActiveMemberCount(t *testing.T, ctx context.Context, tdb *testDatabase, studyID int64, userID string, want int) {
	t.Helper()
	var got int
	if err := tdb.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM study_members WHERE study_id = $1 AND user_id = $2 AND left_at IS NULL`, studyID, userID).Scan(&got); err != nil {
		t.Fatalf("count active members: %v", err)
	}
	if got != want {
		t.Fatalf("expected active member count %d, got %d", want, got)
	}
}
