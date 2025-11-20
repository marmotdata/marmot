<script lang="ts">
	import { fetchApi } from '$lib/api';

	let { user, onCancel, onUpdate } = $props<{
		user: any;
		onCancel: () => void;
		onUpdate: (updatedUser: any) => void;
	}>();
	
	let loading = false;
	let error: string | null = null;
	let editedUser = { ...user };

	async function updateUser() {
		try {
			loading = true;
			const response = await fetchApi(`/users/${user.id}`, {
				method: 'PUT',
				body: JSON.stringify({
					name: editedUser.name,
					active: editedUser.active,
					role_names: editedUser.roles.map((r: any) => r.name)
				})
			});

			if (!response.ok) {
				throw new Error('Failed to update user');
			}

			const updatedUser = await response.json();
			onUpdate(updatedUser);
		} catch (err: any) {
			error = err.message;
		} finally {
			loading = false;
		}
	}
</script>

<div class="bg-earthy-brown-100 dark:bg-gray-800 dark:bg-gray-800 dark:bg-gray-900 rounded-lg p-4 animate-slide-down">
	{#if error}
		<div class="mb-4 text-red-600">{error}</div>
	{/if}
	<div class="space-y-4">
		<div>
			<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 dark:text-gray-300 dark:text-gray-300">Name</label>
			<input
				type="text"
				bind:value={editedUser.name}
				class="mt-1 block w-full px-4 py-2 bg-white dark:bg-gray-800 dark:bg-gray-800 dark:bg-gray-900 rounded-md shadow-sm focus:ring-2 focus:ring-earthy-terracotta-600 dark:focus:ring-earthy-terracotta-600 focus:border-earthy-terracotta-700 dark:focus:border-earthy-terracotta-500 sm:text-sm border-gray-300 dark:border-gray-600 dark:border-gray-600 dark:border-gray-600"
			/>
		</div>
		<div>
			<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 dark:text-gray-300 dark:text-gray-300">Status</label>
			<select
				bind:value={editedUser.active}
				class="mt-1 block w-full px-4 py-2 bg-white dark:bg-gray-800 dark:bg-gray-800 dark:bg-gray-900 rounded-md shadow-sm focus:ring-2 focus:ring-earthy-terracotta-600 dark:focus:ring-earthy-terracotta-600 focus:border-earthy-terracotta-700 dark:focus:border-earthy-terracotta-500 sm:text-sm border-gray-300 dark:border-gray-600 dark:border-gray-600 dark:border-gray-600"
			>
				<option value={true}>Active</option>
				<option value={false}>Inactive</option>
			</select>
		</div>
		<div>
			<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 dark:text-gray-300 dark:text-gray-300">Roles</label>
			<div class="mt-2 space-y-2">
				{#each user.roles as role}
					<label class="inline-flex items-center">
						<input
							type="checkbox"
							checked={editedUser.roles.some((r: any) => r.name === role.name)}
							on:change={(e) => {
								if (e.target.checked) {
									editedUser.roles = [...editedUser.roles, role];
								} else {
									editedUser.roles = editedUser.roles.filter((r: any) => r.name !== role.name);
								}
							}}
							class="rounded border-gray-300 dark:border-gray-600 dark:border-gray-600 text-earthy-terracotta-700 focus:ring-earthy-terracotta-600 dark:focus:ring-earthy-terracotta-600"
						/>
						<span class="ml-2 text-sm text-gray-700 dark:text-gray-300 dark:text-gray-300 dark:text-gray-300">{role.name}</span>
					</label>
				{/each}
			</div>
		</div>
		<div class="flex justify-end space-x-3">
			<button
				type="button"
				class="px-4 py-2 bg-white dark:bg-gray-800 dark:bg-gray-800 border border-gray-300 dark:border-gray-600 dark:border-gray-600 text-gray-700 dark:text-gray-300 dark:text-gray-300 rounded-md hover:bg-gray-50 dark:hover:bg-gray-700 dark:bg-gray-800 dark:bg-gray-900 dark:hover:bg-gray-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-earthy-terracotta-600 dark:focus:ring-earthy-terracotta-600"
				on:click={onCancel}
			>
				Cancel
			</button>
			<button
				type="button"
				class="px-4 py-2 bg-earthy-terracotta-700 text-white rounded-md hover:bg-earthy-terracotta-700 dark:bg-earthy-terracotta-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-earthy-terracotta-600 dark:focus:ring-earthy-terracotta-600"
				on:click={updateUser}
				disabled={loading}
			>
				{#if loading}
					<div class="animate-spin rounded-full h-4 w-4 border-b-2 border-white mr-2"></div>
				{/if}
				Save Changes
			</button>
		</div>
	</div>
</div>
