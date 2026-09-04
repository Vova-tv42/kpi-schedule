<script lang="ts">
	import { page } from '$app/state';
	import ThemeToggle from '$lib/components/ThemeToggle.svelte';
	import { ShieldCheck, ShieldAlert, TerminalSquare, ArrowRight, Lock, KeyRound } from 'lucide-svelte';

	const errorParam = $derived(page.url.searchParams.get('error'));
	const attemptedEmail = $derived(page.url.searchParams.get('email'));

	function getErrorMessage(err: string | null): string {
		switch (err) {
			case 'forbidden':
			case 'access_denied':
				return 'Access Denied: Your Google account is not an authorized administrator. In accordance with security policy, unauthorized accounts are rejected and no data is stored.';
			case 'invalid_state':
				return 'OAuth session expired or state mismatch. Please attempt sign-in again.';
			case 'token_exchange_failed':
				return 'Failed to exchange authorization token with Google OAuth provider.';
			case 'userinfo_failed':
				return 'Failed to fetch user profile from Google OpenID directory.';
			case 'no_email':
			case 'unverified_email':
				return 'Unverified Google email address. A verified email account is required.';
			default:
				return err ? `Authentication failed: ${err}` : '';
		}
	}
</script>

<svelte:head>
	<title>KPI Schedule | Mission Control Sign In</title>
</svelte:head>

<div class="min-h-screen bg-[#f4f5f8] dark:bg-[#0a0b0e] bg-tech-grid flex flex-col items-center justify-center p-6 relative">
	<!-- Top right theme toggle for public access -->
	<div class="absolute top-4 right-4 sm:top-6 sm:right-6">
		<ThemeToggle />
	</div>

	<!-- Main Login Card -->
	<div class="w-full max-w-md bg-white dark:bg-[#12151d] border border-slate-300 dark:border-[#252b3b] shadow-2xl rounded-xs p-8 relative overflow-hidden">
		<!-- Accent corner marker -->
		<div class="absolute top-0 right-0 w-16 h-16 pointer-events-none overflow-hidden">
			<div class="absolute transform rotate-45 bg-[#ccf600] dark:bg-[#d4ff32] text-black font-mono font-bold text-[10px] py-0.5 right-[-35px] top-[18px] w-[120px] text-center tracking-widest uppercase shadow-xs">
				ADMIN ONLY
			</div>
		</div>

		<!-- Branding -->
		<div class="flex items-center gap-3 mb-6">
			<div class="w-10 h-10 rounded-xs bg-[#ccf600]/25 text-slate-950 border border-[#a3e635] dark:bg-[#d4ff32]/10 dark:border-[#d4ff32]/30 dark:text-[#d4ff32] flex items-center justify-center">
				<TerminalSquare size={22} />
			</div>
			<div>
				<h1 class="font-display font-extrabold text-2xl text-slate-950 dark:text-[#f1f5f9] tracking-tight">
					KPI SCHEDULE // CONSOLE
				</h1>
				<p class="font-mono text-xs text-slate-500 dark:text-[#94a3b8]">
					Personalized Schedule Platform Administration
				</p>
			</div>
		</div>

		<!-- Error Banner -->
		{#if errorParam}
			<div class="mb-6 p-3.5 bg-red-100 border border-red-300 rounded-xs text-xs font-mono text-red-900 dark:bg-[#ef4444]/10 dark:border-[#ef4444]/30 dark:text-[#fca5a5] flex items-start gap-2.5 animate-in fade-in duration-200">
				<ShieldAlert size={16} class="text-red-600 dark:text-[#ef4444] shrink-0 mt-0.5" />
				<div class="leading-relaxed">
					{getErrorMessage(errorParam)}
					{#if attemptedEmail}
						<div class="text-slate-600 dark:text-[#94a3b8] mt-1">Identity: <span class="text-slate-950 dark:text-white underline">{attemptedEmail}</span></div>
					{/if}
				</div>
			</div>
		{/if}

		<!-- Security Notice -->
		<div class="mb-6 p-3.5 bg-slate-50 border border-slate-300 dark:bg-[#181c26] dark:border-[#252b3b] rounded-xs text-xs font-mono text-slate-600 dark:text-[#94a3b8]">
			<div class="flex items-center gap-2 text-slate-950 dark:text-[#f1f5f9] font-medium mb-1.5 text-sm">
				<Lock size={14} class="text-emerald-700 dark:text-[#d4ff32]" />
				<span>Single Sign-On Security Rule</span>
			</div>
			<p class="text-[11px] leading-relaxed">
				Access is restricted to authorized administrative emails. Unrecognized accounts are blocked with zero database footprint.
			</p>
		</div>

		<!-- Google OAuth Button -->
		<a
			href="/auth/google"
			class="w-full py-3 px-4 border border-black dark:border-transparent bg-[#ccf600] hover:bg-[#bde800] dark:bg-[#d4ff32] dark:hover:bg-[#e2f952] text-black font-mono font-bold text-sm tracking-wider uppercase rounded-xs flex items-center justify-center gap-2.5 transition-all shadow-[2px_2px_0px_#090d16] dark:shadow-[0_0_20px_rgba(212,255,50,0.15)] dark:hover:shadow-[0_0_25px_rgba(212,255,50,0.3)] active:scale-[0.99] cursor-pointer"
		>
			<svg class="w-4 h-4" viewBox="0 0 24 24">
				<path fill="#000000" d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92c-.26 1.37-1.04 2.53-2.21 3.31v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.09z"/>
				<path fill="#000000" d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z"/>
				<path fill="#000000" d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.06H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.94l2.85-2.22.81-.63z"/>
				<path fill="#000000" d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.06l3.66 2.84c.87-2.6 3.3-4.52 6.16-4.52z"/>
			</svg>
			<span>Authorize with Google</span>
			<ArrowRight size={15} />
		</a>

		<!-- Bottom Security Telemetry Notes -->
		<div class="mt-8 pt-4 border-t border-slate-200 dark:border-[#252b3b]/60 flex items-center justify-between font-mono text-[11px] text-slate-500 dark:text-[#64748b]">
			<div class="flex items-center gap-1.5">
				<ShieldCheck size={13} class="text-emerald-600 dark:text-[#10b981]" />
				<span>HMAC-SHA256 JWT Signed</span>
			</div>
			<div class="flex items-center gap-1.5">
				<KeyRound size={13} class="text-emerald-700 dark:text-[#d4ff32]" />
				<span>Role-Enforced RBAC</span>
			</div>
		</div>
	</div>
</div>
