<script lang="ts">
	import { onMount } from 'svelte';
	import IconifyIcon from '@iconify/svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { listRoles } from '$lib/roles/api';
	import { m } from '$lib/paraglide/messages';
	import {
		BTN_PRIMARY,
		LINK,
		SEARCH_FIELD,
		FILTER_PILL,
		FILTER_PILL_ON,
		FILTER_PILL_OFF
	} from '$lib/ui/classes';
	import Panel from '$components/ui/Panel.svelte';
	import Alert from '$components/ui/Alert.svelte';
	import Skeleton from '$components/ui/Skeleton.svelte';
	import EmptyState from '$components/ui/EmptyState.svelte';
	import RoleTable from './RoleTable.svelte';
	import CreateRoleBlade from './CreateRoleBlade.svelte';
	import type { Role } from '$lib/roles/types';
	import { documentFromRole, type RoleDocument } from '$lib/roles/document';

	// Every role, filtered here because the list is small, with rows that expand to show or edit their permissions
	let {
		createOpen = $bindable(false),
		canManage = true
	}: { createOpen?: boolean; canManage?: boolean } = $props();

	type KindFilter = 'all' | 'system' | 'custom';

	let roles = $state<Role[]>([]);
	let roleQuery = $state('');
	let kindFilter = $state<KindFilter>('all');
	let editingRoleId = $state<string | null>(null);
	// The document a duplicated role starts the create blade with; cleared whenever the blade closes.
	let createInitial = $state<RoleDocument | undefined>(undefined);
	let loading = $state(true);
	let error = $state<string | null>(null);

	let filteredRoles = $derived.by(() => {
		const q = roleQuery.trim().toLowerCase();
		return roles
			.filter((r) =>
				kindFilter === 'all' ? true : kindFilter === 'system' ? r.is_system : !r.is_system
			)
			.filter(
				(r) =>
					!q || r.name.toLowerCase().includes(q) || (r.description ?? '').toLowerCase().includes(q)
			);
	});

	const PILL = FILTER_PILL;
	const PILL_ON = FILTER_PILL_ON;
	const PILL_OFF = FILTER_PILL_OFF;

	async function fetchRoles() {
		try {
			loading = true;
			error = null;
			roles = await listRoles();
		} catch (err) {
			error = err instanceof Error ? err.message : m.roles_error_generic();
		} finally {
			loading = false;
		}
	}

	function handleCreated() {
		createOpen = false;
		createInitial = undefined;
		fetchRoles();
	}

	function handleDuplicate(role: Role) {
		createInitial = { ...documentFromRole(role), name: `${role.name}-copy` };
		createOpen = true;
	}

	function handleRoleUpdated(updatedRole: Role) {
		roles = roles.map((r) => (r.id === updatedRole.id ? updatedRole : r));
		editingRoleId = null;
	}

	function handleRoleDeleted(roleId: string) {
		roles = roles.filter((r) => r.id !== roleId);
	}

	onMount(() => {
		fetchRoles();
		// The old /roles/new route redirects here with ?create=1, so open the blade and drop the flag so a refresh does not reopen it
		if (page.url.searchParams.get('create') === '1') {
			createOpen = true;
			goto(resolve('/settings/roles'), { replaceState: true, noScroll: true, keepFocus: true });
		}
	});
</script>

<div class="mb-4 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
	<div class="flex flex-wrap items-center gap-2" role="group" aria-label={m.roles_filter_aria()}>
		{#each [{ id: 'all', label: m.common_all() }, { id: 'system', label: m.ui_role_system_badge() }, { id: 'custom', label: m.roles_custom_filter() }] as option (option.id)}
			<button
				type="button"
				aria-pressed={kindFilter === option.id}
				onclick={() => (kindFilter = option.id as KindFilter)}
				class="{PILL} {kindFilter === option.id ? PILL_ON : PILL_OFF}">{option.label}</button
			>
		{/each}
	</div>
	<label class="{SEARCH_FIELD} w-full sm:w-72">
		<IconifyIcon icon="material-symbols:search" class="h-4 w-4 text-gray-400" />
		<input
			type="search"
			bind:value={roleQuery}
			placeholder={m.roles_search_placeholder()}
			aria-label={m.roles_search_placeholder()}
			class="min-w-0 flex-1 border-0 bg-transparent p-0 text-sm text-gray-900 placeholder:text-gray-400 focus:outline-none focus:ring-0 dark:text-gray-100"
		/>
	</label>
</div>

{#if error}
	<Alert tone="error">{error}</Alert>
{:else if loading && roles.length === 0}
	<Skeleton class="h-14" rows={4} />
{:else if roles.length === 0}
	<EmptyState icon="material-symbols:badge-outline" title={m.roles_none_defined()}>
		{#snippet action()}
			{#if canManage}
				<button type="button" class={BTN_PRIMARY} onclick={() => (createOpen = true)}>
					<IconifyIcon icon="material-symbols:add" class="h-5 w-5" />
					{m.roles_new_button()}
				</button>
			{/if}
		{/snippet}
	</EmptyState>
{:else if filteredRoles.length === 0}
	<EmptyState icon="material-symbols:search-off" title={m.roles_no_match_search()}>
		{#snippet action()}
			<button
				type="button"
				class="text-sm {LINK}"
				onclick={() => {
					roleQuery = '';
					kindFilter = 'all';
				}}>{m.members_clear_filters()}</button
			>
		{/snippet}
	</EmptyState>
{:else}
	<Panel flush>
		<RoleTable
			roles={filteredRoles}
			{editingRoleId}
			{canManage}
			onEdit={(roleId) => (editingRoleId = roleId)}
			onUpdate={handleRoleUpdated}
			onDelete={handleRoleDeleted}
			onDuplicate={handleDuplicate}
		/>
	</Panel>
{/if}

{#if createOpen}
	<CreateRoleBlade
		initial={createInitial}
		onClose={() => {
			createOpen = false;
			createInitial = undefined;
		}}
		onCreated={handleCreated}
	/>
{/if}
