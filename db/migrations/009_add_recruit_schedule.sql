ALTER TABLE recruit_messages ADD COLUMN IF NOT EXISTS branch TEXT;
ALTER TABLE recruit_messages ADD COLUMN IF NOT EXISTS opens_at TIMESTAMPTZ;
ALTER TABLE recruit_messages ADD COLUMN IF NOT EXISTS closes_at TIMESTAMPTZ;

UPDATE recruit_messages rm
SET branch = derived.branch
FROM (
    SELECT rm_inner.id, MIN(s.branch) AS branch
    FROM recruit_messages rm_inner
    JOIN recruit_message_mappings rmm ON rm_inner.id = rmm.recruit_message_id
    JOIN studies s ON s.id = rmm.study_id
    GROUP BY rm_inner.id
) AS derived
WHERE rm.id = derived.id
  AND rm.branch IS NULL;

UPDATE recruit_messages
SET opens_at = created_at
WHERE opens_at IS NULL;

UPDATE recruit_messages
SET closes_at = created_at
WHERE closes_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_recruit_messages_branch_open ON recruit_messages (branch, closed_at) WHERE closed_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_recruit_messages_closes_at_open ON recruit_messages (closes_at) WHERE closed_at IS NULL;
