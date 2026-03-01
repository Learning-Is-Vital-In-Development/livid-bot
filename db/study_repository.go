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

func (r *StudyRepository) Create(ctx context.Context, name, description, channelID, roleID string) (study.Study, error) {
	var s study.Study
	err := r.pool.QueryRow(ctx,
		`INSERT INTO studies (name, description, channel_id, role_id)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, name, description, channel_id, role_id, created_at, status`,
		name, description, channelID, roleID,
	).Scan(&s.ID, &s.Name, &s.Description, &s.ChannelID, &s.RoleID, &s.CreatedAt, &s.Status)
	if err != nil {
		return study.Study{}, fmt.Errorf("create study: %w", err)
	}
	return s, nil
}

func (r *StudyRepository) FindAllActive(ctx context.Context) ([]study.Study, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, name, description, channel_id, role_id, created_at, status
		 FROM studies WHERE status = 'active' ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("find active studies: %w", err)
	}
	defer rows.Close()

	var studies []study.Study
	for rows.Next() {
		var s study.Study
		if err := rows.Scan(&s.ID, &s.Name, &s.Description, &s.ChannelID, &s.RoleID, &s.CreatedAt, &s.Status); err != nil {
			return nil, fmt.Errorf("scan study: %w", err)
		}
		studies = append(studies, s)
	}
	return studies, rows.Err()
}

func (r *StudyRepository) FindByName(ctx context.Context, name string) (study.Study, error) {
	var s study.Study
	err := r.pool.QueryRow(ctx,
		`SELECT id, name, description, channel_id, role_id, created_at, status
		 FROM studies WHERE name = $1`, name,
	).Scan(&s.ID, &s.Name, &s.Description, &s.ChannelID, &s.RoleID, &s.CreatedAt, &s.Status)
	if err != nil {
		return study.Study{}, fmt.Errorf("find study by name: %w", err)
	}
	return s, nil
}

func (r *StudyRepository) Archive(ctx context.Context, name string) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE studies SET status = 'archived' WHERE name = $1 AND status = 'active'`, name)
	if err != nil {
		return fmt.Errorf("archive study: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("study %q not found or already archived", name)
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
