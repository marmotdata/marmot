<script lang="ts">
	export let isLoading: boolean;
	export let tags: { [key: string]: number };

	const SkeletonTag = () => `
    <div class="bg-earthy-brown-50 dark:bg-gray-900 shadow-md dark:shadow-gray-700 rounded-full px-4 py-2 animate-pulse">
      <div class="h-4 bg-gray-200 dark:bg-gray-600 rounded w-20"></div>
    </div>
  `;

	$: sortedTags = Object.entries(tags)
		.sort((a, b) => b[1] - a[1])
		.slice(0, 20);
</script>

<div class="space-y-4 w-full">
	<h2 class="text-2xl font-bold text-gray-900 dark:text-gray-100">Popular Tags</h2>
	<div class="flex flex-wrap gap-4">
		{#if isLoading}
			{#each Array(12) as _}
				{@html SkeletonTag()}
			{/each}
		{:else}
			{#each sortedTags as [tag, count]}
				<a
					href="/assets?tags={encodeURIComponent(tag)}"
					class="bg-earthy-brown-50 dark:bg-gray-900 shadow-md dark:shadow-gray-700 rounded-full px-4 py-2 flex items-center gap-2 hover:bg-gray-50 dark:hover:bg-gray-700 transition-colors"
				>
					<span class="font-medium text-gray-900 dark:text-gray-100">{tag}</span>
					<span class="text-sm text-gray-600 dark:text-gray-400">({count})</span>
				</a>
			{/each}
		{/if}
	</div>
</div>
