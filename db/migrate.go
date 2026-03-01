package db

import (
	"context"
	"embed"
	"fmt"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	entries, err := migrationFS.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("read migration directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		sql, err := migrationFS.ReadFile("migrations/" + entry.Name())
		if err != nil {
			return fmt.Errorf("read migration %s: %w", entry.Name(), err)
		}

		log.Printf("Running migration: %s", entry.Name())
		if _, err := pool.Exec(ctx, string(sql)); err != nil {
			return fmt.Errorf("execute migration %s: %w", entry.Name(), err)
		}
	}

	return nil
}
