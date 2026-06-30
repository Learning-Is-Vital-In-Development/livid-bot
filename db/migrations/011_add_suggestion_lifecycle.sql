ALTER TABLE study_suggestions
    ALTER COLUMN period_id DROP NOT NULL;

ALTER TABLE study_suggestions
    ADD COLUMN visibility TEXT NOT NULL DEFAULT 'anonymous',
    ADD COLUMN proposer_user_id TEXT,
    ADD COLUMN proposer_display_name TEXT,
    ADD COLUMN threshold INT NOT NULL DEFAULT 3,
    ADD COLUMN expires_at TIMESTAMPTZ,
    ADD COLUMN status TEXT NOT NULL DEFAULT 'open',
    ADD COLUMN opened_study_id BIGINT REFERENCES studies(id),
    ADD COLUMN opened_at TIMESTAMPTZ,
    ADD COLUMN opening_error TEXT;

UPDATE study_suggestions s
SET expires_at = p.closes_at
FROM suggestion_periods p
WHERE s.period_id = p.id
  AND s.expires_at IS NULL;

UPDATE study_suggestions
SET expires_at = created_at + INTERVAL '14 days'
WHERE expires_at IS NULL;

ALTER TABLE study_suggestions
    ALTER COLUMN expires_at SET NOT NULL;

ALTER TABLE study_suggestions
    ADD CONSTRAINT study_suggestions_visibility_check
    CHECK (visibility IN ('anonymous', 'public'));

ALTER TABLE study_suggestions
    ADD CONSTRAINT study_suggestions_status_check
    CHECK (status IN (
        'open',
        'opened',
        'expired',
        'closed_by_proposer',
        'closed_by_moderator',
        'opening_failed'
    ));

ALTER TABLE study_suggestions
    ADD CONSTRAINT study_suggestions_threshold_check
    CHECK (threshold >= 1);

CREATE UNIQUE INDEX IF NOT EXISTS study_suggestions_message_ref_key
    ON study_suggestions(channel_id, message_id);

CREATE INDEX IF NOT EXISTS idx_study_suggestions_status_expires_at
    ON study_suggestions(status, expires_at);
