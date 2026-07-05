<script lang="ts">
	import { onMount } from 'svelte';
	import { listPermissions } from '$lib/roles/api';
	import type { Permission } from '$lib/roles/types';

	export let selectedIds: string[] = [];
	export let onChange: (ids: string[]) => void = () => {};
	export let readonly = false;

	let allPermissions: Permission[] = [];
	let loading = false;
	let error: string | null = null;

	$: visiblePermissions = readonly
		? allPermissions.filter((p) => selectedIds.includes(p.id))
		: allPermissions;
	$: grouped = groupByResourceType(visiblePermissions);

	function groupByResourceType(perms: Permission[]): Record<string, Permission[]> {
		return perms.reduce(
			(acc, p) => {
				if (!acc[p.resource_type]) acc[p.resource_type] = [];
				acc[p.resource_type].push(p);
				return acc;
			},
			{} as Record<string, Permission[]>
		);
	}

	function isSelected(id: string): boolean {
		return selectedIds.includes(id);
	}

	function toggle(id: string) {
		if (readonly) return;
		const next = isSelected(id) ? selectedIds.filter((x) => x !== id) : [...selectedIds, id];
		selectedIds = next;
		onChange(next);
	}

	function toggleGroup(perms: Permission[]) {
		if (readonly) return;
		const allSelected = perms.every((p) => isSelected(p.id));
		let next: string[];
		if (allSelected) {
			const groupIds = new Set(perms.map((p) => p.id));
			next = selectedIds.filter((id) => !groupIds.has(id));
		} else {
			const toAdd = perms.map((p) => p.id).filter((id) => !isSelected(id));
			next = [...selectedIds, ...toAdd];
		}
		selectedIds = next;
		onChange(next);
	}

	onMount(async () => {
		try {
			loading = true;
			allPermissions = await listPermissions();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load permissions';
		} finally {
			loading = false;
		}
	});
</script>

{#if loading}
	<div class="flex justify-center p-4">
		<div class="animate-spin rounded-full h-6 w-6 border-b-2 border-earthy-terracotta-700"></div>
	</div>
{:else if error}
	<div class="text-sm text-red-600 dark:text-red-400 p-2">{error}</div>
{:else}
	<div class="space-y-3">
		{#each Object.entries(grouped) as [resourceType, perms] (resourceType)}
			<div
				class="border border-gray-200 dark:border-gray-700 rounded-md overflow-hidden bg-white dark:bg-gray-900"
			>
				<label
					class="flex items-center gap-2 px-3 py-2 bg-earthy-brown-100 dark:bg-gray-800 border-b border-gray-200 dark:border-gray-700 cursor-pointer hover:bg-earthy-brown-200 dark:hover:bg-gray-700/50"
				>
					{#if !readonly}
						<input
							type="checkbox"
							checked={perms.every((p) => isSelected(p.id))}
							indeterminate={perms.some((p) => isSelected(p.id)) &&
								!perms.every((p) => isSelected(p.id))}
							on:change={() => toggleGroup(perms)}
							class="rounded border-gray-300 dark:border-gray-600 text-earthy-terracotta-600 focus:ring-earthy-terracotta-500"
						/>
					{/if}
					<span
						class="text-xs font-semibold text-gray-600 dark:text-gray-400 uppercase tracking-wider"
					>
						{resourceType}
					</span>
					<span class="ml-auto text-xs text-gray-500 dark:text-gray-500">
						{perms.filter((p) => isSelected(p.id)).length} / {perms.length}
					</span>
				</label>
				<div class="divide-y divide-gray-100 dark:divide-gray-700/50">
					{#each perms as perm (perm.id)}
						<label
							class="flex items-start gap-3 px-3 py-2 hover:bg-earthy-brown-50 dark:hover:bg-gray-800/50 {readonly
								? ''
								: 'cursor-pointer'}"
						>
							{#if !readonly}
								<input
									type="checkbox"
									checked={isSelected(perm.id)}
									on:change={() => toggle(perm.id)}
									class="mt-0.5 rounded border-gray-300 dark:border-gray-600 text-earthy-terracotta-600 focus:ring-earthy-terracotta-500 flex-shrink-0"
								/>
							{/if}
							<div class="flex-1 min-w-0">
								<span class="text-sm font-medium text-gray-900 dark:text-gray-100">{perm.name}</span>
								{#if perm.description}
									<p class="text-xs text-gray-500 dark:text-gray-400 mt-0.5">{perm.description}</p>
								{/if}
							</div>
						</label>
					{/each}
				</div>
			</div>
		{/each}

		{#if allPermissions.length === 0}
			<p class="text-sm text-gray-500 dark:text-gray-400 text-center py-4">
				No permissions defined
			</p>
		{/if}
	</div>
{/if}
