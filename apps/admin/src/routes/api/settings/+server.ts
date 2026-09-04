import { json } from '@sveltejs/kit';
import { ensureSchema, getSQL } from '$lib/server/db';
import type { RequestHandler } from './$types';

export const GET: RequestHandler = async ({ locals }) => {
	if (!locals.user) {
		return json({ error: 'Unauthorized' }, { status: 401 });
	}

	await ensureSchema();
	const sql = getSQL();

	const rows = await sql`
		SELECT key, value FROM admin_settings WHERE key = 'retention_hours' LIMIT 1;
	`;

	const hours = parseInt(rows[0]?.value || '72', 10);
	return json({
		retention_hours: isNaN(hours) ? 72 : hours
	});
};

export const POST: RequestHandler = async ({ request, locals }) => {
	if (!locals.user) {
		return json({ error: 'Unauthorized' }, { status: 401 });
	}

	if (locals.user.role === 'read-only') {
		return json({ error: 'Forbidden: Read-only role cannot update system settings' }, { status: 403 });
	}

	const body = await request.json();
	const hours = parseInt(body.retention_hours, 10);

	if (isNaN(hours) || hours < 1 || hours > 720) {
		return json({ error: 'Retention hours must be between 1 and 720 (30 days)' }, { status: 400 });
	}

	await ensureSchema();
	const sql = getSQL();

	await sql`
		INSERT INTO admin_settings (key, value)
		VALUES ('retention_hours', ${String(hours)})
		ON CONFLICT (key) DO UPDATE SET value = ${String(hours)};
	`;

	return json({ success: true, retention_hours: hours });
};
