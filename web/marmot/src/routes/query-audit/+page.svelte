<script lang="ts">
	import { onMount, untrack } from 'svelte';
	import '@marmotdata/elements/toolbar.js';
	import '@marmotdata/elements/audit-row.js';
	import '@marmotdata/elements/filter-pill.js';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { SvelteURLSearchParams } from 'svelte/reactivity';
	import { setCrumbs } from '$lib/mm';
	import { m } from '$lib/paraglide/messages';
	import { myAccess, loadMyAccess, holdsAtRoot } from '$lib/iam/myAccess';
	import { getErrorMessage, toasts } from '$lib/stores/toast';
	import { exportQueryHistory, listQueryHistory, listQuerySources } from '$lib/dataquery/api';
	import {
		actorKind,
		actorName,
		actorServiceAccount,
		durationMs,
		outcomeReason,
		outcomeState,
		subjectParts,
		tablesRead,
		workspaceHref
	} from '$lib/dataquery/format';
	import type {
		QueryHistoryFilter,
		QueryOutcomeFilter,
		QueryOutcomeReason,
		QueryRecord,
		QuerySource
	} from '$lib/dataquery/types';
	import QueryDetail from '$components/dataquery/QueryDetail.svelte';

	// The record of every data query, for whoever answers for how data is used: stewards and auditors as much as engineers, so it lives with governance rather than system settings. Every filter lives in the URL so other pages can link to a filtered audit, and ?id= opens one query's details.
	type Range = '24h' | '7d' | '30d' | '90d';

	const RANGES: Range[] = ['24h', '7d', '30d', '90d'];
	const HOURS: Record<Range, number> = { '24h': 24, '7d': 7 * 24, '30d': 30 * 24, '90d': 90 * 24 };
	const OUTCOMES: QueryOutcomeFilter[] = ['allowed', 'denied', 'failed', 'cancelled', 'running'];
	const PAGE_SIZE = 50;
	const SEARCH_DELAY_MS = 300;

	setCrumbs([{ label: m.queryaudit_heading() }]);

	let range = $state<Range>('24h');
	// The outcome select's value: an outcome, or an outcome and the reason within it as outcome:reason
	let outcome = $state('');
	let sourceFilter = $state('');
	let caller = $state('');
	let search = $state('');
	let currentPage = $state(1);

	let queries = $state<QueryRecord[]>([]);
	let total = $state(0);
	let loading = $state(true);
	let error = $state('');
	let exporting = $state(false);
	let detailId = $state<string | null>(null);

	let sources = $state<QuerySource[]>([]);
	let providers = $derived(new Map(sources.map((s) => [s.name, s.plugin_id])));
	let canView = $derived(holdsAtRoot($myAccess, 'access:view'));
	let hasFilters = $derived(!!outcome || !!sourceFilter || !!caller || !!search.trim());
	// A caller linked by id reads by name once one of their queries is on the page
	let callerLabel = $derived.by(() => {
		if (!caller) return '';
		const match = queries.find(
			(q) => q.audit_subject === caller || `${q.principal_type}:${q.principal_id}` === caller
		);
		return match ? subjectParts(match.audit_subject).name : subjectParts(caller).name;
	});

	let searchTimer: ReturnType<typeof setTimeout> | undefined;
	let request = 0;
	let lastSearch: string | null = null;
	let started = false;

	// Rounded to the minute so repeated loads ask for the same window
	function from(): string {
		const start = Math.floor((Date.now() - HOURS[range] * 3_600_000) / 60_000) * 60_000;
		return new Date(start).toISOString();
	}

	function filter(): QueryHistoryFilter {
		const [kind, reason] = outcome.split(':');
		return {
			source: sourceFilter,
			caller: caller || undefined,
			outcome: (kind as QueryOutcomeFilter) || undefined,
			reason: (reason as QueryOutcomeReason) || undefined,
			q: search.trim() || undefined,
			from: from()
		};
	}

	function readParams(params: URLSearchParams) {
		const r = params.get('range') as Range | null;
		range = r && RANGES.includes(r) ? r : '24h';
		const o = params.get('outcome') as QueryOutcomeFilter | null;
		const reason = params.get('reason') ?? '';
		outcome = o && OUTCOMES.includes(o) ? (reason ? `${o}:${reason}` : o) : '';
		sourceFilter = params.get('source') ?? '';
		caller = params.get('caller') ?? '';
		search = params.get('q') ?? '';
		currentPage = Math.max(1, Number(params.get('page')) || 1);
		detailId = params.get('id') || null;
	}

	function updateURL() {
		const params = new SvelteURLSearchParams();
		if (range !== '24h') params.set('range', range);
		const [kind, reason] = outcome.split(':');
		if (kind) params.set('outcome', kind);
		if (reason) params.set('reason', reason);
		if (sourceFilter) params.set('source', sourceFilter);
		if (caller) params.set('caller', caller);
		if (search.trim()) params.set('q', search.trim());
		if (currentPage > 1) params.set('page', String(currentPage));
		if (detailId) params.set('id', detailId);
		const qs = params.toString();
		lastSearch = qs ? `?${qs}` : '';
		goto(resolve(qs ? `/query-audit?${qs}` : '/query-audit'), {
			replaceState: true,
			noScroll: true,
			keepFocus: true
		});
	}

	async function load() {
		const id = ++request;
		loading = true;
		error = '';
		try {
			const data = await listQueryHistory({
				...filter(),
				limit: PAGE_SIZE,
				offset: (currentPage - 1) * PAGE_SIZE
			});
			if (id !== request) return;
			queries = data.queries;
			total = data.total;
		} catch (e) {
			if (id === request) error = getErrorMessage(e);
		} finally {
			if (id === request) loading = false;
		}
	}

	function refilter() {
		currentPage = 1;
		updateURL();
		load();
	}

	function onSearch(value: string, now = false) {
		search = value;
		clearTimeout(searchTimer);
		if (now) refilter();
		else searchTimer = setTimeout(refilter, SEARCH_DELAY_MS);
	}

	function longerRange() {
		const next = RANGES[RANGES.indexOf(range) + 1];
		if (!next) return;
		range = next;
		refilter();
	}

	function openDetail(id: string | null) {
		detailId = id;
		updateURL();
	}

	// The server streams the export with the log's own filters, so it covers the whole range rather than the page on screen
	async function exportCsv() {
		exporting = true;
		try {
			const blob = await exportQueryHistory(filter());
			const url = URL.createObjectURL(blob);
			const link = document.createElement('a');
			link.href = url;
			link.download = `query-audit-${range}.csv`;
			link.click();
			URL.revokeObjectURL(url);
		} catch (e) {
			toasts.error(getErrorMessage(e));
		} finally {
			exporting = false;
		}
	}

	// The audit row knows allowed, denied and failed; a cancelled query shows as failed with "Cancelled" as its reason, and a running one as allowed
	function auditOutcome(q: QueryRecord): 'allowed' | 'denied' | 'failed' {
		const state = outcomeState(q);
		if (state === 'cancelled') return 'failed';
		if (state === 'running') return 'allowed';
		return state;
	}

	function openInEditor(query: QueryRecord, run: boolean) {
		goto(resolve(workspaceHref(query.source_name, query.statement, run) as '/query'));
	}

	onMount(() => {
		loadMyAccess();
		listQuerySources({ limit: 200 })
			.then((data) => (sources = data.sources))
			.catch(() => {});
		return () => clearTimeout(searchTimer);
	});

	// Follow the URL (the first load, back and forward, and links in from elsewhere); the log loads once access is known, so someone without access:view never asks for it
	$effect(() => {
		const urlSearch = page.url.search;
		const allowed = canView;
		untrack(() => {
			const changed = urlSearch !== lastSearch;
			if (changed) {
				lastSearch = urlSearch;
				readParams(page.url.searchParams);
			}
			if (allowed && (changed || !started)) {
				started = true;
				load();
			}
		});
	});
</script>

<svelte:head><title>{m.queryaudit_page_title()}</title></svelte:head>

<div class="mm-stack audit">
	<mm-page-header heading={m.queryaudit_heading()} description={m.queryaudit_description()}>
		{#if canView}
			<mm-segmented-control
				slot="actions"
				label={m.dataquery_audit_range_label()}
				hide-label
				value={range}
				onmm-change={(e: CustomEvent<{ value: string }>) => {
					range = e.detail.value as Range;
					refilter();
				}}
			>
				<mm-segment value="24h">{m.dataquery_audit_range_24h()}</mm-segment>
				<mm-segment value="7d">{m.dataquery_audit_range_7d()}</mm-segment>
				<mm-segment value="30d">{m.dataquery_audit_range_30d()}</mm-segment>
				<mm-segment value="90d">{m.dataquery_audit_range_90d()}</mm-segment>
			</mm-segmented-control>
			<mm-button
				slot="actions"
				variant="secondary"
				icon="download"
				loading={exporting}
				disabled={total === 0 || !!error}
				onclick={exportCsv}>{m.queryaudit_export_csv()}</mm-button
			>
		{/if}
	</mm-page-header>

	{#if canView}
		<mm-toolbar label={m.queryaudit_filters_label()}>
			<mm-search-field
				label={m.dataquery_audit_search_placeholder()}
				hide-label
				placeholder={m.dataquery_audit_search_placeholder()}
				shortcut="/"
				value={search}
				onmm-input={(e: CustomEvent<{ value: string }>) => onSearch(e.detail.value)}
				onmm-search={(e: CustomEvent<{ value: string }>) => onSearch(e.detail.value, true)}
				class="search"
			></mm-search-field>
			<mm-select
				label={m.dataquery_audit_outcome_label()}
				hide-label
				value={outcome}
				onmm-change={(e: CustomEvent<{ value: string }>) => {
					outcome = e.detail.value;
					refilter();
				}}
				class="outcome"
			>
				<mm-option value="">{m.queryaudit_outcome_all()}</mm-option>
				<mm-option-group label={m.dataquery_decision_allowed()}>
					<mm-option value="allowed">{m.queryaudit_outcome_any_reason()}</mm-option>
					<mm-option value="allowed:role">{m.queryaudit_outcome_by_role()}</mm-option>
				</mm-option-group>
				<mm-option-group label={m.dataquery_decision_denied()}>
					<mm-option value="denied">{m.queryaudit_outcome_any_reason()}</mm-option>
					<mm-option value="denied:permission">{m.queryaudit_outcome_by_permission()}</mm-option>
					<mm-option value="denied:policy">{m.queryaudit_outcome_by_policy()}</mm-option>
				</mm-option-group>
				<mm-option-group label={m.dataquery_status_failed()}>
					<mm-option value="failed">{m.queryaudit_outcome_any_reason()}</mm-option>
					<mm-option value="failed:source_error">{m.queryaudit_outcome_source_error()}</mm-option>
					<mm-option value="failed:timeout">{m.queryaudit_outcome_timeout()}</mm-option>
					<mm-option value="failed:unreachable">{m.queryaudit_outcome_unreachable()}</mm-option>
				</mm-option-group>
				<mm-option value="cancelled">{m.dataquery_status_cancelled()}</mm-option>
			</mm-select>
			<mm-select
				label={m.dataquery_query_col_source()}
				hide-label
				value={sourceFilter}
				onmm-change={(e: CustomEvent<{ value: string }>) => {
					sourceFilter = e.detail.value;
					refilter();
				}}
				class="source"
			>
				<mm-option value="">{m.dataquery_query_filter_all_sources()}</mm-option>
				{#if sourceFilter && !providers.has(sourceFilter)}
					<mm-option value={sourceFilter}>{sourceFilter}</mm-option>
				{/if}
				{#each sources as s (s.id)}
					<mm-option value={s.name} provider={s.plugin_id}>{s.name}</mm-option>
				{/each}
			</mm-select>
			{#if caller}
				<mm-filter-pill
					removable
					facet={m.queryaudit_caller_facet()}
					onmm-remove={() => {
						caller = '';
						refilter();
					}}>{callerLabel}</mm-filter-pill
				>
			{/if}
		</mm-toolbar>

		<mm-audit-log
			label={m.queryaudit_log_label()}
			loading={loading && queries.length === 0}
			{error}
			empty-heading={hasFilters ? m.dataquery_query_log_no_match() : m.queryaudit_empty_heading()}
			empty-description={hasFilters
				? m.queryaudit_empty_filtered_body()
				: m.queryaudit_empty_body()}
			onmm-retry={load}
			class:stale={loading && queries.length > 0}
		>
			{#if !error}
				{#each queries as q (q.id)}
					<mm-audit-row
						time={q.started_at}
						actor={actorName(q)}
						actor-kind={actorKind(q)}
						service-account={actorServiceAccount(q)}
						client={q.client}
						source={q.source_name}
						provider={providers.get(q.source_name) ?? ''}
						query={q.statement}
						rows={q.rows_returned ?? undefined}
						truncated={q.truncated}
						outcome={auditOutcome(q)}
						reason={outcomeReason(q)}
						duration={durationMs(q)}
						tables={tablesRead(q)}
						query-id={q.id}
						selected={detailId === q.id}
						onmm-open-in-editor={() => openInEditor(q, false)}
						onmm-run-again={() => openInEditor(q, true)}
					>
						<mm-button
							slot="actions"
							size="sm"
							variant="ghost"
							icon="right-panel-open"
							onclick={() => openDetail(q.id)}>{m.queryaudit_view_details()}</mm-button
						>
					</mm-audit-row>
				{/each}
			{/if}
			{#if !hasFilters && range !== '90d'}
				<mm-button slot="empty-action" variant="secondary" icon="history" onclick={longerRange}
					>{m.queryaudit_longer_range()}</mm-button
				>
			{/if}
		</mm-audit-log>

		{#if !error && total > PAGE_SIZE}
			<mm-pagination
				page={currentPage}
				page-size={PAGE_SIZE}
				{total}
				label={m.queryaudit_pages_label()}
				onmm-page-change={(e: CustomEvent<{ page: number; pageSize: number }>) => {
					currentPage = e.detail.page;
					updateURL();
					load();
				}}
			></mm-pagination>
		{/if}
	{:else if $myAccess.checked}
		<mm-empty-state
			heading={m.queryaudit_no_access_title()}
			description={m.queryaudit_no_access_description()}
		></mm-empty-state>
	{/if}
</div>

{#if detailId}
	<QueryDetail queryId={detailId} onClose={() => openDetail(null)} />
{/if}

<style>
	.audit {
		--mm-gap: var(--space-5);
	}

	.search {
		inline-size: min(100%, 22rem);
	}

	.outcome {
		inline-size: 12rem;
	}

	.source {
		inline-size: 12rem;
	}

	.stale {
		opacity: 0.6;
		transition: opacity var(--mm-dur-press) var(--mm-ease);
	}
</style>
