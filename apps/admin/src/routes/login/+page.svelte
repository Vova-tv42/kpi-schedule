<script lang="ts">
	import { page } from '$app/state';

	const error = $derived(page.url.searchParams.get('error'));
	const attemptedEmail = $derived(page.url.searchParams.get('email'));

	const errorMessages: Record<string, string> = {
		forbidden: 'Access Denied: Your account is not on the admin whitelist. No data was stored.',
		invalid_state: 'Security validation failed: Invalid OAuth state. Please try again.',
		token_exchange_failed: 'Failed to exchange authorization token with Google.',
		userinfo_failed: 'Failed to fetch user profile from Google OpenID.',
		no_email: 'Your Google profile does not provide a verified email address.'
	};
</script>

<svelte:head>
	<title>KPI Schedule Console | Sign In</title>
</svelte:head>

<div class="min-h-screen flex items-center justify-center p-4 bg-[#080b10] text-[#f8fafc]">
	<div class="w-full max-w-md border border-[#1e293b] bg-[#0f141d] p-8 shadow-2xl relative">
		<!-- Precision Corner Accents -->
		<div class="absolute -top-[1px] -left-[1px] w-2 h-2 border-t-2 border-l-2 border-cyan-500"></div>
		<div class="absolute -top-[1px] -right-[1px] w-2 h-2 border-t-2 border-r-2 border-cyan-500"></div>
		<div class="absolute -bottom-[1px] -left-[1px] w-2 h-2 border-b-2 border-l-2 border-cyan-500"></div>
		<div class="absolute -bottom-[1px] -right-[1px] w-2 h-2 border-b-2 border-r-2 border-cyan-500"></div>

		<!-- Header -->
		<div class="mb-8 border-b border-[#1e293b] pb-5">
			<div class="flex items-center gap-2 mb-2">
				<span class="inline-block w-2.5 h-2.5 bg-emerald-500 rounded-full animate-pulse"></span>
				<span class="text-sm font-mono tracking-widest uppercase text-slate-400">KPI Core Ops</span>
			</div>
			<h1 class="text-3xl font-bold tracking-tight text-white uppercase font-sans">Admin Console</h1>
		</div>

		<!-- Error Banner -->
		{#if error}
			<div class="mb-6 p-4 border border-red-500/40 bg-red-950/20 text-red-200 text-sm font-mono space-y-1">
				<div class="flex items-center gap-2 font-bold text-red-400 uppercase">
					<span>[Security Notice]</span>
				</div>
				<p>{errorMessages[error] || 'An unexpected authentication error occurred.'}</p>
				{#if attemptedEmail}
					<p class="text-slate-400 truncate">Identity: <span class="text-red-300 underline">{attemptedEmail}</span></p>
				{/if}
			</div>
		{/if}

		<!-- Sign In Button -->
		<a
			href="/auth/google"
			class="w-full flex items-center justify-center gap-3 py-3 px-4 bg-[#141c28] hover:bg-[#1a2536] border border-[#2d3d52] hover:border-cyan-500 text-white font-mono text-base tracking-wide transition-all duration-150 group"
		>
			<svg class="w-5 h-5" viewBox="0 0 24 24">
				<path
					fill="#4285F4"
					d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92c-.26 1.37-1.04 2.53-2.21 3.31v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.09z"
				/>
				<path
					fill="#34A853"
					d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z"
				/>
				<path
					fill="#FBBC05"
					d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.06H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.94l2.85-2.22.81-.63z"
				/>
				<path
					fill="#EA4335"
					d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.06l3.66 2.84c.87-2.6 3.3-4.52 6.16-4.52z"
				/>
			</svg>
			<span class="group-hover:text-cyan-300 font-medium">Sign in with Google</span>
		</a>

		<!-- Build Info -->
		<div class="mt-8 text-center text-xs font-mono text-slate-500 uppercase tracking-widest">
			KPI Schedule Bot System
		</div>
	</div>
</div>
