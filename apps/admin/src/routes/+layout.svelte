<script lang="ts">
	import './layout.css';
	import { ModeWatcher } from 'mode-watcher';
	import { page } from '$app/state';
	import Navbar from '$lib/components/Navbar.svelte';
	import TelemetryHeader from '$lib/components/TelemetryHeader.svelte';

	let { data, children } = $props();
	const user = $derived(data.user);
	const pathname = $derived(page.url.pathname);

	let isPublicRoute = $derived(
		pathname === '/login' || pathname.startsWith('/auth/')
	);

	let mobileMenuOpen = $state(false);

	function toggleMobileMenu() {
		mobileMenuOpen = !mobileMenuOpen;
	}

	function closeMobileMenu() {
		mobileMenuOpen = false;
	}
</script>

<ModeWatcher defaultMode="dark" track={true} />

{#if isPublicRoute || !user}
	<main class="min-h-screen bg-[#f4f5f8] dark:bg-[#0a0b0e] text-[#090d16] dark:text-[#e6edf3]">
		{@render children()}
	</main>
{:else}
	<div class="flex h-screen overflow-hidden bg-[#f4f5f8] dark:bg-[#0a0b0e] text-[#090d16] dark:text-[#e6edf3]">
		<!-- Left Sidebar Rail -->
		<Navbar {user} mobileOpen={mobileMenuOpen} onCloseMobile={closeMobileMenu} />

		<!-- Main Workspace Area -->
		<div class="flex-1 flex flex-col min-w-0 overflow-hidden">
			<TelemetryHeader {user} onToggleMobile={toggleMobileMenu} />
			<main class="flex-1 overflow-y-auto bg-[#f4f5f8] dark:bg-[#0a0b0e] p-4 sm:p-6">
				{@render children()}
			</main>
		</div>
	</div>
{/if}
