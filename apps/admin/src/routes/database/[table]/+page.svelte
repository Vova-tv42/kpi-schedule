<script lang="ts">
	import { page } from '$app/state';
	import { onMount } from 'svelte';
	import Modal from '$lib/components/Modal.svelte';
	import { 
		ArrowLeft, 
		RefreshCw, 
		Edit2, 
		ChevronLeft, 
		ChevronRight, 
		Save, 
		Key, 
		Database, 
		AlertCircle 
	} from 'lucide-svelte';

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
	<title>KPI Schedule // Table: {tableName}</title>
</svelte:head>

<div class="space-y-5">
	<!-- Navigation & Actions Bar -->
	<div class="flex flex-col sm:flex-row sm:items-center justify-between gap-4 border-b border-slate-300 dark:border-[#252b3b] pb-4">
		<div class="flex items-center gap-3">
			<a
				href="/database"
				class="p-2 border border-slate-300 hover:border-slate-400 bg-white hover:bg-slate-100 text-slate-600 hover:text-slate-950 dark:border-[#252b3b] dark:hover:border-[#64748b] dark:bg-[#181c26] dark:hover:bg-[#252b3b] dark:text-[#94a3b8] dark:hover:text-[#f1f5f9] rounded-xs transition-colors shadow-xs dark:shadow-none"
				title="Back to tables list"
			>
				<ArrowLeft size={16} />
			</a>
			<div>
				<div class="text-[11px] font-mono uppercase tracking-widest text-emerald-700 dark:text-[#d4ff32] font-semibold">Table Inspector</div>
				<h1 class="font-display font-extrabold text-2xl text-slate-950 dark:text-[#f1f5f9] tracking-tight flex items-center gap-2">
					<Database size={20} class="text-emerald-600 dark:text-[#d4ff32]" />
					<span>{tableName}</span>
				</h1>
			</div>
		</div>

		<div class="flex items-center gap-2 font-mono text-xs sm:text-sm">
			<select
				bind:value={limit}
				onchange={() => {
					offset = 0;
					loadTable();
				}}
				class="bg-white dark:bg-[#12151d] border border-slate-300 dark:border-[#252b3b] text-slate-900 dark:text-[#f1f5f9] px-3 py-1.5 rounded-xs focus:border-slate-600 dark:focus:border-[#d4ff32] focus:outline-none shadow-xs dark:shadow-none"
			>
				<option value={25}>25 rows</option>
				<option value={50}>50 rows</option>
				<option value={100}>100 rows</option>
				<option value={200}>200 rows</option>
			</select>

			<button
				onclick={loadTable}
				disabled={isLoading}
				class="p-2 border border-slate-300 hover:border-slate-400 bg-white hover:bg-slate-100 text-slate-600 hover:text-slate-950 dark:border-[#252b3b] dark:hover:border-[#64748b] dark:bg-[#181c26] dark:hover:bg-[#252b3b] dark:text-[#94a3b8] dark:hover:text-[#f1f5f9] rounded-xs transition-colors cursor-pointer shadow-xs dark:shadow-none"
				title="Reload table data"
			>
				<RefreshCw size={14} class={isLoading ? 'animate-spin text-emerald-600 dark:text-[#d4ff32]' : ''} />
			</button>
		</div>
	</div>

	<!-- Error Alert -->
	{#if error}
		<div class="p-3.5 border border-red-300 bg-red-100 text-red-900 dark:border-[#ef4444]/40 dark:bg-[#ef4444]/10 dark:text-[#fca5a5] text-xs font-mono rounded-xs">
			<div class="font-bold uppercase mb-0.5">[Table Query Error]</div>
			<div>{error}</div>
		</div>
	{/if}

	<!-- Data Viewport -->
	<div class="border border-slate-300 dark:border-[#252b3b] bg-white dark:bg-[#12151d] rounded-xs overflow-hidden shadow-xs dark:shadow-none">
		{#if isLoading && !data}
			<div class="p-12 text-center text-slate-600 dark:text-[#94a3b8] font-mono text-sm">
				<RefreshCw size={20} class="animate-spin text-emerald-600 dark:text-[#d4ff32] mx-auto mb-2" />
				Loading live rows from SQLite persistent store...
			</div>
		{:else if data}
			<div class="overflow-x-auto">
				<table class="w-full text-left text-xs sm:text-sm font-mono border-collapse">
					<thead class="bg-slate-50 dark:bg-[#151922] border-b border-slate-300 dark:border-[#252b3b] text-xs uppercase text-slate-600 dark:text-[#94a3b8] tracking-wider">
						<tr>
							<th class="py-2.5 px-3 font-medium w-16 text-center">Edit</th>
							{#each data.columns as col (col.name)}
								<th
									class="py-2.5 px-4 font-medium cursor-pointer hover:text-emerald-800 dark:hover:text-[#d4ff32] transition-colors whitespace-nowrap"
									onclick={() => handleSort(col.name)}
								>
									<div class="flex items-center gap-1.5">
										{#if col.primary_key}
											<Key size={13} class="text-amber-500" />
										{/if}
										<span>{col.name}</span>
										{#if sortBy === col.name}
											<span class="text-emerald-700 dark:text-[#d4ff32] font-bold">{sortOrder === 'asc' ? '▲' : '▼'}</span>
										{/if}
										<span class="text-[10px] text-slate-400 dark:text-[#64748b] font-normal">({col.type})</span>
									</div>
								</th>
							{/each}
						</tr>
					</thead>
					<tbody class="divide-y divide-slate-200 dark:divide-[#252b3b]/60">
						{#if data.rows.length === 0}
							<tr>
								<td colspan={data.columns.length + 1} class="py-10 text-center text-slate-500 dark:text-[#64748b] font-mono">
									Table contains no rows.
								</td>
							</tr>
						{:else}
							{#each data.rows as row, idx (idx)}
								<tr class="hover:bg-slate-50 dark:hover:bg-[#181c26]/60 transition-colors">
									<td class="py-2.5 px-3 text-center whitespace-nowrap">
										{#if user?.role !== 'read-only'}
											<button
												onclick={() => startEdit(row)}
												class="p-1 border border-slate-300 hover:border-slate-500 bg-slate-100 hover:bg-slate-200 text-slate-600 hover:text-slate-950 dark:border-[#252b3b] dark:hover:border-[#d4ff32] dark:bg-[#0a0b0e] dark:text-[#94a3b8] dark:hover:text-[#d4ff32] rounded-xs transition-colors cursor-pointer"
												title="Edit Row"
											>
												<Edit2 size={13} />
											</button>
										{:else}
											<span class="text-[10px] text-slate-400 dark:text-[#64748b]" title="Read-only role">LOCK</span>
										{/if}
									</td>
									{#each data.columns as col (col.name)}
										<td class="py-2.5 px-4 max-w-xs truncate text-slate-900 dark:text-[#e6edf3] tabular-nums">
											{#if row[col.name] === null || row[col.name] === undefined}
												<span class="text-slate-400 dark:text-[#64748b] italic">NULL</span>
											{:else if typeof row[col.name] === 'boolean'}
												<span class="{row[col.name] ? 'text-emerald-700 dark:text-[#10b981]' : 'text-slate-400 dark:text-[#64748b]'} font-bold">
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
			<div class="border-t border-slate-300 dark:border-[#252b3b] bg-slate-50 dark:bg-[#0e1117] px-4 py-3 flex items-center justify-between text-xs font-mono text-slate-600 dark:text-[#64748b]">
				<div>
					Showing rows <span class="text-slate-950 dark:text-[#f1f5f9] font-medium">{offset + 1}</span>–<span class="text-slate-950 dark:text-[#f1f5f9] font-medium">
						{Math.min(offset + limit, data.total)}
					</span> of <span class="text-slate-950 dark:text-[#f1f5f9] font-medium">{data.total}</span>
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
						class="p-1.5 border border-slate-300 hover:border-slate-400 bg-white hover:bg-slate-100 disabled:opacity-30 disabled:cursor-not-allowed text-slate-700 hover:text-slate-950 dark:border-[#252b3b] dark:hover:border-[#64748b] dark:bg-transparent dark:text-[#94a3b8] dark:hover:text-white rounded-xs transition-colors cursor-pointer"
					>
						<ChevronLeft size={14} />
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
						class="p-1.5 border border-slate-300 hover:border-slate-400 bg-white hover:bg-slate-100 disabled:opacity-30 disabled:cursor-not-allowed text-slate-700 hover:text-slate-950 dark:border-[#252b3b] dark:hover:border-[#64748b] dark:bg-transparent dark:text-[#94a3b8] dark:hover:text-white rounded-xs transition-colors cursor-pointer"
					>
						<ChevronRight size={14} />
					</button>
				</div>
			</div>
		{/if}
	</div>
</div>

<!-- Row Editor Modal -->
{#if editingRow && data}
	<Modal
		open={true}
		title={`EDIT ROW: ${tableName}`}
		description="Modify SQLite row values. Primary key values are immutable."
		onClose={cancelEdit}
	>
		{#if saveError}
			<div class="mb-4 p-3 border border-red-300 bg-red-100 text-red-900 dark:border-[#ef4444]/40 dark:bg-[#ef4444]/10 dark:text-[#fca5a5] text-xs font-mono rounded-xs">
				{saveError}
			</div>
		{/if}

		<div class="space-y-3 font-mono text-xs sm:text-sm">
			{#each data.columns as col (col.name)}
				<div class="space-y-1">
					<div class="flex items-center justify-between text-slate-600 dark:text-[#94a3b8] text-xs">
						<span class="flex items-center gap-1 font-medium">
							{#if col.primary_key}
								<Key size={12} class="text-amber-500" />
								<span class="text-amber-800 dark:text-amber-300 font-semibold">{col.name} (Primary Key)</span>
							{:else}
								<span class="text-slate-900 dark:text-[#f1f5f9]">{col.name}</span>
							{/if}
						</span>
						<span class="text-slate-400 dark:text-[#64748b] text-[10px] uppercase">({col.type})</span>
					</div>

					{#if col.primary_key}
						<input
							type="text"
							value={editingRow[col.name]}
							disabled
							class="w-full px-3 py-1.5 bg-slate-100 dark:bg-[#0a0b0e] border border-slate-300 dark:border-[#252b3b] text-slate-500 dark:text-[#64748b] cursor-not-allowed text-xs rounded-xs"
						/>
					{:else}
						<input
							type="text"
							bind:value={editUpdates[col.name]}
							class="w-full px-3 py-1.5 bg-white dark:bg-[#0a0b0e] border border-slate-300 dark:border-[#252b3b] focus:border-slate-600 dark:focus:border-[#d4ff32] text-slate-900 dark:text-[#f1f5f9] text-xs rounded-xs focus:outline-none"
						/>
					{/if}
				</div>
			{/each}
		</div>

		{#snippet footer()}
			<div class="flex items-center gap-2">
				<button
					onclick={cancelEdit}
					disabled={isSaving}
					class="px-3.5 py-1.5 border border-slate-300 hover:border-slate-400 bg-white hover:bg-slate-100 text-slate-700 dark:border-[#252b3b] dark:bg-[#181c26] dark:hover:bg-[#252b3b] dark:text-[#94a3b8] text-xs font-mono uppercase tracking-wider rounded-xs transition-colors cursor-pointer"
				>
					Cancel
				</button>
				<button
					onclick={saveEdit}
					disabled={isSaving}
					class="flex items-center gap-1.5 px-4 py-1.5 border border-black dark:border-[#d4ff32]/50 bg-[#ccf600] hover:bg-[#bde800] dark:bg-[#d4ff32] dark:hover:bg-[#e2f952] text-black text-xs font-mono font-bold uppercase tracking-wider rounded-xs transition-all shadow-[2px_2px_0px_#090d16] dark:shadow-[0_0_15px_rgba(212,255,50,0.2)] cursor-pointer"
				>
					<Save size={13} />
					<span>{isSaving ? 'Saving...' : 'Save Updates'}</span>
				</button>
			</div>
		{/snippet}
	</Modal>
{/if}
