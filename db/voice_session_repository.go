package db

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"livid-bot/study"
)

const defaultVoiceStatsLimit = 20

type VoiceSessionRepository struct {
	pool *pgxpool.Pool
}

func NewVoiceSessionRepository(pool *pgxpool.Pool) *VoiceSessionRepository {
	return &VoiceSessionRepository{pool: pool}
}

func voiceSessionEndReason(beforeChannelID, afterChannelID string) string {
	switch {
	case beforeChannelID != "" && afterChannelID != "":
		return "move"
	case beforeChannelID == "" && afterChannelID != "":
		return "replaced"
	default:
		return "leave"
	}
}

func (r *VoiceSessionRepository) RecordVoiceTransition(
	ctx context.Context,
	guildID, userID, beforeChannelID, afterChannelID string,
	occurredAt time.Time,
) error {
	if guildID == "" {
		return fmt.Errorf("record voice transition: guild id is required")
	}
	if userID == "" {
		return fmt.Errorf("record voice transition: user id is required")
	}
	if beforeChannelID == afterChannelID {
		return nil
	}
	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin voice transition: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	if _, err := tx.Exec(ctx,
		`SELECT pg_advisory_xact_lock(hashtextextended($1 || ':' || $2, 0))`,
		guildID, userID,
	); err != nil {
		return fmt.Errorf("lock voice session user: %w", err)
	}

	if beforeChannelID != "" || afterChannelID != "" {
		endReason := voiceSessionEndReason(beforeChannelID, afterChannelID)
		if _, err := tx.Exec(ctx,
			`UPDATE voice_channel_sessions
			 SET left_at = $3, end_reason = $4
			 WHERE guild_id = $1 AND user_id = $2 AND left_at IS NULL`,
			guildID, userID, occurredAt, endReason,
		); err != nil {
			return fmt.Errorf("close open voice session: %w", err)
		}
	}

	if afterChannelID != "" {
		if _, err := tx.Exec(ctx,
			`INSERT INTO voice_channel_sessions (guild_id, channel_id, user_id, joined_at)
			 VALUES ($1, $2, $3, $4)`,
			guildID, afterChannelID, userID, occurredAt,
		); err != nil {
			return fmt.Errorf("open voice session: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit voice transition: %w", err)
	}
	return nil
}

func (r *VoiceSessionRepository) CloseOpenSessions(ctx context.Context, endedAt time.Time, reason string) (int64, error) {
	if endedAt.IsZero() {
		endedAt = time.Now().UTC()
	}
	if reason == "" {
		reason = "closed"
	}
	tag, err := r.pool.Exec(ctx,
		`UPDATE voice_channel_sessions
		 SET left_at = $1, end_reason = $2
		 WHERE left_at IS NULL`,
		endedAt, reason,
	)
	if err != nil {
		return 0, fmt.Errorf("close open voice sessions: %w", err)
	}
	return tag.RowsAffected(), nil
}

func (r *VoiceSessionRepository) ListChannelStats(
	ctx context.Context,
	guildID, channelID string,
	from, to time.Time,
	limit int,
) ([]study.VoiceChannelStat, error) {
	return r.listChannelStatsAt(ctx, guildID, channelID, from, to, limit, time.Now().UTC())
}

func (r *VoiceSessionRepository) listChannelStatsAt(
	ctx context.Context,
	guildID, channelID string,
	from, to time.Time,
	limit int,
	now time.Time,
) ([]study.VoiceChannelStat, error) {
	if guildID == "" {
		return nil, fmt.Errorf("list voice channel stats: guild id is required")
	}
	if channelID == "" {
		return nil, fmt.Errorf("list voice channel stats: channel id is required")
	}
	if !from.Before(to) {
		return nil, fmt.Errorf("list voice channel stats: from must be before to")
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if limit <= 0 {
		limit = defaultVoiceStatsLimit
	}

	rows, err := r.pool.Query(ctx,
		`SELECT
			user_id,
			COUNT(*)::BIGINT AS session_count,
			COALESCE(ROUND(SUM(EXTRACT(EPOCH FROM LEAST(COALESCE(left_at, $6), $4) - GREATEST(joined_at, $3))))::BIGINT, 0) AS total_seconds
		 FROM voice_channel_sessions
		 WHERE guild_id = $1
		   AND channel_id = $2
		   AND joined_at < $4
		   AND COALESCE(left_at, $6) > $3
		 GROUP BY user_id
		 HAVING COALESCE(SUM(EXTRACT(EPOCH FROM LEAST(COALESCE(left_at, $6), $4) - GREATEST(joined_at, $3))), 0) > 0
		 ORDER BY total_seconds DESC, user_id
		 LIMIT $5`,
		guildID, channelID, from, to, limit, now,
	)
	if err != nil {
		return nil, fmt.Errorf("query voice channel stats: %w", err)
	}
	defer rows.Close()

	stats := make([]study.VoiceChannelStat, 0)
	for rows.Next() {
		var stat study.VoiceChannelStat
		if err := rows.Scan(&stat.UserID, &stat.SessionCount, &stat.TotalSeconds); err != nil {
			return nil, fmt.Errorf("scan voice channel stats: %w", err)
		}
		stats = append(stats, stat)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate voice channel stats: %w", err)
	}
	if err := r.appendVoiceChannelSessionDetails(ctx, stats, guildID, channelID, from, to, now); err != nil {
		return nil, err
	}
	return stats, nil
}

func (r *VoiceSessionRepository) appendVoiceChannelSessionDetails(
	ctx context.Context,
	stats []study.VoiceChannelStat,
	guildID, channelID string,
	from, to, now time.Time,
) error {
	if len(stats) == 0 {
		return nil
	}
	userIDs := make([]string, 0, len(stats))
	statsByUserID := make(map[string]int, len(stats))
	for idx := range stats {
		userIDs = append(userIDs, stats[idx].UserID)
		statsByUserID[stats[idx].UserID] = idx
	}

	rows, err := r.pool.Query(ctx,
		`WITH clipped_sessions AS (
			SELECT
				user_id,
				GREATEST(joined_at, $3) AS joined_at,
				LEAST(COALESCE(left_at, $6), $4) AS left_at,
				left_at IS NULL AND $6 < $4 AS is_open
			 FROM voice_channel_sessions
			 WHERE guild_id = $1
			   AND channel_id = $2
			   AND joined_at < $4
			   AND COALESCE(left_at, $6) > $3
			   AND user_id = ANY($5::TEXT[])
		 )
		 SELECT
			user_id,
			joined_at,
			left_at,
			ROUND(EXTRACT(EPOCH FROM left_at - joined_at))::BIGINT AS duration_seconds,
			is_open
		 FROM clipped_sessions
		 WHERE left_at > joined_at
		 ORDER BY user_id, joined_at ASC, left_at ASC`,
		guildID, channelID, from, to, userIDs, now,
	)
	if err != nil {
		return fmt.Errorf("query voice channel session details: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var userID string
		var session study.VoiceChannelSession
		if err := rows.Scan(&userID, &session.JoinedAt, &session.LeftAt, &session.DurationSeconds, &session.IsOpen); err != nil {
			return fmt.Errorf("scan voice channel session details: %w", err)
		}
		idx, ok := statsByUserID[userID]
		if !ok {
			continue
		}
		stats[idx].Sessions = append(stats[idx].Sessions, session)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate voice channel session details: %w", err)
	}
	return nil
}
