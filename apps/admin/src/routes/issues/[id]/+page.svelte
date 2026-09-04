<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import Badge from '$lib/components/Badge.svelte';
	import Modal from '$lib/components/Modal.svelte';
	import {
		ArrowLeft,
		RefreshCw,
		MessageSquare,
		MessageSquareOff,
		Send,
		Lock,
		LockOpen,
		Trash2,
		User,
		Wrench,
		Save
	} from 'lucide-svelte';
	import {
		ISSUE_STATUSES,
		statusLabel,
		statusVariant,
		typeLabel,
		reporterLabel,
		formatIssueDate,
		threadStarted,
		type Issue,
		type IssueComment
	} from '$lib/issues';

	const issueId = $derived(page.params.id);
	const user = $derived(page.data.user);
	const canWrite = $derived(user?.role !== 'read-only');

	// The single 3000-character limit the bot and the Go server both enforce.
	const COMMENT_MAX_LEN = 3000;

	let issue = $state<Issue | null>(null);
	let comments = $state<IssueComment[]>([]);
	let isLoading = $state<boolean>(true);
	let error = $state<string | null>(null);

	let selectedStatus = $state<string>('');
	let statusNote = $state<string>('');
	let isSavingStatus = $state<boolean>(false);
	let statusError = $state<string | null>(null);

	let commentBody = $state<string>('');
	let isPosting = $state<boolean>(false);
	let commentError = $state<string | null>(null);

	let isUpdatingThread = $state<boolean>(false);
	let threadError = $state<string | null>(null);

	let deleteModalOpen = $state<boolean>(false);
	let isDeleting = $state<boolean>(false);
	let deleteError = $state<string | null>(null);

	const statusDirty = $derived(!!issue && selectedStatus !== issue.status);
	const threadIsOpen = $derived(issue?.thread_state === 'open');
	const threadIsClosed = $derived(issue?.thread_state === 'closed');
	const hasThread = $derived(!!issue && threadStarted(issue.thread_state));

	async function loadIssue() {
		isLoading = true;
		error = null;
		try {
			const res = await fetch(`/api/issues/${issueId}`);
			const data = await res.json().catch(() => null);

			if (res.ok) {
				issue = data?.issue ?? null;
				comments = data?.comments || [];
				selectedStatus = issue?.status ?? '';
				// The note belongs to the change being composed, not to the
				// issue, so it always starts empty on (re)load.
				statusNote = '';
			} else {
				error = data?.message || data?.error || `HTTP ${res.status}`;
			}
		} catch (err: any) {
			error = err?.message || 'Failed to fetch the issue';
		} finally {
			isLoading = false;
		}
	}

	async function saveStatus() {
		if (!issue || !statusDirty) return;

		isSavingStatus = true;
		statusError = null;
		try {
			const res = await fetch(`/api/issues/${issueId}`, {
				method: 'PATCH',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ status: selectedStatus, note: statusNote.trim() })
			});
			const data = await res.json().catch(() => null);

			if (res.ok) {
				await loadIssue();
			} else {
				statusError = data?.message || data?.error || `HTTP ${res.status}`;
			}
		} catch (err: any) {
			statusError = err?.message || 'Failed to update the status';
		} finally {
			isSavingStatus = false;
		}
	}

	async function setThreadState(state: 'open' | 'closed') {
		isUpdatingThread = true;
		threadError = null;
		try {
			const res = await fetch(`/api/issues/${issueId}/thread`, {
				method: 'PATCH',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ state })
			});
			const data = await res.json().catch(() => null);

			if (res.ok) {
				await loadIssue();
			} else {
				threadError = data?.message || data?.error || `HTTP ${res.status}`;
			}
		} catch (err: any) {
			threadError = err?.message || 'Failed to update the discussion';
		} finally {
			isUpdatingThread = false;
		}
	}

	async function deleteIssue() {
		isDeleting = true;
		deleteError = null;
		try {
			const res = await fetch(`/api/issues/${issueId}`, { method: 'DELETE' });
			if (res.ok) {
				deleteModalOpen = false;
				// Nothing is left to show on this URL.
				await goto('/issues');
				return;
			}
			const data = await res.json().catch(() => null);
			deleteError = data?.message || data?.error || `HTTP ${res.status}`;
		} catch (err: any) {
			deleteError = err?.message || 'Failed to delete the issue';
		} finally {
			isDeleting = false;
		}
	}

	async function postComment() {
		const body = commentBody.trim();
		if (!body) return;

		isPosting = true;
		commentError = null;
		try {
			const res = await fetch(`/api/issues/${issueId}`, {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ body })
			});
			const data = await res.json().catch(() => null);

			if (res.ok) {
				commentBody = '';
				// Reload rather than append: the first reply also flips
				// thread_open, which changes what the reporter can see.
				await loadIssue();
			} else {
				commentError = data?.message || data?.error || `HTTP ${res.status}`;
			}
		} catch (err: any) {
			commentError = err?.message || 'Failed to post the reply';
		} finally {
			isPosting = false;
		}
	}

	onMount(() => {
		loadIssue();
	});
</script>

<svelte:head>
	<title>KPI Schedule | Issue {issue ? `#${issue.number}` : ''}</title>
</svelte:head>

<div class="space-y-6">
	<!-- Title Bar -->
	<div class="flex flex-col sm:flex-row sm:items-center justify-between gap-4 border-b border-slate-300 dark:border-[#252b3b] pb-4">
		<div class="flex items-center gap-3 min-w-0">
			<a
				href="/issues"
				class="p-2 border border-slate-300 hover:border-slate-400 bg-white hover:bg-slate-100 text-slate-700 dark:border-[#252b3b] dark:hover:border-[#64748b] dark:bg-[#181c26] dark:text-[#94a3b8] dark:hover:text-[#f1f5f9] rounded-xs transition-colors shrink-0"
				title="Back to the queue"
			>
				<ArrowLeft size={14} />
			</a>
			<h1 class="font-display font-extrabold text-xl sm:text-2xl text-slate-950 dark:text-[#f1f5f9] tracking-tight truncate">
				{#if issue}
					<span class="text-slate-500 dark:text-[#64748b] tabular-nums">#{issue.number}</span>
					{issue.title}
				{:else}
					ISSUE
				{/if}
			</h1>
		</div>

		<div class="flex items-center gap-2.5 shrink-0 self-start">
			<button
				onclick={loadIssue}
				disabled={isLoading}
				class="p-2 border border-slate-300 hover:border-slate-400 bg-white hover:bg-slate-100 text-slate-700 dark:border-[#252b3b] dark:hover:border-[#64748b] dark:bg-[#181c26] dark:text-[#94a3b8] dark:hover:text-[#f1f5f9] rounded-xs transition-colors cursor-pointer shadow-xs dark:shadow-none"
				title="Refresh issue"
			>
				<RefreshCw size={14} class={isLoading ? 'animate-spin text-emerald-600 dark:text-[#d4ff32]' : ''} />
			</button>

			{#if canWrite && issue}
				<button
					onclick={() => { deleteModalOpen = true; deleteError = null; }}
					class="p-2 border border-slate-300 hover:border-red-400 bg-white hover:bg-red-50 text-slate-600 hover:text-red-700 dark:border-[#252b3b] dark:hover:border-[#ef4444]/60 dark:bg-[#181c26] dark:text-[#94a3b8] dark:hover:text-[#ef4444] rounded-xs transition-colors cursor-pointer shadow-xs dark:shadow-none"
					title="Delete issue"
				>
					<Trash2 size={14} />
				</button>
			{/if}
		</div>
	</div>

	{#if error}
		<div class="p-3.5 border border-red-300 bg-red-100 text-red-900 dark:border-[#ef4444]/40 dark:bg-[#ef4444]/10 dark:text-[#fca5a5] text-xs font-mono rounded-xs">
			<div class="font-bold uppercase mb-0.5">[Issue Error]</div>
			<div>{error}</div>
		</div>
	{/if}

	{#if issue}
		<!-- Meta + body -->
		<div class="border border-slate-300 dark:border-[#252b3b] bg-white dark:bg-[#12151d] rounded-xs shadow-xs dark:shadow-none">
			<div class="px-4 py-3 border-b border-slate-300 dark:border-[#252b3b] bg-slate-50 dark:bg-[#151922] flex flex-wrap items-center gap-x-4 gap-y-2 font-mono text-xs">
				<Badge variant={statusVariant(issue.status)}>{statusLabel(issue.status)}</Badge>
				<span class="text-slate-600 dark:text-[#94a3b8]">{typeLabel(issue.type)}</span>
				<span class="text-slate-500 dark:text-[#64748b]">
					Filed by <span class="text-slate-900 dark:text-[#f1f5f9]">{reporterLabel(issue)}</span>
					on {formatIssueDate(issue.created_at)}
				</span>
				{#if issue.status_by}
					<span class="text-slate-500 dark:text-[#64748b]">Last triaged by {issue.status_by}</span>
				{/if}
			</div>

			<div class="p-4 font-mono text-xs sm:text-sm text-slate-900 dark:text-[#e6edf3] whitespace-pre-wrap break-words">
				{issue.body}
			</div>

			{#if issue.status_note}
				<div class="mx-4 mb-4 px-3 py-2.5 border-l-2 border-slate-400 dark:border-[#d4ff32]/50 bg-slate-50 dark:bg-[#151922]/60 rounded-xs font-mono text-xs">
					<div class="uppercase tracking-wider text-slate-500 dark:text-[#64748b] mb-1">
						Note sent with the current status
					</div>
					<div class="text-slate-900 dark:text-[#e6edf3] whitespace-pre-wrap break-words">{issue.status_note}</div>
				</div>
			{/if}
		</div>

		<!-- Status control -->
		<div class="border border-slate-300 dark:border-[#252b3b] bg-white dark:bg-[#12151d] rounded-xs p-4 shadow-xs dark:shadow-none space-y-3">
			<div class="font-display font-semibold text-sm text-slate-950 dark:text-[#f1f5f9]">Triage</div>

			{#if canWrite}
				<div class="space-y-3 font-mono text-xs">
					<div class="flex flex-wrap items-center gap-2.5">
						<select
							bind:value={selectedStatus}
							class="bg-white dark:bg-[#0a0b0e] border border-slate-300 dark:border-[#252b3b] rounded-xs px-2.5 py-2 text-xs font-mono text-slate-900 dark:text-[#f1f5f9] focus:outline-none focus:border-slate-600 dark:focus:border-[#d4ff32] cursor-pointer"
						>
							{#each ISSUE_STATUSES as status (status.value)}
								<option value={status.value}>{status.label}</option>
							{/each}
						</select>

						<button
							onclick={saveStatus}
							disabled={!statusDirty || isSavingStatus}
							class="flex items-center gap-1.5 px-4 py-2 border border-black dark:border-[#d4ff32]/50 bg-[#ccf600] hover:bg-[#bde800] dark:bg-[#d4ff32] dark:hover:bg-[#e2f952] text-black text-xs font-mono font-bold uppercase tracking-wider rounded-xs transition-all shadow-[2px_2px_0px_#090d16] dark:shadow-[0_0_15px_rgba(212,255,50,0.2)] disabled:opacity-40 enabled:cursor-pointer"
						>
							<Save size={13} />
							<span>{isSavingStatus ? 'Saving...' : 'Save Status'}</span>
						</button>

						<span class="text-slate-500 dark:text-[#64748b]">
							The reporter is notified in Telegram.
						</span>
					</div>

					<!-- A one-off explanation ("rejected because…") that rides along
					     with the status DM instead of opening a discussion. -->
					<div>
						<label for="status-note" class="block uppercase tracking-wider text-slate-600 dark:text-[#94a3b8] mb-1.5">
							Note to the reporter (optional)
						</label>
						<textarea
							id="status-note"
							bind:value={statusNote}
							rows="2"
							maxlength={COMMENT_MAX_LEN}
							disabled={!statusDirty}
							placeholder={statusDirty
								? 'Explain the change, e.g. why this was rejected...'
								: 'Pick a different status to send a note with it.'}
							class="w-full px-3 py-2 bg-slate-50 dark:bg-[#0a0b0e] border border-slate-300 dark:border-[#252b3b] focus:border-slate-600 dark:focus:border-[#d4ff32] text-slate-950 dark:text-[#f1f5f9] text-xs font-mono rounded-xs focus:outline-none resize-y disabled:opacity-50"
						></textarea>
						<p class="text-[11px] text-slate-500 dark:text-[#64748b] mt-1">
							Sent once, with the status change — it does not open a discussion. Leaving it
							empty clears any note the reporter is currently shown.
						</p>
					</div>
				</div>
			{:else}
				<div class="flex items-center gap-2 font-mono text-xs text-slate-500 dark:text-[#64748b]">
					<Lock size={13} />
					<span>Read-only role: status changes require read-write access.</span>
				</div>
			{/if}

			{#if statusError}
				<div class="p-2.5 border border-red-300 bg-red-100 text-red-900 dark:border-[#ef4444]/40 dark:bg-[#ef4444]/10 dark:text-[#fca5a5] text-xs font-mono rounded-xs">
					{statusError}
				</div>
			{/if}
		</div>

		<!-- Discussion thread -->
		<div class="border border-slate-300 dark:border-[#252b3b] bg-white dark:bg-[#12151d] rounded-xs shadow-xs dark:shadow-none">
			<div class="px-4 py-3 border-b border-slate-300 dark:border-[#252b3b] bg-slate-50 dark:bg-[#151922] flex flex-wrap items-center justify-between gap-2">
				<div class="flex items-center gap-2 font-display font-semibold text-sm text-slate-950 dark:text-[#f1f5f9]">
					{#if threadIsClosed}
						<MessageSquareOff size={16} class="text-slate-500 dark:text-[#64748b]" />
						<span>Discussion ({comments.length})</span>
						<Badge variant="slate">Closed</Badge>
					{:else}
						<MessageSquare size={16} class="text-sky-600 dark:text-[#06b6d4]" />
						<span>Discussion ({comments.length})</span>
					{/if}
				</div>

				{#if !hasThread}
					<span class="font-mono text-xs text-slate-500 dark:text-[#64748b]">
						Not started — your first reply opens it for the reporter
					</span>
				{:else if canWrite}
					<button
						onclick={() => setThreadState(threadIsClosed ? 'open' : 'closed')}
						disabled={isUpdatingThread}
						class="flex items-center gap-1.5 px-3 py-1.5 border border-slate-300 hover:border-slate-400 bg-white hover:bg-slate-100 text-slate-700 dark:border-[#252b3b] dark:hover:border-[#64748b] dark:bg-[#181c26] dark:text-[#94a3b8] dark:hover:text-[#f1f5f9] text-xs font-mono uppercase tracking-wider rounded-xs transition-colors disabled:opacity-40 enabled:cursor-pointer"
						title={threadIsClosed
							? 'Let the reporter reply again'
							: 'Stop the reporter replying; they keep read access'}
					>
						{#if threadIsClosed}
							<LockOpen size={13} />
							<span>{isUpdatingThread ? 'Reopening...' : 'Reopen'}</span>
						{:else}
							<Lock size={13} />
							<span>{isUpdatingThread ? 'Closing...' : 'Close Discussion'}</span>
						{/if}
					</button>
				{/if}
			</div>

			{#if threadError}
				<div class="mx-4 mt-4 p-2.5 border border-red-300 bg-red-100 text-red-900 dark:border-[#ef4444]/40 dark:bg-[#ef4444]/10 dark:text-[#fca5a5] text-xs font-mono rounded-xs">
					{threadError}
				</div>
			{/if}

			<div class="p-4 space-y-3 font-mono text-xs sm:text-sm">
				{#if comments.length === 0}
					<div class="py-6 text-center text-slate-500 dark:text-[#64748b]">
						No messages yet.
					</div>
				{:else}
					{#each comments as comment (comment.id)}
						<div class="border border-slate-200 dark:border-[#252b3b]/60 rounded-xs overflow-hidden">
							<div class="px-3 py-2 flex flex-wrap items-center gap-2 text-xs {comment.author_role === 'admin' ? 'bg-[#ccf600]/15 dark:bg-[#d4ff32]/5' : 'bg-slate-50 dark:bg-[#151922]/60'}">
								{#if comment.author_role === 'admin'}
									<Wrench size={13} class="text-emerald-700 dark:text-[#d4ff32]" />
								{:else}
									<User size={13} class="text-sky-600 dark:text-[#06b6d4]" />
								{/if}
								<span class="font-bold text-slate-950 dark:text-[#f1f5f9]">
									{comment.author_label || (comment.author_role === 'admin' ? 'Team' : 'Reporter')}
								</span>
								<span class="text-slate-500 dark:text-[#64748b]">{formatIssueDate(comment.created_at)}</span>
							</div>
							<div class="px-3 py-2.5 text-slate-900 dark:text-[#e6edf3] whitespace-pre-wrap break-words">
								{comment.body}
							</div>
						</div>
					{/each}
				{/if}
			</div>

			<!-- Composer -->
			<div class="px-4 pb-4">
				{#if canWrite && threadIsClosed}
					<div class="flex flex-wrap items-center gap-2 px-3 py-2.5 border border-slate-300 dark:border-[#252b3b] bg-slate-50 dark:bg-[#0a0b0e] rounded-xs font-mono text-xs text-slate-500 dark:text-[#64748b]">
						<Lock size={13} />
						<span>This discussion is closed. Reopen it to write — the reporter can still read it.</span>
					</div>
				{:else if canWrite}
					<div class="space-y-2">
						<textarea
							bind:value={commentBody}
							rows="3"
							maxlength={COMMENT_MAX_LEN}
							placeholder="Ask the reporter a question or share an update..."
							class="w-full px-3 py-2 bg-slate-50 dark:bg-[#0a0b0e] border border-slate-300 dark:border-[#252b3b] focus:border-slate-600 dark:focus:border-[#d4ff32] text-slate-950 dark:text-[#f1f5f9] text-xs sm:text-sm font-mono rounded-xs focus:outline-none resize-y"
						></textarea>

						<div class="flex flex-wrap items-center justify-between gap-2">
							<span class="font-mono text-xs text-slate-500 dark:text-[#64748b] tabular-nums">
								{commentBody.length}/{COMMENT_MAX_LEN}
							</span>
							<button
								onclick={postComment}
								disabled={isPosting || !commentBody.trim()}
								class="flex items-center gap-1.5 px-4 py-2 border border-black dark:border-[#d4ff32]/50 bg-[#ccf600] hover:bg-[#bde800] dark:bg-[#d4ff32] dark:hover:bg-[#e2f952] text-black text-xs font-mono font-bold uppercase tracking-wider rounded-xs transition-all shadow-[2px_2px_0px_#090d16] dark:shadow-[0_0_15px_rgba(212,255,50,0.2)] disabled:opacity-40 enabled:cursor-pointer"
							>
								<Send size={13} />
								<span>{isPosting ? 'Sending...' : 'Send Reply'}</span>
							</button>
						</div>

						{#if commentError}
							<div class="p-2.5 border border-red-300 bg-red-100 text-red-900 dark:border-[#ef4444]/40 dark:bg-[#ef4444]/10 dark:text-[#fca5a5] text-xs font-mono rounded-xs">
								{commentError}
							</div>
						{/if}
					</div>
				{:else}
					<div class="flex items-center gap-2 px-3 py-2.5 border border-slate-300 dark:border-[#252b3b] bg-slate-50 dark:bg-[#0a0b0e] rounded-xs font-mono text-xs text-slate-500 dark:text-[#64748b]">
						<Lock size={13} />
						<span>Read-only role: replying requires read-write access.</span>
					</div>
				{/if}
			</div>
		</div>
	{:else if !isLoading && !error}
		<div class="py-16 text-center font-mono text-sm text-slate-500 dark:text-[#64748b]">
			Issue not found.
		</div>
	{/if}
</div>

<!-- Delete Issue Confirmation -->
<Modal
	open={deleteModalOpen}
	title="DELETE ISSUE"
	description="This removes the issue and its entire discussion permanently, for the reporter as well. It cannot be undone."
	onClose={() => { deleteModalOpen = false; }}
>
	{#if deleteError}
		<div class="mb-4 p-3 border border-red-300 bg-red-100 text-red-900 dark:border-[#ef4444]/40 dark:bg-[#ef4444]/10 dark:text-[#fca5a5] text-xs font-mono rounded-xs">
			{deleteError}
		</div>
	{/if}

	<div class="font-mono text-xs sm:text-sm text-slate-700 dark:text-[#94a3b8]">
		{#if issue}
			<div class="mb-2">
				<span class="text-slate-500 dark:text-[#64748b] tabular-nums">#{issue.number}</span>
				<span class="text-slate-950 dark:text-[#f1f5f9] font-bold">{issue.title}</span>
			</div>
			<div>
				Filed by {reporterLabel(issue)} · {comments.length}
				{comments.length === 1 ? 'message' : 'messages'} in the discussion.
			</div>
			<p class="mt-3 text-[11px] text-slate-500 dark:text-[#64748b]">
				The reporter is not notified — the issue simply disappears from their /issues list.
			</p>
		{/if}
	</div>

	{#snippet footer()}
		<div class="flex items-center gap-2">
			<button
				onclick={() => { deleteModalOpen = false; }}
				class="px-3.5 py-1.5 border border-slate-300 hover:border-slate-400 bg-white hover:bg-slate-100 text-slate-700 dark:border-[#252b3b] dark:bg-[#181c26] dark:hover:bg-[#252b3b] dark:text-[#94a3b8] text-xs font-mono uppercase tracking-wider rounded-xs transition-colors cursor-pointer"
			>
				Keep It
			</button>
			<button
				onclick={deleteIssue}
				disabled={isDeleting}
				class="flex items-center gap-1.5 px-4 py-1.5 border border-red-700 dark:border-[#ef4444]/50 bg-red-600 hover:bg-red-700 dark:bg-[#ef4444] dark:hover:bg-[#f87171] text-white text-xs font-mono font-bold uppercase tracking-wider rounded-xs transition-all disabled:opacity-40 enabled:cursor-pointer"
			>
				<Trash2 size={13} />
				<span>{isDeleting ? 'Deleting...' : 'Delete Permanently'}</span>
			</button>
		</div>
	{/snippet}
</Modal>
