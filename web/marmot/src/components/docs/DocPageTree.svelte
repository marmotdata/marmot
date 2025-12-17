<script lang="ts">
	import Icon from '@iconify/svelte';
	import type { Page } from '$lib/docs/types';
	import { createEventDispatcher } from 'svelte';

	export let pages: Page[] = [];
	export let selectedPageId: string | null = null;
	export let canEdit: boolean = false;
	export let depth: number = 0;

	const dispatch = createEventDispatcher<{
		select: { page: Page };
		create: { parentId: string | null };
		delete: { page: Page };
	}>();

	let expandedPages: Set<string> = new Set();

	function toggleExpanded(pageId: string) {
		if (expandedPages.has(pageId)) {
			expandedPages.delete(pageId);
		} else {
			expandedPages.add(pageId);
		}
		expandedPages = expandedPages;
	}

	function handleSelect(page: Page) {
		dispatch('select', { page });
	}

	function handleCreate(parentId: string | null) {
		dispatch('create', { parentId });
	}

	function handleDelete(page: Page, event: Event) {
		event.stopPropagation();
		dispatch('delete', { page });
	}

	function formatDate(dateString: string): string {
		return new Date(dateString).toLocaleDateString('en-US', {
			month: 'short',
			day: 'numeric'
		});
	}
</script>

<div class="space-y-0.5">
	{#each pages as page (page.id)}
		{@const hasChildren = page.children && page.children.length > 0}
		{@const isExpanded = expandedPages.has(page.id)}
		{@const isSelected = selectedPageId === page.id}

		<div>
			<div
				class="group flex items-center gap-1 py-1 px-1 rounded-md cursor-pointer transition-colors
					{isSelected
					? 'bg-earthy-terracotta-100 dark:bg-earthy-terracotta-900/30 text-earthy-terracotta-800 dark:text-earthy-terracotta-300'
					: 'hover:bg-gray-100 dark:hover:bg-gray-800 text-gray-700 dark:text-gray-300'}"
				style="padding-left: {depth * 12 + 4}px"
				on:click={() => handleSelect(page)}
				on:keydown={(e) => e.key === 'Enter' && handleSelect(page)}
				role="button"
				tabindex="0"
			>
				<!-- Expand/collapse toggle -->
				{#if hasChildren}
					<button
						type="button"
						class="p-0.5 rounded hover:bg-gray-200 dark:hover:bg-gray-700"
						on:click|stopPropagation={() => toggleExpanded(page.id)}
					>
						<Icon
							icon={isExpanded ? 'mdi:chevron-down' : 'mdi:chevron-right'}
							class="h-3.5 w-3.5 text-gray-400"
						/>
					</button>
				{:else}
					<div class="w-4"></div>
				{/if}

				<!-- Page emoji or icon -->
				{#if page.emoji}
					<span class="text-sm flex-shrink-0">{page.emoji}</span>
				{:else}
					<Icon
						icon={hasChildren ? 'mdi:folder-outline' : 'mdi:file-document-outline'}
						class="h-3.5 w-3.5 flex-shrink-0 {isSelected
							? 'text-earthy-terracotta-600 dark:text-earthy-terracotta-400'
							: 'text-gray-400'}"
					/>
				{/if}

				<!-- Page title -->
				<span class="flex-1 truncate text-sm">{page.title}</span>

				<!-- Actions (visible on hover or when selected) -->
				{#if canEdit}
					<div
						class="flex items-center gap-0.5 opacity-0 group-hover:opacity-100 {isSelected
							? 'opacity-100'
							: ''}"
					>
						<button
							type="button"
							class="p-0.5 rounded hover:bg-gray-200 dark:hover:bg-gray-700"
							on:click|stopPropagation={() => handleCreate(page.id)}
							title="Add sub-page"
						>
							<Icon icon="mdi:plus" class="h-3 w-3 text-gray-500" />
						</button>
						<button
							type="button"
							class="p-0.5 rounded hover:bg-red-100 dark:hover:bg-red-900/20"
							on:click={(e) => handleDelete(page, e)}
							title="Delete"
						>
							<Icon icon="mdi:delete-outline" class="h-3 w-3 text-red-500" />
						</button>
					</div>
				{/if}
			</div>

			<!-- Children -->
			{#if hasChildren && isExpanded}
				<svelte:self
					pages={page.children || []}
					{selectedPageId}
					{canEdit}
					depth={depth + 1}
					on:select
					on:create
					on:delete
				/>
			{/if}
		</div>
	{/each}

	<!-- Add new page button at root level -->
	{#if depth === 0 && canEdit}
		<button
			type="button"
			class="flex items-center gap-1.5 w-full py-1.5 px-2 text-sm text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-200 hover:bg-gray-100 dark:hover:bg-gray-800 rounded-md transition-colors"
			on:click={() => handleCreate(null)}
		>
			<Icon icon="mdi:plus" class="h-3.5 w-3.5" />
			<span>Add page</span>
		</button>
	{/if}
</div>
