import { neon } from '@neondatabase/serverless';
import postgres from 'postgres';
import { env } from '$env/dynamic/private';

let isInitialized = false;
let localSql: ReturnType<typeof postgres> | null = null;

export function getSQL() {
	const connectionString = env.DATABASE_URL || process.env.DATABASE_URL;
	if (!connectionString) {
		throw new Error('DATABASE_URL environment variable is missing');
	}

	const isLocal =
		connectionString.includes('localhost') ||
		connectionString.includes('127.0.0.1') ||
		!connectionString.includes('neon.tech');

	if (isLocal) {
		if (!localSql) {
			localSql = postgres(connectionString);
		}
		return localSql;
	}

	return neon(connectionString);
}

export async function ensureSchema() {
	if (isInitialized) return;
	const sql = getSQL();

	await sql`
		CREATE TABLE IF NOT EXISTS admin_users (
			id TEXT PRIMARY KEY,
			email TEXT UNIQUE NOT NULL,
			role TEXT NOT NULL CHECK (role IN ('read-only', 'read-write')),
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			created_by TEXT NOT NULL
		);
	`;

	await sql`
		CREATE TABLE IF NOT EXISTS admin_sessions (
			id TEXT PRIMARY KEY,
			email TEXT NOT NULL,
			role TEXT NOT NULL,
			expires_at TIMESTAMPTZ NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
	`;

	await sql`
		CREATE INDEX IF NOT EXISTS idx_admin_sessions_email ON admin_sessions (email);
	`;
	await sql`
		CREATE INDEX IF NOT EXISTS idx_admin_sessions_expires ON admin_sessions (expires_at);
	`;

	await sql`
		CREATE TABLE IF NOT EXISTS recent_actions (
			id TEXT PRIMARY KEY,
			action_type TEXT NOT NULL,
			action_name TEXT NOT NULL,
			status_code INTEGER NOT NULL,
			duration_ms BIGINT NOT NULL DEFAULT 0,
			metadata JSONB,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
	`;

	await sql`
		CREATE INDEX IF NOT EXISTS idx_recent_actions_created ON recent_actions (created_at DESC);
	`;

	await sql`
		CREATE TABLE IF NOT EXISTS admin_settings (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL
		);
	`;

	await sql`
		INSERT INTO admin_settings (key, value)
		VALUES ('retention_hours', '72')
		ON CONFLICT (key) DO NOTHING;
	`;

	isInitialized = true;
}
