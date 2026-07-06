<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { listRoles } from '$lib/roles/api';
	import RoleTable from './RoleTable.svelte';
	import type { Role } from '$lib/roles/types';

	let roles: Role[] = [];
	let filteredRoles: Role[] = [];
	let roleQuery = '';
	let editingRoleId: string | null = null;
	let loading = false;
	let error: string | null = null;
	let searchTimer: ReturnType<typeof setTimeout>;

	async function fetchRoles() {
		try {
			loading = true;
			roles = await listRoles();
			applyFilter();
		} catch (err) {
			error = err instanceof Error ? err.message : 'An error occurred';
		} finally {
			loading = false;
		}
	}

	function applyFilter() {
		const q = roleQuery.trim().toLowerCase();
		filteredRoles = q
			? roles.filter(
					(r) =>
						r.name.toLowerCase().includes(q) ||
						(r.description ?? '').toLowerCase().includes(q)
				)
			: roles;
	}

	function scheduleSearch() {
		if (searchTimer) clearTimeout(searchTimer);
		searchTimer = setTimeout(applyFilter, 200);
	}

	$: if (roleQuery !== undefined) scheduleSearch();

	function goCreate() {
		goto(resolve('/roles/new'));
	}

	async function handleRoleUpdated(updatedRole: Role) {
		roles = roles.map((r) => (r.id === updatedRole.id ? updatedRole : r));
		editingRoleId = null;
		applyFilter();
	}

	async function handleRoleDeleted(roleId: string) {
		roles = roles.filter((r) => r.id !== roleId);
		applyFilter();
	}

	onMount(fetchRoles);
</script>

<div
	class="bg-earthy-brown-50 dark:bg-gray-900 rounded-lg border border-gray-200 dark:border-gray-700"
>
	<div class="p-6">
		<div class="flex justify-between items-center mb-6">
			<div class="flex-1 max-w-md">
				<input
					type="text"
					placeholder="Search roles..."
					bind:value={roleQuery}
					class="w-full px-4 py-2 rounded-md border border-gray-300 dark:border-gray-600 focus:ring-2 focus:ring-earthy-terracotta-600 dark:focus:ring-earthy-terracotta-600 focus:border-transparent"
				/>
			</div>
			<button
				class="ml-4 px-4 py-2 bg-earthy-terracotta-700 dark:bg-earthy-terracotta-700 text-white rounded-md hover:bg-earthy-terracotta-800 dark:hover:bg-earthy-terracotta-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-earthy-terracotta-600 dark:focus:ring-earthy-terracotta-600"
				on:click={goCreate}
			>
				Add Role
			</button>
		</div>

		{#if loading && !roles.length}
			<div class="flex justify-center p-8">
				<div class="animate-spin rounded-full h-8 w-8 border-b-2 border-earthy-terracotta-700"></div>
			</div>
		{:else if error}
			<div class="bg-red-50 border border-red-200 rounded-lg p-4 text-red-700">
				{error}
			</div>
		{:else}
			<RoleTable
				roles={filteredRoles}
				{editingRoleId}
				onEdit={(roleId) => (editingRoleId = roleId)}
				onUpdate={handleRoleUpdated}
				onDelete={handleRoleDeleted}
			/>

			{#if filteredRoles.length === 0}
				<p class="text-sm text-gray-500 dark:text-gray-400 text-center py-8">
					{roleQuery ? 'No roles match your search' : 'No roles defined'}
				</p>
			{/if}
		{/if}
	</div>
</div>
