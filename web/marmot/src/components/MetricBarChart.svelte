<script lang="ts">
	import IconifyIcon from '@iconify/svelte';
	import Icon from './Icon.svelte';

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
	export let limit: number = 5;
	export let showIcons: boolean = true;

	const tailwindColors = ['#3b82f6', '#8b5cf6', '#ec4899', '#f59e0b', '#10b981'];

	$: sortedData = [...data].sort((a, b) => b.value - a.value);
	$: limitedData = sortedData.slice(0, limit);
	$: maxValue = Math.max(...limitedData.map((d) => d.value)) || 1;
</script>

<div
	class="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-4 h-full"
>
	<div class="flex items-center gap-2 mb-4">
		<IconifyIcon {icon} class="w-5 h-5 text-orange-600 dark:text-orange-400" />
		<h3 class="text-lg font-semibold text-gray-900 dark:text-gray-100">{title}</h3>
		{#if data.length > limit}
			<span class="text-xs text-gray-500 dark:text-gray-400">Top {limit}</span>
		{/if}
	</div>

	{#if loading}
		<div class="flex items-center justify-center h-32">
			<div class="animate-spin rounded-full h-8 w-8 border-b-2 border-orange-600"></div>
		</div>
	{:else if error}
		<div class="flex items-center justify-center h-32">
			<div class="text-center">
				<IconifyIcon icon="mdi:alert-circle" class="w-8 h-8 text-red-500 mx-auto mb-2" />
				<p class="text-red-600 dark:text-red-400 text-sm">{error}</p>
			</div>
		</div>
	{:else if !data.length}
		<div class="flex items-center justify-center h-32">
			<div class="text-center">
				<IconifyIcon icon="mdi:chart-bar" class="w-8 h-8 text-gray-400 mx-auto mb-2" />
				<p class="text-gray-500 dark:text-gray-400 text-sm">No data available</p>
			</div>
		</div>
	{:else}
		<div class="space-y-4">
			{#each limitedData as item, index}
				<div class="w-full">
					<div class="flex items-center">
						<!-- Bar that scales with container -->
						<div
							class="h-6 rounded flex items-center"
							style="background-color: {tailwindColors[
								index % tailwindColors.length
							]}; width: {(item.value / maxValue) * 60}%"
						></div>

						<!-- Label immediately after bar -->
						<div class="flex items-center gap-2 ml-3">
							{#if showIcons}
								<Icon
									name={item.label.toLowerCase()}
									showLabel={false}
									size="sm"
									class="w-4 h-4 flex-shrink-0"
								/>
							{/if}
							<span class="text-sm text-gray-700 dark:text-gray-300">{item.label}</span>
							<span class="text-sm font-semibold text-gray-900 dark:text-gray-100">
								{item.value}
							</span>
						</div>
					</div>
				</div>
			{/each}
		</div>
	{/if}
</div>
