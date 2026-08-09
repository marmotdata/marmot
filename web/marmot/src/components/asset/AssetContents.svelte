<script lang="ts">
	import { resolve } from '$app/paths';
	import type { Asset } from '$lib/assets/types';
	import AssetIcon from '$lib/components/AssetIcon.svelte';
	import IconifyIcon from '@iconify/svelte';

	// Containment is stored as CONTAINS lineage, so both directions come
	// from the same edges: what this asset holds, and what holds it.
	let { children, parents, loading }: { children: Asset[]; parents: Asset[]; loading: boolean } =
		$props();

	// mrn://type/service/full.qualified.name
	function assetUrl(child: Asset): string {
		if (!child.mrn || !child.type) return '';
		const parts = child.mrn.replace('mrn://', '').split('/');
		if (parts.length < 3) return '';
		return `/discover/${child.type.toLowerCase()}/${parts[1]}/${encodeURIComponent(parts.slice(2).join('/'))}`;
	}

	let sortKey = $state<'name' | 'type'>('name');
	let sortAscending = $state(true);

	function toggleSort(key: 'name' | 'type') {
		if (sortKey === key) {
			sortAscending = !sortAscending;
			return;
		}
		sortKey = key;
		sortAscending = true;
	}

	let sorted = $derived(
		[...children].sort((a, b) => {
			const left = (sortKey === 'name' ? a.name : a.type) || '';
			const right = (sortKey === 'name' ? b.name : b.type) || '';
			const order = left.localeCompare(right);
			return sortAscending ? order : -order;
		})
	);
</script>

{#if loading}
	<div class="py-8 text-center text-sm text-gray-500 dark:text-gray-400">Loading contents...</div>
{:else}
	{#if parents.length > 0}
		<div class="mb-4 flex flex-wrap items-center gap-2 text-sm">
			<IconifyIcon
				icon="material-symbols:subdirectory-arrow-left"
				class="w-4 h-4 text-gray-400 dark:text-gray-500 rotate-180"
			/>
			<span class="text-gray-500 dark:text-gray-400">Inside</span>
			{#each parents as parent (parent.mrn)}
				<a
					href={resolve(assetUrl(parent) as `/${string}`)}
					class="inline-flex items-center gap-1.5 px-2 py-1 rounded-md border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800 font-medium text-earthy-green-700 dark:text-earthy-green-400 hover:bg-gray-50 dark:hover:bg-gray-700/40 transition-colors"
				>
					<AssetIcon assetType={parent.type} providers={parent.providers ?? []} size="sm" />
					{parent.name}
				</a>
			{/each}
		</div>
	{/if}

	{#if children.length === 0}
		<div class="py-8 text-center text-sm text-gray-500 dark:text-gray-400">
			This asset holds nothing.
		</div>
	{:else}
		<div
			class="overflow-hidden rounded-lg border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800"
		>
			<table class="min-w-full divide-y divide-gray-200 dark:divide-gray-700">
				<thead class="bg-gray-50 dark:bg-gray-900/40">
					<tr>
						<th class="px-4 py-2.5 text-left">
							<button
								onclick={() => toggleSort('name')}
								class="flex items-center gap-1 text-xs font-semibold uppercase tracking-wider text-gray-600 dark:text-gray-300 hover:text-gray-900 dark:hover:text-white"
							>
								Name
								<IconifyIcon
									icon={sortKey === 'name' && !sortAscending
										? 'material-symbols:arrow-drop-up'
										: 'material-symbols:arrow-drop-down'}
									class="w-4 h-4 {sortKey === 'name' ? 'opacity-100' : 'opacity-30'}"
								/>
							</button>
						</th>
						<th class="px-4 py-2.5 text-left">
							<button
								onclick={() => toggleSort('type')}
								class="flex items-center gap-1 text-xs font-semibold uppercase tracking-wider text-gray-600 dark:text-gray-300 hover:text-gray-900 dark:hover:text-white"
							>
								Type
								<IconifyIcon
									icon={sortKey === 'type' && !sortAscending
										? 'material-symbols:arrow-drop-up'
										: 'material-symbols:arrow-drop-down'}
									class="w-4 h-4 {sortKey === 'type' ? 'opacity-100' : 'opacity-30'}"
								/>
							</button>
						</th>
						<th
							class="px-4 py-2.5 text-left text-xs font-semibold uppercase tracking-wider text-gray-600 dark:text-gray-300"
						>
							Description
						</th>
					</tr>
				</thead>
				<tbody class="divide-y divide-gray-200 dark:divide-gray-700">
					{#each sorted as child (child.mrn)}
						<tr class="hover:bg-gray-50 dark:hover:bg-gray-700/40 transition-colors">
							<td class="px-4 py-3 whitespace-nowrap">
								<a
									href={resolve(assetUrl(child) as `/${string}`)}
									class="flex items-center gap-2 text-sm font-medium text-earthy-green-700 dark:text-earthy-green-400 hover:underline"
								>
									<AssetIcon assetType={child.type} providers={child.providers ?? []} size="sm" />
									{child.name}
								</a>
							</td>
							<td class="px-4 py-3 whitespace-nowrap text-sm text-gray-600 dark:text-gray-300">
								{child.type}
							</td>
							<td class="px-4 py-3 text-sm text-gray-600 dark:text-gray-300">
								{child.user_description || child.description || '--'}
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	{/if}
{/if}
