<script lang="ts">
	import { toggleMode } from 'mode-watcher';
	import { Sun, Moon } from 'lucide-svelte';

	interface Props {
		class?: string;
	}

	let { class: className = '' }: Props = $props();
</script>

<button
	type="button"
	onclick={toggleMode}
	class="flex items-center gap-1.5 px-2 py-1 sm:px-2.5 sm:py-1 rounded-xs text-xs font-mono border transition-colors cursor-pointer select-none border-slate-300 hover:border-slate-400 bg-white hover:bg-slate-100 text-slate-700 hover:text-slate-950 shadow-xs dark:border-[#252b3b] dark:hover:border-[#64748b] dark:bg-[#12151d] dark:hover:bg-[#181c26] dark:text-[#94a3b8] dark:hover:text-[#f1f5f9] dark:shadow-none {className}"
	title="Toggle Light/Dark Theme"
	aria-label="Toggle Light/Dark Theme"
>
	<span class="theme-dark-content items-center gap-1.5">
		<Moon size={13} class="text-[#d4ff32]" />
		<span class="font-semibold text-[11px] tracking-wider uppercase text-[#f1f5f9]">DARK</span>
	</span>
	<span class="theme-light-content items-center gap-1.5">
		<Sun size={13} class="text-amber-500" />
		<span class="font-semibold text-[11px] tracking-wider uppercase text-slate-900">LIGHT</span>
	</span>
</button>

<style>
	.theme-dark-content {
		display: none;
	}
	.theme-light-content {
		display: inline-flex;
	}

	:global(.dark) .theme-dark-content {
		display: inline-flex;
	}
	:global(.dark) .theme-light-content {
		display: none;
	}
</style>
