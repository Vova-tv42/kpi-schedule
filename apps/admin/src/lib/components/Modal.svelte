<script lang="ts">
	import type { Snippet } from 'svelte';
	import { X } from 'lucide-svelte';

	interface Props {
		open: boolean;
		title: string;
		description?: string;
		onClose: () => void;
		children?: Snippet;
		footer?: Snippet;
	}

	let { open, title, description, onClose, children, footer }: Props = $props();

	function handleKeydown(e: KeyboardEvent) {
		if (e.key === 'Escape' && open) {
			onClose();
		}
	}
</script>

<svelte:window onkeydown={handleKeydown} />

{#if open}
	<!-- Backdrop -->
	<div 
		role="button"
		tabindex="0"
		class="fixed inset-0 z-50 bg-black/50 dark:bg-black/80 backdrop-blur-xs flex items-center justify-center p-4 animate-in fade-in duration-150"
		onclick={(e) => { if (e.target === e.currentTarget) onClose(); }}
		onkeydown={(e) => { if (e.key === 'Enter' || e.key === ' ') onClose(); }}
	>
		<!-- Modal Box -->
		<div class="w-full max-w-xl bg-white dark:bg-[#12151d] border border-slate-300 dark:border-[#252b3b] shadow-2xl rounded-xs overflow-hidden flex flex-col max-h-[90vh] text-slate-900 dark:text-[#f1f5f9]">
			<!-- Header -->
			<div class="px-5 py-4 border-b border-slate-300 dark:border-[#252b3b] flex items-center justify-between bg-slate-50 dark:bg-[#151922]">
				<div>
					<h3 class="font-display font-semibold text-slate-950 dark:text-[#f1f5f9] text-lg tracking-wide flex items-center gap-2">
						<span class="w-1.5 h-1.5 rounded-full bg-emerald-600 dark:bg-[#d4ff32]"></span>
						{title}
					</h3>
					{#if description}
						<p class="text-sm text-slate-500 dark:text-[#94a3b8] font-mono mt-0.5">{description}</p>
					{/if}
				</div>
				<button 
					onclick={onClose}
					class="p-1 text-slate-500 hover:text-slate-950 hover:bg-slate-200/60 dark:text-[#94a3b8] dark:hover:text-[#f1f5f9] dark:hover:bg-[#252b3b]/50 rounded transition-colors cursor-pointer"
					title="Close (Esc)"
				>
					<X size={16} />
				</button>
			</div>

			<!-- Body -->
			<div class="p-5 overflow-y-auto flex-1 font-mono text-sm">
				{@render children?.()}
			</div>

			<!-- Footer -->
			{#if footer}
				<div class="px-5 py-3 border-t border-slate-300 dark:border-[#252b3b] bg-slate-50 dark:bg-[#0e1117] flex justify-end gap-2">
					{@render footer()}
				</div>
			{/if}
		</div>
	</div>
{/if}
