// Shared vocabulary for the issue queue. The wire values come from the Go
// server (see docs/api/admin-endpoints.md §2.5); keep this in step with
// model.IssueStatus / model.IssueType.

export type IssueStatus =
	| 'on_review'
	| 'ready'
	| 'in_development'
	| 'implemented'
	| 'duplicate'
	| 'rejected'
	| 'cancelled';
export type IssueThreadState = 'none' | 'open' | 'closed';
export type IssueType = 'feature' | 'bug' | 'other';
export type IssueCommentRole = 'user' | 'admin';

export interface Issue {
	id: string;
	number: number;
	author_telegram_id: number;
	author_username: string;
	author_first_name: string;
	type: IssueType;
	title: string;
	body: string;
	status: IssueStatus;
	status_by: string;
	/** The optional explanation the last status change carried, if any. */
	status_note: string;
	thread_state: IssueThreadState;
	comment_count: number;
	created_at: string;
	updated_at: string;
}

export interface IssueComment {
	id: string;
	author_role: IssueCommentRole;
	author_label: string;
	body: string;
	created_at: string;
}

export const ISSUE_STATUSES: { value: IssueStatus; label: string }[] = [
	{ value: 'on_review', label: 'On Review' },
	{ value: 'ready', label: 'Ready For Dev' },
	{ value: 'in_development', label: 'In Development' },
	{ value: 'implemented', label: 'Implemented' },
	{ value: 'duplicate', label: 'Duplicate' },
	{ value: 'rejected', label: 'Rejected' },
	{ value: 'cancelled', label: 'Cancelled' }
];

export const ISSUE_TYPES: { value: IssueType; label: string }[] = [
	{ value: 'feature', label: 'Feature Request' },
	{ value: 'bug', label: 'Bug Fix' },
	{ value: 'other', label: 'Other' }
];

const STATUS_LABELS = Object.fromEntries(ISSUE_STATUSES.map((s) => [s.value, s.label]));
const TYPE_LABELS = Object.fromEntries(ISSUE_TYPES.map((t) => [t.value, t.label]));

// Badge palette, matching the variants Badge.svelte defines. 'rejected' and
// 'cancelled' deliberately share crimson: both are terminal refusals, and the
// label already distinguishes them.
const STATUS_VARIANTS: Record<IssueStatus, 'slate' | 'cyan' | 'amber' | 'emerald' | 'crimson' | 'violet'> = {
	on_review: 'slate',
	ready: 'cyan',
	in_development: 'amber',
	implemented: 'emerald',
	duplicate: 'violet',
	rejected: 'crimson',
	cancelled: 'crimson'
};

export function statusLabel(status: string): string {
	return STATUS_LABELS[status] ?? status;
}

export function statusVariant(status: string) {
	return STATUS_VARIANTS[status as IssueStatus] ?? 'slate';
}

export function typeLabel(type: string): string {
	return TYPE_LABELS[type] ?? type;
}

/** How the reporter is addressed in the UI; falls back to their Telegram id. */
export function reporterLabel(issue: Pick<Issue, 'author_username' | 'author_first_name' | 'author_telegram_id'>) {
	if (issue.author_username) return `@${issue.author_username}`;
	if (issue.author_first_name) return issue.author_first_name;
	return String(issue.author_telegram_id);
}

/** Whether a discussion exists at all — open or closed. */
export function threadStarted(state: string): boolean {
	return state === 'open' || state === 'closed';
}

export function formatIssueDate(iso: string): string {
	try {
		return new Date(iso).toLocaleString([], {
			year: 'numeric',
			month: 'short',
			day: 'numeric',
			hour: '2-digit',
			minute: '2-digit'
		});
	} catch {
		return iso;
	}
}
