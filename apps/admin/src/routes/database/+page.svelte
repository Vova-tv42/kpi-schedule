<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { serverStatus } from '$lib/server-status-store.svelte';
	import ServerInterlockModal from '$lib/components/ServerInterlockModal.svelte';
	import { Database, RefreshCw, ArrowRight, ShieldAlert, Zap, Layers } from 'lucide-svelte';

	interface TableMeta {
		name: string;
		row_count: number;
	}

	let tables = $state<TableMeta[]>([]);
	let isLoading = $state<boolean>(false);
	let error = $state<string | null>(null);
	let showInterlock = $state<boolean>(false);
	let hasAwakened = $state<boolean>(false);

	async function fetchTables() {
		isLoading = true;
		error = null;
		try {
			const res = await fetch('/api/main-db/tables');
			if (res.ok) {
				const data = await res.json();
				tables = data.tables || [];
				// Main server is now awake
				serverStatus.checkStatus();
			} else {
				const errData = await res.json().catch(() => null);
				error = errData?.message || `Server returned HTTP ${res.status}`;
			}
		} catch (err: any) {
			error = err?.message || 'Failed to fetch database tables';
		} finally {
			isLoading = false;
		}
	}

	function attemptLoad() {
		if (serverStatus.data.status === 'sleeping' && !hasAwakened) {
			showInterlock = true;
			return;
		}
		fetchTables();
	}

	function handleProceedWake() {
		showInterlock = false;
		hasAwakened = true;
		fetchTables();
	}

	function handleCancelWake() {
		showInterlock = false;
		goto('/actions');
	}

	onMount(() => {
		if (serverStatus.data.status === 'loading') {
			serverStatus.checkStatus().then(() => {
				attemptLoad();
			});
		} else {
			attemptLoad();
		}
	});
</script>

<svelte:head>
	<title>KPI Schedule // Database Tables</title>
</svelte:head>

<div class="space-y-6">
	<!-- Top Bar -->
	<div class="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4 border-b border-[#252b3b] pb-4">
		<div>
			<div class="flex items-center gap-2 mb-1">
				<Database size={22} class="text-[#d4ff32]" />
				<h1 class="font-display font-extrabold text-2xl sm:text-3xl text-[#f1f5f9] tracking-tight">
					DATABASE TABLES
				</h1>
			</div>
		</div>

		<button
			onclick={attemptLoad}
			disabled={isLoading}
			class="flex items-center gap-2 px-3.5 py-2 border border-[#252b3b] hover:border-[#64748b] bg-[#181c26] hover:bg-[#252b3b] text-xs sm:text-sm font-mono text-[#f1f5f9] rounded-xs transition-colors cursor-pointer"
		>
			<RefreshCw size={13} class={isLoading ? 'animate-spin text-[#d4ff32]' : ''} />
			<span>Refresh Schema</span>
		</button>
	</div>

	<!-- Server Sleeping Standby Interlock Banner -->
	{#if serverStatus.data.status === 'sleeping' && tables.length === 0 && !isLoading}
		<div class="border border-amber-500/40 bg-[#12151d] p-6 rounded-xs space-y-4">
			<div class="flex items-center gap-3">
				<div class="p-2 border border-amber-500/30 bg-amber-500/10 text-amber-400 rounded-xs">
					<ShieldAlert size={20} />
				</div>
				<div>
					<div class="text-[11px] font-mono uppercase tracking-widest text-amber-400 font-bold">Scale-to-Zero Guard</div>
					<h3 class="text-base sm:text-lg font-bold text-[#f1f5f9] font-display">Main Server is Currently Sleeping</h3>
				</div>
			</div>

			<p class="text-xs sm:text-sm font-mono text-[#94a3b8] leading-relaxed max-w-2xl">
				The Go server process has shut down after 15 minutes of idle time to conserve compute credits.
				Loading the live SQLite database requires sending an HTTP request through Fly Proxy to start the Firecracker VM (~500ms).
			</p>

			<div class="flex flex-wrap items-center gap-3 pt-2">
				<button
					onclick={handleProceedWake}
					class="flex items-center gap-2 px-4 py-2 border border-[#d4ff32]/50 bg-[#d4ff32] hover:bg-[#e2f952] text-black text-xs sm:text-sm font-mono font-bold uppercase tracking-wider rounded-xs transition-all shadow-[0_0_15px_rgba(212,255,50,0.2)] cursor-pointer"
				>
					<Zap size={14} class="fill-current" />
					<span>Wake Server & Load Tables</span>
				</button>
				<button
					onclick={handleCancelWake}
					class="px-3.5 py-2 border border-[#252b3b] bg-[#181c26] hover:bg-[#252b3b] text-[#94a3b8] font-mono text-xs sm:text-sm uppercase tracking-wider rounded-xs transition-colors cursor-pointer"
				>
					Stay in Standby (Return to Actions)
				</button>
			</div>
		</div>
	{/if}

	<!-- Error Message -->
	{#if error}
		<div class="p-3.5 border border-[#ef4444]/40 bg-[#ef4444]/10 text-[#fca5a5] text-xs font-mono rounded-xs">
			<div class="font-bold text-[#ef4444] uppercase mb-0.5">[Database Request Error]</div>
			<div>{error}</div>
		</div>
	{/if}

	<!-- Loading Spinner -->
	{#if isLoading}
		<div class="border border-[#252b3b] bg-[#12151d] p-12 text-center text-[#94a3b8] font-mono text-sm rounded-xs">
			<RefreshCw size={20} class="animate-spin mx-auto mb-3 text-[#d4ff32]" />
			Connecting to Go server & inspecting SQLite master schema...
		</div>
	{:else if tables.length > 0}
		<!-- Tables Grid -->
		<div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
			{#each tables as table (table.name)}
				<div class="border border-[#252b3b] bg-[#12151d] hover:border-[#d4ff32]/40 transition-all p-5 rounded-xs flex flex-col justify-between group">
					<div>
						<div class="flex items-center justify-between mb-3">
							<div class="flex items-center gap-2">
								<Database size={16} class="text-[#d4ff32]" />
								<span class="font-mono text-base font-bold text-[#f1f5f9] group-hover:text-[#d4ff32] transition-colors">
									{table.name}
								</span>
							</div>
							<span class="text-[10px] font-mono px-1.5 py-0.5 border border-[#252b3b] bg-[#0a0b0e] text-[#94a3b8] rounded-xs">
								TABLE
							</span>
						</div>

						<div class="flex items-center gap-2 text-xs font-mono text-[#94a3b8] py-2 border-t border-[#252b3b]/60">
							<Layers size={13} class="text-[#64748b]" />
							<span>
								{#if table.row_count < 0}
									Row count unavailable
								{:else}
									<strong class="text-[#f1f5f9] font-medium">{table.row_count.toLocaleString()}</strong> rows indexed
								{/if}
							</span>
						</div>
					</div>

					<div class="pt-4 mt-2 border-t border-[#252b3b]/60 flex items-center justify-between">
						<a
							href={`/database/${table.name}`}
							class="text-xs font-mono text-[#d4ff32] hover:underline flex items-center gap-1.5"
						>
							<span>Browse Rows</span>
							<ArrowRight size={13} />
						</a>

						<a
							href={`/database/query`}
							class="text-xs font-mono text-[#64748b] hover:text-[#f1f5f9] transition-colors"
						>
							Query
						</a>
					</div>
				</div>
			{/each}
		</div>
	{/if}
</div>

<ServerInterlockModal
	open={showInterlock}
	actionTitle="Load SQLite Tables & Schema"
	onProceed={handleProceedWake}
	onCancel={handleCancelWake}
/>
