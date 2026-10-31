<script lang="ts">
	import { onMount } from 'svelte';
	import { listRoles } from '$lib/roles/api';
	import type { Role } from '$lib/roles/types';
	import { m } from '$lib/paraglide/messages';

	// Picks roles for a member or service account. An instance has dozens of roles and a principal holds a few, so the few are shown as removable tags and the rest are reached through a search, rather than a checkbox per role.
	interface Props {
		selectedIds: string[];
		onChange: (ids: string[]) => void;
		roles?: Role[];
		placeholder?: string;
		emptyMessage?: string;
		/** Kept for callers that still pass it; the picker no longer pages. */
		pageSize?: number;
	}

	let {
		selectedIds,
		onChange,
		roles: providedRoles,
		placeholder = m.ui_roleselector_search_placeholder(),
		emptyMessage = m.ui_roleselector_empty()
	}: Props = $props();

	let internalRoles = $state<Role[]>([]);
	let loading = $state(false);
	let error = $state<string | null>(null);
	let query = $state('');

	let allRoles = $derived(providedRoles ?? internalRoles);
	let selected = $derived(
		selectedIds.map((id) => allRoles.find((r) => r.id === id)).filter((r): r is Role => !!r)
	);
	let choices = $derived.by(() => {
		const q = query.trim().toLowerCase();
		return allRoles
			.filter((r) => !selectedIds.includes(r.id))
			.filter(
				(r) =>
					!q || r.name.toLowerCase().includes(q) || (r.description ?? '').toLowerCase().includes(q)
			);
	});

	onMount(async () => {
		if (providedRoles) return;
		try {
			loading = true;
			internalRoles = await listRoles();
		} catch (err) {
			error = err instanceof Error ? err.message : m.ui_roles_load_error();
		} finally {
			loading = false;
		}
	});

	function add(role: Role) {
		onChange([...selectedIds, role.id]);
		query = '';
	}

	function remove(id: string) {
		onChange(selectedIds.filter((r) => r !== id));
	}
</script>

{#if loading}
	<mm-skeleton loading count={1}></mm-skeleton>
{:else if error}
	<mm-callout tone="danger" compact heading={error}></mm-callout>
{:else}
	<div class="mm-cluster" style="--mm-gap: var(--space-2)">
		{#each selected as role (role.id)}
			<mm-tag removable title={role.description} onmm-remove={() => remove(role.id)}
				>{role.name}</mm-tag
			>
		{:else}
			<span class="mm-caption">{m.ui_roleselector_none_selected()}</span>
		{/each}
		<mm-popover label={m.ui_roleselector_add()} style="--mm-popover-min-width: 20rem">
			<mm-button slot="trigger" size="sm" icon="add">{m.ui_roleselector_add()}</mm-button>
			<div class="mm-stack" style="--mm-gap: var(--space-3)">
				<mm-search-field
					label={placeholder}
					hide-label
					{placeholder}
					value={query}
					autofocus
					onmm-input={(e: CustomEvent<{ value: string }>) => (query = e.detail.value)}
					onmm-search={(e: CustomEvent<{ value: string }>) => {
						e.preventDefault();
						if (choices[0]) add(choices[0]);
					}}
				></mm-search-field>
				{#if choices.length > 0}
					<mm-list dense class="max-h-72 overflow-y-auto" tabindex="-1">
						{#each choices as role (role.id)}
							<mm-list-item interactive secondary={role.description} onmm-activate={() => add(role)}
								>{role.name}</mm-list-item
							>
						{/each}
					</mm-list>
				{:else}
					<p class="mm-caption m-0" role="status">
						{query.trim() ? m.ui_roleselector_no_match({ query }) : emptyMessage}
					</p>
				{/if}
			</div>
		</mm-popover>
	</div>
{/if}
