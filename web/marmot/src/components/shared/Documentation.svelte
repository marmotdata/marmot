<script lang="ts">
	import { fetchApi } from '$lib/api';
	import { auth } from '$lib/stores/auth';
	import { marked } from 'marked';
	import { sanitizeHtml } from '$lib/sanitize';
	import IconifyIcon from '@iconify/svelte';
	import Button from '$components/ui/Button.svelte';
	import RichTextEditor from '$components/editor/RichTextEditor.svelte';
	import { m } from '$lib/paraglide/messages';
	import { formatDate } from '$lib/utils';

	marked.setOptions({
		gfm: true,
		breaks: true
	});

	let {
		// For single documentation field (data products)
		content = $bindable(''),
		// For fetching documentation from API (assets)
		mrn = undefined,
		// API endpoint for saving (e.g., "/products")
		endpoint = undefined,
		// Entity ID for saving
		id = undefined,
		// Permission checking
		permissionResource = 'assets',
		permissionAction = 'manage',
		// Read-only override
		readOnly = false
	}: {
		content?: string;
		mrn?: string;
		endpoint?: string;
		id?: string;
		permissionResource?: string;
		permissionAction?: string;
		readOnly?: boolean;
	} = $props();

	// State
	let isEditing = $state(false);
	let editedContent = $state('');
	let isSaving = $state(false);
	let isLoading = $state(false);
	let loadError = $state<string | null>(null);

	// For assets: documentation from multiple sources
	let documentationSources = $state<Array<{ source: string; content: string; updated_at: string }>>(
		[]
	);

	// Determine mode
	const isAssetMode = $derived(!!mrn);
	const canEdit = $derived(
		!readOnly && auth.hasPermission(permissionResource, permissionAction) && !isAssetMode
	);

	// Load documentation for assets
	$effect(() => {
		if (mrn) {
			loadAssetDocumentation();
		}
	});

	async function loadAssetDocumentation() {
		if (!mrn) return;

		isLoading = true;
		loadError = null;

		try {
			const encodedMrn = encodeURIComponent(mrn);
			const response = await fetchApi(`/assets/documentation/${encodedMrn}`);
			if (response.ok) {
				const data = await response.json();
				documentationSources = data || [];
			}
		} catch (err) {
			loadError = err instanceof Error ? err.message : m.shared_docs_load_error();
		} finally {
			isLoading = false;
		}
	}

	function startEditing() {
		editedContent = content || '';
		isEditing = true;
	}

	function cancelEditing() {
		isEditing = false;
		editedContent = '';
	}

	async function saveDocumentation() {
		if (!endpoint || !id) return;

		isSaving = true;
		try {
			const response = await fetchApi(`${endpoint}/${id}`, {
				method: 'PUT',
				body: JSON.stringify({
					documentation: editedContent.trim() || null
				})
			});

			if (response.ok) {
				content = editedContent.trim();
				isEditing = false;
			}
		} catch (err) {
			console.error('Failed to save documentation:', err);
		} finally {
			isSaving = false;
		}
	}

	function renderMarkdown(text: string): string {
		return sanitizeHtml(marked(text) as string);
	}
</script>

{#if isAssetMode}
	<!-- Asset Documentation Mode: Multiple sources, read-only -->
	<div class="space-y-6">
		{#if isLoading}
			<div class="p-4 bg-gray-50 dark:bg-gray-800 rounded-lg">
				<p class="text-gray-500 dark:text-gray-400">{m.shared_docs_loading()}</p>
			</div>
		{:else if loadError}
			<div class="p-4 bg-red-50 dark:bg-red-900/20 rounded-lg">
				<p class="text-red-500 dark:text-red-400">{m.shared_docs_load_error()}</p>
			</div>
		{:else if documentationSources.length > 0}
			{#each documentationSources as doc, i (i)}
				<div
					class="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-6"
				>
					<div class="mb-4 flex justify-between items-center">
						<span class="text-sm text-gray-500 dark:text-gray-400"
							>{m.shared_docs_source_label({ source: doc.source })}</span
						>
						<span class="text-sm text-gray-500 dark:text-gray-400">
							{m.shared_docs_updated_label({ date: formatDate(doc.updated_at) })}
						</span>
					</div>
					<div
						class="prose prose-gray dark:prose-invert max-w-none prose-headings:text-gray-900 dark:prose-headings:text-gray-100 prose-p:text-gray-600 dark:prose-p:text-gray-300 prose-a:text-earthy-terracotta-600 dark:prose-a:text-earthy-terracotta-400"
					>
						<!-- eslint-disable-next-line svelte/no-at-html-tags -- sanitized via DOMPurify in renderMarkdown() -->
						{@html renderMarkdown(doc.content)}
					</div>
				</div>
			{/each}
		{:else}
			<div class="p-6 bg-gray-50 dark:bg-gray-800 rounded-lg text-center">
				<IconifyIcon
					icon="material-symbols:description-outline"
					class="w-12 h-12 text-gray-300 dark:text-gray-600 mx-auto mb-3"
				/>
				<p class="text-gray-500 dark:text-gray-400 italic">{m.shared_docs_empty()}</p>
			</div>
		{/if}
	</div>
{:else}
	<!-- Single Documentation Mode: Editable -->
	<div>
		{#if isEditing}
			<div class="space-y-4">
				<RichTextEditor bind:value={editedContent} placeholder={m.shared_docs_add_placeholder()} />
				<div class="flex justify-end gap-2">
					<Button
						variant="clear"
						click={cancelEditing}
						text={m.common_cancel()}
						disabled={isSaving}
					/>
					<Button
						variant="filled"
						click={saveDocumentation}
						text={isSaving ? m.shared_saving() : m.common_save()}
						disabled={isSaving}
					/>
				</div>
			</div>
		{:else}
			<div class="flex items-center justify-between mb-4">
				<h3 class="text-base font-semibold text-gray-900 dark:text-gray-100">
					{m.common_documentation()}
				</h3>
				{#if canEdit}
					<Button
						variant="clear"
						click={startEditing}
						icon="material-symbols:edit"
						text={m.common_edit()}
					/>
				{/if}
			</div>
			{#if content && content.trim()}
				<div
					class="prose prose-gray dark:prose-invert max-w-none prose-headings:text-gray-900 dark:prose-headings:text-gray-100 prose-p:text-gray-600 dark:prose-p:text-gray-300 prose-a:text-earthy-terracotta-600 dark:prose-a:text-earthy-terracotta-400"
				>
					<!-- eslint-disable-next-line svelte/no-at-html-tags -- sanitized via DOMPurify in renderMarkdown() -->
					{@html renderMarkdown(content)}
				</div>
			{:else}
				<div class="p-6 bg-gray-50 dark:bg-gray-800 rounded-lg text-center">
					<IconifyIcon
						icon="material-symbols:description-outline"
						class="w-12 h-12 text-gray-300 dark:text-gray-600 mx-auto mb-3"
					/>
					<p class="text-gray-500 dark:text-gray-400 italic">
						{m.shared_docs_none_yet()}{#if canEdit}
							{m.shared_docs_click_edit_hint()}{/if}
					</p>
				</div>
			{/if}
		{/if}
	</div>
{/if}
