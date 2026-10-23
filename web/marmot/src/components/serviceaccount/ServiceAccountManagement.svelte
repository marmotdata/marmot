<script lang="ts">
	import { onMount } from 'svelte';
	import IconifyIcon from '@iconify/svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { listServiceAccounts } from '$lib/serviceaccounts/api';
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
	import type { ServiceAccount } from '$lib/serviceaccounts/types';
	import ServiceAccountTable from './ServiceAccountTable.svelte';
	import CreateServiceAccountBlade from './CreateServiceAccountBlade.svelte';

	// Service accounts are the identities agents and pipelines act as; the list is small, so search and the status filter run here
	let {
		createOpen = $bindable(false),
		canManage = true
	}: { createOpen?: boolean; canManage?: boolean } = $props();

	type StatusFilter = 'all' | 'active' | 'inactive';

	let accounts = $state<ServiceAccount[]>([]);
	let query = $state('');
	let statusFilter = $state<StatusFilter>('all');
	let loading = $state(true);
	let error = $state<string | null>(null);

	let filtered = $derived.by(() => {
		const q = query.trim().toLowerCase();
		return accounts
			.filter((a) =>
				statusFilter === 'all' ? true : statusFilter === 'active' ? a.active : !a.active
			)
			.filter(
				(a) =>
					!q || a.name.toLowerCase().includes(q) || (a.description ?? '').toLowerCase().includes(q)
			);
	});
	let inactiveCount = $derived(accounts.filter((a) => !a.active).length);

	const PILL = FILTER_PILL;
	const PILL_ON = FILTER_PILL_ON;
	const PILL_OFF = FILTER_PILL_OFF;

	async function fetchAccounts() {
		try {
			loading = true;
			error = null;
			accounts = await listServiceAccounts();
		} catch (err) {
			error = err instanceof Error ? err.message : m.serviceaccounts_error_generic();
		} finally {
			loading = false;
		}
	}

	function handleDeleted(id: string) {
		accounts = accounts.filter((a) => a.id !== id);
	}

	function handleCreated(sa: ServiceAccount) {
		createOpen = false;
		fetchAccounts();
		goto(resolve('/settings/service-accounts/[id]', { id: sa.id }));
	}

	onMount(() => {
		fetchAccounts();
		// The old /service-accounts/new route redirects here with ?create=1, so open the blade and drop the flag so a refresh does not reopen it
		if (page.url.searchParams.get('create') === '1') {
			createOpen = true;
			goto(resolve('/settings/service-accounts'), {
				replaceState: true,
				noScroll: true,
				keepFocus: true
			});
		}
	});
</script>

<div class="mb-4 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
	<div
		class="flex flex-wrap items-center gap-2"
		role="group"
		aria-label={m.serviceaccounts_filter_aria()}
	>
		{#each [{ id: 'all', label: m.common_all() }, { id: 'active', label: m.common_active() }, { id: 'inactive', label: m.common_inactive() }] as option (option.id)}
			<button
				type="button"
				aria-pressed={statusFilter === option.id}
				onclick={() => (statusFilter = option.id as StatusFilter)}
				class="{PILL} {statusFilter === option.id ? PILL_ON : PILL_OFF}"
			>
				{option.label}
				{#if option.id === 'inactive' && inactiveCount > 0}
					<span class="tabular-nums opacity-70">{inactiveCount}</span>
				{/if}
			</button>
		{/each}
	</div>
	<label class="{SEARCH_FIELD} w-full sm:w-72">
		<IconifyIcon icon="material-symbols:search" class="h-4 w-4 text-gray-400" />
		<input
			type="search"
			bind:value={query}
			placeholder={m.serviceaccounts_search_placeholder()}
			aria-label={m.serviceaccounts_search_placeholder()}
			class="min-w-0 flex-1 border-0 bg-transparent p-0 text-sm text-gray-900 placeholder:text-gray-400 focus:outline-none focus:ring-0 dark:text-gray-100"
		/>
	</label>
</div>

{#if error}
	<Alert tone="error">{error}</Alert>
{:else if loading && accounts.length === 0}
	<Skeleton class="h-14" rows={4} />
{:else if accounts.length === 0}
	<EmptyState
		icon="material-symbols:smart-toy-outline"
		title={m.serviceaccounts_none_yet()}
		description={m.serviceaccounts_empty_body()}
	>
		{#snippet action()}
			{#if canManage}
				<button type="button" class={BTN_PRIMARY} onclick={() => (createOpen = true)}>
					<IconifyIcon icon="material-symbols:add" class="h-5 w-5" />
					{m.serviceaccounts_new_button()}
				</button>
			{/if}
		{/snippet}
	</EmptyState>
{:else if filtered.length === 0}
	<EmptyState icon="material-symbols:search-off" title={m.serviceaccounts_no_match()}>
		{#snippet action()}
			<button
				type="button"
				class="text-sm {LINK}"
				onclick={() => {
					query = '';
					statusFilter = 'all';
				}}>{m.members_clear_filters()}</button
			>
		{/snippet}
	</EmptyState>
{:else}
	<Panel flush>
		<ServiceAccountTable accounts={filtered} {canManage} onDelete={handleDeleted} />
	</Panel>
{/if}

{#if createOpen}
	<CreateServiceAccountBlade onClose={() => (createOpen = false)} onCreated={handleCreated} />
{/if}
