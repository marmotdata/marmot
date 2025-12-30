<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { fetchApi } from '$lib/api';
	import Button from '$components/ui/Button.svelte';
	import IconifyIcon from '@iconify/svelte';
	import TopMetricsTable from '$components/metrics/MetricsTable.svelte';
	import MetricCard from '$components/metrics/MetricCard.svelte';
	import PieChart from '$components/metrics/MetricPieChart.svelte';
	import BarChart from '$components/metrics/MetricBarChart.svelte';

	interface TimeRange {
		label: string;
		value: string;
		start: () => string;
		end: () => string;
	}

	interface AssetMetrics {
		totalAssets: number;
		assetsWithSchemas: number;
		schemasPercentage: number;
		assetsByType: Record<string, number>;
		assetsByProvider: Record<string, number>;
		assetsByOwner: Record<string, number>;
	}

	const timeRanges: TimeRange[] = [
		{
			label: 'Last 7 days',
			value: '7d',
			start: () => new Date(Date.now() - 7 * 24 * 60 * 60 * 1000).toISOString(),
			end: () => new Date().toISOString()
		},
		{
			label: 'Last 30 days',
			value: '30d',
			start: () => new Date(Date.now() - 30 * 24 * 60 * 60 * 1000).toISOString(),
			end: () => new Date().toISOString()
		},
		{
			label: 'Last 90 days',
			value: '90d',
			start: () => new Date(Date.now() - 90 * 24 * 60 * 60 * 1000).toISOString(),
			end: () => new Date().toISOString()
		},
		{
			label: 'Last 12 months',
			value: '12m',
			start: () => new Date(Date.now() - 365 * 24 * 60 * 60 * 1000).toISOString(),
			end: () => new Date().toISOString()
		}
	];

	let selectedTimeRange = timeRanges[3];
	let showTimeRangeDropdown = false;

	let assetMetrics: AssetMetrics = {
		totalAssets: 0,
		assetsWithSchemas: 0,
		assetsByType: {},
		assetsByProvider: {},
		assetsByOwner: {}
	};

	let metricsLoading = true;
	let metricsError: string | null = null;

	async function fetchAssetMetrics() {
		try {
			metricsLoading = true;
			metricsError = null;

			const [totalRes, schemasRes, typeRes, providerRes, ownerRes] = await Promise.all([
				fetchApi('/metrics/assets/total'),
				fetchApi('/metrics/assets/with-schemas'),
				fetchApi('/metrics/assets/by-type'),
				fetchApi('/metrics/assets/by-provider'),
				fetchApi('/metrics/assets/by-owner')
			]);

			const [total, schemas, type, provider, owner] = await Promise.all([
				totalRes.json(),
				schemasRes.json(),
				typeRes.json(),
				providerRes.json(),
				ownerRes.json()
			]);

			assetMetrics = {
				totalAssets: total.count,
				assetsWithSchemas: schemas.count,
				schemasPercentage: schemas.percentage,
				assetsByType: type.assets,
				assetsByProvider: provider.assets,
				assetsByOwner: owner.assets
			};
		} catch (err) {
			console.error('Error fetching asset metrics:', err);
			metricsError = err instanceof Error ? err.message : 'Failed to load metrics';
		} finally {
			metricsLoading = false;
		}
	}

	function transformQueriesData(rawData: any[]) {
		return rawData.map((item) => ({
			id: item.query || '',
			name: item.query || '',
			count: item.count,
			clickable: false
		}));
	}

	function transformAssetsData(rawData: any[]) {
		return rawData.map((item) => ({
			id: item.asset_id || '',
			name: item.asset_name || item.asset_id || '',
			count: item.count,
			icon: item.asset_provider,
			clickable: true,
			asset_type: item.asset_type,
			asset_name: item.asset_name
		}));
	}

	function handleAssetClick(item: any) {
		if (item.asset_type && item.asset_name && item.asset_provider) {
			goto(
				`/discover/${encodeURIComponent(item.asset_type)}/${encodeURIComponent(item.asset_provider)}/${encodeURIComponent(item.asset_name)}`
			);
		}
	}

	function handleTimeRangeChange(timeRange: TimeRange) {
		selectedTimeRange = timeRange;
		showTimeRangeDropdown = false;
	}

	function handleClickOutside(event: MouseEvent) {
		const target = event.target as Element;
		if (!target.closest('.time-range-dropdown')) {
			showTimeRangeDropdown = false;
		}
	}

	onMount(() => {
		document.addEventListener('click', handleClickOutside);
		fetchAssetMetrics();

		return () => {
			document.removeEventListener('click', handleClickOutside);
		};
	});

	$: startDate = selectedTimeRange.start();
	$: endDate = selectedTimeRange.end();

	$: typeChartData = Object.entries(assetMetrics.assetsByType).map(([label, value]) => ({
		label,
		value
	}));

	$: providerChartData = Object.entries(assetMetrics.assetsByProvider).map(([label, value]) => ({
		label,
		value
	}));

	$: ownerChartData = Object.entries(assetMetrics.assetsByOwner).map(([label, value]) => ({
		label,
		value
	}));
</script>

<div class="container max-w-7xl mx-auto py-6 px-4 sm:px-6 lg:px-8">
	<div class="flex justify-between items-center mb-8">
		<div>
			<h1 class="text-2xl font-bold text-gray-900 dark:text-gray-100">Metrics</h1>
			<p class="text-gray-600 dark:text-gray-400 mt-1">
				Analytics and insights for your data platform
			</p>
		</div>

		<div class="relative time-range-dropdown">
			<Button
				variant="secondary"
				class="flex items-center gap-2"
				click={() => (showTimeRangeDropdown = !showTimeRangeDropdown)}
			>
				<IconifyIcon icon="mdi:calendar-range" class="w-4 h-4" />
				{selectedTimeRange.label}
				<IconifyIcon
					icon="mdi:chevron-down"
					class="w-4 h-4 transition-transform {showTimeRangeDropdown ? 'rotate-180' : ''}"
				/>
			</Button>

			{#if showTimeRangeDropdown}
				<div
					class="absolute right-0 mt-2 w-48 bg-white dark:bg-gray-800 rounded-lg shadow-lg border border-gray-200 dark:border-gray-700 z-50"
				>
					<div class="py-1">
						{#each timeRanges as timeRange}
							<button
								class="w-full px-4 py-2 text-left text-sm hover:bg-gray-50 dark:hover:bg-gray-700 flex items-center justify-between {selectedTimeRange.value ===
								timeRange.value
									? 'bg-earthy-terracotta-50 dark:bg-earthy-terracotta-900/20 text-earthy-terracotta-700 dark:text-earthy-terracotta-700'
									: 'text-gray-700 dark:text-gray-300'}"
								on:click={() => handleTimeRangeChange(timeRange)}
							>
								{timeRange.label}
								{#if selectedTimeRange.value === timeRange.value}
									<IconifyIcon icon="mdi:check" class="w-4 h-4" />
								{/if}
							</button>
						{/each}
					</div>
				</div>
			{/if}
		</div>
	</div>

	<!-- Asset Overview Cards -->
	<div class="grid grid-cols-1 md:grid-cols-4 gap-4 mb-6">
		<MetricCard
			title="Total Assets"
			value={assetMetrics.totalAssets}
			icon="mdi:database"
			loading={metricsLoading}
			error={metricsError}
		/>

		<MetricCard
			title="With Schemas"
			value={assetMetrics.assetsWithSchemas}
			icon="mdi:database-check"
			loading={metricsLoading}
			error={metricsError}
			subtitle="{Math.round(assetMetrics.schemasPercentage)}% coverage"
		/>

		<MetricCard
			title="Asset Types"
			value={Object.keys(assetMetrics.assetsByType).length}
			icon="mdi:shape"
			loading={metricsLoading}
			error={metricsError}
		/>

		<MetricCard
			title="Data Sources"
			value={Object.keys(assetMetrics.assetsByProvider).length}
			icon="mdi:database-sync"
			loading={metricsLoading}
			error={metricsError}
		/>
	</div>

	<!-- Mixed Grid Layout -->
	<div class="grid grid-cols-1 lg:grid-cols-3 gap-6 mb-6 items-start">
		<!-- Asset Types Pie Chart -->
		<div class="lg:col-span-1">
			<PieChart
				data={typeChartData}
				title="Asset Types"
				icon="mdi:shape-outline"
				loading={metricsLoading}
				error={metricsError}
				width={300}
				height={280}
			/>
		</div>

		<!-- Top Search Queries Table -->
		<div class="lg:col-span-2">
			<TopMetricsTable
				{startDate}
				{endDate}
				timeRangeLabel={selectedTimeRange.label}
				endpoint="/metrics/top-assets"
				title="Most Viewed Assets"
				icon="mdi:eye"
				emptyIcon="mdi:database-outline"
				emptyMessage="No asset views found"
				emptyDescription="Data will appear here once users start viewing assets"
				countLabel="views"
				limit={8}
				transformData={transformAssetsData}
				onItemClick={handleAssetClick}
			/>
		</div>
	</div>

	<!--  Bar Charts -->
	<div class="grid grid-cols-1 lg:grid-cols-2 gap-6 mb-6">
		<BarChart
			data={providerChartData}
			title="Top Data Providers"
			icon="mdi:database-sync-outline"
			loading={metricsLoading}
			error={metricsError}
			width={450}
			height={220}
			limit={5}
			showIcons={true}
		/>

		<BarChart
			data={ownerChartData}
			title="Top Asset Owners"
			icon="mdi:account-group-outline"
			loading={metricsLoading}
			error={metricsError}
			width={450}
			height={220}
			limit={5}
			showIcons={false}
		/>
	</div>

	<div>
		<TopMetricsTable
			{startDate}
			{endDate}
			timeRangeLabel={selectedTimeRange.label}
			endpoint="/metrics/top-queries"
			title="Top Search Queries"
			icon="mdi:magnify"
			emptyIcon="mdi:database-search-outline"
			emptyMessage="No search queries found"
			emptyDescription="Data will appear here once users start searching"
			countLabel="searches"
			limit={5}
			transformData={transformQueriesData}
		/>
	</div>
</div>

<style>
	.time-range-dropdown {
		position: relative;
	}
</style>
