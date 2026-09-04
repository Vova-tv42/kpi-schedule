import { describe, it, expect } from 'vitest';
import {
	ISSUE_STATUSES,
	ISSUE_TYPES,
	statusLabel,
	statusVariant,
	typeLabel,
	reporterLabel,
	threadStarted
} from './issues';

describe('Issue queue vocabulary', () => {
	it('covers every status the Go server can return', () => {
		// Must stay in step with model.IssueStatus (apps/server/internal/model/domain.go).
		expect(ISSUE_STATUSES.map((s) => s.value)).toEqual([
			'on_review',
			'ready',
			'in_development',
			'implemented',
			'duplicate',
			'rejected',
			'cancelled'
		]);
		expect(ISSUE_TYPES.map((t) => t.value)).toEqual(['feature', 'bug', 'other']);
	});

	it('gives every status a badge variant Badge.svelte knows', () => {
		const known = ['lime', 'amber', 'emerald', 'crimson', 'slate', 'cyan', 'violet'];
		for (const status of ISSUE_STATUSES) {
			expect(known).toContain(statusVariant(status.value));
		}
		// 'rejected' and 'cancelled' intentionally share crimson (both are
		// terminal refusals); nothing else may collide.
		const variants = ISSUE_STATUSES.map((s) => statusVariant(s.value));
		expect(new Set(variants).size).toBe(ISSUE_STATUSES.length - 1);
		expect(statusVariant('rejected')).toBe(statusVariant('cancelled'));
		expect(statusVariant('duplicate')).not.toBe(statusVariant('on_review'));
	});

	it('treats a closed discussion as one that exists', () => {
		// The reporter keeps read access to a closed thread, so both states
		// count as started — only 'none' hides the discussion entirely.
		expect(threadStarted('open')).toBe(true);
		expect(threadStarted('closed')).toBe(true);
		expect(threadStarted('none')).toBe(false);
		// The zero value of an unpopulated field must not fake a thread.
		expect(threadStarted('')).toBe(false);
	});

	it('falls back to the raw value for anything unrecognised', () => {
		// A status added server-side before the dashboard knows about it must
		// render as itself rather than as an empty cell.
		expect(statusLabel('archived')).toBe('archived');
		expect(typeLabel('question')).toBe('question');
		expect(statusVariant('archived')).toBe('slate');
	});

	it('prefers the username, then the first name, then the telegram id', () => {
		expect(
			reporterLabel({ author_username: 'student', author_first_name: 'Olha', author_telegram_id: 42 })
		).toBe('@student');
		expect(
			reporterLabel({ author_username: '', author_first_name: 'Olha', author_telegram_id: 42 })
		).toBe('Olha');
		expect(
			reporterLabel({ author_username: '', author_first_name: '', author_telegram_id: 42 })
		).toBe('42');
	});
});
