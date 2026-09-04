<script lang="ts">
	import './layout.css';
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

{#if isPublicRoute || !user}
	<main class="min-h-screen bg-[#0a0b0e] text-[#e6edf3]">
		{@render children()}
	</main>
{:else}
	<div class="flex h-screen overflow-hidden bg-[#0a0b0e]">
		<!-- Left Sidebar Rail -->
		<Navbar {user} mobileOpen={mobileMenuOpen} onCloseMobile={closeMobileMenu} />

		<!-- Main Workspace Area -->
		<div class="flex-1 flex flex-col min-w-0 overflow-hidden">
			<TelemetryHeader {user} onToggleMobile={toggleMobileMenu} />
			<main class="flex-1 overflow-y-auto bg-[#0a0b0e] p-4 sm:p-6">
				{@render children()}
			</main>
		</div>
	</div>
{/if}
