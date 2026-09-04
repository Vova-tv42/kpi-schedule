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

	function handleKeydown(e: KeyboardEvent) {
		if (e.key === 'Escape' && open) {
			onCancel();
		}
	}
</script>

<svelte:window onkeydown={handleKeydown} />

{#if open}
	<div 
		role="button"
		tabindex="0"
		class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/80 backdrop-blur-xs animate-in fade-in duration-150"
		onclick={(e) => { if (e.target === e.currentTarget) onCancel(); }}
		onkeydown={(e) => { if (e.key === 'Enter' || e.key === ' ') onCancel(); }}
	>
		<div class="w-full max-w-lg bg-[#12151d] border border-amber-500/40 shadow-2xl rounded-xs overflow-hidden flex flex-col">
			<!-- Header -->
			<div class="px-5 py-4 border-b border-[#252b3b] flex items-center justify-between bg-[#151922]">
				<div class="flex items-center gap-2.5">
					<div class="p-1.5 rounded-xs bg-amber-500/10 border border-amber-500/30 text-amber-400">
						<AlertTriangle size={16} />
					</div>
					<div>
						<h3 class="font-display font-semibold text-[#f1f5f9] text-base tracking-wide flex items-center gap-2">
							Confirm Server Wake-Up
						</h3>
						<span class="text-[11px] text-amber-400/90 font-mono uppercase tracking-widest">
							Scale-to-Zero Interlock Guard
						</span>
					</div>
				</div>
				<button 
					onclick={onCancel} 
					class="p-1 text-[#94a3b8] hover:text-[#f1f5f9] hover:bg-[#252b3b]/50 rounded transition-colors cursor-pointer"
					title="Close (Esc)"
				>
					<X size={16} />
				</button>
			</div>

			<!-- Body -->
			<div class="p-5 font-mono text-sm space-y-4">
				<div class="p-3 border border-[#252b3b] bg-[#0a0b0e] rounded-xs">
					<div class="text-xs uppercase tracking-wider text-[#64748b]">Target Operation:</div>
					<div class="text-[#f1f5f9] font-bold text-sm truncate mt-0.5">{actionTitle}</div>
				</div>

				<p class="leading-relaxed text-[#94a3b8] text-xs sm:text-sm">
					The Go server microVM is currently in <span class="text-amber-400 font-semibold">Scale-to-Zero Standby</span> on Fly.io to eliminate idle compute costs.
				</p>
				<p class="leading-relaxed text-[#64748b] text-xs">
					Dispatching this request will initiate a Firecracker VM cold boot (~500ms) and reset the in-app 15-minute idle countdown timer.
				</p>
			</div>

			<!-- Footer -->
			<div class="px-5 py-3 border-t border-[#252b3b] bg-[#0e1117] flex items-center justify-end gap-2.5">
				<button
					onclick={onCancel}
					class="px-3.5 py-2 border border-[#252b3b] hover:border-[#64748b] bg-[#181c26] hover:bg-[#252b3b] text-[#94a3b8] hover:text-[#f1f5f9] font-mono text-xs uppercase tracking-wider rounded-xs transition-colors cursor-pointer"
				>
					Cancel (Keep Sleeping)
				</button>
				<button
					onclick={onProceed}
					class="flex items-center gap-1.5 px-4 py-2 border border-[#d4ff32]/50 bg-[#d4ff32] hover:bg-[#e2f952] text-black font-mono text-xs font-bold uppercase tracking-wider rounded-xs transition-colors shadow-[0_0_15px_rgba(212,255,50,0.2)] cursor-pointer"
				>
					<Zap size={13} class="fill-current" />
					<span>Proceed (Wake Server)</span>
				</button>
			</div>
		</div>
	</div>
{/if}
