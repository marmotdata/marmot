<script lang="ts">
	import { untrack } from 'svelte';
	import IconifyIcon from '@iconify/svelte';
	import Button from '$components/ui/Button.svelte';
	import ConfirmModal from '$components/ui/ConfirmModal.svelte';
	import Pagination from '$components/ui/Pagination.svelte';
	import { auth } from '$lib/stores/auth';
	import { toasts } from '$lib/stores/toast';
	import { m } from '$lib/paraglide/messages';
	import {
		forgetMemory,
		listMemory,
		rememberMemory,
		searchMemory,
		updateMemory
	} from '$lib/memory/api';
	import type { Memory, MemoryEntityType, MemorySort } from '$lib/memory/types';

	let { entityType, entityId }: { entityType: MemoryEntityType; entityId: string } = $props();

	let canWrite = $derived(auth.hasPermission('memory', 'write'));

	const PAGE_SIZE = 5;
	// Matches the server's limit: a memory is one short fact.
	const MAX_LENGTH = 280;

	let memories = $state<Memory[]>([]);
	let total = $state(0);
	let loading = $state(true);
	let page = $state(1);
	let sort = $state<MemorySort>('changed');

	let query = $state('');
	let searchResults = $state<Memory[] | null>(null);
	let searching = $state(false);

	let editingId = $state<string | null>(null);
	let editContent = $state('');
	let saving = $state(false);

	let adding = $state(false);
	let newContent = $state('');

	let toDelete = $state<Memory | null>(null);
	let showDelete = $state(false);

	// Guards against an older response overwriting a newer one.
	let loadSeq = 0;
	let searchSeq = 0;
	let searchTimer: ReturnType<typeof setTimeout> | undefined;

	function errorText(err: unknown, fallback: string): string {
		return err instanceof Error ? err.message : fallback;
	}

	async function load() {
		const seq = ++loadSeq;
		loading = true;
		try {
			const result = await listMemory(
				entityType,
				entityId,
				sort,
				PAGE_SIZE,
				(page - 1) * PAGE_SIZE
			);
			if (seq !== loadSeq) return;
			memories = result.memories;
			total = result.total;
			// Past the last page, e.g. after deletions elsewhere: show the last page.
			if (memories.length === 0 && total > 0 && page > 1) {
				page = Math.ceil(total / PAGE_SIZE);
				await load();
			}
		} catch (err) {
			if (seq === loadSeq) toasts.error(errorText(err, m.memory_error_load()));
		} finally {
			if (seq === loadSeq) loading = false;
		}
	}

	function onQueryInput() {
		clearTimeout(searchTimer);
		searchTimer = setTimeout(runSearch, 300);
	}

	async function runSearch() {
		const q = query.trim();
		if (!q) {
			if (searchResults) clearSearch();
			return;
		}
		const seq = ++searchSeq;
		searching = true;
		try {
			const result = await searchMemory(entityType, entityId, q);
			if (seq !== searchSeq) return;
			searchResults = result.memories;
			page = 1;
		} catch (err) {
			if (seq === searchSeq) toasts.error(errorText(err, m.memory_error_search()));
		} finally {
			if (seq === searchSeq) searching = false;
		}
	}

	function clearSearch() {
		clearTimeout(searchTimer);
		searchSeq++;
		searching = false;
		query = '';
		searchResults = null;
		goToPage(1);
	}

	function goToPage(p: number) {
		page = p;
		if (!searchResults) load();
	}

	function replace(list: Memory[], updated: Memory): Memory[] {
		return list.map((mem) => (mem.id === updated.id ? updated : mem));
	}

	function startEdit(mem: Memory) {
		editingId = mem.id;
		editContent = mem.content;
	}

	async function saveEdit(mem: Memory) {
		saving = true;
		try {
			const updated = await updateMemory(entityType, entityId, mem.id, editContent);
			editingId = null;
			if (searchResults) {
				searchResults = replace(searchResults, updated);
			} else {
				// The list is ordered by last change, so the edit moves to the top.
				goToPage(1);
			}
		} catch (err) {
			toasts.error(errorText(err, m.memory_error_save()));
		} finally {
			saving = false;
		}
	}

	async function add() {
		if (!newContent.trim()) return;
		saving = true;
		try {
			await rememberMemory(entityType, entityId, newContent);
			adding = false;
			newContent = '';
			clearSearch();
		} catch (err) {
			toasts.error(errorText(err, m.memory_error_save()));
		} finally {
			saving = false;
		}
	}

	async function confirmDelete() {
		const mem = toDelete;
		showDelete = false;
		toDelete = null;
		if (!mem) return;
		try {
			await forgetMemory(entityType, entityId, mem.id);
			if (searchResults) {
				searchResults = searchResults.filter((x) => x.id !== mem.id);
				page = Math.min(page, Math.max(1, Math.ceil(searchResults.length / PAGE_SIZE)));
			} else {
				// Step back when the last entry on a page is deleted.
				goToPage(memories.length === 1 && page > 1 ? page - 1 : page);
			}
		} catch (err) {
			toasts.error(errorText(err, m.memory_error_delete()));
		}
	}

	function askDelete(mem: Memory) {
		toDelete = mem;
		showDelete = true;
	}

	function formatTime(iso: string): string {
		return new Date(iso).toLocaleString();
	}

	function authorIcon(type: string): string {
		return type === 'user' ? 'material-symbols:person' : 'material-symbols:smart-toy-outline';
	}

	function editedBySomeoneElse(mem: Memory): boolean {
		return mem.updated_by.id !== mem.created_by.id;
	}

	$effect(() => {
		if (entityId) {
			untrack(() => {
				clearTimeout(searchTimer);
				searchSeq++;
				searching = false;
				query = '';
				searchResults = null;
				editingId = null;
				adding = false;
				newContent = '';
				memories = [];
				total = 0;
				page = 1;
				load();
			});
		}
	});

	let count = $derived(searchResults ? searchResults.length : total);
	let shown = $derived(
		searchResults ? searchResults.slice((page - 1) * PAGE_SIZE, page * PAGE_SIZE) : memories
	);
</script>

<section class="space-y-4">
	<div class="flex items-center justify-between">
		<div>
			<h3 class="text-lg font-medium text-gray-900 dark:text-gray-100">{m.memory_tab()}</h3>
			<p class="text-sm text-gray-500 dark:text-gray-400">{m.memory_description()}</p>
		</div>
		{#if canWrite && !adding}
			<Button
				variant="clear"
				icon="material-symbols:add"
				text={m.memory_add()}
				click={() => (adding = true)}
			/>
		{/if}
	</div>

	{#if adding}
		<div class="p-4 rounded-lg border border-gray-200 dark:border-gray-700 space-y-3">
			<input
				class="w-full px-3 py-2.5 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-800 text-sm text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-earthy-terracotta-500 focus:border-earthy-terracotta-500"
				maxlength={MAX_LENGTH}
				placeholder={m.memory_add_placeholder()}
				bind:value={newContent}
			/>
			<div class="flex justify-end gap-2">
				<Button variant="clear" text={m.common_cancel()} click={() => (adding = false)} />
				<Button text={m.common_save()} loading={saving} click={add} />
			</div>
		</div>
	{/if}

	<div class="flex flex-wrap gap-2">
		<div class="relative flex-1 min-w-48">
			<IconifyIcon
				icon="material-symbols:search"
				class="absolute left-3 top-1/2 -translate-y-1/2 w-5 h-5 text-gray-400"
			/>
			<input
				type="text"
				class="w-full pl-10 pr-10 py-2.5 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-800 text-sm text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-earthy-terracotta-500 focus:border-earthy-terracotta-500"
				placeholder={m.memory_search_placeholder()}
				bind:value={query}
				oninput={onQueryInput}
			/>
			{#if searching}
				<div class="absolute right-3 top-1/2 -translate-y-1/2">
					<div
						class="h-4 w-4 border-2 border-earthy-terracotta-500 border-t-transparent rounded-full animate-spin"
					></div>
				</div>
			{:else if query}
				<button
					class="absolute right-3 top-1/2 -translate-y-1/2 text-gray-400 hover:text-gray-600 dark:hover:text-gray-200"
					title={m.common_clear()}
					onclick={clearSearch}
				>
					<IconifyIcon icon="material-symbols:close" class="w-4 h-4" />
				</button>
			{/if}
		</div>
		{#if !searchResults}
			<select
				class="px-3 py-2.5 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-800 text-sm text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-earthy-terracotta-500 focus:border-earthy-terracotta-500"
				aria-label={m.memory_sort_label()}
				bind:value={sort}
				onchange={() => goToPage(1)}
			>
				<option value="changed">{m.memory_sort_changed()}</option>
				<option value="created">{m.memory_sort_created()}</option>
			</select>
		{/if}
	</div>

	{#if searchResults}
		<p class="text-xs text-gray-500 dark:text-gray-400">
			{m.memory_search_results({ count })}
		</p>
	{/if}

	{#if loading && shown.length === 0}
		<p class="text-sm text-gray-500 dark:text-gray-400">{m.common_loading()}</p>
	{:else if shown.length === 0}
		<p class="text-sm text-gray-500 dark:text-gray-400">
			{searchResults ? m.memory_no_match() : m.memory_empty()}
		</p>
	{:else}
		<ol class="space-y-3 transition-opacity" class:opacity-60={loading}>
			{#each shown as mem (mem.id)}
				<li class="p-4 rounded-lg border border-gray-200 dark:border-gray-700 flex gap-4">
					<div class="flex-1 min-w-0">
						{#if editingId === mem.id}
							<input
								class="w-full px-3 py-2.5 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-800 text-sm text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-earthy-terracotta-500 focus:border-earthy-terracotta-500"
								maxlength={MAX_LENGTH}
								bind:value={editContent}
							/>
							<div class="flex justify-end gap-2 mt-2">
								<Button variant="clear" text={m.common_cancel()} click={() => (editingId = null)} />
								<Button text={m.common_save()} loading={saving} click={() => saveEdit(mem)} />
							</div>
						{:else}
							<p class="text-sm text-gray-800 dark:text-gray-200 whitespace-pre-wrap">
								{mem.content}
							</p>
							<div
								class="mt-2 flex flex-wrap items-center gap-x-2 gap-y-1 text-xs text-gray-500 dark:text-gray-400"
							>
								<span class="inline-flex items-center gap-1">
									<IconifyIcon icon={authorIcon(mem.created_by.type)} class="h-3.5 w-3.5" />
									{mem.created_by.name}
								</span>
								{#if editedBySomeoneElse(mem)}
									<span>· {m.memory_edited_by({ name: mem.updated_by.name })}</span>
								{/if}
								<span>· {formatTime(mem.updated_at)}</span>
								{#if mem.session_id}
									<span class="font-mono">· {m.memory_session({ id: mem.session_id })}</span>
								{/if}
							</div>
						{/if}
					</div>
					{#if canWrite && editingId !== mem.id}
						<div class="flex items-start gap-1">
							<button
								class="p-1 text-gray-400 hover:text-gray-700 dark:hover:text-gray-200"
								title={m.common_edit()}
								onclick={() => startEdit(mem)}
							>
								<IconifyIcon icon="material-symbols:edit-outline" class="h-4 w-4" />
							</button>
							<button
								class="p-1 text-gray-400 hover:text-red-600"
								title={m.common_delete()}
								onclick={() => askDelete(mem)}
							>
								<IconifyIcon icon="material-symbols:delete-outline" class="h-4 w-4" />
							</button>
						</div>
					{/if}
				</li>
			{/each}
		</ol>
		<Pagination {page} pageSize={PAGE_SIZE} total={count} disabled={loading} onChange={goToPage} />
	{/if}
</section>

<ConfirmModal
	bind:show={showDelete}
	title={m.memory_delete_title()}
	message={m.memory_delete_message()}
	confirmText={m.common_delete()}
	variant="danger"
	onConfirm={confirmDelete}
	onCancel={() => {
		showDelete = false;
		toDelete = null;
	}}
/>
