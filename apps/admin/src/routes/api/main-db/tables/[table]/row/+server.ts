import { json } from '@sveltejs/kit';
import { fetchMainServer } from '$lib/server/main-server';
import type { RequestHandler } from './$types';

export const PUT: RequestHandler = async ({ params, request, locals }) => {
	if (!locals.user) {
		return json({ error: 'Unauthorized' }, { status: 401 });
	}

	if (locals.user.role === 'read-only') {
		return json({ error: 'Forbidden: Read-only role cannot update database rows' }, { status: 403 });
	}

	const body = await request.json();
	const result = await fetchMainServer(`/api/v1/admin/tables/${params.table}/row`, locals.user, {
		method: 'PUT',
		body: JSON.stringify(body)
	});

	return json(result.data, { status: result.status });
};
