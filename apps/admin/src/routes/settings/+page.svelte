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
	<title>KPI Console | Settings</title>
</svelte:head>

<div class="space-y-6">
	<!-- Title Bar -->
	<div class="border-b border-[#1e293b] pb-4">
		<div class="text-xs font-mono uppercase tracking-widest text-cyan-400">[System Policy]</div>
		<h1 class="text-2xl font-bold font-mono text-white tracking-tight uppercase">Settings & Retention</h1>
		<p class="text-sm font-mono text-slate-400 mt-1">Configure action telemetry lifecycle, storage pruning, and cron jobs</p>
	</div>

	<!-- Alerts -->
	{#if saveSuccess}
		<div class="p-4 border border-emerald-500/40 bg-emerald-950/20 text-emerald-300 text-sm font-mono">
			{saveSuccess}
		</div>
	{/if}

	{#if cleanupSuccess}
		<div class="p-4 border border-cyan-500/40 bg-cyan-950/20 text-cyan-300 text-sm font-mono">
			{cleanupSuccess}
		</div>
	{/if}

	{#if error}
		<div class="p-4 border border-red-500/40 bg-red-950/20 text-red-300 text-sm font-mono">
			{error}
		</div>
	{/if}

	<!-- Retention Policy Card -->
	<div class="border border-[#1e293b] bg-[#0f141d] p-5 space-y-4">
		<div class="flex items-center gap-2">
			<Clock class="w-4 h-4 text-cyan-400" />
			<h2 class="text-base font-bold font-mono text-white uppercase">Action Telemetry Retention Policy</h2>
		</div>

		<p class="text-sm font-mono text-slate-300 leading-relaxed max-w-2xl">
			Anonymous action records (Telegram commands, extension syncs, cron alert logs) are stored in NeonDB.
			Actions older than this threshold will be pruned by scheduled cleanup tasks.
		</p>

		<div class="flex flex-col sm:flex-row sm:items-end gap-3 max-w-md pt-2">
			<div class="space-y-1 flex-1">
				<label for="retention-hours" class="text-xs font-mono text-slate-400">Retention Window (Hours)</label>
				<input
					id="retention-hours"
					type="number"
					min="1"
					max="720"
					bind:value={retentionHours}
					disabled={user?.role === 'read-only'}
					class="w-full px-3 py-2 bg-[#080b10] border border-[#1e293b] focus:border-cyan-500 text-sm font-mono text-white focus:outline-none"
				/>
			</div>

			{#if user?.role !== 'read-only'}
				<button
					onclick={handleSaveSettings}
					disabled={isSaving}
					class="flex items-center justify-center gap-2 py-2 px-5 border border-cyan-500 bg-cyan-600 hover:bg-cyan-500 text-black font-mono text-sm font-bold uppercase tracking-wider transition-colors disabled:opacity-50"
				>
					<Save class="w-4 h-4" />
					<span>{isSaving ? 'Saving...' : 'Save Policy'}</span>
				</button>
			{/if}
		</div>

		<div class="text-xs font-mono text-slate-500">
			Default: 72 hours (3 days). 168 hours = 7 days.
		</div>
	</div>

	<!-- Manual Maintenance Execution Card -->
	<div class="border border-[#1e293b] bg-[#0f141d] p-5 space-y-4">
		<div class="flex items-center gap-2">
			<Trash2 class="w-4 h-4 text-amber-400" />
			<h2 class="text-base font-bold font-mono text-white uppercase">Manual Pruning Trigger</h2>
		</div>

		<p class="text-sm font-mono text-slate-300 leading-relaxed max-w-2xl">
			Manually trigger an immediate database cleanup to delete any action logs older than {retentionHours} hours.
		</p>

		<button
			onclick={handleTriggerCleanup}
			disabled={isCleaning}
			class="flex items-center gap-2 py-2 px-4 border border-[#2d3d52] hover:border-amber-500 bg-[#141c28] hover:bg-[#1c2738] text-amber-400 font-mono text-sm font-bold uppercase tracking-wider transition-colors disabled:opacity-50"
		>
			<RefreshCw class="w-4 h-4 {isCleaning ? 'animate-spin' : ''}" />
			<span>{isCleaning ? 'Pruning...' : 'Run Cleanup Now'}</span>
		</button>
	</div>

	<!-- Scheduled External Cron Instructions Card -->
	<div class="border border-[#1e293b] bg-[#0f141d] p-5 space-y-4">
		<div class="flex items-center gap-2">
			<ExternalLink class="w-4 h-4 text-cyan-400" />
			<h2 class="text-base font-bold font-mono text-white uppercase">Vercel Free Tier Cron Configuration</h2>
		</div>

		<div class="text-sm font-mono text-slate-300 space-y-2 leading-relaxed max-w-3xl">
			<p>
				On Vercel's Hobby (free) tier, native cron schedules are capped at once per 24 hours.
				To clean up actions more frequently (e.g. every 6 or 12 hours), configure a free HTTP task on 
				<a href="https://cron-jobs.com" target="_blank" rel="noreferrer" class="text-cyan-400 hover:underline">cron-jobs.com</a> or
				<a href="https://cron-job.org" target="_blank" rel="noreferrer" class="text-cyan-400 hover:underline">cron-job.org</a>:
			</p>

			<div class="p-4 border border-[#1e293b] bg-[#0a0e14] space-y-2">
				<div>
					<span class="text-slate-400">Endpoint URL:</span>
					<code class="text-cyan-300 block mt-0.5">https://&lt;your-admin-app&gt;.vercel.app/api/cron/cleanup</code>
				</div>
				<div>
					<span class="text-slate-400">HTTP Method:</span>
					<code class="text-white block mt-0.5">POST (or GET)</code>
				</div>
				<div>
					<span class="text-slate-400">HTTP Header:</span>
					<code class="text-white block mt-0.5">Authorization: Bearer &lt;CRON_SECRET&gt;</code>
				</div>
				<div>
					<span class="text-slate-400">Schedule:</span>
					<code class="text-white block mt-0.5">Every 6 hours (or 12 hours)</code>
				</div>
			</div>

			<p class="text-slate-400 text-xs">
				Additionally, the ingestion endpoint performs opportunistic cleanup during normal operations, ensuring NeonDB storage never exhausts free tier quotas even if external cron triggers are delayed.
			</p>
		</div>
	</div>
</div>
