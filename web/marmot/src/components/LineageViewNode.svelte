<script lang="ts">
	import Icon from './Icon.svelte';
	import MetadataView from './MetadataView.svelte';
	import Arrow from './Arrow.svelte';

	export let node: any;
	export let expanded: boolean;
	export let onClick: () => void;
	export let maxMetadataDepth = 1;
	export let compact = false;

	function getAssetUrl(): string {
		if (!node.asset?.mrn) return '#';
		// Parse MRN: mrn://type/service/full.qualified.name
		const mrnParts = node.asset.mrn.replace('mrn://', '').split('/');
		if (mrnParts.length < 3) return '#';
		const type = mrnParts[0];
		const service = mrnParts[1];
		const fullName = mrnParts.slice(2).join('/');
		return `/discover/${encodeURIComponent(type)}/${encodeURIComponent(service)}/${encodeURIComponent(fullName)}`;
	}
</script>

{#if compact}
	<!-- Compact Mode -->
	<div class="rounded border border-gray-200 dark:border-gray-700">
		<a
			href={getAssetUrl()}
			class="flex items-center gap-2 p-2 hover:bg-gray-50 dark:hover:bg-gray-800/50 transition-colors group"
		>
			<Icon
				name={node.asset.providers?.length === 1 ? node.asset.providers[0] : node.type}
				showLabel={false}
				iconSize="xs"
			/>
			<div class="flex-1 min-w-0">
				<p class="text-sm font-medium text-gray-900 dark:text-gray-100 truncate">
					{node.id.split('/').pop()}
				</p>
			</div>
			<svg
				class="w-3.5 h-3.5 text-gray-400 group-hover:text-earthy-terracotta-700 flex-shrink-0"
				fill="none"
				stroke="currentColor"
				viewBox="0 0 24 24"
			>
				<path
					stroke-linecap="round"
					stroke-linejoin="round"
					stroke-width="2"
					d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14"
				/>
			</svg>
		</a>
	</div>
{:else}
	<!-- Full Mode -->
	<div class="rounded-lg border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800">
		<div
			class="flex p-4 cursor-pointer hover:bg-earthy-terracotta-50 dark:hover:bg-gray-700"
			on:click={onClick}
		>
			<div class="flex items-start space-x-3 flex-1 min-w-0">
				<Icon
					name={node.asset.providers?.length === 1 ? node.asset.providers[0] : node.type}
					showLabel={false}
					iconSize="sm"
				/>
				<div class="flex-1 min-w-0">
					<h3 class="font-medium text-gray-900 dark:text-gray-100 truncate">
						{node.id.split('/').pop()}
					</h3>
					<p class="text-sm text-gray-600 dark:text-gray-400 truncate">
						{node.asset.providers?.join(', ') || node.asset.provider}
					</p>
				</div>
			</div>
			<div class="flex items-center gap-2 flex-shrink-0 ml-4">
				<a
					href={getAssetUrl()}
					class="inline-flex items-center text-sm text-gray-600 dark:text-gray-400 hover:text-earthy-terracotta-700 whitespace-nowrap"
					on:click|stopPropagation
				>
					<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
						<path
							stroke-linecap="round"
							stroke-linejoin="round"
							stroke-width="2"
							d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14"
						/>
					</svg>
					<span class="ml-1">View Full</span>
				</a>
				<Arrow {expanded} />
			</div>
		</div>
		{#if expanded}
			<div class="px-4 pb-4 border-t border-gray-200 dark:border-gray-700">
				{#if node.asset.description}
					<div class="mt-3">
						<h5 class="text-sm font-medium text-gray-900 dark:text-gray-100">Description</h5>
						<p class="mt-1 text-sm text-gray-600 dark:text-gray-400 line-clamp-2">
							{node.asset.description}
						</p>
					</div>
				{/if}
				{#if node.asset.tags?.length > 0}
					<div class="mt-3">
						<h5 class="text-sm font-medium text-gray-900 dark:text-gray-100">Tags</h5>
						<div class="mt-1 flex flex-wrap gap-2">
							{#each node.asset.tags as tag}
								<span
									class="text-xs bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-400 px-2 py-1 rounded-full"
									>{tag}</span
								>
							{/each}
						</div>
					</div>
				{/if}
				{#if Object.keys(node.asset.metadata || {}).length > 0}
					<div class="mt-3">
						<h5 class="text-sm font-medium text-gray-900 dark:text-gray-100">Metadata</h5>
						<div class="">
							<MetadataView
								metadata={node.asset.metadata}
								maxDepth={0}
								maxCharLength={50}
								showDetailsLink={getAssetUrl()}
							/>
						</div>
					</div>
				{/if}
			</div>
		{/if}
	</div>
{/if}
