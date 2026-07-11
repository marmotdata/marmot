<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { listServiceAccounts } from '$lib/serviceaccounts/api';
	import type { ServiceAccount } from '$lib/serviceaccounts/types';
	import ServiceAccountTable from './ServiceAccountTable.svelte';

	let accounts: ServiceAccount[] = [];
	let filtered: ServiceAccount[] = [];
	let query = '';
	let loading = false;
	let error: string | null = null;
	let searchTimer: ReturnType<typeof setTimeout>;

	async function fetchAccounts() {
		try {
			loading = true;
			accounts = await listServiceAccounts();
			applyFilter();
		} catch (err) {
			error = err instanceof Error ? err.message : 'An error occurred';
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

	function goCreate() {
		goto(resolve('/service-accounts/new'));
	}

	onMount(fetchAccounts);
</script>

<div
	class="bg-earthy-brown-50 dark:bg-gray-900 rounded-lg border border-gray-200 dark:border-gray-700"
>
	<div class="p-6">
		<div class="flex justify-between items-center mb-6">
			<div class="flex-1 max-w-md">
				<input
					type="text"
					placeholder="Search service accounts..."
					bind:value={query}
					class="w-full px-4 py-2 rounded-md border border-gray-300 dark:border-gray-600 focus:ring-2 focus:ring-earthy-terracotta-600 dark:focus:ring-earthy-terracotta-600 focus:border-transparent"
				/>
			</div>
			<button
				class="ml-4 px-4 py-2 bg-earthy-terracotta-700 dark:bg-earthy-terracotta-700 text-white rounded-md hover:bg-earthy-terracotta-800 dark:hover:bg-earthy-terracotta-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-earthy-terracotta-600 dark:focus:ring-earthy-terracotta-600"
				on:click={goCreate}
			>
				Add Service Account
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
					{query ? 'No service accounts match your search' : 'No service accounts yet'}
				</p>
			{/if}
		{/if}
	</div>
</div>
