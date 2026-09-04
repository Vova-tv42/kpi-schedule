import { redirect } from '@sveltejs/kit';
import * as arctic from 'arctic';
import { getGoogleOAuthClient } from '$lib/server/auth';
import type { RequestHandler } from './$types';

export const GET: RequestHandler = async (event) => {
	const redirectUri = `${event.url.origin}/auth/google/callback`;
	const google = getGoogleOAuthClient(redirectUri);

	const state = arctic.generateState();
	const codeVerifier = arctic.generateCodeVerifier();

	const scopes = ['openid', 'email', 'profile'];
	const url = google.createAuthorizationURL(state, codeVerifier, scopes);

	event.cookies.set('google_oauth_state', state, {
		path: '/',
		httpOnly: true,
		secure: process.env.NODE_ENV === 'production',
		maxAge: 60 * 10, // 10 minutes
		sameSite: 'lax'
	});

	event.cookies.set('google_code_verifier', codeVerifier, {
		path: '/',
		httpOnly: true,
		secure: process.env.NODE_ENV === 'production',
		maxAge: 60 * 10, // 10 minutes
		sameSite: 'lax'
	});

	throw redirect(302, url.toString());
};
