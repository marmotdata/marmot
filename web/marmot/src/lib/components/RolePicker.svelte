<script lang="ts">
	import { onMount } from 'svelte';
	import { listRoles } from '$lib/roles/api';
	import type { Role, Permission } from '$lib/roles/types';
	import { m } from '$lib/paraglide/messages';
	import { INPUT } from '$lib/ui/classes';
	import Pill from '$components/ui/Pill.svelte';
	import Alert from '$components/ui/Alert.svelte';
	import Skeleton from '$components/ui/Skeleton.svelte';

	let {
		selectedIds = [],
		onChange = () => {},
		readonly = false
	}: { selectedIds?: string[]; onChange?: (ids: string[]) => void; readonly?: boolean } = $props();

	let allRoles = $state<Role[]>([]);
	let loading = $state(false);
	let error = $state<string | null>(null);
	let search = $state('');

	let filtered = $derived(
		search
			? allRoles.filter(
					(r) =>
						r.name.toLowerCase().includes(search.toLowerCase()) ||
						r.description?.toLowerCase().includes(search.toLowerCase())
				)
			: allRoles
	);

	let effectivePermissions = $derived(computeEffective(allRoles, selectedIds));

	function computeEffective(roles: Role[], ids: string[]): Permission[] {
		const seen: Record<string, true> = {};
		const perms: Permission[] = [];
		for (const r of roles) {
			if (!ids.includes(r.id)) continue;
			for (const p of r.permissions ?? []) {
				if (!seen[p.id]) {
					seen[p.id] = true;
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
		onChange(isSelected(id) ? selectedIds.filter((x) => x !== id) : [...selectedIds, id]);
	}

	onMount(async () => {
		try {
			loading = true;
			allRoles = await listRoles();
		} catch (e) {
			error = e instanceof Error ? e.message : m.ui_roles_load_error();
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
			placeholder={m.ui_rolepicker_search_placeholder()}
			class={INPUT}
		/>
	{/if}

	{#if loading}
		<Skeleton class="h-10" rows={3} />
	{:else if error}
		<Alert tone="error">{error}</Alert>
	{:else}
		<div
			class="max-h-60 divide-y divide-gray-100 overflow-y-auto rounded-lg border border-gray-200 bg-white dark:divide-gray-700/60 dark:border-gray-700 dark:bg-gray-800"
		>
			{#each filtered as r (r.id)}
				<div
					class="flex items-center gap-3 px-3 py-2 transition-colors hover:bg-gray-50 dark:hover:bg-gray-700/40"
				>
					{#if !readonly}
						<input
							type="checkbox"
							id="role-{r.id}"
							checked={isSelected(r.id)}
							onchange={() => toggle(r.id)}
							class="h-4 w-4 flex-shrink-0 rounded border-gray-300 text-earthy-terracotta-600 dark:border-gray-600"
						/>
					{/if}
					<label for="role-{r.id}" class="min-w-0 flex-1 {readonly ? '' : 'cursor-pointer'}">
						<span class="flex items-center gap-2">
							<span class="text-sm font-medium text-gray-900 dark:text-gray-100">{r.name}</span>
							{#if r.is_system}
								<Pill>{m.ui_role_system_badge()}</Pill>
							{/if}
						</span>
						{#if r.description}
							<span class="block truncate text-xs text-gray-500 dark:text-gray-400"
								>{r.description}</span
							>
						{/if}
					</label>
					<span class="flex-shrink-0 text-xs tabular-nums text-gray-500 dark:text-gray-400">
						{m.ui_rolepicker_perm_count({ count: (r.permissions ?? []).length })}
					</span>
				</div>
			{:else}
				<p class="py-4 text-center text-sm text-gray-500 dark:text-gray-400">
					{m.ui_rolepicker_no_roles()}
				</p>
			{/each}
		</div>

		{#if selectedIds.length > 0 && effectivePermissions.length > 0}
			<div class="mt-3">
				<p class="mb-2 text-xs font-medium text-gray-500 dark:text-gray-400">
					{m.ui_rolepicker_effective_permissions({ count: effectivePermissions.length })}
				</p>
				<div class="flex flex-wrap gap-1">
					{#each effectivePermissions as perm (perm.id)}
						<Pill>{perm.name}</Pill>
					{/each}
				</div>
			</div>
		{/if}
	{/if}
</div>
