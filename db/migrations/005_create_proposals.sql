CREATE TABLE IF NOT EXISTS proposal_periods (
    id         BIGSERIAL PRIMARY KEY,
    channel_id TEXT NOT NULL,
    closes_at  TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS study_proposals (
    id          BIGSERIAL PRIMARY KEY,
    period_id   BIGINT NOT NULL REFERENCES proposal_periods(id),
    title       TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    message_id  TEXT NOT NULL DEFAULT '',
    channel_id  TEXT NOT NULL DEFAULT '',
    vote_count  INT NOT NULL DEFAULT 0,
    confirmed   BOOLEAN NOT NULL DEFAULT FALSE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS study_proposal_votes (
    proposal_id BIGINT NOT NULL REFERENCES study_proposals(id),
    user_id     TEXT NOT NULL,
    voted_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (proposal_id, user_id)
);
