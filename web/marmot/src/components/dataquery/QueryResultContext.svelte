<script lang="ts">
	import { resolve } from '$app/paths';
	import Icon from '$components/ui/Icon.svelte';
	import { m } from '$lib/paraglide/messages';
	import { assetRouteParams } from '$lib/dataquery/format';
	import type { QueryAssetContext } from '$lib/dataquery/types';

	// The catalog entries a query's referenced tables resolved to, so a result can be traced back
	// to the assets that describe it.
	let { context }: { context: QueryAssetContext[] } = $props();
</script>

{#if context.length === 0}
	<p class="p-6 text-sm text-gray-500 dark:text-gray-400">
		{m.dataquery_console_context_empty()}
	</p>
{:else}
	<ul class="grid gap-3 p-4 lg:grid-cols-2 2xl:grid-cols-3">
		{#each context as item, i (item.mrn)}
			{@const route = item.asset_id ? assetRouteParams(item.mrn) : null}
			<li
				class="context-card rounded-xl border border-gray-200 bg-white p-4 dark:border-gray-700 dark:bg-gray-800"
				style="animation-delay: {i * 60}ms"
			>
				<div class="flex items-center gap-2">
					{#if item.providers?.[0]}
						<Icon name={item.providers[0]} size="xs" showLabel={false} />
					{/if}
					{#if route}
						<a
							href={resolve('/discover/[type]/[service]/[name]', route)}
							class="truncate font-semibold text-earthy-terracotta-700 hover:underline dark:text-earthy-terracotta-400"
							>{item.name || item.mrn}</a
						>
					{:else}
						<span class="truncate font-semibold text-gray-900 dark:text-gray-100"
							>{item.name || item.mrn}</span
						>
					{/if}
					{#if item.type}
						<span
							class="ml-auto rounded bg-gray-100 px-1.5 py-0.5 text-[10px] uppercase text-gray-600 dark:bg-gray-700 dark:text-gray-300"
							>{item.type}</span
						>
					{/if}
				</div>
				{#if item.description}
					<p class="mt-2 line-clamp-3 text-sm text-gray-600 dark:text-gray-300">
						{item.description}
					</p>
				{/if}
				{#if item.tags?.length}
					<div class="mt-2 flex flex-wrap gap-1">
						{#each item.tags as tag (tag)}
							<span
								class="rounded-full bg-earthy-brown-100 px-2 py-0.5 text-[11px] text-earthy-brown-900 dark:bg-gray-700 dark:text-gray-300"
								>{tag}</span
							>
						{/each}
					</div>
				{/if}
			</li>
		{/each}
	</ul>
{/if}

<style>
	.context-card {
		animation: result-in 0.3s cubic-bezier(0.2, 0.8, 0.2, 1) both;
	}

	@keyframes result-in {
		from {
			opacity: 0;
			transform: translateY(6px);
		}
	}

	@media (prefers-reduced-motion: reduce) {
		.context-card {
			animation: none;
		}
	}
</style>
