<script lang="ts">
	import { fetchApi } from '$lib/api';
	import { X } from 'lucide-svelte';

	export let teamId: string;
	export let onClose: () => void;
	export let onMemberAdded: () => void;

	let searchQuery = '';
	let searchResults: any[] = [];
	let searching = false;
	let selectedUserId = '';
	let selectedRole = 'member';
	let adding = false;
	let error: string | null = null;
	let searchTimer: ReturnType<typeof setTimeout>;

	$: {
		if (searchQuery.length >= 2) {
			if (searchTimer) clearTimeout(searchTimer);
			searchTimer = setTimeout(() => {
				searchUsers();
			}, 300);
		} else {
			searchResults = [];
		}
	}

	async function searchUsers() {
		try {
			searching = true;
			const response = await fetchApi(`/users?query=${encodeURIComponent(searchQuery)}&limit=20`);
			const data = await response.json();
			searchResults = data.users || [];
		} catch (err: any) {
			console.error('Failed to search users:', err);
		} finally {
			searching = false;
		}
	}

	async function addMember() {
		if (!selectedUserId) {
			error = 'Please select a user';
			return;
		}

		try {
			adding = true;
			error = null;

			const response = await fetchApi(`/teams/${teamId}/members`, {
				method: 'POST',
				body: JSON.stringify({
					user_id: selectedUserId,
					role: selectedRole
				})
			});

			if (response.ok) {
				onMemberAdded();
			} else {
				const data = await response.json();
				error = data.error || 'Failed to add member';
			}
		} catch (err: any) {
			error = err.message;
		} finally {
			adding = false;
		}
	}

	function selectUser(user: any) {
		selectedUserId = user.id;
		searchQuery = user.name;
		searchResults = [];
	}
</script>

<div class="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50" on:click={onClose}>
	<div class="bg-white dark:bg-gray-800 rounded-lg shadow-xl max-w-md w-full mx-4" on:click|stopPropagation>
		<!-- Header -->
		<div class="flex items-center justify-between p-6 border-b border-gray-200 dark:border-gray-700">
			<h3 class="text-lg font-semibold text-gray-900 dark:text-gray-100">
				Add Team Member
			</h3>
			<button
				on:click={onClose}
				class="text-gray-400 hover:text-gray-600 dark:hover:text-gray-300"
			>
				<X class="h-5 w-5" />
			</button>
		</div>

		<!-- Body -->
		<div class="p-6 space-y-4">
			<div>
				<label for="user-search" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
					Search User
				</label>
				<div class="relative">
					<input
						id="user-search"
						type="text"
						bind:value={searchQuery}
						placeholder="Type to search users..."
						class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:ring-earthy-terracotta-600 focus:border-earthy-terracotta-600 dark:bg-gray-700 dark:text-gray-100"
					/>
					{#if searching}
						<div class="absolute right-3 top-2.5">
							<div class="animate-spin rounded-full h-5 w-5 border-b-2 border-earthy-terracotta-700" />
						</div>
					{/if}
				</div>

				{#if searchResults.length > 0}
					<div class="mt-2 max-h-60 overflow-y-auto border border-gray-200 dark:border-gray-700 rounded-md">
						{#each searchResults as user}
							<button
								type="button"
								on:click={() => selectUser(user)}
								class="w-full text-left px-4 py-2 hover:bg-gray-100 dark:hover:bg-gray-700 border-b border-gray-200 dark:border-gray-700 last:border-b-0"
							>
								<div class="text-sm font-medium text-gray-900 dark:text-gray-100">
									{user.name}
								</div>
								<div class="text-xs text-gray-500 dark:text-gray-400">
									{user.username}
								</div>
							</button>
						{/each}
					</div>
				{/if}
			</div>

			<div>
				<label for="role" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
					Role
				</label>
				<select
					id="role"
					bind:value={selectedRole}
					class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:ring-earthy-terracotta-600 focus:border-earthy-terracotta-600 dark:bg-gray-700 dark:text-gray-100"
				>
					<option value="member">Member</option>
					<option value="owner">Owner</option>
				</select>
			</div>

			{#if error}
				<div class="bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-lg p-3 text-red-700 dark:text-red-400 text-sm">
					{error}
				</div>
			{/if}
		</div>

		<!-- Footer -->
		<div class="flex justify-end space-x-3 p-6 border-t border-gray-200 dark:border-gray-700">
			<button
				type="button"
				on:click={onClose}
				class="px-4 py-2 text-sm font-medium text-gray-700 dark:text-gray-300 bg-white dark:bg-gray-700 border border-gray-300 dark:border-gray-600 rounded-md hover:bg-gray-50 dark:hover:bg-gray-600 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-earthy-terracotta-600"
			>
				Cancel
			</button>
			<button
				type="button"
				on:click={addMember}
				disabled={!selectedUserId || adding}
				class="px-4 py-2 text-sm font-medium text-white bg-earthy-terracotta-700 dark:bg-earthy-terracotta-700 rounded-md hover:bg-earthy-terracotta-800 dark:hover:bg-earthy-terracotta-600 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-earthy-terracotta-600 disabled:opacity-50 disabled:cursor-not-allowed"
			>
				{adding ? 'Adding...' : 'Add Member'}
			</button>
		</div>
	</div>
</div>
