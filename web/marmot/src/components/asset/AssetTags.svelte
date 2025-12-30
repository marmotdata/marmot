<script lang="ts">
	import { fetchApi } from '$lib/api';
	import type { Asset } from '$lib/assets/types';
	import { auth } from '$lib/stores/auth';

	let { asset, editable = true }: { asset: Asset; editable?: boolean } = $props();

	let canManageAssets = $derived(editable && auth.hasPermission('assets', 'manage'));

	let tags: string[] = $state(asset?.tags || []);
	let newTag = $state('');
	let showTagInput = $state(false);
	let savingTag = $state(false);
	let tagInputElement = $state<HTMLInputElement>();

	$effect(() => {
		if (asset?.id) {
			tags = asset.tags || [];
		}
	});

	$effect(() => {
		if (showTagInput && tagInputElement) {
			tagInputElement.focus();
		}
	});

	async function addTag() {
		if (!newTag.trim() || !asset?.id) return;

		savingTag = true;
		try {
			const updatedTags = [...tags, newTag.trim()];

			const response = await fetchApi(`/assets/${asset.id}`, {
				method: 'PUT',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({
					tags: updatedTags
				})
			});

			if (response.ok) {
				tags = updatedTags;
				asset.tags = updatedTags;
				newTag = '';
				showTagInput = false;
			} else {
				console.error('Failed to add tag');
			}
		} catch (error) {
			console.error('Error adding tag:', error);
		} finally {
			savingTag = false;
		}
	}

	async function removeTag(tag: string) {
		if (!asset?.id) return;

		try {
			const updatedTags = tags.filter((t) => t !== tag);

			const response = await fetchApi(`/assets/${asset.id}`, {
				method: 'PUT',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({
					tags: updatedTags
				})
			});

			if (response.ok) {
				tags = updatedTags;
				asset.tags = updatedTags;
			} else {
				console.error('Failed to remove tag');
			}
		} catch (error) {
			console.error('Error removing tag:', error);
		}
	}
</script>

{#if tags.length > 0 || canManageAssets}
	<div class="mt-2">
		{#if canManageAssets && !showTagInput && tags.length === 0}
			<button
				onclick={() => (showTagInput = true)}
				class="text-xs text-earthy-terracotta-700 dark:text-earthy-terracotta-700 hover:text-earthy-terracotta-700 dark:hover:text-earthy-terracotta-400 font-medium"
			>
				+ Add tag
			</button>
		{/if}

		{#if tags.length > 0 || showTagInput}
			<div class="flex flex-wrap gap-1.5">
				{#each tags as tag, index (index)}
					<span
						class="inline-flex items-center gap-1 px-2 py-0.5 rounded text-xs font-medium bg-gray-100 dark:bg-gray-700 text-gray-800 dark:text-gray-200"
					>
						{tag}
						{#if canManageAssets}
							<button
								onclick={() => removeTag(tag)}
								class="hover:text-red-600 dark:hover:text-red-400"
								aria-label="Remove tag"
							>
								<svg class="w-3 h-3" fill="currentColor" viewBox="0 0 20 20">
									<path
										fill-rule="evenodd"
										d="M4.293 4.293a1 1 0 011.414 0L10 8.586l4.293-4.293a1 1 0 111.414 1.414L11.414 10l4.293 4.293a1 1 0 01-1.414 1.414L10 11.414l-4.293 4.293a1 1 0 01-1.414-1.414L8.586 10 4.293 5.707a1 1 0 010-1.414z"
										clip-rule="evenodd"
									/>
								</svg>
							</button>
						{/if}
					</span>
				{/each}

				{#if showTagInput}
					<div class="inline-flex items-center gap-1.5">
						<input
							bind:this={tagInputElement}
							type="text"
							bind:value={newTag}
							onkeydown={(e) => e.key === 'Enter' && addTag()}
							placeholder="tag name"
							class="px-2 py-0.5 text-xs border border-gray-300 dark:border-gray-600 rounded bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 focus:ring-1 focus:ring-earthy-terracotta-600 focus:border-transparent w-24"
						/>
						<button
							onclick={addTag}
							disabled={savingTag || !newTag.trim()}
							class="text-xs text-green-600 dark:text-green-400 hover:text-green-700 dark:hover:text-green-300 disabled:opacity-50"
						>
							Add
						</button>
						<button
							onclick={() => {
								showTagInput = false;
								newTag = '';
							}}
							class="text-xs text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300"
						>
							✕
						</button>
					</div>
				{/if}

				{#if canManageAssets && !showTagInput && tags.length > 0}
					<button
						onclick={() => (showTagInput = true)}
						class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium text-earthy-terracotta-700 dark:text-earthy-terracotta-700 hover:bg-earthy-terracotta-50 dark:hover:bg-earthy-terracotta-900/20"
					>
						+
					</button>
				{/if}
			</div>
		{/if}
	</div>
{/if}
