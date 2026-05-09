<script lang="ts">
	import * as d3 from 'd3';

	export interface Bucket {
		hour: Date;
		success: number;
		error: number;
	}

	let { buckets, height = 120 }: { buckets: Bucket[]; height?: number } = $props();

	let containerEl: HTMLDivElement | undefined = $state();
	let width = $state(0);
	let svgEl: SVGSVGElement | undefined = $state();

	$effect(() => {
		if (!containerEl) return;
		const ro = new ResizeObserver((entries) => {
			for (const entry of entries) {
				width = Math.floor(entry.contentRect.width);
			}
		});
		ro.observe(containerEl);
		return () => ro.disconnect();
	});

	$effect(() => {
		if (!svgEl || width === 0 || buckets.length === 0) return;
		render();
	});

	function render() {
		if (!svgEl) return;
		const svg = d3.select(svgEl);
		svg.selectAll('*').remove();

		const margin = { top: 6, right: 8, bottom: 22, left: 28 };
		const innerW = width - margin.left - margin.right;
		const innerH = height - margin.top - margin.bottom;

		const g = svg.append('g').attr('transform', `translate(${margin.left},${margin.top})`);

		const x = d3
			.scaleBand<Date>()
			.domain(buckets.map((b) => b.hour))
			.range([0, innerW])
			.padding(0.25);

		const maxCount = d3.max(buckets, (b) => b.success + b.error) || 1;
		const y = d3.scaleLinear().domain([0, maxCount]).nice().range([innerH, 0]);

		g.append('g')
			.call(d3.axisLeft(y).ticks(3).tickSize(-innerW))
			.call((sel) => sel.select('.domain').remove())
			.call((sel) =>
				sel
					.selectAll('line')
					.attr('stroke', 'currentColor')
					.attr('stroke-opacity', 0.08)
					.attr('stroke-dasharray', '2,2')
			)
			.call((sel) =>
				sel
					.selectAll('text')
					.attr('fill', 'currentColor')
					.attr('opacity', 0.45)
					.attr('font-size', 10)
			);

		const tickValues = buckets.filter((_, i) => i % 4 === 0).map((b) => b.hour);
		g.append('g')
			.attr('transform', `translate(0,${innerH})`)
			.call(
				d3
					.axisBottom(x)
					.tickValues(tickValues)
					.tickFormat((d) => d3.timeFormat('%H:%M')(d as Date))
					.tickSize(0)
					.tickPadding(8)
			)
			.call((sel) => sel.select('.domain').remove())
			.call((sel) =>
				sel
					.selectAll('text')
					.attr('fill', 'currentColor')
					.attr('opacity', 0.45)
					.attr('font-size', 10)
			);

		const groups = g
			.selectAll('g.bar')
			.data(buckets)
			.enter()
			.append('g')
			.attr('class', 'bar')
			.attr('transform', (b) => `translate(${x(b.hour) || 0},0)`);

		groups
			.append('rect')
			.attr('y', (b) => y(b.success + b.error))
			.attr('height', (b) => Math.max(0, y(0) - y(b.success + b.error)))
			.attr('width', x.bandwidth())
			.attr('fill', '#dc2626')
			.attr('rx', 2);

		groups
			.append('rect')
			.attr('y', (b) => y(b.success))
			.attr('height', (b) => Math.max(0, y(0) - y(b.success)))
			.attr('width', x.bandwidth())
			.attr('fill', '#607b60')
			.attr('rx', 2);

		groups
			.append('title')
			.text((b) => `${d3.timeFormat('%H:%M')(b.hour)} — ${b.success} success, ${b.error} error`);
	}
</script>

<div
	bind:this={containerEl}
	class="w-full text-gray-700 dark:text-gray-300"
	style="height: {height}px;"
>
	{#if width > 0}
		<svg bind:this={svgEl} {width} {height}></svg>
	{/if}
</div>
