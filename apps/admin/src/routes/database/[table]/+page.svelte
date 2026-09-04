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
	<div class="flex flex-col sm:flex-row sm:items-center justify-between gap-4 border-b border-[#252b3b] pb-4">
		<div class="flex items-center gap-3">
			<a
				href="/database"
				class="p-2 border border-[#252b3b] hover:border-[#64748b] bg-[#181c26] hover:bg-[#252b3b] text-[#94a3b8] hover:text-[#f1f5f9] rounded-xs transition-colors"
				title="Back to tables list"
			>
				<ArrowLeft size={16} />
			</a>
			<div>
				<div class="text-[11px] font-mono uppercase tracking-widest text-[#d4ff32]">Table Inspector</div>
				<h1 class="font-display font-extrabold text-2xl text-[#f1f5f9] tracking-tight flex items-center gap-2">
					<Database size={20} class="text-[#d4ff32]" />
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
				class="bg-[#12151d] border border-[#252b3b] text-[#f1f5f9] px-3 py-1.5 rounded-xs focus:border-[#d4ff32] focus:outline-none"
			>
				<option value={25}>25 rows</option>
				<option value={50}>50 rows</option>
				<option value={100}>100 rows</option>
				<option value={200}>200 rows</option>
			</select>

			<button
				onclick={loadTable}
				disabled={isLoading}
				class="p-2 border border-[#252b3b] hover:border-[#64748b] bg-[#181c26] hover:bg-[#252b3b] text-[#94a3b8] hover:text-[#f1f5f9] rounded-xs transition-colors cursor-pointer"
				title="Reload table data"
			>
				<RefreshCw size={14} class={isLoading ? 'animate-spin text-[#d4ff32]' : ''} />
			</button>
		</div>
	</div>

	<!-- Error Alert -->
	{#if error}
		<div class="p-3.5 border border-[#ef4444]/40 bg-[#ef4444]/10 text-[#fca5a5] text-xs font-mono rounded-xs">
			<div class="font-bold text-[#ef4444] uppercase mb-0.5">[Table Query Error]</div>
			<div>{error}</div>
		</div>
	{/if}

	<!-- Data Viewport -->
	<div class="border border-[#252b3b] bg-[#12151d] rounded-xs overflow-hidden">
		{#if isLoading && !data}
			<div class="p-12 text-center text-[#94a3b8] font-mono text-sm">
				<RefreshCw size={20} class="animate-spin text-[#d4ff32] mx-auto mb-2" />
				Loading live rows from SQLite persistent store...
			</div>
		{:else if data}
			<div class="overflow-x-auto">
				<table class="w-full text-left text-xs sm:text-sm font-mono border-collapse">
					<thead class="bg-[#151922] border-b border-[#252b3b] text-xs uppercase text-[#94a3b8] tracking-wider">
						<tr>
							<th class="py-2.5 px-3 font-medium w-16 text-center">Edit</th>
							{#each data.columns as col (col.name)}
								<th
									class="py-2.5 px-4 font-medium cursor-pointer hover:text-[#d4ff32] transition-colors whitespace-nowrap"
									onclick={() => handleSort(col.name)}
								>
									<div class="flex items-center gap-1.5">
										{#if col.primary_key}
											<Key size={13} class="text-amber-400" />
										{/if}
										<span>{col.name}</span>
										{#if sortBy === col.name}
											<span class="text-[#d4ff32] font-bold">{sortOrder === 'asc' ? '▲' : '▼'}</span>
										{/if}
										<span class="text-[10px] text-[#64748b] font-normal">({col.type})</span>
									</div>
								</th>
							{/each}
						</tr>
					</thead>
					<tbody class="divide-y divide-[#252b3b]/60">
						{#if data.rows.length === 0}
							<tr>
								<td colspan={data.columns.length + 1} class="py-10 text-center text-[#64748b] font-mono">
									Table contains no rows.
								</td>
							</tr>
						{:else}
							{#each data.rows as row, idx (idx)}
								<tr class="hover:bg-[#181c26]/60 transition-colors">
									<td class="py-2.5 px-3 text-center whitespace-nowrap">
										{#if user?.role !== 'read-only'}
											<button
												onclick={() => startEdit(row)}
												class="p-1 border border-[#252b3b] hover:border-[#d4ff32] bg-[#0a0b0e] text-[#94a3b8] hover:text-[#d4ff32] rounded-xs transition-colors cursor-pointer"
												title="Edit Row"
											>
												<Edit2 size={13} />
											</button>
										{:else}
											<span class="text-[10px] text-[#64748b]" title="Read-only role">LOCK</span>
										{/if}
									</td>
									{#each data.columns as col (col.name)}
										<td class="py-2.5 px-4 max-w-xs truncate text-[#e6edf3] tabular-nums">
											{#if row[col.name] === null || row[col.name] === undefined}
												<span class="text-[#64748b] italic">NULL</span>
											{:else if typeof row[col.name] === 'boolean'}
												<span class="{row[col.name] ? 'text-[#10b981]' : 'text-[#64748b]'} font-bold">
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
			<div class="border-t border-[#252b3b] bg-[#0e1117] px-4 py-3 flex items-center justify-between text-xs font-mono text-[#64748b]">
				<div>
					Showing rows <span class="text-[#f1f5f9] font-medium">{offset + 1}</span>–<span class="text-[#f1f5f9] font-medium">
						{Math.min(offset + limit, data.total)}
					</span> of <span class="text-[#f1f5f9] font-medium">{data.total}</span>
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
						class="p-1.5 border border-[#252b3b] hover:border-[#64748b] disabled:opacity-30 disabled:cursor-not-allowed text-[#94a3b8] hover:text-white rounded-xs transition-colors cursor-pointer"
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
						class="p-1.5 border border-[#252b3b] hover:border-[#64748b] disabled:opacity-30 disabled:cursor-not-allowed text-[#94a3b8] hover:text-white rounded-xs transition-colors cursor-pointer"
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
			<div class="mb-4 p-3 border border-[#ef4444]/40 bg-[#ef4444]/10 text-[#fca5a5] text-xs font-mono rounded-xs">
				{saveError}
			</div>
		{/if}

		<div class="space-y-3 font-mono text-xs sm:text-sm">
			{#each data.columns as col (col.name)}
				<div class="space-y-1">
					<div class="flex items-center justify-between text-[#94a3b8] text-xs">
						<span class="flex items-center gap-1 font-medium">
							{#if col.primary_key}
								<Key size={12} class="text-amber-400" />
								<span class="text-amber-300">{col.name} (Primary Key)</span>
							{:else}
								<span class="text-[#f1f5f9]">{col.name}</span>
							{/if}
						</span>
						<span class="text-[#64748b] text-[10px] uppercase">({col.type})</span>
					</div>

					{#if col.primary_key}
						<input
							type="text"
							value={editingRow[col.name]}
							disabled
							class="w-full px-3 py-1.5 bg-[#0a0b0e] border border-[#252b3b] text-[#64748b] cursor-not-allowed text-xs rounded-xs"
						/>
					{:else}
						<input
							type="text"
							bind:value={editUpdates[col.name]}
							class="w-full px-3 py-1.5 bg-[#0a0b0e] border border-[#252b3b] focus:border-[#d4ff32] text-[#f1f5f9] text-xs rounded-xs focus:outline-none"
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
					class="px-3.5 py-1.5 border border-[#252b3b] bg-[#181c26] hover:bg-[#252b3b] text-[#94a3b8] text-xs font-mono uppercase tracking-wider rounded-xs transition-colors cursor-pointer"
				>
					Cancel
				</button>
				<button
					onclick={saveEdit}
					disabled={isSaving}
					class="flex items-center gap-1.5 px-4 py-1.5 border border-[#d4ff32]/50 bg-[#d4ff32] hover:bg-[#e2f952] text-black text-xs font-mono font-bold uppercase tracking-wider rounded-xs transition-all shadow-[0_0_15px_rgba(212,255,50,0.2)] cursor-pointer"
				>
					<Save size={13} />
					<span>{isSaving ? 'Saving...' : 'Save Updates'}</span>
				</button>
			</div>
		{/snippet}
	</Modal>
{/if}
