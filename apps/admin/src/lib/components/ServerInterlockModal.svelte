<script lang="ts">
	import { AlertTriangle, Zap, X } from 'lucide-svelte';

	let {
		open = false,
		actionTitle = 'Main Server Request',
		onProceed,
		onCancel
	}: {
		open: boolean;
		actionTitle?: string;
		onProceed: () => void;
		onCancel: () => void;
	} = $props();
</script>

{#if open}
	<div class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/80 backdrop-blur-xs animate-in fade-in duration-150">
		<div class="w-full max-w-lg border border-amber-500/40 bg-[#0f141d] p-6 shadow-2xl relative">
			<!-- Tactical Corner Accents -->
			<div class="absolute -top-[1px] -left-[1px] w-3 h-3 border-t-2 border-l-2 border-amber-500"></div>
			<div class="absolute -top-[1px] -right-[1px] w-3 h-3 border-t-2 border-r-2 border-amber-500"></div>
			<div class="absolute -bottom-[1px] -left-[1px] w-3 h-3 border-b-2 border-l-2 border-amber-500"></div>
			<div class="absolute -bottom-[1px] -right-[1px] w-3 h-3 border-b-2 border-r-2 border-amber-500"></div>

			<!-- Dialog Header -->
			<div class="flex items-start justify-between border-b border-[#1e293b] pb-4 mb-4">
				<div class="flex items-center gap-3">
					<div class="p-2 border border-amber-500/30 bg-amber-500/10 text-amber-400">
						<AlertTriangle class="w-5 h-5" />
					</div>
					<div>
						<div class="text-xs font-mono uppercase tracking-widest text-amber-500">[Scale-to-Zero Interlock]</div>
						<h2 class="text-xl font-bold uppercase text-white font-sans">Confirm Server Wake-up</h2>
					</div>
				</div>
				<button onclick={onCancel} class="text-slate-500 hover:text-white p-1">
					<X class="w-4 h-4" />
				</button>
			</div>

			<!-- Dialog Body -->
			<div class="space-y-4 text-sm font-mono text-slate-300 mb-6">
				<div class="p-3 border border-[#1e293b] bg-[#0a0e14]">
					<div class="text-slate-400">Target Action:</div>
					<div class="text-white font-bold text-base truncate mt-0.5">{actionTitle}</div>
				</div>

				<p class="leading-relaxed">
					The main server is currently in <span class="text-amber-400 font-semibold">Scale-to-Zero Standby</span> on Fly.io to save compute billing.
				</p>
				<p class="leading-relaxed text-slate-400">
					Sending this request will wake up the Firecracker microVM (~500ms cold boot) and reset the in-app 15-minute idle countdown.
				</p>
			</div>

			<!-- Dialog Actions -->
			<div class="flex items-center justify-end gap-3 pt-2">
				<button
					onclick={onCancel}
					class="px-4 py-2 border border-[#1e293b] bg-[#141c28] hover:bg-[#1a2536] text-slate-300 hover:text-white font-mono text-sm uppercase tracking-wider transition-colors"
				>
					Cancel (Keep Sleeping)
				</button>
				<button
					onclick={onProceed}
					class="flex items-center gap-2 px-4 py-2 border border-amber-500/60 bg-amber-500 hover:bg-amber-400 text-black font-mono text-sm font-bold uppercase tracking-wider transition-colors"
				>
					<Zap class="w-4 h-4 fill-current" />
					Proceed (Wake Server)
				</button>
			</div>
		</div>
	</div>
{/if}
