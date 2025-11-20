<script lang="ts">
	import { fetchApi } from '$lib/api';
	import type { Asset } from '$lib/assets/types';
	import { auth } from '$lib/stores/auth';
	import IconifyIcon from '@iconify/svelte';

	let { asset, editable = true }: { asset: Asset; editable?: boolean } = $props();

	let canManageAssets = $derived(editable && auth.hasPermission('assets', 'manage'));

	let metadata = $state<Record<string, any>>(asset?.metadata || {});
	let showAddInput = $state(false);
	let newKey = $state('');
	let newValue = $state('');
	let editingKey = $state<string | null>(null);
	let editingValue = $state('');
	let saving = $state(false);

	$effect(() => {
		if (asset?.id) {
			metadata = asset.metadata || {};
		}
	});

	async function saveMetadata(updatedMetadata: Record<string, any>) {
		if (!asset?.id) return;

		saving = true;
		try {
			const response = await fetchApi(`/assets/${asset.id}`, {
				method: 'PUT',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({
					metadata: updatedMetadata
				})
			});

			if (response.ok) {
				metadata = updatedMetadata;
				asset.metadata = updatedMetadata;
			} else {
				console.error('Failed to update metadata');
			}
		} catch (error) {
			console.error('Error updating metadata:', error);
		} finally {
			saving = false;
		}
	}

	async function addMetadata() {
		if (!newKey.trim() || !newValue.trim() || !asset?.id) return;

		const updatedMetadata = { ...metadata, [newKey.trim()]: parseValue(newValue.trim()) };
		await saveMetadata(updatedMetadata);

		newKey = '';
		newValue = '';
		showAddInput = false;
	}

	async function updateMetadata(key: string, value: string) {
		if (!value.trim() || !asset?.id) return;

		const updatedMetadata = { ...metadata, [key]: parseValue(value.trim()) };
		await saveMetadata(updatedMetadata);

		editingKey = null;
		editingValue = '';
	}

	async function removeMetadata(key: string) {
		if (!asset?.id) return;

		const updatedMetadata = { ...metadata };
		delete updatedMetadata[key];
		await saveMetadata(updatedMetadata);
	}

	function parseValue(value: string): any {
		// Try to parse as JSON for booleans, numbers, arrays, objects
		try {
			return JSON.parse(value);
		} catch {
			// If parsing fails, return as string
			return value;
		}
	}

	function formatValue(value: any): string {
		if (typeof value === 'object') {
			return JSON.stringify(value);
		}
		return String(value);
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
		showAddInput = false;
		newKey = '';
		newValue = '';
	}

	const metadataEntries = $derived(Object.entries(metadata));
</script>

{#if metadataEntries.length > 0 || canManageAssets}
	<div class="space-y-2">
		<div class="flex items-center justify-between">
			<h3 class="text-sm font-semibold text-gray-700 dark:text-gray-300">Metadata</h3>
			{#if canManageAssets && !showAddInput}
				<button
					onclick={() => (showAddInput = true)}
					class="text-xs text-earthy-terracotta-700 dark:text-earthy-terracotta-700 hover:text-earthy-terracotta-700 dark:hover:text-earthy-terracotta-400 font-medium flex items-center gap-1"
				>
					<IconifyIcon icon="material-symbols:add-rounded" class="w-3.5 h-3.5" />
					Add field
				</button>
			{/if}
		</div>

		{#if metadataEntries.length > 0}
			<div class="space-y-1.5">
				{#each metadataEntries as [key, value]}
					<div
						class="flex items-start gap-2 p-2 rounded bg-gray-50 dark:bg-gray-800 border border-gray-200 dark:border-gray-700"
					>
						<div class="flex-1 min-w-0">
							{#if editingKey === key}
								<div class="space-y-1.5">
									<div class="flex items-center gap-2">
										<span
											class="text-xs font-medium text-gray-600 dark:text-gray-400 px-2 py-0.5 bg-gray-100 dark:bg-gray-700 rounded"
										>
											{key}
										</span>
									</div>
									<div class="flex items-center gap-1.5">
										<input
											type="text"
											bind:value={editingValue}
											onkeydown={(e) => {
												if (e.key === 'Enter') updateMetadata(key, editingValue);
												if (e.key === 'Escape') cancelEditing();
											}}
											placeholder="value (JSON or text)"
											class="flex-1 px-2 py-1 text-xs border border-gray-300 dark:border-gray-600 rounded bg-white dark:bg-gray-900 text-gray-900 dark:text-gray-100 focus:ring-1 focus:ring-earthy-terracotta-600 focus:border-transparent"
											autofocus
										/>
										<button
											onclick={() => updateMetadata(key, editingValue)}
											disabled={saving || !editingValue.trim()}
											class="text-xs text-green-600 dark:text-green-400 hover:text-green-700 dark:hover:text-green-300 disabled:opacity-50 px-2 py-1"
										>
											Save
										</button>
										<button
											onclick={cancelEditing}
											class="text-xs text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300 px-2 py-1"
										>
											Cancel
										</button>
									</div>
								</div>
							{:else}
								<div class="flex items-start gap-2">
									<span
										class="text-xs font-medium text-gray-600 dark:text-gray-400 shrink-0"
									>
										{key}:
									</span>
									<span class="text-xs text-gray-900 dark:text-gray-100 break-all">
										{formatValue(value)}
									</span>
								</div>
							{/if}
						</div>

						{#if canManageAssets && editingKey !== key}
							<div class="flex items-center gap-1 shrink-0">
								<button
									onclick={() => startEditing(key, value)}
									class="text-gray-400 hover:text-earthy-terracotta-700 dark:hover:text-earthy-terracotta-700 p-1"
									title="Edit value"
								>
									<IconifyIcon icon="material-symbols:edit-outline-rounded" class="w-3.5 h-3.5" />
								</button>
								<button
									onclick={() => removeMetadata(key)}
									class="text-gray-400 hover:text-red-600 dark:hover:text-red-400 p-1"
									title="Remove field"
								>
									<IconifyIcon icon="material-symbols:delete-outline-rounded" class="w-3.5 h-3.5" />
								</button>
							</div>
						{/if}
					</div>
				{/each}
			</div>
		{/if}

		{#if showAddInput}
			<div class="p-3 rounded bg-earthy-terracotta-50 dark:bg-earthy-terracotta-900/10 border border-earthy-terracotta-200 dark:border-earthy-terracotta-800">
				<div class="space-y-2">
					<input
						type="text"
						bind:value={newKey}
						placeholder="Field name (e.g., owner, cost_center)"
						class="w-full px-2 py-1.5 text-xs border border-gray-300 dark:border-gray-600 rounded bg-white dark:bg-gray-900 text-gray-900 dark:text-gray-100 focus:ring-1 focus:ring-earthy-terracotta-600 focus:border-transparent"
						autofocus
					/>
					<input
						type="text"
						bind:value={newValue}
						onkeydown={(e) => e.key === 'Enter' && addMetadata()}
						placeholder='Value (text, number, true/false, {"key": "value"})'
						class="w-full px-2 py-1.5 text-xs border border-gray-300 dark:border-gray-600 rounded bg-white dark:bg-gray-900 text-gray-900 dark:text-gray-100 focus:ring-1 focus:ring-earthy-terracotta-600 focus:border-transparent"
					/>
					<div class="flex items-center gap-2">
						<button
							onclick={addMetadata}
							disabled={saving || !newKey.trim() || !newValue.trim()}
							class="px-3 py-1.5 text-xs bg-earthy-terracotta-700 dark:bg-earthy-terracotta-600 text-white rounded hover:bg-earthy-terracotta-800 dark:hover:bg-earthy-terracotta-700 disabled:opacity-50 disabled:cursor-not-allowed"
						>
							{saving ? 'Adding...' : 'Add Field'}
						</button>
						<button
							onclick={cancelAdding}
							class="px-3 py-1.5 text-xs text-gray-600 dark:text-gray-400 hover:text-gray-800 dark:hover:text-gray-200"
						>
							Cancel
						</button>
					</div>
				</div>
			</div>
		{/if}

		{#if metadataEntries.length === 0 && !showAddInput && !canManageAssets}
			<p class="text-xs text-gray-500 dark:text-gray-400 italic">No metadata</p>
		{/if}
	</div>
{/if}
