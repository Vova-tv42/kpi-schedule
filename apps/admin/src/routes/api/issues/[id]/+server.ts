import { json } from '@sveltejs/kit';
import { fetchMainServer } from '$lib/server/main-server';
import type { RequestHandler } from './$types';

export const GET: RequestHandler = async ({ params, locals }) => {
	if (!locals.user) {
		return json({ error: 'Unauthorized' }, { status: 401 });
	}

	const result = await fetchMainServer(`/api/v1/admin/issues/${params.id}`, locals.user);
	return json(result.data, { status: result.status });
};

// Status change. The Go server enforces the same rule via
// adminWritePermissionMiddleware; this guard just fails fast and gives the
// dashboard a message in its own vocabulary.
export const PATCH: RequestHandler = async ({ params, request, locals }) => {
	if (!locals.user) {
		return json({ error: 'Unauthorized' }, { status: 401 });
	}

	if (locals.user.role === 'read-only') {
		return json({ error: 'Forbidden: Read-only role cannot change issue status' }, { status: 403 });
	}

	const body = await request.json();
	const result = await fetchMainServer(`/api/v1/admin/issues/${params.id}/status`, locals.user, {
		method: 'PATCH',
		body: JSON.stringify(body)
	});

	return json(result.data, { status: result.status });
};

// Posting a comment opens the discussion thread for the reporter and DMs them,
// so it is a write even though nothing about the issue itself changes.
export const POST: RequestHandler = async ({ params, request, locals }) => {
	if (!locals.user) {
		return json({ error: 'Unauthorized' }, { status: 401 });
	}

	if (locals.user.role === 'read-only') {
		return json({ error: 'Forbidden: Read-only role cannot reply to issues' }, { status: 403 });
	}

	const body = await request.json();
	const result = await fetchMainServer(`/api/v1/admin/issues/${params.id}/comments`, locals.user, {
		method: 'POST',
		body: JSON.stringify(body)
	});

	return json(result.data, { status: result.status });
};

// Deletes the issue and its whole discussion. Irreversible, so the page asks
// for confirmation before calling this.
export const DELETE: RequestHandler = async ({ params, locals }) => {
	if (!locals.user) {
		return json({ error: 'Unauthorized' }, { status: 401 });
	}

	if (locals.user.role === 'read-only') {
		return json({ error: 'Forbidden: Read-only role cannot delete issues' }, { status: 403 });
	}

	const result = await fetchMainServer(`/api/v1/admin/issues/${params.id}`, locals.user, {
		method: 'DELETE'
	});

	return json(result.data, { status: result.status });
};
