package db

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
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
		   'study_suggestions_channel_id_non_empty'
		 )`,
	).Scan(&count)
	if err != nil {
		t.Fatalf("query constraints: %v", err)
	}
	if count != 4 {
		t.Fatalf("expected 4 suggestion constraints, got %d", count)
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

	suggestion, err := repo.CreateSuggestion(ctx, period.ID, "Go", "동시성", "message-1", "suggestion-channel")
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

	_, err = repo.CreateSuggestion(ctx, closedPeriodID, "Closed", "", "message-2", "closed-channel")
	if !errors.Is(err, ErrSuggestionClosed) {
		t.Fatalf("expected ErrSuggestionClosed, got %v", err)
	}
}

func TestToggleVoteUpdatesCountAndConfirmsOnce(t *testing.T) {
	tdb := newTestDatabase(t)
	repo := NewSuggestionRepository(tdb.Pool)
	ctx := context.Background()

	period, err := repo.CreatePeriod(ctx, "suggestion-channel", time.Now().Add(24*time.Hour))
	if err != nil {
		t.Fatalf("create active period: %v", err)
	}

	suggestion, err := repo.CreateSuggestion(ctx, period.ID, "Go", "동시성", "message-1", "suggestion-channel")
	if err != nil {
		t.Fatalf("create suggestion: %v", err)
	}

	first, err := repo.ToggleVote(ctx, suggestion.ID, "user-1")
	if err != nil {
		t.Fatalf("first vote: %v", err)
	}
	if !first.Voted || first.VoteCount != 1 || first.JustConfirmed {
		t.Fatalf("unexpected first vote result: %+v", first)
	}

	second, err := repo.ToggleVote(ctx, suggestion.ID, "user-2")
	if err != nil {
		t.Fatalf("second vote: %v", err)
	}
	if second.VoteCount != 2 || second.JustConfirmed {
		t.Fatalf("unexpected second vote result: %+v", second)
	}

	third, err := repo.ToggleVote(ctx, suggestion.ID, "user-3")
	if err != nil {
		t.Fatalf("third vote: %v", err)
	}
	if third.VoteCount != SuggestionConfirmVoteThreshold || !third.JustConfirmed {
		t.Fatalf("expected confirmation on threshold vote, got %+v", third)
	}

	fourth, err := repo.ToggleVote(ctx, suggestion.ID, "user-4")
	if err != nil {
		t.Fatalf("fourth vote: %v", err)
	}
	if fourth.JustConfirmed {
		t.Fatalf("did not expect duplicate confirmation, got %+v", fourth)
	}

	removed, err := repo.ToggleVote(ctx, suggestion.ID, "user-4")
	if err != nil {
		t.Fatalf("remove fourth vote: %v", err)
	}
	if removed.Voted || removed.VoteCount != SuggestionConfirmVoteThreshold {
		t.Fatalf("unexpected remove result: %+v", removed)
	}

	stored, err := repo.GetSuggestion(ctx, suggestion.ID)
	if err != nil {
		t.Fatalf("load suggestion: %v", err)
	}
	if stored == nil || stored.VoteCount != SuggestionConfirmVoteThreshold || !stored.Confirmed {
		t.Fatalf("expected stored confirmed suggestion with 3 votes, got %+v", stored)
	}
}

func TestToggleVoteRejectsClosedSuggestion(t *testing.T) {
	tdb := newTestDatabase(t)
	repo := NewSuggestionRepository(tdb.Pool)
	ctx := context.Background()

	createdAt := time.Now().Add(-48 * time.Hour)
	closesAt := time.Now().Add(-24 * time.Hour)
	var suggestionID int64
	err := tdb.Pool.QueryRow(ctx,
		`WITH period AS (
		   INSERT INTO suggestion_periods (channel_id, closes_at, created_at)
		   VALUES ($1, $2, $3)
		   RETURNING id
		 )
		 INSERT INTO study_suggestions (period_id, title, description, message_id, channel_id)
		 SELECT id, $4, $5, $6, $1
		 FROM period
		 RETURNING id`,
		"suggestion-channel", closesAt, createdAt, "Closed", "", "message-1",
	).Scan(&suggestionID)
	if err != nil {
		t.Fatalf("insert closed suggestion: %v", err)
	}

	_, err = repo.ToggleVote(ctx, suggestionID, "user-1")
	if !errors.Is(err, ErrSuggestionClosed) {
		t.Fatalf("expected ErrSuggestionClosed, got %v", err)
	}
}

func TestToggleVoteConcurrentConsistency(t *testing.T) {
	tdb := newTestDatabase(t)
	repo := NewSuggestionRepository(tdb.Pool)
	ctx := context.Background()

	period, err := repo.CreatePeriod(ctx, "suggestion-channel", time.Now().Add(24*time.Hour))
	if err != nil {
		t.Fatalf("create active period: %v", err)
	}

	suggestion, err := repo.CreateSuggestion(ctx, period.ID, "Go", "동시성", "message-1", "suggestion-channel")
	if err != nil {
		t.Fatalf("create suggestion: %v", err)
	}

	const voters = 10
	start := make(chan struct{})
	errCh := make(chan error, voters)
	var justConfirmedCount atomic.Int32
	var wg sync.WaitGroup

	for idx := 0; idx < voters; idx++ {
		wg.Add(1)
		go func(userID string) {
			defer wg.Done()
			<-start
			result, err := repo.ToggleVote(context.Background(), suggestion.ID, userID)
			if err != nil {
				errCh <- err
				return
			}
			if result.JustConfirmed {
				justConfirmedCount.Add(1)
			}
		}(time.Now().Add(time.Duration(idx) * time.Millisecond).Format("user-20060102150405.000000000"))
	}

	close(start)
	wg.Wait()
	close(errCh)

	for err := range errCh {
		if err != nil {
			t.Fatalf("concurrent toggle vote failed: %v", err)
		}
	}

	var voteRows int
	if err := tdb.Pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM study_suggestion_votes WHERE suggestion_id = $1`,
		suggestion.ID,
	).Scan(&voteRows); err != nil {
		t.Fatalf("count vote rows: %v", err)
	}

	stored, err := repo.GetSuggestion(ctx, suggestion.ID)
	if err != nil {
		t.Fatalf("load suggestion: %v", err)
	}
	if stored == nil {
		t.Fatal("expected stored suggestion")
	}
	if voteRows != voters || stored.VoteCount != voters {
		t.Fatalf("expected %d votes, got rows=%d suggestion=%d", voters, voteRows, stored.VoteCount)
	}
	if justConfirmedCount.Load() != 1 {
		t.Fatalf("expected exactly one confirmation transition, got %d", justConfirmedCount.Load())
	}
}
