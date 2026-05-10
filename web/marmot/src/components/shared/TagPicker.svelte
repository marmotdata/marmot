<script lang="ts">
	import type { Tag } from '$lib/tags/types';
	import { toasts, getErrorMessage } from '$lib/stores/toast';
	import { createTag, listTags } from '$lib/tags/api';
	import { SvelteSet } from 'svelte/reactivity';
	import TagBadge from '$components/shared/TagBadge.svelte';

	const POP_W = 288;
	const POP_MAX_H = 360;

	interface Props {
		anchorRect: DOMRect | null;
		title?: string;
		assignedTagIds: string[];
		onSave: (tagIds: string[]) => Promise<void>;
		onClose: () => void;
		onTagCreated?: (tag: Tag) => void;
	}

	let { anchorRect, title, assignedTagIds, onSave, onClose, onTagCreated }: Props = $props();

	let tags = $state<Tag[]>([]);
	let loading = $state(false);

	let popoverEl = $state<HTMLDivElement | null>(null);
	let pos = $state({ top: 0, left: 0, ready: false });

	$effect(() => {
		if (!anchorRect) {
			pos = { top: 0, left: 0, ready: false };
			return;
		}
		if (!popoverEl) return;
		const ph = popoverEl.offsetHeight;
		const pw = popoverEl.offsetWidth || POP_W;
		const vw = window.innerWidth;
		const vh = window.innerHeight;
		let top = anchorRect.bottom + 6;
		if (top + ph > vh - 8) top = Math.max(8, anchorRect.top - ph - 6);
		let left = anchorRect.left;
		if (left + pw > vw - 8) left = vw - pw - 8;
		if (left < 8) left = 8;
		pos = { top, left, ready: true };
	});

	let query = $state('');
	let isSaving = $state(false);

	// Local mutable selection state — initialized from assignedTagIds prop
	let selectedIds = new SvelteSet<string>(assignedTagIds);

	// Re-sync when assignedTagIds prop changes (e.g. after external save)
	$effect(() => {
		selectedIds = new SvelteSet<string>(assignedTagIds);
	});

	$effect(() => {
		if (!anchorRect) {
			query = '';
		}
	});

	$effect(() => {
		if (!anchorRect) return;
		function onDocClick(e: MouseEvent) {
			if (popoverEl && !popoverEl.contains(e.target as Node)) onClose();
		}
		function onKey(e: KeyboardEvent) {
			if (e.key === 'Escape') onClose();
		}
		document.addEventListener('mousedown', onDocClick);
		document.addEventListener('keydown', onKey);
		return () => {
			document.removeEventListener('mousedown', onDocClick);
			document.removeEventListener('keydown', onKey);
		};
	});

	// Lazy-load tags when picker opens
	$effect(() => {
		if (!anchorRect) return;
		let cancelled = false;
		loading = true;
		listTags()
			.then((t) => {
				if (!cancelled) tags = t;
			})
			.catch(() => {})
			.finally(() => {
				if (!cancelled) loading = false;
			});
		return () => {
			cancelled = true;
		};
	});

	// Inline tag creation state
	let showCreateForm = $state(false);
	let newTagName = $state('');
	let newTagDesc = $state('');
	let isCreating = $state(false);

	$effect(() => {
		if (!anchorRect) {
			showCreateForm = false;
			newTagName = '';
			newTagDesc = '';
		}
	});

	async function handleCreateTag() {
		if (isCreating || !newTagName.trim()) return;
		isCreating = true;
		try {
			const tag = await createTag({
				name: newTagName.trim(),
				description: newTagDesc.trim()
			});
			tags = [...tags, tag];
			selectedIds.add(tag.id);
			onTagCreated?.(tag);
			showCreateForm = false;
			newTagName = '';
			newTagDesc = '';
		} catch (e) {
			toasts.error(getErrorMessage(e));
		} finally {
			isCreating = false;
		}
	}

	const filtered = $derived(
		tags.filter(
			(t) =>
				t.name.toLowerCase().includes(query.trim().toLowerCase()) ||
				(t.description ?? '').toLowerCase().includes(query.trim().toLowerCase())
		)
	);

	function toggle(tagId: string) {
		if (isSaving) return;
		if (selectedIds.has(tagId)) {
			selectedIds.delete(tagId);
		} else {
			selectedIds.add(tagId);
		}
	}

	async function save() {
		if (isSaving) return;
		isSaving = true;
		try {
			await onSave([...selectedIds]);
			onClose();
		} catch (e) {
			toasts.error(getErrorMessage(e));
		} finally {
			isSaving = false;
		}
	}

	function portal(node: HTMLElement) {
		document.body.appendChild(node);
		return {
			destroy() {
				node.remove();
			}
		};
	}
</script>

{#if anchorRect}
	<div use:portal>
		<div
			bind:this={popoverEl}
			role="dialog"
			aria-modal="true"
			aria-label="Tag editor{title ? ` for ${title}` : ''}"
			class="flex flex-col bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-600 rounded-xl shadow-xl text-xs text-gray-900 dark:text-gray-100 overflow-hidden"
			style="
				position: fixed;
				top: {pos.top}px;
				left: {pos.left}px;
				width: {POP_W}px;
				max-height: {POP_MAX_H}px;
				opacity: {pos.ready ? 1 : 0};
				transform: translateY({pos.ready ? '0' : '-4px'});
				transition: opacity .12s ease-out, transform .14s ease-out;
				z-index: 1000;
			"
		>
			<!-- Search -->
			{#if !showCreateForm}
				<div class="p-2.5 border-b border-gray-100 dark:border-gray-700 flex-shrink-0">
					<div
						class="flex items-center gap-2 px-2.5 py-1.5 border border-gray-200 dark:border-gray-600 rounded-md bg-gray-50 dark:bg-gray-700"
					>
						<svg
							width="12"
							height="12"
							viewBox="0 0 24 24"
							fill="none"
							stroke="currentColor"
							stroke-width="1.5"
							stroke-linecap="round"
							stroke-linejoin="round"
							class="text-gray-400 flex-shrink-0"
							aria-hidden="true"
						>
							<circle cx="11" cy="11" r="7" /><path d="m20 20-3.5-3.5" />
						</svg>
						<input
							bind:value={query}
							placeholder="Search tags…"
							autofocus
							class="flex-1 bg-transparent text-xs text-gray-900 dark:text-gray-100 placeholder-gray-400 dark:placeholder-gray-500 focus:outline-none"
						/>
					</div>
				</div>
			{/if}

			<!-- Tag list / create form -->
			<div class="overflow-y-auto p-1 flex-1">
				{#if showCreateForm}
					<div class="p-2.5">
						<div class="flex flex-col gap-2">
							<input
								bind:value={newTagName}
								placeholder="Tag name"
								autofocus
								disabled={isCreating}
								class="px-2.5 py-1.5 border border-gray-200 dark:border-gray-600 rounded-md text-xs bg-gray-50 dark:bg-gray-700 text-gray-900 dark:text-gray-100 placeholder-gray-400 focus:outline-none focus:ring-1 focus:ring-earthy-terracotta-500 disabled:opacity-60"
								onkeydown={(e) => {
									if (e.key === 'Enter') handleCreateTag();
								}}
							/>
							<input
								bind:value={newTagDesc}
								placeholder="Description (optional)"
								disabled={isCreating}
								class="px-2.5 py-1.5 border border-gray-200 dark:border-gray-600 rounded-md text-xs bg-gray-50 dark:bg-gray-700 text-gray-900 dark:text-gray-100 placeholder-gray-400 focus:outline-none focus:ring-1 focus:ring-earthy-terracotta-500 disabled:opacity-60"
							/>
							<div class="flex justify-end gap-1.5">
								<button
									onclick={() => {
										showCreateForm = false;
										newTagName = '';
										newTagDesc = '';
									}}
									disabled={isCreating}
									class="px-3 py-1 rounded-md text-xs font-medium text-gray-600 dark:text-gray-300 bg-gray-100 dark:bg-gray-700 hover:bg-gray-200 dark:hover:bg-gray-600 disabled:opacity-60 transition-colors"
								>
									Cancel
								</button>
								<button
									onclick={handleCreateTag}
									disabled={isCreating || !newTagName.trim()}
									class="px-3 py-1 rounded-md text-xs font-medium text-white bg-earthy-terracotta-700 hover:bg-earthy-terracotta-800 disabled:opacity-50 transition-colors"
								>
									{isCreating ? 'Creating…' : 'Create'}
								</button>
							</div>
						</div>
					</div>
				{:else if loading}
					<div class="px-4 py-7 text-center">
						<div class="text-gray-400 dark:text-gray-500">Loading tags…</div>
					</div>
				{:else if tags.length === 0}
					<div class="px-4 py-7 text-center">
						<div class="text-gray-700 dark:text-gray-300 font-medium mb-1">No tags yet</div>
					</div>
				{:else if filtered.length === 0}
					<div class="px-4 py-8 text-center text-gray-500 dark:text-gray-400">
						No tags match "{query}".
					</div>
				{:else}
					{#each filtered as tag (tag.id)}
						{@const active = selectedIds.has(tag.id)}
						<button
							onclick={() => toggle(tag.id)}
							disabled={isSaving}
							class="flex items-start gap-2.5 w-full px-2.5 py-1.5 rounded-md hover:bg-gray-100 dark:hover:bg-gray-700 text-left disabled:opacity-60 transition-colors"
						>
							<span
								class="w-3.5 h-3.5 rounded-sm border flex-shrink-0 mt-0.5 flex items-center justify-center {active
									? 'bg-earthy-terracotta-700 border-earthy-terracotta-700'
									: 'border-gray-300 dark:border-gray-500'}"
							>
								{#if active}
									<svg
										width="9"
										height="9"
										viewBox="0 0 24 24"
										fill="none"
										stroke="currentColor"
										stroke-width="3"
										stroke-linecap="round"
										stroke-linejoin="round"
										class="text-white"
										aria-hidden="true"><path d="m5 12 5 5L20 7" /></svg
									>
								{/if}
							</span>
							<span class="flex flex-col items-start gap-1 min-w-0">
								<TagBadge name={tag.name} class="whitespace-nowrap" />
								{#if tag.description}
									<span
										class="text-[11px] text-gray-500 dark:text-gray-400 leading-tight pl-0.5 whitespace-normal"
										>{tag.description}</span
									>
								{/if}
							</span>
						</button>
					{/each}
				{/if}
			</div>

			<!-- Footer -->
			{#if !showCreateForm}
				<div
					class="border-t border-gray-100 dark:border-gray-700 p-2 flex justify-end items-center gap-2 bg-gray-50 dark:bg-gray-800 flex-shrink-0"
				>
					<button
						onclick={() => {
							showCreateForm = true;
							query = '';
						}}
						class="flex items-center gap-1 px-3 py-1 text-xs font-medium text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-200 rounded-md hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors"
					>
						<svg
							width="11"
							height="11"
							viewBox="0 0 24 24"
							fill="none"
							stroke="currentColor"
							stroke-width="2"
							stroke-linecap="round"
							stroke-linejoin="round"
							aria-hidden="true"><path d="M12 5v14M5 12h14" /></svg
						>
						New tag
					</button>
					<button
						onclick={save}
						disabled={isSaving}
						class="px-3 py-1 text-xs font-medium rounded-md text-white bg-earthy-terracotta-700 hover:bg-earthy-terracotta-800 disabled:opacity-60 transition-colors"
					>
						{isSaving ? 'Saving…' : 'Done'}
					</button>
				</div>
			{/if}
		</div>
	</div>
{/if}
