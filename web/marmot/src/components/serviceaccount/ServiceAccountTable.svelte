<script lang="ts">
	import { toasts } from '$lib/stores/toast';
	import { m } from '$lib/paraglide/messages';
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
			toasts.success(m.serviceaccounts_deleted_success({ name: toDeleteAccount.name }));
			onDelete(toDeleteAccount.id);
		} catch (err) {
			toasts.error(err instanceof Error ? err.message : m.serviceaccounts_error_delete_account());
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
					>{m.common_name()}</th
				>
				<th
					class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider bg-earthy-brown-100 dark:bg-gray-800"
					>{m.common_description()}</th
				>
				<th
					class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider bg-earthy-brown-100 dark:bg-gray-800"
					>{m.serviceaccounts_step_roles()}</th
				>
				<th
					class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider bg-earthy-brown-100 dark:bg-gray-800"
					>{m.common_status()}</th
				>
				<th
					class="px-6 py-3 text-right text-xs font-medium text-gray-500 uppercase tracking-wider bg-earthy-brown-100 dark:bg-gray-800"
					>{m.common_actions()}</th
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
							{sa.active ? m.common_active() : m.common_inactive()}
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
							{m.serviceaccounts_open_button()}
						</button>
						<button
							type="button"
							class="text-red-600 hover:text-red-900 dark:text-red-400 dark:hover:text-red-300"
							on:click={() => {
								toDeleteAccount = sa;
								showDeleteModal = true;
							}}
						>
							{m.common_delete()}
						</button>
					</td>
				</tr>
			{/each}
		</tbody>
	</table>
</div>

<DeleteModal
	show={showDeleteModal}
	title={m.serviceaccounts_delete_title()}
	message={m.serviceaccounts_delete_confirm()}
	confirmText={m.common_delete()}
	resourceName={toDeleteAccount?.name || ''}
	requireConfirmation={true}
	onConfirm={handleDelete}
	onCancel={() => {
		showDeleteModal = false;
		toDeleteAccount = null;
	}}
/>
