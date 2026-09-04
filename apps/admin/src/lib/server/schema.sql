-- NeonDB PostgreSQL Schema for KPI Schedule Admin Dashboard

CREATE TABLE IF NOT EXISTS admin_users (
    id TEXT PRIMARY KEY,
    email TEXT UNIQUE NOT NULL,
    role TEXT NOT NULL CHECK (role IN ('read-only', 'read-write')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS admin_sessions (
    id TEXT PRIMARY KEY,
    email TEXT NOT NULL,
    role TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_admin_sessions_email ON admin_sessions (email);
CREATE INDEX IF NOT EXISTS idx_admin_sessions_expires ON admin_sessions (expires_at);

CREATE TABLE IF NOT EXISTS recent_actions (
    id TEXT PRIMARY KEY,
    action_type TEXT NOT NULL,
    action_name TEXT NOT NULL,
    status_code INTEGER NOT NULL,
    duration_ms BIGINT NOT NULL DEFAULT 0,
    metadata JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_recent_actions_created ON recent_actions (created_at DESC);
CREATE INDEX IF NOT EXISTS idx_recent_actions_type ON recent_actions (action_type);

CREATE TABLE IF NOT EXISTS admin_settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

INSERT INTO admin_settings (key, value)
VALUES ('retention_hours', '72')
ON CONFLICT (key) DO NOTHING;
