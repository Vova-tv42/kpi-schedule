import { json } from '@sveltejs/kit';
import crypto from 'node:crypto';
import { env } from '$env/dynamic/private';
import { ensureSchema, getSQL } from '$lib/server/db';
import type { RequestHandler } from './$types';

export const POST: RequestHandler = async (event) => {
	return handleCleanup(event, false);
};

export const GET: RequestHandler = async (event) => {
	return handleCleanup(event, true);
};

async function handleCleanup({ request, locals }: { request: Request; locals: App.Locals }, isGet: boolean) {
	const expectedSecret = env.CRON_SECRET || process.env.CRON_SECRET;
	const authHeader = request.headers.get('Authorization');
	const secretHeader = request.headers.get('X-Cron-Secret');

	const providedSecret =
		authHeader?.replace('Bearer ', '') ||
		secretHeader;

	let isAuthorizedCron = false;
	if (expectedSecret && providedSecret) {
		const hashA = crypto.createHash('sha256').update(providedSecret).digest();
		const hashB = crypto.createHash('sha256').update(expectedSecret).digest();
		isAuthorizedCron = crypto.timingSafeEqual(hashA, hashB);
	}

	if (!isGet && locals.user && locals.user.role === 'read-only') {
		return json({ success: false, error: 'Forbidden: Read-only admins cannot trigger manual cleanup' }, { status: 403 });
	}

	const isAuthorizedAdmin = !isGet && !!locals.user && locals.user.role !== 'read-only';

	if (!isAuthorizedCron && !isAuthorizedAdmin) {
		return json({ success: false, error: 'Unauthorized: Invalid cron credentials' }, { status: 401 });
	}

	await ensureSchema();
	const sql = getSQL();

	// Fetch retention window in hours
	const rows = await sql`
		SELECT value FROM admin_settings WHERE key = 'retention_hours' LIMIT 1;
	`;
	const hours = parseInt(rows[0]?.value || '72', 10);
	const retentionHours = isNaN(hours) || hours <= 0 ? 72 : hours;

	const deleteResult = await sql`
		DELETE FROM recent_actions
		WHERE created_at < NOW() - (${retentionHours} * INTERVAL '1 hour')
		RETURNING id;
	`;

	const deletedCount = (deleteResult as any).count ?? deleteResult.length ?? 0;

	return json({
		success: true,
		retention_hours: retentionHours,
		deleted_count: deletedCount,
		timestamp: new Date().toISOString()
	});
}
