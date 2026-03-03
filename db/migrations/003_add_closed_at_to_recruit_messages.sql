ALTER TABLE recruit_messages ADD COLUMN IF NOT EXISTS closed_at TIMESTAMPTZ;
CREATE INDEX IF NOT EXISTS idx_recruit_messages_open ON recruit_messages (closed_at) WHERE closed_at IS NULL;
