package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"livid-bot/study"
)

type MemberRepository struct {
	pool *pgxpool.Pool
}

func NewMemberRepository(pool *pgxpool.Pool) *MemberRepository {
	return &MemberRepository{pool: pool}
}

func (r *MemberRepository) AddMember(ctx context.Context, studyID int64, userID, username string) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO study_members (study_id, user_id, username)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (study_id, user_id)
		 DO UPDATE SET left_at = NULL, username = EXCLUDED.username`,
		studyID, userID, username,
	)
	if err != nil {
		return fmt.Errorf("add member: %w", err)
	}
	return nil
}

func (r *MemberRepository) FindActiveByStudyID(ctx context.Context, studyID int64) ([]study.StudyMember, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT study_id, user_id, username, joined_at
		 FROM study_members
		 WHERE study_id = $1 AND left_at IS NULL
		 ORDER BY joined_at`, studyID)
	if err != nil {
		return nil, fmt.Errorf("find active members by study: %w", err)
	}
	defer rows.Close()

	var members []study.StudyMember
	for rows.Next() {
		var m study.StudyMember
		if err := rows.Scan(&m.StudyID, &m.UserID, &m.Username, &m.JoinedAt); err != nil {
			return nil, fmt.Errorf("scan member: %w", err)
		}
		members = append(members, m)
	}
	return members, rows.Err()
}

func (r *MemberRepository) RemoveMember(ctx context.Context, studyID int64, userID string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE study_members SET left_at = NOW()
		 WHERE study_id = $1 AND user_id = $2 AND left_at IS NULL`,
		studyID, userID,
	)
	if err != nil {
		return fmt.Errorf("remove member: %w", err)
	}
	return nil
}
