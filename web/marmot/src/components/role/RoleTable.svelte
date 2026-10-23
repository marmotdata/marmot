<script lang="ts">
	import IconifyIcon from '@iconify/svelte';
	import { toasts } from '$lib/stores/toast';
	import { m } from '$lib/paraglide/messages';
	import { deleteRole } from '$lib/roles/api';
	import { BTN_GHOST, TABLE, TBODY, TD, TH } from '$lib/ui/classes';
	import Pill from '$components/ui/Pill.svelte';
	import DeleteModal from '$components/ui/DeleteModal.svelte';
	import EditRoleForm from './EditRoleForm.svelte';
	import ViewRoleForm from './ViewRoleForm.svelte';
	import type { Role } from '$lib/roles/types';

	let {
		roles = [],
		editingRoleId = null,
		canManage = true,
		onEdit,
		onUpdate,
		onDelete,
		onDuplicate
	}: {
		roles?: Role[];
		editingRoleId?: string | null;
		canManage?: boolean;
		onEdit: (roleId: string | null) => void;
		onUpdate: (updatedRole: Role) => void;
		onDelete: (roleId: string) => void;
		onDuplicate?: (role: Role) => void;
	} = $props();

	let viewingRoleId = $state<string | null>(null);
	let showDeleteModal = $state(false);
	let roleToDelete = $state<Role | null>(null);

	function toggleView(roleId: string) {
		viewingRoleId = viewingRoleId === roleId ? null : roleId;
		if (viewingRoleId) onEdit(null);
	}

	function switchToEdit(roleId: string) {
		viewingRoleId = null;
		onEdit(roleId);
	}

	function closeAll() {
		viewingRoleId = null;
		onEdit(null);
	}

	async function handleDelete() {
		if (!roleToDelete) return;
		try {
			await deleteRole(roleToDelete.id);
			toasts.success(m.roles_delete_success({ name: roleToDelete.name }));
			onDelete(roleToDelete.id);
		} catch (err) {
			toasts.error(err instanceof Error ? err.message : m.roles_error_delete());
		} finally {
			showDeleteModal = false;
			roleToDelete = null;
		}
	}
</script>

<div class="overflow-x-auto">
	<table class={TABLE}>
		<thead class="border-b border-gray-100 dark:border-gray-700/60">
			<tr>
				<th class="{TH} w-8 pl-5 pr-0"><span class="sr-only">{m.roles_expand_aria()}</span></th>
				<th class={TH}>{m.common_name()}</th>
				<th class={TH}>{m.common_description()}</th>
				<th class="{TH} text-right">{m.roles_header_permissions()}</th>
				<th class="{TH} text-right">{m.roles_header_users()}</th>
				<th class="{TH} pr-5 text-right"><span class="sr-only">{m.common_actions()}</span></th>
			</tr>
		</thead>
		<tbody class={TBODY}>
			{#each roles as role (role.id)}
				{@const isEditing = editingRoleId === role.id}
				{@const isViewing = viewingRoleId === role.id}
				{@const isExpanded = isEditing || isViewing}
				{@const isSystem = role.is_system}

				<tr
					class="cursor-pointer transition-colors {isExpanded
						? 'bg-gray-50 dark:bg-gray-700/40'
						: 'hover:bg-gray-50 dark:hover:bg-gray-700/40'}"
					onclick={() => toggleView(role.id)}
				>
					<td class="{TD} w-8 pl-5 pr-0 text-gray-400">
						<button
							type="button"
							class="flex items-center"
							aria-expanded={isExpanded}
							aria-label={m.roles_expand_aria()}
							onclick={(e) => {
								e.stopPropagation();
								toggleView(role.id);
							}}
						>
							<IconifyIcon
								icon="material-symbols:chevron-right"
								class="h-4 w-4 transition-transform {isExpanded ? 'rotate-90' : ''}"
							/>
						</button>
					</td>
					<td class="{TD} whitespace-nowrap">
						<span class="flex items-center gap-2">
							<span class="font-medium text-gray-900 dark:text-gray-100">{role.name}</span>
							{#if isSystem}
								<Pill>
									<IconifyIcon icon="material-symbols:lock-outline" class="h-3 w-3" />
									{m.ui_role_system_badge()}
								</Pill>
							{/if}
						</span>
					</td>
					<td class="{TD} max-w-md truncate text-gray-500 dark:text-gray-400">
						{role.description || '—'}
					</td>
					<td class="{TD} text-right tabular-nums">{(role.permissions ?? []).length}</td>
					<td class="{TD} text-right tabular-nums">{role.user_count ?? 0}</td>
					<td class="{TD} whitespace-nowrap pr-5 text-right">
						{#if !isSystem && canManage}
							<button
								type="button"
								class={BTN_GHOST}
								onclick={(e) => {
									e.stopPropagation();
									switchToEdit(role.id);
								}}
								aria-label={m.roles_edit_aria({ name: role.name })}
								title={m.common_edit()}
							>
								<IconifyIcon icon="material-symbols:edit-outline" class="h-4 w-4" />
							</button>
							<button
								type="button"
								class="{BTN_GHOST} hover:bg-red-50 hover:text-red-600 dark:hover:bg-red-900/20 dark:hover:text-red-400"
								onclick={(e) => {
									e.stopPropagation();
									roleToDelete = role;
									showDeleteModal = true;
								}}
								aria-label={m.roles_delete_aria({ name: role.name })}
								title={m.common_delete()}
							>
								<IconifyIcon icon="material-symbols:delete-outline" class="h-4 w-4" />
							</button>
						{/if}
					</td>
				</tr>

				{#if isExpanded}
					<tr>
						<td colspan="6" class="bg-gray-50/60 p-0 dark:bg-gray-900/30">
							{#if isEditing}
								<EditRoleForm
									{role}
									onCancel={closeAll}
									onUpdate={(updatedRole) => {
										onUpdate(updatedRole);
										closeAll();
									}}
								/>
							{:else}
								<ViewRoleForm
									{role}
									onClose={closeAll}
									onEdit={isSystem || !canManage ? null : () => switchToEdit(role.id)}
									onDuplicate={canManage && onDuplicate ? () => onDuplicate(role) : null}
								/>
							{/if}
						</td>
					</tr>
				{/if}
			{/each}
		</tbody>
	</table>
</div>

<DeleteModal
	show={showDeleteModal}
	title={m.roles_delete_title()}
	message={roleToDelete && (roleToDelete.user_count ?? 0) > 0
		? m.roles_delete_assigned_warning({ count: roleToDelete.user_count ?? 0 })
		: m.roles_delete_confirm_message()}
	confirmText={m.common_delete()}
	resourceName={roleToDelete?.name || ''}
	requireConfirmation={true}
	onConfirm={handleDelete}
	onCancel={() => {
		showDeleteModal = false;
		roleToDelete = null;
	}}
/>
