<script lang="ts">
	import { page } from '$app/state';
	import {
		LayoutDashboard,
		Database,
		TerminalSquare,
		Activity,
		Inbox,
		ShieldAlert,
		Users,
		Settings,
		Lock,
		X
	} from 'lucide-svelte';

	interface Props {
		user: {
			email: string;
			role: string;
			isSuperadmin: boolean;
		} | null;
		mobileOpen?: boolean;
		onCloseMobile?: () => void;
	}

	let { user, mobileOpen = false, onCloseMobile }: Props = $props();

	const pathname = $derived(page.url.pathname);

	const isOverviewActive = $derived(pathname === '/');
	const isActionsActive = $derived(pathname === '/actions');
	const isIssuesActive = $derived(pathname.startsWith('/issues'));
	const isDatabaseActive = $derived(pathname.startsWith('/database') && pathname !== '/database/query');
	const isQueryActive = $derived(pathname === '/database/query');
	const isAdminsActive = $derived(pathname === '/admins');
	const isSettingsActive = $derived(pathname === '/settings');

	function handleLinkClick() {
		onCloseMobile?.();
	}
</script>

<!-- Mobile Overlay Backdrop -->
{#if mobileOpen}
	<div
		role="button"
		tabindex="0"
		class="fixed inset-0 bg-black/60 dark:bg-black/80 z-40 lg:hidden backdrop-blur-xs transition-opacity"
		onclick={onCloseMobile}
		onkeydown={(e) => { if (e.key === 'Enter' || e.key === ' ') onCloseMobile?.(); }}
	></div>
{/if}

<aside class="w-64 bg-white dark:bg-[#0e1117] border-r border-slate-300 dark:border-[#252b3b] flex flex-col justify-between shrink-0 select-none fixed inset-y-0 left-0 z-50 transition-all duration-200 lg:static lg:translate-x-0 {mobileOpen ? 'translate-x-0 shadow-2xl' : '-translate-x-full'}">
	<!-- Branding / Mission Control Header -->
	<div class="p-4 border-b border-slate-300 dark:border-[#252b3b] flex items-center justify-between">
		<div class="flex items-center gap-2.5">
			<div class="w-8 h-8 rounded-xs bg-[#ccf600]/25 text-slate-950 border border-[#a3e635] shadow-xs dark:bg-[#d4ff32]/10 dark:border-[#d4ff32]/30 dark:text-[#d4ff32] dark:shadow-none flex items-center justify-center">
				<TerminalSquare size={18} />
			</div>
			<div>
				<div class="font-display font-bold text-base tracking-tight text-slate-950 dark:text-[#f1f5f9] flex items-center gap-1.5">
					Dashboard
				</div>
				<div class="font-mono text-xs text-slate-500 dark:text-[#94a3b8] tracking-widest uppercase">
					Ops & Telemetry
				</div>
			</div>
		</div>

		<div class="flex items-center gap-2">
			<span class="inline-block w-2 h-2 rounded-full bg-emerald-600 dark:bg-[#10b981] animate-pulse" title="System Operational"></span>
			<button
				onclick={onCloseMobile}
				class="lg:hidden p-1 text-slate-600 hover:text-slate-950 rounded hover:bg-slate-100 dark:text-[#94a3b8] dark:hover:text-white dark:hover:bg-[#252b3b]/60 transition-colors"
				title="Close Menu"
			>
				<X size={18} />
			</button>
		</div>
	</div>

	<!-- Main Navigation Links -->
	<nav class="flex-1 px-3 py-4 space-y-1 overflow-y-auto">
		<!-- Section: Systems -->
		<div class="px-2 pb-1.5 text-xs font-mono tracking-widest uppercase text-slate-500 dark:text-[#64748b]">
			Telemetry
		</div>

		<a
			href="/"
			onclick={handleLinkClick}
			class="flex items-center gap-2.5 px-3 py-2 rounded-xs text-sm font-mono transition-colors {isOverviewActive ? 'bg-slate-100 text-slate-950 font-bold border border-slate-300 shadow-xs dark:bg-[#d4ff32]/10 dark:text-[#d4ff32] dark:border-[#d4ff32]/20 dark:shadow-none' : 'text-slate-600 hover:bg-slate-100 hover:text-slate-950 dark:text-[#94a3b8] dark:hover:bg-[#181c26] dark:hover:text-[#f1f5f9]'}"
		>
			<LayoutDashboard size={16} />
			<span>Overview</span>
		</a>

		<a
			href="/actions"
			onclick={handleLinkClick}
			class="flex items-center gap-2.5 px-3 py-2 rounded-xs text-sm font-mono transition-colors {isActionsActive ? 'bg-slate-100 text-slate-950 font-bold border border-slate-300 shadow-xs dark:bg-[#d4ff32]/10 dark:text-[#d4ff32] dark:border-[#d4ff32]/20 dark:shadow-none' : 'text-slate-600 hover:bg-slate-100 hover:text-slate-950 dark:text-[#94a3b8] dark:hover:bg-[#181c26] dark:hover:text-[#f1f5f9]'}"
		>
			<Activity size={16} />
			<span>Action Stream</span>
		</a>

		<!-- Section: User Feedback -->
		<div class="pt-5 px-2 pb-1.5 text-xs font-mono tracking-widest uppercase text-slate-500 dark:text-[#64748b]">
			Support
		</div>

		<a
			href="/issues"
			onclick={handleLinkClick}
			class="flex items-center justify-between px-3 py-2 rounded-xs text-sm font-mono transition-colors {isIssuesActive ? 'bg-slate-100 text-slate-950 font-bold border border-slate-300 shadow-xs dark:bg-[#d4ff32]/10 dark:text-[#d4ff32] dark:border-[#d4ff32]/20 dark:shadow-none' : 'text-slate-600 hover:bg-slate-100 hover:text-slate-950 dark:text-[#94a3b8] dark:hover:bg-[#181c26] dark:hover:text-[#f1f5f9]'}"
		>
			<div class="flex items-center gap-2.5">
				<Inbox size={16} />
				<span>Issue Queue</span>
			</div>
			{#if user?.role === 'read-only'}
				<span class="text-[10px] px-1.5 py-0.5 rounded bg-amber-100 text-amber-900 border border-amber-300 dark:bg-amber-500/10 dark:text-amber-400 dark:border-amber-500/30 flex items-center gap-1 font-semibold">
					<Lock size={10} /> RO
				</span>
			{/if}
		</a>

		<!-- Section: Database Storage -->
		<div class="pt-5 px-2 pb-1.5 text-xs font-mono tracking-widest uppercase text-slate-500 dark:text-[#64748b]">
			Storage Engine
		</div>

		<a
			href="/database"
			onclick={handleLinkClick}
			class="flex items-center gap-2.5 px-3 py-2 rounded-xs text-sm font-mono transition-colors {isDatabaseActive ? 'bg-slate-100 text-slate-950 font-bold border border-slate-300 shadow-xs dark:bg-[#d4ff32]/10 dark:text-[#d4ff32] dark:border-[#d4ff32]/20 dark:shadow-none' : 'text-slate-600 hover:bg-slate-100 hover:text-slate-950 dark:text-[#94a3b8] dark:hover:bg-[#181c26] dark:hover:text-[#f1f5f9]'}"
		>
			<Database size={16} />
			<span>Tables & Rows</span>
		</a>

		<a
			href="/database/query"
			onclick={handleLinkClick}
			class="flex items-center justify-between px-3 py-2 rounded-xs text-sm font-mono transition-colors {isQueryActive ? 'bg-slate-100 text-slate-950 font-bold border border-slate-300 shadow-xs dark:bg-[#d4ff32]/10 dark:text-[#d4ff32] dark:border-[#d4ff32]/20 dark:shadow-none' : 'text-slate-600 hover:bg-slate-100 hover:text-slate-950 dark:text-[#94a3b8] dark:hover:bg-[#181c26] dark:hover:text-[#f1f5f9]'}"
		>
			<div class="flex items-center gap-2.5">
				<TerminalSquare size={16} />
				<span>SQL Workspace</span>
			</div>
			{#if user?.role === 'read-only'}
				<span class="text-[10px] px-1.5 py-0.5 rounded bg-amber-100 text-amber-900 border border-amber-300 dark:bg-amber-500/10 dark:text-amber-400 dark:border-amber-500/30 flex items-center gap-1 font-semibold">
					<Lock size={10} /> RO
				</span>
			{/if}
		</a>

		<!-- Section: Security & Access -->
		<div class="pt-5 px-2 pb-1.5 text-xs font-mono tracking-widest uppercase text-slate-500 dark:text-[#64748b]">
			Governance
		</div>

		{#if user?.isSuperadmin}
			<a
				href="/admins"
				onclick={handleLinkClick}
				class="flex items-center gap-2.5 px-3 py-2 rounded-xs text-sm font-mono transition-colors {isAdminsActive ? 'bg-slate-100 text-slate-950 font-bold border border-slate-300 shadow-xs dark:bg-[#d4ff32]/10 dark:text-[#d4ff32] dark:border-[#d4ff32]/20 dark:shadow-none' : 'text-slate-600 hover:bg-slate-100 hover:text-slate-950 dark:text-[#94a3b8] dark:hover:bg-[#181c26] dark:hover:text-[#f1f5f9]'}"
			>
				<Users size={16} />
				<span>Admin Access</span>
			</a>
		{:else}
			<div
				class="flex items-center justify-between px-3 py-2 rounded-xs text-sm font-mono text-slate-400 dark:text-[#475569] cursor-not-allowed opacity-60"
				title="Restricted: Requires Superadmin"
			>
				<div class="flex items-center gap-2.5">
					<Users size={16} />
					<span>Admin Access</span>
				</div>
				<Lock size={13} />
			</div>
		{/if}

		<a
			href="/settings"
			onclick={handleLinkClick}
			class="flex items-center gap-2.5 px-3 py-2 rounded-xs text-sm font-mono transition-colors {isSettingsActive ? 'bg-slate-100 text-slate-950 font-bold border border-slate-300 shadow-xs dark:bg-[#d4ff32]/10 dark:text-[#d4ff32] dark:border-[#d4ff32]/20 dark:shadow-none' : 'text-slate-600 hover:bg-slate-100 hover:text-slate-950 dark:text-[#94a3b8] dark:hover:bg-[#181c26] dark:hover:text-[#f1f5f9]'}"
		>
			<Settings size={16} />
			<span>Settings</span>
		</a>
	</nav>
</aside>
