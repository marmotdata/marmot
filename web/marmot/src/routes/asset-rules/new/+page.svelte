<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import IconifyIcon from '@iconify/svelte';
	import QueryBuilder from '$components/query/QueryBuilder.svelte';
	import ExternalLinks from '$components/shared/ExternalLinks.svelte';
	import RulePreviewPanel from '$components/assetrule/RulePreviewPanel.svelte';
	import ConfirmModal from '$components/ui/ConfirmModal.svelte';
	import { createAssetRule } from '$lib/assetrules/api';
	import type { ExternalLink, CreateAssetRuleInput } from '$lib/assetrules/types';
	import { searchTerms } from '$lib/glossary/api';
	import type { GlossaryTerm } from '$lib/glossary/types';
	import { m } from '$lib/paraglide/messages';
	import { useUnsavedChangesGuard } from '$lib/utils/unsavedChanges.svelte';

	// Basics
	let name = $state('');
	let description = $state('');
	let isEnabled = $state(true);

	// Matching query
	let queryExpression = $state('');

	// Enrichments
	let links = $state<ExternalLink[]>([]);
	let selectedTerms = $state<GlossaryTerm[]>([]);
	let termSearchQuery = $state('');
	let termSearchResults = $state<GlossaryTerm[]>([]);
	let isSearchingTerms = $state(false);
	let showTermSearch = $state(false);
	let termSearchTimeout: ReturnType<typeof setTimeout>;

	let saving = $state(false);
	let error = $state<string | null>(null);
	// Field errors stay hidden until the first submit so the form does not shout at people before they have typed anything
	let submitted = $state(false);
	let created = false;

	let validLinks = $derived(links.filter((l) => l.name.trim() && l.url.trim()));
	let nameError = $derived(name.trim() ? null : m.assetrules_validation_name_required());
	let queryError = $derived(
		queryExpression.trim() ? null : m.assetrules_validation_query_required()
	);
	let enrichmentError = $derived(
		validLinks.length > 0 || selectedTerms.length > 0
			? null
			: m.assetrules_validation_enrichment_required()
	);
	let validationErrors = $derived(
		[nameError, queryError, enrichmentError].filter((e) => e !== null)
	);

	let isDirty = $derived(
		name.trim() !== '' ||
			description.trim() !== '' ||
			!isEnabled ||
			queryExpression.trim() !== '' ||
			links.some((l) => l.name.trim() || l.url.trim()) ||
			selectedTerms.length > 0
	);

	const unsavedGuard = useUnsavedChangesGuard(() => !created && !saving && isDirty);

	function handleTermSearch(e: Event) {
		termSearchQuery = (e.target as HTMLInputElement).value;
		clearTimeout(termSearchTimeout);
		if (!termSearchQuery.trim()) {
			termSearchResults = [];
			return;
		}
		termSearchTimeout = setTimeout(async () => {
			isSearchingTerms = true;
			try {
				const result = await searchTerms(termSearchQuery, null, 0, 10);
				termSearchResults = (result.terms || []).filter(
					(t) => !selectedTerms.some((s) => s.id === t.id)
				);
			} catch {
				termSearchResults = [];
			} finally {
				isSearchingTerms = false;
			}
		}, 300);
	}

	function addTerm(term: GlossaryTerm) {
		if (!selectedTerms.some((t) => t.id === term.id)) {
			selectedTerms = [...selectedTerms, term];
		}
		termSearchQuery = '';
		termSearchResults = [];
		showTermSearch = false;
	}

	function removeTerm(termId: string) {
		selectedTerms = selectedTerms.filter((t) => t.id !== termId);
	}

	async function handleSave() {
		error = null;
		submitted = true;
		if (validationErrors.length > 0) return;

		saving = true;
		try {
			const input: CreateAssetRuleInput = {
				name: name.trim(),
				description: description.trim() || undefined,
				links: validLinks.length > 0 ? validLinks : undefined,
				term_ids: selectedTerms.length > 0 ? selectedTerms.map((t) => t.id) : undefined,
				rule_type: 'query',
				query_expression: queryExpression.trim(),
				priority: 0,
				is_enabled: isEnabled
			};
			const rule = await createAssetRule(input);
			created = true;
			goto(resolve('/asset-rules/[id]', { id: rule.id }));
		} catch (e) {
			error = e instanceof Error ? e.message : m.assetrules_create_error();
		} finally {
			saving = false;
		}
	}

	const inputClass =
		'w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm text-gray-900 transition-all focus:border-transparent focus:ring-2 focus:ring-earthy-terracotta-600 dark:border-gray-600 dark:bg-gray-700 dark:text-gray-100';
	const cardClass =
		'rounded-xl border border-gray-200 bg-white p-6 dark:border-gray-700 dark:bg-gray-800';
</script>

<svelte:head>
	<title>{m.assetrules_new_page_title()}</title>
</svelte:head>

<div class="flex min-h-full flex-col">
	<div class="w-full flex-1 px-4 py-6 sm:px-6 lg:px-8">
		<header class="mb-6">
			<a
				href={resolve('/asset-rules')}
				class="inline-flex items-center gap-1 text-sm text-gray-500 hover:text-gray-800 dark:text-gray-400 dark:hover:text-gray-200"
			>
				<IconifyIcon icon="material-symbols:arrow-back" class="h-4 w-4" />
				{m.assetrules_back_to_rules()}
			</a>
			<h1 class="mt-3 text-2xl font-semibold tracking-tight text-gray-900 dark:text-gray-100">
				{m.assetrules_new_rule_heading()}
			</h1>
			<p class="mt-1 text-sm text-gray-600 dark:text-gray-400">
				{m.assetrules_new_description()}
			</p>
		</header>

		<div class="grid grid-cols-1 gap-6 lg:grid-cols-3">
			<div class="space-y-6 lg:col-span-2">
				<!-- Basics -->
				<section class={cardClass} aria-labelledby="rule-basics-heading">
					<h2
						id="rule-basics-heading"
						class="text-sm font-semibold text-gray-900 dark:text-gray-100"
					>
						{m.assetrules_section_basics_heading()}
					</h2>
					<p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
						{m.assetrules_section_basics_hint()}
					</p>

					<div class="mt-5 space-y-5">
						<div>
							<label
								for="rule-name"
								class="mb-1.5 block text-sm font-medium text-gray-700 dark:text-gray-300"
							>
								{m.common_name()} <span class="text-red-500">*</span>
							</label>
							<input
								id="rule-name"
								type="text"
								bind:value={name}
								placeholder={m.assetrules_name_placeholder()}
								class={inputClass}
								aria-invalid={submitted && nameError !== null}
								aria-describedby={submitted && nameError ? 'rule-name-error' : undefined}
								required
							/>
							{#if submitted && nameError}
								<p id="rule-name-error" class="mt-1.5 text-xs text-red-600 dark:text-red-400">
									{nameError}
								</p>
							{/if}
						</div>

						<div>
							<label
								for="rule-description"
								class="mb-1.5 block text-sm font-medium text-gray-700 dark:text-gray-300"
							>
								{m.common_description()}
							</label>
							<textarea
								id="rule-description"
								bind:value={description}
								rows="2"
								placeholder={m.assetrules_description_placeholder()}
								class="{inputClass} resize-y"
							></textarea>
						</div>

						<div class="flex items-start justify-between gap-4">
							<div>
								<span
									id="rule-enabled-label"
									class="block text-sm font-medium text-gray-700 dark:text-gray-300"
								>
									{m.common_enabled()}
								</span>
								<p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
									{m.assetrules_enabled_hint()}
								</p>
							</div>
							<button
								type="button"
								role="switch"
								aria-checked={isEnabled}
								aria-labelledby="rule-enabled-label"
								onclick={() => (isEnabled = !isEnabled)}
								class="relative mt-0.5 inline-flex h-5 w-9 shrink-0 items-center rounded-full transition-colors {isEnabled
									? 'bg-earthy-terracotta-600'
									: 'bg-gray-300 dark:bg-gray-600'}"
							>
								<span
									class="inline-block h-4 w-4 transform rounded-full bg-white transition-transform {isEnabled
										? 'translate-x-4'
										: 'translate-x-0.5'}"
								></span>
							</button>
						</div>
					</div>
				</section>

				<!-- Matching query -->
				<section class={cardClass} aria-labelledby="rule-match-heading">
					<h2
						id="rule-match-heading"
						class="text-sm font-semibold text-gray-900 dark:text-gray-100"
					>
						{m.assetrules_section_match_heading()}
					</h2>
					<p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
						{m.assetrules_section_match_hint()}
					</p>

					<div class="mt-5">
						<QueryBuilder
							query={queryExpression}
							onQueryChange={(q) => (queryExpression = q)}
							initiallyExpanded={true}
							showRunButton={false}
						/>
					</div>
					{#if submitted && queryError}
						<p class="mt-2 text-xs text-red-600 dark:text-red-400">{queryError}</p>
					{/if}
				</section>

				<!-- Enrichments -->
				<section class={cardClass} aria-labelledby="rule-apply-heading">
					<h2
						id="rule-apply-heading"
						class="text-sm font-semibold text-gray-900 dark:text-gray-100"
					>
						{m.assetrules_section_apply_heading()}
					</h2>
					<p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
						{m.assetrules_section_apply_hint()}
					</p>
					{#if submitted && enrichmentError}
						<p class="mt-2 text-xs text-red-600 dark:text-red-400">{enrichmentError}</p>
					{/if}

					<div class="mt-5 space-y-6">
						<div>
							<h3
								class="mb-3 flex items-center gap-1.5 text-sm font-medium text-gray-700 dark:text-gray-300"
							>
								<IconifyIcon icon="material-symbols:link" class="h-4 w-4 text-gray-400" />
								{m.assetrules_external_links_heading()}
							</h3>
							<ExternalLinks bind:links canEdit={true} />
						</div>

						<div class="border-t border-gray-100 pt-6 dark:border-gray-700">
							<h3
								class="mb-3 flex items-center gap-1.5 text-sm font-medium text-gray-700 dark:text-gray-300"
							>
								<IconifyIcon icon="material-symbols:book" class="h-4 w-4 text-gray-400" />
								{m.assetrules_glossary_terms_heading()}
							</h3>

							{#if selectedTerms.length > 0}
								<div class="mb-4 space-y-2">
									{#each selectedTerms as term (term.id)}
										<div
											class="flex items-center justify-between rounded-lg border border-gray-100 bg-gray-50 p-3 dark:border-gray-700 dark:bg-gray-700/30"
										>
											<div class="flex min-w-0 items-center gap-2">
												<IconifyIcon
													icon="material-symbols:book"
													class="h-4 w-4 flex-shrink-0 text-earthy-terracotta-600 dark:text-earthy-terracotta-400"
												/>
												<div class="min-w-0">
													<span class="text-sm font-medium text-gray-900 dark:text-gray-100">
														{term.name}
													</span>
													{#if term.definition}
														<p class="truncate text-xs text-gray-500 dark:text-gray-400">
															{term.definition}
														</p>
													{/if}
												</div>
											</div>
											<button
												type="button"
												onclick={() => removeTerm(term.id)}
												aria-label={m.assetrules_remove_term_aria({ name: term.name })}
												class="flex-shrink-0 rounded p-1 text-gray-400 hover:text-red-600 dark:hover:text-red-400"
											>
												<IconifyIcon icon="material-symbols:close" class="h-4 w-4" />
											</button>
										</div>
									{/each}
								</div>
							{/if}

							<div class="relative">
								<div class="relative">
									<IconifyIcon
										icon="material-symbols:search"
										class="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-gray-400"
									/>
									<input
										type="text"
										value={termSearchQuery}
										oninput={handleTermSearch}
										onfocus={() => (showTermSearch = true)}
										placeholder={m.assetrules_term_search_placeholder()}
										aria-label={m.assetrules_term_search_placeholder()}
										class="{inputClass} pl-9"
									/>
								</div>
								{#if showTermSearch && (termSearchResults.length > 0 || isSearchingTerms)}
									<div
										class="absolute z-10 mt-1 max-h-48 w-full overflow-y-auto rounded-lg border border-gray-200 bg-white shadow-lg dark:border-gray-700 dark:bg-gray-800"
									>
										{#if isSearchingTerms}
											<div class="px-4 py-3 text-sm text-gray-500 dark:text-gray-400">
												{m.assetrules_searching()}
											</div>
										{:else}
											{#each termSearchResults as term (term.id)}
												<button
													type="button"
													onclick={() => addTerm(term)}
													class="w-full border-b border-gray-100 px-4 py-2.5 text-left transition-colors last:border-b-0 hover:bg-gray-50 dark:border-gray-700 dark:hover:bg-gray-700/50"
												>
													<div class="text-sm font-medium text-gray-900 dark:text-gray-100">
														{term.name}
													</div>
													{#if term.definition}
														<div class="truncate text-xs text-gray-500 dark:text-gray-400">
															{term.definition}
														</div>
													{/if}
												</button>
											{/each}
										{/if}
									</div>
								{/if}
							</div>
						</div>
					</div>
				</section>
			</div>

			<aside class="lg:sticky lg:top-6 lg:self-start">
				<RulePreviewPanel query={queryExpression} />
			</aside>
		</div>
	</div>

	<div
		class="sticky bottom-0 z-20 border-t border-gray-200 bg-white/90 px-4 py-3 backdrop-blur dark:border-gray-700 dark:bg-gray-900/90 sm:px-6 lg:px-8"
	>
		<div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
			<div class="min-w-0 text-sm" aria-live="polite">
				{#if error}
					<p class="flex items-start gap-1.5 text-red-600 dark:text-red-400">
						<IconifyIcon icon="material-symbols:error" class="mt-0.5 h-4 w-4 flex-shrink-0" />
						<span>{error}</span>
					</p>
				{:else if submitted && validationErrors.length > 0}
					<ul class="space-y-0.5 text-red-600 dark:text-red-400">
						{#each validationErrors as message (message)}
							<li class="flex items-start gap-1.5">
								<IconifyIcon icon="material-symbols:error" class="mt-0.5 h-4 w-4 flex-shrink-0" />
								<span>{message}</span>
							</li>
						{/each}
					</ul>
				{/if}
			</div>
			<div class="flex shrink-0 items-center justify-end gap-2">
				<a
					href={resolve('/asset-rules')}
					class="rounded-lg px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-gray-800"
				>
					{m.common_cancel()}
				</a>
				<button
					type="button"
					onclick={handleSave}
					disabled={saving}
					class="rounded-lg bg-earthy-terracotta-600 px-4 py-2 text-sm font-semibold text-white hover:bg-earthy-terracotta-500 disabled:opacity-50"
				>
					{saving ? m.assetrules_creating() : m.assetrules_create_rule_button()}
				</button>
			</div>
		</div>
	</div>
</div>

<ConfirmModal
	show={unsavedGuard.showConfirm}
	title={m.common_discard_prompt()}
	message={m.assetrules_unsaved_changes_confirm()}
	confirmText={m.common_discard()}
	cancelText={m.common_keep_editing()}
	variant="warning"
	onConfirm={unsavedGuard.confirmDiscard}
	onCancel={unsavedGuard.cancelDiscard}
/>
