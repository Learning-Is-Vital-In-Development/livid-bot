package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const ProposalConfirmVoteThreshold = 3

var (
	ErrActiveProposalPeriodExists = errors.New("active proposal period already exists")
	ErrProposalClosed             = errors.New("proposal is closed")
	ErrProposalNotFound           = errors.New("proposal not found")
)

type ProposalPeriod struct {
	ID        int64
	ChannelID string
	ClosesAt  time.Time
	CreatedAt time.Time
}

type StudyProposal struct {
	ID          int64
	PeriodID    int64
	Title       string
	Description string
	MessageID   string
	ChannelID   string
	VoteCount   int
	Confirmed   bool
	CreatedAt   time.Time
}

type ProposalRepository struct {
	pool *pgxpool.Pool
}

type ToggleVoteResult struct {
	ProposalID    int64
	ProposalTitle string
	ChannelID     string
	MessageID     string
	Voted         bool
	VoteCount     int
	JustConfirmed bool
}

func NewProposalRepository(pool *pgxpool.Pool) *ProposalRepository {
	return &ProposalRepository{pool: pool}
}

func (r *ProposalRepository) CreatePeriod(ctx context.Context, channelID string, closesAt time.Time) (*ProposalPeriod, error) {
	var p ProposalPeriod
	err := r.pool.QueryRow(ctx,
		`INSERT INTO proposal_periods (channel_id, closes_at)
		 VALUES ($1, $2)
		 RETURNING id, channel_id, closes_at, created_at`,
		channelID, closesAt,
	).Scan(&p.ID, &p.ChannelID, &p.ClosesAt, &p.CreatedAt)
	if err != nil {
		if isSQLState(err, "23P01") {
			return nil, ErrActiveProposalPeriodExists
		}
		return nil, fmt.Errorf("create period: %w", err)
	}
	return &p, nil
}

func (r *ProposalRepository) GetActivePeriod(ctx context.Context) (*ProposalPeriod, error) {
	var p ProposalPeriod
	err := r.pool.QueryRow(ctx,
		`SELECT id, channel_id, closes_at, created_at
		 FROM proposal_periods
		 WHERE closes_at > NOW()
		 ORDER BY created_at DESC
		 LIMIT 1`,
	).Scan(&p.ID, &p.ChannelID, &p.ClosesAt, &p.CreatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get active period: %w", err)
	}
	return &p, nil
}

func (r *ProposalRepository) CreateProposal(
	ctx context.Context,
	periodID int64,
	title, description, messageID, channelID string,
) (*StudyProposal, error) {
	var p StudyProposal
	err := r.pool.QueryRow(ctx,
		`INSERT INTO study_proposals (period_id, title, description, message_id, channel_id)
		 SELECT pp.id, $2, $3, $4, $5
		 FROM proposal_periods pp
		 WHERE pp.id = $1 AND pp.closes_at > NOW()
		 RETURNING id, period_id, title, description, message_id, channel_id, vote_count, confirmed, created_at`,
		periodID, title, description, messageID, channelID,
	).Scan(&p.ID, &p.PeriodID, &p.Title, &p.Description, &p.MessageID, &p.ChannelID, &p.VoteCount, &p.Confirmed, &p.CreatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrProposalClosed
		}
		return nil, fmt.Errorf("create proposal: %w", err)
	}
	return &p, nil
}

func (r *ProposalRepository) GetProposal(ctx context.Context, id int64) (*StudyProposal, error) {
	var p StudyProposal
	err := r.pool.QueryRow(ctx,
		`SELECT id, period_id, title, description, message_id, channel_id, vote_count, confirmed, created_at
		 FROM study_proposals WHERE id = $1`,
		id,
	).Scan(&p.ID, &p.PeriodID, &p.Title, &p.Description, &p.MessageID, &p.ChannelID, &p.VoteCount, &p.Confirmed, &p.CreatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get proposal: %w", err)
	}
	return &p, nil
}

func (r *ProposalRepository) ListProposals(ctx context.Context, periodID int64) ([]*StudyProposal, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, period_id, title, description, message_id, channel_id, vote_count, confirmed, created_at
		 FROM study_proposals WHERE period_id = $1 ORDER BY created_at`,
		periodID,
	)
	if err != nil {
		return nil, fmt.Errorf("list proposals: %w", err)
	}
	defer rows.Close()

	var proposals []*StudyProposal
	for rows.Next() {
		var p StudyProposal
		if err := rows.Scan(&p.ID, &p.PeriodID, &p.Title, &p.Description, &p.MessageID, &p.ChannelID, &p.VoteCount, &p.Confirmed, &p.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan proposal: %w", err)
		}
		proposals = append(proposals, &p)
	}
	return proposals, rows.Err()
}

func (r *ProposalRepository) ToggleVote(ctx context.Context, proposalID int64, userID string) (*ToggleVoteResult, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	result := &ToggleVoteResult{ProposalID: proposalID}
	var wasConfirmed bool
	var isOpen bool

	err = tx.QueryRow(ctx,
		`SELECT sp.title, sp.channel_id, sp.message_id, sp.confirmed, pp.closes_at > NOW()
		 FROM study_proposals sp
		 JOIN proposal_periods pp ON pp.id = sp.period_id
		 WHERE sp.id = $1
		 FOR UPDATE`,
		proposalID,
	).Scan(&result.ProposalTitle, &result.ChannelID, &result.MessageID, &wasConfirmed, &isOpen)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrProposalNotFound
		}
		return nil, fmt.Errorf("load proposal for vote: %w", err)
	}

	if !isOpen {
		return nil, ErrProposalClosed
	}

	// Try to insert a vote; if it already exists, delete it instead
	var existing string
	scanErr := tx.QueryRow(ctx,
		`SELECT user_id FROM study_proposal_votes WHERE proposal_id = $1 AND user_id = $2`,
		proposalID, userID,
	).Scan(&existing)

	if scanErr == pgx.ErrNoRows {
		// Insert vote
		_, err = tx.Exec(ctx,
			`INSERT INTO study_proposal_votes (proposal_id, user_id) VALUES ($1, $2)`,
			proposalID, userID,
		)
		if err != nil {
			return nil, fmt.Errorf("insert vote: %w", err)
		}
		result.Voted = true
	} else if scanErr == nil {
		// Delete vote
		_, err = tx.Exec(ctx,
			`DELETE FROM study_proposal_votes WHERE proposal_id = $1 AND user_id = $2`,
			proposalID, userID,
		)
		if err != nil {
			return nil, fmt.Errorf("delete vote: %w", err)
		}
		result.Voted = false
	} else {
		return nil, fmt.Errorf("check existing vote: %w", scanErr)
	}

	// Update vote_count
	var isConfirmed bool
	err = tx.QueryRow(ctx,
		`UPDATE study_proposals
		 SET vote_count = counts.vote_count,
		     confirmed = study_proposals.confirmed OR counts.vote_count >= $2
		 FROM (
		   SELECT COUNT(*)::INT AS vote_count
		   FROM study_proposal_votes
		   WHERE proposal_id = $1
		 ) AS counts
		 WHERE id = $1
		 RETURNING study_proposals.vote_count, study_proposals.confirmed`,
		proposalID, ProposalConfirmVoteThreshold,
	).Scan(&result.VoteCount, &isConfirmed)
	if err != nil {
		return nil, fmt.Errorf("update vote count: %w", err)
	}
	result.JustConfirmed = !wasConfirmed && isConfirmed

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit toggle vote: %w", err)
	}

	return result, nil
}

func isSQLState(err error, state string) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == state
	}
	return false
}
