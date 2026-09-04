<script lang="ts">
	import { page } from '$app/state';
	import { onMount } from 'svelte';
	import { ArrowLeft, RefreshCw, Edit2, ChevronLeft, ChevronRight, Save, X, Key, AlertCircle } from 'lucide-svelte';

	interface ColumnInfo {
		name: string;
		type: string;
		primary_key: boolean;
		not_null: boolean;
	}

	interface TableData {
		columns: ColumnInfo[];
		rows: Record<string, any>[];
		total: number;
		limit: number;
		offset: number;
	}

	const tableName = $derived(page.params.table);
	const user = $derived(page.data.user);

	let data = $state<TableData | null>(null);
	let isLoading = $state<boolean>(true);
	let error = $state<string | null>(null);
	let limit = $state<number>(50);
	let offset = $state<number>(0);
	let sortBy = $state<string>('');
	let sortOrder = $state<'asc' | 'desc'>('asc');

	// Row Editing State
	let editingRow = $state<Record<string, any> | null>(null);
	let editUpdates = $state<Record<string, any>>({});
	let isSaving = $state<boolean>(false);
	let saveError = $state<string | null>(null);

	async function loadTable() {
		isLoading = true;
		error = null;
		try {
			const params = new URLSearchParams({
				limit: String(limit),
				offset: String(offset)
			});
			if (sortBy) {
				params.set('sort_by', sortBy);
				params.set('sort_order', sortOrder);
			}

			const res = await fetch(`/api/main-db/tables/${tableName}?${params.toString()}`);
			if (res.ok) {
				data = await res.json();
			} else {
				const errData = await res.json().catch(() => null);
				error = errData?.message || `HTTP ${res.status}`;
			}
		} catch (err: any) {
			error = err?.message || 'Failed to query table rows';
		} finally {
			isLoading = false;
		}
	}

	function handleSort(columnName: string) {
		if (sortBy === columnName) {
			sortOrder = sortOrder === 'asc' ? 'desc' : 'asc';
		} else {
			sortBy = columnName;
			sortOrder = 'asc';
		}
		offset = 0;
		loadTable();
	}

	function startEdit(row: Record<string, any>) {
		if (user?.role === 'read-only') return;
		editingRow = row;
		editUpdates = { ...row };
		saveError = null;
	}

	function cancelEdit() {
		editingRow = null;
		editUpdates = {};
		saveError = null;
	}

	async function saveEdit() {
		if (!editingRow || !data) return;
		isSaving = true;
		saveError = null;

		const pkCol = data.columns.find((c) => c.primary_key)?.name || 'id';
		const pkVal = editingRow[pkCol];

		// Only include changed fields
		const updates: Record<string, any> = {};
		for (const col of data.columns) {
			if (col.name !== pkCol && editUpdates[col.name] !== editingRow[col.name]) {
				updates[col.name] = editUpdates[col.name];
			}
		}

		if (Object.keys(updates).length === 0) {
			cancelEdit();
			isSaving = false;
			return;
		}

		try {
			const res = await fetch(`/api/main-db/tables/${tableName}/row`, {
				method: 'PUT',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({
					primary_key_column: pkCol,
					primary_key_value: pkVal,
					updates
				})
			});

			if (res.ok) {
				cancelEdit();
				loadTable();
			} else {
				const err = await res.json().catch(() => null);
				saveError = err?.message || `Failed to update row (HTTP ${res.status})`;
			}
		} catch (err: any) {
			saveError = err?.message || 'Network error updating row';
		} finally {
			isSaving = false;
		}
	}

	onMount(() => {
		loadTable();
	});
</script>

<svelte:head>
	<title>KPI Console | Table: {tableName}</title>
</svelte:head>

<div class="space-y-5">
	<!-- Navigation & Actions Bar -->
	<div class="flex flex-col sm:flex-row sm:items-center justify-between gap-4 border-b border-[#1e293b] pb-4">
		<div class="flex items-center gap-3">
			<a
				href="/database"
				class="p-2 border border-[#1e293b] hover:border-slate-600 bg-[#0f141d] hover:bg-[#141c28] text-slate-400 hover:text-white transition-colors"
				title="Back to tables list"
			>
				<ArrowLeft class="w-4 h-4" />
			</a>
			<div>
				<div class="text-xs font-mono uppercase tracking-widest text-cyan-400">[Table Inspector]</div>
				<h1 class="text-2xl font-bold font-mono text-white tracking-tight">{tableName}</h1>
			</div>
		</div>

		<div class="flex items-center gap-2">
			<select
				bind:value={limit}
				onchange={() => {
					offset = 0;
					loadTable();
				}}
				class="bg-[#0f141d] border border-[#1e293b] text-sm font-mono text-slate-300 px-3 py-1.5 focus:border-cyan-500 focus:outline-none"
			>
				<option value={25}>25 rows</option>
				<option value={50}>50 rows</option>
				<option value={100}>100 rows</option>
				<option value={200}>200 rows</option>
			</select>

			<button
				onclick={loadTable}
				disabled={isLoading}
				class="p-2 border border-[#1e293b] hover:border-slate-600 bg-[#0f141d] hover:bg-[#141c28] text-slate-400 hover:text-white transition-colors"
				title="Reload table data"
			>
				<RefreshCw class="w-4 h-4 {isLoading ? 'animate-spin text-cyan-400' : ''}" />
			</button>
		</div>
	</div>

	<!-- Error Alert -->
	{#if error}
		<div class="p-4 border border-red-500/40 bg-red-950/20 text-red-200 text-sm font-mono">
			<div class="font-bold text-red-400 uppercase">[Table Query Error]</div>
			<div class="mt-1">{error}</div>
		</div>
	{/if}

	<!-- Data Viewport -->
	<div class="border border-[#1e293b] bg-[#0f141d] overflow-hidden">
		{#if isLoading && !data}
			<div class="p-12 text-center text-slate-400 font-mono text-sm">
				<RefreshCw class="w-5 h-5 animate-spin mx-auto mb-2 text-cyan-400" />
				Loading live rows from SQLite...
			</div>
		{:else if data}
			<div class="overflow-x-auto">
				<table class="w-full text-left text-sm font-mono">
					<thead class="bg-[#0a0e14] border-b border-[#1e293b] text-xs uppercase text-slate-400 tracking-wider">
						<tr>
							<th class="py-2.5 px-3 font-medium w-16 text-center">Edit</th>
							{#each data.columns as col (col.name)}
								<th
									class="py-2.5 px-4 font-medium cursor-pointer hover:text-cyan-300 transition-colors whitespace-nowrap"
									onclick={() => handleSort(col.name)}
								>
									<div class="flex items-center gap-1.5">
										{#if col.primary_key}
											<Key class="w-3.5 h-3.5 text-amber-400" />
										{/if}
										<span>{col.name}</span>
										{#if sortBy === col.name}
											<span class="text-cyan-400 font-bold">{sortOrder === 'asc' ? '▲' : '▼'}</span>
										{/if}
										<span class="text-xs text-slate-600 font-normal">({col.type})</span>
									</div>
								</th>
							{/each}
						</tr>
					</thead>
					<tbody class="divide-y divide-[#1e293b]">
						{#if data.rows.length === 0}
							<tr>
								<td colspan={data.columns.length + 1} class="py-8 text-center text-slate-500 font-mono">
									Table contains no rows.
								</td>
							</tr>
						{:else}
							{#each data.rows as row, idx (idx)}
								<tr class="hover:bg-[#141c28] transition-colors">
									<td class="py-2.5 px-3 text-center whitespace-nowrap">
										{#if user?.role !== 'read-only'}
											<button
												onclick={() => startEdit(row)}
												class="p-1 border border-[#1e293b] hover:border-cyan-500 bg-[#121822] text-slate-400 hover:text-cyan-300 transition-colors"
												title="Edit Row"
											>
												<Edit2 class="w-4 h-4" />
											</button>
										{:else}
											<span class="text-xs text-slate-600" title="Read-only role">LOCK</span>
										{/if}
									</td>
									{#each data.columns as col (col.name)}
										<td class="py-2.5 px-4 max-w-xs truncate text-slate-300 tabular-nums">
											{#if row[col.name] === null || row[col.name] === undefined}
												<span class="text-slate-600 italic">NULL</span>
											{:else if typeof row[col.name] === 'boolean'}
												<span class="{row[col.name] ? 'text-emerald-400' : 'text-slate-500'} font-bold">
													{row[col.name] ? 'TRUE' : 'FALSE'}
												</span>
											{:else}
												{row[col.name]}
											{/if}
										</td>
									{/each}
								</tr>
							{/each}
						{/if}
					</tbody>
				</table>
			</div>

			<!-- Pagination Footer -->
			<div class="border-t border-[#1e293b] bg-[#0a0e14] px-4 py-3 flex items-center justify-between text-sm font-mono text-slate-400">
				<div>
					Showing rows <span class="text-white font-medium">{offset + 1}</span>–<span class="text-white font-medium">
						{Math.min(offset + limit, data.total)}
					</span> of <span class="text-white font-medium">{data.total}</span>
				</div>

				<div class="flex items-center gap-2">
					<button
						onclick={() => {
							if (offset >= limit) {
								offset -= limit;
								loadTable();
							}
						}}
						disabled={offset === 0}
						class="p-1.5 border border-[#1e293b] hover:border-slate-600 disabled:opacity-40 disabled:cursor-not-allowed text-slate-300"
					>
						<ChevronLeft class="w-4 h-4" />
					</button>

					<span>Offset {offset}</span>

					<button
						onclick={() => {
							if (offset + limit < data!.total) {
								offset += limit;
								loadTable();
							}
						}}
						disabled={offset + limit >= data.total}
						class="p-1.5 border border-[#1e293b] hover:border-slate-600 disabled:opacity-40 disabled:cursor-not-allowed text-slate-300"
					>
						<ChevronRight class="w-4 h-4" />
					</button>
				</div>
			</div>
		{/if}
	</div>
</div>

<!-- Row Editor Modal -->
{#if editingRow && data}
	<div class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/80 backdrop-blur-xs">
		<div class="w-full max-w-xl border border-[#1e293b] bg-[#0f141d] p-6 shadow-2xl space-y-5 max-h-[90vh] flex flex-col">
			<!-- Header -->
			<div class="flex items-center justify-between border-b border-[#1e293b] pb-3">
				<div>
					<div class="text-xs font-mono uppercase tracking-widest text-cyan-400">[Row Editor]</div>
					<h2 class="text-lg font-bold font-mono text-white uppercase">Table: {tableName}</h2>
				</div>
				<button onclick={cancelEdit} class="text-slate-500 hover:text-white">
					<X class="w-4 h-4" />
				</button>
			</div>

			<!-- Error in editor -->
			{#if saveError}
				<div class="p-3 border border-red-500/40 bg-red-950/20 text-red-300 text-sm font-mono">
					{saveError}
				</div>
			{/if}

			<!-- Fields Form -->
			<div class="space-y-3 overflow-y-auto flex-1 pr-1 font-mono text-sm">
				{#each data.columns as col (col.name)}
					<div class="space-y-1">
						<div class="flex items-center justify-between text-slate-400 text-xs">
							<span class="flex items-center gap-1 font-medium">
								{#if col.primary_key}
									<Key class="w-3.5 h-3.5 text-amber-400" />
									<span class="text-amber-300">{col.name} (Primary Key)</span>
								{:else}
									<span class="text-slate-200">{col.name}</span>
								{/if}
							</span>
							<span class="text-slate-600 text-xs">{col.type}</span>
						</div>

						{#if col.primary_key}
							<input
								type="text"
								value={editingRow[col.name]}
								disabled
								class="w-full px-3 py-2 bg-[#080b10] border border-slate-800 text-slate-500 cursor-not-allowed text-sm"
							/>
						{:else}
							<input
								type="text"
								bind:value={editUpdates[col.name]}
								class="w-full px-3 py-2 bg-[#0a0e14] border border-[#1e293b] focus:border-cyan-500 text-white text-sm focus:outline-none"
							/>
						{/if}
					</div>
				{/each}
			</div>

			<!-- Actions -->
			<div class="flex items-center justify-end gap-3 pt-3 border-t border-[#1e293b]">
				<button
					onclick={cancelEdit}
					disabled={isSaving}
					class="px-4 py-2 border border-[#1e293b] bg-[#141c28] hover:bg-[#1a2536] text-slate-300 text-sm font-mono uppercase"
				>
					Cancel
				</button>
				<button
					onclick={saveEdit}
					disabled={isSaving}
					class="flex items-center gap-2 px-4 py-2 border border-cyan-500 bg-cyan-600 hover:bg-cyan-500 text-black text-sm font-mono font-bold uppercase transition-colors"
				>
					<Save class="w-4 h-4" />
					<span>{isSaving ? 'Saving...' : 'Save Updates'}</span>
				</button>
			</div>
		</div>
	</div>
{/if}
