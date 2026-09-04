<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { Settings, Save, Trash2, Clock, CheckCircle, ExternalLink, RefreshCw } from 'lucide-svelte';

	const user = $derived(page.data.user);

	let retentionHours = $state<number>(72);
	let isLoading = $state<boolean>(true);
	let isSaving = $state<boolean>(false);
	let isCleaning = $state<boolean>(false);

	let saveSuccess = $state<string | null>(null);
	let cleanupSuccess = $state<string | null>(null);
	let error = $state<string | null>(null);

	async function loadSettings() {
		isLoading = true;
		error = null;
		try {
			const res = await fetch('/api/settings');
			if (res.ok) {
				const data = await res.json();
				retentionHours = data.retention_hours || 72;
			}
		} catch (err: any) {
			error = err?.message || 'Failed to load settings';
		} finally {
			isLoading = false;
		}
	}

	async function handleSaveSettings() {
		if (user?.role === 'read-only') return;
		isSaving = true;
		saveSuccess = null;
		error = null;

		try {
			const res = await fetch('/api/settings', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ retention_hours: retentionHours })
			});

			if (res.ok) {
				saveSuccess = `Retention period updated to ${retentionHours} hours.`;
			} else {
				const data = await res.json().catch(() => null);
				error = data?.error || 'Failed to update retention policy';
			}
		} catch (err: any) {
			error = err?.message || 'Network error';
		} finally {
			isSaving = false;
		}
	}

	async function handleTriggerCleanup() {
		isCleaning = true;
		cleanupSuccess = null;
		error = null;

		try {
			const res = await fetch('/api/cron/cleanup', {
				method: 'POST'
			});

			const data = await res.json();
			if (res.ok) {
				cleanupSuccess = `Cleanup complete: Purged ${data.deleted_count} action records older than ${data.retention_hours} hours.`;
			} else {
				error = data?.error || 'Failed to run cleanup';
			}
		} catch (err: any) {
			error = err?.message || 'Network error';
		} finally {
			isCleaning = false;
		}
	}

	onMount(() => {
		loadSettings();
	});
</script>

<svelte:head>
	<title>KPI Schedule | System Settings</title>
</svelte:head>

<div class="space-y-6">
	<!-- Title Bar -->
	<div class="border-b border-slate-300 dark:border-[#252b3b] pb-4 flex flex-col sm:flex-row sm:items-center justify-between gap-4">
		<div>
			<div class="flex items-center gap-2 mb-1">
				<Settings size={22} class="text-emerald-600 dark:text-[#d4ff32]" />
				<h1 class="font-display font-extrabold text-2xl sm:text-3xl text-slate-950 dark:text-[#f1f5f9] tracking-tight">
					SYSTEM & RETENTION SETTINGS
				</h1>
			</div>
		</div>
	</div>

	<!-- Alerts -->
	{#if saveSuccess}
		<div class="p-3.5 border border-emerald-300 bg-emerald-100 text-emerald-900 dark:border-[#10b981]/40 dark:bg-[#10b981]/10 dark:text-[#a7f3d0] text-xs font-mono rounded-xs flex items-center gap-2">
			<CheckCircle size={15} class="text-emerald-600 dark:text-[#10b981]" />
			<span>{saveSuccess}</span>
		</div>
	{/if}

	{#if cleanupSuccess}
		<div class="p-3.5 border border-sky-300 bg-sky-100 text-sky-900 dark:border-[#06b6d4]/40 dark:bg-[#06b6d4]/10 dark:text-[#a5f3fc] text-xs font-mono rounded-xs flex items-center gap-2">
			<CheckCircle size={15} class="text-sky-600 dark:text-[#06b6d4]" />
			<span>{cleanupSuccess}</span>
		</div>
	{/if}

	{#if error}
		<div class="p-3.5 border border-red-300 bg-red-100 text-red-900 dark:border-[#ef4444]/40 dark:bg-[#ef4444]/10 dark:text-[#fca5a5] text-xs font-mono rounded-xs">
			<div class="font-bold uppercase mb-0.5">[Settings Error]</div>
			<div>{error}</div>
		</div>
	{/if}

	<!-- Retention Policy Card -->
	<div class="border border-slate-300 dark:border-[#252b3b] bg-white dark:bg-[#12151d] p-5 rounded-xs space-y-4 font-mono text-xs sm:text-sm shadow-xs dark:shadow-none">
		<div class="flex items-center gap-2">
			<Clock size={16} class="text-emerald-600 dark:text-[#d4ff32]" />
			<h2 class="text-sm sm:text-base font-bold text-slate-950 dark:text-[#f1f5f9] font-display">
				Action Telemetry Retention Policy
			</h2>
		</div>

		<p class="text-slate-600 dark:text-[#94a3b8] leading-relaxed max-w-2xl text-xs sm:text-sm">
			Action records (Bot commands, extension syncs, cron alert logs) are stored in DB.
			Actions older than this threshold will be pruned by scheduled cleanup jobs.
		</p>

		<div class="flex flex-col sm:flex-row sm:items-end gap-3 max-w-md pt-2">
			<div class="space-y-1.5 flex-1">
				<label for="retention-hours" class="text-xs uppercase tracking-wider text-slate-500 dark:text-[#64748b] block">
					Retention Window (Hours)
				</label>
				<input
					id="retention-hours"
					type="number"
					min="1"
					max="720"
					bind:value={retentionHours}
					disabled={user?.role === 'read-only'}
					class="w-full px-3 py-2 bg-slate-50 dark:bg-[#0a0b0e] border border-slate-300 dark:border-[#252b3b] focus:border-slate-600 dark:focus:border-[#d4ff32] text-xs sm:text-sm font-mono text-slate-950 dark:text-[#f1f5f9] rounded-xs focus:outline-none"
				/>
			</div>

			{#if user?.role !== 'read-only'}
				<button
					onclick={handleSaveSettings}
					disabled={isSaving}
					class="flex items-center justify-center gap-2 py-2 px-5 border border-black dark:border-[#d4ff32]/50 bg-[#ccf600] hover:bg-[#bde800] dark:bg-[#d4ff32] dark:hover:bg-[#e2f952] text-black font-mono text-xs sm:text-sm font-bold uppercase tracking-wider rounded-xs transition-all shadow-[2px_2px_0px_#090d16] dark:shadow-[0_0_15px_rgba(212,255,50,0.2)] disabled:opacity-50 cursor-pointer"
				>
					<Save size={14} />
					<span>{isSaving ? 'Saving...' : 'Save Policy'}</span>
				</button>
			{/if}
		</div>

		<div class="text-[11px] text-slate-500 dark:text-[#64748b]">
			Default: 72 hours (3 days). 168 hours = 7 days. Maximum capped at 720 hours (30 days).
		</div>
	</div>

	<!-- Manual Maintenance Execution Card -->
	<div class="border border-slate-300 dark:border-[#252b3b] bg-white dark:bg-[#12151d] p-5 rounded-xs space-y-4 font-mono text-xs sm:text-sm shadow-xs dark:shadow-none">
		<div class="flex items-center gap-2">
			<Trash2 size={16} class="text-amber-500" />
			<h2 class="text-sm sm:text-base font-bold text-slate-950 dark:text-[#f1f5f9] font-display">
				Manual Pruning Trigger
			</h2>
		</div>

		<p class="text-slate-600 dark:text-[#94a3b8] leading-relaxed max-w-2xl text-xs sm:text-sm">
			Manually trigger an immediate database cleanup to delete any action records older than {retentionHours} hours.
		</p>

		<button
			onclick={handleTriggerCleanup}
			disabled={isCleaning}
			class="flex items-center gap-2 py-2 px-4 border border-slate-300 hover:border-amber-400 bg-slate-100 hover:bg-slate-200 text-amber-800 dark:border-[#252b3b] dark:hover:border-amber-500/50 dark:bg-[#181c26] dark:hover:bg-[#252b3b] dark:text-amber-400 font-mono text-xs sm:text-sm font-bold uppercase tracking-wider rounded-xs transition-colors disabled:opacity-50 cursor-pointer shadow-xs dark:shadow-none"
		>
			<RefreshCw size={13} class={isCleaning ? 'animate-spin' : ''} />
			<span>{isCleaning ? 'Pruning...' : 'Run Cleanup Now'}</span>
		</button>
	</div>
</div>
