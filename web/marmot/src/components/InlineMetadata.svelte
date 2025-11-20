<script lang="ts">
	import { fetchApi } from '$lib/api';
	import type { Asset } from '$lib/assets/types';
	import { auth } from '$lib/stores/auth';
	import IconifyIcon from '@iconify/svelte';

	let { asset }: { asset: Asset } = $props();

	let canManageAssets = $derived(auth.hasPermission('assets', 'manage'));
	let metadata = $state<Record<string, any>>(asset?.metadata || {});
	let showAddInput = $state(false);
	let editingKey = $state<string | null>(null);
	let editingValue = $state('');
	let newKey = $state('');
	let newValue = $state('');
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

	async function addMetadata(event: Event) {
		event.stopPropagation();
		if (!newKey.trim() || !newValue.trim() || !asset?.id) return;

		const updatedMetadata = { ...metadata, [newKey.trim()]: parseValue(newValue.trim()) };
		await saveMetadata(updatedMetadata);

		newKey = '';
		newValue = '';
		showAddInput = false;
	}

	async function updateMetadata(key: string, value: string, event: Event) {
		event.stopPropagation();
		if (!value.trim() || !asset?.id) return;

		const updatedMetadata = { ...metadata, [key]: parseValue(value.trim()) };
		await saveMetadata(updatedMetadata);

		editingKey = null;
		editingValue = '';
	}

	async function removeMetadata(key: string, event: Event) {
		event.stopPropagation();
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

	function startEditing(key: string, value: any, event: Event) {
		event.stopPropagation();
		editingKey = key;
		editingValue = formatValue(value);
	}

	function cancelEditing(event: Event) {
		event.stopPropagation();
		editingKey = null;
		editingValue = '';
	}

	function toggleAddInput(event: Event) {
		event.stopPropagation();
		showAddInput = !showAddInput;
		newKey = '';
		newValue = '';
	}

	const metadataEntries = $derived(Object.entries(metadata));
	const hasMetadata = $derived(metadataEntries.length > 0);
</script>

{#if hasMetadata || canManageAssets}
	<div class="space-y-1.5" onclick={(e) => e.stopPropagation()}>
		<div class="flex items-center justify-between">
			<span class="text-xs font-medium text-gray-600 dark:text-gray-400">Metadata</span>
			{#if canManageAssets && !showAddInput}
				<button
					onclick={toggleAddInput}
					class="text-xs text-earthy-terracotta-700 dark:text-earthy-terracotta-700 hover:text-earthy-terracotta-700 dark:hover:text-earthy-terracotta-400 flex items-center gap-0.5"
					title="Add metadata field"
				>
					<IconifyIcon icon="material-symbols:add-rounded" class="w-3 h-3" />
					Add
				</button>
			{/if}
		</div>

		{#if hasMetadata}
			<div class="flex flex-wrap gap-1.5">
				{#each metadataEntries as [key, value]}
					{#if editingKey === key}
						<div class="inline-flex items-center gap-1 bg-earthy-terracotta-50 dark:bg-earthy-terracotta-900/20 border border-earthy-terracotta-300 dark:border-earthy-terracotta-800 rounded px-1.5 py-0.5">
							<span class="text-xs font-medium text-earthy-terracotta-700 dark:text-earthy-terracotta-400">{key}:</span>
							<input
								type="text"
								bind:value={editingValue}
								onkeydown={(e) => {
									e.stopPropagation();
									if (e.key === 'Enter') updateMetadata(key, editingValue, e);
									if (e.key === 'Escape') cancelEditing(e);
								}}
								onclick={(e) => e.stopPropagation()}
								class="text-xs bg-white dark:bg-gray-800 border border-earthy-terracotta-500 dark:border-earthy-terracotta-700 rounded px-1 py-0.5 w-24 focus:outline-none focus:ring-1 focus:ring-earthy-terracotta-600"
								autofocus
							/>
							<button
								onclick={(e) => updateMetadata(key, editingValue, e)}
								disabled={saving}
								class="text-green-600 dark:text-green-400 hover:text-green-700 dark:hover:text-green-300 disabled:opacity-50"
								title="Save"
							>
								<IconifyIcon icon="material-symbols:check-rounded" class="w-3 h-3" />
							</button>
							<button
								onclick={cancelEditing}
								class="text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300"
								title="Cancel"
							>
								<IconifyIcon icon="material-symbols:close-rounded" class="w-3 h-3" />
							</button>
						</div>
					{:else}
						<div
							class="inline-flex items-center gap-1 bg-gray-100 dark:bg-gray-700 rounded px-2 py-0.5 group"
						>
							<span class="text-xs font-medium text-gray-600 dark:text-gray-400">{key}:</span>
							<span class="text-xs text-gray-900 dark:text-gray-100 max-w-[120px] truncate" title={formatValue(value)}>
								{formatValue(value)}
							</span>
							{#if canManageAssets}
								<div class="flex items-center gap-0.5 opacity-0 group-hover:opacity-100 transition-opacity">
									<button
										onclick={(e) => startEditing(key, value, e)}
										class="text-gray-400 hover:text-earthy-terracotta-700 dark:hover:text-earthy-terracotta-700"
										title="Edit"
									>
										<IconifyIcon icon="material-symbols:edit-outline-rounded" class="w-3 h-3" />
									</button>
									<button
										onclick={(e) => removeMetadata(key, e)}
										class="text-gray-400 hover:text-red-600 dark:hover:text-red-400"
										title="Delete"
									>
										<IconifyIcon icon="material-symbols:delete-outline-rounded" class="w-3 h-3" />
									</button>
								</div>
							{/if}
						</div>
					{/if}
				{/each}
			</div>
		{/if}

		{#if showAddInput}
			<div class="bg-earthy-terracotta-50 dark:bg-earthy-terracotta-900/10 border border-earthy-terracotta-200 dark:border-earthy-terracotta-800 rounded p-2 space-y-1.5">
				<input
					type="text"
					bind:value={newKey}
					placeholder="Key"
					onclick={(e) => e.stopPropagation()}
					onkeydown={(e) => e.stopPropagation()}
					class="w-full text-xs bg-white dark:bg-gray-800 border border-gray-300 dark:border-gray-600 rounded px-2 py-1 focus:outline-none focus:ring-1 focus:ring-earthy-terracotta-600"
					autofocus
				/>
				<input
					type="text"
					bind:value={newValue}
					placeholder="Value (text, number, true, false, JSON)"
					onclick={(e) => e.stopPropagation()}
					onkeydown={(e) => {
						e.stopPropagation();
						if (e.key === 'Enter') addMetadata(e);
					}}
					class="w-full text-xs bg-white dark:bg-gray-800 border border-gray-300 dark:border-gray-600 rounded px-2 py-1 focus:outline-none focus:ring-1 focus:ring-earthy-terracotta-600"
				/>
				<div class="flex gap-1.5">
					<button
						onclick={addMetadata}
						disabled={saving || !newKey.trim() || !newValue.trim()}
						class="flex-1 text-xs bg-earthy-terracotta-700 dark:bg-earthy-terracotta-600 text-white rounded px-2 py-1 hover:bg-earthy-terracotta-800 dark:hover:bg-earthy-terracotta-700 disabled:opacity-50"
					>
						{saving ? 'Adding...' : 'Add'}
					</button>
					<button
						onclick={toggleAddInput}
						class="flex-1 text-xs bg-gray-200 dark:bg-gray-700 text-gray-700 dark:text-gray-300 rounded px-2 py-1 hover:bg-gray-300 dark:hover:bg-gray-600"
					>
						Cancel
					</button>
				</div>
			</div>
		{/if}
	</div>
{/if}
