<script lang="ts">
	import { fetchApi } from '$lib/api';
	import IconifyIcon from '@iconify/svelte';

	let {
		tags = $bindable([]),
		endpoint,
		id,
		canEdit = false,
		saving = $bindable(false)
	}: {
		tags: string[];
		endpoint?: string;
		id?: string;
		canEdit?: boolean;
		saving?: boolean;
	} = $props();

	let newTag = $state('');
	let showTagInput = $state(false);

	// Local-only mode: when no endpoint/id, just update the bound tags array directly
	const isLocalMode = $derived(!endpoint || !id);

	async function addTag() {
		if (!newTag.trim()) return;

		const tagToAdd = newTag.trim();

		// Local-only mode: just update the tags array directly
		if (isLocalMode) {
			if (!tags.includes(tagToAdd)) {
				tags = [...tags, tagToAdd];
			}
			newTag = '';
			showTagInput = false;
			return;
		}

		saving = true;
		try {
			const updatedTags = [...tags, tagToAdd];

			const response = await fetchApi(`${endpoint}/${id}`, {
				method: 'PUT',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({
					tags: updatedTags
				})
			});

			if (response.ok) {
				tags = updatedTags;
				newTag = '';
				showTagInput = false;
			} else {
				console.error('Failed to add tag');
			}
		} catch (error) {
			console.error('Error adding tag:', error);
		} finally {
			saving = false;
		}
	}

	async function removeTag(tag: string) {
		// Local-only mode: just update the tags array directly
		if (isLocalMode) {
			tags = tags.filter((t) => t !== tag);
			return;
		}

		saving = true;
		try {
			const updatedTags = tags.filter((t) => t !== tag);

			const response = await fetchApi(`${endpoint}/${id}`, {
				method: 'PUT',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({
					tags: updatedTags
				})
			});

			if (response.ok) {
				tags = updatedTags;
			} else {
				console.error('Failed to remove tag');
			}
		} catch (error) {
			console.error('Error removing tag:', error);
		} finally {
			saving = false;
		}
	}

	function cancelAdding() {
		showTagInput = false;
		newTag = '';
	}
</script>

<div class="space-y-2">
	<div class="flex flex-wrap gap-1.5">
		{#each tags as tag}
			<span
				class="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-md text-xs font-medium bg-earthy-terracotta-100 dark:bg-earthy-terracotta-900 text-earthy-terracotta-700 dark:text-earthy-terracotta-100"
			>
				{tag}
				{#if canEdit}
					<button
						onclick={() => removeTag(tag)}
						disabled={saving}
						class="hover:text-red-600 dark:hover:text-red-400 disabled:opacity-50"
						aria-label="Remove tag {tag}"
					>
						<IconifyIcon
							icon="material-symbols:close-small-rounded"
							class="w-4 h-4"
							aria-hidden="true"
						/>
					</button>
				{/if}
			</span>
		{/each}

		{#if showTagInput}
			<div class="inline-flex items-center gap-1 bg-gray-50 dark:bg-gray-700 rounded-md px-2 py-1">
				<input
					type="text"
					bind:value={newTag}
					onkeydown={(e) => {
						if (e.key === 'Enter') addTag();
						if (e.key === 'Escape') cancelAdding();
					}}
					placeholder="New tag..."
					aria-label="Enter new tag"
					class="w-24 text-xs bg-transparent border-0 focus:ring-0 focus:outline-none text-gray-900 dark:text-gray-100 placeholder-gray-400 dark:placeholder-gray-500"
					autofocus
				/>
				<button
					onclick={addTag}
					disabled={saving || !newTag.trim()}
					class="p-0.5 text-green-600 dark:text-green-500 hover:bg-green-50 dark:hover:bg-green-900/20 rounded disabled:opacity-50"
					title="Add"
					aria-label="Add tag"
				>
					<IconifyIcon icon="material-symbols:check-rounded" class="w-4 h-4" aria-hidden="true" />
				</button>
				<button
					onclick={cancelAdding}
					disabled={saving}
					class="p-0.5 text-gray-500 hover:bg-gray-100 dark:hover:bg-gray-700 rounded"
					title="Cancel"
					aria-label="Cancel adding tag"
				>
					<IconifyIcon icon="material-symbols:close-rounded" class="w-4 h-4" aria-hidden="true" />
				</button>
			</div>
		{/if}

		{#if canEdit && !showTagInput}
			<button
				onclick={() => (showTagInput = true)}
				disabled={saving}
				class="inline-flex items-center gap-1 px-2.5 py-1 rounded-md text-xs font-medium text-earthy-terracotta-700 dark:text-earthy-terracotta-500 bg-earthy-terracotta-50 dark:bg-earthy-terracotta-900/20 hover:bg-earthy-terracotta-100 dark:hover:bg-earthy-terracotta-900/30 disabled:opacity-50 transition-colors"
			>
				<IconifyIcon icon="material-symbols:add-rounded" class="w-3.5 h-3.5" aria-hidden="true" />
				Add tag
			</button>
		{/if}
	</div>

	{#if tags.length === 0 && !canEdit}
		<p class="text-sm text-gray-400 dark:text-gray-500 italic">No tags</p>
	{/if}
</div>
