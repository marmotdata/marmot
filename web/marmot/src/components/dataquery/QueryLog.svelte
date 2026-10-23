<script lang="ts" module>
	export type Outcome = '' | 'completed' | 'denied' | 'failed';
</script>

<script lang="ts">
	import { onMount } from 'svelte';
	import IconifyIcon from '@iconify/svelte';
	import { m } from '$lib/paraglide/messages';
	import { getLocale } from '$lib/paraglide/runtime';
	import { getErrorMessage } from '$lib/stores/toast';
	import { formatDateTime, formatNumber } from '$lib/utils/format';
	import { listQueryHistory } from '$lib/dataquery/api';
	import { clientLabel, outcome, subjectParts } from '$lib/dataquery/format';
	import type { QueryHistoryFilter, QueryRecord } from '$lib/dataquery/types';
	import QueryDetail from './QueryDetail.svelte';
	import SourcePicker from './SourcePicker.svelte';
	import Alert from '$components/ui/Alert.svelte';
	import Skeleton from '$components/ui/Skeleton.svelte';
	import EmptyState from '$components/ui/EmptyState.svelte';
	import SegmentedControl from '$components/ui/SegmentedControl.svelte';
	import { BTN_GHOST, PANEL, SEARCH_FIELD, TABLE, TBODY, TD, TH, TR } from '$lib/ui/classes';

	const PAGE_SIZE = 25;
	const SEARCH_DELAY_MS = 300;
	const TABLE_CHIPS = 2;

	// Mine limits the log to the caller's own queries, for the Query page's history; the query audit page shows everyone's
	// Heading is left off where the page or blade around the log already names it
	// Flush is set when the log sits edge to edge in a flush Panel, so the table drops its own border and the toolbar takes the panel's padding
	// From bounds the log to queries started since then, and outcome is bindable so a summary above the log can filter it
	let {
		mine = false,
		heading = true,
		flush = false,
		from = undefined,
		outcome: outcomeFilter = $bindable('')
	}: {
		mine?: boolean;
		heading?: boolean;
		flush?: boolean;
		from?: string;
		outcome?: Outcome;
	} = $props();

	let queries = $state<QueryRecord[]>([]);
	let total = $state(0);
	let offset = $state(0);
	let loading = $state(true);
	let error = $state('');

	let sourceFilter = $state('');
	let search = $state('');
	// The search as last sent, which trails the field by a short pause so typing does not fire a request per key
	let appliedSearch = $state('');
	let searchTimer: ReturnType<typeof setTimeout> | undefined;

	let selectedId = $state<string | null>(null);

	let hasFilters = $derived(!!sourceFilter || !!outcomeFilter || !!search.trim());
	let rangeEnd = $derived(Math.min(offset + queries.length, total));

	const outcomeOptions: { id: Outcome; label: string }[] = [
		{ id: '', label: m.dataquery_outcome_all() },
		{ id: 'completed', label: m.dataquery_status_completed() },
		{ id: 'denied', label: m.dataquery_decision_denied() },
		{ id: 'failed', label: m.dataquery_status_failed() }
	];

	// One outcome filter stands for the decision and status pair, since a denied query only ever has the denied status
	function outcomeParams(value: Outcome): Pick<QueryHistoryFilter, 'decision' | 'status'> {
		if (value === 'denied') return { decision: 'denied' };
		if (value === 'completed' || value === 'failed') return { status: value };
		return {};
	}

	async function load() {
		loading = true;
		error = '';
		try {
			const data = await listQueryHistory({
				source: sourceFilter,
				...outcomeParams(outcomeFilter),
				q: appliedSearch || undefined,
				from,
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

	function onSearchInput() {
		clearTimeout(searchTimer);
		searchTimer = setTimeout(() => (appliedSearch = search.trim()), SEARCH_DELAY_MS);
	}

	function setOutcome(value: Outcome) {
		outcomeFilter = value;
	}

	function clearFilters() {
		sourceFilter = '';
		search = '';
		appliedSearch = '';
		outcomeFilter = '';
	}

	function goTo(newOffset: number) {
		offset = Math.max(0, newOffset);
		load();
	}

	// Reload whenever a filter changes, including the range and outcome set from outside through the props
	let lastKey = '';
	$effect(() => {
		const key = `${from ?? ''}|${outcomeFilter}|${sourceFilter}|${appliedSearch}`;
		if (key === lastKey) return;
		const first = lastKey === '';
		lastKey = key;
		if (!first) applyFilters();
	});

	onMount(() => {
		load();
		return () => clearTimeout(searchTimer);
	});

	const dayFormat = new Intl.DateTimeFormat(getLocale(), {
		weekday: 'long',
		day: 'numeric',
		month: 'long'
	});
	const timeFormat = new Intl.DateTimeFormat(getLocale(), { hour: 'numeric', minute: '2-digit' });

	function dayKey(iso: string): string {
		const d = new Date(iso);
		return `${d.getFullYear()}-${d.getMonth()}-${d.getDate()}`;
	}

	function dayLabel(iso: string): string {
		const key = dayKey(iso);
		const today = new Date();
		if (key === dayKey(today.toISOString())) return m.dataquery_audit_today();
		const yesterday = new Date(today.getTime() - 86_400_000);
		if (key === dayKey(yesterday.toISOString())) return m.dataquery_audit_yesterday();
		return dayFormat.format(new Date(iso));
	}

	// The statement's first meaningful line is enough to recognise a query in a list; the blade shows it whole
	function statementPreview(statement: string): string {
		return statement
			.replace(/--[^\n]*/g, ' ')
			.replace(/\s+/g, ' ')
			.trim();
	}

	// mrn://table/postgres/shop.public.orders reads as public.orders: the schema and table say which table without the service prefix
	function tableName(mrn: string): string {
		const last = mrn.split('/').pop() ?? mrn;
		const parts = last.split('.');
		return parts.length > 2 ? parts.slice(-2).join('.') : last;
	}

	function tables(query: QueryRecord): string[] {
		return (query.referenced_mrns ?? []).filter((mrn) => !mrn.startsWith('mrn://source/'));
	}

	function reason(query: QueryRecord): string | null {
		if (query.decision === 'denied') return query.decision_detail?.reason ?? null;
		if (query.status === 'failed') return query.error ?? null;
		return null;
	}

	function principalIcon(query: QueryRecord): string {
		return query.principal_type === 'service_account'
			? 'material-symbols:smart-toy-outline'
			: 'material-symbols:person-outline';
	}
</script>

<div class={flush ? '' : 'space-y-4'}>
	<div
		class="flex flex-col gap-3 {flush
			? 'border-b border-gray-100 px-5 py-4 dark:border-gray-700/60'
			: ''}"
	>
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
			<label class="{SEARCH_FIELD} w-full sm:w-80">
				<IconifyIcon icon="material-symbols:search" class="h-4 w-4 text-gray-400" />
				<input
					bind:value={search}
					oninput={onSearchInput}
					type="search"
					placeholder={m.dataquery_audit_search_placeholder()}
					aria-label={m.dataquery_audit_search_placeholder()}
					class="min-w-0 flex-1 border-0 bg-transparent p-0 text-sm text-gray-900 placeholder:text-gray-400 focus:outline-none focus:ring-0 dark:text-gray-100"
				/>
			</label>
			<SourcePicker
				bind:value={sourceFilter}
				emptyLabel={m.dataquery_query_filter_all_sources()}
				size="sm"
			/>
			<SegmentedControl
				options={outcomeOptions}
				value={outcomeFilter}
				onChange={setOutcome}
				size="sm"
				label={m.dataquery_audit_outcome_label()}
			/>
			{#if hasFilters}
				<button type="button" onclick={clearFilters} class="{BTN_GHOST} !text-xs">
					{m.common_clear()}
				</button>
			{/if}
		</div>
	</div>

	{#if loading && queries.length === 0}
		<div class={flush ? 'p-5' : ''}><Skeleton class="h-10" rows={5} /></div>
	{:else if error}
		<div class={flush ? 'p-5' : ''}><Alert>{error}</Alert></div>
	{:else if queries.length === 0}
		<EmptyState
			dashed={!flush}
			icon="material-symbols:receipt-long-outline"
			title={hasFilters ? m.dataquery_query_log_no_match() : m.dataquery_query_log_empty()}
		/>
	{:else}
		<div
			class="{flush ? '' : `${PANEL} overflow-hidden`} {loading
				? 'opacity-60 transition-opacity'
				: ''}"
		>
			<div class="overflow-x-auto">
				<table class="{TABLE} w-full">
					<thead>
						<tr class="border-b border-gray-100 dark:border-gray-700/60">
							<th class="{TH} w-20">{m.dataquery_query_col_time()}</th>
							<th class={TH}>{m.dataquery_query_col_who()}</th>
							<th class="{TH} w-full">{m.dataquery_query_col_query()}</th>
							<th class={TH}>{m.dataquery_query_col_source()}</th>
							<th class={TH}>{m.dataquery_query_col_outcome()}</th>
							<th class="{TH} text-right">{m.dataquery_query_col_rows()}</th>
						</tr>
					</thead>
					<tbody class={TBODY}>
						{#each queries as query, i (query.id)}
							{@const who = subjectParts(query.audit_subject)}
							{@const badge = outcome(query)}
							{@const read = tables(query)}
							{@const why = reason(query)}
							{#if i === 0 || dayKey(query.started_at) !== dayKey(queries[i - 1].started_at)}
								<tr class="bg-gray-50/80 dark:bg-gray-900/40">
									<th
										colspan="6"
										scope="colgroup"
										class="px-4 py-1.5 text-left text-xs font-medium text-gray-500 dark:text-gray-400"
									>
										{dayLabel(query.started_at)}
									</th>
								</tr>
							{/if}
							<tr
								class="{TR} cursor-pointer align-top {selectedId === query.id
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
									class="{TD} whitespace-nowrap tabular-nums text-gray-500 dark:text-gray-400"
									title={formatDateTime(query.started_at)}
								>
									{timeFormat.format(new Date(query.started_at))}
								</td>
								<td class="{TD} whitespace-nowrap">
									<span class="inline-flex items-center gap-2" title={who.kind}>
										<IconifyIcon
											icon={principalIcon(query)}
											class="h-4 w-4 flex-shrink-0 text-gray-400"
											aria-label={who.kind}
										/>
										<span class="font-medium text-gray-900 dark:text-gray-100">{who.name}</span>
									</span>
								</td>
								<td class="{TD} max-w-0">
									<code
										class="block truncate font-mono text-xs text-gray-800 dark:text-gray-200"
										title={query.statement}>{statementPreview(query.statement)}</code
									>
									{#if why}
										<span class="mt-1 block truncate text-xs text-red-600 dark:text-red-400"
											>{why}</span
										>
									{:else if read.length > 0}
										<span class="mt-1 flex flex-wrap items-center gap-1">
											{#each read.slice(0, TABLE_CHIPS) as mrn (mrn)}
												<span
													class="inline-flex items-center gap-1 rounded bg-gray-100 px-1.5 py-0.5 text-[11px] text-gray-600 dark:bg-gray-700/60 dark:text-gray-300"
													title={mrn}
												>
													<IconifyIcon icon="material-symbols:table-outline" class="h-3 w-3" />
													{tableName(mrn)}
												</span>
											{/each}
											{#if read.length > TABLE_CHIPS}
												<span class="text-[11px] text-gray-500 dark:text-gray-400"
													>{m.dataquery_audit_more_tables({
														count: formatNumber(read.length - TABLE_CHIPS)
													})}</span
												>
											{/if}
										</span>
									{/if}
								</td>
								<td class="{TD} whitespace-nowrap">
									<span class="block text-gray-900 dark:text-gray-100">{query.source_name}</span>
									<span class="block text-xs text-gray-500 dark:text-gray-400"
										>{m.dataquery_audit_via({ client: clientLabel(query.client) })}</span
									>
								</td>
								<td class="{TD} whitespace-nowrap">
									<span
										class="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium {badge.classes}"
										>{badge.label}</span
									>
								</td>
								<td class="{TD} whitespace-nowrap text-right tabular-nums">
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
				class="flex items-center justify-between border-t border-gray-100 px-4 py-3 dark:border-gray-700/60"
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
						class="{BTN_GHOST} !text-xs"
					>
						<IconifyIcon icon="material-symbols:chevron-left" class="h-4 w-4" />
						{m.common_previous()}
					</button>
					<button
						type="button"
						onclick={() => goTo(offset + PAGE_SIZE)}
						disabled={offset + PAGE_SIZE >= total || loading}
						class="{BTN_GHOST} !text-xs"
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
