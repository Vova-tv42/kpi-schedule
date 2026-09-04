import { env } from '$env/dynamic/private';
import type { AuthUser } from './auth';

export async function fetchMainServer(path: string, user: AuthUser, options: RequestInit = {}) {
	const baseUrl = env.MAIN_SERVER_URL || process.env.MAIN_SERVER_URL || 'https://kpi-schedule.fly.dev';
	const adminSecret = env.MAIN_SERVER_ADMIN_SECRET || process.env.MAIN_SERVER_ADMIN_SECRET || env.ADMIN_API_SECRET || process.env.ADMIN_API_SECRET || '';

	const cleanBase = baseUrl.replace(/\/+$/, '');
	const cleanPath = path.startsWith('/') ? path : `/${path}`;
	const targetUrl = `${cleanBase}${cleanPath}`;

	const headers = new Headers(options.headers || {});
	headers.set('X-Admin-Secret', adminSecret);
	headers.set('X-Admin-Role', user.role);
	headers.set('X-Admin-Email', user.email);
	if (!headers.has('Content-Type') && options.body) {
		headers.set('Content-Type', 'application/json');
	}

	try {
		const response = await fetch(targetUrl, {
			...options,
			headers
		});

		const data = await response.json().catch(() => null);
		return {
			ok: response.ok,
			status: response.status,
			data
		};
	} catch (err: any) {
		return {
			ok: false,
			status: 502,
			data: {
				success: false,
				error: 'Failed to communicate with main server',
				detail: err?.message
			}
		};
	}
}
