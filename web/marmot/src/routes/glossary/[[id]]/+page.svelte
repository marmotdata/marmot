<script lang="ts">
	import { untrack } from 'svelte';
	import { SvelteURLSearchParams } from 'svelte/reactivity';
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { fetchApi } from '$lib/api';
	import { toasts, parseApiError, isLimitExceeded } from '$lib/stores/toast';
	import type { GlossaryTerm, TermsListResponse } from '$lib/glossary/types';
	import QueryInput from '$components/query/QueryInput.svelte';
	import Modal from '$components/ui/Modal.svelte';
	import GlossaryTermView from '$components/glossary/GlossaryTermView.svelte';
	import Icon from '@iconify/svelte';
	import { auth } from '$lib/stores/auth';
	import { m } from '$lib/paraglide/messages';

	let terms = $state<GlossaryTerm[]>([]);
	let totalTerms = $state(0);
	let isLoading = $state(true);
	let error = $state<string | null>(null);

	let searchQuery = $state(page.url.searchParams.get('q') || '');
	let searchTimeout: ReturnType<typeof setTimeout>;

	let selectedTerm = $state<GlossaryTerm | null>(null);
	let showCreateModal = $state(false);
	let showDeleteConfirm = $state(false);

	let newTermName = $state('');
	let newTermDefinition = $state('');
	let isCreating = $state(false);
	let createError = $state('');

	const canManageGlossary = auth.hasPermission('glossary', 'manage');
	let didAutoSelect = false;

	$effect(() => {
		const query = page.url.searchParams.get('q') || '';
		if (query !== searchQuery) {
			searchQuery = query;
		}

		const termId = page.params.id;
		if (termId && (!selectedTerm || selectedTerm.id !== termId)) {
			const term = terms.find((t) => t.id === termId);
			if (term) {
				selectedTerm = term;
			}
			// Term not in the loaded list: the effect below fetches it. Calling
			// loadTermById from here would assign selectedTerm and re-trigger this effect.
		} else if (!termId && !selectedTerm && !didAutoSelect && terms.length > 0) {
			// Latch the auto-select to a single fire so the assignment to
			// selectedTerm via selectTerm() doesn't loop this effect.
			didAutoSelect = true;
			selectTerm(terms[0]);
		}
	});

	async function fetchTerms() {
		isLoading = true;
		error = null;

		try {
			const queryParams = new SvelteURLSearchParams({
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
				throw new Error(errorData.error || m.glossary_fetch_error());
			}

			const data: TermsListResponse = await response.json();
			const processedTerms = (data.terms || []).map((term) => ({
				...term,
				tags: term.tags || [],
				metadata: term.metadata || {}
			}));
			terms = processedTerms;
			totalTerms = data.total || 0;
		} catch (err) {
			error = err instanceof Error ? err.message : m.glossary_fetch_error();
		} finally {
			isLoading = false;
		}
	}

	function updateSearchParam(query: string) {
		const params = new SvelteURLSearchParams(page.url.searchParams);
		if (query) {
			params.set('q', query);
		} else {
			params.delete('q');
		}
		goto(resolve(`/glossary/[[id]]?${params}`, { id: page.params.id }), {
			replaceState: true,
			noScroll: true,
			keepFocus: true
		});
	}

	function handleSearch(query: string) {
		clearTimeout(searchTimeout);
		searchQuery = query;

		searchTimeout = setTimeout(() => updateSearchParam(query), 300);
	}

	function handleSearchSubmit() {
		clearTimeout(searchTimeout);
		updateSearchParam(searchQuery);
	}

	function selectTerm(term: GlossaryTerm) {
		selectedTerm = term;

		// Update URL with selected term using path parameter
		goto(resolve(`/glossary/[[id]]?${page.url.searchParams}`, { id: term.id }), {
			replaceState: true,
			noScroll: true,
			keepFocus: true
		});
	}

	async function loadTermById(termId: string) {
		try {
			const response = await fetchApi(`/glossary/${termId}`);
			if (response.ok) {
				const term: GlossaryTerm = await response.json();
				// Initialize tags and metadata if they don't exist
				if (!term.tags) term.tags = [];
				if (!term.metadata) term.metadata = {};
				selectedTerm = term;
			}
		} catch (err) {
			console.error('Failed to load term:', err);
		}
	}

	// Starter terms on the empty glossary open the create blade with the name already filled in
	function startTerm(name: string) {
		handleNewTerm();
		newTermName = name;
	}

	function handleNewTerm() {
		newTermName = '';
		newTermDefinition = '';
		createError = '';
		showCreateModal = true;
	}

	async function createTerm() {
		if (!newTermName || !newTermDefinition) {
			createError = m.glossary_error_name_definition_required();
			return;
		}

		isCreating = true;
		createError = '';

		try {
			const response = await fetchApi('/glossary/', {
				method: 'POST',
				body: JSON.stringify({ name: newTermName, definition: newTermDefinition })
			});

			if (!response.ok) {
				const info = await parseApiError(response);
				if (isLimitExceeded(info)) toasts.warning(info.message);
				throw new Error(info.message);
			}

			// Land on the new term so its owners, description and metadata can be filled in straight away
			const created: GlossaryTerm = await response.json();
			created.tags = created.tags ?? [];
			created.metadata = created.metadata ?? {};
			showCreateModal = false;
			await fetchTerms();
			selectTerm(created);
		} catch (err) {
			createError = err instanceof Error ? err.message : m.glossary_create_error();
		} finally {
			isCreating = false;
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
				throw new Error(errorData.error || m.glossary_delete_error());
			}

			showDeleteConfirm = false;
			selectedTerm = null;
			fetchTerms();
		} catch (err) {
			error = err instanceof Error ? err.message : m.glossary_delete_error();
		}
	}

	let lastFetchedTermId: string | null | undefined = undefined;

	// Refetch whenever the page URL changes (search params, route id, etc). searchQuery and
	// selectedTerm are read untracked so typing in the search box or selecting a term don't
	// themselves retrigger this effect -- only navigation should.
	$effect(() => {
		if (!page.url) return;
		const termId = page.params?.id;

		untrack(() => {
			fetchTerms();

			// Lazy-load any term whose id is in the URL but not yet known.
			if (termId && termId !== lastFetchedTermId && termId !== selectedTerm?.id) {
				lastFetchedTermId = termId;
				loadTermById(termId);
			}
		});
	});
</script>

<svelte:head>
	<title
		>{selectedTerm?.name
			? m.glossary_term_page_title({ name: selectedTerm.name })
			: m.glossary_page_title()}</title
	>
</svelte:head>

<div class="flex h-[calc(100dvh-4rem)] min-h-0">
	<aside
		class="flex w-80 flex-shrink-0 flex-col border-r border-gray-200 bg-gray-50/60 dark:border-gray-800 dark:bg-gray-900/40"
	>
		<div class="px-4 pb-3 pt-5">
			<div class="flex items-center justify-between gap-2">
				<h1 class="text-xl font-semibold tracking-tight text-gray-900 dark:text-gray-100">
					{m.glossary_heading()}
				</h1>
				{#if canManageGlossary}
					<button
						type="button"
						onclick={handleNewTerm}
						class="inline-flex items-center gap-1 rounded-lg bg-earthy-terracotta-600 px-2.5 py-1.5 text-xs font-semibold text-white transition hover:bg-earthy-terracotta-500"
					>
						<Icon icon="material-symbols:add" class="h-4 w-4" />
						{m.glossary_new_term_button()}
					</button>
				{/if}
			</div>
			<p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
				{m.glossary_term_count({ count: totalTerms })}
			</p>
			<div class="mt-3">
				<QueryInput
					value={searchQuery}
					onQueryChange={handleSearch}
					onSubmit={handleSearchSubmit}
					placeholder={m.glossary_search_placeholder()}
				/>
			</div>
		</div>

		<div class="min-h-0 flex-1 overflow-y-auto px-2 pb-3">
			{#if isLoading}
				<div class="space-y-1.5 px-1.5 pt-1">
					{#each Array(6) as _, i (i)}
						<div class="h-11 animate-pulse rounded-md bg-gray-100 dark:bg-gray-800"></div>
					{/each}
				</div>
			{:else if error}
				<p class="px-2 pt-2 text-sm text-red-600 dark:text-red-400">{error}</p>
			{:else if terms.length === 0}
				<p class="px-2 pt-2 text-sm text-gray-500 dark:text-gray-400">
					{searchQuery ? m.glossary_no_terms_found() : m.glossary_no_terms_yet()}
				</p>
			{:else}
				<ul class="space-y-0.5">
					{#each terms as term (term.id)}
						<li>
							<button
								type="button"
								onclick={() => selectTerm(term)}
								aria-current={selectedTerm?.id === term.id ? 'true' : undefined}
								class="w-full rounded-md px-2.5 py-2 text-left transition {selectedTerm?.id ===
								term.id
									? 'bg-earthy-terracotta-50 dark:bg-earthy-terracotta-900/20'
									: 'hover:bg-gray-100 dark:hover:bg-gray-800'}"
							>
								<span
									class="block truncate text-sm {selectedTerm?.id === term.id
										? 'font-semibold text-earthy-terracotta-800 dark:text-earthy-terracotta-200'
										: 'font-medium text-gray-900 dark:text-gray-100'}">{term.name}</span
								>
								<span class="mt-0.5 block truncate text-xs text-gray-500 dark:text-gray-400"
									>{term.definition}</span
								>
							</button>
						</li>
					{/each}
				</ul>
			{/if}
		</div>
	</aside>

	<!-- Term detail -->
	<main class="min-w-0 flex-1 overflow-y-auto px-6 py-6 lg:px-8">
		<div>
			{#if selectedTerm}
				<GlossaryTermView
					term={selectedTerm}
					canManage={canManageGlossary}
					onUpdated={(updated) => {
						selectedTerm = updated;
						terms = terms.map((t) => (t.id === updated.id ? updated : t));
					}}
					onDelete={() => (showDeleteConfirm = true)}
				/>
			{:else if terms.length === 0 && !searchQuery}
				<div class="mx-auto max-w-lg py-16 text-center">
					<span
						class="mx-auto flex h-11 w-11 items-center justify-center rounded-xl bg-gray-100 text-gray-500 dark:bg-gray-800 dark:text-gray-400"
					>
						<Icon icon="material-symbols:menu-book-outline" class="h-6 w-6" />
					</span>
					<h2 class="mt-4 text-lg font-semibold text-gray-900 dark:text-gray-100">
						{m.glossary_empty_heading()}
					</h2>
					<p class="mx-auto mt-2 max-w-md text-sm text-gray-600 dark:text-gray-400">
						{m.glossary_empty_description()}
					</p>
					{#if canManageGlossary}
						<p
							class="mt-6 text-xs font-semibold uppercase tracking-wider text-gray-500 dark:text-gray-400"
						>
							{m.glossary_starter_heading()}
						</p>
						<div class="mt-2 flex flex-wrap justify-center gap-2">
							{#each [m.glossary_starter_customer(), m.glossary_starter_revenue(), m.glossary_starter_churn(), m.glossary_starter_order()] as starter (starter)}
								<button
									type="button"
									onclick={() => startTerm(starter)}
									class="inline-flex items-center gap-1.5 rounded-full border border-gray-200 bg-white px-3 py-1.5 text-sm text-gray-700 transition hover:border-earthy-terracotta-300 hover:text-earthy-terracotta-800 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-200"
								>
									<Icon icon="material-symbols:add" class="h-4 w-4 text-gray-400" />
									{starter}
								</button>
							{/each}
						</div>
						<button
							type="button"
							onclick={handleNewTerm}
							class="mt-6 inline-flex items-center gap-2 rounded-lg bg-earthy-terracotta-600 px-4 py-2 text-sm font-semibold text-white transition hover:bg-earthy-terracotta-500"
						>
							<Icon icon="material-symbols:add" class="h-5 w-5" />
							{m.glossary_create_first_button()}
						</button>
					{/if}
				</div>
			{:else}
				<div class="flex h-full items-center justify-center">
					<div class="max-w-sm text-center">
						<Icon
							icon="material-symbols:book-outline"
							class="mx-auto h-12 w-12 text-gray-400 dark:text-gray-500 mb-4"
						/>
						<h3 class="text-lg font-medium text-gray-900 dark:text-gray-100 mb-2">
							{m.glossary_no_term_selected_heading()}
						</h3>
						<p class="text-sm text-gray-500 dark:text-gray-400">
							{m.glossary_no_term_selected_hint()}
						</p>
					</div>
				</div>
			{/if}
		</div>
	</main>
</div>

<!-- Create Modal -->
{#if showCreateModal}
	<Modal
		title={m.glossary_create_modal_heading()}
		subtitle={m.glossary_create_modal_subtitle()}
		onClose={() => !isCreating && (showCreateModal = false)}
		dirty={!!(newTermName || newTermDefinition)}
	>
		<form
			id="create-term-form"
			onsubmit={(e) => {
				e.preventDefault();
				createTerm();
			}}
			class="space-y-4"
		>
			{#if createError}
				<p
					class="rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700 dark:border-red-800 dark:bg-red-900/20 dark:text-red-200"
				>
					{createError}
				</p>
			{/if}
			<div>
				<label
					for="term-name"
					class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300"
					>{m.common_name()}</label
				>
				<!-- svelte-ignore a11y_autofocus -->
				<input
					id="term-name"
					type="text"
					bind:value={newTermName}
					disabled={isCreating}
					placeholder={m.glossary_name_placeholder()}
					autofocus
					required
					class="w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm text-gray-900 focus:border-earthy-terracotta-500 focus:ring-2 focus:ring-earthy-terracotta-500/30 dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100"
				/>
			</div>
			<div>
				<label
					for="term-definition"
					class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300"
					>{m.glossary_definition_label()}</label
				>
				<textarea
					id="term-definition"
					bind:value={newTermDefinition}
					disabled={isCreating}
					placeholder={m.glossary_definition_placeholder()}
					rows="3"
					required
					onkeydown={(e) => {
						if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) createTerm();
					}}
					class="w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm text-gray-900 focus:border-earthy-terracotta-500 focus:ring-2 focus:ring-earthy-terracotta-500/30 dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100"
				></textarea>
				<p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{m.glossary_create_hint()}</p>
			</div>
		</form>
		{#snippet footer(close)}
			<button
				type="button"
				onclick={close}
				disabled={isCreating}
				class="rounded-lg px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-100 disabled:opacity-50 dark:text-gray-200 dark:hover:bg-gray-700"
				>{m.common_cancel()}</button
			>
			<button
				type="submit"
				form="create-term-form"
				disabled={isCreating}
				class="inline-flex items-center gap-2 rounded-lg bg-earthy-terracotta-600 px-4 py-2 text-sm font-semibold text-white transition hover:bg-earthy-terracotta-500 disabled:opacity-50"
			>
				{isCreating ? m.glossary_creating() : m.glossary_create_term_button()}
			</button>
		{/snippet}
	</Modal>
{/if}

<!-- Delete Confirm Modal -->
{#if showDeleteConfirm}
	<div class="fixed inset-0 z-50 overflow-y-auto">
		<div class="flex items-center justify-center min-h-screen px-4">
			<div
				class="fixed inset-0 bg-black/50 dark:bg-black/70 backdrop-blur-sm transition-opacity"
				onclick={() => (showDeleteConfirm = false)}
				onkeypress={(e) => e.key === 'Enter' && (showDeleteConfirm = false)}
				role="button"
				tabindex="0"
			></div>

			<div
				class="relative z-10 w-full max-w-lg rounded-xl border border-gray-200 bg-white p-6 shadow-2xl dark:border-gray-700 dark:bg-gray-800"
			>
				<h3 class="text-lg font-medium text-gray-900 dark:text-gray-100 mb-2">
					{m.glossary_delete_modal_heading()}
				</h3>
				<p class="text-sm text-gray-500 dark:text-gray-400 mb-4">
					{m.glossary_delete_confirm({ name: selectedTerm?.name ?? '' })}
				</p>
				<div class="flex justify-end gap-3">
					<button
						onclick={() => (showDeleteConfirm = false)}
						class="px-4 py-2 border border-gray-300 dark:border-gray-600 rounded-md text-sm font-medium text-gray-700 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-700"
					>
						{m.common_cancel()}
					</button>
					<button
						onclick={deleteTerm}
						class="px-4 py-2 rounded-md text-sm font-medium text-white bg-red-600 hover:bg-red-700 dark:bg-red-500 dark:hover:bg-red-600"
					>
						{m.common_delete()}
					</button>
				</div>
			</div>
		</div>
	</div>
{/if}
