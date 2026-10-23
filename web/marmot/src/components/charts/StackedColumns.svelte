<script lang="ts" module>
	export interface ColumnSeries<S extends string> {
		key: S;
		label: string;
		fill: string;
	}
</script>

<script lang="ts" generics="K extends string">
	import * as d3 from 'd3';
	import { formatNumber } from '$lib/utils/format';

	let {
		data,
		series,
		height = 192
	}: {
		data: ({ key: string; label: string } & Record<K, number>)[];
		series: ColumnSeries<K>[];
		height?: number;
	} = $props();

	const margin = { top: 8, right: 4, bottom: 22, left: 30 };

	let width = $state(0);
	let hovered = $state<number | null>(null);

	let innerW = $derived(Math.max(0, width - margin.left - margin.right));
	let innerH = $derived(height - margin.top - margin.bottom);
	let x = $derived(
		d3
			.scaleBand<number>()
			.domain(data.map((_, i) => i))
			.range([0, innerW])
			.paddingInner(data.length > 20 ? 0.2 : 0.3)
	);
	let max = $derived(d3.max(data, (d) => d3.sum(series, (s) => d[s.key])) || 1);
	let y = $derived(d3.scaleLinear().domain([0, max]).nice(3).range([innerH, 0]));
	let ticks = $derived(y.ticks(3));
	// The first series sits on the baseline; each later one stacks on top, with only the top segment rounded
	let layers = $derived(
		d3
			.stack<(typeof data)[number], K>()
			.keys(series.map((s) => s.key))(data)
			.map((layer, li) => ({ series: series[li], points: layer }))
	);
	let labelIndexes = $derived(
		data.length <= 1 ? [0] : [0, Math.floor((data.length - 1) / 2), data.length - 1]
	);
	let tooltipX = $derived(
		hovered === null ? 0 : margin.left + (x(hovered) ?? 0) + x.bandwidth() / 2
	);

	function topSeriesIndex(i: number): number {
		for (let li = layers.length - 1; li >= 0; li--) {
			const [lo, hi] = layers[li].points[i];
			if (hi > lo) return li;
		}
		return -1;
	}
</script>

<div class="relative" style="height: {height}px" bind:clientWidth={width}>
	{#if width > 0}
		<svg {width} {height} class="block overflow-visible text-gray-400">
			<g transform="translate({margin.left},{margin.top})">
				{#each ticks as t (t)}
					<line
						x1={0}
						x2={innerW}
						y1={y(t)}
						y2={y(t)}
						stroke="currentColor"
						stroke-opacity={t === 0 ? 0.25 : 0.12}
						stroke-dasharray={t === 0 ? undefined : '2,3'}
					/>
					<text
						x={-8}
						y={y(t)}
						dy="0.32em"
						text-anchor="end"
						font-size="10"
						fill="currentColor"
						class="tabular-nums">{formatNumber(t)}</text
					>
				{/each}
				{#if hovered !== null}
					<rect
						x={(x(hovered) ?? 0) - 2}
						y={0}
						width={x.bandwidth() + 4}
						height={innerH}
						rx={3}
						class="fill-gray-100 dark:fill-gray-700/50"
					/>
				{/if}
				{#each layers as layer, li (layer.series.key)}
					{#each layer.points as p, i (i)}
						{@const h = y(p[0]) - y(p[1])}
						{#if h > 0}
							{@const rounded = topSeriesIndex(i) === li}
							{@const r = Math.min(3, x.bandwidth() / 2, h)}
							{@const bx = x(i) ?? 0}
							{@const bw = x.bandwidth()}
							{@const by = y(p[1])}
							<path
								d={rounded
									? `M${bx},${by + h}V${by + r}Q${bx},${by} ${bx + r},${by}H${bx + bw - r}Q${bx + bw},${by} ${bx + bw},${by + r}V${by + h}Z`
									: `M${bx},${by}h${bw}v${h}h${-bw}Z`}
								class={layer.series.fill}
							/>
						{/if}
					{/each}
				{/each}
				{#each data as _, i (i)}
					<rect
						x={x(i) ?? 0}
						y={0}
						width={x.bandwidth()}
						height={innerH}
						fill="transparent"
						role="presentation"
						onmouseenter={() => (hovered = i)}
						onmouseleave={() => (hovered = null)}
					/>
				{/each}
				{#each labelIndexes as i (i)}
					<text
						x={(x(i) ?? 0) + x.bandwidth() / 2}
						y={innerH + 15}
						text-anchor={i === 0 ? 'start' : i === data.length - 1 ? 'end' : 'middle'}
						font-size="10"
						fill="currentColor">{data[i]?.label}</text
					>
				{/each}
			</g>
		</svg>
		{#if hovered !== null && data[hovered]}
			<div
				class="pointer-events-none absolute z-10 -translate-x-1/2 -translate-y-full whitespace-nowrap rounded bg-gray-900 px-2 py-1 text-[11px] text-white shadow"
				style="left: {tooltipX}px; top: {margin.top}px"
			>
				<div class="font-medium">{data[hovered].label}</div>
				{#each [...series].reverse() as s (s.key)}
					<div>{s.label} · {formatNumber(data[hovered][s.key])}</div>
				{/each}
			</div>
		{/if}
	{/if}
</div>
