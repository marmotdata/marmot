<script lang="ts">
	import IconifyIcon from '@iconify/svelte';

	export let title: string;
	export let value: number | string = 0;
	export let icon: string;
	export let loading: boolean = false;
	export let error: string | null = null;
	export let subtitle: string = '';

	$: displayValue = typeof value === 'number' ? value.toLocaleString() : value;
</script>

<div class="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-4">
	<div class="flex items-center gap-2 mb-2">
		<div class="p-2 bg-orange-100 dark:bg-orange-900/30 rounded-lg">
			<IconifyIcon {icon} class="w-5 h-5 text-orange-600 dark:text-orange-400" />
		</div>
		<h3 class="text-sm font-medium text-gray-700 dark:text-gray-300">{title}</h3>
	</div>

	{#if loading}
		<div class="flex items-center justify-center h-12">
			<div class="animate-spin rounded-full h-6 w-6 border-b-2 border-orange-600"></div>
		</div>
	{:else if error}
		<div class="text-red-500 dark:text-red-400 text-sm">{error}</div>
	{:else}
		<div class="text-2xl font-bold text-gray-900 dark:text-gray-100 mb-1">
			{displayValue}
		</div>
		{#if subtitle}
			<p class="text-xs text-gray-500 dark:text-gray-400">{subtitle}</p>
		{/if}
	{/if}
</div>
