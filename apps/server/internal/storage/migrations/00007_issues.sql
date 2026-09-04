-- +goose Up
CREATE TABLE issues (
    id                 TEXT PRIMARY KEY,
    number             INTEGER NOT NULL UNIQUE,
    author_telegram_id INTEGER NOT NULL,
    author_username    TEXT NOT NULL DEFAULT '',
    author_first_name  TEXT NOT NULL DEFAULT '',
    type               TEXT NOT NULL CHECK (type IN ('feature', 'bug', 'other')),
    title              TEXT NOT NULL,
    body               TEXT NOT NULL,
    status             TEXT NOT NULL DEFAULT 'on_review'
        CHECK (status IN ('on_review', 'ready', 'in_development', 'implemented', 'cancelled')),
    status_by          TEXT NOT NULL DEFAULT '',
    thread_open        BOOLEAN NOT NULL DEFAULT 0,
    created_at         TIMESTAMP NOT NULL,
    updated_at         TIMESTAMP NOT NULL
);

CREATE INDEX idx_issues_author ON issues (author_telegram_id, created_at);
CREATE INDEX idx_issues_status ON issues (status, created_at);

CREATE TABLE issue_comments (
    id           TEXT PRIMARY KEY,
    issue_id     TEXT NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
    author_role  TEXT NOT NULL CHECK (author_role IN ('user', 'admin')),
    author_label TEXT NOT NULL DEFAULT '',
    body         TEXT NOT NULL,
    created_at   TIMESTAMP NOT NULL
);

CREATE INDEX idx_issue_comments_issue ON issue_comments (issue_id, created_at);

CREATE TABLE user_issue_drafts (
    telegram_id       INTEGER PRIMARY KEY,
    chat_id           INTEGER NOT NULL,
    prompt_message_id INTEGER NOT NULL,
    step              TEXT NOT NULL,
    issue_type        TEXT NOT NULL DEFAULT '',
    title             TEXT NOT NULL DEFAULT '',
    issue_id          TEXT,
    expires_at        TIMESTAMP NOT NULL,
    updated_at        TIMESTAMP NOT NULL
);

CREATE INDEX idx_user_issue_drafts_expiry ON user_issue_drafts (expires_at);

-- +goose Down
DROP INDEX IF EXISTS idx_user_issue_drafts_expiry;
DROP TABLE IF EXISTS user_issue_drafts;
DROP INDEX IF EXISTS idx_issue_comments_issue;
DROP TABLE IF EXISTS issue_comments;
DROP INDEX IF EXISTS idx_issues_status;
DROP INDEX IF EXISTS idx_issues_author;
DROP TABLE IF EXISTS issues;
