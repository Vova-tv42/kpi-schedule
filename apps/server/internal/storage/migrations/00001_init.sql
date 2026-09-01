-- +goose Up
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE users (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    telegram_id  bigint NOT NULL UNIQUE,
    group_id     integer,
    group_name   text,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE user_sessions (
    user_id          uuid PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    ciphertext       bytea NOT NULL,
    user_agent       text NOT NULL DEFAULT '',
    status           text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'expired')),
    synced_at        timestamptz NOT NULL DEFAULT now(),
    last_checked_at  timestamptz NOT NULL DEFAULT now(),
    last_error       text
);

CREATE TABLE user_schedule_state (
    user_id            uuid PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    refreshed_at       timestamptz NOT NULL DEFAULT now(),
    lesson_count       integer NOT NULL DEFAULT 0,
    enrichment_status  text NOT NULL DEFAULT 'none' CHECK (enrichment_status IN ('full', 'degraded', 'none')),
    last_error         text
);

CREATE TABLE user_lessons (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id        uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    week           smallint NOT NULL CHECK (week IN (1, 2)),
    day            smallint NOT NULL CHECK (day BETWEEN 1 AND 6),
    slot           smallint NOT NULL CHECK (slot BETWEEN 1 AND 7),
    start_time     text NOT NULL,
    subject        text NOT NULL,
    subject_norm   text NOT NULL,
    tag            text NOT NULL DEFAULT '',
    type           text NOT NULL DEFAULT '',
    lecturer_id    text,
    lecturer_name  text,
    location_title text,
    location_uri   text,
    dates          text[] NOT NULL DEFAULT '{}',
    enriched       boolean NOT NULL DEFAULT false,
    UNIQUE (user_id, week, day, slot, subject_norm)
);

CREATE INDEX idx_user_lessons_user_week_day ON user_lessons (user_id, week, day);

-- +goose Down
DROP TABLE user_lessons;
DROP TABLE user_schedule_state;
DROP TABLE user_sessions;
DROP TABLE users;
