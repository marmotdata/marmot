<script lang="ts">
	import { toasts } from '$lib/stores/toast';
	import { deleteServiceAccount } from '$lib/serviceaccounts/api';
	import DeleteModal from '$components/ui/DeleteModal.svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import IconifyIcon from '@iconify/svelte';
	import type { ServiceAccount } from '$lib/serviceaccounts/types';

	export let accounts: ServiceAccount[] = [];
	export let onDelete: (id: string) => void;

	let showDeleteModal = false;
	let toDeleteAccount: ServiceAccount | null = null;

	function openDetail(id: string) {
		goto(resolve(`/service-accounts/${id}`));
	}

	async function handleDelete() {
		if (!toDeleteAccount) return;
		try {
			await deleteServiceAccount(toDeleteAccount.id);
			toasts.success(`Service account "${toDeleteAccount.name}" deleted`);
			onDelete(toDeleteAccount.id);
		} catch (err) {
			toasts.error(err instanceof Error ? err.message : 'Failed to delete service account');
		} finally {
			showDeleteModal = false;
			toDeleteAccount = null;
		}
	}
</script>

<div class="overflow-x-auto">
	<table class="min-w-full">
		<thead>
			<tr>
				<th
					class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider bg-earthy-brown-100 dark:bg-gray-800"
					>Name</th
				>
				<th
					class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider bg-earthy-brown-100 dark:bg-gray-800"
					>Description</th
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
			{#each accounts as sa (sa.id)}
				<tr
					class="hover:bg-earthy-brown-100 dark:hover:bg-gray-800 transition-colors cursor-pointer"
					on:click={() => openDetail(sa.id)}
				>
					<td class="px-6 py-4 whitespace-nowrap">
						<div class="flex items-center gap-2">
							<IconifyIcon
								icon="material-symbols:smart-toy-outline"
								class="h-4 w-4 text-gray-400 dark:text-gray-500 shrink-0"
							/>
							<span class="text-sm font-medium text-gray-900 dark:text-gray-100">{sa.name}</span>
						</div>
					</td>
					<td class="px-6 py-4 text-sm text-gray-600 dark:text-gray-400 max-w-md truncate">
						{sa.description || '—'}
					</td>
					<td class="px-6 py-4 whitespace-nowrap">
						<div class="flex flex-wrap gap-1">
							{#each sa.roles ?? [] as role (role.id)}
								<span
									class="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium bg-earthy-terracotta-100 dark:bg-earthy-terracotta-900 text-earthy-terracotta-700 dark:text-earthy-terracotta-100"
								>
									{role.name}
								</span>
							{/each}
							{#if !sa.roles?.length}
								<span class="text-xs text-gray-400">—</span>
							{/if}
						</div>
					</td>
					<td class="px-6 py-4 whitespace-nowrap">
						<span
							class="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium
								{sa.active
								? 'bg-green-100 dark:bg-green-900 text-green-700 dark:text-green-300'
								: 'bg-gray-100 dark:bg-gray-700 text-gray-500 dark:text-gray-400'}"
						>
							{sa.active ? 'Active' : 'Inactive'}
						</span>
					</td>
					<td
						class="px-6 py-4 whitespace-nowrap text-right text-sm font-medium"
						on:click|stopPropagation
						role="cell"
					>
						<button
							type="button"
							class="text-earthy-terracotta-700 hover:text-earthy-terracotta-800 dark:text-earthy-terracotta-500 dark:hover:text-earthy-terracotta-400 mr-3"
							on:click={() => openDetail(sa.id)}
						>
							Open
						</button>
						<button
							type="button"
							class="text-red-600 hover:text-red-900 dark:text-red-400 dark:hover:text-red-300"
							on:click={() => {
								toDeleteAccount = sa;
								showDeleteModal = true;
							}}
						>
							Delete
						</button>
					</td>
				</tr>
			{/each}
		</tbody>
	</table>
</div>

<DeleteModal
	show={showDeleteModal}
	title="Delete Service Account"
	message="Are you sure you want to delete this service account? All associated API keys will be revoked."
	confirmText="Delete"
	resourceName={toDeleteAccount?.name || ''}
	requireConfirmation={true}
	onConfirm={handleDelete}
	onCancel={() => {
		showDeleteModal = false;
		toDeleteAccount = null;
	}}
/>
