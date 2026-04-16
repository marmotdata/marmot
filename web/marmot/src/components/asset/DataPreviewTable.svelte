<script lang="ts">
	interface Props {
		columnNames: string[];
		rows: any[][];
		loading?: boolean;
		error?: string | null;
	}

	let { columnNames = [], rows = [], loading = false, error = null }: Props = $props();

	// Column width state
	let columnWidths = $state<number[]>([]);
	let resizingColumn = $state<number | null>(null);
	let startX = $state(0);
	let startWidth = $state(0);

	// Initialize column widths when columnNames change
	$effect(() => {
		if (columnNames.length > 0 && columnWidths.length !== columnNames.length) {
			columnWidths = columnNames.map(() => 200); // Default width
		}
	});

	function handleResizeStart(index: number, e: MouseEvent) {
		resizingColumn = index;
		startX = e.clientX;
		startWidth = columnWidths[index];
		e.preventDefault();
	}

	function handleResize(e: MouseEvent) {
		if (resizingColumn !== null) {
			const diff = e.clientX - startX;
			const newWidth = Math.max(100, startWidth + diff); // Min width 100px
			columnWidths[resizingColumn] = newWidth;
		}
	}

	function handleResizeEnd() {
		resizingColumn = null;
	}

	$effect(() => {
		if (resizingColumn !== null) {
			document.addEventListener('mousemove', handleResize);
			document.addEventListener('mouseup', handleResizeEnd);
			return () => {
				document.removeEventListener('mousemove', handleResize);
				document.removeEventListener('mouseup', handleResizeEnd);
			};
		}
	});
</script>

<div class="w-full">
	{#if loading}
		<div class="flex items-center justify-center py-12">
			<div class="animate-spin rounded-full h-8 w-8 border-b-2 border-earthy-terracotta-700"></div>
		</div>
	{:else if error}
		<div class="flex items-center justify-center py-12">
			<div
				class="bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800/50 rounded-lg p-4 text-red-600 dark:text-red-400 max-w-2xl"
			>
				<div class="font-semibold mb-1">Preview Not Available</div>
				<div class="text-sm">{error}</div>
			</div>
		</div>
	{:else if columnNames.length === 0 || rows.length === 0}
		<div class="flex items-center justify-center py-12">
			<div class="text-gray-500 dark:text-gray-400">No data available for preview</div>
		</div>
	{:else}
		<div
			class="overflow-x-auto max-h-[600px] -mx-8 px-8 {resizingColumn !== null
				? 'select-none'
				: ''}"
		>
			<table class="w-full divide-y divide-gray-200 dark:divide-gray-700">
				<thead class="bg-gray-100 dark:bg-gray-700 sticky top-0 z-10">
					<tr>
						{#each columnNames as columnName, index}
							<th
								class="px-4 py-2 text-left text-xs font-medium text-gray-700 dark:text-gray-300 uppercase tracking-wider whitespace-nowrap relative border-r border-gray-200 dark:border-gray-700"
								style="width: {columnWidths[index]}px; min-width: {columnWidths[
									index
								]}px; max-width: {columnWidths[index]}px;"
							>
								<div class="flex items-center justify-between">
									<span class="truncate">{columnName}</span>
								</div>
								<div
									class="absolute top-0 right-0 w-1 h-full cursor-col-resize hover:bg-earthy-terracotta-500 group"
									onmousedown={(e) => handleResizeStart(index, e)}
								>
									<div class="w-1 h-full bg-transparent group-hover:bg-earthy-terracotta-500"></div>
								</div>
							</th>
						{/each}
					</tr>
				</thead>
				<tbody class="bg-white dark:bg-gray-900 divide-y divide-gray-200 dark:divide-gray-700">
					{#each rows as row, rowIndex}
						<tr
							class="{rowIndex % 2 === 0
								? 'bg-white dark:bg-gray-900'
								: 'bg-gray-50 dark:bg-gray-800'} hover:bg-gray-50 dark:hover:bg-gray-700"
						>
							{#each row as cell, cellIndex}
								<td
									class="px-4 py-2 text-sm text-gray-900 dark:text-gray-100 whitespace-nowrap overflow-hidden text-ellipsis border-r border-gray-200 dark:border-gray-700"
									style="width: {columnWidths[cellIndex]}px; min-width: {columnWidths[
										cellIndex
									]}px; max-width: {columnWidths[cellIndex]}px;"
									title={cell != null ? String(cell) : ''}
								>
									{#if cell == null}
										<span class="text-gray-400 italic">null</span>
									{:else if typeof cell === 'number'}
										<span class="font-mono">{String(cell)}</span>
									{:else if typeof cell === 'object'}
										{JSON.stringify(cell)}
									{:else}
										{String(cell)}
									{/if}
								</td>
							{/each}
						</tr>
					{/each}
				</tbody>
			</table>
		</div>

		<div class="mt-4 text-sm text-gray-500 dark:text-gray-400 text-center">
			Showing {rows.length} row{rows.length !== 1 ? 's' : ''}
		</div>
	{/if}
</div>

<style>
	:global(.overflow-x-auto::-webkit-scrollbar) {
		height: 10px;
	}

	:global(.overflow-x-auto::-webkit-scrollbar-track) {
		background: transparent;
	}

	:global(.overflow-x-auto::-webkit-scrollbar-thumb) {
		background: #d1d5db;
		border-radius: 3px;
	}

	:global(.overflow-x-auto::-webkit-scrollbar-thumb:hover) {
		background: #9ca3af;
	}

	:global(.overflow-x-auto) {
		scrollbar-width: thin;
		scrollbar-color: #d1d5db transparent;
	}
</style>
