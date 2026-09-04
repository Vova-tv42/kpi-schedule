<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { serverStatus } from '$lib/server-status-store.svelte';
	import { RefreshCw, Server, Moon, Zap, AlertTriangle, HelpCircle } from 'lucide-svelte';

	onMount(() => {
		serverStatus.startPolling(20000);
	});

	onDestroy(() => {
		serverStatus.stopPolling();
	});

	const status = $derived(serverStatus.data.status);
	const state = $derived(serverStatus.data.state);
	const region = $derived(serverStatus.data.region);
	const machineId = $derived(serverStatus.data.machine_id);
</script>

<div class="flex items-center gap-2">
	<div
		class="flex items-center gap-2 px-3 py-1.5 border text-sm font-mono tracking-wide rounded-none transition-all duration-200 {status ===
		'awake'
			? 'border-emerald-500/40 bg-emerald-950/20 text-emerald-400'
			: status === 'sleeping'
				? 'border-amber-500/40 bg-amber-950/20 text-amber-400'
				: status === 'transitioning'
					? 'border-yellow-500/40 bg-yellow-950/20 text-yellow-400'
					: status === 'unconfigured'
						? 'border-slate-700 bg-slate-900/50 text-slate-400'
						: 'border-red-500/40 bg-red-950/20 text-red-400'}"
	>
		<!-- Status Icon & Indicator -->
		<div class="flex items-center gap-1.5">
			{#if status === 'awake'}
				<span class="relative flex h-2 w-2">
					<span class="animate-ping absolute inline-flex h-full w-full rounded-full bg-emerald-400 opacity-75"></span>
					<span class="relative inline-flex rounded-full h-2 w-2 bg-emerald-500"></span>
				</span>
				<Zap class="w-4 h-4" />
				<span class="font-bold uppercase">Awake</span>
			{:else if status === 'sleeping'}
				<span class="inline-block h-2 w-2 rounded-full bg-amber-500"></span>
				<Moon class="w-4 h-4" />
				<span class="font-bold uppercase">Standby (15m Idle)</span>
			{:else if status === 'transitioning'}
				<span class="inline-block h-2 w-2 rounded-full bg-yellow-500 animate-pulse"></span>
				<RefreshCw class="w-4 h-4 animate-spin" />
				<span class="font-bold uppercase">Transitioning</span>
			{:else if status === 'unconfigured'}
				<HelpCircle class="w-4 h-4 text-slate-400" />
				<span class="uppercase text-slate-400">Status: Setup Token</span>
			{:else}
				<span class="inline-block h-2 w-2 rounded-full bg-red-500"></span>
				<AlertTriangle class="w-4 h-4" />
				<span class="font-bold uppercase">Offline</span>
			{/if}
		</div>

		{#if region}
			<span class="text-xs text-slate-400 border-l border-slate-700/60 pl-2">
				{region.toUpperCase()}
			</span>
		{/if}

		{#if machineId}
			<span class="text-xs text-slate-500 hidden sm:inline">
				#{machineId.slice(0, 8)}
			</span>
		{/if}
	</div>

	<!-- Safe Recheck Button (talks to Fly Machines API, never wakes VM) -->
	<button
		onclick={() => serverStatus.checkStatus()}
		disabled={serverStatus.isChecking}
		title="Check VM state via Fly Machines API (Does not wake server)"
		class="p-1.5 border border-[#1e293b] hover:border-slate-600 bg-[#0f141d] hover:bg-[#161e2b] text-slate-400 hover:text-white transition-colors"
	>
		<RefreshCw class="w-3.5 h-3.5 {serverStatus.isChecking ? 'animate-spin text-cyan-400' : ''}" />
	</button>
</div>
