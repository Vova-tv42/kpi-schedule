<script lang="ts">
	import { onMount } from 'svelte';
	import Modal from '$lib/components/Modal.svelte';
	import Badge from '$lib/components/Badge.svelte';
	import {
		Users,
		UserPlus,
		Shield,
		Trash2,
		RefreshCw,
		Key,
		AlertCircle,
		ShieldCheck
	} from 'lucide-svelte';

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

	// Add Admin Modal State
	let addModalOpen = $state<boolean>(false);
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
				addModalOpen = false;
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

	function formatDate(iso: string) {
		try {
			const d = new Date(iso);
			return d.toLocaleDateString([], { year: 'numeric', month: 'short', day: 'numeric' });
		} catch {
			return iso;
		}
	}
</script>

<svelte:head>
	<title>KPI Schedule // Admin Access Governance</title>
</svelte:head>

<div class="space-y-6">
	<!-- Title Bar -->
	<div class="flex flex-col sm:flex-row sm:items-center justify-between gap-4 border-b border-[#252b3b] pb-4">
		<div>
			<div class="flex items-center gap-2 mb-1">
				<Users size={22} class="text-[#d4ff32]" />
				<h1 class="font-display font-extrabold text-2xl sm:text-3xl text-[#f1f5f9] tracking-tight">
					ADMIN ACCESS GOVERNANCE
				</h1>
			</div>
		</div>

		<div class="flex items-center gap-2.5">
			<button
				onclick={loadAdmins}
				disabled={isLoading}
				class="p-2 border border-[#252b3b] hover:border-[#64748b] bg-[#181c26] text-[#94a3b8] hover:text-[#f1f5f9] rounded-xs transition-colors cursor-pointer"
				title="Refresh admin list"
			>
				<RefreshCw size={14} class={isLoading ? 'animate-spin text-[#d4ff32]' : ''} />
			</button>

			<button
				onclick={() => { addModalOpen = true; formError = null; formSuccess = null; }}
				class="flex items-center gap-2 px-3.5 py-2 border border-[#d4ff32]/50 bg-[#d4ff32] hover:bg-[#e2f952] text-black text-xs sm:text-sm font-mono font-bold uppercase tracking-wider rounded-xs transition-all shadow-[0_0_15px_rgba(212,255,50,0.2)] cursor-pointer"
			>
				<UserPlus size={14} />
				<span>Add Administrator</span>
			</button>
		</div>
	</div>

	<!-- Error Alert -->
	{#if error}
		<div class="p-3.5 border border-[#ef4444]/40 bg-[#ef4444]/10 text-[#fca5a5] text-xs font-mono rounded-xs">
			<div class="font-bold text-[#ef4444] uppercase mb-0.5">[Governance Error]</div>
			<div>{error}</div>
		</div>
	{/if}

	<!-- Superadmin Card -->
	<div class="border border-[#252b3b] bg-[#12151d] p-5 rounded-xs flex flex-col sm:flex-row sm:items-center justify-between gap-4">
		<div class="flex items-center gap-3">
			<div class="w-10 h-10 rounded-xs bg-[#d4ff32]/10 border border-[#d4ff32]/30 flex items-center justify-center text-[#d4ff32]">
				<ShieldCheck size={20} />
			</div>
			<div>
				<div class="flex items-center gap-2 mb-0.5">
					<span class="font-display font-bold text-base text-[#f1f5f9]">{superadminEmail || 'Loading...'}</span>
					<Badge variant="lime">SUPERADMIN</Badge>
				</div>
			</div>
		</div>
		<div class="text-xs font-mono text-[#64748b] bg-[#0a0b0e] border border-[#252b3b] px-3 py-1.5 rounded-xs">
			Primary Identity Provider Lock
		</div>
	</div>

	<!-- Secondary Administrators Table Card -->
	<div class="border border-[#252b3b] bg-[#12151d] rounded-xs overflow-hidden flex flex-col font-mono text-xs sm:text-sm">
		<div class="px-4 py-3 border-b border-[#252b3b] bg-[#151922] flex items-center justify-between">
			<div class="flex items-center gap-2 font-display font-semibold text-sm text-[#f1f5f9]">
				<Shield size={16} class="text-[#06b6d4]" />
				<span>Whitelisted Operators ({admins.length})</span>
			</div>
			<span class="text-xs text-[#64748b]">Google SSO Whitelist</span>
		</div>

		<div class="overflow-x-auto">
			<table class="w-full text-left border-collapse">
				<thead>
					<tr class="bg-[#151922]/50 border-b border-[#252b3b] text-xs uppercase text-[#94a3b8]">
						<th class="py-2.5 px-4 font-medium">Administrator Email</th>
						<th class="py-2.5 px-4 font-medium">Role Tier</th>
						<th class="py-2.5 px-4 font-medium">Authorized At</th>
						<th class="py-2.5 px-4 font-medium">Authorized By</th>
						<th class="py-2.5 px-4 text-right font-medium">Action</th>
					</tr>
				</thead>

				<tbody class="divide-y divide-[#252b3b]/60">
					{#if isLoading && admins.length === 0}
						<tr>
							<td colspan="5" class="py-10 text-center text-[#64748b]">
								<RefreshCw size={18} class="animate-spin text-[#d4ff32] mx-auto mb-2" />
								Loading administrator directory...
							</td>
						</tr>
					{:else if admins.length === 0}
						<tr>
							<td colspan="5" class="py-10 text-center text-[#64748b]">
								No secondary administrators authorized. Use "Add Administrator" to grant operator access.
							</td>
						</tr>
					{:else}
						{#each admins as admin (admin.id)}
							<tr class="hover:bg-[#181c26]/60 transition-colors">
								<td class="py-3 px-4 font-medium text-[#f1f5f9]">
									{admin.email}
								</td>

								<td class="py-3 px-4 whitespace-nowrap">
									<select
										value={admin.role}
										onchange={(e) => handleRoleChange(admin.id, e.currentTarget.value as any)}
										class="bg-[#0a0b0e] border border-[#252b3b] rounded-xs px-2 py-1 text-xs font-mono text-[#f1f5f9] focus:outline-none focus:border-[#d4ff32] cursor-pointer"
									>
										<option value="read-only">read-only</option>
										<option value="read-write">read-write</option>
									</select>
								</td>

								<td class="py-3 px-4 text-[#94a3b8] whitespace-nowrap">
									{formatDate(admin.created_at)}
								</td>

								<td class="py-3 px-4 text-[#64748b] whitespace-nowrap">
									{admin.created_by || 'superadmin'}
								</td>

								<td class="py-3 px-4 text-right whitespace-nowrap">
									<button
										onclick={() => handleDeleteAdmin(admin.id, admin.email)}
										class="p-1.5 border border-[#252b3b] hover:border-[#ef4444]/60 bg-[#0a0b0e] text-[#94a3b8] hover:text-[#ef4444] rounded-xs transition-colors cursor-pointer"
										title="Revoke Admin Access"
									>
										<Trash2 size={14} />
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

<!-- Add Administrator Modal -->
<Modal
	open={addModalOpen}
	title="AUTHORIZE ADMINISTRATOR"
	description="Authorize a Google account email for console operator access."
	onClose={() => { addModalOpen = false; }}
>
	{#if formError}
		<div class="mb-4 p-3 border border-[#ef4444]/40 bg-[#ef4444]/10 text-[#fca5a5] text-xs font-mono rounded-xs">
			{formError}
		</div>
	{/if}

	<div class="space-y-4 font-mono text-xs sm:text-sm">
		<div>
			<label for="admin-email" class="block text-xs uppercase tracking-wider text-[#94a3b8] mb-1.5">
				Google Account Email:
			</label>
			<input
				id="admin-email"
				type="email"
				bind:value={newEmail}
				placeholder="operator@example.com"
				class="w-full px-3 py-2 bg-[#0a0b0e] border border-[#252b3b] focus:border-[#d4ff32] text-[#f1f5f9] text-xs sm:text-sm rounded-xs focus:outline-none"
			/>
			<p class="text-[11px] text-[#64748b] mt-1">
				Must match the email address associated with their Google account.
			</p>
		</div>

		<div>
			<span class="block text-xs uppercase tracking-wider text-[#94a3b8] mb-2">
				Permission Role Tier:
			</span>
			<div class="grid grid-cols-2 gap-3">
				<label class="border border-[#252b3b] p-3 rounded-xs flex flex-col gap-1 cursor-pointer {newRole === 'read-only' ? 'bg-[#d4ff32]/10 border-[#d4ff32]/40 text-[#f1f5f9]' : 'bg-[#0a0b0e] text-[#94a3b8]'}">
					<div class="flex items-center gap-2">
						<input type="radio" bind:group={newRole} value="read-only" class="accent-[#d4ff32]" />
						<span class="font-bold text-xs uppercase">Read-Only</span>
					</div>
					<span class="text-[10px] text-[#64748b]">View telemetry and table data without mutation access</span>
				</label>

				<label class="border border-[#252b3b] p-3 rounded-xs flex flex-col gap-1 cursor-pointer {newRole === 'read-write' ? 'bg-[#d4ff32]/10 border-[#d4ff32]/40 text-[#f1f5f9]' : 'bg-[#0a0b0e] text-[#94a3b8]'}">
					<div class="flex items-center gap-2">
						<input type="radio" bind:group={newRole} value="read-write" class="accent-[#d4ff32]" />
						<span class="font-bold text-xs uppercase">Read-Write</span>
					</div>
					<span class="text-[10px] text-[#64748b]">Modify table rows and execute custom SQL queries</span>
				</label>
			</div>
		</div>
	</div>

	{#snippet footer()}
		<div class="flex items-center gap-2">
			<button
				onclick={() => { addModalOpen = false; }}
				class="px-3.5 py-1.5 border border-[#252b3b] bg-[#181c26] hover:bg-[#252b3b] text-[#94a3b8] text-xs font-mono uppercase tracking-wider rounded-xs transition-colors cursor-pointer"
			>
				Cancel
			</button>
			<button
				onclick={handleAddAdmin}
				disabled={isAdding || !newEmail.trim()}
				class="flex items-center gap-1.5 px-4 py-1.5 border border-[#d4ff32]/50 bg-[#d4ff32] hover:bg-[#e2f952] text-black text-xs font-mono font-bold uppercase tracking-wider rounded-xs transition-all shadow-[0_0_15px_rgba(212,255,50,0.2)] disabled:opacity-40 cursor-pointer"
			>
				<UserPlus size={13} />
				<span>{isAdding ? 'Authorizing...' : 'Authorize Admin'}</span>
			</button>
		</div>
	{/snippet}
</Modal>
