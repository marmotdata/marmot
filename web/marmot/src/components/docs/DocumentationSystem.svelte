<script lang="ts">
	import { onMount } from 'svelte';
	import Icon from '@iconify/svelte';
	import DocPageTree from './DocPageTree.svelte';
	import DocEditor from './DocEditor.svelte';
	import ConfirmModal from '$components/ui/ConfirmModal.svelte';
	import { fetchApi } from '$lib/api';
	import { auth } from '$lib/stores/auth';
	import type { Page, PageTree, EntityType } from '$lib/docs/types';
	import { marked } from 'marked';
	import { common, createLowlight } from 'lowlight';
	import { toHtml } from 'hast-util-to-html';

	// Create lowlight instance for syntax highlighting
	const lowlight = createLowlight(common);

	export let entityType: EntityType;
	export let entityId: string;

	// State
	let pageTree: PageTree | null = null;
	let selectedPage: Page | null = null;
	let isLoading = true;
	let loadError: string | null = null;
	let isSaving = false;
	let isEditing = false;
	let editedContent = '';
	let editedTitle = '';
	let editedEmoji: string | null = null;

	// Modal state
	let showDeleteModal = false;
	let pageToDelete: Page | null = null;
	let showEmojiPicker = false;

	// Sidebar collapse state
	let sidebarCollapsed = false;

	// Emoji picker - load dynamically on client side
	let emojiPickerElement: HTMLElement | null = null;

	function handleRemoveEmoji() {
		editedEmoji = null;
		showEmojiPicker = false;
	}

	// Attach emoji picker event listener when element is available
	$: if (emojiPickerElement) {
		const handler = (event: Event) => {
			const customEvent = event as CustomEvent;
			editedEmoji = customEvent.detail.unicode;
			showEmojiPicker = false;
		};
		emojiPickerElement.addEventListener('emoji-click', handler);
	}

	$: if (showEmojiPicker && typeof window !== 'undefined') {
		import('emoji-picker-element');
	}

	$: canEdit = auth.hasPermission('assets', 'manage');

	onMount(() => {
		loadPageTree();
	});

	// Find a page by ID recursively in the page tree
	function findPageById(pages: Page[], id: string): Page | null {
		for (const page of pages) {
			if (page.id === id) {
				return page;
			}
			if (page.children && page.children.length > 0) {
				const found = findPageById(page.children, id);
				if (found) return found;
			}
		}
		return null;
	}

	async function loadPageTree() {
		isLoading = true;
		loadError = null;

		try {
			const response = await fetchApi(
				`/docs/entity/${entityType}/${encodeURIComponent(entityId)}/pages`
			);
			if (!response.ok) {
				throw new Error('Failed to load documentation');
			}
			const tree: PageTree = await response.json();
			pageTree = tree;

			// Check for page query parameter
			const urlParams = new URLSearchParams(window.location.search);
			const pageId = urlParams.get('page');

			if (pageId && tree.pages.length > 0) {
				// Try to find and select the specified page
				const targetPage = findPageById(tree.pages, pageId);
				if (targetPage) {
					await selectPage(targetPage);
					return;
				}
			}

			// Auto-select first page if none selected and no page param
			if (!selectedPage && tree.pages.length > 0) {
				await selectPage(tree.pages[0]);
			}
		} catch (err) {
			loadError = err instanceof Error ? err.message : 'Failed to load documentation';
		} finally {
			isLoading = false;
		}
	}

	async function selectPage(page: Page) {
		// If editing current page, prompt to save
		if (isEditing && selectedPage) {
			const shouldSave = confirm('You have unsaved changes. Save before switching?');
			if (shouldSave) {
				await saveContent();
			}
		}

		// Load full page content
		try {
			const response = await fetchApi(`/docs/pages/${page.id}`);
			if (!response.ok) {
				throw new Error('Failed to load page');
			}
			const loadedPage: Page = await response.json();
			selectedPage = loadedPage;
			isEditing = false;
			editedContent = loadedPage.content || '';
			editedTitle = loadedPage.title;
			editedEmoji = loadedPage.emoji;

			// Update URL with page query parameter
			const url = new URL(window.location.href);
			url.searchParams.set('page', page.id);
			window.history.replaceState({}, '', url.toString());
		} catch (err) {
			console.error('Failed to load page:', err);
		}
	}

	async function createPage(parentId: string | null) {
		try {
			const response = await fetchApi(
				`/docs/entity/${entityType}/${encodeURIComponent(entityId)}/pages`,
				{
					method: 'POST',
					body: JSON.stringify({
						parent_id: parentId,
						title: 'Untitled',
						content: ''
					})
				}
			);

			if (!response.ok) {
				throw new Error('Failed to create page');
			}

			const newPage = await response.json();
			await loadPageTree();
			await selectPage(newPage);
			isEditing = true;
		} catch (err) {
			console.error('Failed to create page:', err);
		}
	}

	function startEditing() {
		editedContent = selectedPage?.content || '';
		editedTitle = selectedPage?.title || '';
		editedEmoji = selectedPage?.emoji || null;
		isEditing = true;
	}

	function cancelEditing() {
		editedContent = selectedPage?.content || '';
		editedTitle = selectedPage?.title || '';
		editedEmoji = selectedPage?.emoji || null;
		isEditing = false;
		showEmojiPicker = false;
	}

	async function saveContent() {
		if (!selectedPage) return;

		isSaving = true;
		try {
			const response = await fetchApi(`/docs/pages/${selectedPage.id}`, {
				method: 'PUT',
				body: JSON.stringify({
					title: editedTitle.trim() || 'Untitled',
					emoji: editedEmoji,
					content: editedContent
				})
			});

			if (!response.ok) {
				throw new Error('Failed to save page');
			}

			const savedPage: Page = await response.json();
			selectedPage = savedPage;
			isEditing = false;
			showEmojiPicker = false;
			await loadPageTree();
		} catch (err) {
			console.error('Failed to save page:', err);
		} finally {
			isSaving = false;
		}
	}

	function confirmDeletePage(page: Page) {
		pageToDelete = page;
		showDeleteModal = true;
	}

	async function deletePage() {
		if (!pageToDelete) return;

		try {
			const response = await fetchApi(`/docs/pages/${pageToDelete.id}`, {
				method: 'DELETE'
			});

			if (!response.ok) {
				throw new Error('Failed to delete page');
			}

			if (selectedPage?.id === pageToDelete.id) {
				selectedPage = null;
			}

			showDeleteModal = false;
			pageToDelete = null;
			await loadPageTree();

			// Select first page if available
			if (pageTree && pageTree.pages.length > 0) {
				await selectPage(pageTree.pages[0]);
			}
		} catch (err) {
			console.error('Failed to delete page:', err);
		}
	}

	function preprocessMentions(markdown: string): string {
		// Match markdown link format: [@Label](mention:type:id)
		return markdown.replace(
			/\[@([^\]]+)\]\(mention:(user|team):([^)]+)\)/g,
			(_match, label, mentionType, id) => {
				const mentionClass =
					mentionType === 'team' ? 'mention mention-team' : 'mention mention-user';

				// Make team mentions clickable links
				if (mentionType === 'team' && id) {
					return `<a href="/teams/${id}" class="${mentionClass}">@${label}</a>`;
				}

				return `<span class="${mentionClass}">@${label}</span>`;
			}
		);
	}

	function renderMarkdown(content: string): string {
		// Configure marked with syntax highlighting
		const renderer = new marked.Renderer();

		renderer.code = function ({ text, lang }: { text: string; lang?: string }) {
			const language = lang && lowlight.registered(lang) ? lang : 'plaintext';
			try {
				const highlighted = lowlight.highlight(language, text);
				const html = toHtml(highlighted);
				return `<pre><code class="hljs language-${language}">${html}</code></pre>`;
			} catch {
				// Fallback to non-highlighted
				return `<pre><code class="language-${language}">${text}</code></pre>`;
			}
		};

		return marked(preprocessMentions(content), { renderer }) as string;
	}
</script>

<div class="flex h-full">
	<!-- Sidebar: Page Tree (minimal style) -->
	<div
		class="flex-shrink-0 flex flex-col transition-all duration-200 {sidebarCollapsed
			? 'w-10'
			: 'w-56'}"
	>
		<!-- Collapse toggle -->
		<div class="flex items-center justify-between p-2">
			{#if !sidebarCollapsed}
				<span class="text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wide"
					>Pages</span
				>
			{/if}
			<button
				type="button"
				onclick={() => (sidebarCollapsed = !sidebarCollapsed)}
				class="p-1 rounded hover:bg-gray-100 dark:hover:bg-gray-700 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300"
				title={sidebarCollapsed ? 'Expand sidebar' : 'Collapse sidebar'}
			>
				<Icon icon={sidebarCollapsed ? 'mdi:chevron-right' : 'mdi:chevron-left'} class="h-4 w-4" />
			</button>
		</div>

		{#if !sidebarCollapsed}
			<div class="flex-1 overflow-y-auto px-1">
				{#if isLoading}
					<div class="flex items-center justify-center py-8">
						<Icon icon="mdi:loading" class="h-5 w-5 animate-spin text-gray-400" />
					</div>
				{:else if loadError}
					<div class="text-center py-6">
						<Icon icon="mdi:alert-circle-outline" class="h-6 w-6 text-red-400 mx-auto mb-2" />
						<p class="text-xs text-red-500">{loadError}</p>
						<button
							type="button"
							onclick={loadPageTree}
							class="text-xs text-earthy-terracotta-600 hover:underline mt-2"
						>
							Retry
						</button>
					</div>
				{:else if pageTree}
					<DocPageTree
						pages={pageTree.pages}
						selectedPageId={selectedPage?.id || null}
						{canEdit}
						on:select={(e) => selectPage(e.detail.page)}
						on:create={(e) => createPage(e.detail.parentId)}
						on:delete={(e) => confirmDeletePage(e.detail.page)}
					/>
				{/if}
			</div>
		{/if}
	</div>

	<!-- Main content area -->
	<div class="flex-1 flex flex-col overflow-y-auto min-w-0 max-w-5xl">
		{#if !selectedPage}
			<div class="flex-1 flex items-center justify-center text-gray-400 dark:text-gray-500">
				{#if pageTree && pageTree.pages.length === 0}
					<div class="text-center">
						<Icon icon="mdi:file-document-plus-outline" class="h-12 w-12 mx-auto mb-3 opacity-30" />
						<p class="text-sm mb-3">No documentation yet</p>
						{#if canEdit}
							<button
								type="button"
								onclick={() => createPage(null)}
								class="inline-flex items-center gap-1.5 px-3 py-1.5 text-sm text-earthy-terracotta-600 hover:text-earthy-terracotta-700 hover:bg-earthy-terracotta-50 dark:hover:bg-earthy-terracotta-900/20 rounded-lg transition-colors"
							>
								<Icon icon="mdi:plus" class="h-4 w-4" />
								Create first page
							</button>
						{/if}
					</div>
				{:else}
					<p class="text-sm">Select a page to view</p>
				{/if}
			</div>
		{:else}
			<!-- Page header with H1 title and emoji -->
			<div class="px-4 pt-6 pb-4">
				<div class="flex items-start justify-between gap-4">
					<div class="flex-1 min-w-0">
						{#if isEditing}
							<div class="flex items-center gap-2">
								<!-- Emoji picker button -->
								<div class="relative">
									<button
										type="button"
										onclick={() => (showEmojiPicker = !showEmojiPicker)}
										class="text-4xl hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg p-1 transition-colors"
										title="Change emoji"
									>
										{editedEmoji || '📄'}
									</button>
									{#if showEmojiPicker}
										<div
											class="absolute top-full left-0 mt-1 z-50 rounded-lg shadow-xl border border-gray-200 dark:border-gray-700"
										>
											<emoji-picker bind:this={emojiPickerElement}></emoji-picker>
											<div
												class="bg-white dark:bg-gray-800 border-t border-gray-200 dark:border-gray-700 px-3 py-2 flex justify-between rounded-b-lg"
											>
												<button
													type="button"
													onclick={handleRemoveEmoji}
													class="text-xs text-gray-500 hover:text-gray-700 dark:hover:text-gray-300"
												>
													Remove emoji
												</button>
												<button
													type="button"
													onclick={() => (showEmojiPicker = false)}
													class="text-xs text-gray-500 hover:text-gray-700 dark:hover:text-gray-300"
												>
													Close
												</button>
											</div>
										</div>
									{/if}
								</div>
								<input
									type="text"
									bind:value={editedTitle}
									class="text-3xl font-bold bg-transparent border-b-2 border-gray-300 dark:border-gray-600 focus:border-earthy-terracotta-500 focus:outline-none text-gray-900 dark:text-gray-100 w-full"
									placeholder="Page title"
								/>
							</div>
						{:else}
							<!-- H1 with emoji and title combined -->
							<h1
								class="text-3xl font-bold text-gray-900 dark:text-gray-100 flex items-center gap-3"
							>
								{#if selectedPage.emoji}
									<span>{selectedPage.emoji}</span>
								{/if}
								<span class="truncate">{selectedPage.title}</span>
							</h1>
						{/if}
					</div>

					<div class="flex items-center gap-1 flex-shrink-0 pt-2">
						{#if isEditing}
							<button
								type="button"
								onclick={cancelEditing}
								disabled={isSaving}
								class="px-3 py-1.5 text-sm text-gray-600 dark:text-gray-400 hover:text-gray-800 dark:hover:text-gray-200 disabled:opacity-50"
							>
								Cancel
							</button>
							<button
								type="button"
								onclick={saveContent}
								disabled={isSaving}
								class="px-3 py-1.5 text-sm text-white bg-earthy-terracotta-600 hover:bg-earthy-terracotta-700 rounded disabled:opacity-50"
							>
								{isSaving ? 'Saving...' : 'Save'}
							</button>
						{:else if canEdit}
							<button
								type="button"
								onclick={startEditing}
								class="p-1.5 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 rounded transition-colors"
								title="Edit"
							>
								<Icon icon="mdi:pencil-outline" class="h-4 w-4" />
							</button>
						{/if}
					</div>
				</div>
			</div>

			<!-- Page content -->
			<div class="px-4 pb-4">
				{#if isEditing}
					<DocEditor
						bind:value={editedContent}
						pageId={selectedPage.id}
						placeholder="Start writing..."
						disabled={isSaving}
					/>
				{:else if selectedPage.content}
					<div
						class="doc-content prose prose-sm prose-gray dark:prose-invert max-w-none prose-headings:text-gray-900 dark:prose-headings:text-gray-100 prose-p:text-gray-600 dark:prose-p:text-gray-300 prose-a:text-earthy-terracotta-600 dark:prose-a:text-earthy-terracotta-400 prose-img:rounded-lg"
					>
						{@html renderMarkdown(selectedPage.content)}
					</div>
				{:else}
					<div class="text-center py-8 text-gray-400 dark:text-gray-500">
						<Icon icon="mdi:file-document-outline" class="h-10 w-10 mx-auto mb-2 opacity-30" />
						<p class="text-sm">This page is empty</p>
						{#if canEdit}
							<button
								type="button"
								onclick={startEditing}
								class="text-sm text-earthy-terracotta-600 hover:underline mt-2"
							>
								Add content
							</button>
						{/if}
					</div>
				{/if}
			</div>

			<!-- Page metadata footer (minimal) -->
			<div class="px-4 py-2 text-xs text-gray-400 dark:text-gray-500 flex items-center gap-3">
				<span>Updated {new Date(selectedPage.updated_at).toLocaleDateString()}</span>
				{#if selectedPage.image_count && selectedPage.image_count > 0}
					<span class="flex items-center gap-1">
						<Icon icon="mdi:image-outline" class="h-3 w-3" />
						{selectedPage.image_count}
					</span>
				{/if}
			</div>
		{/if}
	</div>
</div>

<!-- Delete confirmation modal -->
<ConfirmModal
	bind:show={showDeleteModal}
	title="Delete Page"
	message={pageToDelete
		? `Are you sure you want to delete "${pageToDelete.title}"? This will also delete all sub-pages and images.`
		: ''}
	confirmText="Delete"
	cancelText="Cancel"
	variant="danger"
	onConfirm={deletePage}
	onCancel={() => {
		showDeleteModal = false;
		pageToDelete = null;
	}}
/>

<style>
	/* Emoji picker dark mode support */
	:global(.dark emoji-picker) {
		--background: #1f2937;
		--border-color: #374151;
		--indicator-color: #c9601c;
		--input-border-color: #4b5563;
		--input-font-color: #f3f4f6;
		--input-placeholder-color: #9ca3af;
		--outline-color: #c9601c;
		--category-font-color: #9ca3af;
		--button-active-background: #374151;
		--button-hover-background: #374151;
	}

	/* Code block styles - matching DocEditor */
	:global(.doc-content pre) {
		@apply bg-gray-50 dark:bg-gray-800 p-6 rounded-lg my-3 overflow-x-auto;
		margin: 0;
	}

	:global(.doc-content pre code) {
		@apply bg-transparent p-0;
		font-family:
			ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, 'Liberation Mono', 'Courier New',
			monospace;
		font-size: 0.875rem;
		display: block;
		white-space: pre;
		width: max-content;
		min-width: 100%;
		color: #1f2937;
	}

	:global(.dark .doc-content pre code) {
		color: #f3f4f6;
	}

	/* Inline code */
	:global(.doc-content code:not(pre code)) {
		@apply bg-gray-100 dark:bg-gray-800 px-1.5 py-0.5 rounded text-sm font-mono;
	}

	/* Syntax highlighting - Light theme (Earthy colors) */
	:global(.doc-content .hljs-comment),
	:global(.doc-content .hljs-quote) {
		color: #4a674a;
		font-style: italic;
	}

	:global(.doc-content .hljs-keyword),
	:global(.doc-content .hljs-selector-tag),
	:global(.doc-content .hljs-addition) {
		color: #8d3718;
	}

	:global(.doc-content .hljs-number),
	:global(.doc-content .hljs-string),
	:global(.doc-content .hljs-meta .hljs-meta-string),
	:global(.doc-content .hljs-literal),
	:global(.doc-content .hljs-doctag),
	:global(.doc-content .hljs-regexp) {
		color: #35593b;
	}

	:global(.doc-content .hljs-title),
	:global(.doc-content .hljs-section),
	:global(.doc-content .hljs-name),
	:global(.doc-content .hljs-selector-id),
	:global(.doc-content .hljs-selector-class) {
		color: #b34822;
	}

	:global(.doc-content .hljs-attribute),
	:global(.doc-content .hljs-attr),
	:global(.doc-content .hljs-variable),
	:global(.doc-content .hljs-template-variable),
	:global(.doc-content .hljs-class .hljs-title),
	:global(.doc-content .hljs-type) {
		color: #7b5935;
	}

	:global(.doc-content .hljs-symbol),
	:global(.doc-content .hljs-bullet),
	:global(.doc-content .hljs-subst),
	:global(.doc-content .hljs-meta),
	:global(.doc-content .hljs-meta .hljs-keyword),
	:global(.doc-content .hljs-selector-attr),
	:global(.doc-content .hljs-selector-pseudo),
	:global(.doc-content .hljs-link) {
		color: #7b5935;
	}

	:global(.doc-content .hljs-built_in),
	:global(.doc-content .hljs-deletion) {
		color: #b34822;
	}

	:global(.doc-content .hljs-punctuation),
	:global(.doc-content .hljs-operator) {
		color: #4a674a;
	}

	:global(.doc-content .hljs-emphasis) {
		font-style: italic;
	}

	:global(.doc-content .hljs-strong) {
		font-weight: bold;
	}

	/* Syntax highlighting - Dark theme (Brighter earthy tones) */
	:global(.dark .doc-content .hljs-comment),
	:global(.dark .doc-content .hljs-quote) {
		color: #a8c5a8;
		font-style: italic;
	}

	:global(.dark .doc-content .hljs-keyword),
	:global(.dark .doc-content .hljs-selector-tag),
	:global(.dark .doc-content .hljs-addition) {
		color: #ffa77d;
	}

	:global(.dark .doc-content .hljs-number),
	:global(.dark .doc-content .hljs-string),
	:global(.dark .doc-content .hljs-meta .hljs-meta-string),
	:global(.dark .doc-content .hljs-literal),
	:global(.dark .doc-content .hljs-doctag),
	:global(.dark .doc-content .hljs-regexp) {
		color: #b9d9b9;
	}

	:global(.dark .doc-content .hljs-title),
	:global(.dark .doc-content .hljs-section),
	:global(.dark .doc-content .hljs-name),
	:global(.dark .doc-content .hljs-selector-id),
	:global(.dark .doc-content .hljs-selector-class) {
		color: #ffb899;
	}

	:global(.dark .doc-content .hljs-attribute),
	:global(.dark .doc-content .hljs-attr),
	:global(.dark .doc-content .hljs-variable),
	:global(.dark .doc-content .hljs-template-variable),
	:global(.dark .doc-content .hljs-class .hljs-title),
	:global(.dark .doc-content .hljs-type) {
		color: #f0d97e;
	}

	:global(.dark .doc-content .hljs-symbol),
	:global(.dark .doc-content .hljs-bullet),
	:global(.dark .doc-content .hljs-subst),
	:global(.dark .doc-content .hljs-meta),
	:global(.dark .doc-content .hljs-meta .hljs-keyword),
	:global(.dark .doc-content .hljs-selector-attr),
	:global(.dark .doc-content .hljs-selector-pseudo),
	:global(.dark .doc-content .hljs-link) {
		color: #f0d97e;
	}

	:global(.dark .doc-content .hljs-built_in),
	:global(.dark .doc-content .hljs-deletion) {
		color: #ffb899;
	}

	:global(.dark .doc-content .hljs-punctuation),
	:global(.dark .doc-content .hljs-operator) {
		color: #d1e5d1;
	}

	/* Table styles - matching editor */
	:global(.doc-content table) {
		@apply w-full my-4 text-sm border border-gray-300 dark:border-gray-600 rounded-lg overflow-hidden;
		border-collapse: collapse;
	}

	:global(.doc-content th) {
		@apply bg-gray-100 dark:bg-gray-800 font-semibold text-left px-3 py-2.5 text-gray-900 dark:text-gray-100;
		border: 1px solid;
		@apply border-gray-300 dark:border-gray-600;
		border-right-width: 2px;
		@apply border-r-gray-400 dark:border-r-gray-500;
	}

	:global(.doc-content th:last-child) {
		border-right-width: 1px;
		@apply border-r-gray-300 dark:border-r-gray-600;
	}

	:global(.doc-content td) {
		@apply px-3 py-2.5 text-gray-700 dark:text-gray-300;
		border: 1px solid;
		@apply border-gray-300 dark:border-gray-600;
		border-right-width: 2px;
		@apply border-r-gray-400 dark:border-r-gray-500;
	}

	:global(.doc-content td:last-child) {
		border-right-width: 1px;
		@apply border-r-gray-300 dark:border-r-gray-600;
	}

	:global(.doc-content tr:nth-child(even) td) {
		@apply bg-gray-50 dark:bg-gray-800/30;
	}

	:global(.doc-content tr:hover td) {
		@apply bg-blue-50 dark:bg-blue-900/20;
	}

	/* Blockquote */
	:global(.doc-content blockquote) {
		@apply border-l-4 border-gray-300 dark:border-gray-600 pl-4 italic my-2;
	}

	/* Lists */
	:global(.doc-content ul) {
		@apply list-disc list-inside my-2;
	}

	:global(.doc-content ol) {
		@apply list-decimal list-inside my-2;
	}

	/* Images */
	:global(.doc-content img) {
		@apply max-w-full h-auto rounded-lg my-4;
	}

	/* Mentions - shared styles */
	:global(.doc-content .mention) {
		@apply px-1.5 py-0.5 rounded font-medium no-underline;
	}

	/* User mentions - terracotta/orange */
	:global(.doc-content .mention-user) {
		@apply bg-earthy-terracotta-100 dark:bg-earthy-terracotta-900/30 text-earthy-terracotta-700 dark:text-earthy-terracotta-400;
	}

	/* Team mentions - blue */
	:global(.doc-content .mention-team) {
		@apply bg-blue-100 dark:bg-blue-900/30 text-blue-700 dark:text-blue-400;
	}

	/* Clickable mention links */
	:global(.doc-content a.mention) {
		@apply cursor-pointer transition-colors;
	}
	:global(.doc-content a.mention:hover) {
		@apply bg-blue-200 dark:bg-blue-800/50;
	}
</style>
