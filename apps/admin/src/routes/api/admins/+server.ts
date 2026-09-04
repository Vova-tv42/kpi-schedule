import { json } from '@sveltejs/kit';
import { ensureSchema, getSQL } from '$lib/server/db';
import { getSuperadminEmail, revokeAllSessionsForEmail } from '$lib/server/auth';
import type { RequestHandler } from './$types';

export const GET: RequestHandler = async ({ locals }) => {
	if (!locals.user?.isSuperadmin) {
		return json({ error: 'Forbidden: Superadmin access required' }, { status: 403 });
	}

	await ensureSchema();
	const sql = getSQL();

	const rows = await sql`
		SELECT id, email, role, created_at, created_by
		FROM admin_users
		ORDER BY created_at DESC;
	`;

	const superadminEmail = getSuperadminEmail();

	return json({
		superadmin: superadminEmail,
		admins: rows
	});
};

export const POST: RequestHandler = async ({ request, locals }) => {
	if (!locals.user?.isSuperadmin) {
		return json({ error: 'Forbidden: Superadmin access required' }, { status: 403 });
	}

	const body = await request.json();
	const email = (body.email as string)?.trim().toLowerCase();
	const role = body.role as string;

	if (!email || !email.includes('@')) {
		return json({ error: 'Valid email address is required' }, { status: 400 });
	}

	if (role !== 'read-only' && role !== 'read-write') {
		return json({ error: 'Role must be either read-only or read-write' }, { status: 400 });
	}

	const superadminEmail = getSuperadminEmail();
	if (email === superadminEmail) {
		return json({ error: 'Cannot add superadmin as a secondary admin' }, { status: 400 });
	}

	await ensureSchema();
	const sql = getSQL();
	const id = crypto.randomUUID();

	try {
		await sql`
			INSERT INTO admin_users (id, email, role, created_by, created_at)
			VALUES (${id}, ${email}, ${role}, ${locals.user.email}, NOW());
		`;
		return json({ success: true, id, email, role });
	} catch (err: any) {
		if (err?.message?.includes('unique') || err?.code === '23505') {
			return json({ error: 'Admin with this email already exists' }, { status: 409 });
		}
		return json({ error: 'Failed to create admin user', detail: err?.message }, { status: 500 });
	}
};

export const DELETE: RequestHandler = async ({ url, locals }) => {
	if (!locals.user?.isSuperadmin) {
		return json({ error: 'Forbidden: Superadmin access required' }, { status: 403 });
	}

	const id = url.searchParams.get('id');
	const email = url.searchParams.get('email');

	if (!id && !email) {
		return json({ error: 'Admin id or email is required' }, { status: 400 });
	}

	await ensureSchema();
	const sql = getSQL();

	let targetEmail = email;
	if (id && !targetEmail) {
		const row = await sql`SELECT email FROM admin_users WHERE id = ${id} LIMIT 1;`;
		if (row.length > 0) {
			targetEmail = row[0].email as string;
		}
	}

	if (id) {
		await sql`DELETE FROM admin_users WHERE id = ${id};`;
	} else if (email) {
		await sql`DELETE FROM admin_users WHERE LOWER(email) = ${email.trim().toLowerCase()};`;
	}

	if (targetEmail) {
		await revokeAllSessionsForEmail(targetEmail);
	}

	return json({ success: true, deleted: id || email });
};

export const PATCH: RequestHandler = async ({ request, locals }) => {
	if (!locals.user?.isSuperadmin) {
		return json({ error: 'Forbidden: Superadmin access required' }, { status: 403 });
	}

	const body = await request.json();
	const { id, role } = body;

	if (!id || (role !== 'read-only' && role !== 'read-write')) {
		return json({ error: 'Valid id and role (read-only or read-write) are required' }, { status: 400 });
	}

	await ensureSchema();
	const sql = getSQL();

	const row = await sql`SELECT email FROM admin_users WHERE id = ${id} LIMIT 1;`;
	if (row.length === 0) {
		return json({ error: 'Admin user not found' }, { status: 404 });
	}

	await sql`UPDATE admin_users SET role = ${role} WHERE id = ${id};`;
	// Revoke existing sessions so the updated role takes effect on next login
	await revokeAllSessionsForEmail(row[0].email as string);

	return json({ success: true, id, role });
};
