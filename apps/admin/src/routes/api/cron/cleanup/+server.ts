import { json } from '@sveltejs/kit';
import { env } from '$env/dynamic/private';
import { ensureSchema, getSQL } from '$lib/server/db';
import type { RequestHandler } from './$types';

export const POST: RequestHandler = async (event) => {
	return handleCleanup(event);
};

export const GET: RequestHandler = async (event) => {
	return handleCleanup(event);
};

async function handleCleanup({ request, url, locals }: { request: Request; url: URL; locals: App.Locals }) {
	const expectedSecret = env.CRON_SECRET || process.env.CRON_SECRET;
	const authHeader = request.headers.get('Authorization');
	const secretHeader = request.headers.get('X-Cron-Secret');
	const querySecret = url.searchParams.get('secret');

	const providedSecret =
		authHeader?.replace('Bearer ', '') ||
		secretHeader ||
		querySecret;

	// Allow if user is an authenticated admin OR provided the valid CRON_SECRET
	const isAuthorizedCron = expectedSecret && providedSecret === expectedSecret;
	const isAuthorizedAdmin = !!locals.user;

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
		WHERE created_at < NOW() - (${retentionHours} || ' hours')::INTERVAL;
	`;

	return json({
		success: true,
		retention_hours: retentionHours,
		deleted_count: (deleteResult as any).count || 0,
		timestamp: new Date().toISOString()
	});
}
