<script lang="ts">
	import { toasts } from '$lib/stores/toast';
	import { createRole } from '$lib/roles/api';
	import PermissionEditor from '$lib/components/PermissionEditor.svelte';
	import { Shield } from 'lucide-svelte';

	export let onRoleCreated: () => void;
	export let onCancel: () => void = () => {};

	let name = '';
	let description = '';
	let selectedPermIds: string[] = [];
	let loading = false;

	async function handleSubmit() {
		if (!name.trim()) {
			toasts.error('Name is required');
			return;
		}
		try {
			loading = true;
			await createRole({
				name: name.trim(),
				description: description.trim(),
				permission_ids: selectedPermIds
			});
			toasts.success(`Role "${name}" created successfully`);
			name = '';
			description = '';
			selectedPermIds = [];
			onRoleCreated();
		} catch (err) {
			toasts.error(err instanceof Error ? err.message : 'Failed to create role');
		} finally {
			loading = false;
		}
	}
</script>

<div
	class="mb-6 bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-6 animate-slide-down"
>
	<h3 class="text-lg font-semibold text-gray-900 dark:text-gray-100 mb-6 flex items-center">
		<Shield class="h-5 w-5 mr-2 text-gray-500 dark:text-gray-400" />
		Create New Role
	</h3>

	<form on:submit|preventDefault={handleSubmit} class="space-y-6">
		<div class="grid grid-cols-1 md:grid-cols-2 gap-6">
			<div>
				<label
					for="role-name"
					class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2"
				>
					Name
				</label>
				<input
					id="role-name"
					type="text"
					bind:value={name}
					required
					placeholder="e.g. data-reader"
					class="w-full px-3 py-2 bg-white dark:bg-gray-900 border border-gray-300 dark:border-gray-600 rounded-md text-sm text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-earthy-terracotta-500 focus:border-transparent"
				/>
			</div>
			<div>
				<label
					for="role-description"
					class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2"
				>
					Description
				</label>
				<input
					id="role-description"
					type="text"
					bind:value={description}
					placeholder="What this role is for"
					class="w-full px-3 py-2 bg-white dark:bg-gray-900 border border-gray-300 dark:border-gray-600 rounded-md text-sm text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-earthy-terracotta-500 focus:border-transparent"
				/>
			</div>
		</div>

		<div class="border-t border-gray-200 dark:border-gray-700 pt-6">
			<p class="text-sm font-medium text-gray-900 dark:text-gray-100 mb-3">Permissions</p>
			<PermissionEditor
				selectedIds={selectedPermIds}
				onChange={(ids) => (selectedPermIds = ids)}
			/>
		</div>

		<div
			class="flex justify-end space-x-3 pt-6 border-t border-gray-200 dark:border-gray-700"
		>
			<button
				type="button"
				class="px-4 py-2 bg-white dark:bg-gray-800 border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 rounded-md hover:bg-gray-50 dark:hover:bg-gray-700 text-sm font-medium"
				on:click={onCancel}
			>
				Cancel
			</button>
			<button
				type="submit"
				disabled={loading || !name}
				class="px-4 py-2 bg-earthy-terracotta-600 text-white rounded-md hover:bg-earthy-terracotta-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-earthy-terracotta-500 text-sm font-medium disabled:opacity-50 disabled:cursor-not-allowed flex items-center"
			>
				{#if loading}
					<div class="animate-spin rounded-full h-4 w-4 border-b-2 border-white mr-2"></div>
				{/if}
				Create Role
			</button>
		</div>
	</form>
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
