<script lang="ts">
	import { untrack } from 'svelte';
	import { fetchApi } from '$lib/api';
	import { auth } from '$lib/stores/auth';
	import { toasts } from '$lib/stores/toast';
	import type { Asset, FieldAnnotation } from '$lib/assets/types';
	import { m } from '$lib/paraglide/messages';
	import { applyAnnotations, parseSchema } from './schemaColumns';
	import '@marmotdata/elements/schema-table.js';
	import '@marmotdata/elements/code-sample.js';
	import type { MenuSelectDetail } from '@marmotdata/elements/classes/menu.js';
	import type { MmConfirmDialog } from '@marmotdata/elements/classes/confirm-dialog.js';
	import type { MmDialog } from '@marmotdata/elements/classes/dialog.js';
	import type {
		MmEditDescriptionDetail,
		MmSelectFieldDetail
	} from '@marmotdata/elements/classes/schema-table.js';

	// The Schema tab: the outline (with its own JSON and YAML views) for the selected schema, and adding, editing or removing a schema in place.
	let { asset }: { asset: Asset } = $props();

	let canManageAssets = $derived(auth.hasPermission('assets', 'manage'));

	let schemas = $state<Record<string, unknown>>({});
	let activeKey = $state('');
	let mode = $state<'view' | 'add' | 'edit'>('view');
	let draftName = $state('');
	let draftContent = $state('');
	let nameError = $state('');
	let contentError = $state('');
	let saving = $state(false);
	let deleteDialog = $state<MmConfirmDialog>();

	// The field open in the outline's panel, which the marker buttons act on.
	let selectedPath = $state('');
	let fieldSaving = $state(false);
	let descriptionDialog = $state<MmDialog>();
	let descriptionPath = $state('');
	let descriptionDraft = $state('');

	let keys = $derived(Object.keys(schemas));
	let parsed = $derived(activeKey && activeKey in schemas ? parseSchema(schemas[activeKey]) : null);
	let exampleText = $derived(
		parsed?.example && typeof parsed.example === 'object' && Object.keys(parsed.example).length > 0
			? JSON.stringify(parsed.example, null, 2)
			: ''
	);

	let annotations = $derived((asset.field_annotations ?? []).filter((a) => a.schema === activeKey));
	let columns = $derived(parsed ? applyAnnotations(parsed.columns, annotations) : []);
	let selectedAnnotation = $derived(annotations.find((a) => a.path === selectedPath));

	// Reads only the asset, never the state it writes, so an asset without a schema does not loop on a fresh empty object
	$effect(() => {
		const next = (asset?.schema ?? {}) as Record<string, unknown>;
		schemas = next;
		const current = untrack(() => activeKey);
		if (!current || !(current in next)) activeKey = Object.keys(next)[0] ?? '';
	});

	async function saveSchemas(updated: Record<string, unknown>): Promise<boolean> {
		if (!asset?.id) return false;
		saving = true;
		try {
			const response = await fetchApi(`/assets/${asset.id}`, {
				method: 'PUT',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ schema: updated })
			});
			if (!response.ok) {
				const data = await response.json().catch(() => ({}));
				throw new Error(data.error || m.schema_error_unknown());
			}
			schemas = updated;
			asset.schema = updated;
			return true;
		} catch (err) {
			toasts.error(
				m.schema_update_error_generic({
					error: err instanceof Error ? err.message : m.schema_error_unknown()
				})
			);
			return false;
		} finally {
			saving = false;
		}
	}

	function startAdd() {
		draftName = '';
		draftContent = '';
		nameError = '';
		contentError = '';
		mode = 'add';
	}

	function startEdit() {
		const v = schemas[activeKey];
		draftName = activeKey;
		draftContent = typeof v === 'string' ? v : JSON.stringify(v, null, 2);
		nameError = '';
		contentError = '';
		mode = 'edit';
	}

	async function submit(e: SubmitEvent) {
		e.preventDefault();
		const name = draftName.trim();
		const content = draftContent.trim();
		nameError = name ? '' : m.asset_page_schema_name_required();
		contentError = content ? '' : m.asset_page_schema_content_required();
		if (nameError || contentError) return;
		const updated = { ...schemas };
		if (mode === 'edit' && activeKey !== name) delete updated[activeKey];
		updated[name] = content;
		if (await saveSchemas(updated)) {
			activeKey = name;
			mode = 'view';
		}
	}

	function onMore(e: CustomEvent<MenuSelectDetail>) {
		if (e.detail.value === 'edit') startEdit();
		if (e.detail.value === 'delete') deleteDialog?.show();
	}

	async function setField(
		path: string,
		change: Partial<Record<'pii' | 'certified', boolean>> & { description?: string }
	) {
		if (!asset?.id || !path) return false;
		fieldSaving = true;
		try {
			const response = await fetchApi(`/assets/field-annotations/${encodeURIComponent(asset.id)}`, {
				method: 'PUT',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ schema: activeKey, path, ...change })
			});
			const data = await response.json().catch(() => ({}));
			if (!response.ok) throw new Error(data.error || m.schema_error_unknown());
			asset.field_annotations = (data.annotations ?? []) as FieldAnnotation[];
			return true;
		} catch (err) {
			toasts.error(
				m.schema_field_saved_error({
					error: err instanceof Error ? err.message : m.schema_error_unknown()
				})
			);
			return false;
		} finally {
			fieldSaving = false;
		}
	}

	function editDescription(path: string, current: string) {
		descriptionPath = path;
		descriptionDraft = current;
		descriptionDialog?.show();
	}

	async function saveDescription(e?: Event) {
		e?.preventDefault();
		if (await setField(descriptionPath, { description: descriptionDraft.trim() })) {
			descriptionDialog?.close();
		}
	}

	async function confirmDelete() {
		const updated = { ...schemas };
		delete updated[activeKey];
		if (await saveSchemas(updated)) {
			activeKey = Object.keys(updated)[0] ?? '';
		}
	}
</script>

{#if mode !== 'view'}
	<form class="mm-stack mm-measure" style="--mm-gap: var(--space-4)" onsubmit={submit} novalidate>
		<mm-text-field
			label={m.schema_name_label()}
			value={draftName}
			error={nameError}
			placeholder={m.schema_name_placeholder()}
			autofocus
			onmm-input={(e: CustomEvent<{ value: string }>) => (draftName = e.detail.value)}
		></mm-text-field>
		<mm-textarea
			label={m.schema_content_label()}
			value={draftContent}
			error={contentError}
			rows={14}
			mono
			placeholder={m.schema_content_placeholder()}
			onmm-input={(e: CustomEvent<{ value: string }>) => (draftContent = e.detail.value)}
		></mm-textarea>
		<div class="mm-cluster">
			<mm-button type="submit" variant="primary" loading={saving}
				>{mode === 'add' ? m.asset_page_add_schema() : m.asset_page_save_schema()}</mm-button
			>
			<mm-button variant="ghost" disabled={saving} onclick={() => (mode = 'view')}
				>{m.common_cancel()}</mm-button
			>
		</div>
	</form>
{:else if keys.length === 0}
	<mm-empty-state
		heading={m.asset_page_schema_empty_heading()}
		description={canManageAssets
			? m.asset_page_schema_empty_body_manage()
			: m.asset_page_schema_empty_body()}
		seed={7}
	>
		{#if canManageAssets}
			<mm-button slot="action" variant="secondary" icon="add" onclick={startAdd}
				>{m.asset_page_add_schema()}</mm-button
			>
		{/if}
	</mm-empty-state>
{:else}
	<div class="mm-stack" style="--mm-gap: var(--space-4)">
		{#if keys.length > 1 || canManageAssets}
			<div class="mm-cluster mm-cluster--between">
				{#if keys.length > 1}
					<mm-segmented-control
						label={m.asset_page_schema_picker()}
						hide-label
						value={activeKey}
						onmm-change={(e: CustomEvent<{ value: string }>) => {
							activeKey = e.detail.value;
							selectedPath = '';
						}}
					>
						{#each keys as k (k)}
							<mm-segment value={k}>{k}</mm-segment>
						{/each}
					</mm-segmented-control>
				{:else}
					<span></span>
				{/if}
				{#if canManageAssets}
					<div class="mm-cluster" style="--mm-gap: var(--space-2)">
						<mm-button variant="ghost" size="sm" icon="add" onclick={startAdd}
							>{m.asset_page_add_schema()}</mm-button
						>
						<mm-menu-button
							variant="ghost"
							size="sm"
							icon="more-horiz"
							icon-only
							label={m.asset_page_schema_more({ name: activeKey })}
						>
							<mm-menu onmm-select={onMore}>
								<mm-menu-item value="edit" icon="edit">{m.asset_page_edit_schema()}</mm-menu-item>
								<mm-menu-separator></mm-menu-separator>
								<mm-menu-item value="delete" icon="delete" danger
									>{m.asset_page_delete_schema()}</mm-menu-item
								>
							</mm-menu>
						</mm-menu-button>
					</div>
				{/if}
			</div>
		{/if}

		{#if parsed}
			{#if parsed.warnings.length > 0}
				<mm-callout tone="warning" heading={m.asset_page_schema_warnings_heading()}>
					<ul class="warnings">
						{#each parsed.warnings.slice(0, 3) as w, i (i)}
							<li>{w}</li>
						{/each}
					</ul>
				</mm-callout>
			{/if}

			{#if canManageAssets && columns.length > 0}
				<div class="mm-cluster" style="--mm-gap: var(--space-2)">
					{#if selectedPath}
						<mm-text variant="caption" tone="secondary"
							>{m.schema_field_actions({ path: selectedPath })}</mm-text
						>
						<mm-button
							variant="ghost"
							size="sm"
							icon="shield"
							pressed={selectedAnnotation?.pii ? 'true' : 'false'}
							disabled={fieldSaving}
							onclick={() => setField(selectedPath, { pii: !selectedAnnotation?.pii })}
							>{m.schema_mark_pii()}</mm-button
						>
						<mm-button
							variant="ghost"
							size="sm"
							icon="verified"
							pressed={selectedAnnotation?.certified ? 'true' : 'false'}
							disabled={fieldSaving}
							onclick={() => setField(selectedPath, { certified: !selectedAnnotation?.certified })}
							>{m.schema_mark_certified()}</mm-button
						>
						<mm-button
							variant="ghost"
							size="sm"
							icon="edit"
							disabled={fieldSaving}
							onclick={() => editDescription(selectedPath, selectedAnnotation?.description ?? '')}
							>{m.schema_edit_field_description()}</mm-button
						>
					{:else}
						<mm-text variant="caption" tone="secondary">{m.schema_select_field_hint()}</mm-text>
					{/if}
				</div>
			{/if}

			{#key activeKey}
				<mm-schema-table
					label={m.asset_page_schema_label({ name: asset.name })}
					{columns}
					source={parsed.source}
					editable={canManageAssets}
					empty-text={m.schema_no_fields_for({ name: activeKey })}
					onmm-select-field={(e: CustomEvent<MmSelectFieldDetail>) =>
						(selectedPath = e.detail.path)}
					onmm-edit-description={(e: CustomEvent<MmEditDescriptionDetail>) =>
						editDescription(
							e.detail.path,
							annotations.find((a) => a.path === e.detail.path)?.description ?? ''
						)}
				></mm-schema-table>
			{/key}

			{#if parsed.rawText}
				<mm-disclosure summary={m.asset_page_schema_show_source()}>
					<mm-code-sample
						language="text"
						code={parsed.rawText}
						label={m.asset_page_schema_source_label({ name: activeKey })}
					></mm-code-sample>
				</mm-disclosure>
			{/if}

			{#if exampleText}
				<mm-disclosure summary={m.asset_page_schema_show_example()}>
					<mm-code-sample
						language="json"
						code={exampleText}
						label={m.asset_page_schema_example_label({ name: activeKey })}
					></mm-code-sample>
				</mm-disclosure>
			{/if}
		{/if}
	</div>
{/if}

<mm-dialog bind:this={descriptionDialog} heading={m.schema_edit_field_description()} size="sm">
	<form class="mm-stack" style="--mm-gap: var(--space-4)" onsubmit={saveDescription}>
		<mm-textarea
			label={m.schema_field_description_label({ path: descriptionPath })}
			help={m.schema_field_description_hint()}
			rows={4}
			autosize
			value={descriptionDraft}
			onmm-input={(e: CustomEvent<{ value: string }>) => (descriptionDraft = e.detail.value)}
		></mm-textarea>
	</form>
	<mm-button slot="footer" data-close="cancel" disabled={fieldSaving}>{m.common_cancel()}</mm-button
	>
	<mm-button slot="footer" variant="primary" loading={fieldSaving} onclick={saveDescription}
		>{m.common_save()}</mm-button
	>
</mm-dialog>

<mm-confirm-dialog
	bind:this={deleteDialog}
	heading={m.asset_page_delete_schema_heading({ name: activeKey })}
	body={m.schema_delete_confirm()}
	confirm-label={m.asset_page_delete_schema()}
	onmm-confirm={confirmDelete}
></mm-confirm-dialog>

<style>
	.warnings {
		margin: 0;
		padding-inline-start: var(--space-5);
	}
</style>
