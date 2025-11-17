<script lang="ts">
	import { onMount } from 'svelte';
	import { writable, type Writable } from 'svelte/store';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { fetchApi } from '$lib/api';
	import type { GlossaryTerm, TermsListResponse } from '$lib/glossary/types';
	import QueryInput from '../../components/QueryInput.svelte';
	import MarkdownRenderer from '../../components/MarkdownRenderer.svelte';
	import RichTextEditor from '../../components/RichTextEditor.svelte';
	import UserPicker from '../../components/UserPicker.svelte';
	import Icon from '@iconify/svelte';
	import { auth } from '$lib/stores/auth';

	const terms: Writable<GlossaryTerm[]> = writable([]);
	const totalTerms: Writable<number> = writable(0);
	const isLoading: Writable<boolean> = writable(true);
	const error: Writable<string | null> = writable(null);

	let searchQuery = $page.url.searchParams.get('q') || '';
	let searchTimeout: NodeJS.Timeout;

	let selectedTerm: GlossaryTerm | null = null;
	let showCreateModal = false;
	let showDeleteConfirm = false;

	let newTermName = '';
	let newTermDefinition = '';
	let newTermDescription = '';
	let newTermOwnerIds: string[] = [];
	let isCreating = false;
	let createError = '';

	let isEditing = false;
	let editedTerm: GlossaryTerm | null = null;

	$: canManageGlossary = auth.hasPermission('glossary', 'manage');

	$: {
		const query = $page.url.searchParams.get('q') || '';
		if (query !== searchQuery) {
			searchQuery = query;
		}

		// Handle term selection from URL
		const termId = $page.url.searchParams.get('term');
		if (termId && (!selectedTerm || selectedTerm.id !== termId)) {
			// Find the term in the loaded terms
			const term = $terms.find(t => t.id === termId);
			if (term) {
				selectedTerm = term;
			} else if (termId) {
				// Try to load the term directly if not in the list
				loadTermById(termId);
			}
		}
	}

	async function fetchTerms() {
		isLoading.set(true);
		error.set(null);

		try {
			const queryParams = new URLSearchParams({
				limit: '100',
				offset: '0'
			});

			if (searchQuery) {
				queryParams.append('q', searchQuery);
			}

			const endpoint = searchQuery ? '/glossary/search' : '/glossary/list';
			const response = await fetchApi(`${endpoint}?${queryParams}`);

			if (!response.ok) {
				const errorData = await response.json();
				throw new Error(errorData.error || 'Failed to fetch terms');
			}

			const data: TermsListResponse = await response.json();
			terms.set(data.terms);
			totalTerms.set(data.total);
		} catch (err) {
			error.set(err instanceof Error ? err.message : 'Failed to fetch terms');
		} finally {
			isLoading.set(false);
		}
	}

	function handleSearch(query: string) {
		clearTimeout(searchTimeout);
		searchQuery = query;

		searchTimeout = setTimeout(() => {
			const url = new URL(window.location.href);
			if (query) {
				url.searchParams.set('q', query);
			} else {
				url.searchParams.delete('q');
			}
			goto(url.toString(), { replaceState: true, noScroll: true, keepFocus: true });
		}, 300);
	}

	function handleSearchSubmit() {
		clearTimeout(searchTimeout);
		const url = new URL(window.location.href);
		if (searchQuery) {
			url.searchParams.set('q', searchQuery);
		} else {
			url.searchParams.delete('q');
		}
		goto(url.toString(), { replaceState: true, noScroll: true, keepFocus: true });
	}

	function selectTerm(term: GlossaryTerm) {
		selectedTerm = term;
		isEditing = false;
		editedTerm = null;

		// Update URL with selected term
		const url = new URL(window.location.href);
		url.searchParams.set('term', term.id);
		goto(url.toString(), { replaceState: true, noScroll: true, keepFocus: true });
	}

	async function loadTermById(termId: string) {
		try {
			const response = await fetchApi(`/glossary/${termId}`);
			if (response.ok) {
				const term: GlossaryTerm = await response.json();
				selectedTerm = term;
			}
		} catch (err) {
			console.error('Failed to load term:', err);
		}
	}

	function handleNewTerm() {
		newTermName = '';
		newTermDefinition = '';
		newTermDescription = '';
		newTermOwnerIds = [];
		createError = '';
		showCreateModal = true;
	}

	async function createTerm() {
		if (!newTermName || !newTermDefinition) {
			createError = 'Name and definition are required';
			return;
		}

		isCreating = true;
		createError = '';

		try {
			const response = await fetchApi('/glossary/', {
				method: 'POST',
				body: JSON.stringify({
					name: newTermName,
					definition: newTermDefinition,
					description: newTermDescription || undefined,
					owner_ids: newTermOwnerIds.length > 0 ? newTermOwnerIds : undefined
				})
			});

			if (!response.ok) {
				const errorData = await response.json();
				throw new Error(errorData.error || 'Failed to create term');
			}

			showCreateModal = false;
			fetchTerms();
		} catch (err) {
			createError = err instanceof Error ? err.message : 'Failed to create term';
		} finally {
			isCreating = false;
		}
	}

	function startEdit() {
		if (!selectedTerm) return;
		isEditing = true;
		editedTerm = JSON.parse(JSON.stringify(selectedTerm));
	}

	function cancelEdit() {
		isEditing = false;
		editedTerm = null;
	}

	async function saveEdit() {
		if (!selectedTerm || !editedTerm) return;

		try {
			const updateData: any = {};

			if (editedTerm.name !== selectedTerm.name) {
				updateData.name = editedTerm.name;
			}
			if (editedTerm.definition !== selectedTerm.definition) {
				updateData.definition = editedTerm.definition;
			}
			if (editedTerm.description !== selectedTerm.description) {
				updateData.description = editedTerm.description || null;
			}

			const currentOwnerIds = selectedTerm.owners.map((o) => o.id);
			const newOwnerIds = editedTerm.owners.map((o) => o.id);
			const ownersChanged =
				newOwnerIds.length !== currentOwnerIds.length ||
				!newOwnerIds.every((id) => currentOwnerIds.includes(id));

			if (ownersChanged && newOwnerIds.length > 0) {
				updateData.owner_ids = newOwnerIds;
			}

			if (Object.keys(updateData).length === 0) {
				cancelEdit();
				return;
			}

			const response = await fetchApi(`/glossary/${selectedTerm.id}`, {
				method: 'PUT',
				body: JSON.stringify(updateData)
			});

			if (!response.ok) {
				const errorData = await response.json();
				throw new Error(errorData.error || 'Failed to update term');
			}

			const updated = await response.json();
			selectedTerm = updated;

			terms.update((list) =>
				list.map((t) => (t.id === updated.id ? updated : t))
			);

			isEditing = false;
			editedTerm = null;
		} catch (err) {
			error.set(err instanceof Error ? err.message : 'Failed to update term');
		}
	}

	async function deleteTerm() {
		if (!selectedTerm) return;

		try {
			const response = await fetchApi(`/glossary/${selectedTerm.id}`, {
				method: 'DELETE'
			});

			if (!response.ok) {
				const errorData = await response.json();
				throw new Error(errorData.error || 'Failed to delete term');
			}

			showDeleteConfirm = false;
			selectedTerm = null;
			fetchTerms();
		} catch (err) {
			error.set(err instanceof Error ? err.message : 'Failed to delete term');
		}
	}

	$: {
		if ($page.url) {
			fetchTerms();
		}
	}

	onMount(() => {
		fetchTerms();
	});
</script>

<div class="h-[calc(100vh-4rem)] overflow-y-auto">
	<div class="max-w-[1600px] mx-auto px-6 py-6">
		<div class="flex gap-8">
			<!-- Left Sidebar -->
			<div class="w-80 flex-shrink-0 flex flex-col gap-4">
		<div class="flex items-center justify-between">
			<h1 class="text-xl font-bold text-gray-900 dark:text-gray-100">Glossary</h1>
			{#if canManageGlossary}
				<button
					on:click={handleNewTerm}
					class="inline-flex items-center justify-center w-9 h-9 rounded-lg text-white bg-orange-600 hover:bg-orange-700 dark:bg-orange-500 dark:hover:bg-orange-600 transition-colors"
					title="Create new term"
				>
					<Icon icon="material-symbols:add" class="h-5 w-5" />
				</button>
			{/if}
		</div>

		<QueryInput
			value={searchQuery}
			onQueryChange={handleSearch}
			onSubmit={handleSearchSubmit}
			placeholder="Search terms..."
		/>

		<div class="space-y-2">
			{#if $isLoading}
				<div class="flex justify-center items-center h-32">
					<div
						class="animate-spin rounded-full h-8 w-8 border-b-2 border-orange-600 dark:border-orange-400"
					></div>
				</div>
			{:else if $error}
				<div class="rounded-lg bg-red-50 dark:bg-red-900/20 p-3 text-sm text-red-800 dark:text-red-200">
					{$error}
				</div>
			{:else if $terms.length === 0}
				<div class="py-12 text-center">
					<Icon icon="material-symbols:book-outline" class="mx-auto h-12 w-12 text-gray-400 dark:text-gray-600" />
					<p class="mt-2 text-sm text-gray-500 dark:text-gray-400">
						{searchQuery ? 'No terms found' : 'No terms yet'}
					</p>
				</div>
			{:else}
				{#each $terms as term}
					<button
						on:click={() => selectTerm(term)}
						class="w-full px-4 py-3 text-left rounded-lg transition-all {selectedTerm?.id ===
						term.id
							? 'bg-orange-500/10 dark:bg-orange-500/20 ring-2 ring-orange-500/50 dark:ring-orange-400/50'
							: 'hover:bg-gray-100 dark:hover:bg-gray-800'}"
					>
						<div class="font-medium text-gray-900 dark:text-gray-100 text-sm">
							{term.name}
						</div>
						<div class="mt-0.5 text-xs text-gray-500 dark:text-gray-400 line-clamp-1">
							{term.definition}
						</div>
					</button>
				{/each}
			{/if}
		</div>

		<div class="text-xs text-gray-500 dark:text-gray-400 px-2">
			{$totalTerms} {$totalTerms === 1 ? 'term' : 'terms'}
		</div>
	</div>

	<!-- Right Detail Panel -->
	<div class="flex-1">
		{#if selectedTerm}
			<div>
				<div class="bg-white/40 dark:bg-gray-800/40 backdrop-blur-sm rounded-xl border border-gray-200/50 dark:border-gray-700/50">
					<div class="px-6 py-5 border-b border-gray-200/50 dark:border-gray-700/50">
						{#if isEditing && editedTerm}
							<input
								type="text"
								bind:value={editedTerm.name}
								class="w-full text-2xl font-bold border-b border-gray-300 dark:border-gray-600 bg-transparent focus:outline-none focus:border-gray-400 text-gray-900 dark:text-gray-100 pb-1"
								placeholder="Term name"
							/>
						{:else}
							<h2 class="text-2xl font-bold text-gray-900 dark:text-gray-100">
								{selectedTerm.name}
							</h2>
						{/if}
					</div>

					<div class="p-6 space-y-6">
						<div>
							<h3 class="text-xs font-semibold text-gray-500 dark:text-gray-400 uppercase tracking-wide mb-2">
								Definition
							</h3>
							{#if isEditing && editedTerm}
								<textarea
									bind:value={editedTerm.definition}
									rows="3"
									class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-md focus:outline-none focus:ring-1 focus:ring-gray-400 dark:bg-gray-700 dark:text-gray-100"
									placeholder="Clear, concise definition..."
								></textarea>
							{:else}
								<p class="text-gray-700 dark:text-gray-300">{selectedTerm.definition}</p>
							{/if}
						</div>

						<div>
							<h3 class="text-xs font-semibold text-gray-500 dark:text-gray-400 uppercase tracking-wide mb-2">
								Description
							</h3>
							{#if isEditing && editedTerm}
								<RichTextEditor
									bind:value={editedTerm.description}
									placeholder="Add a detailed description..."
								/>
							{:else if selectedTerm.description}
								<MarkdownRenderer content={selectedTerm.description} />
							{:else}
								<p class="text-sm text-gray-400 dark:text-gray-500 italic">No description provided</p>
							{/if}
						</div>

						<div>
							<h3 class="text-xs font-semibold text-gray-500 dark:text-gray-400 uppercase tracking-wide mb-2">
								Owners
							</h3>
							{#if isEditing && editedTerm}
								<UserPicker
									selectedUserIds={editedTerm.owners.map(o => o.id)}
									onChange={(ids, users) => {
										if (editedTerm) {
											editedTerm.owners = users;
										}
									}}
								/>
							{:else}
								<UserPicker
									selectedUserIds={selectedTerm.owners.map(o => o.id)}
									onChange={() => {}}
									disabled={true}
								/>
							{/if}
						</div>

						<div class="pt-4 border-t border-gray-200/50 dark:border-gray-700/50">
							<h3 class="text-xs font-semibold text-gray-500 dark:text-gray-400 uppercase tracking-wide mb-3">
								Metadata
							</h3>
							<dl class="grid grid-cols-2 gap-4 text-sm">
								<div>
									<dt class="text-gray-500 dark:text-gray-400">Created</dt>
									<dd class="text-gray-900 dark:text-gray-100 mt-0.5">
										{new Date(selectedTerm.created_at).toLocaleDateString()}
									</dd>
								</div>
								<div>
									<dt class="text-gray-500 dark:text-gray-400">Last Updated</dt>
									<dd class="text-gray-900 dark:text-gray-100 mt-0.5">
										{new Date(selectedTerm.updated_at).toLocaleDateString()}
									</dd>
								</div>
							</dl>
						</div>

						{#if canManageGlossary}
							<div class="pt-6 border-t border-gray-200/50 dark:border-gray-700/50 flex items-center justify-between">
								{#if isEditing}
									<div class="flex gap-3">
										<button
											on:click={saveEdit}
											class="px-4 py-2 text-sm font-medium text-white bg-orange-600 hover:bg-orange-700 dark:bg-orange-500 dark:hover:bg-orange-600 rounded-md transition-colors"
										>
											Save Changes
										</button>
										<button
											on:click={cancelEdit}
											class="px-4 py-2 text-sm font-medium text-gray-700 dark:text-gray-300 border border-gray-300 dark:border-gray-600 rounded-md hover:bg-gray-50 dark:hover:bg-gray-700 transition-colors"
										>
											Cancel
										</button>
									</div>
								{:else}
									<button
										on:click={startEdit}
										class="px-4 py-2 text-sm font-medium text-gray-700 dark:text-gray-300 border border-gray-300 dark:border-gray-600 rounded-md hover:bg-gray-50 dark:hover:bg-gray-700 transition-colors"
									>
										Edit
									</button>
								{/if}

								<button
									on:click={() => (showDeleteConfirm = true)}
									class="px-4 py-2 text-sm font-medium text-red-600 dark:text-red-400 hover:text-red-700 dark:hover:text-red-300 hover:bg-red-50 dark:hover:bg-red-900/20 rounded-md transition-colors"
								>
									Delete
								</button>
							</div>
						{/if}
					</div>
				</div>
			</div>
		{:else}
			<div class="flex items-center justify-center h-full">
				<div class="text-center max-w-md">
					<div class="inline-flex items-center justify-center w-16 h-16 rounded-xl bg-gray-100/50 dark:bg-gray-800/50 mb-4">
						<Icon icon="material-symbols:book-outline" class="h-8 w-8 text-gray-400 dark:text-gray-500" />
					</div>
					<h3 class="text-lg font-medium text-gray-900 dark:text-gray-100 mb-2">No Term Selected</h3>
					<p class="text-sm text-gray-500 dark:text-gray-400">
						Select a term from the list to view its details
					</p>
				</div>
			</div>
		{/if}
	</div>
		</div>
	</div>
</div>

<!-- Create Modal -->
{#if showCreateModal}
	<div class="fixed inset-0 z-50 overflow-y-auto">
		<div class="flex items-center justify-center min-h-screen px-4">
			<div
				class="fixed inset-0 bg-black/50 dark:bg-black/70 backdrop-blur-sm transition-opacity"
				on:click={() => !isCreating && (showCreateModal = false)}
				on:keypress={(e) => e.key === 'Enter' && !isCreating && (showCreateModal = false)}
				role="button"
				tabindex="0"
			></div>

			<div
				class="relative bg-white/95 dark:bg-gray-800/95 backdrop-blur-md rounded-xl shadow-2xl max-w-2xl w-full p-6 z-10 border border-gray-200/50 dark:border-gray-700/50"
			>
				<h3 class="text-lg font-medium text-gray-900 dark:text-gray-100 mb-4">
					Create New Term
				</h3>

				{#if createError}
					<div class="mb-4 rounded-lg bg-red-50 dark:bg-red-900/20 p-3">
						<div class="flex">
							<Icon icon="material-symbols:error" class="h-5 w-5 text-red-400" />
							<div class="ml-3">
								<p class="text-sm text-red-800 dark:text-red-200">{createError}</p>
							</div>
						</div>
					</div>
				{/if}

				<form on:submit|preventDefault={createTerm} class="space-y-4">
					<div>
						<label
							for="term-name"
							class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1"
						>
							Name <span class="text-red-500">*</span>
						</label>
						<input
							id="term-name"
							type="text"
							bind:value={newTermName}
							disabled={isCreating}
							placeholder="e.g., Customer Lifetime Value"
							class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:ring-orange-500 focus:border-orange-500 dark:bg-gray-700 dark:text-gray-100 disabled:opacity-50"
							required
						/>
					</div>

					<div>
						<label
							for="term-definition"
							class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1"
						>
							Definition <span class="text-red-500">*</span>
						</label>
						<textarea
							id="term-definition"
							bind:value={newTermDefinition}
							disabled={isCreating}
							placeholder="Clear, concise definition of the business term..."
							rows="3"
							class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:ring-orange-500 focus:border-orange-500 dark:bg-gray-700 dark:text-gray-100 disabled:opacity-50"
							required
						></textarea>
					</div>

					<div>
						<label
							for="term-description"
							class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1"
						>
							Description (Optional)
						</label>
						<RichTextEditor
							bind:value={newTermDescription}
							disabled={isCreating}
							placeholder="Additional context, usage examples, or detailed explanation..."
						/>
					</div>

					<div>
						<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
							Owners (Optional)
						</label>
						<UserPicker
							bind:selectedUserIds={newTermOwnerIds}
							onChange={(ids, users) => {
								newTermOwnerIds = ids;
							}}
							placeholder="Search and select owners..."
						/>
						<p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
							Leave empty to default to yourself
						</p>
					</div>

					<div class="flex justify-end gap-3 pt-4">
						<button
							type="button"
							on:click={() => (showCreateModal = false)}
							disabled={isCreating}
							class="px-4 py-2 border border-gray-300 dark:border-gray-600 rounded-md text-sm font-medium text-gray-700 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-700 disabled:opacity-50 disabled:cursor-not-allowed"
						>
							Cancel
						</button>
						<button
							type="submit"
							disabled={isCreating}
							class="inline-flex items-center px-4 py-2 rounded-md text-sm font-medium text-white bg-orange-600 hover:bg-orange-700 dark:bg-orange-500 dark:hover:bg-orange-600 disabled:opacity-50 disabled:cursor-not-allowed"
						>
							{#if isCreating}
								<div class="animate-spin rounded-full h-4 w-4 border-b-2 border-white mr-2"></div>
								Creating...
							{:else}
								Create Term
							{/if}
						</button>
					</div>
				</form>
			</div>
		</div>
	</div>
{/if}

<!-- Delete Confirm Modal -->
{#if showDeleteConfirm}
	<div class="fixed inset-0 z-50 overflow-y-auto">
		<div class="flex items-center justify-center min-h-screen px-4">
			<div
				class="fixed inset-0 bg-black/50 dark:bg-black/70 backdrop-blur-sm transition-opacity"
				on:click={() => (showDeleteConfirm = false)}
				on:keypress={(e) => e.key === 'Enter' && (showDeleteConfirm = false)}
				role="button"
				tabindex="0"
			></div>

			<div
				class="relative bg-white/95 dark:bg-gray-800/95 backdrop-blur-md rounded-xl shadow-2xl max-w-lg w-full p-6 z-10 border border-gray-200/50 dark:border-gray-700/50"
			>
				<h3 class="text-lg font-medium text-gray-900 dark:text-gray-100 mb-2">Delete Term</h3>
				<p class="text-sm text-gray-500 dark:text-gray-400 mb-4">
					Are you sure you want to delete "{selectedTerm?.name}"? This action cannot be undone.
				</p>
				<div class="flex justify-end gap-3">
					<button
						on:click={() => (showDeleteConfirm = false)}
						class="px-4 py-2 border border-gray-300 dark:border-gray-600 rounded-md text-sm font-medium text-gray-700 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-700"
					>
						Cancel
					</button>
					<button
						on:click={deleteTerm}
						class="px-4 py-2 rounded-md text-sm font-medium text-white bg-red-600 hover:bg-red-700 dark:bg-red-500 dark:hover:bg-red-600"
					>
						Delete
					</button>
				</div>
			</div>
		</div>
	</div>
{/if}
