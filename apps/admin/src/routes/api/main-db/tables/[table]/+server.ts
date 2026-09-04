import { json } from '@sveltejs/kit';
import { fetchMainServer } from '$lib/server/main-server';
import type { RequestHandler } from './$types';

export const GET: RequestHandler = async ({ params, url, locals }) => {
	if (!locals.user) {
		return json({ error: 'Unauthorized' }, { status: 401 });
	}

	const queryString = url.search;
	const result = await fetchMainServer(`/api/v1/admin/tables/${params.table}${queryString}`, locals.user);
	return json(result.data, { status: result.status });
};
