CREATE TABLE IF NOT EXISTS command_audit_logs (
    interaction_id TEXT PRIMARY KEY,
    command_name   TEXT NOT NULL,
    actor_user_id  TEXT NOT NULL,
    guild_id       TEXT NOT NULL,
    channel_id     TEXT NOT NULL,
    options_json   JSONB NOT NULL DEFAULT '[]'::jsonb,
    status         TEXT NOT NULL CHECK (status IN ('triggered', 'success', 'error')),
    error_message  TEXT,
    triggered_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at   TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_command_audit_logs_triggered_at
    ON command_audit_logs (triggered_at DESC);

CREATE INDEX IF NOT EXISTS idx_command_audit_logs_command_name_triggered_at
    ON command_audit_logs (command_name, triggered_at DESC);

CREATE INDEX IF NOT EXISTS idx_command_audit_logs_actor_user_id_triggered_at
    ON command_audit_logs (actor_user_id, triggered_at DESC);
