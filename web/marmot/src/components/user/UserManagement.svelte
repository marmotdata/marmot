<script lang="ts">
	import FilterSelect from '$components/ui/FilterSelect.svelte';
	import { onMount } from 'svelte';
	import IconifyIcon from '@iconify/svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { fetchApi } from '$lib/api';
	import { m } from '$lib/paraglide/messages';
	import { listRoles } from '$lib/roles/api';
	import {
		SEARCH_FIELD,
		BTN_PRIMARY,
		LINK,
		FILTER_PILL,
		FILTER_PILL_ON,
		FILTER_PILL_OFF
	} from '$lib/ui/classes';
	import Panel from '$components/ui/Panel.svelte';
	import Alert from '$components/ui/Alert.svelte';
	import Skeleton from '$components/ui/Skeleton.svelte';
	import EmptyState from '$components/ui/EmptyState.svelte';
	import UserTable from './UserTable.svelte';
	import CreateUserBlade from './CreateUserBlade.svelte';
	import type { User } from '$lib/users/types';
	import type { Role } from '$lib/roles/types';

	// The members list: search and role and status filters applied by the server, with the create blade opened from the page header
	let {
		createOpen = $bindable(false),
		canManage = true
	}: { createOpen?: boolean; canManage?: boolean } = $props();

	type StatusFilter = 'all' | 'active' | 'inactive';

	let users = $state<User[]>([]);
	let totalUsers = $state(0);
	let roles = $state<Role[]>([]);
	let userQuery = $state('');
	let roleFilter = $state('');
	let statusFilter = $state<StatusFilter>('all');
	let editingUserId = $state<string | null>(null);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let searchTimer: ReturnType<typeof setTimeout> | undefined;
	let seq = 0;

	let filtered = $derived(!!userQuery.trim() || !!roleFilter || statusFilter !== 'all');

	const PILL = FILTER_PILL;
	const PILL_ON = FILTER_PILL_ON;
	const PILL_OFF = FILTER_PILL_OFF;

	async function fetchUsers() {
		const mine = ++seq;
		try {
			loading = true;
			error = null;
			const entries: [string, string][] = [];
			if (userQuery.trim()) entries.push(['query', userQuery.trim()]);
			if (roleFilter) entries.push(['role_ids', roleFilter]);
			if (statusFilter !== 'all') entries.push(['active', String(statusFilter === 'active')]);
			const params = new URLSearchParams(entries);
			const response = await fetchApi(`/users?${params}`);
			const data = await response.json();
			if (mine !== seq) return;
			users = data.users ?? [];
			totalUsers = data.total ?? users.length;
		} catch (err) {
			if (mine === seq) error = err instanceof Error ? err.message : m.users_error_generic();
		} finally {
			if (mine === seq) loading = false;
		}
	}

	function scheduleSearch() {
		clearTimeout(searchTimer);
		searchTimer = setTimeout(fetchUsers, 300);
	}

	function setRole(id: string) {
		roleFilter = id;
		fetchUsers();
	}

	function setStatus(next: StatusFilter) {
		statusFilter = next;
		fetchUsers();
	}

	function clearFilters() {
		userQuery = '';
		roleFilter = '';
		statusFilter = 'all';
		fetchUsers();
	}

	function handleCreated() {
		createOpen = false;
		fetchUsers();
	}

	async function handleUserUpdated(updatedUser: User) {
		users = users.map((u) => (u.id === updatedUser.id ? updatedUser : u));
		editingUserId = null;
		await fetchUsers();
	}

	async function handleUserDeleted(userId: string) {
		users = users.filter((u) => u.id !== userId);
		await fetchUsers();
	}

	onMount(() => {
		fetchUsers();
		listRoles()
			.then((list) => (roles = list))
			.catch(() => (roles = []));
		// The old /users/new route redirects here with ?create=1, so open the blade and drop the flag so a refresh does not reopen it
		if (page.url.searchParams.get('create') === '1') {
			createOpen = true;
			goto(resolve('/settings/members'), { replaceState: true, noScroll: true, keepFocus: true });
		}
	});
</script>

<div class="mb-4 flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
	<div class="flex flex-wrap items-center gap-2" role="group" aria-label={m.members_filter_aria()}>
		{#each [{ id: 'all', label: m.common_all() }, { id: 'active', label: m.common_active() }, { id: 'inactive', label: m.common_inactive() }] as option (option.id)}
			<button
				type="button"
				aria-pressed={statusFilter === option.id}
				onclick={() => setStatus(option.id as StatusFilter)}
				class="{PILL} {statusFilter === option.id ? PILL_ON : PILL_OFF}">{option.label}</button
			>
		{/each}
		{#if roles.length > 0}
			<span class="mx-1 h-4 w-px bg-gray-200 dark:bg-gray-700" aria-hidden="true"></span>
			<FilterSelect
				label={m.members_role_filter_label()}
				anyLabel={m.members_role_filter_any()}
				searchPlaceholder={m.members_role_filter_search()}
				options={roles.map((role) => ({ id: role.id, label: role.name }))}
				value={roleFilter}
				onChange={(id) => setRole(id)}
			/>
		{/if}
	</div>
	<label class="{SEARCH_FIELD} w-full lg:w-72">
		<IconifyIcon icon="material-symbols:search" class="h-4 w-4 text-gray-400" />
		<input
			type="search"
			bind:value={userQuery}
			oninput={scheduleSearch}
			placeholder={m.users_search_placeholder()}
			aria-label={m.users_search_placeholder()}
			class="min-w-0 flex-1 border-0 bg-transparent p-0 text-sm text-gray-900 placeholder:text-gray-400 focus:outline-none focus:ring-0 dark:text-gray-100"
		/>
	</label>
</div>

{#if error}
	<Alert tone="error">{error}</Alert>
{:else if loading && users.length === 0}
	<Skeleton class="h-14" rows={5} />
{:else if users.length === 0}
	{#if filtered}
		<EmptyState
			icon="material-symbols:person-search-outline"
			title={m.members_no_match_title()}
			description={m.members_no_match_body()}
		>
			{#snippet action()}
				<button type="button" class="text-sm {LINK}" onclick={clearFilters}>
					{m.members_clear_filters()}
				</button>
			{/snippet}
		</EmptyState>
	{:else}
		<EmptyState
			icon="material-symbols:person-outline"
			title={m.members_empty_title()}
			description={m.members_empty_body()}
		>
			{#snippet action()}
				{#if canManage}
					<button type="button" class={BTN_PRIMARY} onclick={() => (createOpen = true)}>
						<IconifyIcon icon="material-symbols:person-add-outline" class="h-5 w-5" />
						{m.members_add_button()}
					</button>
				{/if}
			{/snippet}
		</EmptyState>
	{/if}
{:else}
	<Panel flush>
		<UserTable
			{users}
			{editingUserId}
			{canManage}
			onEdit={(userId) => (editingUserId = userId)}
			onUpdate={handleUserUpdated}
			onDelete={handleUserDeleted}
		/>
		<div
			class="border-t border-gray-100 px-5 py-3 text-xs tabular-nums text-gray-500 dark:border-gray-700/60 dark:text-gray-400"
		>
			{totalUsers > users.length
				? m.members_showing_first({ count: users.length, total: totalUsers })
				: m.members_count({ count: totalUsers })}
		</div>
	</Panel>
{/if}

{#if createOpen}
	<CreateUserBlade onClose={() => (createOpen = false)} onCreated={handleCreated} />
{/if}
