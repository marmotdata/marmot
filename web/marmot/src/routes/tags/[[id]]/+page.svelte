<script lang="ts">
	import type { Tag } from '$lib/tags/types';
	import { auth } from '$lib/stores/auth';
	import { listTags, createTag, updateTag, deleteTag } from '$lib/tags/api';
	import TagBadge from '$components/shared/TagBadge.svelte';
	import Icon from '@iconify/svelte';

	const canManage = $derived(auth.hasPermission('assets', 'manage'));

	let tags = $state<Tag[]>([]);
	let isLoading = $state(true);
	let loadError = $state('');
	let search = $state('');

	// Create form state
	let showCreate = $state(false);
	let createName = $state('');
	let createDescription = $state('');
	let createError = $state('');
	let isCreating = $state(false);

	// Edit state (one row at a time)
	let editingId = $state<string | null>(null);
	let editName = $state('');
	let editDescription = $state('');
	let editError = $state('');
	let isSaving = $state(false);

	// Delete confirmation
	let deletingId = $state<string | null>(null);
	let isDeleting = $state(false);

	// Sort
	type SortKey = 'name-asc' | 'name-desc' | 'newest' | 'oldest';
	let sortKey = $state<SortKey>('name-asc');
	let showSortMenu = $state(false);

	const SORT_LABELS: Record<SortKey, string> = {
		'name-asc': 'Name A–Z',
		'name-desc': 'Name Z–A',
		newest: 'Newest',
		oldest: 'Oldest'
	};

	function sortTags(list: Tag[], key: SortKey): Tag[] {
		return [...list].sort((a, b) => {
			switch (key) {
				case 'name-asc':
					return a.name.localeCompare(b.name);
				case 'name-desc':
					return b.name.localeCompare(a.name);
				case 'newest':
					return new Date(b.created_at ?? 0).getTime() - new Date(a.created_at ?? 0).getTime();
				case 'oldest':
					return new Date(a.created_at ?? 0).getTime() - new Date(b.created_at ?? 0).getTime();
			}
		});
	}

	const filtered = $derived(
		sortTags(
			search.trim()
				? tags.filter(
						(t) =>
							t.name.toLowerCase().includes(search.toLowerCase()) ||
							(t.description ?? '').toLowerCase().includes(search.toLowerCase())
					)
				: tags,
			sortKey
		)
	);

	async function load() {
		isLoading = true;
		loadError = '';
		try {
			tags = (await listTags()) ?? [];
		} catch (e) {
			loadError = e instanceof Error ? e.message : 'Failed to load tags';
		} finally {
			isLoading = false;
		}
	}

	$effect(() => {
		load();
	});

	function openCreate() {
		showCreate = true;
		createName = '';
		createDescription = '';
		createError = '';
		editingId = null;
	}

	function cancelCreate() {
		showCreate = false;
		createName = '';
		createDescription = '';
		createError = '';
	}

	async function submitCreate() {
		if (!createName.trim()) {
			createError = 'Tag name is required.';
			return;
		}
		isCreating = true;
		createError = '';
		try {
			const created = await createTag({
				name: createName.trim(),
				description: createDescription.trim()
			});
			tags = [...tags, created].sort((a, b) => a.name.localeCompare(b.name));
			cancelCreate();
		} catch (e) {
			const msg = e instanceof Error ? e.message : 'Failed to create tag';
			createError = msg.includes('already exists') ? `"${createName.trim()}" already exists.` : msg;
		} finally {
			isCreating = false;
		}
	}

	function openEdit(tag: Tag) {
		editingId = tag.id;
		editName = tag.name;
		editDescription = tag.description ?? '';
		editError = '';
		showCreate = false;
	}

	function cancelEdit() {
		editingId = null;
		editName = '';
		editDescription = '';
		editError = '';
	}

	async function submitEdit() {
		if (!editName.trim()) {
			editError = 'Tag name is required.';
			return;
		}
		if (!editingId) return;
		isSaving = true;
		editError = '';
		try {
			const updated = await updateTag(editingId, {
				name: editName.trim(),
				description: editDescription.trim()
			});
			tags = tags
				.map((t) => (t.id === updated.id ? updated : t))
				.sort((a, b) => a.name.localeCompare(b.name));
			cancelEdit();
		} catch (e) {
			const msg = e instanceof Error ? e.message : 'Failed to save';
			editError = msg.includes('already exists') ? `"${editName.trim()}" already exists.` : msg;
		} finally {
			isSaving = false;
		}
	}

	async function confirmDelete(id: string) {
		isDeleting = true;
		try {
			await deleteTag(id);
			tags = tags.filter((t) => t.id !== id);
			deletingId = null;
		} catch (e) {
			// deletion failed — just close
			deletingId = null;
		} finally {
			isDeleting = false;
		}
	}
</script>

<svelte:head>
	<title>Tags – Marmot</title>
</svelte:head>

{#snippet sortButton()}
	<div class="relative">
		<button
			onclick={() => (showSortMenu = !showSortMenu)}
			class="inline-flex items-center gap-1 text-xs font-medium text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-200 transition-colors"
		>
			Sort: {SORT_LABELS[sortKey]}
			<Icon icon="material-symbols:arrow-drop-down" class="w-4 h-4" />
		</button>
		{#if showSortMenu}
			<!-- svelte-ignore a11y_no_static_element_interactions -->
			<!-- svelte-ignore a11y_click_events_have_key_events -->
			<div class="fixed inset-0 z-10" onclick={() => (showSortMenu = false)}></div>
			<div
				class="absolute right-0 top-full mt-1 w-36 bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-md shadow-lg z-20 py-1"
			>
				{#each Object.entries(SORT_LABELS) as [key, label] (key)}
					<button
						onclick={() => {
							sortKey = key as SortKey;
							showSortMenu = false;
						}}
						class="w-full text-left px-3 py-1.5 text-sm transition-colors {sortKey === key
							? 'text-earthy-terracotta-700 dark:text-earthy-terracotta-400 font-medium bg-earthy-terracotta-50 dark:bg-earthy-terracotta-900/20'
							: 'text-gray-700 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-700/50'}"
						>{label}</button
					>
				{/each}
			</div>
		{/if}
	</div>
{/snippet}

<div class="max-w-4xl mx-auto px-4 py-8">
	<!-- toolbar -->
	<div class="flex items-center gap-3 mb-4">
		<div class="relative flex-1 max-w-xs">
			<Icon
				icon="material-symbols:search"
				class="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-400 pointer-events-none"
			/>
			<input
				type="search"
				bind:value={search}
				placeholder="Search all tags"
				class="w-full pl-9 pr-3 py-1.5 text-sm border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 placeholder-gray-400 focus:outline-none focus:ring-2 focus:ring-earthy-terracotta-600 focus:border-earthy-terracotta-700"
			/>
		</div>

		{#if canManage}
			<button
				onclick={openCreate}
				class="ml-auto inline-flex items-center gap-1.5 px-3 py-1.5 text-sm font-medium rounded-md bg-earthy-terracotta-700 hover:bg-earthy-terracotta-800 text-white focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-earthy-terracotta-600 transition-colors"
			>
				<Icon icon="material-symbols:add" class="w-4 h-4" />
				New tag
			</button>
		{/if}
	</div>

	<!-- create form -->
	{#if showCreate}
		<div
			class="border border-gray-200 dark:border-gray-700 rounded-t-md bg-gray-50 dark:bg-gray-800/50 px-4 py-3"
		>
			<!-- inputs + buttons on one line -->
			<div class="grid grid-cols-[1fr_2fr_auto] gap-3 items-end">
				<div>
					<label
						for="create-name"
						class="block text-xs font-medium text-gray-500 dark:text-gray-400 mb-1">Tag name</label
					>
					<input
						id="create-name"
						type="text"
						bind:value={createName}
						placeholder="e.g. PII:Email"
						disabled={isCreating}
						class="w-full px-3 py-1.5 text-sm border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:outline-none focus:ring-2 focus:ring-earthy-terracotta-600 disabled:opacity-50"
					/>
				</div>
				<div>
					<label
						for="create-desc"
						class="block text-xs font-medium text-gray-500 dark:text-gray-400 mb-1"
						>Description</label
					>
					<input
						id="create-desc"
						type="text"
						bind:value={createDescription}
						placeholder="Optional description"
						disabled={isCreating}
						class="w-full px-3 py-1.5 text-sm border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:outline-none focus:ring-2 focus:ring-earthy-terracotta-600 disabled:opacity-50"
					/>
				</div>
				<div class="flex items-center gap-2 whitespace-nowrap">
					<button
						onclick={submitCreate}
						disabled={isCreating || !createName.trim()}
						class="px-2.5 py-1 text-sm font-medium rounded-md bg-earthy-terracotta-700 hover:bg-earthy-terracotta-800 text-white transition-colors disabled:opacity-50"
						>{isCreating ? 'Creating…' : 'Create'}</button
					>
					<button
						onclick={cancelCreate}
						disabled={isCreating}
						class="text-sm font-medium text-gray-500 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100 transition-colors disabled:opacity-50"
						>Cancel</button
					>
				</div>
			</div>
			{#if createError}
				<p class="mt-1 text-xs text-red-600 dark:text-red-400">{createError}</p>
			{/if}
		</div>
	{/if}

	<!-- tag list -->
	{#if isLoading}
		<div
			class="border border-gray-200 dark:border-gray-700 rounded-md {showCreate
				? 'border-t-0 rounded-t-none'
				: ''}"
		>
			<!-- header -->
			<div
				class="flex items-center justify-between px-4 py-2 border-b border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-800/60 rounded-t-md"
			>
				<div class="grid grid-cols-[1fr_2fr] gap-4 flex-1 mr-4">
					<span
						class="text-xs font-semibold uppercase tracking-wider text-gray-500 dark:text-gray-400"
						>Tag</span
					>
					<span
						class="text-xs font-semibold uppercase tracking-wider text-gray-500 dark:text-gray-400"
						>Description</span
					>
				</div>
			</div>
			{#each Array(5) as _, i (i)}
				<div
					class="grid grid-cols-[1fr_2fr_auto] gap-4 items-center px-4 py-3 border-b border-gray-100 dark:border-gray-700/50 last:border-b-0 animate-pulse"
				>
					<div class="h-5 w-28 bg-gray-200 dark:bg-gray-700 rounded-full"></div>
					<div class="h-4 w-48 bg-gray-100 dark:bg-gray-700/50 rounded"></div>
					<div class="w-24"></div>
				</div>
			{/each}
		</div>
	{:else if loadError}
		<div class="border border-red-200 dark:border-red-800 rounded-md px-4 py-6 text-center">
			<p class="text-sm text-red-600 dark:text-red-400">{loadError}</p>
			<button onclick={load} class="mt-2 text-sm text-earthy-terracotta-700 hover:underline"
				>Retry</button
			>
		</div>
	{:else if filtered.length === 0 && !showCreate}
		<div class="border border-gray-200 dark:border-gray-700 rounded-md">
			<!-- header -->
			<div
				class="flex items-center justify-between px-4 py-2 border-b border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-800/60 rounded-t-md"
			>
				<div class="grid grid-cols-[1fr_2fr] gap-4 flex-1 mr-4">
					<span
						class="text-xs font-semibold uppercase tracking-wider text-gray-500 dark:text-gray-400"
						>Tag</span
					>
					<span
						class="text-xs font-semibold uppercase tracking-wider text-gray-500 dark:text-gray-400"
						>Description</span
					>
				</div>
				{@render sortButton()}
			</div>
			<div class="px-4 py-12 text-center">
				<Icon
					icon="material-symbols:label-outline"
					class="mx-auto w-10 h-10 text-gray-300 dark:text-gray-600 mb-3"
				/>
				<p class="text-sm text-gray-500 dark:text-gray-400">
					{search.trim() ? 'No tags match your search.' : 'No tags yet.'}
				</p>
			</div>
		</div>
	{:else}
		<div
			class="border border-gray-200 dark:border-gray-700 rounded-md {showCreate
				? 'border-t-0 rounded-t-none'
				: ''}"
		>
			<!-- header -->
			<div
				class="flex items-center justify-between px-4 py-2 border-b border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-800/60 {showCreate
					? ''
					: 'rounded-t-md'}"
			>
				<div class="grid grid-cols-[1fr_2fr] gap-4 flex-1 mr-4">
					<span
						class="text-xs font-semibold uppercase tracking-wider text-gray-500 dark:text-gray-400"
						>Tag</span
					>
					<span
						class="text-xs font-semibold uppercase tracking-wider text-gray-500 dark:text-gray-400"
						>Description</span
					>
				</div>
				{@render sortButton()}
			</div>

			{#each filtered as tag (tag.id)}
				{#if editingId === tag.id}
					<!-- inline edit form -->
					<div
						class="px-4 py-3 border-b border-gray-100 dark:border-gray-700/50 last:border-b-0 bg-gray-50 dark:bg-gray-800/40"
					>
						<div class="grid grid-cols-[1fr_2fr_auto] gap-3 items-center">
							<input
								id="edit-name"
								type="text"
								bind:value={editName}
								disabled={isSaving}
								placeholder="Tag name"
								class="w-full px-3 py-1.5 text-sm border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:outline-none focus:ring-2 focus:ring-earthy-terracotta-600 disabled:opacity-50"
							/>
							<input
								id="edit-desc"
								type="text"
								bind:value={editDescription}
								disabled={isSaving}
								placeholder="Description"
								class="w-full px-3 py-1.5 text-sm border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:outline-none focus:ring-2 focus:ring-earthy-terracotta-600 disabled:opacity-50"
							/>
							<div class="flex items-center gap-2 whitespace-nowrap">
								<button
									onclick={submitEdit}
									disabled={isSaving || !editName.trim()}
									class="px-2.5 py-1 text-sm font-medium rounded-md bg-earthy-terracotta-700 hover:bg-earthy-terracotta-800 text-white transition-colors disabled:opacity-50"
									>{isSaving ? 'Saving…' : 'Save'}</button
								>
								<button
									onclick={cancelEdit}
									disabled={isSaving}
									class="text-sm font-medium text-gray-500 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100 transition-colors disabled:opacity-50"
									>Cancel</button
								>
							</div>
						</div>
						{#if editError}
							<p class="mt-1 text-xs text-red-600 dark:text-red-400">{editError}</p>
						{/if}
					</div>
				{:else if deletingId === tag.id}
					<!-- delete confirmation -->
					<div
						class="grid grid-cols-[1fr_2fr_auto] gap-4 items-center px-4 py-3 border-b border-gray-100 dark:border-gray-700/50 last:border-b-0 bg-red-50 dark:bg-red-900/10"
					>
						<TagBadge name={tag.name} class="w-fit" />
						<span class="text-sm text-gray-700 dark:text-gray-300">
							This tag will be removed from all assets and columns.
						</span>
						<div class="flex items-center gap-2 w-24 justify-end">
							<button
								onclick={() => (deletingId = null)}
								disabled={isDeleting}
								class="text-sm font-medium text-gray-500 dark:text-gray-400 hover:text-gray-900 transition-colors disabled:opacity-50"
								>Cancel</button
							>
							<button
								onclick={() => confirmDelete(tag.id)}
								disabled={isDeleting}
								class="px-2.5 py-1 text-sm font-medium rounded-md bg-red-600 hover:bg-red-700 text-white transition-colors disabled:opacity-50"
								>{isDeleting ? '…' : 'Delete'}</button
							>
						</div>
					</div>
				{:else}
					<!-- normal row -->
					<div
						class="grid grid-cols-[1fr_2fr_auto] gap-4 items-center px-4 py-3 border-b border-gray-100 dark:border-gray-700/50 last:border-b-0 hover:bg-gray-50 dark:hover:bg-gray-800/30 group"
					>
						<TagBadge name={tag.name} class="w-fit max-w-full truncate" />

						<span class="text-sm text-gray-600 dark:text-gray-400 truncate">
							{tag.description ?? ''}
						</span>

						<div
							class="w-24 flex items-center gap-3 justify-end {canManage
								? 'opacity-0 group-hover:opacity-100'
								: ''} transition-opacity"
						>
							{#if canManage}
								<button
									onclick={() => openEdit(tag)}
									class="text-sm text-gray-500 dark:text-gray-400 hover:text-earthy-terracotta-700 dark:hover:text-earthy-terracotta-400 font-medium transition-colors"
									>Edit</button
								>
								<button
									onclick={() => (deletingId = tag.id)}
									class="text-sm text-gray-500 dark:text-gray-400 hover:text-red-600 dark:hover:text-red-400 font-medium transition-colors"
									>Delete</button
								>
							{/if}
						</div>
					</div>
				{/if}
			{/each}
		</div>
	{/if}
</div>
