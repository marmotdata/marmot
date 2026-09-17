<script lang="ts">
	import { toasts } from '$lib/stores/toast';
	import { m } from '$lib/paraglide/messages';
	import { deleteRole } from '$lib/roles/api';
	import EditRoleForm from './EditRoleForm.svelte';
	import ViewRoleForm from './ViewRoleForm.svelte';
	import DeleteModal from '$components/ui/DeleteModal.svelte';
	import { Lock, ChevronRight, ChevronDown } from 'lucide-svelte';
	import type { Role } from '$lib/roles/types';

	export let roles: Role[] = [];
	export let editingRoleId: string | null = null;
	export let onEdit: (roleId: string | null) => void;
	export let onUpdate: (updatedRole: Role) => void;
	export let onDelete: (roleId: string) => void;

	let viewingRoleId: string | null = null;
	let showDeleteModal = false;
	let roleToDelete: Role | null = null;

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
	<table class="min-w-full">
		<thead>
			<tr>
				<th
					class="w-8 px-3 py-3 bg-earthy-brown-100 dark:bg-gray-800"
					aria-label={m.roles_expand_aria()}
				></th>
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
					>{m.roles_header_permissions()}</th
				>
				<th
					class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider bg-earthy-brown-100 dark:bg-gray-800"
					>{m.roles_header_users()}</th
				>
				<th
					class="px-6 py-3 text-right text-xs font-medium text-gray-500 uppercase tracking-wider bg-earthy-brown-100 dark:bg-gray-800"
					>{m.common_actions()}</th
				>
			</tr>
		</thead>
		<tbody class="divide-y divide-earthy-brown-100 bg-earthy-brown-50 dark:bg-gray-900">
			{#each roles as role (role.id)}
				{@const isEditing = editingRoleId === role.id}
				{@const isViewing = viewingRoleId === role.id}
				{@const isExpanded = isEditing || isViewing}
				{@const isSystem = role.is_system}

				<tr
					class="hover:bg-earthy-brown-100 dark:hover:bg-gray-800 transition-colors cursor-pointer {isExpanded
						? 'bg-earthy-brown-100 dark:bg-gray-800'
						: ''}"
					on:click={() => toggleView(role.id)}
				>
					<td class="w-8 px-3 py-4 text-gray-400 dark:text-gray-500">
						{#if isExpanded}
							<ChevronDown class="h-4 w-4" />
						{:else}
							<ChevronRight class="h-4 w-4" />
						{/if}
					</td>
					<td class="px-6 py-4 whitespace-nowrap">
						<div class="flex items-center gap-2">
							<span class="text-sm font-medium text-gray-900 dark:text-gray-100">
								{role.name}
							</span>
							{#if isSystem}
								<span
									class="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium bg-blue-100 dark:bg-blue-900 text-blue-800 dark:text-blue-200"
								>
									<Lock class="h-3 w-3" />
									{m.ui_role_system_badge()}
								</span>
							{/if}
						</div>
					</td>
					<td class="px-6 py-4 text-sm text-gray-600 dark:text-gray-400 max-w-md truncate">
						{role.description || '—'}
					</td>
					<td class="px-6 py-4 whitespace-nowrap">
						<span
							class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-earthy-terracotta-100 dark:bg-earthy-terracotta-900 text-earthy-terracotta-700 dark:text-earthy-terracotta-100"
						>
							{(role.permissions ?? []).length}
						</span>
					</td>
					<td class="px-6 py-4 whitespace-nowrap text-sm text-gray-600 dark:text-gray-400">
						{role.user_count ?? 0}
					</td>
					<td
						class="px-6 py-4 whitespace-nowrap text-right text-sm font-medium"
						on:click|stopPropagation
						role="cell"
					>
						{#if !isSystem}
							<button
								type="button"
								class="text-earthy-terracotta-700 hover:text-earthy-terracotta-800 dark:text-earthy-terracotta-500 dark:hover:text-earthy-terracotta-400 mr-3"
								on:click={() => switchToEdit(role.id)}
							>
								{m.common_edit()}
							</button>
							<button
								type="button"
								class="text-red-600 hover:text-red-900 dark:text-red-400 dark:hover:text-red-300"
								on:click={() => {
									roleToDelete = role;
									showDeleteModal = true;
								}}
							>
								{m.common_delete()}
							</button>
						{/if}
					</td>
				</tr>

				{#if isExpanded}
					<tr class="bg-earthy-brown-50 dark:bg-gray-900">
						<td colspan="6" class="p-0">
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
									onEdit={isSystem ? null : () => switchToEdit(role.id)}
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
		? m.roles_delete_assigned_warning({ count: roleToDelete.user_count })
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
