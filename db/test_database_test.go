package db

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type testDatabase struct {
	Pool        *pgxpool.Pool
	DatabaseURL string
}

func newTestDatabase(t *testing.T) *testDatabase {
	t.Helper()

	baseURL := os.Getenv("TEST_DATABASE_URL")
	if baseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}

	ctx := context.Background()
	adminPool, err := NewPool(ctx, baseURL)
	if err != nil {
		t.Fatalf("connect admin pool: %v", err)
	}

	databaseName := fmt.Sprintf("%s_%d", sanitizeDatabaseName(databaseNameFromURL(t, baseURL)), time.Now().UnixNano())
	if _, err := adminPool.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{databaseName}.Sanitize()); err != nil {
		adminPool.Close()
		t.Fatalf("create test database %q: %v", databaseName, err)
	}

	testURL := replaceDatabaseName(t, baseURL, databaseName)
	pool, err := NewPool(ctx, testURL)
	if err != nil {
		_, _ = adminPool.Exec(ctx, "DROP DATABASE "+pgx.Identifier{databaseName}.Sanitize()+" WITH (FORCE)")
		adminPool.Close()
		t.Fatalf("connect test pool: %v", err)
	}

	if err := Migrate(ctx, pool); err != nil {
		pool.Close()
		_, _ = adminPool.Exec(ctx, "DROP DATABASE "+pgx.Identifier{databaseName}.Sanitize()+" WITH (FORCE)")
		adminPool.Close()
		t.Fatalf("migrate test database: %v", err)
	}

	t.Cleanup(func() {
		pool.Close()
		if _, err := adminPool.Exec(ctx, "DROP DATABASE "+pgx.Identifier{databaseName}.Sanitize()+" WITH (FORCE)"); err != nil {
			t.Fatalf("drop test database %q: %v", databaseName, err)
		}
		adminPool.Close()
	})

	return &testDatabase{
		Pool:        pool,
		DatabaseURL: testURL,
	}
}

func replaceDatabaseName(t *testing.T, rawURL, databaseName string) string {
	t.Helper()

	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("parse database url: %v", err)
	}
	parsed.Path = "/" + databaseName
	return parsed.String()
}

func databaseNameFromURL(t *testing.T, rawURL string) string {
	t.Helper()

	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("parse database url: %v", err)
	}
	name := strings.TrimPrefix(parsed.Path, "/")
	if name == "" {
		t.Fatal("database url must include a database name")
	}
	return name
}

func sanitizeDatabaseName(name string) string {
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r + ('a' - 'A'))
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	if b.Len() == 0 {
		return "suggestion_test"
	}
	return b.String()
}
