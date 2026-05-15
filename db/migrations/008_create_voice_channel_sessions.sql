CREATE TABLE IF NOT EXISTS voice_channel_sessions (
    id         BIGSERIAL PRIMARY KEY,
    guild_id   TEXT NOT NULL,
    channel_id TEXT NOT NULL,
    user_id    TEXT NOT NULL,
    joined_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    left_at    TIMESTAMPTZ,
    end_reason TEXT,
    CHECK (left_at IS NULL OR left_at >= joined_at)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_voice_channel_sessions_one_open_per_user
    ON voice_channel_sessions (guild_id, user_id)
    WHERE left_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_voice_channel_sessions_channel_time
    ON voice_channel_sessions (guild_id, channel_id, joined_at DESC);

CREATE INDEX IF NOT EXISTS idx_voice_channel_sessions_user_time
    ON voice_channel_sessions (guild_id, user_id, joined_at DESC);
