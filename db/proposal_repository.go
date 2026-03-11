package db

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
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

func (r *ProposalRepository) CreateProposal(ctx context.Context, periodID int64, title, description string) (*StudyProposal, error) {
	var p StudyProposal
	err := r.pool.QueryRow(ctx,
		`INSERT INTO study_proposals (period_id, title, description)
		 VALUES ($1, $2, $3)
		 RETURNING id, period_id, title, description, message_id, channel_id, vote_count, confirmed, created_at`,
		periodID, title, description,
	).Scan(&p.ID, &p.PeriodID, &p.Title, &p.Description, &p.MessageID, &p.ChannelID, &p.VoteCount, &p.Confirmed, &p.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("create proposal: %w", err)
	}
	return &p, nil
}

func (r *ProposalRepository) UpdateProposalMessage(ctx context.Context, proposalID int64, messageID, channelID string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE study_proposals SET message_id = $1, channel_id = $2 WHERE id = $3`,
		messageID, channelID, proposalID,
	)
	if err != nil {
		return fmt.Errorf("update proposal message: %w", err)
	}
	return nil
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

func (r *ProposalRepository) ToggleVote(ctx context.Context, proposalID int64, userID string) (voted bool, newCount int, err error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return false, 0, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

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
			return false, 0, fmt.Errorf("insert vote: %w", err)
		}
		voted = true
	} else if scanErr == nil {
		// Delete vote
		_, err = tx.Exec(ctx,
			`DELETE FROM study_proposal_votes WHERE proposal_id = $1 AND user_id = $2`,
			proposalID, userID,
		)
		if err != nil {
			return false, 0, fmt.Errorf("delete vote: %w", err)
		}
		voted = false
	} else {
		return false, 0, fmt.Errorf("check existing vote: %w", scanErr)
	}

	// Update vote_count
	err = tx.QueryRow(ctx,
		`UPDATE study_proposals
		 SET vote_count = (SELECT COUNT(*) FROM study_proposal_votes WHERE proposal_id = $1)
		 WHERE id = $1
		 RETURNING vote_count`,
		proposalID,
	).Scan(&newCount)
	if err != nil {
		return false, 0, fmt.Errorf("update vote count: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return false, 0, fmt.Errorf("commit toggle vote: %w", err)
	}
	return voted, newCount, nil
}

func (r *ProposalRepository) MarkConfirmed(ctx context.Context, proposalID int64) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE study_proposals SET confirmed = TRUE WHERE id = $1`,
		proposalID,
	)
	if err != nil {
		return fmt.Errorf("mark confirmed: %w", err)
	}
	return nil
}
