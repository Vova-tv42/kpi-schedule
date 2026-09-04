import { json } from '@sveltejs/kit';
import { fetchMainServer } from '$lib/server/main-server';
import type { RequestHandler } from './$types';

export const GET: RequestHandler = async ({ locals }) => {
	if (!locals.user) {
		return json({ error: 'Unauthorized' }, { status: 401 });
	}

	const result = await fetchMainServer('/api/v1/admin/tables', locals.user);
	return json(result.data, { status: result.status });
};
