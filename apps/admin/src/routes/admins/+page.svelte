<script lang="ts">
	import { onMount } from 'svelte';
	import { Shield, UserPlus, Trash2, Check, RefreshCw, Key, AlertCircle } from 'lucide-svelte';

	interface AdminUser {
		id: string;
		email: string;
		role: 'read-only' | 'read-write';
		created_at: string;
		created_by: string;
	}

	let superadminEmail = $state<string>('');
	let admins = $state<AdminUser[]>([]);
	let isLoading = $state<boolean>(true);
	let error = $state<string | null>(null);

	// New Admin Form State
	let newEmail = $state<string>('');
	let newRole = $state<'read-only' | 'read-write'>('read-only');
	let isAdding = $state<boolean>(false);
	let formSuccess = $state<string | null>(null);
	let formError = $state<string | null>(null);

	async function loadAdmins() {
		isLoading = true;
		error = null;
		try {
			const res = await fetch('/api/admins');
			if (res.ok) {
				const data = await res.json();
				superadminEmail = data.superadmin || '';
				admins = data.admins || [];
			} else {
				const err = await res.json().catch(() => null);
				error = err?.error || `HTTP ${res.status}`;
			}
		} catch (err: any) {
			error = err?.message || 'Failed to fetch admin users';
		} finally {
			isLoading = false;
		}
	}

	async function handleAddAdmin() {
		if (!newEmail.trim() || !newEmail.includes('@')) {
			formError = 'Please enter a valid email address';
			return;
		}

		isAdding = true;
		formError = null;
		formSuccess = null;

		try {
			const res = await fetch('/api/admins', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({
					email: newEmail.trim().toLowerCase(),
					role: newRole
				})
			});

			const data = await res.json();
			if (res.ok) {
				formSuccess = `Admin ${newEmail} added with ${newRole} role.`;
				newEmail = '';
				loadAdmins();
			} else {
				formError = data?.error || 'Failed to add admin user';
			}
		} catch (err: any) {
			formError = err?.message || 'Network error';
		} finally {
			isAdding = false;
		}
	}

	async function handleRoleChange(id: string, role: 'read-only' | 'read-write') {
		try {
			const res = await fetch('/api/admins', {
				method: 'PATCH',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ id, role })
			});
			if (res.ok) {
				loadAdmins();
			}
		} catch (err) {
			console.error('Failed to update admin role:', err);
		}
	}

	async function handleDeleteAdmin(id: string, email: string) {
		if (!confirm(`Are you sure you want to revoke access for ${email}? All active sessions for this account will be invalidated immediately.`)) {
			return;
		}

		try {
			const res = await fetch(`/api/admins?id=${id}`, {
				method: 'DELETE'
			});
			if (res.ok) {
				loadAdmins();
			}
		} catch (err) {
			console.error('Failed to delete admin:', err);
		}
	}

	onMount(() => {
		loadAdmins();
	});
</script>

<svelte:head>
	<title>KPI Console | Admin Access Control</title>
</svelte:head>

<div class="space-y-6">
	<!-- Title Bar -->
	<div class="border-b border-[#1e293b] pb-4">
		<div class="text-xs font-mono uppercase tracking-widest text-cyan-400">[Security Perimeter]</div>
		<h1 class="text-2xl font-bold font-mono text-white tracking-tight uppercase">Admin Access Control</h1>
		<p class="text-sm font-mono text-slate-400 mt-1">Whitelist management for secondary console administrators</p>
	</div>

	<!-- Add Admin Form Card -->
	<div class="border border-[#1e293b] bg-[#0f141d] p-5 space-y-4">
		<div class="flex items-center gap-2">
			<UserPlus class="w-4 h-4 text-cyan-400" />
			<h2 class="text-base font-bold font-mono text-white uppercase">Authorize New Administrator</h2>
		</div>

		{#if formSuccess}
			<div class="p-3 border border-emerald-500/40 bg-emerald-950/20 text-emerald-300 text-sm font-mono">
				{formSuccess}
			</div>
		{/if}

		{#if formError}
			<div class="p-3 border border-red-500/40 bg-red-950/20 text-red-300 text-sm font-mono">
				{formError}
			</div>
		{/if}

		<form onsubmit={(e) => { e.preventDefault(); handleAddAdmin(); }} class="grid grid-cols-1 md:grid-cols-3 gap-3 items-end">
			<div class="space-y-1">
				<label for="admin-email" class="text-xs font-mono text-slate-400">Google Account Email</label>
				<input
					id="admin-email"
					type="email"
					bind:value={newEmail}
					placeholder="colleague@gmail.com"
					required
					class="w-full px-3 py-2 bg-[#080b10] border border-[#1e293b] focus:border-cyan-500 text-sm font-mono text-white placeholder-slate-600 focus:outline-none"
				/>
			</div>

			<div class="space-y-1">
				<label for="admin-role" class="text-xs font-mono text-slate-400">Assigned Privilege Role</label>
				<select
					id="admin-role"
					bind:value={newRole}
					class="w-full px-3 py-2 bg-[#080b10] border border-[#1e293b] focus:border-cyan-500 text-sm font-mono text-white focus:outline-none"
				>
					<option value="read-only">read-only (Inspect tables & telemetry only)</option>
					<option value="read-write">read-write (Edit rows & run custom SQL)</option>
				</select>
			</div>

			<div>
				<button
					type="submit"
					disabled={isAdding}
					class="w-full flex items-center justify-center gap-2 py-2 px-4 border border-cyan-500 bg-cyan-600 hover:bg-cyan-500 text-black font-mono text-sm font-bold uppercase tracking-wider transition-colors disabled:opacity-50"
				>
					<UserPlus class="w-4 h-4" />
					<span>{isAdding ? 'Adding...' : 'Authorize Admin'}</span>
				</button>
			</div>
		</form>
	</div>

	<!-- Whitelist Table -->
	<div class="border border-[#1e293b] bg-[#0f141d] overflow-hidden space-y-3">
		<div class="p-4 bg-[#0a0e14] border-b border-[#1e293b] flex items-center justify-between">
			<div class="flex items-center gap-2">
				<Shield class="w-4 h-4 text-cyan-400" />
				<span class="text-sm font-mono font-bold uppercase text-white">Active Authorized Administrators</span>
			</div>
			<button onclick={loadAdmins} class="text-slate-400 hover:text-white p-1">
				<RefreshCw class="w-4 h-4 {isLoading ? 'animate-spin' : ''}" />
			</button>
		</div>

		<div class="overflow-x-auto">
			<table class="w-full text-left text-sm font-mono">
				<thead class="bg-[#0e131b] border-b border-[#1e293b] text-xs uppercase text-slate-400">
					<tr>
						<th class="py-2.5 px-4 font-medium">Administrator Identity</th>
						<th class="py-2.5 px-4 font-medium">Privilege Role</th>
						<th class="py-2.5 px-4 font-medium">Authorized At</th>
						<th class="py-2.5 px-4 font-medium text-right">Actions</th>
					</tr>
				</thead>
				<tbody class="divide-y divide-[#1e293b]">
					<!-- Superadmin Root Row -->
					{#if superadminEmail}
						<tr class="bg-cyan-950/10">
							<td class="py-3 px-4 text-white font-medium">
								<div class="flex items-center gap-2">
									<Key class="w-4 h-4 text-cyan-400" />
									<span>{superadminEmail}</span>
									<span class="px-1.5 py-0.5 text-xs border border-cyan-500/50 bg-cyan-950/40 text-cyan-300 font-bold uppercase">
										ROOT SUPERADMIN (.env)
									</span>
								</div>
							</td>
							<td class="py-3 px-4 text-cyan-400 font-bold">
								superadmin (Full Root)
							</td>
							<td class="py-3 px-4 text-slate-500">
								System Configuration
							</td>
							<td class="py-3 px-4 text-right text-slate-600 text-xs">
								IMMUTABLE
							</td>
						</tr>
					{/if}

					<!-- Secondary Admins -->
					{#if admins.length === 0}
						<tr>
							<td colspan="4" class="py-6 text-center text-slate-500 font-mono">
								No secondary administrators currently provisioned.
							</td>
						</tr>
					{:else}
						{#each admins as admin (admin.id)}
							<tr class="hover:bg-[#141c28] transition-colors">
								<td class="py-3 px-4 text-slate-200 font-medium">
									{admin.email}
								</td>
								<td class="py-3 px-4">
									<select
										value={admin.role}
										onchange={(e) => handleRoleChange(admin.id, e.currentTarget.value as any)}
										class="bg-[#080b10] border border-[#1e293b] text-sm font-mono text-slate-300 px-2.5 py-1 focus:border-cyan-500 focus:outline-none"
									>
										<option value="read-only">read-only</option>
										<option value="read-write">read-write</option>
									</select>
								</td>
								<td class="py-3 px-4 text-slate-400">
									{new Date(admin.created_at).toLocaleDateString()}
								</td>
								<td class="py-3 px-4 text-right">
									<button
										onclick={() => handleDeleteAdmin(admin.id, admin.email)}
										class="p-1 border border-[#1e293b] hover:border-red-500/60 bg-[#121822] hover:bg-red-950/20 text-slate-400 hover:text-red-400 transition-colors"
										title="Revoke Admin Access"
									>
										<Trash2 class="w-4 h-4" />
									</button>
								</td>
							</tr>
						{/each}
					{/if}
				</tbody>
			</table>
		</div>
	</div>
</div>
