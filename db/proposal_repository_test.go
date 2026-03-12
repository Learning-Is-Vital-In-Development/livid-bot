package db

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestMigrateAddsProposalConstraints(t *testing.T) {
	tdb := newTestDatabase(t)
	ctx := context.Background()

	var count int
	err := tdb.Pool.QueryRow(ctx,
		`SELECT COUNT(*)
		 FROM pg_constraint
		 WHERE conname IN (
		   'proposal_periods_closes_after_create',
		   'proposal_periods_no_overlap',
		   'study_proposals_message_id_non_empty',
		   'study_proposals_channel_id_non_empty'
		 )`,
	).Scan(&count)
	if err != nil {
		t.Fatalf("query constraints: %v", err)
	}
	if count != 4 {
		t.Fatalf("expected 4 proposal constraints, got %d", count)
	}
}

func TestCreatePeriodRejectsOverlappingActivePeriod(t *testing.T) {
	tdb := newTestDatabase(t)
	repo := NewProposalRepository(tdb.Pool)
	ctx := context.Background()

	firstClose := time.Now().Add(48 * time.Hour)
	if _, err := repo.CreatePeriod(ctx, "channel-1", firstClose); err != nil {
		t.Fatalf("create first period: %v", err)
	}

	_, err := repo.CreatePeriod(ctx, "channel-2", time.Now().Add(72*time.Hour))
	if !errors.Is(err, ErrActiveProposalPeriodExists) {
		t.Fatalf("expected ErrActiveProposalPeriodExists, got %v", err)
	}
}

func TestCreateProposalRequiresOpenPeriodAndStoresMessageRefs(t *testing.T) {
	tdb := newTestDatabase(t)
	repo := NewProposalRepository(tdb.Pool)
	ctx := context.Background()

	period, err := repo.CreatePeriod(ctx, "proposal-channel", time.Now().Add(24*time.Hour))
	if err != nil {
		t.Fatalf("create active period: %v", err)
	}

	proposal, err := repo.CreateProposal(ctx, period.ID, "Go", "동시성", "message-1", "proposal-channel")
	if err != nil {
		t.Fatalf("create proposal: %v", err)
	}
	if proposal.MessageID != "message-1" || proposal.ChannelID != "proposal-channel" {
		t.Fatalf("expected message refs to be stored, got %+v", proposal)
	}

	createdAt := time.Now().Add(-48 * time.Hour)
	closesAt := time.Now().Add(-24 * time.Hour)
	var closedPeriodID int64
	err = tdb.Pool.QueryRow(ctx,
		`INSERT INTO proposal_periods (channel_id, closes_at, created_at)
		 VALUES ($1, $2, $3)
		 RETURNING id`,
		"closed-channel", closesAt, createdAt,
	).Scan(&closedPeriodID)
	if err != nil {
		t.Fatalf("insert closed period: %v", err)
	}

	_, err = repo.CreateProposal(ctx, closedPeriodID, "Closed", "", "message-2", "closed-channel")
	if !errors.Is(err, ErrProposalClosed) {
		t.Fatalf("expected ErrProposalClosed, got %v", err)
	}
}

func TestToggleVoteUpdatesCountAndConfirmsOnce(t *testing.T) {
	tdb := newTestDatabase(t)
	repo := NewProposalRepository(tdb.Pool)
	ctx := context.Background()

	period, err := repo.CreatePeriod(ctx, "proposal-channel", time.Now().Add(24*time.Hour))
	if err != nil {
		t.Fatalf("create active period: %v", err)
	}

	proposal, err := repo.CreateProposal(ctx, period.ID, "Go", "동시성", "message-1", "proposal-channel")
	if err != nil {
		t.Fatalf("create proposal: %v", err)
	}

	first, err := repo.ToggleVote(ctx, proposal.ID, "user-1")
	if err != nil {
		t.Fatalf("first vote: %v", err)
	}
	if !first.Voted || first.VoteCount != 1 || first.JustConfirmed {
		t.Fatalf("unexpected first vote result: %+v", first)
	}

	second, err := repo.ToggleVote(ctx, proposal.ID, "user-2")
	if err != nil {
		t.Fatalf("second vote: %v", err)
	}
	if second.VoteCount != 2 || second.JustConfirmed {
		t.Fatalf("unexpected second vote result: %+v", second)
	}

	third, err := repo.ToggleVote(ctx, proposal.ID, "user-3")
	if err != nil {
		t.Fatalf("third vote: %v", err)
	}
	if third.VoteCount != ProposalConfirmVoteThreshold || !third.JustConfirmed {
		t.Fatalf("expected confirmation on threshold vote, got %+v", third)
	}

	fourth, err := repo.ToggleVote(ctx, proposal.ID, "user-4")
	if err != nil {
		t.Fatalf("fourth vote: %v", err)
	}
	if fourth.JustConfirmed {
		t.Fatalf("did not expect duplicate confirmation, got %+v", fourth)
	}

	removed, err := repo.ToggleVote(ctx, proposal.ID, "user-4")
	if err != nil {
		t.Fatalf("remove fourth vote: %v", err)
	}
	if removed.Voted || removed.VoteCount != ProposalConfirmVoteThreshold {
		t.Fatalf("unexpected remove result: %+v", removed)
	}

	stored, err := repo.GetProposal(ctx, proposal.ID)
	if err != nil {
		t.Fatalf("load proposal: %v", err)
	}
	if stored == nil || stored.VoteCount != ProposalConfirmVoteThreshold || !stored.Confirmed {
		t.Fatalf("expected stored confirmed proposal with 3 votes, got %+v", stored)
	}
}

func TestToggleVoteRejectsClosedProposal(t *testing.T) {
	tdb := newTestDatabase(t)
	repo := NewProposalRepository(tdb.Pool)
	ctx := context.Background()

	createdAt := time.Now().Add(-48 * time.Hour)
	closesAt := time.Now().Add(-24 * time.Hour)
	var proposalID int64
	err := tdb.Pool.QueryRow(ctx,
		`WITH period AS (
		   INSERT INTO proposal_periods (channel_id, closes_at, created_at)
		   VALUES ($1, $2, $3)
		   RETURNING id
		 )
		 INSERT INTO study_proposals (period_id, title, description, message_id, channel_id)
		 SELECT id, $4, $5, $6, $1
		 FROM period
		 RETURNING id`,
		"proposal-channel", closesAt, createdAt, "Closed", "", "message-1",
	).Scan(&proposalID)
	if err != nil {
		t.Fatalf("insert closed proposal: %v", err)
	}

	_, err = repo.ToggleVote(ctx, proposalID, "user-1")
	if !errors.Is(err, ErrProposalClosed) {
		t.Fatalf("expected ErrProposalClosed, got %v", err)
	}
}

func TestToggleVoteConcurrentConsistency(t *testing.T) {
	tdb := newTestDatabase(t)
	repo := NewProposalRepository(tdb.Pool)
	ctx := context.Background()

	period, err := repo.CreatePeriod(ctx, "proposal-channel", time.Now().Add(24*time.Hour))
	if err != nil {
		t.Fatalf("create active period: %v", err)
	}

	proposal, err := repo.CreateProposal(ctx, period.ID, "Go", "동시성", "message-1", "proposal-channel")
	if err != nil {
		t.Fatalf("create proposal: %v", err)
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
			result, err := repo.ToggleVote(context.Background(), proposal.ID, userID)
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
		`SELECT COUNT(*) FROM study_proposal_votes WHERE proposal_id = $1`,
		proposal.ID,
	).Scan(&voteRows); err != nil {
		t.Fatalf("count vote rows: %v", err)
	}

	stored, err := repo.GetProposal(ctx, proposal.ID)
	if err != nil {
		t.Fatalf("load proposal: %v", err)
	}
	if stored == nil {
		t.Fatal("expected stored proposal")
	}
	if voteRows != voters || stored.VoteCount != voters {
		t.Fatalf("expected %d votes, got rows=%d proposal=%d", voters, voteRows, stored.VoteCount)
	}
	if justConfirmedCount.Load() != 1 {
		t.Fatalf("expected exactly one confirmation transition, got %d", justConfirmedCount.Load())
	}
}
