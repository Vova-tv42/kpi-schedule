import { redirect } from '@sveltejs/kit';
import { invalidateSession } from '$lib/server/auth';
import type { RequestHandler } from './$types';

export const POST: RequestHandler = async (event) => {
	const sessionId = event.cookies.get('admin_session_token');
	if (sessionId) {
		await invalidateSession(sessionId);
		event.cookies.delete('admin_session_token', { path: '/' });
	}
	throw redirect(303, '/login');
};

export const GET: RequestHandler = async (event) => {
	const sessionId = event.cookies.get('admin_session_token');
	if (sessionId) {
		await invalidateSession(sessionId);
		event.cookies.delete('admin_session_token', { path: '/' });
	}
	throw redirect(303, '/login');
};
