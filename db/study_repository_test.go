package db

import (
	"context"
	"testing"
)

func TestNextAvailableNameAddsSuffixWithinBranch(t *testing.T) {
	tdb := newTestDatabase(t)
	repo := NewStudyRepository(tdb.Pool)
	ctx := context.Background()

	if _, err := repo.Create(ctx, "", "Go", "", "channel-1", "role-1"); err != nil {
		t.Fatalf("create first study: %v", err)
	}
	if _, err := repo.Create(ctx, "", "Go-2", "", "channel-2", "role-2"); err != nil {
		t.Fatalf("create second study: %v", err)
	}
	if _, err := repo.Create(ctx, "26-2", "Go", "", "channel-3", "role-3"); err != nil {
		t.Fatalf("create branch study: %v", err)
	}

	got, err := repo.NextAvailableName(ctx, "", "Go")
	if err != nil {
		t.Fatalf("next available name: %v", err)
	}
	if got != "Go-3" {
		t.Fatalf("expected Go-3, got %q", got)
	}

	branchGot, err := repo.NextAvailableName(ctx, "26-2", "Go")
	if err != nil {
		t.Fatalf("next available branch name: %v", err)
	}
	if branchGot != "Go-2" {
		t.Fatalf("expected branch-local suffix Go-2, got %q", branchGot)
	}
}
