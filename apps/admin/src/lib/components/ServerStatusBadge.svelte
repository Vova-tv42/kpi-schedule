<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { serverStatus } from '$lib/server-status-store.svelte';
	import { RefreshCw, Moon, Zap, AlertTriangle, HelpCircle } from 'lucide-svelte';

	onMount(() => {
		serverStatus.startPolling(20000);
	});

	onDestroy(() => {
		serverStatus.stopPolling();
	});

	const status = $derived(serverStatus.data.status);
	const region = $derived(serverStatus.data.region);
	const machineId = $derived(serverStatus.data.machine_id);
</script>

<div class="flex items-center gap-2">
	<div
		class="flex items-center gap-2 px-2.5 py-1 border text-xs font-mono tracking-wider uppercase rounded-xs transition-all duration-200 {status ===
		'awake'
			? 'border-[#10b981]/40 bg-[#10b981]/10 text-[#10b981]'
			: status === 'sleeping'
				? 'border-amber-500/40 bg-amber-500/10 text-amber-400'
				: status === 'transitioning'
					? 'border-yellow-500/40 bg-yellow-500/10 text-yellow-400'
					: status === 'unconfigured'
						? 'border-[#252b3b] bg-[#12151d] text-[#94a3b8]'
						: 'border-[#ef4444]/40 bg-[#ef4444]/10 text-[#ef4444]'}"
	>
		<!-- Status Icon & Indicator -->
		<div class="flex items-center gap-1.5">
			{#if status === 'awake'}
				<span class="w-1.5 h-1.5 rounded-full bg-[#10b981] animate-ping"></span>
				<Zap size={12} />
				<span class="font-bold">Awake</span>
			{:else if status === 'sleeping'}
				<span class="w-1.5 h-1.5 rounded-full bg-amber-500"></span>
				<Moon size={12} />
				<span class="font-bold">Standby (0/15m)</span>
			{:else if status === 'transitioning'}
				<RefreshCw size={12} class="animate-spin" />
				<span class="font-bold">Transitioning</span>
			{:else if status === 'unconfigured'}
				<HelpCircle size={12} class="text-[#64748b]" />
				<span>Unconfigured</span>
			{:else}
				<AlertTriangle size={12} />
				<span class="font-bold">Offline</span>
			{/if}
		</div>

		{#if region}
			<span class="text-[10px] text-[#64748b] border-l border-[#252b3b] pl-1.5 ml-0.5">
				{region.toUpperCase()}
			</span>
		{/if}

		{#if machineId}
			<span class="text-[10px] text-[#64748b] hidden sm:inline">
				#{machineId.slice(0, 8)}
			</span>
		{/if}
	</div>

	<!-- Safe Recheck Button (talks to Fly Machines API, never wakes VM) -->
	<button
		onclick={() => serverStatus.checkStatus()}
		disabled={serverStatus.isChecking}
		title="Check VM state via Fly Machines API (Does not wake server)"
		class="p-1 border border-[#252b3b] hover:border-[#64748b] bg-[#12151d] hover:bg-[#181c26] text-[#94a3b8] hover:text-white rounded-xs transition-colors cursor-pointer"
	>
		<RefreshCw size={12} class={serverStatus.isChecking ? 'animate-spin text-[#d4ff32]' : ''} />
	</button>
</div>
