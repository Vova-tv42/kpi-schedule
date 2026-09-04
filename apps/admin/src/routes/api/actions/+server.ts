import { json } from '@sveltejs/kit';
import { ensureSchema, getSQL } from '$lib/server/db';
import type { RequestHandler } from './$types';

export const GET: RequestHandler = async ({ url, locals }) => {
	if (!locals.user) {
		return json({ error: 'Unauthorized' }, { status: 401 });
	}

	await ensureSchema();
	const sql = getSQL();

	const limit = Math.min(Math.max(parseInt(url.searchParams.get('limit') || '50', 10), 1), 200);
	const offset = Math.max(parseInt(url.searchParams.get('offset') || '0', 10), 0);
	const actionType = url.searchParams.get('action_type') || '';
	const status = url.searchParams.get('status') ? parseInt(url.searchParams.get('status')!, 10) : null;
	const search = url.searchParams.get('search')?.trim() || '';

	let rows;
	let countResult;

	// Build query with conditional filters safely
	if (actionType && status !== null && search) {
		rows = await sql`
			SELECT id, action_type, action_name, status_code, duration_ms, metadata, created_at
			FROM recent_actions
			WHERE action_type = ${actionType} AND status_code = ${status} AND action_name ILIKE ${'%' + search + '%'}
			ORDER BY created_at DESC
			LIMIT ${limit} OFFSET ${offset};
		`;
		countResult = await sql`
			SELECT COUNT(*) as total FROM recent_actions
			WHERE action_type = ${actionType} AND status_code = ${status} AND action_name ILIKE ${'%' + search + '%'};
		`;
	} else if (actionType && status !== null) {
		rows = await sql`
			SELECT id, action_type, action_name, status_code, duration_ms, metadata, created_at
			FROM recent_actions
			WHERE action_type = ${actionType} AND status_code = ${status}
			ORDER BY created_at DESC
			LIMIT ${limit} OFFSET ${offset};
		`;
		countResult = await sql`
			SELECT COUNT(*) as total FROM recent_actions
			WHERE action_type = ${actionType} AND status_code = ${status};
		`;
	} else if (actionType && search) {
		rows = await sql`
			SELECT id, action_type, action_name, status_code, duration_ms, metadata, created_at
			FROM recent_actions
			WHERE action_type = ${actionType} AND action_name ILIKE ${'%' + search + '%'}
			ORDER BY created_at DESC
			LIMIT ${limit} OFFSET ${offset};
		`;
		countResult = await sql`
			SELECT COUNT(*) as total FROM recent_actions
			WHERE action_type = ${actionType} AND action_name ILIKE ${'%' + search + '%'};
		`;
	} else if (status !== null && search) {
		rows = await sql`
			SELECT id, action_type, action_name, status_code, duration_ms, metadata, created_at
			FROM recent_actions
			WHERE status_code = ${status} AND action_name ILIKE ${'%' + search + '%'}
			ORDER BY created_at DESC
			LIMIT ${limit} OFFSET ${offset};
		`;
		countResult = await sql`
			SELECT COUNT(*) as total FROM recent_actions
			WHERE status_code = ${status} AND action_name ILIKE ${'%' + search + '%'};
		`;
	} else if (actionType) {
		rows = await sql`
			SELECT id, action_type, action_name, status_code, duration_ms, metadata, created_at
			FROM recent_actions
			WHERE action_type = ${actionType}
			ORDER BY created_at DESC
			LIMIT ${limit} OFFSET ${offset};
		`;
		countResult = await sql`
			SELECT COUNT(*) as total FROM recent_actions WHERE action_type = ${actionType};
		`;
	} else if (status !== null) {
		rows = await sql`
			SELECT id, action_type, action_name, status_code, duration_ms, metadata, created_at
			FROM recent_actions
			WHERE status_code = ${status}
			ORDER BY created_at DESC
			LIMIT ${limit} OFFSET ${offset};
		`;
		countResult = await sql`
			SELECT COUNT(*) as total FROM recent_actions WHERE status_code = ${status};
		`;
	} else if (search) {
		rows = await sql`
			SELECT id, action_type, action_name, status_code, duration_ms, metadata, created_at
			FROM recent_actions
			WHERE action_name ILIKE ${'%' + search + '%'}
			ORDER BY created_at DESC
			LIMIT ${limit} OFFSET ${offset};
		`;
		countResult = await sql`
			SELECT COUNT(*) as total FROM recent_actions WHERE action_name ILIKE ${'%' + search + '%'};
		`;
	} else {
		rows = await sql`
			SELECT id, action_type, action_name, status_code, duration_ms, metadata, created_at
			FROM recent_actions
			ORDER BY created_at DESC
			LIMIT ${limit} OFFSET ${offset};
		`;
		countResult = await sql`SELECT COUNT(*) as total FROM recent_actions;`;
	}

	const total = parseInt(countResult[0]?.total || '0', 10);

	return json({
		actions: rows,
		total,
		limit,
		offset
	});
};
