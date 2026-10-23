<script lang="ts">
	import * as d3 from 'd3';
	import { formatNumber } from '$lib/utils/format';
	import { m } from '$lib/paraglide/messages';

	export interface SparkPoint {
		label: string;
		value: number;
	}

	let {
		points,
		tone = 'neutral',
		height = 32
	}: { points: SparkPoint[]; tone?: 'neutral' | 'critical' | 'calm'; height?: number } = $props();

	let width = $state(0);
	let hovered = $state<number | null>(null);

	let x = $derived(
		d3
			.scaleBand<number>()
			.domain(points.map((_, i) => i))
			.range([0, width])
			.paddingInner(points.length > 20 ? 0.15 : 0.25)
	);
	let y = $derived(
		d3
			.scaleLinear()
			.domain([0, d3.max(points, (p) => p.value) || 1])
			.range([0, height])
	);
	// Empty buckets keep a hairline so the time axis still reads, and small values stay visible against the largest
	let bars = $derived(
		points.map((p, i) => {
			const h = p.value === 0 ? 2 : Math.max(height * 0.1, y(p.value));
			return { x: x(i) ?? 0, w: x.bandwidth(), h, y: height - h, empty: p.value === 0 };
		})
	);
	let fill = $derived(
		tone === 'critical'
			? 'fill-red-400 dark:fill-red-400/80'
			: tone === 'calm'
				? 'fill-earthy-blue-500 dark:fill-earthy-blue-400'
				: 'fill-earthy-terracotta-400 dark:fill-earthy-terracotta-500/80'
	);
</script>

<div class="relative mt-3" style="height: {height}px" bind:clientWidth={width}>
	{#if width > 0}
		<svg {width} {height} class="block">
			{#each bars as bar, i (i)}
				<rect
					x={bar.x}
					y={bar.y}
					width={bar.w}
					height={bar.h}
					rx={Math.min(2, bar.w / 2)}
					aria-hidden="true"
					class="{bar.empty
						? 'fill-gray-100 dark:fill-gray-700/60'
						: fill} transition-opacity {hovered !== null && hovered !== i ? 'opacity-50' : ''}"
				/>
				<rect
					x={bar.x}
					y={0}
					width={bar.w}
					{height}
					fill="transparent"
					role="button"
					tabindex="0"
					aria-label={m.charts_sparkline_point_aria({
						label: points[i].label,
						value: formatNumber(points[i].value)
					})}
					onmouseenter={() => (hovered = i)}
					onmouseleave={() => (hovered = null)}
					onfocus={() => (hovered = i)}
					onblur={() => (hovered = null)}
				/>
			{/each}
		</svg>
		{#if hovered !== null && points[hovered]}
			<span
				class="pointer-events-none absolute bottom-full z-10 mb-1 -translate-x-1/2 whitespace-nowrap rounded bg-gray-900 px-1.5 py-0.5 text-[10px] text-white"
				style="left: {bars[hovered].x + bars[hovered].w / 2}px"
				>{points[hovered].label} · {formatNumber(points[hovered].value)}</span
			>
		{/if}
	{/if}
</div>
