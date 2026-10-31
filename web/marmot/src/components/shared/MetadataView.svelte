<script lang="ts">
	// Free-form metadata: read-only views are an mm-metadata-viewer (keys in mono, values in the text face, nested values folded), and the editable view is a table of key and value rows with Add field, saving each change straight to the endpoint.
	import { fetchApi } from '$lib/api';
	import { auth } from '$lib/stores/auth';
	import type { Asset } from '$lib/assets/types';
	import '@marmotdata/elements/metadata-viewer.js';
	import DeleteModal from '$components/ui/DeleteModal.svelte';
	import { m } from '$lib/paraglide/messages';
	import { INPUT, TH, TD, TBODY } from '$lib/ui/classes';

	let {
		asset = undefined,
		metadata: metadataProp = $bindable(undefined),
		readOnly = false,
		maxDepth: _maxDepth = 1,
		maxCharLength = undefined,
		endpoint = undefined,
		id = undefined,
		permissionResource = undefined,
		permissionAction = undefined
	}: {
		asset?: Asset;
		metadata?: Record<string, unknown>;
		readOnly?: boolean;
		maxDepth?: number;
		maxCharLength?: number;
		showDetailsLink?: string;
		endpoint?: string;
		id?: string;
		permissionResource?: string;
		permissionAction?: string;
	} = $props();

	// Determine if we're in read-only mode
	// Respect explicit readOnly prop first, otherwise check if we have an entity to edit
	let isReadOnly = $derived(readOnly);

	// Local-only mode: when no endpoint/id/asset, just update the bound metadata directly (for create forms)
	const isLocalMode = $derived(!endpoint && !id && !asset);

	let canEdit = $derived(() => {
		if (isReadOnly) return false;

		// Local mode (create forms): always allow editing since we're just updating bound state
		if (isLocalMode) {
			return true;
		}

		// Check permissions
		let hasPermission = false;
		if (permissionResource && permissionAction) {
			hasPermission = auth.hasPermission(permissionResource, permissionAction);
			// When permission props are provided, we're in edit mode - allow editing
			return hasPermission;
		} else if (asset) {
			// Default to assets permission for backward compatibility
			hasPermission = auth.hasPermission('assets', 'manage');
		}

		if (!hasPermission) return false;

		return true;
	});

	// Use provided metadata or asset metadata
	let metadata = $state<Record<string, unknown>>(metadataProp || asset?.metadata || {});

	let showAddRow = $state(false);
	let editingKey = $state<string | null>(null);
	let editingValue = $state('');
	let newKey = $state('');
	let newValue = $state('');
	let saving = $state(false);
	let expandedDetails: { [key: string]: boolean } = $state({});
	let showDeleteModal = $state(false);
	let keyToDelete = $state<string | null>(null);

	$effect(() => {
		if (metadataProp) {
			metadata = metadataProp;
		} else if (asset?.id) {
			metadata = asset.metadata || {};
		}
	});

	function isObject(value: unknown): boolean {
		return typeof value === 'object' && value !== null && !Array.isArray(value);
	}

	function isArray(value: unknown): value is unknown[] {
		return Array.isArray(value);
	}

	function containsObject(value: unknown[]): boolean {
		return value.some((item) => isObject(item));
	}

	// Values read as plain text in ink: booleans as Yes and No, empty values as Not set, never coloured by type.
	function plainValue(value: unknown): string {
		if (value === null || value === undefined || value === '') return m.shared_metadata_not_set();
		if (typeof value === 'boolean') return value ? m.common_yes() : m.common_no();
		return formatDisplayValue(value);
	}

	async function saveMetadata(updatedMetadata: Record<string, unknown>) {
		const entityId = id || asset?.id;
		const apiEndpoint = endpoint || (asset ? '/assets' : null);

		// If no endpoint/id provided, just update the metadata locally (for parent to handle save)
		if (!entityId || !apiEndpoint) {
			metadata = updatedMetadata;
			metadataProp = updatedMetadata;
			return;
		}

		saving = true;
		try {
			const response = await fetchApi(`${apiEndpoint}/${entityId}`, {
				method: 'PUT',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({
					metadata: updatedMetadata
				})
			});

			if (response.ok) {
				metadata = updatedMetadata;
				metadataProp = updatedMetadata;
				if (asset) {
					asset.metadata = updatedMetadata;
				}
			} else {
				const errorData = await response.json();
				console.error('Failed to update metadata:', errorData);
			}
		} catch (error) {
			console.error('Error updating metadata:', error);
		} finally {
			saving = false;
		}
	}

	async function addMetadata() {
		if (!newKey.trim()) return;

		const updatedMetadata = { ...metadata, [newKey.trim()]: parseValue(newValue.trim() || '""') };
		await saveMetadata(updatedMetadata);

		newKey = '';
		newValue = '';
		showAddRow = false;
	}

	async function updateMetadata(key: string) {
		const updatedMetadata = { ...metadata, [key]: parseValue(editingValue.trim()) };
		await saveMetadata(updatedMetadata);

		editingKey = null;
		editingValue = '';
	}

	function promptDeleteMetadata(key: string) {
		keyToDelete = key;
		showDeleteModal = true;
	}

	async function confirmDeleteMetadata() {
		if (!keyToDelete) return;

		const updatedMetadata = { ...metadata };
		delete updatedMetadata[keyToDelete];
		await saveMetadata(updatedMetadata);

		showDeleteModal = false;
		keyToDelete = null;
	}

	function cancelDeleteMetadata() {
		showDeleteModal = false;
		keyToDelete = null;
	}

	function parseValue(value: string): unknown {
		if (!value.trim()) return '';
		try {
			return JSON.parse(value);
		} catch {
			return value;
		}
	}

	function formatValue(value: unknown): string {
		if (typeof value === 'object') {
			return JSON.stringify(value);
		}
		return String(value);
	}

	function formatJson(value: unknown): string {
		return JSON.stringify(value, null, 2);
	}

	function formatDisplayValue(value: unknown): string {
		if (isObject(value)) {
			return formatJson(value);
		}
		return value?.toString() || '';
	}

	function truncatePreview(value: string, maxLength = 160): string {
		return value.length > maxLength ? `${value.slice(0, maxLength)}...` : value;
	}

	function getObjectPreview(value: unknown): string {
		const lines = formatJson(value).split('\n');
		return truncatePreview(lines.slice(0, 4).join('\n') + (lines.length > 4 ? '\n...' : ''));
	}

	function getArrayPreview(value: unknown[]): string {
		const preview = value
			.slice(0, 2)
			.map((item) => formatDisplayValue(item).replace(/\s+/g, ' '))
			.join(' | ');
		return truncatePreview(preview + (value.length > 2 ? ' | ...' : ''));
	}

	function getStringPreview(value: string): string {
		const lines = value.split('\n');
		return truncatePreview(lines.slice(0, 2).join('\n') + (lines.length > 2 ? '\n...' : ''));
	}

	function isExpandableString(value: unknown): value is string {
		return typeof value === 'string' && (value.includes('\n') || value.length > 200);
	}

	function truncateValue(value: string): string {
		if (!maxCharLength || value.length <= maxCharLength) return value;
		return value.slice(0, maxCharLength) + '...';
	}

	function startEditing(key: string, value: unknown) {
		editingKey = key;
		editingValue = formatValue(value);
	}

	function cancelEditing() {
		editingKey = null;
		editingValue = '';
	}

	function cancelAdding() {
		showAddRow = false;
		newKey = '';
		newValue = '';
	}

	const metadataEntries = $derived(Object.entries(metadata));
</script>

{#snippet metadataDisclosure(key: string, label: string, preview: string, content: string)}
	<details
		class="group/details"
		open={expandedDetails[key]}
		ontoggle={(e) => (expandedDetails[key] = e.currentTarget.open)}
	>
		<summary class="flex cursor-pointer list-none items-start gap-space-1 text-mm-text">
			<mm-icon
				name="chevron-right"
				class="mt-[3px] flex-none text-mm-text-2 transition-transform group-open/details:rotate-90"
				aria-hidden="true"
			></mm-icon>
			<span class="min-w-0">
				<span class="block hover:underline">{label}</span>
				<span
					class="metadata-preview block max-w-full {preview.includes('\n')
						? 'line-clamp-2 whitespace-pre-line'
						: 'truncate'} break-all text-caption text-mm-text-2"
				>
					{preview}
				</span>
			</span>
		</summary>
		<pre
			class="mt-space-2 whitespace-pre-wrap break-all rounded-sm bg-mm-sunken p-space-2 font-mono text-mono-dense text-mm-text">{content}</pre>
	</details>
{/snippet}

{#if isReadOnly}
	<mm-metadata-viewer
		class="block"
		{metadata}
		label={m.shared_metadata_label()}
		empty-text={m.shared_metadata_no_config()}
		hide-toolbar={metadataEntries.length < 8}
	></mm-metadata-viewer>
{:else}
	<div class="rounded-md border border-mm-rule bg-mm-surface">
		{#if metadataEntries.length === 0 && !showAddRow}
			<p class="px-space-5 py-space-6 text-center text-caption text-mm-text-2">
				{m.shared_metadata_none_yet()}
			</p>
		{:else}
			<div class="overflow-x-auto" tabindex="-1">
				<table class="min-w-full text-dense">
					<thead>
						<tr class="border-b border-mm-rule">
							<th scope="col" class="{TH} w-1/3">{m.shared_metadata_key_header()}</th>
							<th scope="col" class={TH}>{m.shared_metadata_value_header()}</th>
							{#if canEdit()}
								<th scope="col" class="{TH} w-10"
									><span class="mm-visually-hidden">{m.common_edit()}</span></th
								>
							{/if}
						</tr>
					</thead>
					<tbody class={TBODY}>
						{#each metadataEntries as [key, value] (key)}
							<tr class="group">
								<td class="{TD} align-top font-mono text-mono-dense text-mm-text-2">{key}</td>
								<td class="{TD} align-top">
									{#if editingKey === key}
										<div class="flex flex-wrap items-center gap-space-2">
											<input
												type="text"
												bind:value={editingValue}
												onkeydown={(e) => {
													if (e.key === 'Enter') updateMetadata(key);
													if (e.key === 'Escape') cancelEditing();
												}}
												aria-label={m.shared_metadata_value_header()}
												class="{INPUT} min-w-0 flex-1"
												autofocus
											/>
											<mm-button
												size="sm"
												variant="primary"
												disabled={saving}
												onclick={() => updateMetadata(key)}>{m.common_save()}</mm-button
											>
											<mm-button size="sm" variant="ghost" disabled={saving} onclick={cancelEditing}
												>{m.common_cancel()}</mm-button
											>
											<mm-button
												size="sm"
												variant="ghost"
												icon="delete"
												label={m.shared_metadata_delete_field()}
												disabled={saving}
												onclick={() => promptDeleteMetadata(key)}
											></mm-button>
										</div>
									{:else if isObject(value)}
										{@render metadataDisclosure(
											key,
											m.shared_metadata_view_object(),
											getObjectPreview(value),
											formatJson(value)
										)}
									{:else if isArray(value) && containsObject(value)}
										{@render metadataDisclosure(
											key,
											m.shared_metadata_view_array(),
											getArrayPreview(value),
											formatJson(value)
										)}
									{:else if isArray(value)}
										<span class="break-words text-mm-text"
											>{value.map((item) => formatDisplayValue(item)).join(', ')}</span
										>
									{:else if isExpandableString(value)}
										{@render metadataDisclosure(
											key,
											m.shared_metadata_view_text(),
											getStringPreview(value),
											formatDisplayValue(value)
										)}
									{:else}
										<span class="break-words text-mm-text">{truncateValue(plainValue(value))}</span>
									{/if}
								</td>
								{#if canEdit()}
									<td class="{TD} align-top">
										{#if editingKey !== key}
											<mm-button
												size="sm"
												variant="ghost"
												icon="edit"
												label={m.shared_metadata_edit_field({ key })}
												onclick={() => startEditing(key, value)}
											></mm-button>
										{/if}
									</td>
								{/if}
							</tr>
						{/each}

						{#if showAddRow}
							<tr>
								<td class="{TD} align-top">
									<input
										type="text"
										bind:value={newKey}
										placeholder={m.shared_metadata_key_placeholder()}
										aria-label={m.shared_metadata_key_header()}
										class="{INPUT} font-mono"
										autofocus
									/>
								</td>
								<td class="{TD} align-top" colspan={canEdit() ? 2 : 1}>
									<div class="flex flex-wrap items-center gap-space-2">
										<input
											type="text"
											bind:value={newValue}
											onkeydown={(e) => {
												if (e.key === 'Enter') addMetadata();
												if (e.key === 'Escape') cancelAdding();
											}}
											placeholder={m.shared_metadata_value_placeholder()}
											aria-label={m.shared_metadata_value_header()}
											class="{INPUT} min-w-0 flex-1"
										/>
										<mm-button
											size="sm"
											variant="primary"
											disabled={saving || !newKey.trim()}
											onclick={addMetadata}>{m.shared_metadata_add_field()}</mm-button
										>
										<mm-button size="sm" variant="ghost" disabled={saving} onclick={cancelAdding}
											>{m.common_cancel()}</mm-button
										>
									</div>
								</td>
							</tr>
						{/if}
					</tbody>
				</table>
			</div>
		{/if}

		{#if canEdit() && !showAddRow}
			<div class="border-t border-mm-rule p-space-2">
				<mm-button size="sm" variant="ghost" icon="add" onclick={() => (showAddRow = true)}
					>{m.shared_metadata_add_field()}</mm-button
				>
			</div>
		{/if}
	</div>
{/if}

<DeleteModal
	show={showDeleteModal}
	title={m.shared_metadata_delete_title()}
	message={m.shared_metadata_delete_confirm({ key: keyToDelete ?? '' })}
	confirmText={m.shared_metadata_delete_field()}
	requireConfirmation={false}
	onConfirm={confirmDeleteMetadata}
	onCancel={cancelDeleteMetadata}
/>

<style>
	details[open] .metadata-preview {
		display: none;
	}
</style>
