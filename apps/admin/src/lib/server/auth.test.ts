import { describe, it, expect, beforeEach, vi } from 'vitest';
import { getSuperadminEmail, authorizeUser } from './auth';

describe('Admin Authentication & Whitelist Verification', () => {
	beforeEach(() => {
		vi.resetModules();
		process.env.SUPERADMIN_EMAIL = 'root-admin@kpi.ua';
	});

	it('identifies superadmin email correctly from environment', () => {
		expect(getSuperadminEmail()).toBe('root-admin@kpi.ua');
	});

	it('normalizes email with whitespace and uppercase', () => {
		process.env.SUPERADMIN_EMAIL = '  ROOT-Admin@KPI.UA  ';
		expect(getSuperadminEmail()).toBe('root-admin@kpi.ua');
	});

	it('authorizes superadmin without requiring database lookup', async () => {
		process.env.SUPERADMIN_EMAIL = 'owner@kpi.ua';
		const auth = await authorizeUser('owner@kpi.ua');
		expect(auth).not.toBeNull();
		expect(auth?.role).toBe('superadmin');
		expect(auth?.isSuperadmin).toBe(true);
		expect(auth?.email).toBe('owner@kpi.ua');
	});
});
