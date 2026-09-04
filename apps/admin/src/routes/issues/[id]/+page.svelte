<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import Badge from '$lib/components/Badge.svelte';
	import {
		ArrowLeft,
		RefreshCw,
		MessageSquare,
		Send,
		Lock,
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
	let isSavingStatus = $state<boolean>(false);
	let statusError = $state<string | null>(null);

	let commentBody = $state<string>('');
	let isPosting = $state<boolean>(false);
	let commentError = $state<string | null>(null);

	const statusDirty = $derived(!!issue && selectedStatus !== issue.status);

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
				body: JSON.stringify({ status: selectedStatus })
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

		<button
			onclick={loadIssue}
			disabled={isLoading}
			class="p-2 border border-slate-300 hover:border-slate-400 bg-white hover:bg-slate-100 text-slate-700 dark:border-[#252b3b] dark:hover:border-[#64748b] dark:bg-[#181c26] dark:text-[#94a3b8] dark:hover:text-[#f1f5f9] rounded-xs transition-colors cursor-pointer shadow-xs dark:shadow-none shrink-0 self-start"
			title="Refresh issue"
		>
			<RefreshCw size={14} class={isLoading ? 'animate-spin text-emerald-600 dark:text-[#d4ff32]' : ''} />
		</button>
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
		</div>

		<!-- Status control -->
		<div class="border border-slate-300 dark:border-[#252b3b] bg-white dark:bg-[#12151d] rounded-xs p-4 shadow-xs dark:shadow-none space-y-3">
			<div class="font-display font-semibold text-sm text-slate-950 dark:text-[#f1f5f9]">Triage</div>

			{#if canWrite}
				<div class="flex flex-wrap items-center gap-2.5 font-mono text-xs">
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
			<div class="px-4 py-3 border-b border-slate-300 dark:border-[#252b3b] bg-slate-50 dark:bg-[#151922] flex items-center justify-between">
				<div class="flex items-center gap-2 font-display font-semibold text-sm text-slate-950 dark:text-[#f1f5f9]">
					<MessageSquare size={16} class="text-sky-600 dark:text-[#06b6d4]" />
					<span>Discussion ({comments.length})</span>
				</div>
				{#if !issue.thread_open}
					<span class="font-mono text-xs text-slate-500 dark:text-[#64748b]">
						Not started — your first reply opens it for the reporter
					</span>
				{/if}
			</div>

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
				{#if canWrite}
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
