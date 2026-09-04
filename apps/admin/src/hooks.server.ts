import { redirect, json, type Handle } from '@sveltejs/kit';
import { validateSession } from '$lib/server/auth';

const PUBLIC_PATHS = [
	'/login',
	'/auth/google',
	'/auth/google/callback',
	'/api/ingest/action',
	'/api/cron/cleanup'
];

export const handle: Handle = async ({ event, resolve }) => {
	const sessionId = event.cookies.get('admin_session_token');
	let user = null;

	if (sessionId) {
		try {
			user = await validateSession(sessionId);
		} catch (err) {
			console.error('Session validation error:', err);
		}
	}

	event.locals.user = user;

	const { pathname } = event.url;
	const isPublic = PUBLIC_PATHS.some((p) => pathname === p || pathname.startsWith(p + '/'));

	if (!isPublic && !user) {
		if (pathname.startsWith('/api/')) {
			return json({ success: false, error: 'Unauthorized: Admin authentication required' }, { status: 401 });
		}
		throw redirect(303, '/login');
	}

	// Superadmin-only route guard
	if ((pathname.startsWith('/admins') || pathname.startsWith('/api/admins')) && (!user || !user.isSuperadmin)) {
		if (pathname.startsWith('/api/')) {
			return json({ success: false, error: 'Forbidden: Superadmin access required' }, { status: 403 });
		}
		throw redirect(303, '/actions');
	}

	// Read-only route guard for custom query console
	if (pathname.startsWith('/database/query') && user?.role === 'read-only') {
		throw redirect(303, '/database');
	}

	return resolve(event);
};
