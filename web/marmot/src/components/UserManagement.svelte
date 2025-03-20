<script lang="ts">
	import { onMount } from 'svelte';
	import { fetchApi } from '$lib/api';
	import CreateUserForm from './CreateUserForm.svelte';
	import UserTable from './UserTable.svelte';

	let users: any[] = [];
	let totalUsers = 0;
	let offset = 0;
	let limit = 10;
	let userQuery = '';
	let creatingUser = false;
	let editingUserId: string | null = null;
	let loading = false;
	let error: string | null = null;
	let searchTimer: ReturnType<typeof setTimeout>;

	async function handleUserCreated() {
		creatingUser = false;
		await fetchUsers();
	}

	async function fetchUsers() {
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
		} catch (err: any) {
			error = err.message;
		} finally {
			loading = false;
		}
	}

	onMount(fetchUsers);

	$: {
		if (userQuery !== undefined) {
			if (searchTimer) clearTimeout(searchTimer);
			searchTimer = setTimeout(() => {
				offset = 0;
				fetchUsers();
			}, 300);
		}
	}

	async function handleUserUpdated(updatedUser: any) {
		users = users.map((u) => (u.id === updatedUser.id ? updatedUser : u));
		editingUserId = null;
		await fetchUsers();
	}

	async function handleUserDeleted(userId: string) {
		users = users.filter((u) => u.id !== userId);
		await fetchUsers();
	}
</script>

<div class="bg-earthy-brown-50 dark:bg-gray-900 rounded-lg border border-gray-200 dark:border-gray-700">
	<div class="p-6">
		<div class="flex justify-between items-center mb-6">
			<div class="flex-1 max-w-md">
				<input
					type="text"
					placeholder="Search users..."
					bind:value={userQuery}
					class="w-full px-4 py-2 rounded-md border border-gray-300 dark:border-gray-600 focus:ring-2 focus:ring-orange-500 dark:focus:ring-orange-400 focus:border-transparent"
				/>
			</div>
			<button
				class="ml-4 px-4 py-2 bg-amber-700 dark:bg-amber-600 text-white rounded-md hover:bg-amber-800 dark:hover:bg-amber-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-amber-500 dark:focus:ring-amber-400"
				on:click={() => (creatingUser = !creatingUser)}
			>
				{creatingUser ? 'Cancel' : 'Add User'}
			</button>
		</div>

		{#if creatingUser}
			<CreateUserForm onUserCreated={handleUserCreated} />
		{/if}

		{#if loading && !users.length}
			<div class="flex justify-center p-8">
				<div class="animate-spin rounded-full h-8 w-8 border-b-2 border-orange-600" />
			</div>
		{:else if error}
			<div class="bg-red-50 border border-red-200 rounded-lg p-4 text-red-700">
				{error}
			</div>
		{:else}
			<UserTable
				users={users}
				editingUserId={editingUserId}
				onEdit={(userId) => (editingUserId = userId)}
				onUpdate={handleUserUpdated}
				onDelete={handleUserDeleted}
			/>

			<div class="mt-4 flex items-center justify-between">
				<div class="flex-1 flex justify-between items-center">
					<p class="text-sm text-gray-700 dark:text-gray-300">
						Showing {offset + 1} to {Math.min(offset + users.length, totalUsers)} of {totalUsers} users
					</p>
					<div class="flex space-x-2">
						<button
							class="px-3 py-1 border border-gray-300 dark:border-gray-600 rounded-md text-sm disabled:opacity-50"
							disabled={offset === 0}
							on:click={() => (offset = Math.max(0, offset - limit))}
						>
							Previous
						</button>
						<button
							class="px-3 py-1 border border-gray-300 dark:border-gray-600 rounded-md text-sm disabled:opacity-50"
							disabled={offset + users.length >= totalUsers}
							on:click={() => (offset = offset + limit)}
						>
							Next
						</button>
					</div>
				</div>
			</div>
		{/if}
	</div>
</div>
