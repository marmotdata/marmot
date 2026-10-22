<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { listServiceAccounts } from '$lib/serviceaccounts/api';
	import { m } from '$lib/paraglide/messages';
	import type { ServiceAccount } from '$lib/serviceaccounts/types';
	import ServiceAccountTable from './ServiceAccountTable.svelte';
	import CreateServiceAccountBlade from './CreateServiceAccountBlade.svelte';
	import { page } from '$app/stores';

	let accounts: ServiceAccount[] = [];
	let filtered: ServiceAccount[] = [];
	let query = '';
	let loading = false;
	let error: string | null = null;
	let searchTimer: ReturnType<typeof setTimeout>;
	let showCreate = false;

	async function fetchAccounts() {
		try {
			loading = true;
			accounts = await listServiceAccounts();
			applyFilter();
		} catch (err) {
			error = err instanceof Error ? err.message : m.serviceaccounts_error_generic();
		} finally {
			loading = false;
		}
	}

	function applyFilter() {
		const q = query.trim().toLowerCase();
		filtered = q
			? accounts.filter(
					(a) => a.name.toLowerCase().includes(q) || (a.description ?? '').toLowerCase().includes(q)
				)
			: accounts;
	}

	function scheduleSearch() {
		if (searchTimer) clearTimeout(searchTimer);
		searchTimer = setTimeout(applyFilter, 200);
	}

	$: if (query !== undefined) scheduleSearch();

	function handleDeleted(id: string) {
		accounts = accounts.filter((a) => a.id !== id);
		applyFilter();
	}

	function handleCreated(sa: ServiceAccount) {
		showCreate = false;
		fetchAccounts();
		goto(resolve('/service-accounts/[id]', { id: sa.id }));
	}

	onMount(() => {
		fetchAccounts();
		// The old /service-accounts/new route redirects here with ?create=1, so open the blade and drop the flag so a refresh does not reopen it
		if ($page.url.searchParams.get('create') === '1') {
			showCreate = true;
			goto(resolve('/admin?tab=service_accounts'), {
				replaceState: true,
				noScroll: true,
				keepFocus: true
			});
		}
	});
</script>

<div
	class="bg-earthy-brown-50 dark:bg-gray-900 rounded-lg border border-gray-200 dark:border-gray-700"
>
	<div class="p-6">
		<div class="flex justify-between items-center mb-6">
			<div class="flex-1 max-w-md">
				<input
					type="text"
					placeholder={m.serviceaccounts_search_placeholder()}
					bind:value={query}
					class="w-full px-4 py-2 rounded-md border border-gray-300 dark:border-gray-600 focus:ring-2 focus:ring-earthy-terracotta-600 dark:focus:ring-earthy-terracotta-600 focus:border-transparent"
				/>
			</div>
			<button
				class="ml-4 px-4 py-2 bg-earthy-terracotta-700 dark:bg-earthy-terracotta-700 text-white rounded-md hover:bg-earthy-terracotta-800 dark:hover:bg-earthy-terracotta-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-earthy-terracotta-600 dark:focus:ring-earthy-terracotta-600"
				on:click={() => (showCreate = true)}
			>
				{m.serviceaccounts_add_button()}
			</button>
		</div>

		{#if loading && !accounts.length}
			<div class="flex justify-center p-8">
				<div
					class="animate-spin rounded-full h-8 w-8 border-b-2 border-earthy-terracotta-700"
				></div>
			</div>
		{:else if error}
			<div class="bg-red-50 border border-red-200 rounded-lg p-4 text-red-700">
				{error}
			</div>
		{:else}
			<ServiceAccountTable accounts={filtered} onDelete={handleDeleted} />

			{#if filtered.length === 0}
				<p class="text-sm text-gray-500 dark:text-gray-400 text-center py-8">
					{query ? m.serviceaccounts_no_match() : m.serviceaccounts_none_yet()}
				</p>
			{/if}
		{/if}
	</div>
</div>

{#if showCreate}
	<CreateServiceAccountBlade onClose={() => (showCreate = false)} onCreated={handleCreated} />
{/if}
