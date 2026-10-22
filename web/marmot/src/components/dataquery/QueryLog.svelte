<script lang="ts">
	import { onMount } from 'svelte';
	import IconifyIcon from '@iconify/svelte';
	import { m } from '$lib/paraglide/messages';
	import { getErrorMessage } from '$lib/stores/toast';
	import { formatDateTime, formatNumber } from '$lib/utils/format';
	import { listQueryHistory } from '$lib/dataquery/api';
	import { clientLabel, outcome, subjectParts } from '$lib/dataquery/format';
	import type { QueryDecision, QueryRecord, QueryRecordStatus } from '$lib/dataquery/types';
	import QueryDetail from './QueryDetail.svelte';
	import SourcePicker from './SourcePicker.svelte';

	const PAGE_SIZE = 25;

	// Mine limits the log to the caller's own queries, for the Query page's history; the Data Access page shows everyone's
	// Heading is left off where the page or blade around the log already names it
	let { mine = false, heading = true }: { mine?: boolean; heading?: boolean } = $props();

	let queries = $state<QueryRecord[]>([]);
	let total = $state(0);
	let offset = $state(0);
	let loading = $state(true);
	let error = $state('');

	let sourceFilter = $state('');
	let decisionFilter = $state<QueryDecision | ''>('');
	let statusFilter = $state<QueryRecordStatus | ''>('');

	let selectedId = $state<string | null>(null);

	let hasFilters = $derived(!!sourceFilter || !!decisionFilter || !!statusFilter);
	let rangeEnd = $derived(Math.min(offset + queries.length, total));

	async function load() {
		loading = true;
		error = '';
		try {
			const data = await listQueryHistory({
				source: sourceFilter,
				decision: decisionFilter,
				status: statusFilter,
				mine: mine || undefined,
				limit: PAGE_SIZE,
				offset
			});
			queries = data.queries;
			total = data.total;
		} catch (e) {
			error = getErrorMessage(e);
		} finally {
			loading = false;
		}
	}

	function applyFilters() {
		offset = 0;
		load();
	}

	function clearFilters() {
		sourceFilter = '';
		decisionFilter = '';
		statusFilter = '';
		applyFilters();
	}

	function goTo(newOffset: number) {
		offset = Math.max(0, newOffset);
		load();
	}

	onMount(async () => {
		load();
	});

	const selectClasses =
		'appearance-none pl-3 pr-9 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-earthy-terracotta-600 focus:border-transparent';
</script>

<div class="space-y-4">
	<div class="flex flex-col lg:flex-row lg:items-end lg:justify-between gap-4">
		{#if heading}
			<div>
				<h2 class="text-base font-semibold text-gray-900 dark:text-gray-100">
					{m.dataquery_query_log_heading()}
				</h2>
				<p class="text-sm text-gray-500 dark:text-gray-400 mt-1 max-w-prose">
					{m.dataquery_query_log_description()}
				</p>
			</div>
		{/if}
		<div class="flex flex-wrap items-center gap-2">
			<SourcePicker
				bind:value={sourceFilter}
				emptyLabel={m.dataquery_query_filter_all_sources()}
				onchange={applyFilters}
				size="sm"
			/>
			<div class="relative">
				<select
					bind:value={decisionFilter}
					onchange={applyFilters}
					class={selectClasses}
					aria-label={m.dataquery_query_filter_decision_aria()}
				>
					<option value="">{m.dataquery_query_filter_all_decisions()}</option>
					<option value="allowed">{m.dataquery_decision_allowed()}</option>
					<option value="denied">{m.dataquery_decision_denied()}</option>
				</select>
				<IconifyIcon
					icon="material-symbols:expand-more"
					class="pointer-events-none absolute right-2 top-1/2 -translate-y-1/2 h-4 w-4 text-gray-400"
				/>
			</div>
			<div class="relative">
				<select
					bind:value={statusFilter}
					onchange={applyFilters}
					class={selectClasses}
					aria-label={m.dataquery_query_filter_status_aria()}
				>
					<option value="">{m.dataquery_query_filter_all_statuses()}</option>
					<option value="running">{m.dataquery_status_running()}</option>
					<option value="completed">{m.dataquery_status_completed()}</option>
					<option value="failed">{m.dataquery_status_failed()}</option>
					<option value="denied">{m.dataquery_status_denied()}</option>
				</select>
				<IconifyIcon
					icon="material-symbols:expand-more"
					class="pointer-events-none absolute right-2 top-1/2 -translate-y-1/2 h-4 w-4 text-gray-400"
				/>
			</div>
			{#if hasFilters}
				<button
					type="button"
					onclick={clearFilters}
					class="px-2.5 py-2 text-xs font-medium text-gray-600 dark:text-gray-300 hover:underline"
				>
					{m.common_clear()}
				</button>
			{/if}
		</div>
	</div>

	{#if loading && queries.length === 0}
		<div class="flex items-center justify-center py-16">
			<div class="animate-spin rounded-full h-8 w-8 border-b-2 border-earthy-terracotta-700"></div>
		</div>
	{:else if error}
		<div
			class="bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800/50 rounded-lg p-4"
		>
			<p class="text-sm text-red-700 dark:text-red-300">{error}</p>
		</div>
	{:else if queries.length === 0}
		<div
			class="bg-white dark:bg-gray-800 rounded-xl border border-dashed border-gray-300 dark:border-gray-600 p-12 text-center"
		>
			<IconifyIcon
				icon="material-symbols:receipt-long-outline"
				class="h-10 w-10 mx-auto text-gray-300 dark:text-gray-600"
			/>
			<p class="mt-3 text-sm text-gray-500 dark:text-gray-400">
				{hasFilters ? m.dataquery_query_log_no_match() : m.dataquery_query_log_empty()}
			</p>
		</div>
	{:else}
		<div
			class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 overflow-hidden"
		>
			<div class="overflow-x-auto">
				<table class="w-full text-sm">
					<thead>
						<tr
							class="text-left text-[11px] uppercase tracking-wider text-gray-500 dark:text-gray-400 bg-gray-50 dark:bg-gray-900/50 border-b border-gray-200 dark:border-gray-700"
						>
							<th class="px-4 py-3 font-semibold">{m.dataquery_query_col_time()}</th>
							<th class="px-4 py-3 font-semibold">{m.dataquery_query_col_who()}</th>
							<th class="px-4 py-3 font-semibold">{m.dataquery_query_col_client()}</th>
							<th class="px-4 py-3 font-semibold">{m.dataquery_query_col_source()}</th>
							<th class="px-4 py-3 font-semibold">{m.dataquery_query_col_outcome()}</th>
							<th class="px-4 py-3 font-semibold text-right">{m.dataquery_query_col_rows()}</th>
						</tr>
					</thead>
					<tbody class="divide-y divide-gray-100 dark:divide-gray-700/60">
						{#each queries as query (query.id)}
							{@const who = subjectParts(query.audit_subject)}
							{@const badge = outcome(query)}
							<tr
								class="cursor-pointer transition-colors hover:bg-gray-50 dark:hover:bg-gray-700/40 {selectedId ===
								query.id
									? 'bg-gray-50 dark:bg-gray-700/40'
									: ''}"
								onclick={() => (selectedId = query.id)}
								onkeydown={(e) => {
									if (e.key === 'Enter' || e.key === ' ') {
										e.preventDefault();
										selectedId = query.id;
									}
								}}
								tabindex="0"
								aria-label={m.dataquery_query_open_aria({ source: query.source_name })}
							>
								<td
									class="px-4 py-3 whitespace-nowrap text-gray-600 dark:text-gray-400 tabular-nums"
								>
									{formatDateTime(query.started_at)}
								</td>
								<td class="px-4 py-3 whitespace-nowrap">
									<span class="text-gray-900 dark:text-gray-100 font-medium">{who.name}</span>
									{#if who.kind}
										<span class="block text-xs text-gray-500 dark:text-gray-400">{who.kind}</span>
									{/if}
								</td>
								<td class="px-4 py-3 whitespace-nowrap text-gray-700 dark:text-gray-300">
									{clientLabel(query.client)}
								</td>
								<td class="px-4 py-3 whitespace-nowrap text-gray-700 dark:text-gray-300">
									{query.source_name}
								</td>
								<td class="px-4 py-3">
									<span
										class="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium {badge.classes}"
										>{badge.label}</span
									>
								</td>
								<td
									class="px-4 py-3 text-right whitespace-nowrap tabular-nums text-gray-700 dark:text-gray-300"
								>
									{query.rows_returned !== undefined && query.rows_returned !== null
										? formatNumber(query.rows_returned)
										: '—'}
									{#if query.truncated}
										<span
											class="ml-1 inline-flex px-1.5 py-0.5 rounded text-[10px] font-medium bg-amber-100 dark:bg-amber-900/30 text-amber-700 dark:text-amber-300"
											title={m.dataquery_query_truncated_title()}
											>{m.dataquery_query_truncated()}</span
										>
									{/if}
								</td>
							</tr>
						{/each}
					</tbody>
				</table>
			</div>

			<div
				class="flex items-center justify-between px-4 py-3 border-t border-gray-200 dark:border-gray-700 bg-gray-50/50 dark:bg-gray-900/30"
			>
				<span class="text-xs text-gray-500 dark:text-gray-400 tabular-nums">
					{m.dataquery_query_log_range({
						start: formatNumber(offset + 1),
						end: formatNumber(rangeEnd),
						total: formatNumber(total)
					})}
				</span>
				<div class="flex items-center gap-1">
					<button
						type="button"
						onclick={() => goTo(offset - PAGE_SIZE)}
						disabled={offset === 0 || loading}
						class="inline-flex items-center gap-1 px-2.5 py-1.5 rounded-md text-xs font-medium text-gray-600 dark:text-gray-300 hover:bg-gray-200 dark:hover:bg-gray-700 disabled:opacity-40 disabled:cursor-not-allowed"
					>
						<IconifyIcon icon="material-symbols:chevron-left" class="h-4 w-4" />
						{m.common_previous()}
					</button>
					<button
						type="button"
						onclick={() => goTo(offset + PAGE_SIZE)}
						disabled={offset + PAGE_SIZE >= total || loading}
						class="inline-flex items-center gap-1 px-2.5 py-1.5 rounded-md text-xs font-medium text-gray-600 dark:text-gray-300 hover:bg-gray-200 dark:hover:bg-gray-700 disabled:opacity-40 disabled:cursor-not-allowed"
					>
						{m.common_next()}
						<IconifyIcon icon="material-symbols:chevron-right" class="h-4 w-4" />
					</button>
				</div>
			</div>
		</div>
	{/if}
</div>

{#if selectedId}
	<QueryDetail queryId={selectedId} onClose={() => (selectedId = null)} />
{/if}
