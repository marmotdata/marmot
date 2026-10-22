<script lang="ts">
	import * as d3 from 'd3';
	import { formatNumber } from '$lib/utils/format';
	import { m } from '$lib/paraglide/messages';

	export interface DonutSlice {
		key: string;
		label: string;
		value: number;
		colour: string;
	}

	let {
		slices,
		unit,
		size = 144
	}: { slices: DonutSlice[]; unit: string; size?: number } = $props();

	let hovered = $state<string | null>(null);

	let total = $derived(d3.sum(slices, (s) => s.value));
	let radius = $derived(size / 2);
	let arc = $derived(
		d3
			.arc<d3.PieArcDatum<DonutSlice>>()
			.innerRadius(radius * 0.64)
			.outerRadius(radius)
			.cornerRadius(1)
	);
	let hoverArc = $derived(
		d3
			.arc<d3.PieArcDatum<DonutSlice>>()
			.innerRadius(radius * 0.64)
			.outerRadius(radius + 3)
			.cornerRadius(1)
	);
	// Slices keep the order they are given in, so colour and legend position always agree
	let arcs = $derived(
		d3
			.pie<DonutSlice>()
			.sort(null)
			.value((s) => s.value)(slices)
	);
	let focus = $derived(slices.find((s) => s.key === hovered));
</script>

<div class="flex flex-col items-center gap-5 sm:flex-row">
	<div class="relative flex-shrink-0" style="width: {size + 6}px; height: {size + 6}px">
		<svg
			width={size + 6}
			height={size + 6}
			viewBox="{-radius - 3} {-radius - 3} {size + 6} {size + 6}"
			role="img"
			aria-label={unit}
		>
			{#each arcs as a (a.data.key)}
				<path
					d={(hovered === a.data.key ? hoverArc : arc)(a) ?? ''}
					fill={a.data.colour}
					class="cursor-default transition-opacity {hovered && hovered !== a.data.key
						? 'opacity-40'
						: ''}"
					aria-hidden="true"
					onmouseenter={() => (hovered = a.data.key)}
					onmouseleave={() => (hovered = null)}
				/>
			{/each}
		</svg>
		<div
			class="pointer-events-none absolute inset-0 flex flex-col items-center justify-center px-6 text-center"
		>
			<span class="text-xl font-semibold tabular-nums text-gray-900 dark:text-gray-100"
				>{formatNumber(focus ? focus.value : total)}</span
			>
			<span class="max-w-full truncate text-[11px] text-gray-500 dark:text-gray-400"
				>{focus ? focus.label : unit}</span
			>
		</div>
	</div>
	<ul class="w-full min-w-0 flex-1 space-y-1.5">
		{#each slices as slice (slice.key)}
			{@const percent = total ? Math.round((slice.value / total) * 100) : 0}
			<li>
				<button
					type="button"
					class="flex w-full items-center justify-between gap-3 rounded px-1 text-sm transition-colors {hovered ===
					slice.key
						? 'bg-gray-50 dark:bg-gray-700/50'
						: ''}"
					onmouseenter={() => (hovered = slice.key)}
					onmouseleave={() => (hovered = null)}
					onfocus={() => (hovered = slice.key)}
					onblur={() => (hovered = null)}
					aria-label={m.charts_donut_slice_aria({
						label: slice.label,
						value: formatNumber(slice.value),
						percent
					})}
				>
					<span class="flex min-w-0 items-center gap-2">
						<span
							class="h-2.5 w-2.5 flex-shrink-0 rounded-sm"
							style="background-color: {slice.colour}"
						></span>
						<span class="truncate text-gray-800 dark:text-gray-200">{slice.label}</span>
					</span>
					<span class="whitespace-nowrap tabular-nums text-gray-500 dark:text-gray-400">
						{formatNumber(slice.value)}
						<span class="ml-1 text-xs text-gray-400">{percent}%</span>
					</span>
				</button>
			</li>
		{/each}
	</ul>
</div>
