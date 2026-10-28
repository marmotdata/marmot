<script lang="ts">
	import IconifyIcon from '@iconify/svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { toasts } from '$lib/stores/toast';
	import { m } from '$lib/paraglide/messages';
	import { deleteServiceAccount } from '$lib/serviceaccounts/api';
	import { BTN_GHOST, TABLE, TBODY, TD, TH, TR } from '$lib/ui/classes';
	import Pill from '$components/ui/Pill.svelte';
	import DeleteModal from '$components/ui/DeleteModal.svelte';
	import type { ServiceAccount } from '$lib/serviceaccounts/types';

	let {
		accounts = [],
		canManage = true,
		onDelete
	}: {
		accounts?: ServiceAccount[];
		canManage?: boolean;
		onDelete: (id: string) => void;
	} = $props();

	let showDeleteModal = $state(false);
	let toDeleteAccount = $state<ServiceAccount | null>(null);

	function openDetail(id: string) {
		goto(resolve('/settings/service-accounts/[id]', { id }));
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
	<table class={TABLE}>
		<thead class="border-b border-gray-100 dark:border-gray-700/60">
			<tr>
				<th class="{TH} pl-5">{m.common_name()}</th>
				<th class={TH}>{m.common_description()}</th>
				<th class={TH}>{m.serviceaccounts_step_roles()}</th>
				<th class={TH}>{m.common_status()}</th>
				<th class="{TH} pr-5 text-right"><span class="sr-only">{m.common_actions()}</span></th>
			</tr>
		</thead>
		<tbody class={TBODY}>
			{#each accounts as sa (sa.id)}
				<tr class="{TR} cursor-pointer" onclick={() => openDetail(sa.id)}>
					<td class="{TD} whitespace-nowrap pl-5">
						<a
							href={resolve('/settings/service-accounts/[id]', { id: sa.id })}
							class="flex items-center gap-2.5 font-medium text-gray-900 hover:underline dark:text-gray-100"
							onclick={(e) => e.stopPropagation()}
						>
							<IconifyIcon
								icon="material-symbols:smart-toy-outline"
								class="h-4 w-4 shrink-0 text-gray-400"
							/>
							{sa.name}
						</a>
					</td>
					<td class="{TD} max-w-md truncate text-gray-500 dark:text-gray-400">
						{sa.description || '—'}
					</td>
					<td class={TD}>
						<span class="flex flex-wrap gap-1">
							{#each sa.roles ?? [] as role (role.id)}
								<Pill>{role.name}</Pill>
							{:else}
								<span class="text-gray-400 dark:text-gray-500">—</span>
							{/each}
						</span>
					</td>
					<td class={TD}>
						<Pill tone={sa.active ? 'success' : 'warning'}>
							{sa.active ? m.common_active() : m.common_inactive()}
						</Pill>
					</td>
					<td class="{TD} whitespace-nowrap pr-5 text-right">
						{#if canManage}
							<button
								type="button"
								class="{BTN_GHOST} hover:bg-red-50 hover:text-red-600 dark:hover:bg-red-900/20 dark:hover:text-red-400"
								onclick={(e) => {
									e.stopPropagation();
									toDeleteAccount = sa;
									showDeleteModal = true;
								}}
								aria-label={m.serviceaccounts_delete_aria({ name: sa.name })}
								title={m.common_delete()}
							>
								<IconifyIcon icon="material-symbols:delete-outline" class="h-4 w-4" />
							</button>
						{/if}
						<IconifyIcon
							icon="material-symbols:chevron-right"
							class="ml-1 inline h-4 w-4 align-middle text-gray-300 dark:text-gray-600"
						/>
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
