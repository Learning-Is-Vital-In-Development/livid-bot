DELETE FROM study_proposal_votes v
USING study_proposals p
WHERE v.proposal_id = p.id
  AND (p.message_id = '' OR p.channel_id = '');

DELETE FROM study_proposals
WHERE message_id = '' OR channel_id = '';

WITH shifted AS (
    SELECT
        id,
        created_at + ((ROW_NUMBER() OVER (PARTITION BY created_at ORDER BY id) - 1) * INTERVAL '1 microsecond') AS adjusted_created_at
    FROM proposal_periods
)
UPDATE proposal_periods p
SET created_at = shifted.adjusted_created_at
FROM shifted
WHERE p.id = shifted.id
  AND p.created_at <> shifted.adjusted_created_at;

UPDATE proposal_periods
SET closes_at = created_at + INTERVAL '1 second'
WHERE closes_at <= created_at;

WITH ordered AS (
    SELECT
        id,
        closes_at,
        LEAD(created_at) OVER (ORDER BY created_at, id) AS next_created_at
    FROM proposal_periods
)
UPDATE proposal_periods p
SET closes_at = ordered.next_created_at
FROM ordered
WHERE p.id = ordered.id
  AND ordered.next_created_at IS NOT NULL
  AND ordered.closes_at > ordered.next_created_at;

ALTER TABLE proposal_periods
    ADD CONSTRAINT proposal_periods_closes_after_create
    CHECK (closes_at > created_at);

ALTER TABLE proposal_periods
    ADD CONSTRAINT proposal_periods_no_overlap
    EXCLUDE USING gist (tstzrange(created_at, closes_at, '[)') WITH &&);

ALTER TABLE study_proposals
    ALTER COLUMN message_id DROP DEFAULT,
    ALTER COLUMN channel_id DROP DEFAULT;

ALTER TABLE study_proposals
    ADD CONSTRAINT study_proposals_message_id_non_empty CHECK (message_id <> ''),
    ADD CONSTRAINT study_proposals_channel_id_non_empty CHECK (channel_id <> '');
