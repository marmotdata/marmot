<script lang="ts">
	import '@marmotdata/elements/kpi-tile.js';
	import type { DataTableColumn } from '@marmotdata/elements/classes/data-table';
	import { resolve } from '$app/paths';
	import { m } from '$lib/paraglide/messages';
	import { getQueryStats, listQueryHistory } from '$lib/dataquery/api';
	import { assetRouteParams, subjectParts } from '$lib/dataquery/format';
	import type { QueryRecord, QueryStats } from '$lib/dataquery/types';
	import RankedPanel, { type RankedItem } from './RankedPanel.svelte';
	import { percent, rangeStart, type InsightsRange } from './range';

	// Callers without access:view still see their own activity, but only access:view holders can open the query audit log
	let { range, canViewLog }: { range: InsightsRange; canViewLog: boolean } = $props();

	const RECENT_LIMIT = 8;

	let stats = $state<QueryStats | null>(null);
	let statsLoading = $state(true);
	let statsError = $state('');

	let recent = $state<QueryRecord[]>([]);
	let recentLoading = $state(true);
	let recentError = $state('');

	let totals = $derived(stats?.totals);
	let spark = $derived((stats?.series ?? []).map((p) => p.succeeded + p.failed + p.denied));

	function providerOf(mrn: string): string | undefined {
		return assetRouteParams(mrn)?.service.toLowerCase();
	}

	let tables = $derived<RankedItem[]>(
		(stats?.top_tables ?? []).map((t) => {
			const route = assetRouteParams(t.key);
			return {
				key: t.key,
				label: t.label || route?.name || t.key,
				value: t.total,
				provider: providerOf(t.key),
				href: route ? resolve('/discover/[type]/[service]/[name]', route) : undefined
			};
		})
	);

	let callers = $derived<RankedItem[]>(
		(stats?.top_callers ?? []).map((c) => ({
			key: c.key,
			label: subjectParts(c.label).name || c.key,
			value: c.total,
			avatar: c.principal_type === 'service_account' ? 'agent' : 'person'
		}))
	);

	let sources = $derived<RankedItem[]>(
		(stats?.top_sources ?? []).map((s) => ({
			key: s.key,
			label: s.label || s.key,
			value: s.total,
			provider: s.plugin_id || undefined,
			// Each source opens its pipeline, where its schedule, runs and query settings live
			href: s.schedule_id ? resolve('/runs/[id]', { id: s.schedule_id }) : undefined
		}))
	);

	// Allowed is the default outcome, so the status column leaves it without a pill
	function outcomeState(q: QueryRecord): string {
		if (q.decision === 'denied') return 'denied';
		if (q.status === 'failed') return 'failed';
		if (q.status === 'running') return 'running';
		return 'allowed';
	}

	const columns: DataTableColumn<QueryRecord>[] = [
		{
			key: 'who',
			header: m.dataquery_query_col_who(),
			width: 200,
			value: (q) => subjectParts(q.audit_subject).name
		},
		{ key: 'statement', header: m.insights_col_statement(), type: 'mono', minWidth: 200 },
		{ key: 'source_name', header: m.dataquery_query_col_source(), width: 160 },
		{
			key: 'outcome',
			header: m.dataquery_query_col_outcome(),
			type: 'status',
			width: 112,
			value: outcomeState
		},
		{ key: 'started_at', header: m.insights_col_when(), type: 'date', width: 120 }
	];

	async function loadStats(from: Date) {
		statsLoading = true;
		statsError = '';
		try {
			stats = await getQueryStats(from);
		} catch {
			stats = null;
			statsError = m.insights_queries_error();
		} finally {
			statsLoading = false;
		}
	}

	async function loadRecent(from: Date) {
		recentLoading = true;
		recentError = '';
		try {
			recent = (await listQueryHistory({ from: from.toISOString(), limit: RECENT_LIMIT })).queries;
		} catch {
			recent = [];
			recentError = m.insights_recent_error();
		} finally {
			recentLoading = false;
		}
	}

	$effect(() => {
		const from = rangeStart(range);
		loadStats(from);
		loadRecent(from);
	});

	let auditHref = $derived(canViewLog ? resolve('/query-audit') : '');
</script>

{#if statsError}
	<mm-callout tone="danger" compact>
		{statsError}
		<!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions (mm-button is a real button inside) -->
		<mm-button slot="actions" size="sm" icon="refresh" onclick={() => loadStats(rangeStart(range))}
			>{m.insights_try_again()}</mm-button
		>
	</mm-callout>
{/if}

<mm-kpi-row label={m.insights_queries_kpi_row()}>
	<mm-kpi-tile
		label={m.insights_kpi_queries()}
		value={totals?.queries ?? 0}
		spark={spark.length > 1 ? spark : []}
		loading={statsLoading}
	></mm-kpi-tile>
	<mm-kpi-tile
		label={m.dataquery_decision_allowed()}
		value={percent(totals?.allowed ?? 0, totals?.queries ?? 0)}
		loading={statsLoading}
	></mm-kpi-tile>
	<mm-kpi-tile
		label={m.dataquery_decision_denied()}
		value={totals?.denied ?? 0}
		context={m.insights_kpi_denied_context()}
		loading={statsLoading}
	></mm-kpi-tile>
	<mm-kpi-tile
		label={m.dataquery_status_failed()}
		value={totals?.failed ?? 0}
		context={m.insights_kpi_failed_context()}
		loading={statsLoading}
	></mm-kpi-tile>
</mm-kpi-row>

<div class="grid items-start gap-space-6 lg:grid-cols-2">
	<RankedPanel
		heading={m.insights_tables_heading()}
		icon="table"
		emptyText={m.insights_tables_empty()}
		items={tables}
		loading={statsLoading}
	/>
	<RankedPanel
		heading={m.insights_callers_heading()}
		icon="group"
		emptyText={m.insights_callers_empty()}
		items={callers}
		loading={statsLoading}
		linkHref={auditHref}
		linkLabel={m.insights_open_audit()}
	/>
</div>

<div class="mm-split">
	<mm-panel
		heading={m.insights_recent_heading()}
		icon="history"
		link-href={auditHref || undefined}
		link-label={auditHref ? m.insights_recent_link() : undefined}
	>
		<mm-data-table
			label={m.insights_recent_label()}
			density="compact"
			{columns}
			rows={recent}
			loading={recentLoading}
			error={recentError}
			empty-heading={m.insights_recent_empty_heading()}
			empty-text={m.insights_recent_empty_text()}
		></mm-data-table>
		{#if recentError}
			<!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions (mm-button is a real button inside) -->
			<mm-button
				slot="actions"
				size="sm"
				variant="ghost"
				icon="refresh"
				onclick={() => loadRecent(rangeStart(range))}>{m.insights_try_again()}</mm-button
			>
		{/if}
	</mm-panel>
	<RankedPanel
		heading={m.insights_sources_heading()}
		icon="database"
		emptyText={m.insights_sources_empty()}
		items={sources}
		loading={statsLoading}
	/>
</div>
