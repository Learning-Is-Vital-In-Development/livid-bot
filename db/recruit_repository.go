package db

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"livid-bot/study"
)

type RecruitRepository struct {
	pool *pgxpool.Pool
}

func NewRecruitRepository(pool *pgxpool.Pool) *RecruitRepository {
	return &RecruitRepository{pool: pool}
}

type SaveRecruitMessageParams struct {
	MessageID string
	ChannelID string
	Branch    string
	OpensAt   time.Time
	ClosesAt  time.Time
	Mappings  []study.RecruitMapping
}

func (r *RecruitRepository) SaveRecruitMessage(ctx context.Context, params SaveRecruitMessageParams) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	var recruitID int64
	err = tx.QueryRow(ctx,
		`INSERT INTO recruit_messages (message_id, channel_id, branch, opens_at, closes_at)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id`,
		params.MessageID, params.ChannelID, params.Branch, params.OpensAt, params.ClosesAt,
	).Scan(&recruitID)
	if err != nil {
		return fmt.Errorf("insert recruit message: %w", err)
	}

	for _, m := range params.Mappings {
		_, err := tx.Exec(ctx,
			`INSERT INTO recruit_message_mappings (recruit_message_id, emoji, study_id, role_id)
			 VALUES ($1, $2, $3, $4)`,
			recruitID, m.Emoji, m.StudyID, m.RoleID,
		)
		if err != nil {
			return fmt.Errorf("insert mapping for emoji %s: %w", m.Emoji, err)
		}
	}

	return tx.Commit(ctx)
}

type EmojiRoleMapping struct {
	MessageID string
	Emoji     string
	StudyID   int64
	RoleID    string
}

func (r *RecruitRepository) LoadAllMappings(ctx context.Context) ([]EmojiRoleMapping, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT rm.message_id, rmm.emoji, rmm.study_id, rmm.role_id
		 FROM recruit_message_mappings rmm
		 JOIN recruit_messages rm ON rm.id = rmm.recruit_message_id
		 JOIN studies s ON s.id = rmm.study_id
		 WHERE s.status = 'active' AND rm.closed_at IS NULL`)
	if err != nil {
		return nil, fmt.Errorf("load mappings: %w", err)
	}
	defer rows.Close()

	var mappings []EmojiRoleMapping
	for rows.Next() {
		var m EmojiRoleMapping
		if err := rows.Scan(&m.MessageID, &m.Emoji, &m.StudyID, &m.RoleID); err != nil {
			return nil, fmt.Errorf("scan mapping: %w", err)
		}
		mappings = append(mappings, m)
	}
	return mappings, rows.Err()
}

type OpenRecruitMapping struct {
	RecruitMessageID string
	RecruitChannelID string
	Branch           string
	OpensAt          time.Time
	ClosesAt         time.Time
	Emoji            string
	StudyID          int64
	StudyName        string
	StudyChannelID   string
	RoleID           string
}

func (r *RecruitRepository) FindOpenRecruitMappingsByBranch(ctx context.Context, branch string) ([]OpenRecruitMapping, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT rm.message_id,
		        rm.channel_id,
		        COALESCE(rm.branch, s.branch),
		        COALESCE(rm.opens_at, rm.created_at),
		        COALESCE(rm.closes_at, rm.created_at),
		        rmm.emoji,
		        s.id,
		        s.name,
		        s.channel_id,
		        rmm.role_id
		 FROM recruit_messages rm
		 JOIN recruit_message_mappings rmm ON rm.id = rmm.recruit_message_id
		 JOIN studies s ON s.id = rmm.study_id
		 WHERE s.branch = $1
		   AND s.status = 'active'
		   AND rm.closed_at IS NULL
		   AND (rm.branch IS NULL OR rm.branch = $1)
		 ORDER BY s.id, rm.created_at, rmm.emoji`, branch)
	if err != nil {
		return nil, fmt.Errorf("find open recruit mappings by branch: %w", err)
	}
	defer rows.Close()

	var mappings []OpenRecruitMapping
	for rows.Next() {
		var m OpenRecruitMapping
		if err := rows.Scan(
			&m.RecruitMessageID,
			&m.RecruitChannelID,
			&m.Branch,
			&m.OpensAt,
			&m.ClosesAt,
			&m.Emoji,
			&m.StudyID,
			&m.StudyName,
			&m.StudyChannelID,
			&m.RoleID,
		); err != nil {
			return nil, fmt.Errorf("scan open recruit mapping: %w", err)
		}
		mappings = append(mappings, m)
	}
	return mappings, rows.Err()
}

func (r *RecruitRepository) CloseByBranch(ctx context.Context, branch string) (int64, error) {
	tag, err := r.pool.Exec(ctx,
		`UPDATE recruit_messages SET closed_at = NOW()
		 WHERE closed_at IS NULL AND id IN (
		   SELECT DISTINCT rm.id
		   FROM recruit_messages rm
		   JOIN recruit_message_mappings rmm ON rm.id = rmm.recruit_message_id
		   JOIN studies s ON s.id = rmm.study_id
		   WHERE s.branch = $1 AND s.status = 'active'
		 )`, branch)
	if err != nil {
		return 0, fmt.Errorf("close recruit messages by branch: %w", err)
	}
	return tag.RowsAffected(), nil
}
