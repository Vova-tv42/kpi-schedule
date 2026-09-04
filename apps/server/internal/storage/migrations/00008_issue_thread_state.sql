-- Widens the issue status vocabulary ('duplicate', 'rejected'), replaces the
-- thread_open boolean with a three-state thread lifecycle, and adds the
-- optional note an admin can attach to a status change.
--
-- The status CHECK constraint can only be changed by rebuilding the table, and
-- foreign keys are enabled on this connection (see storage.dsn), so DROP TABLE
-- issues would cascade-delete every comment. The comments are therefore parked
-- in an FK-free table first and restored last; everything runs inside goose's
-- transaction.

-- +goose Up
CREATE TABLE issue_comments_backup (
    id           TEXT NOT NULL,
    issue_id     TEXT NOT NULL,
    author_role  TEXT NOT NULL,
    author_label TEXT NOT NULL,
    body         TEXT NOT NULL,
    created_at   TIMESTAMP NOT NULL
);

INSERT INTO issue_comments_backup (id, issue_id, author_role, author_label, body, created_at)
SELECT id, issue_id, author_role, author_label, body, created_at FROM issue_comments;

DROP INDEX IF EXISTS idx_issue_comments_issue;
DROP TABLE issue_comments;

CREATE TABLE issues_new (
    id                 TEXT PRIMARY KEY,
    number             INTEGER NOT NULL UNIQUE,
    author_telegram_id INTEGER NOT NULL,
    author_username    TEXT NOT NULL DEFAULT '',
    author_first_name  TEXT NOT NULL DEFAULT '',
    type               TEXT NOT NULL CHECK (type IN ('feature', 'bug', 'other')),
    title              TEXT NOT NULL,
    body               TEXT NOT NULL,
    status             TEXT NOT NULL DEFAULT 'on_review'
        CHECK (status IN ('on_review', 'ready', 'in_development', 'implemented',
                          'duplicate', 'rejected', 'cancelled')),
    status_by          TEXT NOT NULL DEFAULT '',
    status_note        TEXT NOT NULL DEFAULT '',
    thread_state       TEXT NOT NULL DEFAULT 'none'
        CHECK (thread_state IN ('none', 'open', 'closed')),
    created_at         TIMESTAMP NOT NULL,
    updated_at         TIMESTAMP NOT NULL
);

INSERT INTO issues_new (id, number, author_telegram_id, author_username, author_first_name,
                        type, title, body, status, status_by, status_note, thread_state,
                        created_at, updated_at)
SELECT id, number, author_telegram_id, author_username, author_first_name,
       type, title, body, status, status_by, '',
       CASE WHEN thread_open = 1 THEN 'open' ELSE 'none' END,
       created_at, updated_at
FROM issues;

DROP INDEX IF EXISTS idx_issues_status;
DROP INDEX IF EXISTS idx_issues_author;
DROP TABLE issues;
ALTER TABLE issues_new RENAME TO issues;

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

INSERT INTO issue_comments (id, issue_id, author_role, author_label, body, created_at)
SELECT id, issue_id, author_role, author_label, body, created_at FROM issue_comments_backup;

DROP TABLE issue_comments_backup;

CREATE INDEX idx_issue_comments_issue ON issue_comments (issue_id, created_at);

-- +goose Down
-- Statuses added by this migration have no pre-00008 equivalent; both are
-- terminal rejections, so they fold into 'cancelled' rather than blocking the
-- rollback on a CHECK violation. status_note is dropped with the column.
CREATE TABLE issue_comments_backup (
    id           TEXT NOT NULL,
    issue_id     TEXT NOT NULL,
    author_role  TEXT NOT NULL,
    author_label TEXT NOT NULL,
    body         TEXT NOT NULL,
    created_at   TIMESTAMP NOT NULL
);

INSERT INTO issue_comments_backup (id, issue_id, author_role, author_label, body, created_at)
SELECT id, issue_id, author_role, author_label, body, created_at FROM issue_comments;

DROP INDEX IF EXISTS idx_issue_comments_issue;
DROP TABLE issue_comments;

CREATE TABLE issues_old (
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

INSERT INTO issues_old (id, number, author_telegram_id, author_username, author_first_name,
                        type, title, body, status, status_by, thread_open, created_at, updated_at)
SELECT id, number, author_telegram_id, author_username, author_first_name,
       type, title, body,
       CASE WHEN status IN ('duplicate', 'rejected') THEN 'cancelled' ELSE status END,
       status_by,
       CASE WHEN thread_state = 'none' THEN 0 ELSE 1 END,
       created_at, updated_at
FROM issues;

DROP INDEX IF EXISTS idx_issues_status;
DROP INDEX IF EXISTS idx_issues_author;
DROP TABLE issues;
ALTER TABLE issues_old RENAME TO issues;

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

INSERT INTO issue_comments (id, issue_id, author_role, author_label, body, created_at)
SELECT id, issue_id, author_role, author_label, body, created_at FROM issue_comments_backup;

DROP TABLE issue_comments_backup;

CREATE INDEX idx_issue_comments_issue ON issue_comments (issue_id, created_at);
