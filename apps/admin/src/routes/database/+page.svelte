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
		// If server is explicitly sleeping and user hasn't confirmed wake yet, show interlock dialog!
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
		// Wait for status check if loading
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
	<title>KPI Console | Database Tables</title>
</svelte:head>

<div class="space-y-6">
	<!-- Top Bar -->
	<div class="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4 border-b border-[#1e293b] pb-4">
		<div>
			<div class="text-xs font-mono uppercase tracking-widest text-cyan-400">[Fly.io SQLite Persistent Storage]</div>
			<h1 class="text-2xl font-bold uppercase text-white font-sans mt-0.5">Database Tables</h1>
			<p class="text-sm font-mono text-slate-400 mt-1">Mounted on persistent NVMe volume at <code class="text-slate-300">/data/kpi.db</code></p>
		</div>

		<button
			onclick={attemptLoad}
			disabled={isLoading}
			class="flex items-center gap-2 px-4 py-2 border border-[#1e293b] hover:border-slate-600 bg-[#141c28] hover:bg-[#1a2536] text-sm font-mono text-slate-200 transition-colors"
		>
			<RefreshCw class="w-4 h-4 {isLoading ? 'animate-spin text-cyan-400' : ''}" />
			<span>Refresh Schema</span>
		</button>
	</div>

	<!-- Server Sleeping Standby Interlock Banner -->
	{#if serverStatus.data.status === 'sleeping' && tables.length === 0 && !isLoading}
		<div class="border border-amber-500/40 bg-amber-950/20 p-6 space-y-4">
			<div class="flex items-center gap-3">
				<div class="p-2 border border-amber-500/30 bg-amber-500/10 text-amber-400">
					<ShieldAlert class="w-6 h-6" />
				</div>
				<div>
					<div class="text-xs font-mono uppercase tracking-widest text-amber-500 font-bold">[Scale-to-Zero Guard]</div>
					<h3 class="text-lg font-bold uppercase text-white font-sans">Main Server is Currently Sleeping</h3>
				</div>
			</div>

			<p class="text-sm font-mono text-slate-300 leading-relaxed max-w-2xl">
				The Go server process has shut down after 15 minutes of idle time to conserve compute credits.
				Loading the live SQLite database requires sending an HTTP request through Fly Proxy to start the Firecracker VM (~500ms).
			</p>

			<div class="flex items-center gap-3 pt-2">
				<button
					onclick={handleProceedWake}
					class="flex items-center gap-2 px-4 py-2 border border-amber-500 bg-amber-500 hover:bg-amber-400 text-black text-sm font-mono font-bold uppercase tracking-wider transition-colors"
				>
					<Zap class="w-4 h-4 fill-current" />
					Wake Server & Load Tables
				</button>
				<button
					onclick={handleCancelWake}
					class="px-4 py-2 border border-[#1e293b] bg-[#141c28] hover:bg-[#1a2536] text-slate-300 font-mono text-sm uppercase transition-colors"
				>
					Stay in Standby (Return to Actions)
				</button>
			</div>
		</div>
	{/if}

	<!-- Error Message -->
	{#if error}
		<div class="p-4 border border-red-500/40 bg-red-950/20 text-red-200 text-sm font-mono">
			<div class="font-bold text-red-400 uppercase">[Database Request Error]</div>
			<div class="mt-1">{error}</div>
		</div>
	{/if}

	<!-- Loading Spinner -->
	{#if isLoading}
		<div class="border border-[#1e293b] bg-[#0f141d] p-12 text-center text-slate-400 font-mono text-sm">
			<RefreshCw class="w-6 h-6 animate-spin mx-auto mb-3 text-cyan-400" />
			Connecting to Go server & inspecting SQLite master...
		</div>
	{:else if tables.length > 0}
		<!-- Tables Grid -->
		<div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
			{#each tables as table (table.name)}
				<div class="border border-[#1e293b] bg-[#0f141d] hover:border-slate-600 transition-all p-5 flex flex-col justify-between group">
					<div>
						<div class="flex items-center justify-between">
							<div class="flex items-center gap-2">
								<Database class="w-4 h-4 text-cyan-400" />
								<span class="font-mono text-base font-bold text-white group-hover:text-cyan-300 transition-colors">
									{table.name}
								</span>
							</div>
							<span class="text-xs font-mono px-2 py-0.5 border border-slate-700/60 bg-[#141c28] text-slate-400">
								TABLE
							</span>
						</div>

						<div class="mt-4 flex items-baseline gap-2">
							<span class="text-3xl font-bold font-mono text-white tabular-nums">{table.row_count}</span>
							<span class="text-sm font-mono text-slate-400">rows</span>
						</div>
					</div>

					<div class="mt-6 pt-4 border-t border-[#1e293b] flex items-center justify-end">
						<a
							href="/database/{table.name}"
							class="flex items-center gap-1 text-sm font-mono text-cyan-400 hover:text-cyan-300 font-medium group-hover:translate-x-0.5 transition-all"
						>
							<span>Browse Rows</span>
							<ArrowRight class="w-4 h-4" />
						</a>
					</div>
				</div>
			{/each}
		</div>
	{/if}
</div>

<!-- Interlock Warning Modal -->
<ServerInterlockModal
	open={showInterlock}
	actionTitle="Load Database Tables from Main Server"
	onProceed={handleProceedWake}
	onCancel={handleCancelWake}
/>
