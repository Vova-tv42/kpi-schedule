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
		class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/50 dark:bg-black/80 backdrop-blur-xs animate-in fade-in duration-150"
		onclick={(e) => { if (e.target === e.currentTarget) onCancel(); }}
		onkeydown={(e) => { if (e.key === 'Enter' || e.key === ' ') onCancel(); }}
	>
		<div class="w-full max-w-lg bg-white dark:bg-[#12151d] border border-amber-500/50 shadow-2xl rounded-xs overflow-hidden flex flex-col">
			<!-- Header -->
			<div class="px-5 py-4 border-b border-slate-300 dark:border-[#252b3b] flex items-center justify-between bg-amber-50/50 dark:bg-[#151922]">
				<div class="flex items-center gap-2.5">
					<div class="p-1.5 rounded-xs bg-amber-100 dark:bg-amber-500/10 border border-amber-300 dark:border-amber-500/30 text-amber-900 dark:text-amber-400">
						<AlertTriangle size={16} />
					</div>
					<div>
						<h3 class="font-display font-semibold text-slate-950 dark:text-[#f1f5f9] text-base tracking-wide flex items-center gap-2">
							Confirm Server Wake-Up
						</h3>
						<span class="text-[11px] text-amber-800 dark:text-amber-400/90 font-mono uppercase tracking-widest font-semibold">
							Scale-to-Zero Interlock Guard
						</span>
					</div>
				</div>
				<button 
					onclick={onCancel} 
					class="p-1 text-slate-500 hover:text-slate-950 hover:bg-slate-200/50 dark:text-[#94a3b8] dark:hover:text-[#f1f5f9] dark:hover:bg-[#252b3b]/50 rounded transition-colors cursor-pointer"
					title="Close (Esc)"
				>
					<X size={16} />
				</button>
			</div>

			<!-- Body -->
			<div class="p-5 font-mono text-sm space-y-4">
				<div class="p-3 border border-slate-300 dark:border-[#252b3b] bg-slate-50 dark:bg-[#0a0b0e] rounded-xs">
					<div class="text-xs uppercase tracking-wider text-slate-500 dark:text-[#64748b]">Target Operation:</div>
					<div class="text-slate-950 dark:text-[#f1f5f9] font-bold text-sm truncate mt-0.5">{actionTitle}</div>
				</div>

				<p class="leading-relaxed text-slate-600 dark:text-[#94a3b8] text-xs sm:text-sm">
					The Go server microVM is currently in <span class="text-amber-700 dark:text-amber-400 font-semibold">Scale-to-Zero Standby</span> on Fly.io to eliminate idle compute costs.
				</p>
				<p class="leading-relaxed text-slate-500 dark:text-[#64748b] text-xs">
					Dispatching this request will initiate a Firecracker VM cold boot (~500ms) and reset the in-app 15-minute idle countdown timer.
				</p>
			</div>

			<!-- Footer -->
			<div class="px-5 py-3 border-t border-slate-300 dark:border-[#252b3b] bg-slate-50 dark:bg-[#0e1117] flex items-center justify-end gap-2.5">
				<button
					onclick={onCancel}
					class="px-3.5 py-2 border border-slate-300 hover:border-slate-400 bg-white hover:bg-slate-100 text-slate-700 hover:text-slate-950 dark:border-[#252b3b] dark:hover:border-[#64748b] dark:bg-[#181c26] dark:hover:bg-[#252b3b] dark:text-[#94a3b8] dark:hover:text-[#f1f5f9] font-mono text-xs uppercase tracking-wider rounded-xs transition-colors cursor-pointer"
				>
					Cancel (Keep Sleeping)
				</button>
				<button
					onclick={onProceed}
					class="flex items-center gap-1.5 px-4 py-2 border border-black dark:border-[#d4ff32]/50 bg-[#ccf600] hover:bg-[#bde800] dark:bg-[#d4ff32] dark:hover:bg-[#e2f952] text-black font-mono text-xs font-bold uppercase tracking-wider rounded-xs transition-colors shadow-[2px_2px_0px_#090d16] dark:shadow-[0_0_15px_rgba(212,255,50,0.2)] cursor-pointer"
				>
					<Zap size={13} class="fill-current" />
					<span>Proceed (Wake Server)</span>
				</button>
			</div>
		</div>
	</div>
{/if}
