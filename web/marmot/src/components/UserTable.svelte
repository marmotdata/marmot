<script lang="ts">
	import { fetchApi } from '$lib/api';
	import EditUserForm from './EditUserForm.svelte';
	import DeleteModal from './DeleteModal.svelte';

	export let users = [];
	export let editingUserId = null;
	export let onEdit;
	export let onUpdate;
	export let onDelete;

	let showDeleteModal = false;
	let userToDelete = null;

	async function handleDelete() {
		if (!userToDelete) return;

		try {
			await fetchApi(`/users/${userToDelete.id}`, {
				method: 'DELETE'
			});
			onDelete(userToDelete.id);
			showDeleteModal = false;
			userToDelete = null;
		} catch (err) {
			console.error('Failed to delete user:', err);
		}
	}
</script>

<div class="overflow-x-auto">
	<table class="min-w-full">
		<thead>
			<tr>
				<th
					class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider bg-earthy-brown-100 dark:bg-gray-800"
					>Username</th
				>
				<th
					class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider bg-earthy-brown-100 dark:bg-gray-800"
					>Name</th
				>
				<th
					class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider bg-earthy-brown-100 dark:bg-gray-800"
					>Roles</th
				>
				<th
					class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider bg-earthy-brown-100 dark:bg-gray-800"
					>Status</th
				>
				<th
					class="px-6 py-3 text-right text-xs font-medium text-gray-500 uppercase tracking-wider bg-earthy-brown-100 dark:bg-gray-800"
					>Actions</th
				>
			</tr>
		</thead>
		<tbody class="divide-y divide-earthy-brown-100 bg-earthy-brown-50 dark:bg-gray-900">
			{#each users as user}
				<tr class="hover:bg-earthy-brown-100 dark:hover:bg-gray-800 transition-colors">
					{#if editingUserId === user.id}
						<td colspan="5">
							<EditUserForm
								{user}
								onCancel={() => onEdit(null)}
								onUpdate={(updatedUser) => onUpdate(updatedUser)}
							/>
						</td>
					{:else}
						<td
							class="px-6 py-4 whitespace-nowrap text-sm font-medium text-gray-900 dark:text-gray-100"
							>{user.username}</td
						>
						<td class="px-6 py-4 whitespace-nowrap text-sm text-gray-600 dark:text-gray-400"
							>{user.name}</td
						>
						<td class="px-6 py-4 whitespace-nowrap">
							<div class="flex flex-wrap gap-1">
								{#each user.roles as role}
									<span
										class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-orange-100 dark:bg-orange-900 text-orange-800 dark:text-orange-100"
										>{role.name}</span
									>
								{/each}
							</div>
						</td>
						<td class="px-6 py-4 whitespace-nowrap">
							<span
								class={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${user.active ? 'bg-green-100 text-green-800' : 'bg-red-100 text-red-800'}`}
							>
								{user.active ? 'Active' : 'Inactive'}
							</span>
						</td>
						<td class="px-6 py-4 whitespace-nowrap text-right text-sm font-medium">
							<button
								type="button"
								class="text-amber-600 hover:text-amber-900 mr-3"
								on:click={() => onEdit(user.id)}
							>
								Edit
							</button>
							<button
								type="button"
								class="text-red-600 hover:text-red-900"
								on:click={() => {
									userToDelete = user;
									showDeleteModal = true;
								}}
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

<DeleteModal
	show={showDeleteModal}
	title="Delete User"
	message="Are you sure you want to delete this user? This action cannot be undone."
	confirmText="Delete"
	resourceName={userToDelete?.username || ''}
	requireConfirmation={true}
	onConfirm={handleDelete}
	onCancel={() => {
		showDeleteModal = false;
		userToDelete = null;
	}}
/>
