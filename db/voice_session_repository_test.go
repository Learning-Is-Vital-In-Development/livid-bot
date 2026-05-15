package db

import (
	"context"
	"testing"
	"time"
)

func TestVoiceSessionEndReasonForTransition(t *testing.T) {
	tests := []struct {
		name   string
		before string
		after  string
		want   string
	}{
		{name: "leave", before: "voice-1", after: "", want: "leave"},
		{name: "move", before: "voice-1", after: "voice-2", want: "move"},
		{name: "replace stale open session on join", before: "", after: "voice-2", want: "replaced"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := voiceSessionEndReason(tt.before, tt.after); got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

func TestVoiceSessionRepositoryRecordsTransitionsAndStats(t *testing.T) {
	td := newTestDatabase(t)
	repo := NewVoiceSessionRepository(td.Pool)
	ctx := context.Background()
	base := time.Date(2026, 5, 1, 9, 0, 0, 0, time.UTC)

	if err := repo.RecordVoiceTransition(ctx, "guild-1", "user-1", "", "voice-1", base); err != nil {
		t.Fatalf("record user-1 join voice-1: %v", err)
	}
	if err := repo.RecordVoiceTransition(ctx, "guild-1", "user-1", "voice-1", "voice-2", base.Add(45*time.Minute)); err != nil {
		t.Fatalf("record user-1 move voice-2: %v", err)
	}
	if err := repo.RecordVoiceTransition(ctx, "guild-1", "user-1", "voice-2", "", base.Add(90*time.Minute)); err != nil {
		t.Fatalf("record user-1 leave voice-2: %v", err)
	}
	if err := repo.RecordVoiceTransition(ctx, "guild-1", "user-2", "", "voice-1", base.Add(15*time.Minute)); err != nil {
		t.Fatalf("record user-2 join voice-1: %v", err)
	}

	stats, err := repo.ListChannelStats(ctx, "guild-1", "voice-1", base, base.Add(time.Hour), 10)
	if err != nil {
		t.Fatalf("list voice-1 stats: %v", err)
	}
	if len(stats) != 2 {
		t.Fatalf("expected 2 stats rows, got %d: %+v", len(stats), stats)
	}
	if stats[0].UserID != "user-1" || stats[0].TotalSeconds != int64(45*time.Minute/time.Second) || stats[0].SessionCount != 1 {
		t.Fatalf("unexpected first stats row: %+v", stats[0])
	}
	if stats[1].UserID != "user-2" || stats[1].TotalSeconds != int64(45*time.Minute/time.Second) || stats[1].SessionCount != 1 {
		t.Fatalf("unexpected second stats row: %+v", stats[1])
	}
}

func TestVoiceSessionRepositoryCapsOpenSessionStatsAtNow(t *testing.T) {
	td := newTestDatabase(t)
	repo := NewVoiceSessionRepository(td.Pool)
	ctx := context.Background()
	base := time.Date(2026, 5, 1, 9, 0, 0, 0, time.UTC)

	if err := repo.RecordVoiceTransition(ctx, "guild-1", "user-1", "", "voice-1", base); err != nil {
		t.Fatalf("record open join: %v", err)
	}

	stats, err := repo.listChannelStatsAt(ctx, "guild-1", "voice-1", base, base.Add(24*time.Hour), 10, base.Add(2*time.Hour))
	if err != nil {
		t.Fatalf("list stats: %v", err)
	}
	if len(stats) != 1 {
		t.Fatalf("expected one stats row, got %d: %+v", len(stats), stats)
	}
	if stats[0].TotalSeconds != int64(2*time.Hour/time.Second) {
		t.Fatalf("expected open session to be capped at now, got %d seconds", stats[0].TotalSeconds)
	}
}

func TestVoiceSessionRepositoryReplacesStaleOpenSessionOnJoin(t *testing.T) {
	td := newTestDatabase(t)
	repo := NewVoiceSessionRepository(td.Pool)
	ctx := context.Background()
	joinedAt := time.Date(2026, 5, 1, 9, 0, 0, 0, time.UTC)

	if err := repo.RecordVoiceTransition(ctx, "guild-1", "user-1", "", "voice-1", joinedAt); err != nil {
		t.Fatalf("record initial join: %v", err)
	}
	if err := repo.RecordVoiceTransition(ctx, "guild-1", "user-1", "", "voice-2", joinedAt.Add(10*time.Minute)); err != nil {
		t.Fatalf("record replacement join: %v", err)
	}

	var oldReason string
	if err := td.Pool.QueryRow(ctx,
		`SELECT end_reason FROM voice_channel_sessions WHERE guild_id = $1 AND user_id = $2 AND channel_id = $3`,
		"guild-1", "user-1", "voice-1",
	).Scan(&oldReason); err != nil {
		t.Fatalf("load replaced session: %v", err)
	}
	if oldReason != "replaced" {
		t.Fatalf("expected replaced end reason, got %q", oldReason)
	}

	var openChannelID string
	if err := td.Pool.QueryRow(ctx,
		`SELECT channel_id FROM voice_channel_sessions WHERE guild_id = $1 AND user_id = $2 AND left_at IS NULL`,
		"guild-1", "user-1",
	).Scan(&openChannelID); err != nil {
		t.Fatalf("load open session: %v", err)
	}
	if openChannelID != "voice-2" {
		t.Fatalf("expected new open session in voice-2, got %q", openChannelID)
	}
}

func TestVoiceSessionRepositoryClosesOpenSessionsOnRestart(t *testing.T) {
	td := newTestDatabase(t)
	repo := NewVoiceSessionRepository(td.Pool)
	ctx := context.Background()
	joinedAt := time.Date(2026, 5, 1, 9, 0, 0, 0, time.UTC)
	closedAt := joinedAt.Add(10 * time.Minute)

	if err := repo.RecordVoiceTransition(ctx, "guild-1", "user-1", "", "voice-1", joinedAt); err != nil {
		t.Fatalf("record join: %v", err)
	}
	closed, err := repo.CloseOpenSessions(ctx, closedAt, "bot_restart")
	if err != nil {
		t.Fatalf("close open sessions: %v", err)
	}
	if closed != 1 {
		t.Fatalf("expected one closed session, got %d", closed)
	}

	var leftAt time.Time
	var endReason string
	if err := td.Pool.QueryRow(ctx,
		`SELECT left_at, end_reason FROM voice_channel_sessions WHERE guild_id = $1 AND user_id = $2`,
		"guild-1", "user-1",
	).Scan(&leftAt, &endReason); err != nil {
		t.Fatalf("load closed session: %v", err)
	}
	if !leftAt.Equal(closedAt) {
		t.Fatalf("expected left_at %s, got %s", closedAt, leftAt)
	}
	if endReason != "bot_restart" {
		t.Fatalf("expected bot_restart end reason, got %q", endReason)
	}
}

func TestVoiceSessionRepositoryDoesNotStoreDisplayNames(t *testing.T) {
	td := newTestDatabase(t)
	ctx := context.Background()

	var count int
	if err := td.Pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM information_schema.columns
		WHERE table_name = 'voice_channel_sessions'
		  AND column_name IN ('username', 'nickname', 'display_name', 'global_name')
	`).Scan(&count); err != nil {
		t.Fatalf("query voice session columns: %v", err)
	}
	if count != 0 {
		t.Fatalf("voice_channel_sessions should not store display names, found %d display-name columns", count)
	}
}
