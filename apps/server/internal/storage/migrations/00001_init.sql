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

-- No table stores my.kpi.ua credentials. The browser extension authenticates
-- to my.kpi.ua using the student's own already-logged-in browser session,
-- extracts the parsed schedule client-side, and pushes only the resulting
-- lesson list here — the server never sees or stores raw session cookies.
-- See docs/architecture/data-storage.md §1 and docs/extension/browser-extension-design.md.
CREATE TABLE user_schedule_state (
    user_id            uuid PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    refreshed_at       timestamptz NOT NULL DEFAULT now(),
    lesson_count       integer NOT NULL DEFAULT 0,
    enrichment_status  text NOT NULL DEFAULT 'none' CHECK (enrichment_status IN ('full', 'degraded', 'none')),
    last_error         text
);

-- Each row is one concrete, dated class occurrence, as returned directly by
-- my.kpi.ua's personal calendar feed (already exact-dated, already filtered
-- to only this student's actual enrollment). week/day/slot are derived from
-- `date` at refresh time and stored for display grouping and for matching
-- against the Campus API's week-pattern schedule; `date` is authoritative.
CREATE TABLE user_lessons (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id        uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    date           date NOT NULL,
    week           smallint NOT NULL CHECK (week IN (1, 2)),
    day            smallint NOT NULL CHECK (day BETWEEN 1 AND 7),
    slot           smallint NOT NULL DEFAULT 0 CHECK (slot BETWEEN 0 AND 7),
    start_time     text NOT NULL,
    end_time       text NOT NULL DEFAULT '',
    subject        text NOT NULL,
    subject_norm   text NOT NULL,
    tag            text NOT NULL DEFAULT '',
    teacher_raw    text NOT NULL DEFAULT '',
    location_raw   text NOT NULL DEFAULT '',
    lecturer_id    text,
    lecturer_name  text,
    location_title text,
    location_uri   text,
    enriched       boolean NOT NULL DEFAULT false,
    UNIQUE (user_id, date, start_time, subject_norm)
);

CREATE INDEX idx_user_lessons_user_date ON user_lessons (user_id, date);

-- +goose Down
DROP TABLE user_lessons;
DROP TABLE user_schedule_state;
DROP TABLE users;
