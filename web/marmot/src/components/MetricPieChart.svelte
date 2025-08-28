<script lang="ts">
	import { onMount } from 'svelte';
	import * as d3 from 'd3';
	import IconifyIcon from '@iconify/svelte';

	interface ChartData {
		label: string;
		value: number;
		color?: string;
	}

	export let data: ChartData[] = [];
	export let title: string;
	export let icon: string;
	export let loading: boolean = false;
	export let error: string | null = null;
	export let width: number = 300;
	export let height: number = 300;

	let svgElement: SVGElement;
	let tooltipElement: HTMLDivElement;

	const margin = { top: 20, right: 20, bottom: 20, left: 20 };
	const radius = Math.min(width, height) / 2 - Math.max(margin.top, margin.right);

	const colors = d3.scaleOrdinal(['#3b82f6', '#8b5cf6', '#ec4899', '#f59e0b', '#10b981']);

	function createChart() {
		if (!svgElement || !data.length || loading) return;

		const limitedData = data.slice(0, 5);

		d3.select(svgElement).selectAll('*').remove();

		const svg = d3.select(svgElement);
		const g = svg.append('g').attr('transform', `translate(${width / 2}, ${height / 2})`);

		const pie = d3
			.pie<ChartData>()
			.value((d) => d.value)
			.sort(null);

		const arc = d3.arc<d3.PieArcDatum<ChartData>>().innerRadius(0).outerRadius(radius);

		const arcs = g
			.selectAll('.arc')
			.data(pie(limitedData))
			.enter()
			.append('g')
			.attr('class', 'arc');

		arcs
			.append('path')
			.attr('d', arc)
			.attr('fill', (d, i) => d.data.color || colors(i.toString()))
			.attr('stroke', 'white')
			.attr('stroke-width', 2)
			.style('cursor', 'pointer')
			.on('mouseover', function (event, d) {
				d3.select(this).attr('opacity', 0.8);
				showTooltip(event, d.data);
			})
			.on('mousemove', function (event, d) {
				showTooltip(event, d.data);
			})
			.on('mouseout', function () {
				d3.select(this).attr('opacity', 1);
				hideTooltip();
			});

		arcs
			.append('text')
			.attr('transform', (d) => `translate(${arc.centroid(d)})`)
			.attr('text-anchor', 'middle')
			.attr('font-size', '12px')
			.attr('fill', 'white')
			.attr('font-weight', 'bold')
			.style('pointer-events', 'none')
			.text((d) => (d.data.value > 0 ? d.data.value : ''));
	}

	function showTooltip(event: MouseEvent, data: ChartData) {
		if (!tooltipElement) return;

		tooltipElement.style.opacity = '1';
		tooltipElement.style.left = `${event.pageX + 10}px`;
		tooltipElement.style.top = `${event.pageY - 10}px`;
		tooltipElement.innerHTML = `
			<div class="font-semibold">${data.label}</div>
			<div>Count: ${data.value.toLocaleString()}</div>
		`;
	}

	function hideTooltip() {
		if (!tooltipElement) return;
		tooltipElement.style.opacity = '0';
	}

	onMount(() => {
		createChart();
	});

	$: if (svgElement && data && !loading) {
		createChart();
	}
</script>

<div
	class="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-4 h-full"
>
	<div class="flex items-center gap-2 mb-4">
		<IconifyIcon {icon} class="w-5 h-5 text-orange-600 dark:text-orange-400" />
		<h3 class="text-lg font-semibold text-gray-900 dark:text-gray-100">{title}</h3>
	</div>

	{#if loading}
		<div class="flex items-center justify-center" style="height: {height}px">
			<div class="animate-spin rounded-full h-8 w-8 border-b-2 border-orange-600"></div>
		</div>
	{:else if error}
		<div class="flex items-center justify-center" style="height: {height}px">
			<div class="text-center">
				<IconifyIcon icon="mdi:alert-circle" class="w-8 h-8 text-red-500 mx-auto mb-2" />
				<p class="text-red-600 dark:text-red-400 text-sm">{error}</p>
			</div>
		</div>
	{:else if !data.length}
		<div class="flex items-center justify-center" style="height: {height}px">
			<div class="text-center">
				<IconifyIcon icon="mdi:chart-pie" class="w-8 h-8 text-gray-400 mx-auto mb-2" />
				<p class="text-gray-500 dark:text-gray-400 text-sm">No data available</p>
			</div>
		</div>
	{:else}
		<svg
			bind:this={svgElement}
			{width}
			{height}
			viewBox="0 0 {width} {height}"
			class="w-full h-auto"
		></svg>
	{/if}
</div>

<div
	bind:this={tooltipElement}
	class="absolute bg-gray-900 text-white text-xs rounded px-2 py-1 pointer-events-none opacity-0 transition-opacity z-10"
	style="transition: opacity 0.2s"
></div>
