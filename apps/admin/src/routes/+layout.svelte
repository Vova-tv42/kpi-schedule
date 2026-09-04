<script lang="ts">
	import './layout.css';
	import { page } from '$app/state';
	import ServerStatusBadge from '$lib/components/ServerStatusBadge.svelte';
	import { Activity, Database, Terminal, Shield, Settings, LogOut } from 'lucide-svelte';

	let { data, children } = $props();
	const user = $derived(data.user);
	const pathname = $derived(page.url.pathname);
</script>

{#if user}
	<div class="min-h-screen bg-[#080b10] text-[#f8fafc] flex flex-col font-sans">
		<!-- Top Tactical Command Rail -->
		<header class="border-b border-[#1e293b] bg-[#0c1017] sticky top-0 z-40">
			<div class="max-w-7xl mx-auto px-4 sm:px-6 flex items-center justify-between h-14">
				<!-- Brand & Server Status -->
				<div class="flex items-center gap-6">
					<div class="flex items-center gap-2">
						<div class="w-2.5 h-2.5 bg-cyan-500 rounded-none transform rotate-45"></div>
						<span class="font-bold tracking-tight text-white uppercase text-base font-sans">KPI Core Console</span>
					</div>

					<div class="hidden md:block border-l border-[#1e293b] pl-6">
						<ServerStatusBadge />
					</div>
				</div>

				<!-- Navigation Tabs -->
				<nav class="hidden lg:flex items-center gap-1">
					<a
						href="/actions"
						class="flex items-center gap-2 px-3 py-1.5 text-sm font-mono tracking-wide transition-colors {pathname ===
						'/actions'
							? 'bg-[#141c28] text-cyan-400 border-b-2 border-cyan-400 font-semibold'
							: 'text-slate-400 hover:text-slate-200 hover:bg-[#121822]'}"
					>
						<Activity class="w-4 h-4" />
						<span>Recent Actions</span>
					</a>

					<a
						href="/database"
						class="flex items-center gap-2 px-3 py-1.5 text-sm font-mono tracking-wide transition-colors {pathname.startsWith(
							'/database'
						) && pathname !== '/database/query'
							? 'bg-[#141c28] text-cyan-400 border-b-2 border-cyan-400 font-semibold'
							: 'text-slate-400 hover:text-slate-200 hover:bg-[#121822]'}"
					>
						<Database class="w-4 h-4" />
						<span>DB Tables</span>
					</a>

					{#if user.role !== 'read-only'}
						<a
							href="/database/query"
							class="flex items-center gap-2 px-3 py-1.5 text-sm font-mono tracking-wide transition-colors {pathname ===
							'/database/query'
								? 'bg-[#141c28] text-cyan-400 border-b-2 border-cyan-400 font-semibold'
								: 'text-slate-400 hover:text-slate-200 hover:bg-[#121822]'}"
						>
							<Terminal class="w-4 h-4" />
							<span>Custom Query</span>
						</a>
					{/if}

					{#if user.isSuperadmin}
						<a
							href="/admins"
							class="flex items-center gap-2 px-3 py-1.5 text-sm font-mono tracking-wide transition-colors {pathname ===
							'/admins'
								? 'bg-[#141c28] text-cyan-400 border-b-2 border-cyan-400 font-semibold'
								: 'text-slate-400 hover:text-slate-200 hover:bg-[#121822]'}"
						>
							<Shield class="w-4 h-4" />
							<span>Admin Users</span>
						</a>
					{/if}

					<a
						href="/settings"
						class="flex items-center gap-2 px-3 py-1.5 text-sm font-mono tracking-wide transition-colors {pathname ===
						'/settings'
							? 'bg-[#141c28] text-cyan-400 border-b-2 border-cyan-400 font-semibold'
							: 'text-slate-400 hover:text-slate-200 hover:bg-[#121822]'}"
					>
						<Settings class="w-4 h-4" />
						<span>Settings</span>
					</a>
				</nav>

				<!-- User Profile & Sign Out -->
				<div class="flex items-center gap-3">
					<div class="hidden sm:flex flex-col items-end">
						<span class="text-sm font-mono text-slate-300 font-medium truncate max-w-[180px]">{user.email}</span>
						<span
							class="text-xs font-mono uppercase px-1.5 py-0.5 mt-0.5 border {user.role === 'superadmin'
								? 'border-cyan-500/40 bg-cyan-950/20 text-cyan-400 font-bold'
								: user.role === 'read-write'
									? 'border-emerald-500/40 bg-emerald-950/20 text-emerald-400'
									: 'border-slate-600 bg-slate-800 text-slate-400'}"
						>
							{user.role}
						</span>
					</div>

					<a
						href="/auth/logout"
						class="p-1.5 border border-[#1e293b] hover:border-red-500/50 hover:bg-red-950/20 hover:text-red-400 text-slate-400 transition-colors"
						title="Sign Out"
					>
						<LogOut class="w-4 h-4" />
					</a>
				</div>
			</div>

			<!-- Mobile Navigation Bar -->
			<div class="flex lg:hidden items-center overflow-x-auto border-t border-[#1e293b] px-4 py-2 gap-2 text-sm font-mono">
				<a href="/actions" class="whitespace-nowrap px-2 py-1 {pathname === '/actions' ? 'text-cyan-400' : 'text-slate-400'}">Actions</a>
				<a href="/database" class="whitespace-nowrap px-2 py-1 {pathname.startsWith('/database') && pathname !== '/database/query' ? 'text-cyan-400' : 'text-slate-400'}">Tables</a>
				{#if user.role !== 'read-only'}
					<a href="/database/query" class="whitespace-nowrap px-2 py-1 {pathname === '/database/query' ? 'text-cyan-400' : 'text-slate-400'}">Query</a>
				{/if}
				{#if user.isSuperadmin}
					<a href="/admins" class="whitespace-nowrap px-2 py-1 {pathname === '/admins' ? 'text-cyan-400' : 'text-slate-400'}">Admins</a>
				{/if}
				<a href="/settings" class="whitespace-nowrap px-2 py-1 {pathname === '/settings' ? 'text-cyan-400' : 'text-slate-400'}">Settings</a>
			</div>
		</header>

		<!-- Main Workspace Viewport -->
		<main class="flex-1 max-w-7xl w-full mx-auto p-4 sm:p-6">
			{@render children()}
		</main>

		<!-- Minimal Footer -->
		<footer class="border-t border-[#1e293b] bg-[#090d14] py-3 text-center text-xs font-mono text-slate-500">
			KPI Personalized Schedule Platform • Anonymous Telemetry Engine • Vercel & Fly.io Scale-to-Zero Architecture
		</footer>
	</div>
{:else}
	{@render children()}
{/if}
