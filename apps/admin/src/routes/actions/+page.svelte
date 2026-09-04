<script lang="ts">
	import { onMount } from 'svelte';
	import { Activity, RefreshCw, Search, Filter, Clock, CheckCircle, AlertCircle, Info, ChevronLeft, ChevronRight, Zap } from 'lucide-svelte';

	interface ActionItem {
		id: string;
		action_type: string;
		action_name: string;
		status_code: number;
		duration_ms: number;
		metadata: any;
		created_at: string;
	}

	let actions = $state<ActionItem[]>([]);
	let total = $state<number>(0);
	let limit = $state<number>(50);
	let offset = $state<number>(0);
	let isLoading = $state<boolean>(true);
	let autoRefresh = $state<boolean>(true);
	let autoRefreshTimer: any = null;

	// Filters
	let filterType = $state<string>('');
	let filterStatus = $state<string>('');
	let searchQuery = $state<string>('');
	let selectedMetadata = $state<any | null>(null);

	async function loadActions() {
		isLoading = true;
		try {
			const params = new URLSearchParams({
				limit: String(limit),
				offset: String(offset)
			});
			if (filterType) params.set('action_type', filterType);
			if (filterStatus) params.set('status', filterStatus);
			if (searchQuery) params.set('search', searchQuery);

			const res = await fetch(`/api/actions?${params.toString()}`);
			if (res.ok) {
				const data = await res.json();
				actions = data.actions || [];
				total = data.total || 0;
			}
		} catch (err) {
			console.error('Failed to load actions:', err);
		} finally {
			isLoading = false;
		}
	}

	function handleSearch() {
		offset = 0;
		loadActions();
	}

	function setFilterType(type: string) {
		filterType = type;
		offset = 0;
		loadActions();
	}

	function setFilterStatus(st: string) {
		filterStatus = st;
		offset = 0;
		loadActions();
	}

	function setupAutoRefresh() {
		if (autoRefreshTimer) clearInterval(autoRefreshTimer);
		if (autoRefresh) {
			autoRefreshTimer = setInterval(() => {
				loadActions();
			}, 8000);
		}
	}

	$effect(() => {
		setupAutoRefresh();
		return () => {
			if (autoRefreshTimer) clearInterval(autoRefreshTimer);
		};
	});

	onMount(() => {
		loadActions();
	});

	// Derived metrics
	const successCount = $derived(actions.filter((a) => a.status_code >= 200 && a.status_code < 300).length);
	const successRate = $derived(actions.length > 0 ? Math.round((successCount / actions.length) * 100) : 100);
	const avgDuration = $derived(
		actions.length > 0 ? Math.round(actions.reduce((acc, a) => acc + (Number(a.duration_ms) || 0), 0) / actions.length) : 0
	);

	function formatRelativeTime(dateStr: string) {
		try {
			const diff = (Date.now() - new Date(dateStr).getTime()) / 1000;
			if (diff < 60) return `${Math.floor(diff)}s ago`;
			if (diff < 3600) return `${Math.floor(diff / 60)}m ago`;
			if (diff < 86400) return `${Math.floor(diff / 3600)}h ago`;
			return `${Math.floor(diff / 86400)}d ago`;
		} catch {
			return dateStr;
		}
	}
</script>

<svelte:head>
	<title>KPI Console | Recent Actions</title>
</svelte:head>

<div class="space-y-6">
	<!-- Top Metric Instrumentation Cards -->
	<div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
		<div class="border border-[#1e293b] bg-[#0f141d] p-4 relative">
			<div class="text-xs font-mono uppercase tracking-widest text-slate-400">Total Telemetry Events</div>
			<div class="text-3xl font-bold font-mono text-white mt-1 tabular-nums">{total}</div>
			<div class="text-xs font-mono text-slate-400 mt-1">NeonDB Retention Store</div>
			<div class="absolute top-2 right-2 text-slate-700">
				<Activity class="w-4 h-4" />
			</div>
		</div>

		<div class="border border-[#1e293b] bg-[#0f141d] p-4 relative">
			<div class="text-xs font-mono uppercase tracking-widest text-slate-400">Success Rate</div>
			<div class="text-3xl font-bold font-mono {successRate >= 95 ? 'text-emerald-400' : 'text-amber-400'} mt-1 tabular-nums">
				{successRate}%
			</div>
			<div class="text-xs font-mono text-slate-400 mt-1">{successCount} of {actions.length} recent OK</div>
			<div class="absolute top-2 right-2 text-slate-700">
				<CheckCircle class="w-4 h-4" />
			</div>
		</div>

		<div class="border border-[#1e293b] bg-[#0f141d] p-4 relative">
			<div class="text-xs font-mono uppercase tracking-widest text-slate-400">Avg Execution Latency</div>
			<div class="text-3xl font-bold font-mono text-cyan-400 mt-1 tabular-nums">{avgDuration} ms</div>
			<div class="text-xs font-mono text-slate-400 mt-1">Serverless telemetry</div>
			<div class="absolute top-2 right-2 text-slate-700">
				<Zap class="w-4 h-4" />
			</div>
		</div>

		<div class="border border-[#1e293b] bg-[#0f141d] p-4 relative">
			<div class="text-xs font-mono uppercase tracking-widest text-slate-400">Telemetry Stream</div>
			<div class="flex items-center gap-2 mt-2">
				<label class="relative inline-flex items-center cursor-pointer">
					<input
						type="checkbox"
						bind:checked={autoRefresh}
						class="sr-only peer"
					/>
					<div class="w-9 h-5 bg-slate-800 peer-focus:outline-none rounded-none peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-slate-300 after:border after:h-4 after:w-4 after:transition-all peer-checked:bg-cyan-600"></div>
					<span class="ml-2 text-sm font-mono text-slate-300">{autoRefresh ? '8s Live Sync' : 'Paused'}</span>
				</label>
			</div>
			<div class="text-xs font-mono text-slate-400 mt-1">Reads NeonDB (Zero main server impact)</div>
		</div>
	</div>

	<!-- Controls & Filters Bar -->
	<div class="border border-[#1e293b] bg-[#0f141d] p-4 space-y-3">
		<div class="flex flex-col md:flex-row items-stretch md:items-center justify-between gap-3">
			<!-- Action Type Filter Pills -->
			<div class="flex items-center gap-1.5 overflow-x-auto pb-1 md:pb-0">
				<button
					onclick={() => setFilterType('')}
					class="px-3 py-1.5 text-sm font-mono tracking-wide border transition-colors {filterType === ''
						? 'border-cyan-500 bg-cyan-950/30 text-cyan-300 font-bold'
						: 'border-[#1e293b] bg-[#141c28] text-slate-400 hover:text-white'}"
				>
					ALL
				</button>
				<button
					onclick={() => setFilterType('telegram_command')}
					class="px-3 py-1.5 text-sm font-mono tracking-wide border transition-colors {filterType === 'telegram_command'
						? 'border-cyan-500 bg-cyan-950/30 text-cyan-300 font-bold'
						: 'border-[#1e293b] bg-[#141c28] text-slate-400 hover:text-white'}"
				>
					TELEGRAM COMMANDS
				</button>
				<button
					onclick={() => setFilterType('telegram_callback')}
					class="px-3 py-1.5 text-sm font-mono tracking-wide border transition-colors {filterType === 'telegram_callback'
						? 'border-cyan-500 bg-cyan-950/30 text-cyan-300 font-bold'
						: 'border-[#1e293b] bg-[#141c28] text-slate-400 hover:text-white'}"
				>
					CALLBACKS
				</button>
				<button
					onclick={() => setFilterType('extension_sync')}
					class="px-3 py-1.5 text-sm font-mono tracking-wide border transition-colors {filterType === 'extension_sync'
						? 'border-cyan-500 bg-cyan-950/30 text-cyan-300 font-bold'
						: 'border-[#1e293b] bg-[#141c28] text-slate-400 hover:text-white'}"
				>
					EXTENSION SYNCS
				</button>
				<button
					onclick={() => setFilterType('cron_alert')}
					class="px-3 py-1.5 text-sm font-mono tracking-wide border transition-colors {filterType === 'cron_alert'
						? 'border-cyan-500 bg-cyan-950/30 text-cyan-300 font-bold'
						: 'border-[#1e293b] bg-[#141c28] text-slate-400 hover:text-white'}"
				>
					CRON ALERTS
				</button>
			</div>

			<!-- Status Filter Pills -->
			<div class="flex items-center gap-1.5">
				<button
					onclick={() => setFilterStatus('')}
					class="px-2.5 py-1 text-xs font-mono border {filterStatus === '' ? 'border-white text-white' : 'border-[#1e293b] text-slate-400'}"
				>
					ALL STATUS
				</button>
				<button
					onclick={() => setFilterStatus('200')}
					class="px-2.5 py-1 text-xs font-mono border {filterStatus === '200' ? 'border-emerald-500 text-emerald-400' : 'border-[#1e293b] text-slate-400'}"
				>
					200 OK
				</button>
				<button
					onclick={() => setFilterStatus('500')}
					class="px-2.5 py-1 text-xs font-mono border {filterStatus === '500' ? 'border-red-500 text-red-400' : 'border-[#1e293b] text-slate-400'}"
				>
					500 ERR
				</button>
			</div>
		</div>

		<!-- Search & Manual Refresh -->
		<div class="flex items-center gap-2 pt-2 border-t border-[#1e293b]">
			<div class="relative flex-1">
				<Search class="w-4 h-4 text-slate-500 absolute left-3 top-1/2 -translate-y-1/2" />
				<input
					type="text"
					bind:value={searchQuery}
					onkeydown={(e) => e.key === 'Enter' && handleSearch()}
					placeholder="Search action name (e.g. /today, sync, lesson_alerts)..."
					class="w-full pl-9 pr-4 py-2 bg-[#080b10] border border-[#1e293b] focus:border-cyan-500 text-sm font-mono text-white placeholder-slate-600 focus:outline-none"
				/>
			</div>
			<button
				onclick={handleSearch}
				class="px-4 py-2 border border-[#1e293b] bg-[#141c28] hover:bg-[#1a2536] text-slate-300 font-mono text-sm uppercase"
			>
				Filter
			</button>
			<button
				onclick={loadActions}
				disabled={isLoading}
				class="p-2 border border-[#1e293b] bg-[#141c28] hover:bg-[#1a2536] text-slate-300 transition-colors"
				title="Reload telemetry"
			>
				<RefreshCw class="w-3.5 h-3.5 {isLoading ? 'animate-spin text-cyan-400' : ''}" />
			</button>
		</div>
	</div>

	<!-- Actions Table -->
	<div class="border border-[#1e293b] bg-[#0f141d] overflow-hidden">
		<div class="overflow-x-auto">
			<table class="w-full text-left text-sm font-mono">
				<thead class="bg-[#0a0e14] border-b border-[#1e293b] text-xs uppercase text-slate-400 tracking-wider">
					<tr>
						<th class="py-2.5 px-4 font-medium">Timestamp</th>
						<th class="py-2.5 px-4 font-medium">Category</th>
						<th class="py-2.5 px-4 font-medium">Action Name</th>
						<th class="py-2.5 px-4 font-medium">Status</th>
						<th class="py-2.5 px-4 font-medium">Duration</th>
						<th class="py-2.5 px-4 font-medium text-right">Details</th>
					</tr>
				</thead>
				<tbody class="divide-y divide-[#1e293b]">
					{#if isLoading && actions.length === 0}
						<tr>
							<td colspan="6" class="py-8 text-center text-slate-500 font-mono">
								<RefreshCw class="w-5 h-5 animate-spin mx-auto mb-2 text-cyan-500" />
								Querying NeonDB telemetry stream...
							</td>
						</tr>
					{:else if actions.length === 0}
						<tr>
							<td colspan="6" class="py-8 text-center text-slate-500 font-mono">
								No action events recorded matching current filter parameters.
							</td>
						</tr>
					{:else}
						{#each actions as action (action.id)}
							<tr class="hover:bg-[#141c28] transition-colors">
								<td class="py-3 px-4 text-slate-400 whitespace-nowrap">
									<div class="text-white font-medium">{formatRelativeTime(action.created_at)}</div>
									<div class="text-xs text-slate-500">{new Date(action.created_at).toLocaleTimeString()}</div>
								</td>

								<td class="py-3 px-4 whitespace-nowrap">
									<span
										class="px-2.5 py-0.5 text-xs border {action.action_type === 'telegram_command'
											? 'border-blue-500/40 bg-blue-950/20 text-blue-400'
											: action.action_type === 'extension_sync'
												? 'border-emerald-500/40 bg-emerald-950/20 text-emerald-400'
												: action.action_type === 'cron_alert'
													? 'border-purple-500/40 bg-purple-950/20 text-purple-400'
													: 'border-slate-700 bg-slate-800 text-slate-400'}"
									>
										{action.action_type}
									</span>
								</td>

								<td class="py-3 px-4 font-semibold text-white">
									<code class="text-cyan-300">{action.action_name}</code>
								</td>

								<td class="py-3 px-4 whitespace-nowrap">
									<span
										class="px-2.5 py-0.5 text-xs font-bold border {action.status_code >= 200 && action.status_code < 300
											? 'border-emerald-500/50 bg-emerald-950/20 text-emerald-400'
											: action.status_code >= 400 && action.status_code < 500
												? 'border-yellow-500/50 bg-yellow-950/20 text-yellow-400'
												: 'border-red-500/50 bg-red-950/20 text-red-400'}"
									>
										{action.status_code}
									</span>
								</td>

								<td class="py-3 px-4 text-slate-300 tabular-nums whitespace-nowrap">
									{action.duration_ms} ms
								</td>

								<td class="py-3 px-4 text-right whitespace-nowrap">
									{#if action.metadata}
										<button
											onclick={() => (selectedMetadata = action.metadata)}
											class="p-1 border border-[#1e293b] hover:border-cyan-500 bg-[#141c28] text-slate-400 hover:text-cyan-300 transition-colors"
											title="Inspect Anonymous Payload"
										>
											<Info class="w-4 h-4" />
										</button>
									{:else}
										<span class="text-slate-600">—</span>
									{/if}
								</td>
							</tr>
						{/each}
					{/if}
				</tbody>
			</table>
		</div>

		<!-- Pagination Footer -->
		<div class="border-t border-[#1e293b] bg-[#0a0e14] px-4 py-3 flex items-center justify-between text-sm font-mono text-slate-400">
			<div>
				Showing <span class="text-white font-medium">{actions.length}</span> of <span class="text-white font-medium">{total}</span> actions
			</div>

			<div class="flex items-center gap-2">
				<button
					onclick={() => {
						if (offset >= limit) {
							offset -= limit;
							loadActions();
						}
					}}
					disabled={offset === 0}
					class="p-1.5 border border-[#1e293b] hover:border-slate-600 disabled:opacity-40 disabled:cursor-not-allowed text-slate-300"
				>
					<ChevronLeft class="w-4 h-4" />
				</button>

				<span>Offset {offset}</span>

				<button
					onclick={() => {
						if (offset + limit < total) {
							offset += limit;
							loadActions();
						}
					}}
					disabled={offset + limit >= total}
					class="p-1.5 border border-[#1e293b] hover:border-slate-600 disabled:opacity-40 disabled:cursor-not-allowed text-slate-300"
				>
					<ChevronRight class="w-4 h-4" />
				</button>
			</div>
		</div>
	</div>
</div>

<!-- Metadata Inspector Modal -->
{#if selectedMetadata}
	<div class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/80 backdrop-blur-xs">
		<div class="w-full max-w-md border border-[#1e293b] bg-[#0f141d] p-5 shadow-2xl space-y-4">
			<div class="flex items-center justify-between border-b border-[#1e293b] pb-3">
				<div class="text-sm font-mono uppercase tracking-widest text-cyan-400">[Anonymous Metadata]</div>
				<button onclick={() => (selectedMetadata = null)} class="text-slate-500 hover:text-white">✕</button>
			</div>
			<div class="text-sm font-mono bg-[#080b10] border border-[#1e293b] p-3 text-slate-300 overflow-x-auto max-h-60">
				<pre>{JSON.stringify(selectedMetadata, null, 2)}</pre>
			</div>
			<div class="text-xs font-mono text-slate-500">
				Note: All user identifiers (IDs, names, tokens) are strictly scrubbed to guarantee user anonymity.
			</div>
		</div>
	</div>
{/if}
