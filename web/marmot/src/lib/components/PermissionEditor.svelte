<script lang="ts">
	import { onMount } from 'svelte';
	import { listPermissions } from '$lib/roles/api';
	import type { Permission } from '$lib/roles/types';
	import { m } from '$lib/paraglide/messages';
	import { LABEL } from '$lib/ui/classes';
	import Alert from '$components/ui/Alert.svelte';
	import Skeleton from '$components/ui/Skeleton.svelte';

	// Every permission grouped by resource type, as a checklist or, when readonly, just the granted ones
	let {
		selectedIds = [],
		onChange = () => {},
		readonly = false
	}: { selectedIds?: string[]; onChange?: (ids: string[]) => void; readonly?: boolean } = $props();

	let allPermissions = $state<Permission[]>([]);
	let loading = $state(false);
	let error = $state<string | null>(null);

	let visiblePermissions = $derived(
		readonly ? allPermissions.filter((p) => selectedIds.includes(p.id)) : allPermissions
	);
	let grouped = $derived(
		visiblePermissions.reduce(
			(acc, p) => {
				(acc[p.resource_type] ??= []).push(p);
				return acc;
			},
			{} as Record<string, Permission[]>
		)
	);

	function isSelected(id: string): boolean {
		return selectedIds.includes(id);
	}

	function toggle(id: string) {
		if (readonly) return;
		onChange(isSelected(id) ? selectedIds.filter((x) => x !== id) : [...selectedIds, id]);
	}

	function toggleGroup(perms: Permission[]) {
		if (readonly) return;
		if (perms.every((p) => isSelected(p.id))) {
			const groupIds = new Set(perms.map((p) => p.id));
			onChange(selectedIds.filter((id) => !groupIds.has(id)));
		} else {
			onChange([...selectedIds, ...perms.map((p) => p.id).filter((id) => !isSelected(id))]);
		}
	}

	onMount(async () => {
		try {
			loading = true;
			allPermissions = await listPermissions();
		} catch (e) {
			error = e instanceof Error ? e.message : m.ui_permissions_load_error();
		} finally {
			loading = false;
		}
	});
</script>

{#if loading}
	<Skeleton class="h-24" rows={2} />
{:else if error}
	<Alert tone="error">{error}</Alert>
{:else}
	<div class="space-y-3">
		{#each Object.entries(grouped) as [resourceType, perms] (resourceType)}
			<div
				class="overflow-hidden rounded-lg border border-gray-200 bg-white dark:border-gray-700 dark:bg-gray-800"
			>
				<svelte:element
					this={readonly ? 'div' : 'label'}
					class="flex items-center gap-2 border-b border-gray-100 px-3 py-2 dark:border-gray-700/60 {readonly
						? ''
						: 'cursor-pointer hover:bg-gray-50 dark:hover:bg-gray-700/40'}"
				>
					{#if !readonly}
						<input
							type="checkbox"
							checked={perms.every((p) => isSelected(p.id))}
							indeterminate={perms.some((p) => isSelected(p.id)) &&
								!perms.every((p) => isSelected(p.id))}
							onchange={() => toggleGroup(perms)}
							class="rounded border-gray-300 text-earthy-terracotta-600 focus:ring-earthy-terracotta-500 dark:border-gray-600"
						/>
					{/if}
					<span class={LABEL}>{resourceType}</span>
					{#if !readonly}
						<!-- A selection counter, so it only belongs where there is a selection to make. -->
						<span class="ml-auto text-xs tabular-nums text-gray-500 dark:text-gray-400">
							{perms.filter((p) => isSelected(p.id)).length} / {perms.length}
						</span>
					{/if}
				</svelte:element>
				<div class="divide-y divide-gray-100 dark:divide-gray-700/60">
					{#each perms as perm (perm.id)}
						<svelte:element
							this={readonly ? 'div' : 'label'}
							class="flex items-start gap-3 px-3 py-2 {readonly
								? ''
								: 'cursor-pointer hover:bg-gray-50 dark:hover:bg-gray-700/40'}"
						>
							{#if !readonly}
								<input
									type="checkbox"
									checked={isSelected(perm.id)}
									onchange={() => toggle(perm.id)}
									class="mt-0.5 flex-shrink-0 rounded border-gray-300 text-earthy-terracotta-600 focus:ring-earthy-terracotta-500 dark:border-gray-600"
								/>
							{/if}
							<div class="min-w-0 flex-1">
								<span class="text-sm font-medium text-gray-900 dark:text-gray-100">{perm.name}</span
								>
								{#if perm.description}
									<p class="mt-0.5 text-sm text-gray-500 dark:text-gray-400">{perm.description}</p>
								{/if}
							</div>
						</svelte:element>
					{/each}
				</div>
			</div>
		{/each}

		{#if allPermissions.length === 0}
			<p class="py-4 text-center text-sm text-gray-500 dark:text-gray-400">
				{m.ui_permissioneditor_empty()}
			</p>
		{/if}
	</div>
{/if}
