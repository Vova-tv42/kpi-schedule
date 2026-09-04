import { redirect } from '@sveltejs/kit';
import { getGoogleOAuthClient, authorizeUser, createSession } from '$lib/server/auth';
import type { RequestHandler } from './$types';

export const GET: RequestHandler = async (event) => {
	const code = event.url.searchParams.get('code');
	const state = event.url.searchParams.get('state');

	const storedState = event.cookies.get('google_oauth_state');
	const storedVerifier = event.cookies.get('google_code_verifier');

	// Clear temporary auth cookies
	event.cookies.delete('google_oauth_state', { path: '/' });
	event.cookies.delete('google_code_verifier', { path: '/' });

	if (!code || !state || !storedState || !storedVerifier || state !== storedState) {
		throw redirect(303, '/login?error=invalid_state');
	}

	const redirectUri = `${event.url.origin}/auth/google/callback`;
	const google = getGoogleOAuthClient(redirectUri);

	let tokens;
	try {
		tokens = await google.validateAuthorizationCode(code, storedVerifier);
	} catch (err) {
		console.error('Failed to validate authorization code:', err);
		throw redirect(303, '/login?error=token_exchange_failed');
	}

	const accessToken = tokens.accessToken();
	const userinfoResponse = await fetch('https://openidconnect.googleapis.com/v1/userinfo', {
		headers: {
			Authorization: `Bearer ${accessToken}`
		}
	});

	if (!userinfoResponse.ok) {
		throw redirect(303, '/login?error=userinfo_failed');
	}

	const profile = await userinfoResponse.json();
	const email = profile.email;
	if (!email) {
		throw redirect(303, '/login?error=no_email');
	}
	if (!profile.email_verified) {
		throw redirect(303, '/login?error=unverified_email');
	}

	// Strict Whitelist Check
	const authorizedUser = await authorizeUser(email);
	if (!authorizedUser) {
		// Strict rejection: Do not create a session and do not store anything!
		throw redirect(303, `/login?error=forbidden&email=${encodeURIComponent(email)}`);
	}

	// Create session and set cookie
	const sessionId = await createSession(authorizedUser);
	event.cookies.set('admin_session_token', sessionId, {
		path: '/',
		httpOnly: true,
		secure: process.env.NODE_ENV === 'production',
		sameSite: 'lax',
		maxAge: 7 * 24 * 60 * 60 // 7 days
	});

	throw redirect(303, '/actions');
};
