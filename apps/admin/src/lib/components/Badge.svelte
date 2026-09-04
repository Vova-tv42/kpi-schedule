<script lang="ts">
	import type { Snippet } from 'svelte';

	interface Props {
		variant?: 'lime' | 'amber' | 'emerald' | 'crimson' | 'slate' | 'cyan';
		size?: 'sm' | 'md';
		class?: string;
		children?: Snippet;
	}

	let { variant = 'slate', size = 'sm', class: className = '', children }: Props = $props();

	const variantStyles = {
		lime: 'bg-[#d4ff32]/10 text-[#d4ff32] border-[#d4ff32]/30',
		amber: 'bg-[#f59e0b]/10 text-[#f59e0b] border-[#f59e0b]/30',
		emerald: 'bg-[#10b981]/10 text-[#10b981] border-[#10b981]/30',
		crimson: 'bg-[#ef4444]/10 text-[#ef4444] border-[#ef4444]/30',
		slate: 'bg-[#252b3b]/40 text-[#94a3b8] border-[#252b3b]',
		cyan: 'bg-[#06b6d4]/10 text-[#06b6d4] border-[#06b6d4]/30'
	};

	const sizeStyles = {
		sm: 'px-1.5 py-0.5 text-xs tracking-wider uppercase font-mono font-medium',
		md: 'px-2 py-1 text-sm tracking-wider uppercase font-mono font-medium'
	};
</script>

<span class="inline-flex items-center gap-1.5 border rounded-xs {variantStyles[variant]} {sizeStyles[size]} {className}">
	{@render children?.()}
</span>
