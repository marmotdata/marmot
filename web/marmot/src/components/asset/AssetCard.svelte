<script lang="ts">
	import type { Asset } from '$lib/assets/types';
	import Icon from '$components/ui/Icon.svelte';

	export let asset: Asset;
	export let onClick: () => void = () => {};
	export let showDescription = false;
	export let compact = false;
	export let selected = false;

	$: iconName = asset.providers?.length === 1 ? asset.providers[0] : asset.type;
</script>

<div
	class="bg-white dark:bg-gray-800 p-4 transition-colors duration-150
    {!compact
		? 'rounded-lg border border-gray-200 dark:border-gray-700 shadow-md dark:shadow-gray-700'
		: ''}
    hover:bg-gray-50 dark:hover:bg-gray-700
    {selected ? '!bg-gray-100 dark:!bg-gray-700' : ''}
    {!compact ? 'cursor-pointer' : ''}"
	role="button"
	tabindex="0"
	aria-label="View asset {asset.name}"
	onclick={onClick}
	onkeydown={(e) => e.key === 'Enter' && onClick()}
>
	<div class="flex items-start space-x-4">
		<div class="flex-shrink-0">
			<Icon name={iconName} size={compact ? 'sm' : 'lg'} showLabel={false} />
		</div>
		<div class="flex-1 min-w-0">
			<h3 class="text-lg font-semibold text-gray-900 dark:text-gray-100 truncate">
				{asset.name || ''}
			</h3>
			<p class="text-sm text-gray-500 dark:text-gray-400 truncate">{asset.mrn || ''}</p>
			{#if asset.tags && asset.tags.length > 0}
				<div class="mt-2 flex flex-wrap gap-2">
					{#each asset.tags as tag, index (index)}
						<span
							class="px-2.5 py-0.5 rounded-full text-xs font-medium bg-gray-100 dark:bg-gray-700 text-gray-800 dark:text-gray-200"
						>
							{tag}
						</span>
					{/each}
				</div>
			{/if}
			{#if !compact}
				<div class="mt-2 flex items-center space-x-2">
					{#if asset.providers?.length > 0}
						<span
							class="px-2.5 py-0.5 rounded-full text-xs font-medium bg-gray-100 dark:bg-gray-700 text-gray-800 dark:text-gray-200"
						>
							{asset.providers.join(', ')}
						</span>
					{/if}
				</div>
			{/if}
		</div>
	</div>
	{#if showDescription && asset.description}
		<p class="mt-2 text-sm text-gray-600 dark:text-gray-400">{asset.description}</p>
	{/if}
</div>
