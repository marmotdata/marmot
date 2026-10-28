<script lang="ts">
	import { onMount, untrack } from 'svelte';
	import IconifyIcon from '@iconify/svelte';
	import { toasts } from '$lib/stores/toast';
	import { m } from '$lib/paraglide/messages';
	import { updateRole, replacePermissions, listPermissions } from '$lib/roles/api';
	import { documentFromForm, idsFromKeys, type RoleDocument } from '$lib/roles/document';
	import { documentViewOptions, type DocumentView } from '$lib/document/format';
	import { DocumentError } from '$lib/document/errors';
	import { BTN_PRIMARY, BTN_SECONDARY, INPUT, LABEL } from '$lib/ui/classes';
	import PermissionEditor from '$lib/components/PermissionEditor.svelte';
	import SegmentedControl from '$components/ui/SegmentedControl.svelte';
	import Alert from '$components/ui/Alert.svelte';
	import RoleDocumentEditor from './RoleDocumentEditor.svelte';
	import type { Permission, Role } from '$lib/roles/types';

	let {
		role,
		onCancel,
		onUpdate
	}: { role: Role; onCancel: () => void; onUpdate: (updatedRole: Role) => void } = $props();

	// The form edits a copy taken when it opens; the row is remounted for each role so it never needs to follow the prop
	const initial = untrack(() => role);
	let name = $state(initial.name);
	let description = $state(initial.description ?? '');
	let selectedPermIds = $state((initial.permissions ?? []).map((p) => p.id));
	let loading = $state(false);
	let error = $state<string | null>(null);

	let permissions = $state<Permission[]>([]);
	let permissionsLoaded: Promise<Permission[]> = Promise.resolve([]);
	let view = $state<DocumentView>('visual');
	let doc = $state<RoleDocument>({ name: '', permission_keys: [] });
	let documentEditor = $state<RoleDocumentEditor>();

	onMount(() => {
		permissionsLoaded = listPermissions()
			.then((loaded) => (permissions = loaded))
			.catch(() => []);
	});

	// Leaving a code view takes its document with it, and a document the visual form cannot show keeps the user where they are.
	async function switchView(next: DocumentView) {
		if (next === view) return;
		error = null;
		if (view === 'visual') {
			const loaded = await permissionsLoaded;
			doc = documentFromForm(name, description, selectedPermIds, loaded);
		} else {
			const result = await documentEditor?.exportDocument();
			if (!result) return;
			if (!result.ok) {
				error = result.message;
				return;
			}
			const { ids, unknown } = idsFromKeys(result.doc.permission_keys, permissions);
			if (next === 'visual' && unknown.length > 0) {
				error = m.roles_document_unknown_keys({ keys: unknown.join(', ') });
				return;
			}
			name = result.doc.name;
			description = result.doc.description ?? '';
			if (unknown.length === 0) selectedPermIds = ids;
			doc = result.doc;
		}
		view = next;
	}

	async function write(
		nextName: string,
		nextDescription: string,
		permissionsInput: { permission_ids: string[] } | { permission_keys: string[] }
	) {
		try {
			loading = true;
			error = null;
			let updated = role;

			const patch: Record<string, string> = {};
			if (nextName !== role.name) patch.name = nextName;
			if (nextDescription !== (role.description ?? '')) patch.description = nextDescription;
			if (Object.keys(patch).length > 0) {
				updated = await updateRole(role.id, patch);
			}

			updated = await replacePermissions(role.id, permissionsInput);
			toasts.success(m.roles_update_success({ name: updated.name }));
			onUpdate(updated);
		} catch (err) {
			if (err instanceof DocumentError && err.fields.length > 0) {
				documentEditor?.showProblems(err.fields);
			}
			const message = err instanceof Error ? err.message : m.roles_error_update();
			if (view === 'visual') toasts.error(message);
			else error = message;
		} finally {
			loading = false;
		}
	}

	async function handleSave() {
		if (view === 'visual') {
			await write(name, description, { permission_ids: selectedPermIds });
			return;
		}
		error = null;
		const result = await documentEditor?.exportDocument();
		if (!result) return;
		if (!result.ok) {
			error = result.message;
			return;
		}
		await write(result.doc.name, result.doc.description ?? '', {
			permission_keys: result.doc.permission_keys
		});
	}
</script>

<div class="px-5 py-5">
	<div class="flex flex-wrap items-center justify-between gap-3">
		<h3 class="flex items-center gap-2 text-sm font-semibold text-gray-900 dark:text-gray-100">
			<IconifyIcon icon="material-symbols:shield-outline" class="h-4 w-4 text-gray-400" />
			{m.roles_edit_heading()}
		</h3>
		<SegmentedControl
			options={documentViewOptions()}
			value={view}
			onChange={switchView}
			size="sm"
			label={m.ui_document_view_label()}
		/>
	</div>

	{#if error}
		<div class="mt-4"><Alert tone="error">{error}</Alert></div>
	{/if}

	{#if view === 'visual'}
		<div class="mt-5 grid grid-cols-1 gap-5 md:grid-cols-2">
			<div>
				<label for="edit-role-name-{role.id}" class="{LABEL} mb-1.5 block">{m.common_name()}</label>
				<input id="edit-role-name-{role.id}" type="text" bind:value={name} class={INPUT} />
			</div>
			<div>
				<label for="edit-role-description-{role.id}" class="{LABEL} mb-1.5 block"
					>{m.common_description()}</label
				>
				<input
					id="edit-role-description-{role.id}"
					type="text"
					bind:value={description}
					class={INPUT}
				/>
			</div>
		</div>

		<div class="mt-6">
			<p class="{LABEL} mb-2">{m.roles_header_permissions()}</p>
			<PermissionEditor selectedIds={selectedPermIds} onChange={(ids) => (selectedPermIds = ids)} />
		</div>
	{:else}
		<div class="mt-5">
			{#key view}
				<RoleDocumentEditor
					bind:this={documentEditor}
					mode={view}
					initial={doc}
					{permissions}
					roleId={role.id}
					onSubmit={handleSave}
				/>
			{/key}
		</div>
	{/if}

	<div class="mt-6 flex justify-end gap-2 border-t border-gray-200 pt-5 dark:border-gray-700">
		<button type="button" class={BTN_SECONDARY} onclick={onCancel}>{m.common_cancel()}</button>
		<button type="button" class={BTN_PRIMARY} onclick={handleSave} disabled={loading}>
			{loading ? m.members_saving_label() : m.roles_save_changes()}
		</button>
	</div>
</div>
