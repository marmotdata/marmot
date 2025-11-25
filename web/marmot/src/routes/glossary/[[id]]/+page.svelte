<script lang="ts">
	import { onMount } from 'svelte';
	import { writable, type Writable } from 'svelte/store';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { fetchApi } from '$lib/api';
	import type { GlossaryTerm, TermsListResponse, Owner } from '$lib/glossary/types';
	import QueryInput from '../../../components/QueryInput.svelte';
	import MarkdownRenderer from '../../../components/MarkdownRenderer.svelte';
	import RichTextEditor from '../../../components/RichTextEditor.svelte';
	import OwnerSelector from '../../../components/OwnerSelector.svelte';
	import Button from '../../../components/Button.svelte';
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
	let newTermOwners: Owner[] = [];
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

		// Handle term selection from URL path
		const termId = $page.params.id;
		if (termId && (!selectedTerm || selectedTerm.id !== termId)) {
			// Find the term in the loaded terms
			const term = $terms.find(t => t.id === termId);
			if (term) {
				selectedTerm = term;
			} else if (termId) {
				// Try to load the term directly if not in the list
				loadTermById(termId);
			}
		} else if (!termId && !selectedTerm && $terms.length > 0) {
			// Auto-select first term if no term is selected and no term in URL
			selectTerm($terms[0]);
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

		// Update URL with selected term using path parameter
		const searchParams = $page.url.searchParams.toString();
		const url = searchParams ? `/glossary/${term.id}?${searchParams}` : `/glossary/${term.id}`;
		goto(url, { replaceState: true, noScroll: true, keepFocus: true });
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
		newTermOwners = [];
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
			const owners = newTermOwners.length > 0
				? newTermOwners.map(o => ({ id: o.id, type: o.type }))
				: undefined;

			const response = await fetchApi('/glossary/', {
				method: 'POST',
				body: JSON.stringify({
					name: newTermName,
					definition: newTermDefinition,
					description: newTermDescription || undefined,
					owners
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

			const currentOwners = selectedTerm.owners.map((o) => ({ id: o.id, type: o.type }));
			const newOwners = editedTerm.owners.map((o) => ({ id: o.id, type: o.type }));
			const ownersChanged =
				newOwners.length !== currentOwners.length ||
				!newOwners.every((no) => currentOwners.some((co) => co.id === no.id && co.type === no.type));

			if (ownersChanged) {
				updateData.owners = newOwners;
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
		<div class="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-4">
			<div class="flex items-center justify-between mb-4">
				<h1 class="text-xl font-bold text-gray-900 dark:text-gray-100">Glossary</h1>
				{#if canManageGlossary}
					<Button
						click={handleNewTerm}
						icon="material-symbols:add"
						variant="filled"
						class="!p-2"
					/>
				{/if}
			</div>

			<QueryInput
				value={searchQuery}
				onQueryChange={handleSearch}
				onSubmit={handleSearchSubmit}
				placeholder="Search terms..."
			/>

			<div class="text-xs text-gray-500 dark:text-gray-400 mt-3">
				{$totalTerms} {$totalTerms === 1 ? 'term' : 'terms'}
			</div>
		</div>

		<div class="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 overflow-hidden flex-1 min-h-0">
			{#if $isLoading}
				<div class="flex justify-center items-center h-32">
					<div
						class="animate-spin rounded-full h-8 w-8 border-b-2 border-earthy-terracotta-700 dark:border-earthy-terracotta-500"
					></div>
				</div>
			{:else if $error}
				<div class="m-4 rounded-lg bg-red-50 dark:bg-red-900/20 p-3 text-sm text-red-800 dark:text-red-200">
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
				<div class="overflow-y-auto max-h-full">
					{#each $terms as term}
						<button
							on:click={() => selectTerm(term)}
							class="w-full px-4 py-3 text-left transition-all border-l-4 {selectedTerm?.id ===
							term.id
								? 'bg-earthy-terracotta-50 dark:bg-earthy-terracotta-900/20 border-earthy-terracotta-700 dark:border-earthy-terracotta-500'
								: 'hover:bg-gray-50 dark:hover:bg-gray-700/50 border-transparent'}"
						>
							<div class="font-medium text-gray-900 dark:text-gray-100 text-sm">
								{term.name}
							</div>
							<div class="mt-0.5 text-xs text-gray-500 dark:text-gray-400 line-clamp-1">
								{term.definition}
							</div>
						</button>
					{/each}
				</div>
			{/if}
		</div>
	</div>

	<!-- Right Detail Panel -->
	<div class="flex-1">
		{#if selectedTerm}
			<div>
				<div class="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 overflow-hidden">
					<!-- Header Section -->
					<div class="relative px-6 py-5 border-b border-gray-200 dark:border-gray-700">
						<!-- Term Name -->
						{#if isEditing && editedTerm}
							<input
								type="text"
								bind:value={editedTerm.name}
								class="w-full text-2xl font-bold bg-transparent border-b-2 border-earthy-terracotta-300 dark:border-earthy-terracotta-700 focus:outline-none focus:border-earthy-terracotta-700 dark:focus:border-earthy-terracotta-500 text-gray-900 dark:text-gray-100 pb-2 mb-3"
								placeholder="Term name"
							/>
						{:else}
							<h2 class="text-2xl font-bold text-gray-900 dark:text-gray-100 mb-3">
								{selectedTerm.name}
							</h2>
						{/if}

						<!-- Definition -->
						{#if isEditing && editedTerm}
							<textarea
								bind:value={editedTerm.definition}
								rows="2"
								class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-lg focus:outline-none focus:ring-2 focus:ring-earthy-terracotta-600 focus:border-transparent dark:bg-gray-700 dark:text-gray-100 resize-none"
								placeholder="Clear, concise definition..."
							></textarea>
						{:else}
							<p class="text-sm text-gray-600 dark:text-gray-400 leading-relaxed">
								{selectedTerm.definition}
							</p>
						{/if}
					</div>

					<!-- Body Section -->
					<div class="p-6 space-y-5">
						<!-- Owners Section -->
						<div>
							<div class="flex items-center gap-2 mb-2">
								<Icon icon="material-symbols:person-outline" class="w-4 h-4 text-gray-500 dark:text-gray-400" />
								<h3 class="text-xs font-semibold text-gray-500 dark:text-gray-400 uppercase tracking-wider">
									Owners
								</h3>
							</div>
							{#if isEditing && editedTerm}
								<OwnerSelector
									bind:selectedOwners={editedTerm.owners}
									onChange={(owners) => {
										if (editedTerm) {
											editedTerm.owners = owners;
										}
									}}
								/>
							{:else}
								<OwnerSelector
									selectedOwners={selectedTerm.owners}
									onChange={() => {}}
									disabled={true}
								/>
							{/if}
						</div>
						<!-- Description (Markdown Body) -->
						{#if selectedTerm.description || (isEditing && editedTerm)}
							<div>
								<div class="flex items-center gap-2 mb-2">
									<Icon icon="material-symbols:description-outline" class="w-4 h-4 text-gray-500 dark:text-gray-400" />
									<h3 class="text-xs font-semibold text-gray-500 dark:text-gray-400 uppercase tracking-wider">
										Description
									</h3>
								</div>
								{#if isEditing && editedTerm}
									<RichTextEditor
										bind:value={editedTerm.description}
										placeholder="Add a detailed description with examples, context, or usage notes..."
									/>
								{:else if selectedTerm.description}
									<div class="prose prose-sm dark:prose-invert max-w-none">
										<MarkdownRenderer content={selectedTerm.description} />
									</div>
								{:else}
									<p class="text-sm text-gray-400 dark:text-gray-500 italic">No description provided</p>
								{/if}
							</div>
						{/if}

						<!-- Metadata -->
						<div>
							<div class="flex items-center gap-2 mb-2">
								<Icon icon="material-symbols:info-outline" class="w-4 h-4 text-gray-500 dark:text-gray-400" />
								<h3 class="text-xs font-semibold text-gray-500 dark:text-gray-400 uppercase tracking-wider">
									Details
								</h3>
							</div>
							<dl class="grid grid-cols-2 gap-4">
								<div>
									<dt class="text-xs text-gray-500 dark:text-gray-400">Created</dt>
									<dd class="text-sm text-gray-900 dark:text-gray-100 mt-0.5">
										{new Date(selectedTerm.created_at).toLocaleDateString('en-US', {
											year: 'numeric',
											month: 'short',
											day: 'numeric'
										})}
									</dd>
								</div>
								<div>
									<dt class="text-xs text-gray-500 dark:text-gray-400">Last Updated</dt>
									<dd class="text-sm text-gray-900 dark:text-gray-100 mt-0.5">
										{new Date(selectedTerm.updated_at).toLocaleDateString('en-US', {
											year: 'numeric',
											month: 'short',
											day: 'numeric'
										})}
									</dd>
								</div>
							</dl>
						</div>

						<!-- Actions -->
						{#if canManageGlossary}
							<div class="pt-5 border-t border-gray-200 dark:border-gray-700 flex items-center justify-between">
								{#if isEditing}
									<div class="flex gap-2">
										<Button
											click={saveEdit}
											icon="material-symbols:check"
											text="Save Changes"
											variant="filled"
										/>
										<Button
											click={cancelEdit}
											text="Cancel"
											variant="clear"
										/>
									</div>
								{:else}
									<button
										on:click={startEdit}
										class="inline-flex items-center gap-2 px-4 py-2 text-sm font-medium text-gray-700 dark:text-gray-300 border border-gray-300 dark:border-gray-600 rounded-lg hover:bg-gray-50 dark:hover:bg-gray-700 transition-colors"
									>
										<Icon icon="material-symbols:edit-outline" class="w-4 h-4" />
										Edit
									</button>
								{/if}

								<button
									on:click={() => (showDeleteConfirm = true)}
									class="inline-flex items-center gap-2 px-4 py-2 text-sm font-medium text-red-600 dark:text-red-400 hover:text-white hover:bg-red-600 dark:hover:bg-red-500 border border-red-300 dark:border-red-600 hover:border-transparent rounded-lg transition-colors"
								>
									<Icon icon="material-symbols:delete-outline" class="w-4 h-4" />
									Delete
								</button>
							</div>
						{/if}
					</div>
				</div>
			</div>
		{:else}
			<div class="flex items-center justify-center h-full">
				<div class="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-12 text-center max-w-md">
					<Icon icon="material-symbols:book-outline" class="mx-auto h-12 w-12 text-gray-400 dark:text-gray-500 mb-4" />
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
							class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:ring-earthy-terracotta-600 focus:border-earthy-terracotta-700 dark:bg-gray-700 dark:text-gray-100 disabled:opacity-50"
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
							class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:ring-earthy-terracotta-600 focus:border-earthy-terracotta-700 dark:bg-gray-700 dark:text-gray-100 disabled:opacity-50"
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
						<OwnerSelector
							bind:selectedOwners={newTermOwners}
							onChange={(owners) => {
								newTermOwners = owners;
							}}
							placeholder="Search and select owners..."
						/>
						<p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
							Leave empty to default to yourself
						</p>
					</div>

					<div class="flex justify-end gap-3 pt-4">
						<Button
							type="button"
							click={() => (showCreateModal = false)}
							disabled={isCreating}
							text="Cancel"
							variant="clear"
						/>
						<Button
							type="submit"
							disabled={isCreating}
							loading={isCreating}
							text={isCreating ? 'Creating...' : 'Create Term'}
							variant="filled"
						/>
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
