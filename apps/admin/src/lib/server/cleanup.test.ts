import { describe, it, expect } from 'vitest';

export function calculateRetentionCutoff(retentionHours: number, referenceDate: Date = new Date()): Date {
	const validHours = isNaN(retentionHours) || retentionHours <= 0 ? 72 : Math.min(retentionHours, 720);
	return new Date(referenceDate.getTime() - validHours * 60 * 60 * 1000);
}

describe('Action Retention and Cleanup Logic', () => {
	it('defaults to 72 hours when retention hours is 0 or negative', () => {
		const now = new Date('2026-09-04T12:00:00Z');
		const cutoff = calculateRetentionCutoff(0, now);
		const expectedDiffMs = 72 * 60 * 60 * 1000;
		expect(now.getTime() - cutoff.getTime()).toBe(expectedDiffMs);
	});

	it('computes exact cutoff date for valid retention period', () => {
		const now = new Date('2026-09-04T12:00:00Z');
		const cutoff = calculateRetentionCutoff(24, now);
		const expectedDiffMs = 24 * 60 * 60 * 1000;
		expect(now.getTime() - cutoff.getTime()).toBe(expectedDiffMs);
	});

	it('caps maximum retention period at 720 hours (30 days)', () => {
		const now = new Date('2026-09-04T12:00:00Z');
		const cutoff = calculateRetentionCutoff(1000, now);
		const expectedDiffMs = 720 * 60 * 60 * 1000;
		expect(now.getTime() - cutoff.getTime()).toBe(expectedDiffMs);
	});
});
