<script lang="ts">
	import { toasts } from '$lib/stores/toast';
	import { updateRole, replacePermissions } from '$lib/roles/api';
	import PermissionEditor from '$lib/components/PermissionEditor.svelte';
	import { Shield } from 'lucide-svelte';
	import type { Role } from '$lib/roles/types';

	export let role: Role;
	export let onCancel: () => void;
	export let onUpdate: (updatedRole: Role) => void;

	let name = role.name;
	let description = role.description ?? '';
	let selectedPermIds = (role.permissions ?? []).map((p) => p.id);
	let loading = false;

	async function handleSave() {
		try {
			loading = true;
			let updated = role;

			const patch: Record<string, string> = {};
			if (name !== role.name) patch.name = name;
			if (description !== (role.description ?? '')) patch.description = description;
			if (Object.keys(patch).length > 0) {
				updated = await updateRole(role.id, patch);
			}

			updated = await replacePermissions(role.id, { permission_ids: selectedPermIds });
			toasts.success(`Role "${updated.name}" updated`);
			onUpdate(updated);
		} catch (err) {
			toasts.error(err instanceof Error ? err.message : 'Failed to update role');
		} finally {
			loading = false;
		}
	}
</script>

<div
	class="bg-white dark:bg-gray-800 rounded-lg shadow-sm border border-gray-200 dark:border-gray-700 p-6 m-4"
>
	<h3 class="text-lg font-semibold text-gray-900 dark:text-gray-100 mb-6 flex items-center">
		<Shield class="h-5 w-5 mr-2 text-gray-500 dark:text-gray-400" />
		Edit Role
	</h3>

	<div class="space-y-6">
		<div class="grid grid-cols-1 md:grid-cols-2 gap-6">
			<div>
				<label
					for="edit-role-name"
					class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2"
				>
					Name
				</label>
				<input
					id="edit-role-name"
					type="text"
					bind:value={name}
					class="w-full px-3 py-2 bg-white dark:bg-gray-900 border border-gray-300 dark:border-gray-600 rounded-md text-sm text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-earthy-terracotta-500 focus:border-transparent"
				/>
			</div>
			<div>
				<label
					for="edit-role-description"
					class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2"
				>
					Description
				</label>
				<input
					id="edit-role-description"
					type="text"
					bind:value={description}
					class="w-full px-3 py-2 bg-white dark:bg-gray-900 border border-gray-300 dark:border-gray-600 rounded-md text-sm text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-earthy-terracotta-500 focus:border-transparent"
				/>
			</div>
		</div>

		<div class="border-t border-gray-200 dark:border-gray-700 pt-6">
			<h4 class="text-sm font-medium text-gray-900 dark:text-gray-100 mb-3">Permissions</h4>
			<PermissionEditor selectedIds={selectedPermIds} onChange={(ids) => (selectedPermIds = ids)} />
		</div>
	</div>

	<div class="flex justify-end space-x-3 mt-6 pt-6 border-t border-gray-200 dark:border-gray-700">
		<button
			type="button"
			class="px-4 py-2 bg-white dark:bg-gray-800 border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 rounded-md hover:bg-gray-50 dark:hover:bg-gray-700 text-sm font-medium"
			on:click={onCancel}
		>
			Cancel
		</button>
		<button
			type="button"
			class="px-4 py-2 bg-earthy-terracotta-600 text-white rounded-md hover:bg-earthy-terracotta-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-earthy-terracotta-500 text-sm font-medium disabled:opacity-50 disabled:cursor-not-allowed flex items-center"
			on:click={handleSave}
			disabled={loading}
		>
			{#if loading}
				<div class="animate-spin rounded-full h-4 w-4 border-b-2 border-white mr-2"></div>
			{/if}
			Save Changes
		</button>
	</div>
</div>
