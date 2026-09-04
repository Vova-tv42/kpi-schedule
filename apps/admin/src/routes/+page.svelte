<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { serverStatus } from '$lib/server-status-store.svelte';
	import {
		Database,
		Activity,
		Server,
		RefreshCw,
		ArrowUpRight,
		Clock,
		Zap,
		Layers
	} from 'lucide-svelte';

	interface TableMeta {
		name: string;
		row_count: number;
	}

	interface ActionItem {
		id: string;
		action_type: string;
		action_name: string;
		status_code: number;
		duration_ms: number;
		metadata: any;
		created_at: string;
	}

	const user = $derived(page.data.user);

	let tables = $state<TableMeta[]>([]);
	let recentActions = $state<ActionItem[]>([]);
	let totalActionsCount = $state<number>(0);
	let loading = $state(true);
	let tablesLoading = $state(false);
	let error = $state<string | null>(null);

	let totalRows = $derived(
		tables.reduce((acc, t) => acc + (t.row_count > 0 ? t.row_count : 0), 0)
	);

	async function loadOverviewData() {
		loading = true;
		error = null;

		// Load telemetry stream first (NeonDB - always awake, never wakes Fly VM)
		try {
			const res = await fetch('/api/actions?limit=15');
			if (res.ok) {
				const data = await res.json();
				recentActions = data.actions || [];
				totalActionsCount = data.total || 0;
			}
		} catch (err: any) {
			console.error('Failed to load recent telemetry actions:', err);
		}

		// Check if VM is awake or if user previously loaded tables
		if (serverStatus.data.status === 'awake' || tables.length > 0) {
			await loadTablesData();
		}

		loading = false;
	}

	async function loadTablesData() {
		tablesLoading = true;
		try {
			const res = await fetch('/api/main-db/tables');
			if (res.ok) {
				const data = await res.json();
				tables = data.tables || [];
				serverStatus.checkStatus();
			}
		} catch (err: any) {
			console.error('Failed to fetch tables:', err);
		} finally {
			tablesLoading = false;
		}
	}

	onMount(() => {
		loadOverviewData();
	});

	function formatTime(iso: string) {
		try {
			const d = new Date(iso);
			return d.toLocaleTimeString([], { hour12: false, hour: '2-digit', minute: '2-digit', second: '2-digit' });
		} catch {
			return iso;
		}
	}
</script>

<svelte:head>
	<title>KPI Schedule | Mission Control & Admin Telemetry</title>
</svelte:head>

<div class="space-y-6">
	<!-- Page Title & Operational Status -->
	<div class="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4 border-b border-slate-300 dark:border-[#252b3b] pb-4">
		<div>
			<div class="flex items-center gap-2 mb-1">
				<span class="w-2.5 h-2.5 rounded-full bg-emerald-600 dark:bg-[#10b981] animate-pulse"></span>
				<h1 class="font-display font-extrabold text-2xl sm:text-3xl text-slate-950 dark:text-[#f1f5f9] tracking-tight">
					TELEMETRY OVERVIEW
				</h1>
			</div>
		</div>

		<div class="flex items-center gap-2">
			{#if user}
				<div class="text-xs font-mono px-2.5 py-1.5 bg-white dark:bg-[#12151d] border border-slate-300 dark:border-[#252b3b] rounded-xs flex items-center gap-2 shadow-xs dark:shadow-none">
					<span class="text-slate-500 dark:text-[#64748b]">ROLE:</span>
					<span class="font-semibold text-slate-900 dark:text-[#f1f5f9] uppercase">{user.role}</span>
				</div>
			{/if}
			<button
				onclick={loadOverviewData}
				disabled={loading}
				class="px-3 py-1.5 bg-white hover:bg-slate-100 text-slate-700 dark:bg-[#181c26] dark:hover:bg-[#252b3b] dark:text-[#f1f5f9] border border-slate-300 dark:border-[#252b3b] text-xs sm:text-sm font-mono rounded-xs flex items-center gap-1.5 transition-colors cursor-pointer shadow-xs dark:shadow-none"
			>
				<RefreshCw size={13} class={loading ? 'animate-spin text-emerald-600 dark:text-[#d4ff32]' : ''} />
				<span>Refresh</span>
			</button>
		</div>
	</div>

	{#if error}
		<div class="p-3.5 bg-red-100 border border-red-300 rounded-xs text-xs font-mono text-red-900 dark:bg-[#ef4444]/10 dark:border-[#ef4444]/30 dark:text-[#fca5a5]">
			{error}
		</div>
	{/if}

	<!-- Metric Cards Grid -->
	<div class="grid grid-cols-1 sm:grid-cols-3 gap-4">
		<!-- Card 1: Active Tables -->
		<div class="bg-white dark:bg-[#12151d] border border-slate-300 dark:border-[#252b3b] p-4 rounded-xs relative overflow-hidden shadow-xs dark:shadow-none">
			<div class="flex items-center justify-between text-slate-500 dark:text-[#94a3b8] mb-2">
				<span class="text-xs font-mono tracking-wider uppercase text-slate-500 dark:text-[#64748b]">Active Tables</span>
				<Database size={16} class="text-emerald-600 dark:text-[#d4ff32]" />
			</div>
			<div class="font-display font-bold text-3xl text-slate-950 dark:text-[#f1f5f9]">
				{#if tables.length > 0}
					{tables.length}
				{:else if serverStatus.data.status === 'sleeping'}
					<span class="text-amber-600 dark:text-amber-400 text-xl font-mono">STANDBY</span>
				{:else}
					—
				{/if}
			</div>
			<div class="text-[11px] font-mono text-slate-500 dark:text-[#64748b] mt-1">
				SQLite Storage Schema
			</div>
		</div>

		<!-- Card 2: Total Records -->
		<div class="bg-white dark:bg-[#12151d] border border-slate-300 dark:border-[#252b3b] p-4 rounded-xs relative overflow-hidden shadow-xs dark:shadow-none">
			<div class="flex items-center justify-between text-slate-500 dark:text-[#94a3b8] mb-2">
				<span class="text-xs font-mono tracking-wider uppercase text-slate-500 dark:text-[#64748b]">Indexed Records</span>
				<Server size={16} class="text-emerald-600 dark:text-[#10b981]" />
			</div>
			<div class="font-display font-bold text-3xl text-slate-950 dark:text-[#f1f5f9]">
				{#if tables.length > 0}
					{totalRows.toLocaleString()}
				{:else if serverStatus.data.status === 'sleeping'}
					<span class="text-amber-600 dark:text-amber-400 text-xl font-mono">STANDBY</span>
				{:else}
					—
				{/if}
			</div>
			<div class="text-[11px] font-mono text-slate-500 dark:text-[#64748b] mt-1">
				Across all tables
			</div>
		</div>

		<!-- Card 3: Total Telemetry Events -->
		<div class="bg-white dark:bg-[#12151d] border border-slate-300 dark:border-[#252b3b] p-4 rounded-xs relative overflow-hidden shadow-xs dark:shadow-none">
			<div class="flex items-center justify-between text-slate-500 dark:text-[#94a3b8] mb-2">
				<span class="text-xs font-mono tracking-wider uppercase text-slate-500 dark:text-[#64748b]">Total Telemetry Events</span>
				<Activity size={16} class="text-sky-600 dark:text-[#06b6d4]" />
			</div>
			<div class="font-display font-bold text-3xl text-slate-950 dark:text-[#f1f5f9]">
				{totalActionsCount.toLocaleString()}
			</div>
		</div>
	</div>

	<!-- Two-column Section: Tables breakdown + Recent Actions Feed -->
	<div class="grid grid-cols-1 lg:grid-cols-2 gap-6">
		<!-- Left: Database Tables Summary -->
		<div class="bg-white dark:bg-[#12151d] border border-slate-300 dark:border-[#252b3b] rounded-xs flex flex-col shadow-xs dark:shadow-none">
			<div class="px-4 py-3 border-b border-slate-300 dark:border-[#252b3b] flex items-center justify-between bg-slate-50 dark:bg-[#151922]">
				<div class="flex items-center gap-2">
					<Database size={16} class="text-emerald-600 dark:text-[#d4ff32]" />
					<h3 class="font-display font-semibold text-base text-slate-950 dark:text-[#f1f5f9]">
						Database Tables
					</h3>
				</div>
				<a
					href="/database"
					class="text-xs font-mono text-emerald-700 hover:text-emerald-900 dark:text-[#d4ff32] dark:hover:underline flex items-center gap-1 font-medium"
				>
					<span>Open Table Explorer</span>
					<ArrowUpRight size={14} />
				</a>
			</div>

			<div class="p-3 overflow-y-auto max-h-[380px] divide-y divide-slate-200 dark:divide-[#252b3b]/50 font-mono text-xs sm:text-sm">
				{#if tablesLoading}
					<div class="p-6 text-center text-slate-500 dark:text-[#64748b]">
						<RefreshCw size={18} class="animate-spin text-emerald-600 dark:text-[#d4ff32] mx-auto mb-2" />
						Loading table statistics...
					</div>
				{:else if tables.length === 0}
					<div class="p-6 text-center space-y-3">
						{#if serverStatus.data.status === 'sleeping'}
							<p class="text-xs text-amber-700 dark:text-amber-400 font-mono font-medium">
								Main server is currently in Scale-to-Zero standby.
							</p>
							<button
								onclick={loadTablesData}
								class="px-3 py-1.5 bg-[#ccf600] hover:bg-[#bde800] text-black font-bold text-xs uppercase tracking-wider rounded-xs border border-black shadow-[2px_2px_0px_#090d16] dark:border-transparent dark:bg-[#d4ff32] dark:hover:bg-[#e2f952] dark:shadow-none transition-colors cursor-pointer"
							>
								Wake Server & Load Tables
							</button>
						{:else}
							<p class="text-slate-500 dark:text-[#64748b]">No database tables found</p>
						{/if}
					</div>
				{:else}
					{#each tables as table}
						<div class="py-2.5 px-2 flex items-center justify-between hover:bg-slate-50 dark:hover:bg-[#181c26]/60 rounded-xs transition-colors">
							<span class="font-medium text-slate-950 dark:text-[#f1f5f9]">{table.name}</span>
							<div class="flex items-center gap-3">
								<span class="text-slate-500 dark:text-[#94a3b8] text-xs">
									{table.row_count < 0 ? 'N/A' : `${table.row_count.toLocaleString()} rows`}
								</span>
								<a
									href={`/database/${table.name}`}
									class="text-xs text-slate-600 hover:text-slate-950 hover:bg-slate-200 border border-slate-300 px-2.5 py-0.5 rounded-xs bg-slate-100 dark:text-[#64748b] dark:hover:text-[#d4ff32] dark:border-[#252b3b] dark:bg-[#0a0b0e] transition-colors"
								>
									View
								</a>
							</div>
						</div>
					{/each}
				{/if}
			</div>
		</div>

		<!-- Right: Recent Activity Stream Preview -->
		<div class="bg-white dark:bg-[#12151d] border border-slate-300 dark:border-[#252b3b] rounded-xs flex flex-col shadow-xs dark:shadow-none">
			<div class="px-4 py-3 border-b border-slate-300 dark:border-[#252b3b] flex items-center justify-between bg-slate-50 dark:bg-[#151922]">
				<div class="flex items-center gap-2">
					<Activity size={16} class="text-sky-600 dark:text-[#06b6d4]" />
					<h3 class="font-display font-semibold text-base text-slate-950 dark:text-[#f1f5f9]">
						Anonymous Action Stream (Last {recentActions.length})
					</h3>
				</div>
				<a
					href="/actions"
					class="text-xs font-mono text-sky-700 hover:text-sky-900 dark:text-[#06b6d4] dark:hover:underline flex items-center gap-1 font-medium"
				>
					<span>View Full Stream</span>
					<ArrowUpRight size={14} />
				</a>
			</div>

			<div class="p-3 overflow-y-auto max-h-[380px] divide-y divide-slate-200 dark:divide-[#252b3b]/50 font-mono text-xs sm:text-sm">
				{#if loading && recentActions.length === 0}
					<div class="p-6 text-center text-slate-500 dark:text-[#64748b]">
						<RefreshCw size={18} class="animate-spin text-sky-600 dark:text-[#06b6d4] mx-auto mb-2" />
						Loading recent action events...
					</div>
				{:else if recentActions.length === 0}
					<div class="p-6 text-center text-slate-500 dark:text-[#64748b]">No recent actions recorded</div>
				{:else}
					{#each recentActions as act}
						<div class="py-2.5 px-2 flex items-center justify-between hover:bg-slate-50 dark:hover:bg-[#181c26]/60 rounded-xs transition-colors gap-2">
							<div class="flex items-center gap-2 min-w-0">
								<span class="text-xs px-1.5 py-0.5 rounded-xs font-bold {act.status_code >= 200 && act.status_code < 300 ? 'bg-emerald-100 text-emerald-900 dark:bg-[#10b981]/15 dark:text-[#10b981]' : act.status_code >= 400 ? 'bg-red-100 text-red-900 dark:bg-[#ef4444]/15 dark:text-[#ef4444]' : 'bg-amber-100 text-amber-900 dark:bg-[#f59e0b]/15 dark:text-[#f59e0b]'}">
									{act.status_code}
								</span>
								<span class="text-slate-950 dark:text-[#f1f5f9] truncate text-xs font-medium">{act.action_name}</span>
								<span class="text-[10px] text-slate-500 dark:text-[#64748b] hidden sm:inline px-1 py-0.2 rounded-xs bg-slate-100 dark:bg-[#0a0b0e] border border-slate-200 dark:border-[#252b3b]">
									{act.action_type}
								</span>
							</div>

							<div class="flex items-center gap-2 shrink-0 text-xs text-slate-500 dark:text-[#64748b]">
								<span class="px-1.5 py-0.5 rounded-xs bg-slate-100 dark:bg-[#181c26] text-slate-700 dark:text-[#94a3b8]">{act.duration_ms}ms</span>
								<span>{formatTime(act.created_at)}</span>
							</div>
						</div>
					{/each}
				{/if}
			</div>
		</div>
	</div>
</div>
