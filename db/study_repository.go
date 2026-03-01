package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"livid-bot/study"
)

type StudyRepository struct {
	pool *pgxpool.Pool
}

func NewStudyRepository(pool *pgxpool.Pool) *StudyRepository {
	return &StudyRepository{pool: pool}
}

func (r *StudyRepository) Create(ctx context.Context, branch, name, description, channelID, roleID string) (study.Study, error) {
	var s study.Study
	err := r.pool.QueryRow(ctx,
		`INSERT INTO studies (branch, name, description, channel_id, role_id)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, branch, name, description, channel_id, role_id, created_at, status`,
		branch, name, description, channelID, roleID,
	).Scan(&s.ID, &s.Branch, &s.Name, &s.Description, &s.ChannelID, &s.RoleID, &s.CreatedAt, &s.Status)
	if err != nil {
		return study.Study{}, fmt.Errorf("create study: %w", err)
	}
	return s, nil
}

func (r *StudyRepository) FindAllActive(ctx context.Context) ([]study.Study, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, branch, name, description, channel_id, role_id, created_at, status
		 FROM studies WHERE status = 'active' ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("find active studies: %w", err)
	}
	defer rows.Close()

	var studies []study.Study
	for rows.Next() {
		var s study.Study
		if err := rows.Scan(&s.ID, &s.Branch, &s.Name, &s.Description, &s.ChannelID, &s.RoleID, &s.CreatedAt, &s.Status); err != nil {
			return nil, fmt.Errorf("scan study: %w", err)
		}
		studies = append(studies, s)
	}
	return studies, rows.Err()
}

func (r *StudyRepository) FindAllActiveByBranch(ctx context.Context, branch string) ([]study.Study, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, branch, name, description, channel_id, role_id, created_at, status
		 FROM studies WHERE status = 'active' AND branch = $1 ORDER BY id`, branch)
	if err != nil {
		return nil, fmt.Errorf("find active studies by branch: %w", err)
	}
	defer rows.Close()

	var studies []study.Study
	for rows.Next() {
		var s study.Study
		if err := rows.Scan(&s.ID, &s.Branch, &s.Name, &s.Description, &s.ChannelID, &s.RoleID, &s.CreatedAt, &s.Status); err != nil {
			return nil, fmt.Errorf("scan study: %w", err)
		}
		studies = append(studies, s)
	}
	return studies, rows.Err()
}

func (r *StudyRepository) FindDistinctActiveBranches(ctx context.Context) ([]string, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT DISTINCT branch
		 FROM studies
		 WHERE status = 'active' AND branch <> ''
		 ORDER BY branch`)
	if err != nil {
		return nil, fmt.Errorf("find distinct active branches: %w", err)
	}
	defer rows.Close()

	branches := make([]string, 0)
	for rows.Next() {
		var branch string
		if err := rows.Scan(&branch); err != nil {
			return nil, fmt.Errorf("scan branch: %w", err)
		}
		branches = append(branches, branch)
	}
	return branches, rows.Err()
}

func (r *StudyRepository) FindByName(ctx context.Context, name string) (study.Study, error) {
	var s study.Study
	err := r.pool.QueryRow(ctx,
		`SELECT id, branch, name, description, channel_id, role_id, created_at, status
		 FROM studies WHERE name = $1`, name,
	).Scan(&s.ID, &s.Branch, &s.Name, &s.Description, &s.ChannelID, &s.RoleID, &s.CreatedAt, &s.Status)
	if err != nil {
		return study.Study{}, fmt.Errorf("find study by name: %w", err)
	}
	return s, nil
}

func (r *StudyRepository) FindByChannelID(ctx context.Context, channelID string) (study.Study, error) {
	var s study.Study
	err := r.pool.QueryRow(ctx,
		`SELECT id, branch, name, description, channel_id, role_id, created_at, status
		 FROM studies WHERE channel_id = $1`, channelID,
	).Scan(&s.ID, &s.Branch, &s.Name, &s.Description, &s.ChannelID, &s.RoleID, &s.CreatedAt, &s.Status)
	if err != nil {
		return study.Study{}, fmt.Errorf("find study by channel id: %w", err)
	}
	return s, nil
}

func (r *StudyRepository) ArchiveByID(ctx context.Context, studyID int64) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE studies SET status = 'archived' WHERE id = $1 AND status = 'active'`, studyID)
	if err != nil {
		return fmt.Errorf("archive study: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("study %d not found or already archived", studyID)
	}
	return nil
}

func (r *StudyRepository) ArchiveAll(ctx context.Context) (int64, error) {
	tag, err := r.pool.Exec(ctx,
		`UPDATE studies SET status = 'archived' WHERE status = 'active'`)
	if err != nil {
		return 0, fmt.Errorf("archive all studies: %w", err)
	}
	return tag.RowsAffected(), nil
}
