<script lang="ts">
	import IconifyIcon from '@iconify/svelte';
	import { m } from '$lib/paraglide/messages';
	import { formatNumber } from '$lib/utils/format';
	import type { QueryResult } from '$lib/dataquery/types';

	// Shows a completed query's rows, either as the sortable-by-eye table or as a CSV/JSON preview
	// that can be copied or downloaded; `resultView` is owned by the toolbar above this component.
	let {
		result,
		resultView,
		source
	}: {
		result: QueryResult;
		resultView: 'table' | 'csv' | 'json';
		source: string;
	} = $props();

	const PREVIEW_LINES = 300;

	let copied = $state(false);
	let exportText = $derived(resultView === 'table' ? '' : exportBody(resultView));
	let exportLines = $derived(exportText ? exportText.split('\n') : []);

	function cell(value: unknown): string {
		if (value === null || value === undefined) return '';
		if (typeof value === 'object') return JSON.stringify(value);
		return String(value);
	}

	function isNumeric(type: string): boolean {
		return /int|numeric|decimal|double|real|float|bigint/i.test(type);
	}

	function exportBody(kind: 'csv' | 'json'): string {
		const names = result.columns.map((c) => c.name);
		if (kind === 'json') {
			return JSON.stringify(
				result.rows.map((row) => Object.fromEntries(names.map((n, i) => [n, row[i] ?? null]))),
				null,
				2
			);
		}
		const quote = (v: unknown) => {
			const text = cell(v);
			return /[",\n]/.test(text) ? `"${text.replace(/"/g, '""')}"` : text;
		};
		return [names.map(quote).join(','), ...result.rows.map((r) => r.map(quote).join(','))].join(
			'\n'
		);
	}

	async function copyExport() {
		try {
			await navigator.clipboard.writeText(exportText);
			copied = true;
			setTimeout(() => (copied = false), 1500);
		} catch {
			// Clipboard access can be refused; Download still works
		}
	}

	function downloadExport() {
		const url = URL.createObjectURL(
			new Blob([exportText], { type: resultView === 'json' ? 'application/json' : 'text/csv' })
		);
		const link = document.createElement('a');
		link.href = url;
		link.download = `${source}-${result.query_id.slice(0, 8)}.${resultView}`;
		link.click();
		URL.revokeObjectURL(url);
	}
</script>

{#if resultView !== 'table' && result.columns.length > 0}
	<div class="flex h-full flex-col">
		<div
			class="flex flex-shrink-0 flex-wrap items-center gap-3 border-b border-gray-100 px-4 py-2 dark:border-gray-800"
		>
			<span class="text-xs text-gray-500 dark:text-gray-400">
				{exportLines.length > PREVIEW_LINES
					? m.dataquery_export_truncated({ shown: PREVIEW_LINES })
					: m.dataquery_export_rows({ count: formatNumber(result.rows.length) })}
			</span>
			<div class="ml-auto flex items-center gap-2">
				<button
					type="button"
					onclick={copyExport}
					class="inline-flex items-center gap-1.5 rounded-lg border border-gray-200 bg-white px-3 py-1.5 text-xs font-medium text-gray-700 transition hover:bg-gray-50 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-200 dark:hover:bg-gray-700"
				>
					<IconifyIcon
						icon={copied ? 'material-symbols:check' : 'material-symbols:content-copy-outline'}
						class="h-4 w-4"
					/>
					{copied ? m.common_copied() : m.common_copy()}
				</button>
				<button
					type="button"
					onclick={downloadExport}
					class="inline-flex items-center gap-1.5 rounded-lg bg-earthy-terracotta-600 px-3 py-1.5 text-xs font-semibold text-white transition hover:bg-earthy-terracotta-500"
				>
					<IconifyIcon icon="material-symbols:download" class="h-4 w-4" />
					{m.dataquery_export_download()}
				</button>
			</div>
		</div>
		<pre
			class="min-h-0 flex-1 overflow-auto bg-gray-950 p-4 font-mono text-xs leading-relaxed text-gray-100">{exportLines
				.slice(0, PREVIEW_LINES)
				.join('\n')}</pre>
	</div>
{:else if result.columns.length === 0}
	<p class="p-6 text-sm text-gray-500">{m.dataquery_console_no_columns()}</p>
{:else}
	<table class="result-in min-w-full text-sm">
		<thead class="sticky top-0 z-10 bg-gray-50 dark:bg-gray-900">
			<tr>
				<th
					class="w-10 border-b border-r border-gray-200 px-2 py-2 text-right text-[10px] font-normal text-gray-400 dark:border-gray-700"
					>#</th
				>
				{#each result.columns as column, i (i)}
					<th
						class="whitespace-nowrap border-b border-gray-200 px-4 py-2 text-left dark:border-gray-700"
					>
						<div class="font-semibold text-gray-900 dark:text-gray-100">
							{column.name}
						</div>
						<div class="font-mono text-[10px] font-normal uppercase text-gray-400">
							{column.type}
						</div>
					</th>
				{/each}
			</tr>
		</thead>
		<tbody class="divide-y divide-gray-100 dark:divide-gray-800">
			{#each result.rows as row, r (r)}
				<tr class="hover:bg-earthy-terracotta-50/40 dark:hover:bg-gray-800/60">
					<td
						class="border-r border-gray-100 px-2 py-1.5 text-right font-mono text-[11px] tabular-nums text-gray-400 dark:border-gray-800"
						>{r + 1}</td
					>
					{#each result.columns as column, c (c)}
						<td
							class="max-w-xs truncate whitespace-nowrap px-4 py-1.5 font-mono text-xs text-gray-700 dark:text-gray-300 {isNumeric(
								column.type
							)
								? 'text-right tabular-nums'
								: ''}"
							title={cell(row[c])}
						>
							{#if row[c] === null || row[c] === undefined}
								<span class="italic text-gray-400">{m.dataquery_console_null()}</span>
							{:else}
								{cell(row[c])}
							{/if}
						</td>
					{/each}
				</tr>
			{/each}
		</tbody>
	</table>
{/if}

<style>
	.result-in {
		animation: result-in 0.3s cubic-bezier(0.2, 0.8, 0.2, 1) both;
	}

	@keyframes result-in {
		from {
			opacity: 0;
			transform: translateY(6px);
		}
	}

	@media (prefers-reduced-motion: reduce) {
		.result-in {
			animation: none;
		}
	}
</style>
