<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { fetchApi } from '$lib/api';
	import { m } from '$lib/paraglide/messages';
	import UserTable from './UserTable.svelte';
	import Pagination from '$components/ui/Pagination.svelte';
	import type { User } from '$lib/users/types';

	let users: User[] = [];
	let totalUsers = 0;
	let offset = 0;
	let limit = 10;
	let userQuery = '';
	let editingUserId: string | null = null;
	let loading = false;
	let error: string | null = null;
	let searchTimer: ReturnType<typeof setTimeout>;

	function goCreate() {
		goto(resolve('/users/new'));
	}

	async function fetchUsers(stepBack = true) {
		try {
			loading = true;
			const params = new URLSearchParams({
				limit: limit.toString(),
				offset: offset.toString(),
				...(userQuery && { query: userQuery })
			});

			const response = await fetchApi(`/users?${params}`);
			const data = await response.json();
			users = data.users;
			totalUsers = data.total;
			// Past the last page, e.g. after deleting its only user: show the last page, once.
			if (stepBack && users.length === 0 && offset > 0 && totalUsers > 0) {
				offset = Math.floor((totalUsers - 1) / limit) * limit;
				await fetchUsers(false);
			}
		} catch (err) {
			error = err instanceof Error ? err.message : m.users_error_generic();
		} finally {
			loading = false;
		}
	}

	onMount(fetchUsers);

	function scheduleSearch() {
		if (searchTimer) clearTimeout(searchTimer);
		searchTimer = setTimeout(() => {
			offset = 0;
			fetchUsers();
		}, 300);
	}

	$: if (userQuery !== undefined) scheduleSearch();

	async function handleUserUpdated(updatedUser: User) {
		users = users.map((u) => (u.id === updatedUser.id ? updatedUser : u));
		editingUserId = null;
		await fetchUsers();
	}

	async function handleUserDeleted(userId: string) {
		users = users.filter((u) => u.id !== userId);
		await fetchUsers();
	}
</script>

<div
	class="bg-earthy-brown-50 dark:bg-gray-900 rounded-lg border border-gray-200 dark:border-gray-700"
>
	<div class="p-6">
		<div class="flex justify-between items-center mb-6">
			<div class="flex-1 max-w-md">
				<input
					type="text"
					placeholder={m.users_search_placeholder()}
					bind:value={userQuery}
					class="w-full px-4 py-2 rounded-md border border-gray-300 dark:border-gray-600 focus:ring-2 focus:ring-earthy-terracotta-600 dark:focus:ring-earthy-terracotta-600 focus:border-transparent"
				/>
			</div>
			<button
				class="ml-4 px-4 py-2 bg-earthy-terracotta-700 dark:bg-earthy-terracotta-700 text-white rounded-md hover:bg-earthy-terracotta-800 dark:hover:bg-earthy-terracotta-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-earthy-terracotta-600 dark:focus:ring-earthy-terracotta-600"
				on:click={goCreate}
			>
				{m.users_add_button()}
			</button>
		</div>

		{#if loading && !users.length}
			<div class="flex justify-center p-8">
				<div class="animate-spin rounded-full h-8 w-8 border-b-2 border-earthy-terracotta-700" />
			</div>
		{:else if error}
			<div class="bg-red-50 border border-red-200 rounded-lg p-4 text-red-700">
				{error}
			</div>
		{:else}
			<UserTable
				{users}
				{editingUserId}
				onEdit={(userId) => (editingUserId = userId)}
				onUpdate={handleUserUpdated}
				onDelete={handleUserDeleted}
			/>

			<div class="mt-4">
				<Pagination
					page={offset / limit + 1}
					pageSize={limit}
					total={totalUsers}
					disabled={loading}
					summary={m.users_pagination_showing}
					onChange={(page) => {
						offset = (page - 1) * limit;
						fetchUsers();
					}}
				/>
			</div>
		{/if}
	</div>
</div>
