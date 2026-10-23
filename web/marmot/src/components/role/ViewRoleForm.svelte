<script lang="ts">
	import IconifyIcon from '@iconify/svelte';
	import { m } from '$lib/paraglide/messages';
	import { BTN_GHOST } from '$lib/ui/classes';
	import SegmentedControl from '$components/ui/SegmentedControl.svelte';
	import RoleDocumentEditor from './RoleDocumentEditor.svelte';
	import RolePermissionSummary from './RolePermissionSummary.svelte';
	import { documentFromRole } from '$lib/roles/document';
	import { documentViewOptions, type DocumentView } from '$lib/document/format';
	import type { Role } from '$lib/roles/types';

	// onDuplicate opens the create blade with this role's document, which is how a system role, fixed itself, becomes the start of a custom one.
	let {
		role,
		onClose,
		onEdit = null,
		onDuplicate = null
	}: {
		role: Role;
		onClose: () => void;
		onEdit?: (() => void) | null;
		onDuplicate?: (() => void) | null;
	} = $props();

	let grantedKeys = $derived(
		role.permission_keys ?? (role.permissions ?? []).map((p) => `${p.resource_type}:${p.action}`)
	);
	let view = $state<DocumentView>('visual');
	let doc = $derived(documentFromRole(role));
</script>

<!-- The row above already names and describes the role, so this opens straight onto what it grants -->
<div class="px-5 py-4">
	<div class="flex flex-wrap items-center justify-between gap-3">
		<p class="text-xs tabular-nums text-gray-500 dark:text-gray-400">
			{m.roles_users_assigned_count({ count: role.user_count ?? 0 })}
			<span aria-hidden="true">·</span>
			{m.roles_permissions_granted_count({ count: (role.permissions ?? []).length })}
			{#if role.is_system}
				<span aria-hidden="true">·</span>
				{m.roles_document_system_readonly()}
			{/if}
		</p>
		<div class="flex flex-wrap items-center gap-1.5">
			<SegmentedControl
				options={documentViewOptions()}
				value={view}
				onChange={(next) => (view = next)}
				size="sm"
				label={m.ui_document_view_label()}
			/>
			{#if onDuplicate}
				<button type="button" class={BTN_GHOST} onclick={onDuplicate}>
					<IconifyIcon icon="material-symbols:content-copy-outline" class="h-4 w-4" />
					{m.roles_document_duplicate()}
				</button>
			{/if}
			{#if onEdit}
				<button type="button" class={BTN_GHOST} onclick={onEdit}>
					<IconifyIcon icon="material-symbols:edit-outline" class="h-4 w-4" />
					{m.common_edit()}
				</button>
			{/if}
			<button
				type="button"
				class={BTN_GHOST}
				onclick={onClose}
				aria-label={m.common_close()}
				title={m.common_close()}
			>
				<IconifyIcon icon="material-symbols:close" class="h-4 w-4" />
			</button>
		</div>
	</div>

	<div class="mt-3">
		{#if view === 'visual'}
			<RolePermissionSummary {grantedKeys} />
		{:else}
			{#key view}
				<RoleDocumentEditor mode={view} initial={doc} permissions={[]} readonly height="16rem" />
			{/key}
		{/if}
	</div>
</div>
