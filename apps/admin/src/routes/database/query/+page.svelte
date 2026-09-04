<script lang="ts">
	import { page } from '$app/state';
	import { serverStatus } from '$lib/server-status-store.svelte';
	import ServerInterlockModal from '$lib/components/ServerInterlockModal.svelte';
	import { Terminal, Play, RefreshCw, Clock, CheckCircle, AlertTriangle, Copy, Trash2 } from 'lucide-svelte';

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
				// Main server has awakened
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
</script>

<svelte:head>
	<title>KPI Console | SQL Console</title>
</svelte:head>

<div class="space-y-5">
	<!-- Title Bar -->
	<div class="border-b border-[#1e293b] pb-4">
		<div class="text-xs font-mono uppercase tracking-widest text-cyan-400">[Direct Query Terminal]</div>
		<h1 class="text-2xl font-bold font-mono text-white tracking-tight uppercase">Custom SQL Console</h1>
		<p class="text-sm font-mono text-slate-400 mt-1">Direct query runner against live SQLite instance on Fly.io volume</p>
	</div>

	{#if user?.role === 'read-only'}
		<div class="border border-amber-500/40 bg-amber-950/20 p-6 text-sm font-mono text-amber-200 space-y-2">
			<div class="flex items-center gap-2 font-bold text-amber-400 uppercase">
				<AlertTriangle class="w-5 h-5" />
				<span>[Access Restriction]</span>
			</div>
			<p>Custom SQL execution is disabled for accounts with the <strong>read-only</strong> role to protect production state.</p>
			<p class="text-slate-400">Contact the superadmin ({page.data.user.email}) to request a role elevation to read-write.</p>
		</div>
	{:else}
		<!-- Query Editor & Presets Card -->
		<div class="border border-[#1e293b] bg-[#0f141d] p-5 space-y-3">
			<!-- Presets Bar -->
			<div class="flex items-center justify-between gap-2 overflow-x-auto pb-1 text-sm font-mono">
				<div class="flex items-center gap-1.5">
					<span class="text-slate-500 text-xs uppercase mr-1">Presets:</span>
					{#each presets as p}
						<button
							onclick={() => (query = p.sql)}
							class="px-2.5 py-1 text-xs border border-[#1e293b] bg-[#141c28] hover:bg-[#1c2738] text-slate-300 hover:text-white transition-colors"
						>
							{p.label}
						</button>
					{/each}
				</div>

				<button
					onclick={() => (query = '')}
					class="p-1 text-slate-500 hover:text-red-400 text-sm"
					title="Clear editor"
				>
					<Trash2 class="w-4 h-4" />
				</button>
			</div>

			<!-- SQL Editor Area -->
			<div class="relative">
				<textarea
					bind:value={query}
					onkeydown={handleKeyDown}
					rows="5"
					placeholder="Enter SQL statement (e.g. SELECT * FROM users;)"
					class="w-full p-3 bg-[#080b10] border border-[#1e293b] focus:border-cyan-500 text-sm font-mono text-cyan-200 focus:outline-none resize-y leading-relaxed"
				></textarea>
			</div>

			<!-- Action Bar -->
			<div class="flex items-center justify-between pt-1">
				<div class="text-xs font-mono text-slate-500 hidden sm:block">
					Tip: Press <kbd class="px-1.5 py-0.5 border border-slate-700 bg-slate-800 text-slate-300">Ctrl</kbd> + <kbd class="px-1.5 py-0.5 border border-slate-700 bg-slate-800 text-slate-300">Enter</kbd> to execute
				</div>

				<button
					onclick={runQuery}
					disabled={isExecuting || !query.trim()}
					class="flex items-center gap-2 px-5 py-2 border border-cyan-500 bg-cyan-600 hover:bg-cyan-500 text-black text-sm font-mono font-bold uppercase tracking-wider transition-colors disabled:opacity-40"
				>
					<Play class="w-4 h-4 fill-current" />
					<span>{isExecuting ? 'Executing...' : 'Run Query'}</span>
				</button>
			</div>
		</div>

		<!-- Error Banner -->
		{#if error}
			<div class="p-4 border border-red-500/40 bg-red-950/20 text-red-200 text-sm font-mono space-y-1">
				<div class="font-bold text-red-400 uppercase">[SQL Execution Error]</div>
				<div class="font-mono text-sm">{error}</div>
			</div>
		{/if}

		<!-- Results Viewport -->
		{#if result}
			<div class="border border-[#1e293b] bg-[#0f141d] overflow-hidden space-y-2">
				<!-- Metrics Bar -->
				<div class="bg-[#0a0e14] border-b border-[#1e293b] px-4 py-2.5 flex items-center justify-between text-sm font-mono text-slate-400">
					<div class="flex items-center gap-4">
						<span class="flex items-center gap-1.5 text-emerald-400 font-semibold">
							<CheckCircle class="w-4 h-4" />
							<span>Query Completed</span>
						</span>
						<span>
							Rows: <strong class="text-white">{result.rows ? result.rows.length : result.rows_affected}</strong>
						</span>
					</div>

					{#if result.duration_ms !== undefined}
						<span class="flex items-center gap-1 text-slate-400">
							<Clock class="w-3.5 h-3.5 text-cyan-400" />
							<span>{result.duration_ms} ms</span>
						</span>
					{/if}
				</div>

				<!-- Table of Results (if SELECT) -->
				{#if result.columns && result.rows}
					<div class="overflow-x-auto">
						<table class="w-full text-left text-sm font-mono">
							<thead class="bg-[#0e131b] border-b border-[#1e293b] text-xs uppercase text-slate-400">
								<tr>
									{#each result.columns as col}
										<th class="py-2.5 px-3 font-medium whitespace-nowrap">{col}</th>
									{/each}
								</tr>
							</thead>
							<tbody class="divide-y divide-[#1e293b]">
								{#if result.rows.length === 0}
									<tr>
										<td colspan={result.columns.length} class="py-6 text-center text-slate-500">
											Zero rows returned.
										</td>
									</tr>
								{:else}
									{#each result.rows as row, idx}
										<tr class="hover:bg-[#141c28] transition-colors">
											{#each result.columns as col}
												<td class="py-2 px-3 max-w-sm truncate text-slate-300 tabular-nums">
													{#if row[col] === null || row[col] === undefined}
														<span class="text-slate-600 italic">NULL</span>
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
					<div class="p-6 text-center font-mono text-sm text-slate-300">
						Query executed successfully. Rows affected: <strong class="text-cyan-400">{result.rows_affected}</strong>
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
