<script lang="ts">
	import Icon from '$components/ui/Icon.svelte';

	export let title: string;
	export let isLoading: boolean;
	export let items: Record<string, number | { count: number; provider: string }>;
	export let filterType: 'types' | 'providers';

	const SkeletonCard = () => `
    <div class="bg-earthy-brown-50 dark:bg-gray-800 shadow-md dark:shadow-gray-700 rounded-lg p-4 flex flex-col items-center animate-pulse">
      <div class="w-12 h-12 bg-gray-200 dark:bg-gray-600 rounded-full mb-2"></div>
      <div class="h-6 bg-gray-200 dark:bg-gray-600 rounded w-24 mb-2"></div>
      <div class="h-4 bg-gray-200 dark:bg-gray-600 rounded w-16"></div>
    </div>
  `;

	function getCount(value: number | { count: number; provider: string }): number {
		return typeof value === 'number' ? value : value.count;
	}
</script>

<div class="space-y-4 w-full">
	<h2 class="text-2xl font-bold text-gray-900 dark:text-gray-100">{title}</h2>
	<div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4 w-full">
		{#if isLoading}
			{#each Array(6) as _}
				{@html SkeletonCard()}
			{/each}
		{:else}
			{#each Object.entries(items) as [key, value]}
				<a
					href="/discover?{filterType}={key}"
					class="w-full bg-earthy-brown-50 dark:bg-gray-900 shadow-md dark:shadow-gray-700 rounded-lg p-4 flex flex-col items-center cursor-pointer transition-all duration-300 hover:bg-gray-100 dark:hover:bg-gray-700"
				>
					<Icon name={key} size="lg" />
					<p class="text-gray-600 dark:text-gray-400 mt-2">
						{getCount(value)}
						{getCount(value) === 1 ? 'asset' : 'assets'}
					</p>
				</a>
			{/each}
		{/if}
	</div>
</div>
