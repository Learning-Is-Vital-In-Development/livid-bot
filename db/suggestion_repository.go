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

const SuggestionConfirmVoteThreshold = 3

var (
	ErrActiveSuggestionPeriodExists = errors.New("active suggestion period already exists")
	ErrSuggestionClosed             = errors.New("suggestion is closed")
	ErrSuggestionNotFound           = errors.New("suggestion not found")
)

type SuggestionPeriod struct {
	ID        int64
	ChannelID string
	ClosesAt  time.Time
	CreatedAt time.Time
}

type StudySuggestion struct {
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

type SuggestionRepository struct {
	pool *pgxpool.Pool
}

type ToggleVoteResult struct {
	SuggestionID    int64
	SuggestionTitle string
	ChannelID       string
	MessageID       string
	Voted           bool
	VoteCount       int
	JustConfirmed   bool
}

func NewSuggestionRepository(pool *pgxpool.Pool) *SuggestionRepository {
	return &SuggestionRepository{pool: pool}
}

func (r *SuggestionRepository) CreatePeriod(ctx context.Context, channelID string, closesAt time.Time) (*SuggestionPeriod, error) {
	var p SuggestionPeriod
	err := r.pool.QueryRow(ctx,
		`INSERT INTO suggestion_periods (channel_id, closes_at)
		 VALUES ($1, $2)
		 RETURNING id, channel_id, closes_at, created_at`,
		channelID, closesAt,
	).Scan(&p.ID, &p.ChannelID, &p.ClosesAt, &p.CreatedAt)
	if err != nil {
		if isSQLState(err, "23P01") {
			return nil, ErrActiveSuggestionPeriodExists
		}
		return nil, fmt.Errorf("create period: %w", err)
	}
	return &p, nil
}

func (r *SuggestionRepository) GetActivePeriod(ctx context.Context) (*SuggestionPeriod, error) {
	var p SuggestionPeriod
	err := r.pool.QueryRow(ctx,
		`SELECT id, channel_id, closes_at, created_at
		 FROM suggestion_periods
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

func (r *SuggestionRepository) CreateSuggestion(
	ctx context.Context,
	periodID int64,
	title, description, messageID, channelID string,
) (*StudySuggestion, error) {
	var p StudySuggestion
	err := r.pool.QueryRow(ctx,
		`INSERT INTO study_suggestions (period_id, title, description, message_id, channel_id)
		 SELECT pp.id, $2, $3, $4, $5
		 FROM suggestion_periods pp
		 WHERE pp.id = $1 AND pp.closes_at > NOW()
		 RETURNING id, period_id, title, description, message_id, channel_id, vote_count, confirmed, created_at`,
		periodID, title, description, messageID, channelID,
	).Scan(&p.ID, &p.PeriodID, &p.Title, &p.Description, &p.MessageID, &p.ChannelID, &p.VoteCount, &p.Confirmed, &p.CreatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrSuggestionClosed
		}
		return nil, fmt.Errorf("create suggestion: %w", err)
	}
	return &p, nil
}

func (r *SuggestionRepository) GetSuggestion(ctx context.Context, id int64) (*StudySuggestion, error) {
	var p StudySuggestion
	err := r.pool.QueryRow(ctx,
		`SELECT id, period_id, title, description, message_id, channel_id, vote_count, confirmed, created_at
		 FROM study_suggestions WHERE id = $1`,
		id,
	).Scan(&p.ID, &p.PeriodID, &p.Title, &p.Description, &p.MessageID, &p.ChannelID, &p.VoteCount, &p.Confirmed, &p.CreatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get suggestion: %w", err)
	}
	return &p, nil
}

func (r *SuggestionRepository) ListSuggestions(ctx context.Context, periodID int64) ([]*StudySuggestion, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, period_id, title, description, message_id, channel_id, vote_count, confirmed, created_at
		 FROM study_suggestions WHERE period_id = $1 ORDER BY created_at`,
		periodID,
	)
	if err != nil {
		return nil, fmt.Errorf("list suggestions: %w", err)
	}
	defer rows.Close()

	var suggestions []*StudySuggestion
	for rows.Next() {
		var p StudySuggestion
		if err := rows.Scan(&p.ID, &p.PeriodID, &p.Title, &p.Description, &p.MessageID, &p.ChannelID, &p.VoteCount, &p.Confirmed, &p.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan suggestion: %w", err)
		}
		suggestions = append(suggestions, &p)
	}
	return suggestions, rows.Err()
}

func (r *SuggestionRepository) ToggleVote(ctx context.Context, suggestionID int64, userID string) (*ToggleVoteResult, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	result := &ToggleVoteResult{SuggestionID: suggestionID}
	var wasConfirmed bool
	var isOpen bool

	err = tx.QueryRow(ctx,
		`SELECT sp.title, sp.channel_id, sp.message_id, sp.confirmed, pp.closes_at > NOW()
		 FROM study_suggestions sp
		 JOIN suggestion_periods pp ON pp.id = sp.period_id
		 WHERE sp.id = $1
		 FOR UPDATE`,
		suggestionID,
	).Scan(&result.SuggestionTitle, &result.ChannelID, &result.MessageID, &wasConfirmed, &isOpen)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrSuggestionNotFound
		}
		return nil, fmt.Errorf("load suggestion for vote: %w", err)
	}

	if !isOpen {
		return nil, ErrSuggestionClosed
	}

	// Try to insert a vote; if it already exists, delete it instead
	var existing string
	scanErr := tx.QueryRow(ctx,
		`SELECT user_id FROM study_suggestion_votes WHERE suggestion_id = $1 AND user_id = $2`,
		suggestionID, userID,
	).Scan(&existing)

	if scanErr == pgx.ErrNoRows {
		// Insert vote
		_, err = tx.Exec(ctx,
			`INSERT INTO study_suggestion_votes (suggestion_id, user_id) VALUES ($1, $2)`,
			suggestionID, userID,
		)
		if err != nil {
			return nil, fmt.Errorf("insert vote: %w", err)
		}
		result.Voted = true
	} else if scanErr == nil {
		// Delete vote
		_, err = tx.Exec(ctx,
			`DELETE FROM study_suggestion_votes WHERE suggestion_id = $1 AND user_id = $2`,
			suggestionID, userID,
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
		`UPDATE study_suggestions
		 SET vote_count = counts.vote_count,
		     confirmed = study_suggestions.confirmed OR counts.vote_count >= $2
		 FROM (
		   SELECT COUNT(*)::INT AS vote_count
		   FROM study_suggestion_votes
		 WHERE suggestion_id = $1
		 ) AS counts
		 WHERE id = $1
		 RETURNING study_suggestions.vote_count, study_suggestions.confirmed`,
		suggestionID, SuggestionConfirmVoteThreshold,
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
