<script lang="ts">
	import { onMount } from 'svelte';
	import { SvelteSet } from 'svelte/reactivity';
	import { listRoles } from '$lib/roles/api';
	import type { Role, Permission } from '$lib/roles/types';

	export let selectedIds: string[] = [];
	export let onChange: (ids: string[]) => void = () => {};
	export let readonly = false;

	let allRoles: Role[] = [];
	let loading = false;
	let error: string | null = null;
	let search = '';

	$: filtered = search
		? allRoles.filter(
				(r) =>
					r.name.toLowerCase().includes(search.toLowerCase()) ||
					r.description?.toLowerCase().includes(search.toLowerCase())
			)
		: allRoles;

	$: effectivePermissions = computeEffective(allRoles, selectedIds);

	function computeEffective(roles: Role[], ids: string[]): Permission[] {
		const seen = new SvelteSet<string>();
		const perms: Permission[] = [];
		for (const r of roles) {
			if (!ids.includes(r.id)) continue;
			for (const p of r.permissions ?? []) {
				if (!seen.has(p.id)) {
					seen.add(p.id);
					perms.push(p);
				}
			}
		}
		return perms.sort(
			(a, b) => a.resource_type.localeCompare(b.resource_type) || a.name.localeCompare(b.name)
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

	onMount(async () => {
		try {
			loading = true;
			allRoles = await listRoles();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load roles';
		} finally {
			loading = false;
		}
	});
</script>

<div class="space-y-3">
	{#if !readonly}
		<input
			type="text"
			bind:value={search}
			placeholder="Search roles..."
			class="w-full px-3 py-2 text-sm bg-white dark:bg-gray-900 border border-gray-300 dark:border-gray-600 rounded-md text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-earthy-terracotta-500 focus:border-transparent"
		/>
	{/if}

	{#if loading}
		<div class="flex justify-center p-4">
			<div class="animate-spin rounded-full h-6 w-6 border-b-2 border-earthy-terracotta-700"></div>
		</div>
	{:else if error}
		<div class="text-sm text-red-600 dark:text-red-400">{error}</div>
	{:else}
		<div
			class="border border-gray-200 dark:border-gray-700 rounded-md divide-y divide-gray-100 dark:divide-gray-700/50 max-h-60 overflow-y-auto"
		>
			{#each filtered as r (r.id)}
				<div
					class="flex items-center gap-3 px-3 py-2 hover:bg-gray-50 dark:hover:bg-gray-800/50 {isSelected(
						r.id
					)
						? 'bg-earthy-terracotta-50 dark:bg-earthy-terracotta-900/10'
						: ''}"
				>
					{#if !readonly}
						<input
							type="checkbox"
							id="role-{r.id}"
							checked={isSelected(r.id)}
							on:change={() => toggle(r.id)}
							class="h-4 w-4 text-earthy-terracotta-600 rounded border-gray-300 dark:border-gray-600 flex-shrink-0"
						/>
					{/if}
					<label for="role-{r.id}" class="flex-1 min-w-0 {readonly ? '' : 'cursor-pointer'}">
						<div class="flex items-center gap-2">
							<span class="text-sm font-medium text-gray-900 dark:text-gray-100">{r.name}</span>
							{#if r.is_system}
								<span
									class="text-xs px-1.5 py-0.5 bg-blue-100 dark:bg-blue-900/30 text-blue-700 dark:text-blue-400 rounded"
								>
									system
								</span>
							{/if}
						</div>
						{#if r.description}
							<p class="text-xs text-gray-500 dark:text-gray-400 truncate">{r.description}</p>
						{/if}
					</label>
					<span class="flex-shrink-0 text-xs text-gray-400 dark:text-gray-500">
						{(r.permissions ?? []).length} perms
					</span>
				</div>
			{:else}
				<p class="text-sm text-gray-500 dark:text-gray-400 text-center py-4">No roles found</p>
			{/each}
		</div>

		{#if selectedIds.length > 0 && effectivePermissions.length > 0}
			<div class="mt-3">
				<p class="text-xs font-medium text-gray-500 dark:text-gray-400 mb-2">
					Effective permissions ({effectivePermissions.length})
				</p>
				<div class="flex flex-wrap gap-1">
					{#each effectivePermissions as perm (perm.id)}
						<span
							class="text-xs px-2 py-0.5 bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-300 rounded"
						>
							{perm.name}
						</span>
					{/each}
				</div>
			</div>
		{/if}
	{/if}
</div>
