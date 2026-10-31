<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { m } from '$lib/paraglide/messages';
	import { getErrorMessage } from '$lib/stores/toast';
	import { formatNumber, formatRelativeTime } from '$lib/utils/format';
	import { getAssetQueryStats, listQueryHistory } from '$lib/dataquery/api';
	import { durationMs, outcomeReason, outcomeState, workspaceHref } from '$lib/dataquery/format';
	import type { QueryAssetStats, QueryRecord } from '$lib/dataquery/types';
	import { formatMs } from '$components/asset/assetFormat';
	import QueryDetail from './QueryDetail.svelte';
	import '@marmotdata/elements/query-history.js';
	import '@marmotdata/elements/sparkline.js';
	import type { QueryHistoryItem } from '@marmotdata/elements/classes/query-history.js';

	// The asset's Queries tab: queries that went through Marmot, newest first; the "Query table" button lives in the page header so it is not repeated here.
	let { mrn, provider = '' }: { mrn: string; name?: string; provider?: string } = $props();

	let queries = $state<QueryRecord[]>([]);
	let stats = $state<QueryAssetStats | null>(null);
	let total = $state(0);
	let loading = $state(true);
	let error = $state('');
	let selectedId = $state<string | null>(null);

	let items = $derived<QueryHistoryItem[]>(
		queries.map((q) => {
			const state = outcomeState(q);
			return {
				id: q.id,
				sql: q.statement,
				status: state === 'allowed' ? 'succeeded' : state,
				source: { name: q.source_name, provider: q.source_provider || provider },
				rows: q.rows_returned ?? null,
				durationMs: durationMs(q) ?? null,
				startedAt: q.started_at,
				reason: outcomeReason(q) || undefined
			};
		})
	);

	function load() {
		loading = true;
		error = '';
		// The tiles are a summary on top of the list, so a failure to load them leaves the list usable.
		Promise.all([listQueryHistory({ mrn, limit: 25 }), getAssetQueryStats(mrn).catch(() => null)])
			.then(([data, s]) => {
				queries = data.queries;
				total = data.total;
				stats = s;
			})
			.catch((e) => (error = getErrorMessage(e)))
			.finally(() => (loading = false));
	}

	$effect(() => {
		void mrn;
		load();
	});

	function sourceOf(id: string): string {
		return queries.find((q) => q.id === id)?.source_name ?? '';
	}
</script>

{#if loading}
	<mm-loading active caption={m.asset_page_queries_loading()}></mm-loading>
{:else if error}
	<mm-callout tone="danger" heading={m.asset_page_queries_error_heading()}>
		{error}
		<mm-button slot="actions" size="sm" variant="secondary" icon="refresh" onclick={load}
			>{m.common_retry()}</mm-button
		>
	</mm-callout>
{:else if total === 0}
	<mm-empty-state
		heading={m.asset_page_queries_empty_heading()}
		description={m.asset_page_queries_empty_body()}
		seed={9}
	></mm-empty-state>
{:else}
	<div class="mm-stack" style="--mm-gap: var(--space-6)">
		<mm-section
			heading={m.asset_page_queries_heading()}
			description={m.asset_page_queries_description()}
		>
			{#if stats}
				<mm-description-list layout="inline-grid" columns={4}>
					<mm-description term={m.asset_page_queries_14_days()}>
						<span class="mm-cluster">
							<mm-text numeric>{formatNumber(stats.queries)}</mm-text>
							{#if stats.per_day.length > 1}
								<mm-sparkline label={m.asset_page_queries_per_day_label()} values={stats.per_day}
								></mm-sparkline>
							{/if}
						</span>
					</mm-description>
					<mm-description term={m.asset_page_queries_people_agents()}
						><mm-text numeric>{formatNumber(stats.people + stats.agents)}</mm-text></mm-description
					>
					<mm-description term={m.asset_page_queries_median()}
						><mm-text numeric
							>{stats.median_duration_ms != null ? formatMs(stats.median_duration_ms) : ''}</mm-text
						></mm-description
					>
					<mm-description term={m.asset_page_queries_denied()}
						><mm-text numeric>{formatNumber(stats.denied)}</mm-text></mm-description
					>
				</mm-description-list>
			{:else}
				<mm-description-list layout="inline-grid" columns={2}>
					<mm-description term={m.asset_page_queries_total()}
						><mm-text numeric>{formatNumber(total)}</mm-text></mm-description
					>
					<mm-description term={m.asset_page_queries_last()}
						>{formatRelativeTime(queries[0].started_at)}</mm-description
					>
				</mm-description-list>
			{/if}
		</mm-section>
		<mm-query-history
			label={m.asset_page_queries_list_label()}
			queries={items}
			onmm-open-query={(e: CustomEvent<{ query: QueryHistoryItem }>) =>
				(selectedId = e.detail.query.id)}
			onmm-run={(e: CustomEvent<{ sql: string; query: QueryHistoryItem }>) =>
				goto(resolve(workspaceHref(sourceOf(e.detail.query.id), e.detail.sql, true) as '/query'))}
		></mm-query-history>
	</div>
{/if}

{#if selectedId}
	<QueryDetail queryId={selectedId} onClose={() => (selectedId = null)} />
{/if}
