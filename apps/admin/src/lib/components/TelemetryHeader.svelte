<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { serverStatus } from '$lib/server-status-store.svelte';
	import Badge from './Badge.svelte';
	import { LogOut, User, RefreshCw, Zap, Moon, AlertTriangle, Menu } from 'lucide-svelte';

	interface Props {
		user: {
			email: string;
			role: string;
			isSuperadmin: boolean;
		} | null;
		onToggleMobile?: () => void;
	}

	let { user, onToggleMobile }: Props = $props();

	onMount(() => {
		serverStatus.startPolling(20000);
	});

	onDestroy(() => {
		serverStatus.stopPolling();
	});

	const vmStatus = $derived(serverStatus.data.status);
	const region = $derived(serverStatus.data.region);
</script>

<header class="h-14 border-b border-[#252b3b] bg-[#0e1117] px-4 sm:px-6 flex items-center justify-between shrink-0 select-none">
	<!-- Left: Mobile Menu Toggle & Fly.io Scale-to-Zero VM Status -->
	<div class="flex items-center gap-3 sm:gap-4 text-xs sm:text-sm font-mono">
		<button
			onclick={onToggleMobile}
			class="lg:hidden p-1.5 text-[#94a3b8] hover:text-white rounded hover:bg-[#181c26] border border-[#252b3b]"
			title="Toggle Navigation Menu"
		>
			<Menu size={16} />
		</button>

		<!-- Fly.io Scale-to-Zero VM Status -->
		<div class="flex items-center gap-2">
			<div 
				class="flex items-center gap-1.5 px-2.5 py-1 border rounded-xs text-xs font-mono tracking-wider uppercase {vmStatus === 'awake'
					? 'border-[#10b981]/40 bg-[#10b981]/10 text-[#10b981]'
					: vmStatus === 'sleeping'
						? 'border-amber-500/40 bg-amber-500/10 text-amber-400'
						: vmStatus === 'transitioning'
							? 'border-yellow-500/40 bg-yellow-500/10 text-yellow-400'
							: 'border-red-500/40 bg-red-500/10 text-red-400'}"
			>
				{#if vmStatus === 'awake'}
					<span class="w-1.5 h-1.5 rounded-full bg-[#10b981] animate-ping"></span>
					<Zap size={12} class="hidden sm:inline" />
					<span class="font-semibold">VM AWAKE</span>
				{:else if vmStatus === 'sleeping'}
					<span class="w-1.5 h-1.5 rounded-full bg-amber-500"></span>
					<Moon size={12} class="hidden sm:inline" />
					<span class="font-semibold">STANDBY (0/15m)</span>
				{:else if vmStatus === 'transitioning'}
					<RefreshCw size={12} class="animate-spin" />
					<span class="font-semibold">STARTING...</span>
				{:else}
					<AlertTriangle size={12} />
					<span class="font-semibold">VM OFFLINE</span>
				{/if}

				{#if region}
					<span class="text-[10px] text-[#64748b] hidden md:inline border-l border-[#252b3b] pl-1.5 ml-0.5">
						{region.toUpperCase()}
					</span>
				{/if}
			</div>

			<!-- Safe Recheck Button (talks to Machines API without waking VM) -->
			<button
				onclick={() => { serverStatus.checkStatus(); }}
				disabled={serverStatus.isChecking}
				title="Safe poll via Fly Machines API (Does not wake VM)"
				class="p-1 border border-[#252b3b] hover:border-[#64748b] bg-[#12151d] hover:bg-[#181c26] text-[#94a3b8] hover:text-[#f1f5f9] rounded-xs transition-colors cursor-pointer"
			>
				<RefreshCw size={12} class={serverStatus.isChecking ? 'animate-spin text-[#d4ff32]' : ''} />
			</button>
		</div>
	</div>

	<!-- Right: User session & sign out -->
	<div class="flex items-center gap-2 sm:gap-3">
		{#if user}
			<div class="flex items-center gap-2">
				<div class="text-right hidden sm:block">
					<div class="text-xs sm:text-sm font-mono font-medium text-[#f1f5f9] flex items-center gap-1.5 truncate max-w-[160px] md:max-w-[220px]">
						<User size={13} class="text-[#94a3b8] shrink-0" />
						<span class="truncate">{user.email}</span>
					</div>
				</div>

				{#if user.isSuperadmin}
					<Badge variant="lime" size="sm">SUPERADMIN</Badge>
				{:else if user.role === 'read-write'}
					<Badge variant="cyan" size="sm">READ & WRITE</Badge>
				{:else}
					<Badge variant="amber" size="sm">READ ONLY</Badge>
				{/if}
			</div>

			<div class="h-4 w-px bg-[#252b3b]"></div>

			<a
				href="/auth/logout"
				class="flex items-center gap-1.5 px-2 py-1 sm:px-2.5 sm:py-1.5 rounded-xs text-xs font-mono text-[#94a3b8] hover:text-[#ef4444] hover:bg-[#ef4444]/10 transition-colors border border-transparent hover:border-[#ef4444]/20"
				title="End session"
			>
				<LogOut size={13} />
				<span class="hidden sm:inline">Disconnect</span>
			</a>
		{/if}
	</div>
</header>
