<script lang="ts">
	import { onMount, untrack } from 'svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { m } from '$lib/paraglide/messages';
	import { toasts } from '$lib/stores/toast';
	import {
		createRole,
		deleteRole,
		listPermissions,
		replacePermissions,
		updateRole
	} from '$lib/roles/api';
	import {
		documentFromForm,
		documentFromRole,
		idsFromKeys,
		type RoleDocument
	} from '$lib/roles/document';
	import { permissionMatrix } from '$lib/roles/summary';
	import { DocumentError } from '$lib/document/errors';
	import type { DocumentView } from '$lib/document/format';
	import type { Permission, Role } from '$lib/roles/types';
	import RoleDocumentEditor from './RoleDocumentEditor.svelte';

	// One page for a role: its permissions as a grid of areas and actions, or the same role as a JSON or YAML document. With no role it creates one, starting from `initial` when duplicating.
	let {
		role,
		initial,
		canManage,
		onSaved
	}: {
		role?: Role;
		initial?: RoleDocument;
		canManage: boolean;
		/** Told about the saved role, so the page around it can follow a rename. */
		onSaved?: (role: Role) => void;
	} = $props();

	const creating = untrack(() => !role);
	let editable = $derived(canManage && !role?.is_system);

	const start = (): RoleDocument =>
		untrack(() => (role ? documentFromRole(role) : (initial ?? { name: '', permission_keys: [] })));

	let permissions = $state<Permission[]>([]);
	let permissionsError = $state(false);
	let permissionsLoaded: Promise<Permission[]> = Promise.resolve([]);
	let name = $state(start().name);
	let description = $state(start().description ?? '');
	let selected = $state<string[]>([]);
	let view = $state<DocumentView>('visual');
	let doc = $state<RoleDocument>(start());
	// Bumped by Discard changes so a code view remounts with the saved document
	let revision = $state(0);
	let error = $state<string | null>(null);
	let saving = $state(false);
	let deleteOpen = $state(false);
	let deleting = $state(false);
	let tabs = $state<HTMLElement & { value: string }>();
	let documentEditor = $state<RoleDocumentEditor>();

	const codeViews = ['json', 'yaml'] as const;
	let matrix = $derived(permissionMatrix(permissions));
	let selectedSet = $derived(new Set(selected));

	onMount(() => {
		permissionsLoaded = listPermissions()
			.then((loaded) => {
				permissions = loaded;
				selected = idsFromKeys(start().permission_keys, loaded).ids;
				return loaded;
			})
			.catch(() => {
				permissionsError = true;
				return [];
			});
	});

	function toggle(id: string, on: boolean) {
		selected = on ? [...selected, id] : selected.filter((x) => x !== id);
	}

	// Leaving a code view takes its document with it, and a document the grid cannot show keeps the person where they are
	async function switchView(next: DocumentView) {
		if (next === view) return;
		error = null;
		if (view === 'visual') {
			const loaded = await permissionsLoaded;
			doc = documentFromForm(name.trim(), description.trim(), selected, loaded);
		} else {
			const result = await documentEditor?.exportDocument();
			if (result && !result.ok) {
				error = result.message;
			} else if (result) {
				const { ids, unknown } = idsFromKeys(result.doc.permission_keys, permissions);
				if (next === 'visual' && unknown.length > 0) {
					error = m.roles_document_unknown_keys({ keys: unknown.join(', ') });
				} else {
					name = result.doc.name;
					description = result.doc.description ?? '';
					if (unknown.length === 0) selected = ids;
					doc = result.doc;
				}
			}
			if (error) {
				if (tabs) tabs.value = view;
				return;
			}
		}
		view = next;
	}

	function discard() {
		const saved = start();
		name = saved.name;
		description = saved.description ?? '';
		selected = idsFromKeys(saved.permission_keys, permissions).ids;
		doc = saved;
		error = null;
		revision++;
	}

	type Input = { name: string; description: string } & (
		{ permission_ids: string[] } | { permission_keys: string[] }
	);

	// The visual grid sends ids and a document sends keys, so either way the API checks exactly what the person sees
	async function current(): Promise<Input | null> {
		if (view === 'visual') {
			if (name.trim().length < 2) {
				error = name.trim() ? m.roles_error_name_min() : m.roles_error_name_required();
				return null;
			}
			return { name: name.trim(), description: description.trim(), permission_ids: selected };
		}
		const result = await documentEditor?.exportDocument();
		if (!result) return null;
		if (!result.ok) {
			error = result.message;
			return null;
		}
		return {
			name: result.doc.name.trim(),
			description: result.doc.description?.trim() ?? '',
			permission_keys: result.doc.permission_keys
		};
	}

	async function save() {
		error = null;
		const input = await current();
		if (!input) return;
		saving = true;
		try {
			if (!role) {
				const created = await createRole({ ...input, description: input.description || undefined });
				toasts.success(m.roles_create_success({ name: created.name }));
				await goto(resolve('/settings/roles/[id]', { id: created.id }));
				return;
			}
			const patch: Record<string, string> = {};
			if (input.name !== role.name) patch.name = input.name;
			if (input.description !== (role.description ?? '')) patch.description = input.description;
			if (Object.keys(patch).length > 0) role = await updateRole(role.id, patch);
			role = await replacePermissions(
				role.id,
				'permission_ids' in input
					? { permission_ids: input.permission_ids }
					: { permission_keys: input.permission_keys }
			);
			toasts.success(m.roles_update_success({ name: role.name }));
			onSaved?.(role);
		} catch (err) {
			if (err instanceof DocumentError && err.fields.length > 0) {
				documentEditor?.showProblems(err.fields);
			}
			error =
				err instanceof Error ? err.message : role ? m.roles_error_update() : m.roles_error_create();
		} finally {
			saving = false;
		}
	}

	function onMenuSelect(e: CustomEvent<{ value: string }>) {
		if (!role) return;
		if (e.detail.value === 'duplicate') {
			goto(
				resolve(`/settings/roles/new?from=${encodeURIComponent(role.id)}` as '/settings/roles/new')
			);
		} else if (e.detail.value === 'delete') {
			deleteOpen = true;
		}
	}

	async function confirmDelete(e: Event) {
		if (!role) return;
		e.preventDefault();
		const dialog = e.currentTarget as HTMLElement & { close(value?: string): void };
		deleting = true;
		try {
			await deleteRole(role.id);
			toasts.success(m.roles_delete_success({ name: role.name }));
			dialog.close('confirm');
			await goto(resolve('/settings/roles'));
		} catch (err) {
			toasts.error(err instanceof Error ? err.message : m.roles_error_delete());
		} finally {
			deleting = false;
		}
	}

	let caption = $derived(
		role
			? [
					role.is_system ? m.roles_caption_built_in() : m.roles_caption_custom(),
					m.roles_member_count({ count: role.user_count ?? 0 })
				].join(' · ')
			: undefined
	);
</script>

<mm-page-header
	heading={role ? role.name : m.roles_create_title()}
	icon="admin-panel-settings"
	caption={caption ?? ''}
	description={role?.description ?? ''}
>
	{#if role && canManage}
		<mm-menu-button
			slot="actions"
			variant="ghost"
			icon="more-horiz"
			icon-only
			label={m.roles_more_actions({ name: role.name })}
		>
			<mm-menu onmm-select={onMenuSelect}>
				<mm-menu-item value="duplicate" icon="content-copy">{m.roles_duplicate()}</mm-menu-item>
				{#if !role.is_system}
					<mm-menu-separator></mm-menu-separator>
					<mm-menu-item value="delete" icon="delete" danger
						>{m.roles_delete_named({ name: role.name })}</mm-menu-item
					>
				{/if}
			</mm-menu>
		</mm-menu-button>
	{/if}
	{#if creating}
		<mm-button slot="actions" href={resolve('/settings/roles')}>{m.common_cancel()}</mm-button>
		<mm-button slot="actions" variant="primary" loading={saving} onclick={save}
			>{m.roles_create_title()}</mm-button
		>
	{:else if editable}
		<mm-button slot="actions" onclick={discard} disabled={saving}>{m.roles_discard()}</mm-button>
		<mm-button slot="actions" variant="primary" loading={saving} onclick={save}
			>{m.roles_save()}</mm-button
		>
	{/if}
</mm-page-header>

{#if role?.is_system}
	<mm-callout tone="info" compact heading={m.roles_document_system_readonly()}></mm-callout>
{/if}

{#if error}
	<mm-callout tone="danger" compact heading={error}></mm-callout>
{/if}

<mm-tabs
	bind:this={tabs}
	label={m.roles_editor_label()}
	value={view}
	onmm-tab-change={(e: CustomEvent<{ value: string }>) =>
		switchView(e.detail.value as DocumentView)}
>
	<mm-tab value="visual" icon="checklist">{m.ui_document_view_visual()}</mm-tab>
	<mm-tab value="json" icon="data-object">{m.ui_document_view_json()}</mm-tab>
	<mm-tab value="yaml" icon="code">{m.ui_document_view_yaml()}</mm-tab>
	<mm-tab-panel value="visual">
		{#if view === 'visual'}
			<div class="mm-stack" style="--mm-gap: var(--space-6)">
				{#if editable || creating}
					<div class="mm-cluster" style="align-items: start">
						<!-- svelte-ignore a11y_autofocus -->
						<mm-text-field
							class="role-field"
							label={m.common_name()}
							required
							autofocus={creating}
							value={name}
							help={m.roles_name_hint()}
							onmm-input={(e: CustomEvent<{ value: string }>) => (name = e.detail.value)}
						></mm-text-field>
						<mm-text-field
							class="role-field role-field--wide"
							label={m.common_description()}
							optional
							value={description}
							help={m.roles_description_hint()}
							onmm-input={(e: CustomEvent<{ value: string }>) => (description = e.detail.value)}
						></mm-text-field>
					</div>
				{/if}
				{#if permissionsError}
					<mm-callout tone="danger" compact heading={m.ui_permissions_load_error()}></mm-callout>
				{:else if permissions.length === 0}
					<mm-skeleton loading shape="table-row" count={6}></mm-skeleton>
				{:else}
					<div class="mm-table-wrap">
						<table class="mm-table">
							<caption class="mm-visually-hidden"
								>{m.roles_matrix_caption({ name: role?.name ?? name })}</caption
							>
							<thead>
								<tr>
									<th scope="col">{m.roles_matrix_area()}</th>
									{#each matrix.columns as column (column.id)}
										<th scope="col" class="text-center">{column.label}</th>
									{/each}
								</tr>
							</thead>
							<tbody>
								{#each matrix.rows as row (row.id)}
									<tr>
										<th scope="row" class="area">{row.label}</th>
										{#each matrix.columns as column (column.id)}
											{@const permission = row.cells[column.id]}
											<td class="text-center">
												{#if permission}
													<mm-checkbox
														class="inline-flex"
														label="{column.label} {row.label}"
														hide-label
														title={permission.description}
														checked={selectedSet.has(permission.id)}
														disabled={!(editable || creating)}
														onmm-change={(e: CustomEvent<{ checked: boolean }>) =>
															toggle(permission.id, e.detail.checked)}
													></mm-checkbox>
												{:else}
													<span class="mm-faint" aria-hidden="true">–</span><span
														class="mm-visually-hidden">{m.roles_matrix_not_offered()}</span
													>
												{/if}
											</td>
										{/each}
									</tr>
								{/each}
							</tbody>
						</table>
					</div>
				{/if}
			</div>
		{/if}
	</mm-tab-panel>
	{#each codeViews as mode (mode)}
		<mm-tab-panel value={mode}>
			{#if view === mode}
				{#key revision}
					<RoleDocumentEditor
						bind:this={documentEditor}
						{mode}
						initial={doc}
						{permissions}
						roleId={role?.id}
						readonly={!(editable || creating)}
						height="26rem"
						onSubmit={save}
					/>
				{/key}
			{/if}
		</mm-tab-panel>
	{/each}
</mm-tabs>

{#if role}
	<mm-confirm-dialog
		open={deleteOpen}
		heading={m.roles_delete_heading({ name: role.name })}
		body={(role.user_count ?? 0) > 0 ? m.roles_delete_assigned() : m.roles_delete_body()}
		confirm-label={m.roles_delete_role()}
		confirm-text={role.name}
		loading={deleting}
		onmm-confirm={confirmDelete}
		onmm-close={() => (deleteOpen = false)}
	></mm-confirm-dialog>
{/if}

<style>
	.role-field {
		width: 16rem;
	}
	.role-field--wide {
		flex: 1;
		min-width: 16rem;
		width: auto;
	}
	.area {
		font: var(--text-dense);
		color: var(--mm-text);
		background: none;
		position: static;
	}
</style>
