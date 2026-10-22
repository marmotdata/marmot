<script lang="ts">
	import { onMount } from 'svelte';
	import { resolve } from '$app/paths';
	import IconifyIcon from '@iconify/svelte';
	import Icon from '$components/ui/Icon.svelte';
	import Sparkline from '$components/charts/Sparkline.svelte';
	import DonutChart, { type DonutSlice } from '$components/charts/DonutChart.svelte';
	import StackedColumns from '$components/charts/StackedColumns.svelte';
	import { fetchApi } from '$lib/api';
	import { m } from '$lib/paraglide/messages';
	import { formatNumber } from '$lib/utils/format';

	type Range = '7d' | '30d';

	interface Ranked {
		key: string;
		label: string;
		value: number;
		icon?: string;
		route?: { type: string; service: string; name: string };
	}

	interface Day {
		key: string;
		label: string;
		views: number;
		searches: number;
	}

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

	// Kept in step with the data query overview so both Insights tabs read as one dashboard
	const PANEL =
		'rounded-xl border border-gray-200 bg-white shadow-sm dark:border-gray-700 dark:bg-gray-800';
	const KPI_LABEL = 'text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400';
	const KPI_VALUE = 'mt-1 text-2xl font-semibold tabular-nums text-gray-900 dark:text-gray-100';
	const KPI_SUB = 'mt-0.5 text-xs text-gray-500 dark:text-gray-400';

	// Categorical hues in a fixed order, so a provider keeps its colour however the list is filtered; the remainder rolls up into one grey slice
	const SLICE_COLOURS = ['#758c9f', '#ffa382', '#8ba88b', '#e7c776', '#c6b4a2'];
	const OTHER_COLOUR = '#c9c9c9';
	const ACTIVITY_SERIES = [
		{
			key: 'views' as const,
			label: m.insights_legend_views(),
			fill: 'fill-earthy-blue-600 dark:fill-earthy-blue-500',
			swatch: 'bg-earthy-blue-600 dark:bg-earthy-blue-500'
		},
		{
			key: 'searches' as const,
			label: m.insights_legend_searches(),
			fill: 'fill-earthy-terracotta-300 dark:fill-earthy-terracotta-400',
			swatch: 'bg-earthy-terracotta-300 dark:bg-earthy-terracotta-400'
		}
	];

	// Views and searches are stored as daily rollups and the series endpoint serves at most 30 days
	const RANGE_DAYS: Record<Range, number> = { '7d': 7, '30d': 30 };
	const ranges: { id: Range; label: string }[] = [
		{ id: '7d', label: m.dataquery_range_7d() },
		{ id: '30d', label: m.dataquery_range_30d() }
	];

	let range = $state<Range>('30d');
	let loading = $state(true);
	let usageLoading = $state(true);
	let error = $state('');

	let total = $state(0);
	let withSchemas = $state(0);
	let schemaShare = $state(0);
	let byType = $state<Record<string, number>>({});
	let byProvider = $state<Record<string, number>>({});
	let byOwner = $state<Record<string, number>>({});
	let topAssets = $state<AssetCount[]>([]);
	let topSearches = $state<QueryCount[]>([]);
	let days = $state<Day[]>([]);

	function rank(counts: Record<string, number>, icon?: (key: string) => string): Ranked[] {
		return Object.entries(counts)
			.map(([key, value]) => ({ key, label: key, value, icon: icon?.(key) }))
			.sort((a, b) => b.value - a.value);
	}

	let types = $derived(rank(byType, (key) => key.toLowerCase()));
	let providers = $derived(rank(byProvider, (key) => key.toLowerCase()));
	let owners = $derived(rank(byOwner));
	let viewed = $derived<Ranked[]>(
		topAssets.map((a) => ({
			key: a.asset_id,
			label: a.asset_name || a.asset_id,
			value: a.count,
			icon: a.asset_provider?.toLowerCase(),
			route:
				a.asset_type && a.asset_provider && a.asset_name
					? { type: a.asset_type, service: a.asset_provider, name: a.asset_name }
					: undefined
		}))
	);
	let searches = $derived<Ranked[]>(
		topSearches.map((q) => ({ key: q.query, label: q.query, value: q.count }))
	);

	let totalViews = $derived(days.reduce((n, d) => n + d.views, 0));
	let totalSearches = $derived(days.reduce((n, d) => n + d.searches, 0));

	function toSlices(items: Ranked[]): DonutSlice[] {
		const shown = items.slice(0, SLICE_COLOURS.length);
		const rest = items.slice(SLICE_COLOURS.length).reduce((n, i) => n + i.value, 0);
		return [
			...shown.map((item, i) => ({
				key: item.key,
				label: item.label,
				value: item.value,
				colour: SLICE_COLOURS[i]
			})),
			...(rest > 0
				? [{ key: '__other', label: m.insights_other(), value: rest, colour: OTHER_COLOUR }]
				: [])
		];
	}

	function dayKey(date: Date): string {
		return date.toLocaleDateString('en-CA');
	}

	function emptyDays(count: number): Day[] {
		const out: Day[] = [];
		for (let i = count - 1; i >= 0; i--) {
			const date = new Date(Date.now() - i * 86_400_000);
			out.push({
				key: dayKey(date),
				label: date.toLocaleDateString([], { day: 'numeric', month: 'short' }),
				views: 0,
				searches: 0
			});
		}
		return out;
	}

	async function json<T>(path: string): Promise<T> {
		const res = await fetchApi(path);
		if (!res.ok) throw new Error(m.metrics_load_error());
		return res.json();
	}

	async function loadInventory() {
		loading = true;
		error = '';
		try {
			const [t, s, ty, pr, ow] = await Promise.all([
				json<{ count: number }>('/metrics/assets/total'),
				json<{ count: number; percentage: number }>('/metrics/assets/with-schemas'),
				json<{ assets: Record<string, number> }>('/metrics/assets/by-type'),
				json<{ assets: Record<string, number> }>('/metrics/assets/by-provider'),
				json<{ assets: Record<string, number> }>('/metrics/assets/by-owner')
			]);
			total = t.count;
			withSchemas = s.count;
			schemaShare = Math.round(s.percentage ?? 0);
			byType = ty.assets ?? {};
			byProvider = pr.assets ?? {};
			byOwner = ow.assets ?? {};
		} catch (e) {
			error = e instanceof Error ? e.message : m.metrics_load_error();
		} finally {
			loading = false;
		}
	}

	async function loadUsage() {
		usageLoading = true;
		const end = new Date();
		const start = new Date(
			end.getFullYear(),
			end.getMonth(),
			end.getDate() - (RANGE_DAYS[range] - 1)
		);
		const window = `start=${encodeURIComponent(start.toISOString())}&end=${encodeURIComponent(end.toISOString())}`;
		const filled = emptyDays(RANGE_DAYS[range]);
		try {
			const [assets, queries, series] = await Promise.all([
				json<AssetCount[]>(`/metrics/top-assets?${window}&limit=8`),
				json<QueryCount[]>(`/metrics/top-queries?${window}&limit=8`),
				json<{ metrics: SeriesPoint[] | null }>(
					`/metrics?${window}&bucket_size=1d&aggregation=sum&metric_names=asset_views_total&metric_names=search_queries_detailed`
				)
			]);
			topAssets = assets ?? [];
			topSearches = (queries ?? []).filter((q) => q.query);
			const byDay = new Map(filled.map((d) => [d.key, d]));
			for (const point of series.metrics ?? []) {
				const day = byDay.get(dayKey(new Date(point.bucket_start)));
				if (!day) continue;
				if (point.name === 'asset_views_total') day.views += point.value;
				else day.searches += point.value;
			}
		} catch {
			topAssets = [];
			topSearches = [];
		} finally {
			days = filled;
			usageLoading = false;
		}
	}

	function setRange(next: Range) {
		range = next;
		loadUsage();
	}

	onMount(() => {
		loadInventory();
		loadUsage();
	});
</script>

{#snippet header(title: string, icon: string)}
	<h3 class="flex items-center gap-2 text-sm font-semibold text-gray-900 dark:text-gray-100">
		<IconifyIcon {icon} class="h-4 w-4 text-gray-400" />
		{title}
	</h3>
{/snippet}

{#snippet ranked(items: Ranked[], empty: string, busy: boolean)}
	{@const max = Math.max(1, ...items.map((i) => i.value))}
	{#if busy}
		<div class="mt-4 space-y-3">
			{#each Array(5) as _, i (i)}
				<div class="h-6 animate-pulse rounded bg-gray-100 dark:bg-gray-700/60"></div>
			{/each}
		</div>
	{:else if items.length === 0}
		<p class="mt-6 text-sm text-gray-500 dark:text-gray-400">{empty}</p>
	{:else}
		<ul class="mt-3 space-y-2.5">
			{#each items as item (item.key)}
				<li title={item.label}>
					<div class="flex items-baseline justify-between gap-2 text-sm">
						<span class="flex min-w-0 items-center gap-1.5">
							{#if item.icon}
								<span class="flex-shrink-0 self-center">
									<Icon name={item.icon} size="xs" showLabel={false} />
								</span>
							{/if}
							{#if item.route}
								<a
									href={resolve('/discover/[type]/[service]/[name]', item.route)}
									class="truncate text-gray-800 hover:text-earthy-terracotta-700 hover:underline dark:text-gray-200 dark:hover:text-earthy-terracotta-400"
									>{item.label}</a
								>
							{:else}
								<span class="truncate text-gray-800 dark:text-gray-200">{item.label}</span>
							{/if}
						</span>
						<span class="tabular-nums text-gray-500 dark:text-gray-400"
							>{formatNumber(item.value)}</span
						>
					</div>
					<div class="mt-1 h-1.5 rounded-full bg-gray-100 dark:bg-gray-700/60">
						<div
							class="h-full rounded-full bg-earthy-blue-400 dark:bg-earthy-blue-500"
							style="width: {(item.value / max) * 100}%"
						></div>
					</div>
				</li>
			{/each}
		</ul>
	{/if}
{/snippet}

{#snippet donut(items: Ranked[], unit: string, busy: boolean)}
	{#if busy}
		<div class="mt-4 flex items-center gap-6">
			<div class="h-36 w-36 animate-pulse rounded-full bg-gray-100 dark:bg-gray-700/60"></div>
			<div class="flex-1 space-y-2">
				{#each Array(4) as _, i (i)}
					<div class="h-4 animate-pulse rounded bg-gray-100 dark:bg-gray-700/60"></div>
				{/each}
			</div>
		</div>
	{:else if items.length === 0}
		<p class="mt-6 text-sm text-gray-500 dark:text-gray-400">{m.metrics_no_data()}</p>
	{:else}
		<div class="mt-4"><DonutChart slices={toSlices(items)} {unit} /></div>
	{/if}
{/snippet}

<div class="space-y-5">
	<div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
		<p class="text-sm text-gray-600 dark:text-gray-400">{m.metrics_subheading()}</p>
		<div
			class="inline-flex self-start rounded-lg border border-gray-200 bg-white p-0.5 dark:border-gray-700 dark:bg-gray-800"
			role="group"
			aria-label={m.dataquery_range_label()}
		>
			{#each ranges as r (r.id)}
				<button
					type="button"
					onclick={() => setRange(r.id)}
					aria-pressed={range === r.id}
					class="rounded-md px-3 py-1 text-xs font-medium transition {range === r.id
						? 'bg-gray-900 text-white dark:bg-gray-100 dark:text-gray-900'
						: 'text-gray-600 hover:text-gray-900 dark:text-gray-400 dark:hover:text-gray-100'}"
					>{r.label}</button
				>
			{/each}
		</div>
	</div>

	{#if error}
		<div
			class="rounded-xl border border-red-200 bg-red-50 p-4 text-sm text-red-700 dark:border-red-900 dark:bg-red-900/20 dark:text-red-300"
		>
			{error}
		</div>
	{/if}

	<!-- KPI row -->
	<div class="grid grid-cols-2 gap-4 xl:grid-cols-4">
		<div class="{PANEL} p-4">
			<div class={KPI_LABEL}>{m.metrics_total_assets_title()}</div>
			<div class={KPI_VALUE}>{loading ? '–' : formatNumber(total)}</div>
			<div class={KPI_SUB}>
				{m.insights_kpi_across_types({ count: formatNumber(Object.keys(byType).length) })}
			</div>
			<div
				class="mt-3 flex h-2 overflow-hidden rounded-full bg-gray-100 dark:bg-gray-700/60"
				aria-hidden="true"
			>
				{#each types.slice(0, SLICE_COLOURS.length) as item, i (item.key)}
					<div
						style="width: {total
							? (item.value / total) * 100
							: 0}%; background-color: {SLICE_COLOURS[i]}"
						title="{item.label} · {formatNumber(item.value)}"
					></div>
				{/each}
			</div>
		</div>
		<div class="{PANEL} p-4">
			<div class={KPI_LABEL}>{m.metrics_with_schemas_title()}</div>
			<div class={KPI_VALUE}>
				{loading ? '–' : schemaShare}<span class="text-base text-gray-400">%</span>
			</div>
			<div class={KPI_SUB}>
				{m.insights_kpi_of_assets({ count: formatNumber(withSchemas), total: formatNumber(total) })}
			</div>
			<div
				class="mt-3 flex h-2 overflow-hidden rounded-full bg-gray-200 dark:bg-gray-700"
				aria-hidden="true"
			>
				<div class="bg-earthy-green-500" style="width: {total ? schemaShare : 0}%"></div>
			</div>
		</div>
		<div class="{PANEL} p-4">
			<div class={KPI_LABEL}>{m.insights_kpi_views()}</div>
			<div class={KPI_VALUE}>{usageLoading ? '–' : formatNumber(totalViews)}</div>
			<div class={KPI_SUB}>{m.insights_kpi_views_hint()}</div>
			<Sparkline points={days.map((d) => ({ label: d.label, value: d.views }))} tone="calm" />
		</div>
		<div class="{PANEL} p-4">
			<div class={KPI_LABEL}>{m.insights_kpi_searches()}</div>
			<div class={KPI_VALUE}>{usageLoading ? '–' : formatNumber(totalSearches)}</div>
			<div class={KPI_SUB}>{m.insights_kpi_searches_hint()}</div>
			<Sparkline points={days.map((d) => ({ label: d.label, value: d.searches }))} tone="calm" />
		</div>
	</div>

	<!-- Activity over time and what the catalog holds -->
	<div class="grid gap-5 lg:grid-cols-3">
		<div class="{PANEL} p-4 lg:col-span-2">
			<div class="flex items-center justify-between">
				{@render header(m.insights_activity_heading(), 'material-symbols:bar-chart')}
				<div class="flex items-center gap-3 text-xs text-gray-500 dark:text-gray-400">
					<span class="flex items-center gap-1.5">
						<span class="h-2.5 w-2.5 rounded-sm {ACTIVITY_SERIES[0].swatch}"></span>
						{ACTIVITY_SERIES[0].label}
					</span>
					<span class="flex items-center gap-1.5">
						<span class="h-2.5 w-2.5 rounded-sm {ACTIVITY_SERIES[1].swatch}"></span>
						{ACTIVITY_SERIES[1].label}
					</span>
				</div>
			</div>
			{#if usageLoading}
				<div class="mt-4 h-48 animate-pulse rounded bg-gray-100 dark:bg-gray-700/60"></div>
			{:else if totalViews + totalSearches === 0}
				<div class="flex h-48 flex-col items-center justify-center text-center">
					<IconifyIcon icon="material-symbols:bar-chart" class="h-8 w-8 text-gray-300" />
					<p class="mt-2 text-sm text-gray-500 dark:text-gray-400">{m.insights_activity_empty()}</p>
				</div>
			{:else}
				<div class="mt-4">
					<StackedColumns data={days} series={ACTIVITY_SERIES} />
				</div>
			{/if}
		</div>
		<div class="{PANEL} p-4">
			{@render header(m.metrics_asset_types_title(), 'material-symbols:category-outline')}
			{@render donut(types, m.insights_donut_assets(), loading)}
		</div>
	</div>

	<!-- Usage in the selected window -->
	<div class="grid gap-5 lg:grid-cols-2">
		<div class="{PANEL} p-4">
			{@render header(m.metrics_most_viewed_assets_title(), 'material-symbols:visibility-outline')}
			{@render ranked(viewed, m.metrics_no_asset_views_hint(), usageLoading)}
		</div>
		<div class="{PANEL} p-4">
			{@render header(m.metrics_top_search_queries_title(), 'material-symbols:search')}
			{@render ranked(searches, m.metrics_no_search_queries_hint(), usageLoading)}
		</div>
	</div>

	<!-- Where assets come from and who owns them -->
	<div class="grid gap-5 lg:grid-cols-2">
		<div class="{PANEL} p-4">
			{@render header(m.metrics_top_data_providers_title(), 'material-symbols:database-outline')}
			{@render donut(providers, m.insights_donut_assets(), loading)}
		</div>
		<div class="{PANEL} p-4">
			{@render header(m.metrics_top_asset_owners_title(), 'material-symbols:groups-outline')}
			{@render ranked(owners.slice(0, 8), m.metrics_no_data(), loading)}
		</div>
	</div>
</div>
