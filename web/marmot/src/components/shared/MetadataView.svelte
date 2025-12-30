<script lang="ts">
	import { fetchApi } from '$lib/api';
	import { auth } from '$lib/stores/auth';
	import type { Asset } from '$lib/assets/types';
	import IconifyIcon from '@iconify/svelte';
	import Arrow from '$components/ui/Arrow.svelte';
	import DeleteModal from '$components/ui/DeleteModal.svelte';

	let {
		asset = undefined,
		metadata: metadataProp = $bindable(undefined),
		readOnly = false,
		maxDepth = 1,
		maxCharLength = undefined,
		showDetailsLink = undefined,
		endpoint = undefined,
		id = undefined,
		permissionResource = undefined,
		permissionAction = undefined
	}: {
		asset?: Asset;
		metadata?: Record<string, any>;
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
	let metadata = $state<Record<string, any>>(metadataProp || asset?.metadata || {});

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

	function isObject(value: any): boolean {
		return typeof value === 'object' && value !== null && !Array.isArray(value);
	}

	function isArray(value: any): boolean {
		return Array.isArray(value);
	}

	function getValueClass(value: any): string {
		if (typeof value === 'boolean') {
			return value
				? 'bg-green-100 dark:bg-green-900/30 text-green-800 dark:text-green-200'
				: 'bg-red-100 dark:bg-red-900/30 text-red-800 dark:text-red-200';
		}
		if (typeof value === 'number')
			return 'bg-blue-100 dark:bg-blue-900/30 text-blue-800 dark:text-blue-200';
		if (typeof value === 'string')
			return 'bg-gray-100 dark:bg-gray-700 text-gray-800 dark:text-gray-200';
		return '';
	}

	function toggleDetails(key: string) {
		expandedDetails[key] = !expandedDetails[key];
	}

	async function saveMetadata(updatedMetadata: Record<string, any>) {
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

	function parseValue(value: string): any {
		if (!value.trim()) return '';
		try {
			return JSON.parse(value);
		} catch {
			return value;
		}
	}

	function formatValue(value: any): string {
		if (typeof value === 'object') {
			return JSON.stringify(value);
		}
		return String(value);
	}

	function truncateValue(value: string): string {
		if (!maxCharLength || value.length <= maxCharLength) return value;
		return value.slice(0, maxCharLength) + '...';
	}

	function startEditing(key: string, value: any) {
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

	function renderValue(value: any, depth: number = 0): any {
		// For read-only mode, render nested objects more nicely
		if (isReadOnly && depth < maxDepth) {
			if (isObject(value)) {
				return Object.entries(value);
			}
		}
		return value;
	}

	const metadataEntries = $derived(Object.entries(metadata));
</script>

{#if isReadOnly}
	<!-- Read-Only Mode -->
	<div class="space-y-2">
		{#if metadataEntries.length === 0}
			<div class="text-center py-6">
				<p class="text-sm text-gray-500 dark:text-gray-400">No configuration data</p>
			</div>
		{:else}
			{#each metadataEntries as [key, value]}
				<div class="border-b border-gray-200 dark:border-gray-700 last:border-0 pb-2 last:pb-0">
					<div class="flex items-start gap-2">
						<dt
							class="text-sm font-medium text-gray-600 dark:text-gray-400 min-w-[120px] flex-shrink-0"
						>
							{key}
						</dt>
						<dd class="text-sm text-gray-900 dark:text-gray-100 flex-1 min-w-0">
							{#if isObject(value)}
								<details class="group">
									<summary
										class="cursor-pointer text-earthy-terracotta-700 dark:text-earthy-terracotta-500 hover:text-earthy-terracotta-800 dark:hover:text-earthy-terracotta-600 flex items-center"
									>
										<Arrow expanded={false} />
										<span class="ml-1">View Details</span>
									</summary>
									<div class="mt-2 p-2 bg-gray-50 dark:bg-gray-900 rounded">
										<pre
											class="text-xs text-gray-800 dark:text-gray-200 overflow-x-auto">{JSON.stringify(
												value,
												null,
												2
											)}</pre>
									</div>
								</details>
							{:else if isArray(value)}
								<div class="flex flex-wrap gap-1">
									{#each value as item, i}
										{#if i < 5}
											<span
												class="px-2 py-0.5 text-xs bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300 rounded"
											>
												{truncateValue(item?.toString() || '')}
											</span>
										{/if}
									{/each}
									{#if value.length > 5}
										<span class="text-xs text-gray-500 dark:text-gray-400">
											+{value.length - 5} more
										</span>
									{/if}
								</div>
							{:else}
								<span class="font-mono break-all">
									{truncateValue(value?.toString() || '')}
								</span>
							{/if}
						</dd>
					</div>
				</div>
			{/each}
		{/if}
	</div>
{:else}
	<!-- Editable Mode -->
	<div class="space-y-4">
		<div class="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700">
			<div class="overflow-x-auto">
				{#if metadataEntries.length === 0 && !showAddRow}
					<div class="px-6 py-12 text-center">
						<div class="flex flex-col items-center gap-3">
							<IconifyIcon
								icon="material-symbols:database-outline"
								class="w-12 h-12 text-gray-300 dark:text-gray-600"
							/>
							<p class="text-sm text-gray-500 dark:text-gray-400">No metadata fields yet</p>
						</div>
					</div>
				{:else}
					<table class="min-w-full">
						<thead>
							<tr class="border-b border-gray-200 dark:border-gray-700">
								<th
									class="px-4 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-400"
								>
									Key
								</th>
								<th
									class="px-4 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-400"
								>
									Value
								</th>
								{#if canEdit()}
									<th class="px-4 py-2 w-10"></th>
								{/if}
							</tr>
						</thead>
						<tbody>
							{#each metadataEntries as [key, value]}
								<tr
									class="group border-b border-gray-200 dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-700/30 transition-colors"
								>
									<td
										class="px-4 py-2.5 text-sm font-medium text-gray-700 dark:text-gray-300 align-top"
									>
										{key}
									</td>
									<td class="px-4 py-2.5 text-sm align-top">
										{#if editingKey === key}
											<div class="flex items-start gap-2">
												<input
													type="text"
													bind:value={editingValue}
													onkeydown={(e) => {
														if (e.key === 'Enter') updateMetadata(key);
														if (e.key === 'Escape') cancelEditing();
													}}
													class="flex-1 px-2 py-1.5 text-sm border border-earthy-terracotta-500 dark:border-earthy-terracotta-700 rounded bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 focus:ring-1 focus:ring-earthy-terracotta-600"
													autofocus
												/>
												<div class="flex items-center gap-1 flex-shrink-0">
													<button
														onclick={() => updateMetadata(key)}
														disabled={saving}
														class="p-1.5 text-green-600 dark:text-green-500 hover:bg-green-50 dark:hover:bg-green-900/20 rounded disabled:opacity-50 transition-colors"
														title="Save"
													>
														<IconifyIcon icon="material-symbols:check-rounded" class="w-5 h-5" />
													</button>
													<button
														onclick={cancelEditing}
														disabled={saving}
														class="p-1.5 text-gray-500 hover:bg-gray-100 dark:hover:bg-gray-700 rounded transition-colors"
														title="Cancel"
													>
														<IconifyIcon icon="material-symbols:close-rounded" class="w-5 h-5" />
													</button>
													<button
														onclick={() => promptDeleteMetadata(key)}
														disabled={saving}
														class="p-1.5 text-red-600 dark:text-red-500 hover:bg-red-50 dark:hover:bg-red-900/20 rounded transition-colors"
														title="Delete"
													>
														<IconifyIcon
															icon="material-symbols:delete-outline-rounded"
															class="w-5 h-5"
														/>
													</button>
												</div>
											</div>
										{:else if isObject(value)}
											<details class="group/details">
												<summary
													class="cursor-pointer text-earthy-terracotta-700 dark:text-earthy-terracotta-500 hover:text-earthy-terracotta-800 dark:hover:text-earthy-terracotta-600 flex items-center text-sm"
													onclick={(e) => {
														e.preventDefault();
														toggleDetails(key);
													}}
												>
													<Arrow expanded={expandedDetails[key]} />
													<span class="ml-1">View object</span>
												</summary>
												<div class="mt-2 p-2 bg-gray-50 dark:bg-gray-900 rounded text-xs">
													<pre
														class="text-gray-800 dark:text-gray-200 overflow-x-auto">{JSON.stringify(
															value,
															null,
															2
														)}</pre>
												</div>
											</details>
										{:else if isArray(value)}
											<div class="flex flex-wrap gap-1.5">
												{#each value as item}
													<span
														class="px-2 py-0.5 text-xs bg-earthy-terracotta-100 dark:bg-earthy-terracotta-900 text-earthy-terracotta-700 dark:text-earthy-terracotta-100 rounded"
													>
														{item?.toString() || ''}
													</span>
												{/each}
											</div>
										{:else}
											<span class="px-2 py-1 text-sm rounded-full {getValueClass(value)}">
												{value?.toString() || ''}
											</span>
										{/if}
									</td>
									{#if canEdit()}
										<td class="px-4 py-2.5 align-top">
											{#if editingKey !== key}
												<button
													onclick={() => startEditing(key, value)}
													class="opacity-0 group-hover:opacity-100 p-1.5 text-gray-400 hover:text-earthy-terracotta-700 dark:hover:text-earthy-terracotta-500 hover:bg-gray-100 dark:hover:bg-gray-700 rounded transition-all"
													title="Edit"
												>
													<IconifyIcon
														icon="material-symbols:edit-outline-rounded"
														class="w-4 h-4"
													/>
												</button>
											{/if}
										</td>
									{/if}
								</tr>
							{/each}

							{#if showAddRow}
								<tr
									class="group border-b border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-700/30"
								>
									<td class="px-4 py-2">
										<input
											type="text"
											bind:value={newKey}
											placeholder="Key"
											class="w-full px-2 py-1.5 text-sm border-0 bg-transparent text-gray-900 dark:text-gray-100 focus:ring-1 focus:ring-earthy-terracotta-600 rounded"
											autofocus
										/>
									</td>
									<td class="px-4 py-2">
										<input
											type="text"
											bind:value={newValue}
											onkeydown={(e) => e.key === 'Enter' && addMetadata()}
											placeholder="Value"
											class="w-full px-2 py-1.5 text-sm border-0 bg-transparent text-gray-900 dark:text-gray-100 focus:ring-1 focus:ring-earthy-terracotta-600 rounded"
										/>
									</td>
									<td class="px-4 py-2">
										<div class="flex items-center gap-1">
											<button
												onclick={addMetadata}
												disabled={saving || !newKey.trim()}
												class="p-1.5 text-green-600 dark:text-green-500 hover:bg-green-50 dark:hover:bg-green-900/20 rounded disabled:opacity-50 transition-colors"
												title="Save"
											>
												<IconifyIcon icon="material-symbols:check-rounded" class="w-5 h-5" />
											</button>
											<button
												onclick={cancelAdding}
												disabled={saving}
												class="p-1.5 text-gray-500 hover:bg-gray-100 dark:hover:bg-gray-700 rounded transition-colors"
												title="Cancel"
											>
												<IconifyIcon icon="material-symbols:close-rounded" class="w-5 h-5" />
											</button>
										</div>
									</td>
								</tr>
							{/if}
						</tbody>
					</table>
				{/if}
			</div>

			{#if canEdit() && !showAddRow}
				<div class="border-t border-gray-200 dark:border-gray-700 p-2">
					<button
						onclick={() => (showAddRow = true)}
						class="w-full flex items-center gap-2 px-3 py-2 text-sm text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100 hover:bg-gray-50 dark:hover:bg-gray-700/30 rounded transition-colors"
					>
						<IconifyIcon icon="material-symbols:add-rounded" class="w-4 h-4" />
						<span>Add field</span>
					</button>
				</div>
			{/if}
		</div>
	</div>
{/if}

<DeleteModal
	show={showDeleteModal}
	title="Delete Metadata Field"
	message="Are you sure you want to delete the metadata field '{keyToDelete}'? This action cannot be undone."
	confirmText="Delete"
	requireConfirmation={false}
	onConfirm={confirmDeleteMetadata}
	onCancel={cancelDeleteMetadata}
/>
