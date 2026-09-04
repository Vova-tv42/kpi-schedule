<script lang="ts">
	import { page } from '$app/state';
	import { serverStatus } from '$lib/server-status-store.svelte';
	import ServerInterlockModal from '$lib/components/ServerInterlockModal.svelte';
	import {
		TerminalSquare,
		Play,
		Clock,
		CheckCircle,
		AlertTriangle,
		Download,
		Trash2
	} from 'lucide-svelte';

	const user = $derived(page.data.user);

	let query = $state<string>('SELECT * FROM users LIMIT 10;');
	let isExecuting = $state<boolean>(false);
	let error = $state<string | null>(null);
	let result = $state<{
		columns?: string[];
		rows?: Record<string, any>[];
		rows_affected?: number;
		duration_ms?: number;
	} | null>(null);

	let showInterlock = $state<boolean>(false);

	const presets = [
		{ label: 'Users Count', sql: 'SELECT COUNT(*) AS total_users FROM users;' },
		{ label: 'Recent Users', sql: 'SELECT * FROM users ORDER BY created_at DESC LIMIT 10;' },
		{ label: 'Bot Groups', sql: 'SELECT * FROM bot_groups ORDER BY created_at DESC;' },
		{ label: 'Campus Cache', sql: 'SELECT key, fetched_at FROM campus_cache ORDER BY fetched_at DESC LIMIT 10;' }
	];

	async function runQuery() {
		if (!query.trim() || user?.role === 'read-only') return;

		// If server is sleeping, prompt user before waking it
		if (serverStatus.data.status === 'sleeping') {
			showInterlock = true;
			return;
		}

		execute();
	}

	async function execute() {
		isExecuting = true;
		error = null;
		result = null;

		try {
			const res = await fetch('/api/main-db/query', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ query: query.trim() })
			});

			const data = await res.json();
			if (res.ok) {
				result = data;
				serverStatus.checkStatus();
			} else {
				error = data?.message || data?.error || `HTTP ${res.status}`;
			}
		} catch (err: any) {
			error = err?.message || 'Network communication error';
		} finally {
			isExecuting = false;
		}
	}

	function handleKeyDown(e: KeyboardEvent) {
		if ((e.ctrlKey || e.metaKey) && e.key === 'Enter') {
			e.preventDefault();
			runQuery();
		}
	}

	function exportJSON() {
		if (!result || !result.rows) return;
		const dataStr = 'data:text/json;charset=utf-8,' + encodeURIComponent(JSON.stringify(result.rows, null, 2));
		const downloadAnchor = document.createElement('a');
		downloadAnchor.setAttribute('href', dataStr);
		downloadAnchor.setAttribute('download', `query_result_${Date.now()}.json`);
		document.body.appendChild(downloadAnchor);
		downloadAnchor.click();
		downloadAnchor.remove();
	}
</script>

<svelte:head>
	<title>KPI Schedule // SQL Workspace</title>
</svelte:head>

<div class="space-y-5">
	<!-- Title Bar -->
	<div class="border-b border-slate-300 dark:border-[#252b3b] pb-4 flex flex-col sm:flex-row sm:items-center justify-between gap-4">
		<div>
			<div class="flex items-center gap-2 mb-1">
				<TerminalSquare size={22} class="text-emerald-600 dark:text-[#d4ff32]" />
				<h1 class="font-display font-extrabold text-2xl sm:text-3xl text-slate-950 dark:text-[#f1f5f9] tracking-tight">
					SQL WORKSPACE
				</h1>
			</div>
		</div>
	</div>

	{#if user?.role === 'read-only'}
		<div class="border border-amber-500/50 bg-white dark:bg-[#12151d] p-6 rounded-xs text-xs sm:text-sm font-mono text-slate-900 dark:text-[#e6edf3] space-y-2 shadow-xs dark:shadow-none">
			<div class="flex items-center gap-2 font-bold text-amber-800 dark:text-amber-400 uppercase">
				<AlertTriangle size={18} />
				<span>Access Restriction: Read-Only Operator</span>
			</div>
			<p class="leading-relaxed text-slate-600 dark:text-[#94a3b8]">
				Custom SQL execution is disabled for accounts with the <strong>read-only</strong> role to safeguard system integrity.
			</p>
			<p class="text-slate-500 dark:text-[#64748b] text-xs">
				Contact a superadmin to request role elevation to read-write.
			</p>
		</div>
	{:else}
		<!-- Query Editor & Presets Card -->
		<div class="border border-slate-300 dark:border-[#252b3b] bg-white dark:bg-[#12151d] p-4 sm:p-5 rounded-xs space-y-3 shadow-xs dark:shadow-none">
			<!-- Presets Bar -->
			<div class="flex items-center justify-between gap-2 overflow-x-auto pb-1 text-xs font-mono">
				<div class="flex items-center gap-1.5 flex-wrap">
					<span class="text-slate-500 dark:text-[#64748b] uppercase mr-1">Presets:</span>
					{#each presets as p}
						<button
							onclick={() => (query = p.sql)}
							class="px-2.5 py-1 text-xs border border-slate-300 bg-slate-100 hover:bg-slate-200 text-slate-700 hover:text-slate-950 dark:border-[#252b3b] dark:bg-[#181c26] dark:hover:bg-[#252b3b] dark:hover:border-[#d4ff32]/40 dark:text-[#94a3b8] dark:hover:text-[#f1f5f9] rounded-xs transition-colors cursor-pointer font-medium"
						>
							{p.label}
						</button>
					{/each}
				</div>

				<button
					onclick={() => (query = '')}
					class="p-1.5 text-slate-400 hover:text-red-600 rounded-xs hover:bg-red-50 dark:text-[#64748b] dark:hover:text-[#ef4444] dark:hover:bg-[#ef4444]/10 transition-colors cursor-pointer"
					title="Clear editor"
				>
					<Trash2 size={15} />
				</button>
			</div>

			<!-- SQL Editor Area -->
			<div class="relative">
				<textarea
					bind:value={query}
					onkeydown={handleKeyDown}
					rows="5"
					placeholder="Enter SQL statement (e.g. SELECT * FROM users LIMIT 10;)"
					class="w-full p-3 bg-slate-50 dark:bg-[#0a0b0e] border border-slate-300 dark:border-[#252b3b] focus:border-slate-600 dark:focus:border-[#d4ff32] text-xs sm:text-sm font-mono text-slate-950 dark:text-[#f1f5f9] rounded-xs focus:outline-none resize-y leading-relaxed"
				></textarea>
			</div>

			<!-- Action Bar -->
			<div class="flex items-center justify-between pt-1 font-mono text-xs">
				<div class="text-slate-500 dark:text-[#64748b] hidden sm:block">
					Tip: Press <kbd class="px-1.5 py-0.5 border border-slate-300 bg-slate-100 text-slate-700 dark:border-[#252b3b] dark:bg-[#0a0b0e] dark:text-[#94a3b8] rounded-xs font-semibold">Ctrl</kbd> + <kbd class="px-1.5 py-0.5 border border-slate-300 bg-slate-100 text-slate-700 dark:border-[#252b3b] dark:bg-[#0a0b0e] dark:text-[#94a3b8] rounded-xs font-semibold">Enter</kbd> to execute
				</div>

				<button
					onclick={runQuery}
					disabled={isExecuting || !query.trim()}
					class="flex items-center gap-2 px-5 py-2 border border-black dark:border-[#d4ff32]/50 bg-[#ccf600] hover:bg-[#bde800] dark:bg-[#d4ff32] dark:hover:bg-[#e2f952] text-black text-xs sm:text-sm font-mono font-bold uppercase tracking-wider rounded-xs transition-all shadow-[2px_2px_0px_#090d16] dark:shadow-[0_0_15px_rgba(212,255,50,0.2)] disabled:opacity-40 disabled:cursor-not-allowed cursor-pointer"
				>
					<Play size={13} class="fill-current" />
					<span>{isExecuting ? 'Executing...' : 'Run Query'}</span>
				</button>
			</div>
		</div>

		<!-- Error Banner -->
		{#if error}
			<div class="p-3.5 border border-red-300 bg-red-100 text-red-900 dark:border-[#ef4444]/40 dark:bg-[#ef4444]/10 dark:text-[#fca5a5] text-xs font-mono rounded-xs space-y-1">
				<div class="font-bold uppercase">[SQL Execution Error]</div>
				<div>{error}</div>
			</div>
		{/if}

		<!-- Results Viewport -->
		{#if result}
			<div class="border border-slate-300 dark:border-[#252b3b] bg-white dark:bg-[#12151d] rounded-xs overflow-hidden space-y-2 shadow-xs dark:shadow-none">
				<!-- Metrics Bar -->
				<div class="bg-slate-50 dark:bg-[#151922] border-b border-slate-300 dark:border-[#252b3b] px-4 py-2.5 flex items-center justify-between text-xs font-mono text-slate-600 dark:text-[#94a3b8]">
					<div class="flex items-center gap-4">
						<span class="flex items-center gap-1.5 text-emerald-700 dark:text-[#10b981] font-semibold">
							<CheckCircle size={14} />
							<span>Query Completed</span>
						</span>
						<span>
							Rows: <strong class="text-slate-950 dark:text-[#f1f5f9]">{result.rows ? result.rows.length : result.rows_affected}</strong>
						</span>
						{#if result.duration_ms !== undefined}
							<span class="flex items-center gap-1 text-slate-500 dark:text-[#64748b]">
								<Clock size={12} class="text-emerald-600 dark:text-[#d4ff32]" />
								<span>{result.duration_ms} ms</span>
							</span>
						{/if}
					</div>

					{#if result.rows && result.rows.length > 0}
						<button
							onclick={exportJSON}
							class="flex items-center gap-1.5 px-2.5 py-1 border border-slate-300 hover:border-slate-400 bg-white hover:bg-slate-100 text-slate-700 hover:text-slate-950 dark:border-[#252b3b] dark:hover:border-[#64748b] dark:bg-[#181c26] dark:text-[#94a3b8] dark:hover:text-[#f1f5f9] rounded-xs transition-colors cursor-pointer shadow-xs dark:shadow-none font-medium"
						>
							<Download size={12} />
							<span>Export JSON</span>
						</button>
					{/if}
				</div>

				<!-- Table of Results (if SELECT) -->
				{#if result.columns && result.rows}
					<div class="overflow-x-auto">
						<table class="w-full text-left text-xs sm:text-sm font-mono border-collapse">
							<thead class="bg-slate-50 dark:bg-[#151922] border-b border-slate-300 dark:border-[#252b3b] text-xs uppercase text-slate-600 dark:text-[#94a3b8]">
								<tr>
									{#each result.columns as col}
										<th class="py-2.5 px-3 font-medium whitespace-nowrap">{col}</th>
									{/each}
								</tr>
							</thead>
							<tbody class="divide-y divide-slate-200 dark:divide-[#252b3b]/60">
								{#if result.rows.length === 0}
									<tr>
										<td colspan={result.columns.length} class="py-8 text-center text-slate-500 dark:text-[#64748b]">
											Zero rows returned.
										</td>
									</tr>
								{:else}
									{#each result.rows as row, idx}
										<tr class="hover:bg-slate-50 dark:hover:bg-[#181c26]/60 transition-colors">
											{#each result.columns as col}
												<td class="py-2 px-3 max-w-sm truncate text-slate-900 dark:text-[#e6edf3] tabular-nums">
													{#if row[col] === null || row[col] === undefined}
														<span class="text-slate-400 dark:text-[#64748b] italic">NULL</span>
													{:else}
														{String(row[col])}
													{/if}
												</td>
											{/each}
										</tr>
									{/each}
								{/if}
							</tbody>
						</table>
					</div>
				{:else}
					<!-- Non-SELECT result banner -->
					<div class="p-6 text-center font-mono text-sm text-slate-900 dark:text-[#e6edf3]">
						Query executed successfully. Rows affected: <strong class="text-emerald-700 dark:text-[#d4ff32] font-bold">{result.rows_affected}</strong>
					</div>
				{/if}
			</div>
		{/if}
	{/if}
</div>

<!-- Interlock Warning Modal -->
<ServerInterlockModal
	open={showInterlock}
	actionTitle="Custom SQL Query Execution"
	onProceed={() => {
		showInterlock = false;
		execute();
	}}
	onCancel={() => {
		showInterlock = false;
	}}
/>
