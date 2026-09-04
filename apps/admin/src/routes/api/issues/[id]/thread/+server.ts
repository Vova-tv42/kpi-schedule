import { json } from '@sveltejs/kit';
import { fetchMainServer } from '$lib/server/main-server';
import type { RequestHandler } from './$types';

// Closes or reopens a discussion. Closing leaves the transcript readable to the
// reporter but stops them replying; the main server notifies them either way.
export const PATCH: RequestHandler = async ({ params, request, locals }) => {
	if (!locals.user) {
		return json({ error: 'Unauthorized' }, { status: 401 });
	}

	if (locals.user.role === 'read-only') {
		return json({ error: 'Forbidden: Read-only role cannot change the discussion state' }, { status: 403 });
	}

	const body = await request.json();
	const result = await fetchMainServer(`/api/v1/admin/issues/${params.id}/thread`, locals.user, {
		method: 'PATCH',
		body: JSON.stringify(body)
	});

	return json(result.data, { status: result.status });
};
