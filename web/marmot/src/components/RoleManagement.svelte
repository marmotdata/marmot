<script lang="ts">
	import { onMount } from 'svelte';
	import { fetchApi } from '$lib/api';

	let roles: any[] = [];
	let loading = false;
	let error: string | null = null;
	let editingRoleId: string | null = null;
	let creatingRole = false;

	// Temporary: Mock roles until backend is ready
	const mockRoles = [
		{
			id: '1',
			name: 'Admin',
			description: 'Full system access',
			permissions: []
		},
		{
			id: '2',
			name: 'User',
			description: 'Basic system access',
			permissions: []
		}
	];

	onMount(() => {
		roles = mockRoles;
	});

	async function createRole() {
		// TODO: Implement when backend is ready
		alert('Role creation not yet implemented');
	}

	async function updateRole() {
		// TODO: Implement when backend is ready
		alert('Role update not yet implemented');
	}

	async function deleteRole() {
		// TODO: Implement when backend is ready
		alert('Role deletion not yet implemented');
	}
</script>

<div class="bg-earthy-brown-50 dark:bg-gray-900 dark:bg-gray-900 rounded-lg border border-gray-200 dark:border-gray-700 dark:border-gray-700 dark:border-gray-700">
	<div class="p-6">
		<div class="flex justify-between items-center mb-6">
			<h3 class="text-lg font-medium text-gray-900 dark:text-gray-100 dark:text-gray-100 dark:text-gray-100 dark:text-gray-200">Roles & Permissions</h3>
			<button
				class="ml-4 px-4 py-2 bg-amber-700 dark:bg-amber-600 text-white rounded-md hover:bg-amber-800 dark:hover:bg-amber-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-amber-500 dark:focus:ring-amber-400"
				on:click={() => (creatingRole = !creatingRole)}
			>
				{creatingRole ? 'Cancel' : 'Add Role'}
			</button>
		</div>

		{#if creatingRole}
			<div class="mb-6 bg-earthy-brown-100 dark:bg-gray-800 dark:bg-gray-800 dark:bg-gray-900 rounded-lg p-6 animate-slide-down">
				<h4 class="text-base font-medium text-gray-900 dark:text-gray-100 dark:text-gray-100 dark:text-gray-200 mb-4">Create New Role</h4>
				<div class="space-y-4">
					<div>
						<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 dark:text-gray-300 dark:text-gray-300">Role Name</label>
						<input
							type="text"
							bind:value={newRole.name}
							class="mt-1 block w-full px-4 py-2 bg-white dark:bg-gray-800 dark:bg-gray-800 dark:bg-gray-900 rounded-md shadow-sm focus:ring-2 focus:ring-orange-500 dark:focus:ring-orange-400 focus:border-orange-500 dark:focus:border-orange-400 sm:text-sm border-gray-300 dark:border-gray-600 dark:border-gray-600 dark:border-gray-600"
						/>
					</div>
					<div>
						<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 dark:text-gray-300 dark:text-gray-300">Description</label>
						<textarea
							bind:value={newRole.description}
							class="mt-1 block w-full px-4 py-2 bg-white dark:bg-gray-800 dark:bg-gray-800 dark:bg-gray-900 rounded-md shadow-sm focus:ring-2 focus:ring-orange-500 dark:focus:ring-orange-400 focus:border-orange-500 dark:focus:border-orange-400 sm:text-sm border-gray-300 dark:border-gray-600 dark:border-gray-600 dark:border-gray-600"
						/>
					</div>
					<div class="flex justify-end">
						<button
							class="px-4 py-2 bg-amber-600 text-white rounded-md hover:bg-amber-700 dark:bg-amber-600 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-amber-500 dark:focus:ring-amber-400"
							on:click={createRole}
							disabled={loading}
						>
							Create Role
						</button>
					</div>
				</div>
			</div>
		{/if}

		{#if loading && !roles.length}
			<div class="flex justify-center p-8">
				<div class="animate-spin rounded-full h-8 w-8 border-b-2 border-orange-600"></div>
			</div>
		{:else if error}
			<div class="bg-red-50 border border-red-200 rounded-lg p-4 text-red-700">
				{error}
			</div>
		{:else}
			<div class="overflow-x-auto">
				<table class="min-w-full">
					<thead>
						<tr>
							<th
								class="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-500 dark:text-gray-500 uppercase tracking-wider bg-earthy-brown-100 dark:bg-gray-800 dark:bg-gray-800 dark:bg-gray-800 dark:bg-gray-900"
								>Role Name</th
							>
							<th
								class="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-500 dark:text-gray-500 uppercase tracking-wider bg-earthy-brown-100 dark:bg-gray-800 dark:bg-gray-800 dark:bg-gray-800 dark:bg-gray-900"
								>Description</th
							>
							<th
								class="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-500 dark:text-gray-500 uppercase tracking-wider bg-earthy-brown-100 dark:bg-gray-800 dark:bg-gray-800 dark:bg-gray-800 dark:bg-gray-900"
								>Users</th
							>
							<th
								class="px-6 py-3 text-right text-xs font-medium text-gray-500 dark:text-gray-500 dark:text-gray-500 uppercase tracking-wider bg-earthy-brown-100 dark:bg-gray-800 dark:bg-gray-800 dark:bg-gray-800 dark:bg-gray-900"
								>Actions</th
							>
						</tr>
					</thead>
					<tbody class="divide-y divide-earthy-brown-100 bg-earthy-brown-50 dark:bg-gray-900 dark:bg-gray-900 dark:bg-gray-900">
						{#each roles as role}
							<tr class="hover:bg-earthy-brown-100 dark:bg-gray-800 dark:bg-gray-800 dark:bg-gray-900 transition-colors">
								{#if editingRoleId === role.id}
									<td colspan="4">
										<div class="bg-earthy-brown-100 dark:bg-gray-800 dark:bg-gray-800 dark:bg-gray-900 rounded-lg p-4 animate-slide-down">
											<div class="space-y-4">
												<div>
													<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 dark:text-gray-300 dark:text-gray-300">Role Name</label>
													<input
														type="text"
														bind:value={role.name}
														class="mt-1 block w-full px-4 py-2 bg-white dark:bg-gray-800 dark:bg-gray-800 dark:bg-gray-900 rounded-md shadow-sm focus:ring-2 focus:ring-orange-500 dark:focus:ring-orange-400 focus:border-orange-500 dark:focus:border-orange-400 sm:text-sm border-gray-300 dark:border-gray-600 dark:border-gray-600 dark:border-gray-600"
													/>
												</div>
												<div>
													<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 dark:text-gray-300 dark:text-gray-300">Description</label>
													<textarea
														bind:value={role.description}
														class="mt-1 block w-full px-4 py-2 bg-white dark:bg-gray-800 dark:bg-gray-800 dark:bg-gray-900 rounded-md shadow-sm focus:ring-2 focus:ring-orange-500 dark:focus:ring-orange-400 focus:border-orange-500 dark:focus:border-orange-400 sm:text-sm border-gray-300 dark:border-gray-600 dark:border-gray-600 dark:border-gray-600"
													/>
												</div>
												<div class="flex justify-end space-x-3">
													<button
														class="px-4 py-2 bg-white dark:bg-gray-800 dark:bg-gray-800 border border-gray-300 dark:border-gray-600 dark:border-gray-600 text-gray-700 dark:text-gray-300 dark:text-gray-300 rounded-md hover:bg-gray-50 dark:hover:bg-gray-700 dark:bg-gray-800 dark:bg-gray-900 dark:hover:bg-gray-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-orange-500 dark:focus:ring-orange-400"
														on:click={() => (editingRoleId = null)}
													>
														Cancel
													</button>
													<button
														class="px-4 py-2 bg-amber-600 text-white rounded-md hover:bg-amber-700 dark:bg-amber-600 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-amber-500 dark:focus:ring-amber-400"
														on:click={() => updateRole(role)}
													>
														Save Changes
													</button>
												</div>
											</div>
										</div>
									</td>
								{:else}
									<td class="px-6 py-4 whitespace-nowrap text-sm font-medium text-gray-900 dark:text-gray-100 dark:text-gray-100 dark:text-gray-100 dark:text-gray-200"
										>{role.name}</td
									>
									<td class="px-6 py-4 text-sm text-gray-600 dark:text-gray-400 dark:text-gray-400 dark:text-gray-400">{role.description}</td>
									<td class="px-6 py-4 whitespace-nowrap text-sm text-gray-600 dark:text-gray-400 dark:text-gray-400 dark:text-gray-400"
										>{role.users?.length || 0} users</td
									>
									<td class="px-6 py-4 whitespace-nowrap text-right text-sm font-medium">
										<button
											class="text-amber-600 hover:text-amber-900 mr-3"
											on:click={() => (editingRoleId = role.id)}
										>
											Edit
										</button>
										<button
											class="text-red-600 hover:text-red-900"
											on:click={() => deleteRole(role.id)}
										>
											Delete
										</button>
									</td>
								{/if}
							</tr>
						{/each}
					</tbody>
				</table>
			</div>
		{/if}
	</div>
</div>

<style>
	.animate-slide-down {
		animation: slideDown 0.2s ease-out;
	}

	@keyframes slideDown {
		from {
			opacity: 0;
			transform: translateY(-10px);
		}
		to {
			opacity: 1;
			transform: translateY(0);
		}
	}
</style>
