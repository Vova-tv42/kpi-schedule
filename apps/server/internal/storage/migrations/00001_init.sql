-- +goose Up
CREATE TABLE users (
    id           TEXT PRIMARY KEY,
    telegram_id  INTEGER NOT NULL UNIQUE,
    group_id     INTEGER,
    group_name   TEXT,
    created_at   TIMESTAMP NOT NULL,
    updated_at   TIMESTAMP NOT NULL
);

-- No table stores my.kpi.ua credentials. The browser extension authenticates
-- to my.kpi.ua using the student's own already-logged-in browser session,
-- extracts the parsed schedule client-side, and pushes only the resulting
-- lesson list here — the server never sees or stores raw session cookies.
-- See docs/architecture/data-storage.md §1 and docs/extension/browser-extension-design.md.
CREATE TABLE user_schedule_state (
    user_id            TEXT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    refreshed_at       TIMESTAMP NOT NULL,
    lesson_count       INTEGER NOT NULL DEFAULT 0,
    enrichment_status  TEXT NOT NULL DEFAULT 'none' CHECK (enrichment_status IN ('full', 'degraded', 'none')),
    last_error         TEXT
);

-- Each row is one concrete, dated class occurrence, as returned directly by
-- my.kpi.ua's personal calendar feed (already exact-dated, already filtered
-- to only this student's actual enrollment). week/day/slot are derived from
-- `date` at refresh time and stored for display grouping and for matching
-- against the Campus API's week-pattern schedule; `date` is authoritative.
CREATE TABLE user_lessons (
    id             TEXT PRIMARY KEY,
    user_id        TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    date           TIMESTAMP NOT NULL,
    week           INTEGER NOT NULL CHECK (week IN (1, 2)),
    day            INTEGER NOT NULL CHECK (day BETWEEN 1 AND 7),
    slot           INTEGER NOT NULL DEFAULT 0 CHECK (slot BETWEEN 0 AND 7),
    start_time     TEXT NOT NULL,
    end_time       TEXT NOT NULL DEFAULT '',
    subject        TEXT NOT NULL,
    subject_norm   TEXT NOT NULL,
    tag            TEXT NOT NULL DEFAULT '',
    teacher_raw    TEXT NOT NULL DEFAULT '',
    location_raw   TEXT NOT NULL DEFAULT '',
    lecturer_id    TEXT,
    lecturer_name  TEXT,
    location_title TEXT,
    location_uri   TEXT,
    enriched       BOOLEAN NOT NULL DEFAULT 0,
    -- True if this lesson happens every week of its `week` parity; false if
    -- it only occurs on specific calendar dates (per the matched Campus
    -- group lesson's dates[]). Defaults true when unenriched. Read paths use
    -- this to exclude one-off/irregular lessons from the generic /week
    -- template view — see docs/architecture/merging-engine.md §6.
    is_recurring   BOOLEAN NOT NULL DEFAULT 1,
    UNIQUE (user_id, date, start_time, subject_norm)
);

CREATE INDEX idx_user_lessons_user_date ON user_lessons (user_id, date);

-- Disk-backed replacement for the old in-memory internal/cache TTL map.
-- Generic JSON key/value cache for api.campus.kpi.ua responses (current time,
-- lesson slots, group catalog, per-group schedules). Living on disk (rather
-- than in RAM) means a cache warmed before the host VM goes to sleep is still
-- warm on wake, instead of forcing a cold-start burst of re-fetches from the
-- Campus API — see docs/architecture/data-storage.md §5.
CREATE TABLE campus_cache (
    key        TEXT PRIMARY KEY,
    value      TEXT NOT NULL,
    fetched_at TIMESTAMP NOT NULL
);

-- Short-lived 6-digit pairing codes generated via Telegram bot /link command
-- for browser extension authentication and linking.
CREATE TABLE pairing_codes (
    code         TEXT PRIMARY KEY,
    telegram_id  INTEGER NOT NULL,
    expires_at   TIMESTAMP NOT NULL
);

-- Extension authentication tokens issued upon successful pairing verification,
-- allowing future sync requests without re-entering a pairing code.
CREATE TABLE user_tokens (
    token        TEXT PRIMARY KEY,
    user_id      TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at   TIMESTAMP NOT NULL
);

CREATE INDEX idx_user_tokens_user_id ON user_tokens (user_id);

-- +goose Down
DROP TABLE user_tokens;
DROP TABLE pairing_codes;
DROP TABLE campus_cache;
DROP TABLE user_lessons;
DROP TABLE user_schedule_state;
DROP TABLE users;

