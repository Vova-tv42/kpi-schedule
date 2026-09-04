<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import Modal from '$lib/components/Modal.svelte';
	import {
		Activity,
		RefreshCw,
		Search,
		Filter,
		Clock,
		Zap,
		AlertCircle,
		CheckCircle,
		Info,
		ChevronLeft,
		ChevronRight
	} from 'lucide-svelte';

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

	onDestroy(() => {
		if (autoRefreshTimer) clearInterval(autoRefreshTimer);
	});

	// Derived metrics
	const successCount = $derived(actions.filter((a) => a.status_code >= 200 && a.status_code < 300).length);
	const successRate = $derived(actions.length > 0 ? Math.round((successCount / actions.length) * 100) : 100);
	const avgDuration = $derived(
		actions.length > 0 ? Math.round(actions.reduce((acc, a) => acc + (Number(a.duration_ms) || 0), 0) / actions.length) : 0
	);
	const errorCount = $derived(actions.filter((a) => a.status_code >= 400).length);

	function formatTime(iso: string) {
		try {
			const d = new Date(iso);
			return d.toLocaleTimeString([], { hour12: false, hour: '2-digit', minute: '2-digit', second: '2-digit' });
		} catch {
			return iso;
		}
	}

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
	<title>KPI Schedule // Action Telemetry Stream</title>
</svelte:head>

<div class="space-y-6">
	<!-- Page Header -->
	<div class="flex flex-wrap items-center justify-between gap-4 border-b border-[#252b3b] pb-4">
		<div>
			<div class="flex items-center gap-2 mb-1">
				<Activity size={22} class="text-[#06b6d4]" />
				<h1 class="font-display font-extrabold text-2xl sm:text-3xl text-[#f1f5f9] tracking-tight">
					ANONYMOUS ACTION TELEMETRY
				</h1>
			</div>
		</div>

		<!-- Stream Mode & Reconnect Controls -->
		<div class="flex items-center gap-3">
			{#if autoRefresh}
				<span class="flex items-center gap-1.5 px-3 py-1.5 rounded-xs bg-[#10b981]/15 text-[#10b981] text-xs font-mono font-medium border border-[#10b981]/30">
					<span class="w-2 h-2 rounded-full bg-[#10b981] animate-pulse"></span>
					<span>8s LIVE SYNC</span>
				</span>
			{:else}
				<span class="flex items-center gap-1.5 px-3 py-1.5 rounded-xs bg-[#252b3b] text-[#94a3b8] text-xs font-mono font-medium">
					<span class="w-2 h-2 rounded-full bg-[#64748b]"></span>
					<span>PAUSED</span>
				</span>
			{/if}

			<button
				onclick={loadActions}
				disabled={isLoading}
				class="p-2 bg-[#181c26] hover:bg-[#252b3b] text-[#f1f5f9] border border-[#252b3b] rounded-xs text-xs sm:text-sm font-mono flex items-center gap-1.5 transition-colors cursor-pointer"
				title="Re-sync telemetry stream"
			>
				<RefreshCw size={13} class={isLoading ? 'animate-spin text-[#d4ff32]' : ''} />
				<span>Re-sync</span>
			</button>
		</div>
	</div>

	<!-- Telemetry Diagnostics Cards -->
	<div class="grid grid-cols-1 sm:grid-cols-3 gap-4">
		<div class="bg-[#12151d] border border-[#252b3b] p-4 rounded-xs">
			<span class="text-xs font-mono tracking-wider uppercase text-[#64748b] block mb-1">Total Telemetry Events</span>
			<div class="font-display font-bold text-3xl text-[#f1f5f9]">{total.toLocaleString()}</div>
		</div>

		<div class="bg-[#12151d] border border-[#252b3b] p-4 rounded-xs">
			<span class="text-xs font-mono tracking-wider uppercase text-[#64748b] block mb-1">Avg Execution Duration</span>
			<div class="font-display font-bold text-3xl text-[#10b981]">{avgDuration}ms</div>
		</div>

		<div class="bg-[#12151d] border border-[#252b3b] p-4 rounded-xs">
			<span class="text-xs font-mono tracking-wider uppercase text-[#64748b] block mb-1">Success Rate</span>
			<div class="flex items-center gap-2">
  			<div class="font-display font-bold text-3xl {successRate >= 95 ? 'text-[#10b981]' : 'text-amber-400'}">{successRate}%</div>
        {#if successCount > 0 && errorCount > 0}
     			<span class="text-sm font-mono text-[#64748b]">(<span class="text-[#10b981cc]">{successCount} OK</span> • <span class="text-amber-400/80">{errorCount} Errors</span>)</span>
   			{/if}
			</div>
		</div>
	</div>

	<!-- Filter Strip -->
	<div class="bg-[#12151d] border border-[#252b3b] p-3.5 rounded-xs space-y-3 font-mono text-xs sm:text-sm">
		<div class="flex flex-col md:flex-row items-stretch md:items-center justify-between gap-3">
			<!-- Action Type Filter Pills -->
			<div class="flex items-center gap-1.5 overflow-x-auto pb-1 md:pb-0">
				<button
					onclick={() => setFilterType('')}
					class="px-2.5 py-1 text-xs font-mono tracking-wider uppercase rounded-xs border transition-colors cursor-pointer {filterType === ''
						? 'bg-[#d4ff32]/10 text-[#d4ff32] font-semibold border-[#d4ff32]/30'
						: 'border-[#252b3b] bg-[#0a0b0e] text-[#94a3b8] hover:text-[#f1f5f9]'}"
				>
					ALL
				</button>
				<button
					onclick={() => setFilterType('telegram_command')}
					class="px-2.5 py-1 text-xs font-mono tracking-wider uppercase rounded-xs border transition-colors cursor-pointer whitespace-nowrap {filterType === 'telegram_command'
						? 'bg-[#d4ff32]/10 text-[#d4ff32] font-semibold border-[#d4ff32]/30'
						: 'border-[#252b3b] bg-[#0a0b0e] text-[#94a3b8] hover:text-[#f1f5f9]'}"
				>
					COMMANDS
				</button>
				<button
					onclick={() => setFilterType('telegram_callback')}
					class="px-2.5 py-1 text-xs font-mono tracking-wider uppercase rounded-xs border transition-colors cursor-pointer whitespace-nowrap {filterType === 'telegram_callback'
						? 'bg-[#d4ff32]/10 text-[#d4ff32] font-semibold border-[#d4ff32]/30'
						: 'border-[#252b3b] bg-[#0a0b0e] text-[#94a3b8] hover:text-[#f1f5f9]'}"
				>
					CALLBACKS
				</button>
				<button
					onclick={() => setFilterType('extension_sync')}
					class="px-2.5 py-1 text-xs font-mono tracking-wider uppercase rounded-xs border transition-colors cursor-pointer whitespace-nowrap {filterType === 'extension_sync'
						? 'bg-[#d4ff32]/10 text-[#d4ff32] font-semibold border-[#d4ff32]/30'
						: 'border-[#252b3b] bg-[#0a0b0e] text-[#94a3b8] hover:text-[#f1f5f9]'}"
				>
					SYNCS
				</button>
				<button
					onclick={() => setFilterType('cron_alert')}
					class="px-2.5 py-1 text-xs font-mono tracking-wider uppercase rounded-xs border transition-colors cursor-pointer whitespace-nowrap {filterType === 'cron_alert'
						? 'bg-[#d4ff32]/10 text-[#d4ff32] font-semibold border-[#d4ff32]/30'
						: 'border-[#252b3b] bg-[#0a0b0e] text-[#94a3b8] hover:text-[#f1f5f9]'}"
				>
					ALERTS
				</button>
				<button
					onclick={() => setFilterType('admin_action')}
					class="px-2.5 py-1 text-xs font-mono tracking-wider uppercase rounded-xs border transition-colors cursor-pointer whitespace-nowrap {filterType === 'admin_action'
						? 'bg-[#d4ff32]/10 text-[#d4ff32] font-semibold border-[#d4ff32]/30'
						: 'border-[#252b3b] bg-[#0a0b0e] text-[#94a3b8] hover:text-[#f1f5f9]'}"
				>
					ADMIN
				</button>
			</div>

			<!-- Status Filter Pills & Live Sync Toggle -->
			<div class="flex items-center gap-3">
				<div class="flex items-center gap-1.5">
					<button
						onclick={() => setFilterStatus('')}
						class="px-2 py-0.5 text-xs font-mono rounded-xs border transition-colors cursor-pointer {filterStatus === '' ? 'border-[#f1f5f9] text-[#f1f5f9] bg-[#181c26]' : 'border-[#252b3b] text-[#64748b]'}"
					>
						ALL
					</button>
					<button
						onclick={() => setFilterStatus('200')}
						class="px-2 py-0.5 text-xs font-mono rounded-xs border transition-colors cursor-pointer {filterStatus === '200' ? 'border-[#10b981]/50 bg-[#10b981]/15 text-[#10b981]' : 'border-[#252b3b] text-[#64748b]'}"
					>
						200 OK
					</button>
					<button
						onclick={() => setFilterStatus('500')}
						class="px-2 py-0.5 text-xs font-mono rounded-xs border transition-colors cursor-pointer {filterStatus === '500' ? 'border-[#ef4444]/50 bg-[#ef4444]/15 text-[#ef4444]' : 'border-[#252b3b] text-[#64748b]'}"
					>
						500 ERR
					</button>
				</div>

				<span class="text-[#252b3b]">|</span>

				<label class="flex items-center gap-2 cursor-pointer select-none text-xs text-[#94a3b8]">
					<input type="checkbox" bind:checked={autoRefresh} class="accent-[#d4ff32]" />
					<span>Auto Sync</span>
				</label>
			</div>
		</div>

		<!-- Search Bar -->
		<div class="flex items-center gap-2 pt-2 border-t border-[#252b3b]/60">
			<div class="relative flex-1">
				<Search size={14} class="absolute left-2.5 top-2.5 text-[#64748b]" />
				<input
					type="text"
					bind:value={searchQuery}
					onkeydown={(e) => e.key === 'Enter' && handleSearch()}
					placeholder="Search action name (e.g. /today, sync, lesson_alerts)..."
					class="w-full pl-8 pr-3 py-1.5 bg-[#0a0b0e] text-[#f1f5f9] border border-[#252b3b] rounded-xs text-xs sm:text-sm focus:outline-none focus:border-[#d4ff32]"
				/>
			</div>
			<button
				onclick={handleSearch}
				class="px-3.5 py-1.5 border border-[#252b3b] bg-[#181c26] hover:bg-[#252b3b] text-[#f1f5f9] font-mono text-xs uppercase tracking-wider rounded-xs transition-colors cursor-pointer"
			>
				Filter
			</button>
		</div>
	</div>

	<!-- Action Event Stream List -->
	<div class="bg-[#12151d] border border-[#252b3b] rounded-xs overflow-hidden flex flex-col font-mono text-xs sm:text-sm">
		<div class="overflow-x-auto">
			<table class="w-full text-left border-collapse">
				<thead>
					<tr class="bg-[#151922] border-b border-[#252b3b] text-[#94a3b8]">
						<th class="py-2.5 px-4 font-medium">Timestamp</th>
						<th class="py-2.5 px-4 font-medium">Category</th>
						<th class="py-2.5 px-4 font-medium">Action Name</th>
						<th class="py-2.5 px-4 font-medium">Status</th>
						<th class="py-2.5 px-4 font-medium">Duration</th>
						<th class="py-2.5 px-4 text-right font-medium">Details</th>
					</tr>
				</thead>

				<tbody class="divide-y divide-[#252b3b]/60">
					{#if isLoading && actions.length === 0}
						<tr>
							<td colspan="6" class="py-10 text-center text-[#64748b]">
								<RefreshCw size={18} class="animate-spin text-[#d4ff32] mx-auto mb-2" />
								Querying NeonDB telemetry stream...
							</td>
						</tr>
					{:else if actions.length === 0}
						<tr>
							<td colspan="6" class="py-10 text-center text-[#64748b]">
								No action events recorded matching current filter parameters
							</td>
						</tr>
					{:else}
						{#each actions as act (act.id)}
							<tr class="hover:bg-[#181c26]/60 transition-colors">
								<td class="py-2.5 px-4 text-[#64748b] whitespace-nowrap">
									<div class="text-[#f1f5f9] font-medium">{formatRelativeTime(act.created_at)}</div>
									<div class="text-[11px] text-[#64748b]">{formatTime(act.created_at)}</div>
								</td>

								<td class="py-2.5 px-4 whitespace-nowrap">
									<span class="px-2 py-0.5 text-xs rounded-xs font-semibold {act.action_type === 'telegram_command'
										? 'bg-[#06b6d4]/15 text-[#06b6d4] border border-[#06b6d4]/30'
										: act.action_type === 'extension_sync'
											? 'bg-[#10b981]/15 text-[#10b981] border border-[#10b981]/30'
											: act.action_type === 'cron_alert'
												? 'bg-purple-500/15 text-purple-400 border border-purple-500/30'
												: act.action_type === 'admin_action'
													? 'bg-[#d4ff32]/15 text-[#d4ff32] border border-[#d4ff32]/30'
													: 'bg-[#252b3b] text-[#94a3b8]'}">
										{act.action_type}
									</span>
								</td>

								<td class="py-2.5 px-4 font-semibold text-[#f1f5f9]">
									<code class="text-[#d4ff32] font-mono">{act.action_name}</code>
								</td>

								<td class="py-2.5 px-4 whitespace-nowrap">
									{#if act.status_code >= 200 && act.status_code < 300}
										<span class="text-[#10b981] font-semibold">{act.status_code} OK</span>
									{:else if act.status_code >= 400 && act.status_code < 500}
										<span class="text-amber-400 font-semibold">{act.status_code}</span>
									{:else}
										<span class="text-[#ef4444] font-semibold">{act.status_code} ERR</span>
									{/if}
								</td>

								<td class="py-2.5 px-4 whitespace-nowrap text-[#94a3b8] tabular-nums">
									{act.duration_ms}ms
								</td>

								<td class="py-2.5 px-4 text-right whitespace-nowrap">
									{#if act.metadata}
										<button
											onclick={() => (selectedMetadata = act.metadata)}
											class="p-1 border border-[#252b3b] hover:border-[#d4ff32] bg-[#0a0b0e] text-[#94a3b8] hover:text-[#d4ff32] rounded-xs transition-colors cursor-pointer"
											title="Inspect Anonymous Payload"
										>
											<Info size={14} />
										</button>
									{:else}
										<span class="text-[#64748b]">—</span>
									{/if}
								</td>
							</tr>
						{/each}
					{/if}
				</tbody>
			</table>
		</div>

		<!-- Pagination Footer -->
		<div class="border-t border-[#252b3b] bg-[#0e1117] px-4 py-3 flex items-center justify-between text-xs font-mono text-[#64748b]">
			<div>
				Showing <span class="text-[#f1f5f9] font-medium">{actions.length}</span> of <span class="text-[#f1f5f9] font-medium">{total}</span> actions
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
					class="p-1.5 border border-[#252b3b] hover:border-[#64748b] disabled:opacity-30 disabled:cursor-not-allowed text-[#94a3b8] hover:text-white rounded-xs transition-colors cursor-pointer"
				>
					<ChevronLeft size={14} />
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
					class="p-1.5 border border-[#252b3b] hover:border-[#64748b] disabled:opacity-30 disabled:cursor-not-allowed text-[#94a3b8] hover:text-white rounded-xs transition-colors cursor-pointer"
				>
					<ChevronRight size={14} />
				</button>
			</div>
		</div>
	</div>
</div>

<!-- Metadata Inspector Modal -->
<Modal
	open={selectedMetadata !== null}
	title="ANONYMOUS ACTION PAYLOAD"
	description="All user identifiers (IDs, names, tokens) are scrubbed at source."
	onClose={() => (selectedMetadata = null)}
>
	<div class="bg-[#0a0b0e] border border-[#252b3b] p-3 text-[#e6edf3] rounded-xs overflow-x-auto max-h-72">
		<pre class="text-xs">{JSON.stringify(selectedMetadata, null, 2)}</pre>
	</div>

	{#snippet footer()}
		<button
			onclick={() => (selectedMetadata = null)}
			class="px-3.5 py-1.5 border border-[#252b3b] hover:border-[#64748b] bg-[#181c26] text-[#f1f5f9] text-xs font-mono uppercase tracking-wider rounded-xs transition-colors cursor-pointer"
		>
			Close
		</button>
	{/snippet}
</Modal>
