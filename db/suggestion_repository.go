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
	ID                  int64
	PeriodID            int64
	Title               string
	Description         string
	MessageID           string
	ChannelID           string
	VoteCount           int
	Confirmed           bool
	Visibility          string
	ProposerUserID      string
	ProposerDisplayName string
	Threshold           int
	ExpiresAt           time.Time
	Status              string
	CreatedAt           time.Time
}

type CreateSuggestionParams struct {
	PeriodID            int64
	Title               string
	Description         string
	MessageID           string
	ChannelID           string
	Visibility          string
	ProposerUserID      string
	ProposerDisplayName string
	Threshold           int
	ExpiresAt           time.Time
}

type SuggestionRepository struct {
	pool *pgxpool.Pool
}

type SyncVotesResult struct {
	SuggestionID          int64
	SuggestionTitle       string
	SuggestionDescription string
	ChannelID             string
	MessageID             string
	Threshold             int
	VoteCount             int
	JustConfirmed         bool
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

func (r *SuggestionRepository) CreateSuggestion(ctx context.Context, params CreateSuggestionParams) (*StudySuggestion, error) {
	if params.Visibility == "" {
		params.Visibility = "anonymous"
	}
	if params.Threshold < 1 {
		params.Threshold = SuggestionConfirmVoteThreshold
	}
	if params.ExpiresAt.IsZero() {
		params.ExpiresAt = time.Now().Add(14 * 24 * time.Hour)
	}

	var p StudySuggestion
	err := r.pool.QueryRow(ctx,
		`INSERT INTO study_suggestions (
		   period_id, title, description, message_id, channel_id,
		   visibility, proposer_user_id, proposer_display_name, threshold, expires_at
		 )
		 SELECT NULLIF($1, 0), $2, $3, $4, $5, $6, NULLIF($7, ''), NULLIF($8, ''), $9, $10
		 WHERE $1 = 0 OR EXISTS (
		   SELECT 1 FROM suggestion_periods WHERE id = $1 AND closes_at > NOW()
		 )
		 RETURNING id, COALESCE(period_id, 0), title, description, message_id, channel_id,
		   vote_count, confirmed, visibility, COALESCE(proposer_user_id, ''),
		   COALESCE(proposer_display_name, ''), threshold, expires_at, status, created_at`,
		params.PeriodID, params.Title, params.Description, params.MessageID, params.ChannelID,
		params.Visibility, params.ProposerUserID, params.ProposerDisplayName, params.Threshold, params.ExpiresAt,
	).Scan(
		&p.ID, &p.PeriodID, &p.Title, &p.Description, &p.MessageID, &p.ChannelID,
		&p.VoteCount, &p.Confirmed, &p.Visibility, &p.ProposerUserID,
		&p.ProposerDisplayName, &p.Threshold, &p.ExpiresAt, &p.Status, &p.CreatedAt,
	)
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
		`SELECT id, COALESCE(period_id, 0), title, description, message_id, channel_id,
		   vote_count, confirmed, visibility, COALESCE(proposer_user_id, ''),
		   COALESCE(proposer_display_name, ''), threshold, expires_at, status, created_at
		 FROM study_suggestions WHERE id = $1`,
		id,
	).Scan(
		&p.ID, &p.PeriodID, &p.Title, &p.Description, &p.MessageID, &p.ChannelID,
		&p.VoteCount, &p.Confirmed, &p.Visibility, &p.ProposerUserID,
		&p.ProposerDisplayName, &p.Threshold, &p.ExpiresAt, &p.Status, &p.CreatedAt,
	)
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
		`SELECT id, COALESCE(period_id, 0), title, description, message_id, channel_id,
		   vote_count, confirmed, visibility, COALESCE(proposer_user_id, ''),
		   COALESCE(proposer_display_name, ''), threshold, expires_at, status, created_at
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
		if err := rows.Scan(
			&p.ID, &p.PeriodID, &p.Title, &p.Description, &p.MessageID, &p.ChannelID,
			&p.VoteCount, &p.Confirmed, &p.Visibility, &p.ProposerUserID,
			&p.ProposerDisplayName, &p.Threshold, &p.ExpiresAt, &p.Status, &p.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan suggestion: %w", err)
		}
		suggestions = append(suggestions, &p)
	}
	return suggestions, rows.Err()
}

func (r *SuggestionRepository) ListOpenSuggestionsForNudge(ctx context.Context) ([]*StudySuggestion, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, COALESCE(period_id, 0), title, description, message_id, channel_id,
		   vote_count, confirmed, visibility, COALESCE(proposer_user_id, ''),
		   COALESCE(proposer_display_name, ''), threshold, expires_at, status, created_at
		 FROM study_suggestions
		 WHERE status = 'open' AND confirmed = FALSE AND expires_at > NOW()
		 ORDER BY expires_at, created_at`,
	)
	if err != nil {
		return nil, fmt.Errorf("list open suggestions for nudge: %w", err)
	}
	defer rows.Close()

	var suggestions []*StudySuggestion
	for rows.Next() {
		var p StudySuggestion
		if err := rows.Scan(
			&p.ID, &p.PeriodID, &p.Title, &p.Description, &p.MessageID, &p.ChannelID,
			&p.VoteCount, &p.Confirmed, &p.Visibility, &p.ProposerUserID,
			&p.ProposerDisplayName, &p.Threshold, &p.ExpiresAt, &p.Status, &p.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan open suggestion: %w", err)
		}
		suggestions = append(suggestions, &p)
	}
	return suggestions, rows.Err()
}

func (r *SuggestionRepository) GetOpenSuggestionByMessageRef(ctx context.Context, channelID, messageID string) (*StudySuggestion, error) {
	var p StudySuggestion
	err := r.pool.QueryRow(ctx,
		`SELECT id, COALESCE(period_id, 0), title, description, message_id, channel_id,
		   vote_count, confirmed, visibility, COALESCE(proposer_user_id, ''),
		   COALESCE(proposer_display_name, ''), threshold, expires_at, status, created_at
		 FROM study_suggestions
		 WHERE channel_id = $1 AND message_id = $2 AND status = 'open' AND expires_at > NOW()`,
		channelID, messageID,
	).Scan(
		&p.ID, &p.PeriodID, &p.Title, &p.Description, &p.MessageID, &p.ChannelID,
		&p.VoteCount, &p.Confirmed, &p.Visibility, &p.ProposerUserID,
		&p.ProposerDisplayName, &p.Threshold, &p.ExpiresAt, &p.Status, &p.CreatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get open suggestion by message ref: %w", err)
	}
	return &p, nil
}

func (r *SuggestionRepository) SyncVotes(ctx context.Context, suggestionID int64, userIDs []string) (*SyncVotesResult, error) {
	userIDs = uniqueNonEmpty(userIDs)
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	result := &SyncVotesResult{SuggestionID: suggestionID}
	var wasConfirmed bool
	var isOpen bool
	err = tx.QueryRow(ctx,
		`SELECT title, description, channel_id, message_id, confirmed,
		        status = 'open' AND expires_at > NOW(), threshold
		 FROM study_suggestions
		 WHERE id = $1
		 FOR UPDATE`,
		suggestionID,
	).Scan(&result.SuggestionTitle, &result.SuggestionDescription, &result.ChannelID, &result.MessageID, &wasConfirmed, &isOpen, &result.Threshold)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrSuggestionNotFound
		}
		return nil, fmt.Errorf("load suggestion for vote sync: %w", err)
	}
	if !isOpen {
		return nil, ErrSuggestionClosed
	}

	if len(userIDs) == 0 {
		if _, err = tx.Exec(ctx, `DELETE FROM study_suggestion_votes WHERE suggestion_id = $1`, suggestionID); err != nil {
			return nil, fmt.Errorf("clear votes: %w", err)
		}
	} else if _, err = tx.Exec(ctx,
		`DELETE FROM study_suggestion_votes WHERE suggestion_id = $1 AND NOT (user_id = ANY($2))`,
		suggestionID, userIDs,
	); err != nil {
		return nil, fmt.Errorf("prune votes: %w", err)
	}

	for _, userID := range userIDs {
		if _, err = tx.Exec(ctx,
			`INSERT INTO study_suggestion_votes (suggestion_id, user_id)
			 VALUES ($1, $2)
			 ON CONFLICT (suggestion_id, user_id) DO NOTHING`,
			suggestionID, userID,
		); err != nil {
			return nil, fmt.Errorf("insert synced vote: %w", err)
		}
	}

	var isConfirmed bool
	err = tx.QueryRow(ctx,
		`UPDATE study_suggestions
		 SET vote_count = counts.vote_count,
		     confirmed = counts.vote_count >= study_suggestions.threshold
		 FROM (
		   SELECT COUNT(*)::INT AS vote_count
		   FROM study_suggestion_votes
		   WHERE suggestion_id = $1
		 ) AS counts
		 WHERE id = $1
		 RETURNING study_suggestions.vote_count, study_suggestions.confirmed`,
		suggestionID,
	).Scan(&result.VoteCount, &isConfirmed)
	if err != nil {
		return nil, fmt.Errorf("update synced vote count: %w", err)
	}
	result.JustConfirmed = !wasConfirmed && isConfirmed
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit vote sync: %w", err)
	}
	return result, nil
}

func (r *SuggestionRepository) MarkOpened(ctx context.Context, suggestionID, studyID int64) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE study_suggestions
		 SET status = 'opened', opened_study_id = $2, opened_at = NOW(), opening_error = NULL
		 WHERE id = $1`,
		suggestionID, studyID,
	)
	if err != nil {
		return fmt.Errorf("mark suggestion opened: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrSuggestionNotFound
	}
	return nil
}

func (r *SuggestionRepository) MarkOpeningFailed(ctx context.Context, suggestionID int64, reason string) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE study_suggestions
		 SET status = 'opening_failed', opening_error = $2
		 WHERE id = $1 AND status = 'open'`,
		suggestionID, reason,
	)
	if err != nil {
		return fmt.Errorf("mark suggestion opening failed: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrSuggestionNotFound
	}
	return nil
}

func uniqueNonEmpty(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func isSQLState(err error, state string) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == state
	}
	return false
}
