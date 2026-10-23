<script lang="ts">
	import { onMount, untrack } from 'svelte';
	import Blade from '$components/ui/Blade.svelte';
	import SegmentedControl from '$components/ui/SegmentedControl.svelte';
	import PermissionEditor from '$lib/components/PermissionEditor.svelte';
	import RoleDocumentEditor from './RoleDocumentEditor.svelte';
	import { createRole, listPermissions } from '$lib/roles/api';
	import { documentFromForm, idsFromKeys, type RoleDocument } from '$lib/roles/document';
	import type { Permission } from '$lib/roles/types';
	import { documentViewOptions, type DocumentView } from '$lib/document/format';
	import { DocumentError } from '$lib/document/errors';
	import { m } from '$lib/paraglide/messages';
	import { BTN_PRIMARY, BTN_SECONDARY, INPUT, LABEL } from '$lib/ui/classes';
	import Alert from '$components/ui/Alert.svelte';
	import { toasts } from '$lib/stores/toast';

	// initial prefills the form, as "Duplicate as new role" does with another role's document.
	interface Props {
		onClose: () => void;
		onCreated: () => void;
		initial?: RoleDocument;
	}

	let { onClose, onCreated, initial }: Props = $props();

	const formId = 'create-role-form';
	const sectionClass = LABEL;
	const labelClass = 'mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300';
	const inputClass = INPUT;

	const prefill = untrack(() => initial);
	let name = $state(prefill?.name ?? '');
	let description = $state(prefill?.description ?? '');
	let selectedPermIds = $state<string[]>([]);
	let saving = $state(false);
	let error = $state<string | null>(null);

	let permissions = $state<Permission[]>([]);
	let permissionsLoaded: Promise<Permission[]> = Promise.resolve([]);
	let view = $state<DocumentView>('visual');
	let doc = $state<RoleDocument>({ name: '', permission_keys: [] });
	let documentEditor = $state<RoleDocumentEditor>();
	let codeTouched = $state(false);

	let nameValid = $derived(name.trim().length >= 2);
	let dirty = $derived(!!(name || description || selectedPermIds.length || codeTouched));

	onMount(() => {
		permissionsLoaded = listPermissions()
			.then((loaded) => {
				permissions = loaded;
				if (prefill) selectedPermIds = idsFromKeys(prefill.permission_keys, loaded).ids;
				return loaded;
			})
			.catch(() => []);
	});

	// Leaving a code view takes its document with it, and a document the visual form cannot show keeps the user where they are.
	async function switchView(next: DocumentView) {
		if (next === view) return;
		error = null;
		if (view === 'visual') {
			const loaded = await permissionsLoaded;
			doc = documentFromForm(name.trim(), description.trim(), selectedPermIds, loaded);
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
		codeTouched = codeTouched || view !== 'visual';
		view = next;
	}

	async function save(
		input: { name: string; description?: string } & (
			{ permission_ids: string[] } | { permission_keys: string[] }
		)
	) {
		try {
			saving = true;
			await createRole(input);
			toasts.success(m.roles_create_success({ name: input.name }));
			onCreated();
		} catch (err) {
			if (err instanceof DocumentError && err.fields.length > 0) {
				documentEditor?.showProblems(err.fields);
			}
			error = err instanceof Error ? err.message : m.roles_error_create();
		} finally {
			saving = false;
		}
	}

	async function submitDocument() {
		error = null;
		const result = await documentEditor?.exportDocument();
		if (!result) return;
		if (!result.ok) {
			error = result.message;
			return;
		}
		await save({
			name: result.doc.name.trim(),
			description: result.doc.description?.trim() || undefined,
			permission_keys: result.doc.permission_keys
		});
	}

	async function handleSubmit(e: SubmitEvent) {
		e.preventDefault();
		error = null;
		if (view !== 'visual') {
			await submitDocument();
			return;
		}
		if (!name.trim()) {
			error = m.roles_error_name_required();
			return;
		}
		if (!nameValid) {
			error = m.roles_error_name_min();
			return;
		}
		await save({
			name: name.trim(),
			description: description.trim() || undefined,
			permission_ids: selectedPermIds
		});
	}
</script>

<Blade title={m.roles_create_title()} {onClose} width="2xl" {dirty}>
	<form id={formId} onsubmit={handleSubmit} class="space-y-6">
		<SegmentedControl
			options={documentViewOptions()}
			value={view}
			onChange={switchView}
			size="sm"
			label={m.ui_document_view_label()}
		/>

		{#if error}
			<Alert tone="error">{error}</Alert>
		{/if}

		{#if view === 'visual'}
			<section class="space-y-4">
				<h3 class={sectionClass}>{m.roles_basic_information_heading()}</h3>
				<div>
					<label for="role-name" class={labelClass}>
						{m.common_name()} <span class="text-red-500">*</span>
					</label>
					<input
						id="role-name"
						type="text"
						bind:value={name}
						placeholder={m.roles_name_placeholder()}
						class={inputClass}
						required
					/>
					<p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">{m.roles_name_hint()}</p>
				</div>
				<div>
					<label for="role-description" class={labelClass}>{m.common_description()}</label>
					<textarea
						id="role-description"
						bind:value={description}
						rows="3"
						placeholder={m.roles_description_placeholder()}
						class="{inputClass} resize-none"
					></textarea>
					<p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
						{m.roles_description_hint()}
					</p>
				</div>
			</section>

			<section class="space-y-3">
				<div class="flex items-center justify-between">
					<h3 class={sectionClass}>
						{m.roles_step_permissions()}
						<span class="ml-1 font-normal normal-case tracking-normal"
							>{m.roles_optional_badge()}</span
						>
					</h3>
					<span class="text-xs text-gray-500 dark:text-gray-400">
						{m.roles_selected_count({ count: selectedPermIds.length })}
					</span>
				</div>
				<p class="text-sm text-gray-600 dark:text-gray-400">{m.roles_permissions_intro()}</p>
				<PermissionEditor
					selectedIds={selectedPermIds}
					onChange={(ids) => (selectedPermIds = ids)}
				/>
			</section>
		{:else}
			<section class="space-y-3">
				<p class="text-sm text-gray-600 dark:text-gray-400">{m.roles_document_intro()}</p>
				{#key view}
					<RoleDocumentEditor
						bind:this={documentEditor}
						mode={view}
						initial={doc}
						{permissions}
						height="26rem"
						onSubmit={submitDocument}
					/>
				{/key}
			</section>
		{/if}
	</form>

	{#snippet footer(close)}
		<div class="flex items-center justify-end gap-2">
			<button type="button" onclick={close} class={BTN_SECONDARY}>
				{m.common_cancel()}
			</button>
			<button
				type="submit"
				form={formId}
				disabled={saving || (view === 'visual' && !nameValid)}
				class={BTN_PRIMARY}
			>
				{saving ? m.roles_creating_label() : m.roles_create_title()}
			</button>
		</div>
	{/snippet}
</Blade>
