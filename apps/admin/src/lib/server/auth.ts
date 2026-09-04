import * as arctic from 'arctic';
import { env } from '$env/dynamic/private';
import { ensureSchema, getSQL } from './db';

export type AdminRole = 'superadmin' | 'read-write' | 'read-only';

export interface AuthUser {
	email: string;
	role: AdminRole;
	isSuperadmin: boolean;
}

export function getGoogleOAuthClient(redirectUri: string) {
	const clientId = env.GOOGLE_CLIENT_ID || process.env.GOOGLE_CLIENT_ID;
	const clientSecret = env.GOOGLE_CLIENT_SECRET || process.env.GOOGLE_CLIENT_SECRET;

	if (!clientId || !clientSecret) {
		throw new Error('GOOGLE_CLIENT_ID and GOOGLE_CLIENT_SECRET are required');
	}

	return new arctic.Google(clientId, clientSecret, redirectUri);
}

export function getSuperadminEmail(): string {
	const email = process.env.SUPERADMIN_EMAIL || env.SUPERADMIN_EMAIL || '';
	return email.trim().toLowerCase();
}

/**
 * Checks whether an email address is authorized.
 * If unauthorized, returns null (does not store anything).
 */
export async function authorizeUser(email: string): Promise<AuthUser | null> {
	const normalized = email.trim().toLowerCase();
	const superadmin = getSuperadminEmail();

	if (superadmin && normalized === superadmin) {
		return {
			email: normalized,
			role: 'superadmin',
			isSuperadmin: true
		};
	}

	await ensureSchema();
	const sql = getSQL();
	const rows = await sql`
		SELECT role FROM admin_users
		WHERE LOWER(email) = ${normalized}
		LIMIT 1;
	`;

	if (rows.length === 0) {
		return null;
	}

	const role = rows[0].role as 'read-write' | 'read-only';
	return {
		email: normalized,
		role,
		isSuperadmin: false
	};
}

export async function createSession(user: AuthUser): Promise<string> {
	await ensureSchema();
	const sql = getSQL();
	const sessionId = crypto.randomUUID();
	const expiresAt = new Date(Date.now() + 7 * 24 * 60 * 60 * 1000); // 7 days

	await sql`
		INSERT INTO admin_sessions (id, email, role, expires_at)
		VALUES (${sessionId}, ${user.email}, ${user.role}, ${expiresAt.toISOString()});
	`;

	return sessionId;
}

export async function validateSession(sessionId: string): Promise<AuthUser | null> {
	if (!sessionId) return null;
	await ensureSchema();
	const sql = getSQL();

	const rows = await sql`
		SELECT id, email, role, expires_at
		FROM admin_sessions
		WHERE id = ${sessionId} AND expires_at > NOW()
		LIMIT 1;
	`;

	if (rows.length === 0) {
		return null;
	}

	const session = rows[0];
	const email = (session.email as string).toLowerCase();
	const superadmin = getSuperadminEmail();

	if (superadmin && email === superadmin) {
		return {
			email,
			role: 'superadmin',
			isSuperadmin: true
		};
	}

	// Verify the non-superadmin still exists in admin_users
	const userRows = await sql`
		SELECT role FROM admin_users
		WHERE LOWER(email) = ${email}
		LIMIT 1;
	`;

	if (userRows.length === 0) {
		// Admin was removed; revoke session immediately
		await sql`DELETE FROM admin_sessions WHERE id = ${sessionId};`;
		return null;
	}

	const activeRole = userRows[0].role as 'read-write' | 'read-only';
	return {
		email,
		role: activeRole,
		isSuperadmin: false
	};
}

export async function invalidateSession(sessionId: string): Promise<void> {
	if (!sessionId) return;
	await ensureSchema();
	const sql = getSQL();
	await sql`DELETE FROM admin_sessions WHERE id = ${sessionId};`;
}

export async function revokeAllSessionsForEmail(email: string): Promise<void> {
	if (!email) return;
	await ensureSchema();
	const sql = getSQL();
	await sql`DELETE FROM admin_sessions WHERE LOWER(email) = ${email.trim().toLowerCase()};`;
}
