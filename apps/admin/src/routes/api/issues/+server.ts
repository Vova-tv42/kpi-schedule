import { json } from '@sveltejs/kit';
import { fetchMainServer } from '$lib/server/main-server';
import type { RequestHandler } from './$types';

// Proxies the issue queue from the Go server. Readable by every admin role —
// only the mutations in ./[id]/+server.ts are gated.
export const GET: RequestHandler = async ({ url, locals }) => {
	if (!locals.user) {
		return json({ error: 'Unauthorized' }, { status: 401 });
	}

	const query = new URLSearchParams();
	for (const key of ['status', 'type', 'q', 'limit', 'offset']) {
		const value = url.searchParams.get(key);
		if (value) query.set(key, value);
	}

	const suffix = query.toString() ? `?${query.toString()}` : '';
	const result = await fetchMainServer(`/api/v1/admin/issues${suffix}`, locals.user);

	return json(result.data, { status: result.status });
};
