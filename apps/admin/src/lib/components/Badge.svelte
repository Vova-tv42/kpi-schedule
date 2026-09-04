<script lang="ts">
	import type { Snippet } from 'svelte';

	interface Props {
		variant?: 'lime' | 'amber' | 'emerald' | 'crimson' | 'slate' | 'cyan' | 'violet';
		size?: 'sm' | 'md';
		class?: string;
		children?: Snippet;
	}

	let { variant = 'slate', size = 'sm', class: className = '', children }: Props = $props();

	const variantStyles = {
		lime: 'bg-[#ccf600]/25 text-[#2e5a08] border-[#a3e635] dark:bg-[#d4ff32]/10 dark:text-[#d4ff32] dark:border-[#d4ff32]/30',
		amber: 'bg-amber-100 text-amber-900 border-amber-300 dark:bg-[#f59e0b]/10 dark:text-[#f59e0b] dark:border-[#f59e0b]/30',
		emerald: 'bg-emerald-100 text-emerald-900 border-emerald-300 dark:bg-[#10b981]/10 dark:text-[#10b981] dark:border-[#10b981]/30',
		crimson: 'bg-red-100 text-red-900 border-red-300 dark:bg-[#ef4444]/10 dark:text-[#ef4444] dark:border-[#ef4444]/30',
		slate: 'bg-slate-100 text-slate-700 border-slate-300 dark:bg-[#252b3b]/40 dark:text-[#94a3b8] dark:border-[#252b3b]',
		cyan: 'bg-cyan-100 text-cyan-900 border-cyan-300 dark:bg-[#06b6d4]/10 dark:text-[#06b6d4] dark:border-[#06b6d4]/30',
		violet: 'bg-violet-100 text-violet-900 border-violet-300 dark:bg-[#a78bfa]/10 dark:text-[#a78bfa] dark:border-[#a78bfa]/30'
	};

	const sizeStyles = {
		sm: 'px-1.5 py-0.5 text-xs tracking-wider uppercase font-mono font-medium',
		md: 'px-2 py-1 text-sm tracking-wider uppercase font-mono font-medium'
	};
</script>

<span class="inline-flex items-center gap-1.5 border rounded-xs {variantStyles[variant]} {sizeStyles[size]} {className}">
	{@render children?.()}
</span>
