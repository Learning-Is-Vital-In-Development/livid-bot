package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"livid-bot/study"
)

type RecruitRepository struct {
	pool *pgxpool.Pool
}

func NewRecruitRepository(pool *pgxpool.Pool) *RecruitRepository {
	return &RecruitRepository{pool: pool}
}

func (r *RecruitRepository) SaveRecruitMessage(ctx context.Context, messageID, channelID string, mappings []study.RecruitMapping) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	var recruitID int64
	err = tx.QueryRow(ctx,
		`INSERT INTO recruit_messages (message_id, channel_id)
		 VALUES ($1, $2)
		 RETURNING id`,
		messageID, channelID,
	).Scan(&recruitID)
	if err != nil {
		return fmt.Errorf("insert recruit message: %w", err)
	}

	for _, m := range mappings {
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
		 WHERE s.status = 'active'`)
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
