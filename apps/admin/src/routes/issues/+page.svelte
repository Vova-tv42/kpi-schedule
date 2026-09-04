<script lang="ts">
	import { onMount } from 'svelte';
	import Badge from '$lib/components/Badge.svelte';
	import {
		Inbox,
		RefreshCw,
		Search,
		MessageSquare,
		ChevronLeft,
		ChevronRight,
		Lock
	} from 'lucide-svelte';
	import { page } from '$app/state';
	import {
		ISSUE_STATUSES,
		ISSUE_TYPES,
		statusLabel,
		statusVariant,
		typeLabel,
		reporterLabel,
		formatIssueDate,
		type Issue
	} from '$lib/issues';

	const user = $derived(page.data.user);

	const PAGE_SIZE = 25;

	let issues = $state<Issue[]>([]);
	let statusCounts = $state<Record<string, number>>({});
	let total = $state<number>(0);
	let offset = $state<number>(0);
	let isLoading = $state<boolean>(true);
	let error = $state<string | null>(null);

	let statusFilter = $state<string>('');
	let typeFilter = $state<string>('');
	let search = $state<string>('');
	// Only applied on submit, so typing doesn't fire a request per keystroke.
	let appliedSearch = $state<string>('');

	const allCount = $derived(Object.values(statusCounts).reduce((sum, n) => sum + n, 0));

	async function loadIssues() {
		isLoading = true;
		error = null;
		try {
			const params = new URLSearchParams({
				limit: String(PAGE_SIZE),
				offset: String(offset)
			});
			if (statusFilter) params.set('status', statusFilter);
			if (typeFilter) params.set('type', typeFilter);
			if (appliedSearch) params.set('q', appliedSearch);

			const res = await fetch(`/api/issues?${params.toString()}`);
			const data = await res.json().catch(() => null);

			if (res.ok) {
				issues = data?.issues || [];
				total = data?.total ?? 0;
				statusCounts = data?.status_counts || {};
			} else {
				error = data?.message || data?.error || `HTTP ${res.status}`;
			}
		} catch (err: any) {
			error = err?.message || 'Failed to fetch the issue queue';
		} finally {
			isLoading = false;
		}
	}

	// Any filter change resets paging: page 3 of the old filter is meaningless.
	function applyFilter(next: { status?: string; type?: string }) {
		if (next.status !== undefined) statusFilter = next.status;
		if (next.type !== undefined) typeFilter = next.type;
		offset = 0;
		loadIssues();
	}

	function applySearch(event: SubmitEvent) {
		event.preventDefault();
		appliedSearch = search.trim();
		offset = 0;
		loadIssues();
	}

	function changePage(delta: number) {
		const next = offset + delta * PAGE_SIZE;
		if (next < 0 || next >= total) return;
		offset = next;
		loadIssues();
	}

	onMount(() => {
		loadIssues();
	});
</script>

<svelte:head>
	<title>KPI Schedule | Issue Queue</title>
</svelte:head>

<div class="space-y-6">
	<!-- Title Bar -->
	<div class="flex flex-col sm:flex-row sm:items-center justify-between gap-4 border-b border-slate-300 dark:border-[#252b3b] pb-4">
		<div class="flex items-center gap-2">
			<Inbox size={22} class="text-emerald-600 dark:text-[#d4ff32]" />
			<h1 class="font-display font-extrabold text-2xl sm:text-3xl text-slate-950 dark:text-[#f1f5f9] tracking-tight">
				ISSUE QUEUE
			</h1>
		</div>

		<div class="flex items-center gap-2.5">
			{#if user?.role === 'read-only'}
				<span class="text-[10px] px-2 py-1 rounded-xs bg-amber-100 text-amber-900 border border-amber-300 dark:bg-amber-500/10 dark:text-amber-400 dark:border-amber-500/30 flex items-center gap-1 font-mono font-semibold uppercase tracking-wider">
					<Lock size={10} /> Read-only
				</span>
			{/if}
			<button
				onclick={loadIssues}
				disabled={isLoading}
				class="p-2 border border-slate-300 hover:border-slate-400 bg-white hover:bg-slate-100 text-slate-700 dark:border-[#252b3b] dark:hover:border-[#64748b] dark:bg-[#181c26] dark:text-[#94a3b8] dark:hover:text-[#f1f5f9] rounded-xs transition-colors cursor-pointer shadow-xs dark:shadow-none"
				title="Refresh issue queue"
			>
				<RefreshCw size={14} class={isLoading ? 'animate-spin text-emerald-600 dark:text-[#d4ff32]' : ''} />
			</button>
		</div>
	</div>

	{#if error}
		<div class="p-3.5 border border-red-300 bg-red-100 text-red-900 dark:border-[#ef4444]/40 dark:bg-[#ef4444]/10 dark:text-[#fca5a5] text-xs font-mono rounded-xs">
			<div class="font-bold uppercase mb-0.5">[Queue Error]</div>
			<div>{error}</div>
		</div>
	{/if}

	<!-- Status Tabs -->
	<div class="flex flex-wrap items-center gap-2">
		<button
			onclick={() => applyFilter({ status: '' })}
			class="px-3 py-1.5 border rounded-xs text-xs font-mono uppercase tracking-wider transition-colors cursor-pointer {statusFilter === '' ? 'bg-[#ccf600]/25 border-slate-900 text-slate-950 font-bold dark:bg-[#d4ff32]/10 dark:border-[#d4ff32]/40 dark:text-[#d4ff32]' : 'bg-white border-slate-300 text-slate-600 hover:bg-slate-100 dark:bg-[#181c26] dark:border-[#252b3b] dark:text-[#94a3b8] dark:hover:text-[#f1f5f9]'}"
		>
			All ({allCount})
		</button>
		{#each ISSUE_STATUSES as status (status.value)}
			<button
				onclick={() => applyFilter({ status: status.value })}
				class="px-3 py-1.5 border rounded-xs text-xs font-mono uppercase tracking-wider transition-colors cursor-pointer {statusFilter === status.value ? 'bg-[#ccf600]/25 border-slate-900 text-slate-950 font-bold dark:bg-[#d4ff32]/10 dark:border-[#d4ff32]/40 dark:text-[#d4ff32]' : 'bg-white border-slate-300 text-slate-600 hover:bg-slate-100 dark:bg-[#181c26] dark:border-[#252b3b] dark:text-[#94a3b8] dark:hover:text-[#f1f5f9]'}"
			>
				{status.label} ({statusCounts[status.value] ?? 0})
			</button>
		{/each}
	</div>

	<!-- Type filter + search -->
	<div class="flex flex-col sm:flex-row gap-3 sm:items-center">
		<select
			value={typeFilter}
			onchange={(e) => applyFilter({ type: e.currentTarget.value })}
			class="bg-white dark:bg-[#0a0b0e] border border-slate-300 dark:border-[#252b3b] rounded-xs px-2.5 py-2 text-xs font-mono text-slate-900 dark:text-[#f1f5f9] focus:outline-none focus:border-slate-600 dark:focus:border-[#d4ff32] cursor-pointer"
		>
			<option value="">All types</option>
			{#each ISSUE_TYPES as type (type.value)}
				<option value={type.value}>{type.label}</option>
			{/each}
		</select>

		<form onsubmit={applySearch} class="flex items-center gap-2 flex-1">
			<div class="relative flex-1">
				<Search size={14} class="absolute left-2.5 top-1/2 -translate-y-1/2 text-slate-400 dark:text-[#64748b]" />
				<input
					type="search"
					bind:value={search}
					placeholder="Search title and description..."
					class="w-full pl-8 pr-3 py-2 bg-white dark:bg-[#0a0b0e] border border-slate-300 dark:border-[#252b3b] focus:border-slate-600 dark:focus:border-[#d4ff32] text-slate-950 dark:text-[#f1f5f9] text-xs font-mono rounded-xs focus:outline-none"
				/>
			</div>
			<button
				type="submit"
				class="px-3.5 py-2 border border-slate-300 hover:border-slate-400 bg-white hover:bg-slate-100 text-slate-700 dark:border-[#252b3b] dark:hover:border-[#64748b] dark:bg-[#181c26] dark:text-[#94a3b8] dark:hover:text-[#f1f5f9] text-xs font-mono uppercase tracking-wider rounded-xs transition-colors cursor-pointer"
			>
				Search
			</button>
		</form>
	</div>

	<!-- Queue Table -->
	<div class="border border-slate-300 dark:border-[#252b3b] bg-white dark:bg-[#12151d] rounded-xs overflow-hidden font-mono text-xs sm:text-sm shadow-xs dark:shadow-none">
		<div class="overflow-x-auto">
			<table class="w-full text-left border-collapse">
				<thead>
					<tr class="bg-slate-50 dark:bg-[#151922]/50 border-b border-slate-300 dark:border-[#252b3b] text-xs uppercase text-slate-600 dark:text-[#94a3b8]">
						<th class="py-2.5 px-4 font-medium">#</th>
						<th class="py-2.5 px-4 font-medium">Title</th>
						<th class="py-2.5 px-4 font-medium">Type</th>
						<th class="py-2.5 px-4 font-medium">Reporter</th>
						<th class="py-2.5 px-4 font-medium">Status</th>
						<th class="py-2.5 px-4 font-medium">Thread</th>
						<th class="py-2.5 px-4 font-medium">Filed</th>
					</tr>
				</thead>
				<tbody class="divide-y divide-slate-200 dark:divide-[#252b3b]/60">
					{#if isLoading && issues.length === 0}
						<tr>
							<td colspan="7" class="py-10 text-center text-slate-500 dark:text-[#64748b]">
								<RefreshCw size={18} class="animate-spin text-emerald-600 dark:text-[#d4ff32] mx-auto mb-2" />
								Loading issue queue...
							</td>
						</tr>
					{:else if issues.length === 0}
						<tr>
							<td colspan="7" class="py-10 text-center text-slate-500 dark:text-[#64748b]">
								No issues match this filter. Reports arrive from the bot's /issues command.
							</td>
						</tr>
					{:else}
						{#each issues as issue (issue.id)}
							<tr class="hover:bg-slate-50 dark:hover:bg-[#181c26]/60 transition-colors cursor-pointer">
								<td class="py-3 px-4 tabular-nums text-slate-500 dark:text-[#64748b] whitespace-nowrap">
									<a href="/issues/{issue.id}" class="block">#{issue.number}</a>
								</td>
								<td class="py-3 px-4 max-w-md truncate font-medium text-slate-950 dark:text-[#f1f5f9]">
									<a href="/issues/{issue.id}" class="block truncate hover:underline">{issue.title}</a>
								</td>
								<td class="py-3 px-4 whitespace-nowrap text-slate-600 dark:text-[#94a3b8]">
									<a href="/issues/{issue.id}" class="block">{typeLabel(issue.type)}</a>
								</td>
								<td class="py-3 px-4 whitespace-nowrap text-slate-600 dark:text-[#94a3b8]">
									<a href="/issues/{issue.id}" class="block">{reporterLabel(issue)}</a>
								</td>
								<td class="py-3 px-4 whitespace-nowrap">
									<a href="/issues/{issue.id}" class="block">
										<Badge variant={statusVariant(issue.status)}>{statusLabel(issue.status)}</Badge>
									</a>
								</td>
								<td class="py-3 px-4 whitespace-nowrap text-slate-500 dark:text-[#64748b]">
									<a href="/issues/{issue.id}" class="flex items-center gap-1.5">
										{#if issue.thread_open}
											<MessageSquare size={13} class="text-sky-600 dark:text-[#06b6d4]" />
											<span class="tabular-nums">{issue.comment_count}</span>
										{:else}
											<span class="text-slate-400 dark:text-[#475569]">—</span>
										{/if}
									</a>
								</td>
								<td class="py-3 px-4 whitespace-nowrap text-slate-500 dark:text-[#64748b]">
									<a href="/issues/{issue.id}" class="block">{formatIssueDate(issue.created_at)}</a>
								</td>
							</tr>
						{/each}
					{/if}
				</tbody>
			</table>
		</div>

		{#if total > PAGE_SIZE}
			<div class="px-4 py-3 border-t border-slate-300 dark:border-[#252b3b] bg-slate-50 dark:bg-[#151922] flex items-center justify-between text-xs text-slate-600 dark:text-[#94a3b8]">
				<span class="tabular-nums">
					{offset + 1}–{Math.min(offset + PAGE_SIZE, total)} of {total}
				</span>
				<div class="flex items-center gap-2">
					<button
						onclick={() => changePage(-1)}
						disabled={offset === 0}
						class="p-1.5 border border-slate-300 dark:border-[#252b3b] bg-white dark:bg-[#0a0b0e] rounded-xs disabled:opacity-40 enabled:cursor-pointer enabled:hover:bg-slate-100 dark:enabled:hover:bg-[#181c26] transition-colors"
						title="Previous page"
					>
						<ChevronLeft size={14} />
					</button>
					<button
						onclick={() => changePage(1)}
						disabled={offset + PAGE_SIZE >= total}
						class="p-1.5 border border-slate-300 dark:border-[#252b3b] bg-white dark:bg-[#0a0b0e] rounded-xs disabled:opacity-40 enabled:cursor-pointer enabled:hover:bg-slate-100 dark:enabled:hover:bg-[#181c26] transition-colors"
						title="Next page"
					>
						<ChevronRight size={14} />
					</button>
				</div>
			</div>
		{/if}
	</div>
</div>
