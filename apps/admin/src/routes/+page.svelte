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
		if (serverStatus.data.status !== 'sleeping' || tables.length > 0) {
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
	<title>KPI Schedule // Mission Control & Admin Telemetry</title>
</svelte:head>

<div class="space-y-6">
	<!-- Page Title & Operational Status -->
	<div class="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4 border-b border-[#252b3b] pb-4">
		<div>
			<div class="flex items-center gap-2 mb-1">
				<span class="w-2.5 h-2.5 rounded-full bg-[#10b981] animate-pulse"></span>
				<h1 class="font-display font-extrabold text-2xl sm:text-3xl text-[#f1f5f9] tracking-tight">
					TELEMETRY OVERVIEW
				</h1>
			</div>
		</div>

		<div class="flex items-center gap-2">
			{#if user}
				<div class="text-xs font-mono px-2.5 py-1.5 bg-[#12151d] border border-[#252b3b] rounded-xs flex items-center gap-2">
					<span class="text-[#64748b]">ROLE:</span>
					<span class="font-semibold text-[#f1f5f9] uppercase">{user.role}</span>
				</div>
			{/if}
			<button
				onclick={loadOverviewData}
				disabled={loading}
				class="px-3 py-1.5 bg-[#181c26] hover:bg-[#252b3b] text-[#f1f5f9] border border-[#252b3b] text-xs sm:text-sm font-mono rounded-xs flex items-center gap-1.5 transition-colors cursor-pointer"
			>
				<RefreshCw size={13} class={loading ? 'animate-spin text-[#d4ff32]' : ''} />
				<span>Refresh</span>
			</button>
		</div>
	</div>

	{#if error}
		<div class="p-3.5 bg-[#ef4444]/10 border border-[#ef4444]/30 rounded-xs text-xs font-mono text-[#fca5a5]">
			{error}
		</div>
	{/if}

	<!-- Metric Cards Grid -->
	<div class="grid grid-cols-1 sm:grid-cols-3 gap-4">
		<!-- Card 1: Active Tables -->
		<div class="bg-[#12151d] border border-[#252b3b] p-4 rounded-xs relative overflow-hidden">
			<div class="flex items-center justify-between text-[#94a3b8] mb-2">
				<span class="text-xs font-mono tracking-wider uppercase text-[#64748b]">Active Tables</span>
				<Database size={16} class="text-[#d4ff32]" />
			</div>
			<div class="font-display font-bold text-3xl text-[#f1f5f9]">
				{#if tables.length > 0}
					{tables.length}
				{:else if serverStatus.data.status === 'sleeping'}
					<span class="text-amber-400 text-xl font-mono">STANDBY</span>
				{:else}
					—
				{/if}
			</div>
			<div class="text-[11px] font-mono text-[#64748b] mt-1">
				SQLite Storage Schema
			</div>
		</div>

		<!-- Card 2: Total Records -->
		<div class="bg-[#12151d] border border-[#252b3b] p-4 rounded-xs relative overflow-hidden">
			<div class="flex items-center justify-between text-[#94a3b8] mb-2">
				<span class="text-xs font-mono tracking-wider uppercase text-[#64748b]">Indexed Records</span>
				<Server size={16} class="text-[#10b981]" />
			</div>
			<div class="font-display font-bold text-3xl text-[#f1f5f9]">
				{#if tables.length > 0}
					{totalRows.toLocaleString()}
				{:else if serverStatus.data.status === 'sleeping'}
					<span class="text-amber-400 text-xl font-mono">STANDBY</span>
				{:else}
					—
				{/if}
			</div>
			<div class="text-[11px] font-mono text-[#64748b] mt-1">
				Across all tables
			</div>
		</div>

		<!-- Card 3: Telemetry Stream -->
		<div class="bg-[#12151d] border border-[#252b3b] p-4 rounded-xs relative overflow-hidden">
			<div class="flex items-center justify-between text-[#94a3b8] mb-2">
				<span class="text-xs font-mono tracking-wider uppercase text-[#64748b]">Telemetry Stream</span>
				<Activity size={16} class="text-[#06b6d4]" />
			</div>
			<div class="font-display font-bold text-3xl text-[#f1f5f9]">
				{totalActionsCount.toLocaleString()}
			</div>
		</div>
	</div>

	<!-- Two-column Section: Tables breakdown + Recent Actions Feed -->
	<div class="grid grid-cols-1 lg:grid-cols-2 gap-6">
		<!-- Left: Database Tables Summary -->
		<div class="bg-[#12151d] border border-[#252b3b] rounded-xs flex flex-col">
			<div class="px-4 py-3 border-b border-[#252b3b] flex items-center justify-between bg-[#151922]">
				<div class="flex items-center gap-2">
					<Database size={16} class="text-[#d4ff32]" />
					<h3 class="font-display font-semibold text-base text-[#f1f5f9]">
						Database Tables
					</h3>
				</div>
				<a
					href="/database"
					class="text-xs font-mono text-[#d4ff32] hover:underline flex items-center gap-1"
				>
					<span>Open Table Explorer</span>
					<ArrowUpRight size={14} />
				</a>
			</div>

			<div class="p-3 overflow-y-auto max-h-[380px] divide-y divide-[#252b3b]/50 font-mono text-xs sm:text-sm">
				{#if tablesLoading}
					<div class="p-6 text-center text-[#64748b]">
						<RefreshCw size={18} class="animate-spin text-[#d4ff32] mx-auto mb-2" />
						Loading table statistics...
					</div>
				{:else if tables.length === 0}
					<div class="p-6 text-center space-y-3">
						{#if serverStatus.data.status === 'sleeping'}
							<p class="text-xs text-amber-400 font-mono">
								Main server is currently in Scale-to-Zero standby.
							</p>
							<button
								onclick={loadTablesData}
								class="px-3 py-1.5 bg-[#d4ff32] text-black font-bold text-xs uppercase tracking-wider rounded-xs hover:bg-[#e2f952] transition-colors"
							>
								Wake Server & Load Tables
							</button>
						{:else}
							<p class="text-[#64748b]">No database tables found</p>
						{/if}
					</div>
				{:else}
					{#each tables as table}
						<div class="py-2.5 px-2 flex items-center justify-between hover:bg-[#181c26]/60 rounded-xs transition-colors">
							<span class="font-medium text-[#f1f5f9]">{table.name}</span>
							<div class="flex items-center gap-3">
								<span class="text-[#94a3b8] text-xs">
									{table.row_count < 0 ? 'N/A' : `${table.row_count.toLocaleString()} rows`}
								</span>
								<a
									href={`/database/${table.name}`}
									class="text-xs text-[#64748b] hover:text-[#d4ff32] border border-[#252b3b] px-2.5 py-0.5 rounded-xs bg-[#0a0b0e] transition-colors"
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
		<div class="bg-[#12151d] border border-[#252b3b] rounded-xs flex flex-col">
			<div class="px-4 py-3 border-b border-[#252b3b] flex items-center justify-between bg-[#151922]">
				<div class="flex items-center gap-2">
					<Activity size={16} class="text-[#06b6d4]" />
					<h3 class="font-display font-semibold text-base text-[#f1f5f9]">
						Anonymous Action Stream (Last {recentActions.length})
					</h3>
				</div>
				<a
					href="/actions"
					class="text-xs font-mono text-[#06b6d4] hover:underline flex items-center gap-1"
				>
					<span>View Full Stream</span>
					<ArrowUpRight size={14} />
				</a>
			</div>

			<div class="p-3 overflow-y-auto max-h-[380px] divide-y divide-[#252b3b]/50 font-mono text-xs sm:text-sm">
				{#if loading && recentActions.length === 0}
					<div class="p-6 text-center text-[#64748b]">
						<RefreshCw size={18} class="animate-spin text-[#06b6d4] mx-auto mb-2" />
						Loading recent action events...
					</div>
				{:else if recentActions.length === 0}
					<div class="p-6 text-center text-[#64748b]">No recent actions recorded</div>
				{:else}
					{#each recentActions as act}
						<div class="py-2.5 px-2 flex items-center justify-between hover:bg-[#181c26]/60 rounded-xs transition-colors gap-2">
							<div class="flex items-center gap-2 min-w-0">
								<span class="text-xs px-1.5 py-0.5 rounded-xs font-bold {act.status_code >= 200 && act.status_code < 300 ? 'bg-[#10b981]/15 text-[#10b981]' : act.status_code >= 400 ? 'bg-[#ef4444]/15 text-[#ef4444]' : 'bg-[#f59e0b]/15 text-[#f59e0b]'}">
									{act.status_code}
								</span>
								<span class="text-[#f1f5f9] truncate text-xs font-medium">{act.action_name}</span>
								<span class="text-[10px] text-[#64748b] hidden sm:inline px-1 py-0.2 rounded-xs bg-[#0a0b0e] border border-[#252b3b]">
									{act.action_type}
								</span>
							</div>

							<div class="flex items-center gap-2 shrink-0 text-xs text-[#64748b]">
								<span class="px-1.5 py-0.5 rounded-xs bg-[#181c26] text-[#94a3b8]">{act.duration_ms}ms</span>
								<span>{formatTime(act.created_at)}</span>
							</div>
						</div>
					{/each}
				{/if}
			</div>
		</div>
	</div>
</div>
