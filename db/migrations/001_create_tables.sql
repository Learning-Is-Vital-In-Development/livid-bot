CREATE TABLE IF NOT EXISTS studies (
    id          BIGSERIAL PRIMARY KEY,
    name        TEXT NOT NULL UNIQUE,
    description TEXT NOT NULL DEFAULT '',
    channel_id  TEXT NOT NULL,
    role_id     TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    status      TEXT NOT NULL DEFAULT 'active'
);

CREATE TABLE IF NOT EXISTS study_members (
    study_id  BIGINT NOT NULL REFERENCES studies(id),
    user_id   TEXT NOT NULL,
    username  TEXT NOT NULL,
    joined_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    left_at   TIMESTAMPTZ,
    PRIMARY KEY (study_id, user_id)
);

CREATE TABLE IF NOT EXISTS recruit_messages (
    id         BIGSERIAL PRIMARY KEY,
    message_id TEXT NOT NULL UNIQUE,
    channel_id TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS recruit_message_mappings (
    recruit_message_id BIGINT NOT NULL REFERENCES recruit_messages(id),
    emoji              TEXT NOT NULL,
    study_id           BIGINT NOT NULL REFERENCES studies(id),
    role_id            TEXT NOT NULL,
    PRIMARY KEY (recruit_message_id, emoji)
);
