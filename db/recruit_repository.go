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

type RecruitStudyInfo struct {
	StudyID   int64
	StudyName string
	ChannelID string
	RoleID    string
}

func (r *RecruitRepository) FindOpenMappingsByBranch(ctx context.Context, branch string) ([]string, []RecruitStudyInfo, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT DISTINCT rm.message_id, s.id, s.name, s.channel_id, s.role_id
		 FROM recruit_messages rm
		 JOIN recruit_message_mappings rmm ON rm.id = rmm.recruit_message_id
		 JOIN studies s ON s.id = rmm.study_id
		 WHERE s.branch = $1 AND s.status = 'active' AND rm.closed_at IS NULL
		 ORDER BY s.id`, branch)
	if err != nil {
		return nil, nil, fmt.Errorf("find open mappings by branch: %w", err)
	}
	defer rows.Close()

	messageIDSet := make(map[string]struct{})
	var messageIDs []string
	studyIDSet := make(map[int64]struct{})
	var studies []RecruitStudyInfo

	for rows.Next() {
		var msgID string
		var info RecruitStudyInfo
		if err := rows.Scan(&msgID, &info.StudyID, &info.StudyName, &info.ChannelID, &info.RoleID); err != nil {
			return nil, nil, fmt.Errorf("scan open mapping: %w", err)
		}
		if _, exists := messageIDSet[msgID]; !exists {
			messageIDSet[msgID] = struct{}{}
			messageIDs = append(messageIDs, msgID)
		}
		if _, exists := studyIDSet[info.StudyID]; !exists {
			studyIDSet[info.StudyID] = struct{}{}
			studies = append(studies, info)
		}
	}
	return messageIDs, studies, rows.Err()
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
