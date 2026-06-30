package db

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestMigrateAddsSuggestionConstraints(t *testing.T) {
	tdb := newTestDatabase(t)
	ctx := context.Background()

	var count int
	err := tdb.Pool.QueryRow(ctx,
		`SELECT COUNT(*)
		 FROM pg_constraint
		 WHERE conname IN (
		   'suggestion_periods_closes_after_create',
		   'suggestion_periods_no_overlap',
		   'study_suggestions_message_id_non_empty',
		   'study_suggestions_channel_id_non_empty',
		   'study_suggestions_visibility_check',
		   'study_suggestions_status_check',
		   'study_suggestions_threshold_check'
		 )`,
	).Scan(&count)
	if err != nil {
		t.Fatalf("query constraints: %v", err)
	}
	if count != 7 {
		t.Fatalf("expected 7 suggestion constraints, got %d", count)
	}

	var suggestionTables int
	err = tdb.Pool.QueryRow(ctx,
		`SELECT COUNT(*)
		 FROM information_schema.tables
		 WHERE table_schema = 'public'
		   AND table_name IN ('suggestion_periods', 'study_suggestions', 'study_suggestion_votes')`,
	).Scan(&suggestionTables)
	if err != nil {
		t.Fatalf("query suggestion tables: %v", err)
	}
	if suggestionTables != 3 {
		t.Fatalf("expected 3 suggestion tables, got %d", suggestionTables)
	}

	var proposalTables int
	err = tdb.Pool.QueryRow(ctx,
		`SELECT COUNT(*)
		 FROM information_schema.tables
		 WHERE table_schema = 'public'
		   AND table_name IN ('proposal_periods', 'study_proposals', 'study_proposal_votes')`,
	).Scan(&proposalTables)
	if err != nil {
		t.Fatalf("query proposal tables: %v", err)
	}
	if proposalTables != 0 {
		t.Fatalf("expected proposal tables to be renamed away, got %d remaining", proposalTables)
	}

	var suggestionVoteColumnCount int
	err = tdb.Pool.QueryRow(ctx,
		`SELECT COUNT(*)
		 FROM information_schema.columns
		 WHERE table_schema = 'public'
		   AND table_name = 'study_suggestion_votes'
		   AND column_name = 'suggestion_id'`,
	).Scan(&suggestionVoteColumnCount)
	if err != nil {
		t.Fatalf("query suggestion vote column: %v", err)
	}
	if suggestionVoteColumnCount != 1 {
		t.Fatalf("expected study_suggestion_votes.suggestion_id column, got %d matches", suggestionVoteColumnCount)
	}
}

func TestCreatePeriodRejectsOverlappingActivePeriod(t *testing.T) {
	tdb := newTestDatabase(t)
	repo := NewSuggestionRepository(tdb.Pool)
	ctx := context.Background()

	firstClose := time.Now().Add(48 * time.Hour)
	if _, err := repo.CreatePeriod(ctx, "channel-1", firstClose); err != nil {
		t.Fatalf("create first period: %v", err)
	}

	_, err := repo.CreatePeriod(ctx, "channel-2", time.Now().Add(72*time.Hour))
	if !errors.Is(err, ErrActiveSuggestionPeriodExists) {
		t.Fatalf("expected ErrActiveSuggestionPeriodExists, got %v", err)
	}
}

func TestCreateSuggestionRequiresOpenPeriodAndStoresMessageRefs(t *testing.T) {
	tdb := newTestDatabase(t)
	repo := NewSuggestionRepository(tdb.Pool)
	ctx := context.Background()

	period, err := repo.CreatePeriod(ctx, "suggestion-channel", time.Now().Add(24*time.Hour))
	if err != nil {
		t.Fatalf("create active period: %v", err)
	}

	suggestion, err := repo.CreateSuggestion(ctx, CreateSuggestionParams{PeriodID: period.ID, Title: "Go", Description: "동시성", MessageID: "message-1", ChannelID: "suggestion-channel"})
	if err != nil {
		t.Fatalf("create suggestion: %v", err)
	}
	if suggestion.MessageID != "message-1" || suggestion.ChannelID != "suggestion-channel" {
		t.Fatalf("expected message refs to be stored, got %+v", suggestion)
	}

	createdAt := time.Now().Add(-48 * time.Hour)
	closesAt := time.Now().Add(-24 * time.Hour)
	var closedPeriodID int64
	err = tdb.Pool.QueryRow(ctx,
		`INSERT INTO suggestion_periods (channel_id, closes_at, created_at)
		 VALUES ($1, $2, $3)
		 RETURNING id`,
		"closed-channel", closesAt, createdAt,
	).Scan(&closedPeriodID)
	if err != nil {
		t.Fatalf("insert closed period: %v", err)
	}

	_, err = repo.CreateSuggestion(ctx, CreateSuggestionParams{PeriodID: closedPeriodID, Title: "Closed", Description: "", MessageID: "message-2", ChannelID: "closed-channel"})
	if !errors.Is(err, ErrSuggestionClosed) {
		t.Fatalf("expected ErrSuggestionClosed, got %v", err)
	}
}

func TestSyncVotesMirrorsActualReactionUsers(t *testing.T) {
	tdb := newTestDatabase(t)
	repo := NewSuggestionRepository(tdb.Pool)
	ctx := context.Background()

	suggestion, err := repo.CreateSuggestion(ctx, CreateSuggestionParams{Title: "Go", Description: "동시성", MessageID: "message-sync", ChannelID: "thread-sync", Threshold: 3, ExpiresAt: time.Now().Add(24 * time.Hour)})
	if err != nil {
		t.Fatalf("create suggestion: %v", err)
	}

	first, err := repo.SyncVotes(ctx, suggestion.ID, []string{"user-1", "user-2", "user-3", "user-3", ""})
	if err != nil {
		t.Fatalf("sync votes: %v", err)
	}
	if first.VoteCount != 3 || !first.JustConfirmed {
		t.Fatalf("expected first sync to confirm at 3 votes, got %+v", first)
	}

	second, err := repo.SyncVotes(ctx, suggestion.ID, []string{"user-1", "user-2"})
	if err != nil {
		t.Fatalf("sync reduced votes: %v", err)
	}
	if second.VoteCount != 2 || second.JustConfirmed {
		t.Fatalf("expected reduced sync without confirmation, got %+v", second)
	}

	stored, err := repo.GetSuggestion(ctx, suggestion.ID)
	if err != nil {
		t.Fatalf("load suggestion: %v", err)
	}
	if stored == nil || stored.VoteCount != 2 || stored.Confirmed {
		t.Fatalf("expected actual reaction state to be mirrored, got %+v", stored)
	}
}

func TestMarkSuggestionOpenedAndOpeningFailed(t *testing.T) {
	tdb := newTestDatabase(t)
	repo := NewSuggestionRepository(tdb.Pool)
	ctx := context.Background()

	suggestion, err := repo.CreateSuggestion(ctx, CreateSuggestionParams{Title: "Go", Description: "", MessageID: "message-opened", ChannelID: "thread-opened", ExpiresAt: time.Now().Add(24 * time.Hour)})
	if err != nil {
		t.Fatalf("create suggestion: %v", err)
	}
	var studyID int64
	if err := tdb.Pool.QueryRow(ctx,
		`INSERT INTO studies (branch, name, description, channel_id, role_id)
		 VALUES ('', 'Go', '', 'study-channel', 'role-id')
		 RETURNING id`,
	).Scan(&studyID); err != nil {
		t.Fatalf("insert study: %v", err)
	}

	if err := repo.MarkOpened(ctx, suggestion.ID, studyID); err != nil {
		t.Fatalf("mark opened: %v", err)
	}
	stored, err := repo.GetSuggestion(ctx, suggestion.ID)
	if err != nil {
		t.Fatalf("load suggestion: %v", err)
	}
	if stored == nil || stored.Status != "opened" {
		t.Fatalf("expected opened suggestion, got %+v", stored)
	}

	failed, err := repo.CreateSuggestion(ctx, CreateSuggestionParams{Title: "Rust", Description: "", MessageID: "message-failed", ChannelID: "thread-failed", ExpiresAt: time.Now().Add(24 * time.Hour)})
	if err != nil {
		t.Fatalf("create failed suggestion: %v", err)
	}
	if err := repo.MarkOpeningFailed(ctx, failed.ID, "boom"); err != nil {
		t.Fatalf("mark opening failed: %v", err)
	}
	stored, err = repo.GetSuggestion(ctx, failed.ID)
	if err != nil {
		t.Fatalf("load failed suggestion: %v", err)
	}
	if stored == nil || stored.Status != "opening_failed" {
		t.Fatalf("expected opening_failed suggestion, got %+v", stored)
	}
}
