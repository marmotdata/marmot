<script lang="ts">
	import IconifyIcon from '@iconify/svelte';
	import Icon from '$components/ui/Icon.svelte';
	import { m } from '$lib/paraglide/messages';

	// A picture of what Marmot does on a blank instance: sources flow in on the left, people and agents read through it on the right, so the page explains itself without paragraphs
	const sources = ['PostgreSQL', 'Snowflake', 'Kafka', 'dbt'];
	const consumers = [
		{ icon: 'material-symbols:smart-toy-outline', label: m.home_flow_agents(), via: 'MCP' },
		{ icon: 'material-symbols:terminal', label: m.home_flow_engineers(), via: 'SQL · CLI' },
		{ icon: 'material-symbols:api', label: m.home_flow_apps(), via: 'API' }
	];
	const layers = [
		{ icon: 'material-symbols:hub-outline', label: m.home_pillar_context_title() },
		{ icon: 'material-symbols:shield-outline', label: m.home_pillar_governance_title() },
		{ icon: 'material-symbols:key-outline', label: m.home_pillar_access_title() }
	];

	// Rows sit at even fractions of the height so the connector paths can be drawn in a 100 by 100 box
	const rowY = (i: number, n: number) => ((i + 0.5) / n) * 100;
</script>

<div class="relative h-64 w-full select-none" aria-hidden="true">
	<svg
		class="absolute inset-0 h-full w-full"
		viewBox="0 0 100 100"
		preserveAspectRatio="none"
		fill="none"
	>
		{#each sources as _, i (i)}
			<path
				d="M 27 {rowY(i, sources.length)} C 33 {rowY(i, sources.length)}, 33 50, 39 50"
				class="flow-line stroke-gray-300 dark:stroke-gray-600"
				vector-effect="non-scaling-stroke"
			/>
		{/each}
		{#each consumers as _, i (i)}
			<path
				d="M 61 50 C 67 50, 67 {rowY(i, consumers.length)}, 73 {rowY(i, consumers.length)}"
				class="flow-line stroke-earthy-terracotta-300 dark:stroke-earthy-terracotta-700"
				vector-effect="non-scaling-stroke"
			/>
		{/each}
	</svg>

	<!-- Sources -->
	<ul class="absolute inset-y-0 left-0 flex w-[27%] flex-col justify-around">
		{#each sources as name (name)}
			<li
				class="flex items-center gap-2 rounded-lg border border-gray-200 bg-white px-2.5 py-1.5 shadow-sm dark:border-gray-700 dark:bg-gray-800"
			>
				<Icon {name} size="xs" showLabel={false} />
				<span class="truncate text-xs font-medium text-gray-700 dark:text-gray-200">{name}</span>
			</li>
		{/each}
	</ul>

	<!-- Marmot -->
	<div class="absolute inset-y-0 left-[39%] flex w-[22%] items-center">
		<div
			class="w-full rounded-xl border border-gray-900/10 bg-gray-900 p-3 text-white shadow-lg dark:border-white/10 dark:bg-gray-950"
		>
			<div class="flex items-center gap-2">
				<img src="/images/marmot.svg" alt="" class="h-6 w-6" />
				<span class="text-sm font-semibold">Marmot</span>
			</div>
			<ul class="mt-3 space-y-1.5">
				{#each layers as layer (layer.label)}
					<li class="flex items-center gap-1.5 rounded-md bg-white/10 px-2 py-1 text-[11px]">
						<IconifyIcon icon={layer.icon} class="h-3.5 w-3.5 text-earthy-terracotta-300" />
						{layer.label}
					</li>
				{/each}
			</ul>
		</div>
	</div>

	<!-- People and agents -->
	<ul class="absolute inset-y-0 right-0 flex w-[27%] flex-col justify-around">
		{#each consumers as c (c.label)}
			<li
				class="flex items-center gap-2 rounded-lg border border-gray-200 bg-white px-2.5 py-1.5 shadow-sm dark:border-gray-700 dark:bg-gray-800"
			>
				<IconifyIcon icon={c.icon} class="h-4 w-4 flex-shrink-0 text-earthy-terracotta-600" />
				<span class="min-w-0">
					<span class="block truncate text-xs font-medium text-gray-700 dark:text-gray-200"
						>{c.label}</span
					>
					<span class="block truncate text-[10px] text-gray-400">{c.via}</span>
				</span>
			</li>
		{/each}
	</ul>
</div>

<style>
	.flow-line {
		stroke-width: 1.5;
		stroke-dasharray: 4 4;
		animation: flow 1.2s linear infinite;
	}

	@keyframes flow {
		to {
			stroke-dashoffset: -8;
		}
	}

	@media (prefers-reduced-motion: reduce) {
		.flow-line {
			animation: none;
		}
	}
</style>
