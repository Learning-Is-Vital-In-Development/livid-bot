package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type CommandAuditRepository struct {
	pool *pgxpool.Pool
}

func NewCommandAuditRepository(pool *pgxpool.Pool) *CommandAuditRepository {
	return &CommandAuditRepository{pool: pool}
}

func (r *CommandAuditRepository) RecordTriggered(
	ctx context.Context,
	interactionID, commandName, actorUserID, guildID, channelID, optionsJSON string,
) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO command_audit_logs (
			interaction_id, command_name, actor_user_id, guild_id, channel_id, options_json, status
		)
		VALUES ($1, $2, $3, $4, $5, $6::jsonb, 'triggered')
		ON CONFLICT (interaction_id) DO NOTHING`,
		interactionID, commandName, actorUserID, guildID, channelID, optionsJSON,
	)
	if err != nil {
		return fmt.Errorf("record command audit trigger: %w", err)
	}
	return nil
}

func (r *CommandAuditRepository) RecordSuccess(ctx context.Context, interactionID string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE command_audit_logs
		 SET status = 'success', completed_at = NOW()
		 WHERE interaction_id = $1 AND status = 'triggered'`,
		interactionID,
	)
	if err != nil {
		return fmt.Errorf("record command audit success: %w", err)
	}
	return nil
}

func (r *CommandAuditRepository) RecordError(ctx context.Context, interactionID, errorMessage string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE command_audit_logs
		 SET status = 'error',
		     completed_at = COALESCE(completed_at, NOW()),
		     error_message = COALESCE(error_message, $2)
		 WHERE interaction_id = $1 AND status = 'triggered'`,
		interactionID, errorMessage,
	)
	if err != nil {
		return fmt.Errorf("record command audit error: %w", err)
	}
	return nil
}
