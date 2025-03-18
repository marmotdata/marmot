<script>
	import MetadataView from '$lib/../components/MetadataView.svelte';
	import Icon from '$lib/../components/Icon.svelte';
	import Arrow from '$lib/../components/Arrow.svelte';

	export let sources = [];
	let expandedSources = new Set();

	function toggleSource(name) {
		if (expandedSources.has(name)) {
			expandedSources.delete(name);
		} else {
			expandedSources.add(name);
		}
		expandedSources = expandedSources;
	}
</script>

{#if sources && sources.length > 0}
	<div class="mb-6">
		<div class="space-y-4">
			{#each sources as source}
				<div
					class="bg-earthy-brown-50 dark:bg-gray-900 rounded-lg border border-gray-200 dark:border-gray-700 p-4"
				>
					<div class="flex justify-between items-start">
						<div
							class="flex items-center cursor-pointer"
							on:click={() => toggleSource(source.name)}
						>
							<div class="mr-2">
								<Arrow expanded={expandedSources.has(source.name)} />
							</div>
							<div class="p-2 rounded-lg mr-3">
								<Icon name={source.name} showLabel={false} class="w-5 h-5" />
							</div>
							<div>
								<h4 class="text-base font-medium text-gray-900 dark:text-gray-100">
									{source.name}
								</h4>
								<p class="text-sm text-gray-500 dark:text-gray-400 mt-1">
									Last synced: {new Date(source.last_sync_at).toLocaleString()}
								</p>
							</div>
						</div>
						<span
							class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-blue-100 dark:bg-blue-900 text-blue-800 dark:text-blue-100"
						>
							Priority: {source.priority}
						</span>
					</div>
					{#if expandedSources.has(source.name) && source.properties}
						<div class="mt-4 pl-6">
							<MetadataView metadata={source.properties} />
						</div>
					{/if}
				</div>
			{/each}
		</div>
	</div>
{/if}
