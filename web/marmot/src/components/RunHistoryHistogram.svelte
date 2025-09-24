<script lang="ts">
	import { fetchApi } from '$lib/api';
	import { onMount } from 'svelte';
	import IconifyIcon from '@iconify/svelte';
	import * as d3 from 'd3';

	export let assetId: string;
	export let period: string = '30d';
	export let minimal = false;

	interface HistogramBucket {
		date: string;
		total: number;
		complete: number;
		fail: number;
		running: number;
		abort: number;
		other: number;
	}

	interface HistogramResponse {
		buckets: HistogramBucket[];
		period: string;
	}

	let svgElement: SVGElement;
	let containerWidth = 800;
	let containerHeight = 100;
	let histogramData: HistogramBucket[] = [];
	let loading = true;
	let error: string | null = null;
	let lastAssetId = '';
	let lastPeriod = '';

	const margin = { top: 10, right: 10, bottom: 10, left: 10 };
	const width = containerWidth - margin.left - margin.right;
	const height = containerHeight - margin.top - margin.bottom;

	async function fetchHistogramData() {
		try {
			loading = true;
			error = null;

			const response = await fetchApi(`/assets/run-history-histogram/${assetId}?period=${period}`);

			if (!response.ok) {
				throw new Error('Failed to fetch histogram data');
			}

			const data: HistogramResponse = await response.json();
			histogramData = data.buckets;
		} catch (err) {
			console.error('Error fetching histogram data:', err);
			error = err instanceof Error ? err.message : 'Failed to load histogram data';
		} finally {
			loading = false;
		}
	}

	function createHistogram() {
		if (!svgElement || !histogramData.length || loading) return;

		d3.select(svgElement).selectAll('*').remove();

		const svg = d3.select(svgElement);
		const g = svg.append('g').attr('transform', `translate(${margin.left},${margin.top})`);

		const xScale = d3
			.scaleBand()
			.domain(histogramData.map((d) => d.date))
			.range([0, width])
			.padding(0.1);

		const yScale = d3
			.scaleLinear()
			.domain([0, d3.max(histogramData, (d) => d.total) || 10])
			.range([height, 0]);

		const stack = d3
			.stack()
			.keys(['complete', 'fail', 'running', 'abort', 'other'])
			.order(d3.stackOrderNone)
			.offset(d3.stackOffsetNone);

		const stackedData = stack(histogramData);

		const colorScale = d3
			.scaleOrdinal()
			.domain(['complete', 'fail', 'running', 'abort', 'other'])
			.range(['#10b981', '#ef4444', '#f59e0b', '#6b7280', '#8b5cf6']);

		g.selectAll('.bar-group')
			.data(stackedData)
			.enter()
			.append('g')
			.attr('class', 'bar-group')
			.attr('fill', (d) => colorScale(d.key))
			.selectAll('rect')
			.data((d) => d)
			.enter()
			.append('rect')
			.attr('x', (d) => xScale(d.data.date) || 0)
			.attr('y', (d) => yScale(d[1]))
			.attr('height', (d) => yScale(d[0]) - yScale(d[1]))
			.attr('width', xScale.bandwidth())
			.attr('rx', 2)
			.on('mouseover', function (event, d) {
				const tooltip = d3
					.select('body')
					.append('div')
					.attr('class', 'tooltip')
					.style('position', 'absolute')
					.style('background', 'rgba(0, 0, 0, 0.9)')
					.style('color', 'white')
					.style('padding', '8px 12px')
					.style('border-radius', '6px')
					.style('font-size', '12px')
					.style('pointer-events', 'none')
					.style('z-index', '1000')
					.style('box-shadow', '0 4px 6px -1px rgba(0, 0, 0, 0.1)');

				const date = new Date(d.data.date + 'T00:00:00');
				const formattedDate = date.toLocaleDateString('en-US', {
					month: 'short',
					day: 'numeric'
				});

				tooltip
					.html(
						`
					<div><strong>${formattedDate}</strong></div>
					<div>Total: ${d.data.total}</div>
					${d.data.complete ? `<div>✓ Complete: ${d.data.complete}</div>` : ''}
					${d.data.fail ? `<div>✗ Failed: ${d.data.fail}</div>` : ''}
					${d.data.running ? `<div>● Running: ${d.data.running}</div>` : ''}
					${d.data.abort ? `<div>⬜ Aborted: ${d.data.abort}</div>` : ''}
					${d.data.other ? `<div>○ Other: ${d.data.other}</div>` : ''}
				`
					)
					.style('left', event.pageX + 10 + 'px')
					.style('top', event.pageY - 10 + 'px');
			})
			.on('mouseout', function () {
				d3.selectAll('.tooltip').remove();
			});
	}

	onMount(() => {
		if (assetId) {
			lastAssetId = assetId;
			lastPeriod = period;
			fetchHistogramData();
		}
	});

	$: if ((assetId && assetId !== lastAssetId) || (period && period !== lastPeriod)) {
		lastAssetId = assetId;
		lastPeriod = period;
		fetchHistogramData();
	}

	$: if (svgElement && histogramData && histogramData.length && !loading) {
		createHistogram();
	}
</script>

<div class="w-full">
	<div class="flex items-center justify-between mb-2">
		<div class="text-sm text-gray-600 dark:text-gray-400">
			Last {period === '7d' ? '7 days' : period === '30d' ? '30 days' : '90 days'}
		</div>
		<div class="flex gap-1">
			<button
				onclick={() => (period = '7d')}
				class="px-2 py-1 text-xs rounded {period === '7d'
					? 'bg-orange-100 text-orange-800 dark:bg-orange-900/30 dark:text-orange-300'
					: 'bg-gray-100 text-gray-600 dark:bg-gray-700 dark:text-gray-300'}"
			>
				7d
			</button>
			<button
				onclick={() => (period = '30d')}
				class="px-2 py-1 text-xs rounded {period === '30d'
					? 'bg-orange-100 text-orange-800 dark:bg-orange-900/30 dark:text-orange-300'
					: 'bg-gray-100 text-gray-600 dark:bg-gray-700 dark:text-gray-300'}"
			>
				30d
			</button>
			<button
				onclick={() => (period = '90d')}
				class="px-2 py-1 text-xs rounded {period === '90d'
					? 'bg-orange-100 text-orange-800 dark:bg-orange-900/30 dark:text-orange-300'
					: 'bg-gray-100 text-gray-600 dark:bg-gray-700 dark:text-gray-300'}"
			>
				90d
			</button>
		</div>
	</div>

	{#if loading}
		<div class="flex items-center justify-center h-24">
			<div class="animate-spin rounded-full h-6 w-6 border-b-2 border-orange-600"></div>
		</div>
	{:else if error}
		<div
			class="bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800/50 rounded-lg p-4"
		>
			<p class="text-red-600 dark:text-red-400 text-sm">{error}</p>
		</div>
	{:else if histogramData.length === 0 || histogramData.every((d) => d.total === 0)}
		<div
			class="bg-gray-50 dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg p-6 text-center"
		>
			<div class="text-gray-600 dark:text-gray-400 mb-3">
				<IconifyIcon icon="mdi:chart-bar" class="w-12 h-12 mx-auto" />
			</div>
			<h4 class="text-lg font-medium text-gray-900 dark:text-gray-100 mb-1">No Run History</h4>
			<p class="text-sm text-gray-500 dark:text-gray-400">
				No runs found in the last {period === '7d'
					? '7 days'
					: period === '30d'
						? '30 days'
						: '90 days'}
			</p>
		</div>
	{:else}
		<div class="mb-4">
			<svg
				bind:this={svgElement}
				width={containerWidth}
				height={containerHeight}
				class="w-full h-auto"
				viewBox="0 0 {containerWidth} {containerHeight}"
			></svg>
		</div>
	{/if}
</div>

<style>
	:global(.tooltip) {
		pointer-events: none !important;
	}
</style>
