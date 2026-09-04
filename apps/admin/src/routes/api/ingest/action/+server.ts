import { json } from '@sveltejs/kit';
import { env } from '$env/dynamic/private';
import { ensureSchema, getSQL } from '$lib/server/db';
import type { RequestHandler } from './$types';

// Strict field sanitizer to guarantee complete anonymity
function sanitizeMetadata(raw: any): Record<string, any> | null {
	if (!raw || typeof raw !== 'object') return null;
	const clean: Record<string, any> = {};
	const forbiddenKeys = ['user_id', 'telegram_id', 'chat_id', 'id', 'name', 'username', 'token', 'phone', 'email'];

	for (const [k, v] of Object.entries(raw)) {
		const lowerKey = k.toLowerCase();
		if (forbiddenKeys.includes(lowerKey) || lowerKey.includes('token') || lowerKey.includes('id')) {
			continue;
		}
		if (v && typeof v === 'object' && !Array.isArray(v)) {
			clean[k] = sanitizeMetadata(v);
		} else {
			clean[k] = v;
		}
	}
	return clean;
}

export const POST: RequestHandler = async ({ request }) => {
	const expectedKey = env.ADMIN_INGEST_KEY || process.env.ADMIN_INGEST_KEY;
	const providedKey = request.headers.get('X-Ingest-Key') || request.headers.get('Authorization')?.replace('Bearer ', '');

	if (!expectedKey || providedKey !== expectedKey) {
		return json({ success: false, error: 'Unauthorized: Invalid ingest key' }, { status: 401 });
	}

	let body;
	try {
		body = await request.json();
	} catch {
		return json({ success: false, error: 'Invalid JSON payload' }, { status: 400 });
	}

	const { action_type, action_name, status_code, duration_ms, metadata } = body;
	if (!action_type || !action_name) {
		return json({ success: false, error: 'action_type and action_name are required' }, { status: 400 });
	}

	await ensureSchema();
	const sql = getSQL();
	const id = crypto.randomUUID();
	const sanitized = sanitizeMetadata(metadata);
	const statusCode = typeof status_code === 'number' ? status_code : 200;
	const duration = typeof duration_ms === 'number' ? duration_ms : 0;

	await sql`
		INSERT INTO recent_actions (id, action_type, action_name, status_code, duration_ms, metadata, created_at)
		VALUES (
			${id},
			${String(action_type)},
			${String(action_name)},
			${statusCode},
			${duration},
			${sanitized ? JSON.stringify(sanitized) : null},
			NOW()
		);
	`;

	// Opportunistic cleanup: 5% chance to prune expired actions without waiting for cron
	if (Math.random() < 0.05) {
		try {
			const setting = await sql`SELECT value FROM admin_settings WHERE key = 'retention_hours' LIMIT 1;`;
			const hours = parseInt(setting[0]?.value || '72', 10);
			if (!isNaN(hours) && hours > 0) {
				await sql`
					DELETE FROM recent_actions 
					WHERE created_at < NOW() - (${hours} * INTERVAL '1 hour');
				`;
			}
		} catch (cleanupErr) {
			console.warn('Opportunistic cleanup failed:', cleanupErr);
		}
	}

	return json({ success: true, id });
};
