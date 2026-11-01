<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import type { DataTableColumn } from '@marmotdata/elements/classes/data-table';
	import { myAccess, holdsAtRoot } from '$lib/iam/myAccess';
	import { listRoles } from '$lib/roles/api';
	import type { Role } from '$lib/roles/types';
	import { m } from '$lib/paraglide/messages';

	let canManage = $derived(holdsAtRoot($myAccess, 'roles:manage'));
	let roles = $state<Role[]>([]);
	let loading = $state(true);
	let error = $state('');
	let query = $state('');

	// Roles are few, so the list is filtered here rather than by the API
	let filtered = $derived.by(() => {
		const q = query.trim().toLowerCase();
		if (!q) return roles;
		return roles.filter(
			(r) => r.name.toLowerCase().includes(q) || (r.description ?? '').toLowerCase().includes(q)
		);
	});

	const columns: DataTableColumn<Role>[] = [
		{
			key: 'name',
			header: m.roles_column_role(),
			sortable: true,
			width: 180,
			href: (r) => resolve('/settings/roles/[id]', { id: r.id })
		},
		{ key: 'description', header: m.roles_column_allows(), empty: m.roles_description_empty() },
		{
			key: 'is_system',
			header: m.roles_column_type(),
			sortable: true,
			width: 120,
			value: (r) => (r.is_system ? m.roles_kind_built_in() : m.roles_kind_custom())
		},
		{
			key: 'user_count',
			header: m.roles_column_members(),
			type: 'number',
			sortable: true,
			width: 110,
			value: (r) => r.user_count ?? 0
		}
	];

	async function load() {
		loading = true;
		error = '';
		try {
			roles = await listRoles();
		} catch (err) {
			error = err instanceof Error ? err.message : m.roles_error_generic();
		} finally {
			loading = false;
		}
	}

	onMount(() => {
		// The old /roles/new route redirects here with ?create=1, which now means the create page
		if (page.url.searchParams.get('create') === '1') {
			goto(resolve('/settings/roles/new'), { replaceState: true });
			return;
		}
		load();
	});
</script>

<mm-page-header
	heading={m.settings_roles()}
	compact
	count={loading || error ? undefined : roles.length}
	description={m.roles_page_description()}
>
	{#if canManage}
		<mm-button slot="actions" variant="primary" icon="add" href={resolve('/settings/roles/new')}
			>{m.roles_create_title()}</mm-button
		>
	{/if}
</mm-page-header>

<mm-search-field
	class="w-80 max-w-full"
	label={m.roles_search_placeholder()}
	hide-label
	placeholder={m.roles_search_placeholder()}
	value={query}
	onmm-input={(e: CustomEvent<{ value: string }>) => (query = e.detail.value)}
></mm-search-field>

<mm-data-table
	label={m.settings_roles()}
	{columns}
	rows={filtered}
	sort={{ key: 'name', direction: 'ascending' }}
	{loading}
	{error}
	empty-heading={query.trim() ? m.roles_no_match_search() : m.roles_none_defined()}
	onmm-retry={load}
></mm-data-table>
