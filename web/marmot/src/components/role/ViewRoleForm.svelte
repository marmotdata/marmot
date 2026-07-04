<script lang="ts">
	import PermissionEditor from '$lib/components/PermissionEditor.svelte';
	import { Shield, Lock, Users } from 'lucide-svelte';
	import type { Role } from '$lib/roles/types';

	export let role: Role;
	export let onClose: () => void;
	export let onEdit: (() => void) | null = null;

	$: selectedPermIds = (role.permissions ?? []).map((p) => p.id);
</script>

<div
	class="bg-white dark:bg-gray-800 rounded-lg shadow-sm border border-gray-200 dark:border-gray-700 p-6 m-4"
>
	<div class="flex items-start justify-between mb-6">
		<div class="flex items-start gap-3">
			<div class="p-2 bg-earthy-terracotta-100 dark:bg-earthy-terracotta-900/30 rounded-md">
				<Shield class="h-5 w-5 text-earthy-terracotta-700 dark:text-earthy-terracotta-400" />
			</div>
			<div>
				<div class="flex items-center gap-2">
					<h3 class="text-lg font-semibold text-gray-900 dark:text-gray-100">{role.name}</h3>
					{#if role.is_system}
						<span
							class="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium bg-blue-100 dark:bg-blue-900 text-blue-800 dark:text-blue-200"
						>
							<Lock class="h-3 w-3" />
							system
						</span>
					{/if}
				</div>
				{#if role.description}
					<p class="text-sm text-gray-600 dark:text-gray-400 mt-1">{role.description}</p>
				{/if}
				<div class="flex items-center gap-4 mt-2 text-xs text-gray-500 dark:text-gray-400">
					<span class="inline-flex items-center gap-1">
						<Users class="h-3.5 w-3.5" />
						{role.user_count ?? 0} user{(role.user_count ?? 0) === 1 ? '' : 's'} assigned
					</span>
					<span>
						{(role.permissions ?? []).length} permission{(role.permissions ?? []).length === 1 ? '' : 's'} granted
					</span>
				</div>
			</div>
		</div>

		<div class="flex items-center gap-2">
			{#if onEdit}
				<button
					type="button"
					class="px-3 py-1.5 text-sm font-medium text-earthy-terracotta-700 dark:text-earthy-terracotta-400 hover:bg-earthy-terracotta-50 dark:hover:bg-earthy-terracotta-900/20 rounded-md"
					on:click={onEdit}
				>
					Edit
				</button>
			{/if}
			<button
				type="button"
				class="px-3 py-1.5 text-sm font-medium text-gray-600 dark:text-gray-400 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-md"
				on:click={onClose}
			>
				Close
			</button>
		</div>
	</div>

	<div class="border-t border-gray-200 dark:border-gray-700 pt-6">
		<h4 class="text-sm font-medium text-gray-900 dark:text-gray-100 mb-3">
			Granted permissions
		</h4>
		{#if (role.permissions ?? []).length === 0}
			<p class="text-sm text-gray-500 dark:text-gray-400 italic">No permissions granted</p>
		{:else}
			<PermissionEditor selectedIds={selectedPermIds} readonly={true} />
		{/if}
	</div>
</div>
