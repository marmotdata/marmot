<script lang="ts">
	import { onMount } from 'svelte';
	import DocEditor from './DocEditor.svelte';
	import { fetchApi } from '$lib/api';
	import { auth } from '$lib/stores/auth';
	import { m } from '$lib/paraglide/messages';
	import { formatDate } from '$lib/utils';
	import type { Page, PageTree, EntityType } from '$lib/docs/types';
	import { marked } from 'marked';
	import { common, createLowlight } from 'lowlight';
	import { toHtml } from 'hast-util-to-html';
	import { sanitizeHtml } from '$lib/sanitize';
	import { deferDocImages, docImages } from '$lib/docs/images';
	import '@marmotdata/elements/tree.js';
	import type {
		MmTreeActionDetail,
		MmTreeSelectDetail,
		TreeNode
	} from '@marmotdata/elements/classes/tree.js';
	import type { MmConfirmDialog } from '@marmotdata/elements/classes/confirm-dialog.js';

	const lowlight = createLowlight(common);

	// Documentation pages for an asset or a data product: a page tree beside the selected page, edited in place.
	let { entityType, entityId }: { entityType: EntityType; entityId: string } = $props();

	let pageTree = $state<PageTree | null>(null);
	let selectedPage = $state<Page | null>(null);
	let isLoading = $state(true);
	let loadError = $state<string | null>(null);
	let isSaving = $state(false);
	let isEditing = $state(false);
	let editedContent = $state('');
	let editedTitle = $state('');
	let editedEmoji = $state<string | null>(null);

	let pageToDelete = $state<Page | null>(null);
	let deleteDialog = $state<MmConfirmDialog>();
	let showEmojiPicker = $state(false);
	let emojiPickerElement = $state<HTMLElement | null>(null);
	let expandedIds = $state<string[]>([]);

	const canEdit = auth.hasPermission('assets', 'manage');

	$effect(() => {
		const el = emojiPickerElement;
		if (!el) return;
		const handler = (event: Event) => {
			editedEmoji = (event as CustomEvent).detail.unicode;
			showEmojiPicker = false;
		};
		el.addEventListener('emoji-click', handler);
		return () => el.removeEventListener('emoji-click', handler);
	});

	$effect(() => {
		if (showEmojiPicker) import('emoji-picker-element');
	});

	onMount(() => {
		loadPageTree();
	});

	function findPageById(pages: Page[], id: string): Page | null {
		for (const p of pages) {
			if (p.id === id) return p;
			if (p.children?.length) {
				const found = findPageById(p.children, id);
				if (found) return found;
			}
		}
		return null;
	}

	function parentIds(pages: Page[]): string[] {
		return pages.flatMap((p) => (p.children?.length ? [p.id, ...parentIds(p.children)] : []));
	}

	function toNodes(pages: Page[]): TreeNode[] {
		return pages.map((p) => ({
			id: p.id,
			label: p.emoji ? `${p.emoji} ${p.title}` : p.title,
			icon: 'article',
			data: p,
			children: p.children?.length ? toNodes(p.children) : undefined
		}));
	}

	let nodes = $derived(pageTree ? toNodes(pageTree.pages) : []);
	let treeActions = $derived(
		canEdit
			? [
					{ id: 'add', label: m.docs_add_subpage_title(), icon: 'note-add' },
					{ id: 'delete', label: m.asset_page_docs_delete_page(), icon: 'delete' }
				]
			: []
	);

	async function loadPageTree() {
		isLoading = true;
		loadError = null;
		try {
			const response = await fetchApi(
				`/docs/entity/${entityType}/${encodeURIComponent(entityId)}/pages`
			);
			if (!response.ok) throw new Error(m.docs_load_error());
			const tree: PageTree = await response.json();
			pageTree = tree;
			expandedIds = [...new Set([...expandedIds, ...parentIds(tree.pages)])];

			const pageId = new URLSearchParams(window.location.search).get('page');
			if (pageId && tree.pages.length > 0) {
				const target = findPageById(tree.pages, pageId);
				if (target) {
					await selectPage(target);
					return;
				}
			}
			if (!selectedPage && tree.pages.length > 0) {
				await selectPage(tree.pages[0]);
			}
		} catch (err) {
			loadError = err instanceof Error ? err.message : m.docs_load_error();
		} finally {
			isLoading = false;
		}
	}

	async function selectPage(target: Page) {
		if (isEditing && selectedPage && selectedPage.id !== target.id) {
			if (confirm(m.docs_unsaved_changes_confirm())) await saveContent();
		}
		try {
			const response = await fetchApi(`/docs/pages/${target.id}`);
			if (!response.ok) throw new Error('Failed to load page');
			const loaded: Page = await response.json();
			selectedPage = loaded;
			isEditing = false;
			editedContent = loaded.content || '';
			editedTitle = loaded.title;
			editedEmoji = loaded.emoji;
			const url = new URL(window.location.href);
			url.searchParams.set('page', target.id);
			window.history.replaceState(history.state, '', url.toString());
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
					body: JSON.stringify({ parent_id: parentId, title: m.docs_untitled_page(), content: '' })
				}
			);
			if (!response.ok) throw new Error('Failed to create page');
			const newPage = await response.json();
			if (parentId) expandedIds = [...new Set([...expandedIds, parentId])];
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
		startEditing();
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
					title: editedTitle.trim() || m.docs_untitled_page(),
					emoji: editedEmoji,
					content: editedContent
				})
			});
			if (!response.ok) throw new Error('Failed to save page');
			selectedPage = await response.json();
			isEditing = false;
			showEmojiPicker = false;
			await loadPageTree();
		} catch (err) {
			console.error('Failed to save page:', err);
		} finally {
			isSaving = false;
		}
	}

	async function deletePage() {
		const target = pageToDelete;
		if (!target) return;
		try {
			const response = await fetchApi(`/docs/pages/${target.id}`, { method: 'DELETE' });
			if (!response.ok) throw new Error('Failed to delete page');
			if (selectedPage?.id === target.id) selectedPage = null;
			pageToDelete = null;
			await loadPageTree();
			if (!selectedPage && pageTree && pageTree.pages.length > 0) {
				await selectPage(pageTree.pages[0]);
			}
		} catch (err) {
			console.error('Failed to delete page:', err);
		}
	}

	function onTreeSelect(e: CustomEvent<MmTreeSelectDetail>) {
		const p = e.detail.node.data as Page | undefined;
		if (p && p.id !== selectedPage?.id) selectPage(p);
	}

	function onTreeAction(e: CustomEvent<MmTreeActionDetail>) {
		const p = e.detail.node.data as Page | undefined;
		if (!p) return;
		if (e.detail.action === 'add') createPage(p.id);
		if (e.detail.action === 'delete') {
			pageToDelete = p;
			deleteDialog?.show();
		}
	}

	function preprocessMentions(markdown: string): string {
		// Mentions are stored as markdown links: [@Label](mention:type:id)
		return markdown.replace(
			/\[@([^\]]+)\]\(mention:(user|team):([^)]+)\)/g,
			(_match, label, mentionType, id) => {
				const cls = mentionType === 'team' ? 'mention mention-team' : 'mention mention-user';
				if (mentionType === 'team' && id) {
					return `<a href="/settings/teams/${id}" class="${cls}">@${label}</a>`;
				}
				return `<span class="${cls}">@${label}</span>`;
			}
		);
	}

	function renderMarkdown(content: string): string {
		const renderer = new marked.Renderer();
		renderer.code = function ({ text, lang }: { text: string; lang?: string }) {
			const language = lang && lowlight.registered(lang) ? lang : 'plaintext';
			try {
				const highlighted = toHtml(lowlight.highlight(language, text));
				return `<pre><code class="hljs language-${language}">${highlighted}</code></pre>`;
			} catch {
				return `<pre><code class="language-${language}">${text}</code></pre>`;
			}
		};
		return deferDocImages(
			sanitizeHtml(marked(preprocessMentions(content), { renderer }) as string)
		);
	}
</script>

{#if isLoading && !pageTree}
	<mm-skeleton count={6}></mm-skeleton>
{:else if loadError}
	<mm-callout tone="danger" heading={loadError}>
		<mm-button slot="actions" size="sm" variant="secondary" icon="refresh" onclick={loadPageTree}
			>{m.common_retry()}</mm-button
		>
	</mm-callout>
{:else if pageTree && pageTree.pages.length === 0}
	<mm-empty-state
		heading={m.docs_no_documentation_yet()}
		description={canEdit ? m.asset_page_docs_empty_body_manage() : m.asset_page_docs_empty_body()}
		seed={4}
	>
		{#if canEdit}
			<mm-button slot="action" variant="secondary" icon="note-add" onclick={() => createPage(null)}
				>{m.asset_page_docs_write_first()}</mm-button
			>
		{/if}
	</mm-empty-state>
{:else}
	<div class="docs">
		<div class="mm-stack" style="--mm-gap: var(--space-2); align-items: flex-start">
			<mm-tree
				label={m.asset_page_docs_tree_label()}
				selection="single"
				style="--mm-tree-height: 60vh; inline-size: 100%"
				{nodes}
				{expandedIds}
				selectedIds={selectedPage ? [selectedPage.id] : []}
				actions={treeActions}
				onmm-select={onTreeSelect}
				onmm-action={onTreeAction}
				onmm-expand={(e: CustomEvent<{ node: TreeNode; expanded: boolean }>) =>
					(expandedIds = e.detail.expanded
						? [...expandedIds, e.detail.node.id]
						: expandedIds.filter((id) => id !== e.detail.node.id))}
			></mm-tree>
			{#if canEdit}
				<mm-button variant="ghost" size="sm" icon="note-add" onclick={() => createPage(null)}
					>{m.docs_add_page()}</mm-button
				>
			{/if}
		</div>

		<article class="mm-stack" style="--mm-gap: var(--space-4); min-width: 0">
			{#if !selectedPage}
				<mm-text variant="dense" tone="secondary">{m.docs_select_page_hint()}</mm-text>
			{:else if isEditing}
				<div class="mm-cluster" style="--mm-gap: var(--space-2); align-items: flex-end">
					<div class="emoji">
						<mm-button
							variant="ghost"
							icon={editedEmoji ? undefined : 'add'}
							label={editedEmoji ? m.docs_change_emoji_title() : undefined}
							expanded={showEmojiPicker}
							onclick={() => (showEmojiPicker = !showEmojiPicker)}
							>{editedEmoji || m.asset_page_docs_add_emoji()}</mm-button
						>
						{#if showEmojiPicker}
							<div class="emoji-panel">
								<emoji-picker bind:this={emojiPickerElement}></emoji-picker>
								<div class="mm-cluster mm-cluster--between" style="padding: var(--space-2)">
									<mm-button
										variant="ghost"
										size="sm"
										onclick={() => {
											editedEmoji = null;
											showEmojiPicker = false;
										}}>{m.docs_remove_emoji()}</mm-button
									>
									<mm-button variant="ghost" size="sm" onclick={() => (showEmojiPicker = false)}
										>{m.common_close()}</mm-button
									>
								</div>
							</div>
						{/if}
					</div>
					<mm-text-field
						label={m.docs_page_title_placeholder()}
						hide-label
						value={editedTitle}
						placeholder={m.docs_page_title_placeholder()}
						style="flex: 1; min-inline-size: 12rem"
						onmm-input={(e: CustomEvent<{ value: string }>) => (editedTitle = e.detail.value)}
					></mm-text-field>
					<mm-button variant="ghost" disabled={isSaving} onclick={cancelEditing}
						>{m.common_cancel()}</mm-button
					>
					<mm-button variant="primary" loading={isSaving} onclick={saveContent}
						>{m.asset_page_docs_save_page()}</mm-button
					>
				</div>
				<DocEditor
					bind:value={editedContent}
					pageId={selectedPage.id}
					placeholder={m.docs_start_writing_placeholder()}
					disabled={isSaving}
				/>
			{:else}
				<div class="mm-cluster mm-cluster--between">
					<mm-text variant="caption" tone="secondary"
						>{m.docs_updated_at({ date: formatDate(selectedPage.updated_at) })}</mm-text
					>
					{#if canEdit}
						<mm-button variant="secondary" size="sm" icon="edit" onclick={startEditing}
							>{m.asset_page_docs_edit_page()}</mm-button
						>
					{/if}
				</div>
				<div class="mm-prose doc-content" use:docImages>
					<h1>
						{#if selectedPage.emoji}<span aria-hidden="true">{selectedPage.emoji}</span>
						{/if}{selectedPage.title}
					</h1>
					{#if selectedPage.content}
						<!-- eslint-disable-next-line svelte/no-at-html-tags -- sanitized via DOMPurify in renderMarkdown() -->
						{@html renderMarkdown(selectedPage.content)}
					{:else}
						<p class="empty">{m.docs_page_empty()}</p>
					{/if}
				</div>
			{/if}
		</article>
	</div>
{/if}

<mm-confirm-dialog
	bind:this={deleteDialog}
	heading={m.asset_page_docs_delete_heading({ title: pageToDelete?.title ?? '' })}
	body={m.asset_page_docs_delete_body()}
	confirm-label={m.asset_page_docs_delete_page()}
	onmm-confirm={deletePage}
	onmm-close={() => (pageToDelete = null)}
></mm-confirm-dialog>

<style>
	.docs {
		display: grid;
		grid-template-columns: 13rem minmax(0, 1fr);
		gap: var(--space-6);
		align-items: start;
	}
	@media (max-width: 720px) {
		.docs {
			grid-template-columns: minmax(0, 1fr);
		}
	}
	.emoji {
		position: relative;
	}
	.emoji-panel {
		position: absolute;
		inset-block-start: calc(100% + var(--space-1));
		inset-inline-start: 0;
		z-index: var(--mm-z-popover, 50);
		background: var(--mm-raised);
		border: 1px solid var(--mm-rule);
		border-radius: var(--radius-md);
		box-shadow: var(--shadow-float);
		overflow: hidden;
	}
	.emoji-panel :global(emoji-picker) {
		--background: var(--mm-raised);
		--border-color: var(--mm-rule);
		--indicator-color: var(--mm-text);
		--input-border-color: var(--mm-control-border);
		--input-font-color: var(--mm-text);
		--input-placeholder-color: var(--mm-text-2);
		--outline-color: var(--mm-text);
		--category-font-color: var(--mm-text-2);
		--button-active-background: var(--mm-selected);
		--button-hover-background: var(--mm-selected);
	}
	.doc-content .empty {
		color: var(--mm-text-2);
	}
	.doc-content :global(code:not(pre code)) {
		background: var(--mm-sunken);
		font: var(--text-mono);
		padding: 0 var(--space-1);
		border-radius: var(--radius-sm);
	}
	.doc-content :global(.hljs-comment),
	.doc-content :global(.hljs-quote) {
		color: var(--mm-syn-comment);
		font-style: italic;
	}
	.doc-content :global(.hljs-keyword),
	.doc-content :global(.hljs-selector-tag),
	.doc-content :global(.hljs-built_in),
	.doc-content :global(.hljs-meta) {
		color: var(--mm-syn-keyword);
	}
	.doc-content :global(.hljs-string),
	.doc-content :global(.hljs-regexp),
	.doc-content :global(.hljs-addition) {
		color: var(--mm-syn-string);
	}
	.doc-content :global(.hljs-number),
	.doc-content :global(.hljs-literal),
	.doc-content :global(.hljs-symbol) {
		color: var(--mm-syn-number);
	}
	.doc-content :global(.hljs-title),
	.doc-content :global(.hljs-section),
	.doc-content :global(.hljs-name),
	.doc-content :global(.hljs-attr),
	.doc-content :global(.hljs-attribute),
	.doc-content :global(.hljs-type) {
		color: var(--mm-syn-function);
	}
	.doc-content :global(.mention) {
		background: var(--mm-info-soft);
		color: var(--mm-text);
		padding: 0 var(--space-1);
		border-radius: var(--radius-sm);
		text-decoration: none;
	}
	.doc-content :global(a.mention:hover) {
		text-decoration: underline;
	}
</style>
