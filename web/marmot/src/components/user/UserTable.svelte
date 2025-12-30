<script lang="ts">
	import { fetchApi } from '$lib/api';
	import { auth } from '$lib/stores/auth';
	import { toasts, handleApiError } from '$lib/stores/toast';
	import EditUserForm from './EditUserForm.svelte';
	import DeleteModal from '$components/ui/DeleteModal.svelte';
	import { Lock, User as UserIcon, Mail } from 'lucide-svelte';

	export let users = [];
	export let editingUserId = null;
	export let onEdit;
	export let onUpdate;
	export let onDelete;

	let showDeleteModal = false;
	let userToDelete = null;

	$: currentUserId = auth.getCurrentUserId();

	async function handleDelete() {
		if (!userToDelete) return;

		try {
			const response = await fetchApi(`/users/${userToDelete.id}`, {
				method: 'DELETE'
			});
			if (!response.ok) {
				const errorMsg = await handleApiError(response);
				toasts.error(errorMsg);
				return;
			}
			toasts.success(`User "${userToDelete.username}" deleted successfully`);
			onDelete(userToDelete.id);
			showDeleteModal = false;
			userToDelete = null;
		} catch (err) {
			toasts.error(err instanceof Error ? err.message : 'Failed to delete user');
		}
	}

	function getProviderDisplay(provider: string): string {
		const providerMap: Record<string, string> = {
			google: 'Google',
			github: 'GitHub',
			gitlab: 'GitLab',
			okta: 'Okta',
			slack: 'Slack',
			auth0: 'Auth0'
		};
		return providerMap[provider] || provider.charAt(0).toUpperCase() + provider.slice(1);
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
					>Auth</th
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
						<td colspan="6">
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
							{#if user.identities && user.identities.length > 0}
								<div class="flex flex-wrap gap-1">
									{#each user.identities as identity}
										<span
											class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200"
										>
											<Lock class="h-3 w-3 mr-1" />
											{getProviderDisplay(identity.provider)}
										</span>
									{/each}
								</div>
							{:else}
								<span
									class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-200"
								>
									<Mail class="h-3 w-3 mr-1" />
									Password
								</span>
							{/if}
						</td>
						<td class="px-6 py-4 whitespace-nowrap">
							<div class="flex flex-wrap gap-1">
								{#each user.roles as role}
									<span
										class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-earthy-terracotta-100 dark:bg-earthy-terracotta-900 text-earthy-terracotta-700 dark:text-earthy-terracotta-100"
										>{role.name}</span
									>
								{/each}
							</div>
						</td>
						<td class="px-6 py-4 whitespace-nowrap">
							<span
								class={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${user.active ? 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200' : 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200'}`}
							>
								{user.active ? 'Active' : 'Inactive'}
							</span>
						</td>
						<td class="px-6 py-4 whitespace-nowrap text-right text-sm font-medium">
							{#if currentUserId !== user.id}
								<button
									type="button"
									class="text-earthy-terracotta-700 hover:text-earthy-terracotta-800 dark:text-earthy-terracotta-500 dark:hover:text-earthy-terracotta-400 mr-3"
									on:click={() => onEdit(user.id)}
								>
									Edit
								</button>
							{/if}
							{#if currentUserId !== user.id && user.username !== 'admin'}
								<button
									type="button"
									class="text-red-600 hover:text-red-900 dark:text-red-400 dark:hover:text-red-300"
									on:click={() => {
										userToDelete = user;
										showDeleteModal = true;
									}}
								>
									Delete
								</button>
							{/if}
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
