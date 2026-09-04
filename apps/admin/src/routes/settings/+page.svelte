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
	<title>KPI Schedule // System Settings</title>
</svelte:head>

<div class="space-y-6">
	<!-- Title Bar -->
	<div class="border-b border-[#252b3b] pb-4 flex flex-col sm:flex-row sm:items-center justify-between gap-4">
		<div>
			<div class="flex items-center gap-2 mb-1">
				<Settings size={22} class="text-[#d4ff32]" />
				<h1 class="font-display font-extrabold text-2xl sm:text-3xl text-[#f1f5f9] tracking-tight">
					SYSTEM & RETENTION SETTINGS
				</h1>
			</div>
		</div>
	</div>

	<!-- Alerts -->
	{#if saveSuccess}
		<div class="p-3.5 border border-[#10b981]/40 bg-[#10b981]/10 text-[#a7f3d0] text-xs font-mono rounded-xs flex items-center gap-2">
			<CheckCircle size={15} class="text-[#10b981]" />
			<span>{saveSuccess}</span>
		</div>
	{/if}

	{#if cleanupSuccess}
		<div class="p-3.5 border border-[#06b6d4]/40 bg-[#06b6d4]/10 text-[#a5f3fc] text-xs font-mono rounded-xs flex items-center gap-2">
			<CheckCircle size={15} class="text-[#06b6d4]" />
			<span>{cleanupSuccess}</span>
		</div>
	{/if}

	{#if error}
		<div class="p-3.5 border border-[#ef4444]/40 bg-[#ef4444]/10 text-[#fca5a5] text-xs font-mono rounded-xs">
			<div class="font-bold text-[#ef4444] uppercase mb-0.5">[Settings Error]</div>
			<div>{error}</div>
		</div>
	{/if}

	<!-- Retention Policy Card -->
	<div class="border border-[#252b3b] bg-[#12151d] p-5 rounded-xs space-y-4 font-mono text-xs sm:text-sm">
		<div class="flex items-center gap-2">
			<Clock size={16} class="text-[#d4ff32]" />
			<h2 class="text-sm sm:text-base font-bold text-[#f1f5f9] font-display">
				Action Telemetry Retention Policy
			</h2>
		</div>

		<p class="text-[#94a3b8] leading-relaxed max-w-2xl text-xs sm:text-sm">
			Action records (Bot commands, extension syncs, cron alert logs) are stored in DB.
			Actions older than this threshold will be pruned by scheduled cleanup jobs.
		</p>

		<div class="flex flex-col sm:flex-row sm:items-end gap-3 max-w-md pt-2">
			<div class="space-y-1.5 flex-1">
				<label for="retention-hours" class="text-xs uppercase tracking-wider text-[#64748b] block">
					Retention Window (Hours)
				</label>
				<input
					id="retention-hours"
					type="number"
					min="1"
					max="720"
					bind:value={retentionHours}
					disabled={user?.role === 'read-only'}
					class="w-full px-3 py-2 bg-[#0a0b0e] border border-[#252b3b] focus:border-[#d4ff32] text-xs sm:text-sm font-mono text-[#f1f5f9] rounded-xs focus:outline-none"
				/>
			</div>

			{#if user?.role !== 'read-only'}
				<button
					onclick={handleSaveSettings}
					disabled={isSaving}
					class="flex items-center justify-center gap-2 py-2 px-5 border border-[#d4ff32]/50 bg-[#d4ff32] hover:bg-[#e2f952] text-black font-mono text-xs sm:text-sm font-bold uppercase tracking-wider rounded-xs transition-all shadow-[0_0_15px_rgba(212,255,50,0.2)] disabled:opacity-50 cursor-pointer"
				>
					<Save size={14} />
					<span>{isSaving ? 'Saving...' : 'Save Policy'}</span>
				</button>
			{/if}
		</div>

		<div class="text-[11px] text-[#64748b]">
			Default: 72 hours (3 days). 168 hours = 7 days. Maximum capped at 720 hours (30 days).
		</div>
	</div>

	<!-- Manual Maintenance Execution Card -->
	<div class="border border-[#252b3b] bg-[#12151d] p-5 rounded-xs space-y-4 font-mono text-xs sm:text-sm">
		<div class="flex items-center gap-2">
			<Trash2 size={16} class="text-amber-400" />
			<h2 class="text-sm sm:text-base font-bold text-[#f1f5f9] font-display">
				Manual Pruning Trigger
			</h2>
		</div>

		<p class="text-[#94a3b8] leading-relaxed max-w-2xl text-xs sm:text-sm">
			Manually trigger an immediate database cleanup to delete any action records older than {retentionHours} hours.
		</p>

		<button
			onclick={handleTriggerCleanup}
			disabled={isCleaning}
			class="flex items-center gap-2 py-2 px-4 border border-[#252b3b] hover:border-amber-500/50 bg-[#181c26] hover:bg-[#252b3b] text-amber-400 font-mono text-xs sm:text-sm font-bold uppercase tracking-wider rounded-xs transition-colors disabled:opacity-50 cursor-pointer"
		>
			<RefreshCw size={13} class={isCleaning ? 'animate-spin' : ''} />
			<span>{isCleaning ? 'Pruning...' : 'Run Cleanup Now'}</span>
		</button>
	</div>
</div>
