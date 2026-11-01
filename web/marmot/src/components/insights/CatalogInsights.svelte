<script lang="ts">
	import '@marmotdata/elements/kpi-tile.js';
	import '@marmotdata/elements/proportion-bar.js';
	import { onMount } from 'svelte';
	import { resolve } from '$app/paths';
	import { fetchApi } from '$lib/api';
	import { m } from '$lib/paraglide/messages';
	import { formatRelativeTime } from '$lib/utils/format';
	import { assetRouteParams } from '$lib/dataquery/format';
	import { listStaleAssets, type StaleAsset } from '$lib/home/api';
	import RankedPanel, { type RankedItem } from './RankedPanel.svelte';
	import { RANGE_HOURS, percent, type InsightsRange } from './range';

	let { range }: { range: InsightsRange } = $props();

	interface AssetCount {
		asset_id: string;
		asset_type: string;
		asset_name: string;
		asset_provider: string;
		count: number;
	}

	interface QueryCount {
		query: string;
		count: number;
	}

	interface SeriesPoint {
		name: string;
		value: number;
		bucket_start: string;
	}

	const TOP_LIMIT = 8;

	let loading = $state(true);
	let error = $state('');
	let usageLoading = $state(true);
	let usageError = $state('');

	let owned = $state<{ count: number; total: number } | null>(null);
	let stale = $state<StaleAsset[]>([]);
	let staleTotal = $state(0);
	let staleLoading = $state(true);
	let emptySearches = $state<QueryCount[]>([]);
	let byType = $state<Record<string, number>>({});
	let byProvider = $state<Record<string, number>>({});
	let byOwner = $state<Record<string, number>>({});
	let topAssets = $state<AssetCount[]>([]);
	let topSearches = $state<QueryCount[]>([]);
	let views = $state<number[]>([]);
	let searches = $state<number[]>([]);

	function ranked(counts: Record<string, number>): [string, number][] {
		return Object.entries(counts).sort((a, b) => b[1] - a[1]);
	}

	let types = $derived(ranked(byType).map(([label, value]) => ({ label, value })));
	let providers = $derived<RankedItem[]>(
		ranked(byProvider)
			.slice(0, TOP_LIMIT)
			.map(([key, value]) => ({ key, label: key, value, provider: key.toLowerCase() }))
	);
	let owners = $derived<RankedItem[]>(
		ranked(byOwner)
			.slice(0, TOP_LIMIT)
			.map(([key, value]) => ({ key, label: key, value, avatar: 'person' }))
	);
	let viewed = $derived<RankedItem[]>(
		topAssets.map((a) => ({
			key: a.asset_id,
			label: a.asset_name || a.asset_id,
			value: a.count,
			provider: a.asset_provider?.toLowerCase() || undefined,
			href:
				a.asset_type && a.asset_provider && a.asset_name
					? resolve('/discover/[type]/[service]/[name]', {
							type: a.asset_type,
							service: a.asset_provider,
							name: a.asset_name
						})
					: undefined
		}))
	);
	let searched = $derived<RankedItem[]>(
		topSearches.map((q) => ({
			key: q.query,
			label: q.query,
			value: q.count,
			href: `${resolve('/discover')}?q=${encodeURIComponent(q.query)}`
		}))
	);

	let staleItems = $derived<RankedItem[]>(
		stale.map((a) => {
			const route = assetRouteParams(a.mrn);
			return {
				key: a.id,
				label: a.name,
				provider: a.providers?.[0]?.toLowerCase() || undefined,
				secondary: a.last_refreshed_at
					? m.insights_stale_refreshed({ when: formatRelativeTime(a.last_refreshed_at) })
					: a.pipeline_name || undefined,
				href: route ? resolve('/discover/[type]/[service]/[name]', route) : undefined
			};
		})
	);
	let unanswered = $derived<RankedItem[]>(
		emptySearches.map((q) => ({
			key: q.query,
			label: q.query,
			value: q.count,
			href: `${resolve('/discover')}?q=${encodeURIComponent(q.query)}`
		}))
	);

	const sum = (values: number[]) => values.reduce((n, v) => n + v, 0);
	// A 24 hour window covers one or two daily rollups, which is not a trend worth drawing
	let showSpark = $derived(range !== '24h');

	async function json<T>(path: string): Promise<T> {
		const res = await fetchApi(path);
		if (!res.ok) throw new Error(res.statusText);
		return res.json();
	}

	function dayKey(date: Date): string {
		return date.toLocaleDateString('en-CA');
	}

	async function loadInventory() {
		loading = true;
		error = '';
		try {
			const [ty, pr, ow, own] = await Promise.all([
				json<{ assets: Record<string, number> }>('/metrics/assets/by-type'),
				json<{ assets: Record<string, number> }>('/metrics/assets/by-provider'),
				json<{ assets: Record<string, number> }>('/metrics/assets/by-owner'),
				json<{ count: number; total: number }>('/metrics/assets/owned').catch(() => null)
			]);
			owned = own;
			byType = ty.assets ?? {};
			byProvider = pr.assets ?? {};
			byOwner = ow.assets ?? {};
		} catch {
			error = m.metrics_load_error();
		} finally {
			loading = false;
		}
	}

	// Staleness is measured against each pipeline's schedule rather than the chosen window, so it loads once
	async function loadStale() {
		staleLoading = true;
		try {
			const data = await listStaleAssets(TOP_LIMIT);
			stale = data.assets;
			staleTotal = data.total;
		} catch {
			stale = [];
			staleTotal = 0;
		} finally {
			staleLoading = false;
		}
	}

	async function loadUsage(current: InsightsRange) {
		usageLoading = true;
		usageError = '';
		const end = new Date();
		const start = new Date(end.getTime() - RANGE_HOURS[current] * 3_600_000);
		const window = `start=${encodeURIComponent(start.toISOString())}&end=${encodeURIComponent(end.toISOString())}`;
		// Every day in the window gets a bucket so quiet days read as zero rather than vanishing from the trend
		const days: Record<string, { views: number; searches: number }> = {};
		for (let d = new Date(start); d <= end; d = new Date(d.getTime() + 86_400_000)) {
			days[dayKey(d)] = { views: 0, searches: 0 };
		}
		try {
			const [assets, queries, empty, series] = await Promise.all([
				json<AssetCount[]>(`/metrics/top-assets?${window}&limit=${TOP_LIMIT}`),
				json<QueryCount[]>(`/metrics/top-queries?${window}&limit=${TOP_LIMIT}`),
				json<QueryCount[]>(`/metrics/top-queries?${window}&limit=${TOP_LIMIT}&zero_results=true`),
				// Searches that found nothing are a metric of their own, so the searches total adds both
				json<{ metrics: SeriesPoint[] | null }>(
					`/metrics?${window}&bucket_size=1d&aggregation=sum&metric_names=asset_views_total&metric_names=search_queries_detailed&metric_names=search_queries_empty`
				)
			]);
			topAssets = assets ?? [];
			topSearches = (queries ?? []).filter((q) => q.query);
			emptySearches = (empty ?? []).filter((q) => q.query);
			for (const point of series.metrics ?? []) {
				const day = days[dayKey(new Date(point.bucket_start))];
				if (!day) continue;
				if (point.name === 'asset_views_total') day.views += point.value;
				else day.searches += point.value;
			}
		} catch {
			topAssets = [];
			topSearches = [];
			emptySearches = [];
			usageError = m.metrics_load_error();
		} finally {
			views = Object.values(days).map((d) => d.views);
			searches = Object.values(days).map((d) => d.searches);
			usageLoading = false;
		}
	}

	onMount(() => {
		loadInventory();
		loadStale();
	});

	$effect(() => {
		loadUsage(range);
	});
</script>

{#if error || usageError}
	<mm-callout tone="danger" compact>{error || usageError}</mm-callout>
{/if}

<mm-kpi-row label={m.insights_catalog_kpi_row()}>
	<mm-kpi-tile
		label={m.insights_kpi_views()}
		value={sum(views)}
		spark={showSpark ? views : []}
		loading={usageLoading}
	></mm-kpi-tile>
	<mm-kpi-tile
		label={m.insights_kpi_searches()}
		value={sum(searches)}
		spark={showSpark ? searches : []}
		loading={usageLoading}
	></mm-kpi-tile>
	<mm-kpi-tile
		label={m.insights_kpi_stale()}
		value={staleTotal}
		context={m.insights_kpi_stale_context()}
		loading={staleLoading}
	></mm-kpi-tile>
	<mm-kpi-tile
		label={m.insights_kpi_owned()}
		value={owned ? percent(owned.count, owned.total) : '–'}
		context={m.insights_kpi_owned_context()}
		{loading}
	></mm-kpi-tile>
</mm-kpi-row>

<div class="grid items-start gap-space-6 lg:grid-cols-3">
	<RankedPanel
		heading={m.metrics_most_viewed_assets_title()}
		icon="visibility"
		emptyText={m.metrics_no_asset_views_hint()}
		items={viewed}
		loading={usageLoading}
	/>
	<RankedPanel
		heading={m.metrics_top_search_queries_title()}
		icon="search"
		emptyText={m.metrics_no_search_queries_hint()}
		items={searched}
		loading={usageLoading}
	/>
	<RankedPanel
		heading={m.insights_stale_heading()}
		icon="schedule"
		emptyText={m.insights_stale_empty()}
		items={staleItems}
		loading={staleLoading}
	/>
</div>

<div class="mm-split mm-split--even">
	<RankedPanel
		heading={m.metrics_top_asset_owners_title()}
		icon="group"
		emptyText={m.insights_owners_empty()}
		items={owners}
		{loading}
	/>
	<RankedPanel
		heading={m.insights_empty_searches_heading()}
		icon="search"
		emptyText={m.insights_empty_searches_empty()}
		items={unanswered}
		loading={usageLoading}
	/>
</div>

<div class="mm-split mm-split--even">
	<mm-panel
		heading={m.metrics_asset_types_title()}
		icon="category"
		padding="default"
		{loading}
		empty={!loading && types.length === 0}
	>
		<mm-proportion-bar
			label={m.metrics_asset_types_title()}
			data={types}
			list
			unit={m.insights_unit_assets()}
			category-label={m.insights_asset_type_label()}
		></mm-proportion-bar>
		<mm-empty-state slot="empty" compact description={m.insights_types_empty()}></mm-empty-state>
	</mm-panel>
	<RankedPanel
		heading={m.metrics_top_data_providers_title()}
		icon="database"
		emptyText={m.insights_providers_empty()}
		items={providers}
		{loading}
	/>
</div>
