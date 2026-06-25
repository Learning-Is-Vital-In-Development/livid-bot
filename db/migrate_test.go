package db

import (
	"context"
	"testing"
)

func TestMigrateDropsLegacyVoiceChannelSessions(t *testing.T) {
	tdb := newEmptyTestDatabase(t)
	ctx := context.Background()

	if _, err := tdb.Pool.Exec(ctx, `
		CREATE TABLE schema_migrations (
			filename TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
		CREATE TABLE voice_channel_sessions (
			id BIGSERIAL PRIMARY KEY,
			guild_id TEXT NOT NULL,
			channel_id TEXT NOT NULL,
			user_id TEXT NOT NULL,
			joined_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			left_at TIMESTAMPTZ,
			end_reason TEXT
		);
		INSERT INTO voice_channel_sessions (guild_id, channel_id, user_id)
		VALUES ('guild-1', 'channel-1', 'user-1');
	`); err != nil {
		t.Fatalf("seed legacy voice sessions table: %v", err)
	}

	entries, err := migrationFS.ReadDir("migrations")
	if err != nil {
		t.Fatalf("read migrations: %v", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || entry.Name() >= "010_drop_voice_channel_sessions.sql" {
			continue
		}
		if _, err := tdb.Pool.Exec(ctx, `INSERT INTO schema_migrations (filename) VALUES ($1)`, entry.Name()); err != nil {
			t.Fatalf("mark migration %s applied: %v", entry.Name(), err)
		}
	}
	if _, err := tdb.Pool.Exec(ctx, `INSERT INTO schema_migrations (filename) VALUES ('008_create_voice_channel_sessions.sql')`); err != nil {
		t.Fatalf("mark legacy voice migration applied: %v", err)
	}

	if err := Migrate(ctx, tdb.Pool); err != nil {
		t.Fatalf("migrate legacy database: %v", err)
	}

	var tableName *string
	if err := tdb.Pool.QueryRow(ctx, `SELECT to_regclass('public.voice_channel_sessions')::TEXT`).Scan(&tableName); err != nil {
		t.Fatalf("check legacy voice sessions table: %v", err)
	}
	if tableName != nil {
		t.Fatalf("expected voice_channel_sessions to be dropped, found %q", *tableName)
	}
}
