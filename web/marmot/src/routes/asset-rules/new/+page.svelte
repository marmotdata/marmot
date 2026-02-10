<script lang="ts">
	import { goto } from '$app/navigation';
	import IconifyIcon from '@iconify/svelte';
	import Button from '$components/ui/Button.svelte';
	import Stepper from '$components/ui/Stepper.svelte';
	import Step from '$components/ui/Step.svelte';
	import QueryBuilder from '$components/query/QueryBuilder.svelte';
	import ExternalLinks from '$components/shared/ExternalLinks.svelte';
	import { auth } from '$lib/stores/auth';
	import { createAssetRule, previewAssetRule } from '$lib/assetrules/api';
	import type { ExternalLink, CreateAssetRuleInput } from '$lib/assetrules/types';
	import { searchTerms } from '$lib/glossary/api';
	import type { GlossaryTerm } from '$lib/glossary/types';

	// Step state
	let currentStep = $state(1);
	let totalSteps = $state(0);

	// Step 1: Basic Information
	let name = $state('');
	let description = $state('');

	// Step 2: Enrichments
	let links = $state<ExternalLink[]>([]);
	let selectedTerms = $state<GlossaryTerm[]>([]);
	let termSearchQuery = $state('');
	let termSearchResults = $state<GlossaryTerm[]>([]);
	let isSearchingTerms = $state(false);
	let showTermSearch = $state(false);
	let termSearchTimeout: ReturnType<typeof setTimeout>;

	// Step 3: Query
	let queryExpression = $state('');

	let saving = $state(false);
	let error = $state<string | null>(null);
	let previewing = $state(false);
	let previewCount = $state<number | null>(null);

	let canManage = $derived(auth.hasPermission('assets', 'manage'));
	let canProceedFromStep1 = $derived(name.trim() !== '');

	function canNavigateToStep(stepNumber: number): boolean {
		if (stepNumber === 1) return true;
		return canProceedFromStep1;
	}

	function handleNextStep() {
		if (currentStep === 1 && !name.trim()) {
			error = 'Name is required';
			return;
		}
		if (currentStep < totalSteps) {
			error = null;
			currentStep++;
		}
	}

	// Term management
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

	// Preview
	async function handlePreview() {
		previewing = true;
		error = null;
		previewCount = null;
		try {
			const result = await previewAssetRule({
				rule_type: 'query',
				query_expression: queryExpression,
				limit: 100
			});
			previewCount = result.asset_count;
		} catch (e: any) {
			error = e.message || 'Failed to preview rule';
		} finally {
			previewing = false;
		}
	}

	// Save
	async function handleSave() {
		error = null;
		if (!name.trim()) {
			error = 'Name is required';
			currentStep = 1;
			return;
		}
		const validLinks = links.filter((l) => l.name.trim() && l.url.trim());
		if (validLinks.length === 0 && selectedTerms.length === 0) {
			error = 'At least one link or glossary term is required';
			currentStep = 2;
			return;
		}
		if (!queryExpression.trim()) {
			error = 'Query expression is required';
			return;
		}

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
				is_enabled: true
			};
			const created = await createAssetRule(input);
			goto(`/asset-rules/${created.id}`);
		} catch (e: any) {
			error = e.message || 'Failed to create asset rule';
		} finally {
			saving = false;
		}
	}
</script>

<svelte:head>
	<title>New Asset Rule - Marmot</title>
</svelte:head>

<div class="min-h-screen">
	<!-- Header -->
	<div class="border-b border-gray-200 dark:border-gray-700">
		<div class="container max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-6">
			<div class="flex items-center gap-4">
				<button
					onclick={() => goto('/asset-rules')}
					aria-label="Back to asset rules"
					class="p-2 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg transition-colors"
				>
					<IconifyIcon
						icon="material-symbols:arrow-back"
						class="h-6 w-6 text-gray-600 dark:text-gray-400"
					/>
				</button>
				<div>
					<h1 class="text-2xl font-bold text-gray-900 dark:text-gray-100">New Asset Rule</h1>
					<p class="text-sm text-gray-600 dark:text-gray-400 mt-1">
						Step {currentStep} of {totalSteps}
					</p>
				</div>
			</div>
		</div>
	</div>

	<!-- Step Indicator -->
	<div class="border-b border-gray-200 dark:border-gray-700">
		<div class="container max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-4">
			<Stepper
				{currentStep}
				bind:totalSteps
				onStepClick={(step) => (currentStep = step)}
				{canNavigateToStep}
			>
				<Step title="Basic Info" icon="material-symbols:info-outline" />
				<Step title="Enrichments" icon="material-symbols:link" />
				<Step title="Query" icon="material-symbols:filter-list" />
			</Stepper>
		</div>
	</div>

	<!-- Main Content -->
	<div class="container max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
		{#if error}
			<div
				class="mb-6 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800/50 rounded-lg p-4"
			>
				<div class="flex items-start">
					<IconifyIcon
						icon="material-symbols:error"
						class="h-5 w-5 text-red-400 mt-0.5 flex-shrink-0"
					/>
					<p class="ml-3 text-sm text-red-700 dark:text-red-300">{error}</p>
				</div>
			</div>
		{/if}

		<!-- Step 1: Basic Information -->
		{#if currentStep === 1}
			<div
				class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-6"
			>
				<h3 class="text-base font-semibold text-gray-900 dark:text-gray-100 mb-4 flex items-center">
					<IconifyIcon
						icon="material-symbols:info-outline"
						class="h-5 w-5 mr-2 text-earthy-terracotta-600"
					/>
					Basic Information
				</h3>

				<div class="space-y-5">
					<div>
						<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
							Name <span class="text-red-500">*</span>
						</label>
						<input
							type="text"
							bind:value={name}
							placeholder="e.g., AWS Console Links"
							class="w-full px-4 py-2.5 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-earthy-terracotta-600 focus:border-transparent transition-all"
							required
						/>
					</div>

					<div>
						<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
							Description
						</label>
						<textarea
							bind:value={description}
							rows="2"
							placeholder="Optional description of what this rule does"
							class="w-full px-4 py-2.5 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-earthy-terracotta-600 focus:border-transparent transition-all resize-none"
						></textarea>
					</div>
				</div>
			</div>
		{/if}

		<!-- Step 2: Enrichments -->
		{#if currentStep === 2}
			<div class="space-y-6">
				<!-- External Links -->
				<div
					class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-6"
				>
					<h3
						class="text-base font-semibold text-gray-900 dark:text-gray-100 mb-4 flex items-center"
					>
						<IconifyIcon
							icon="material-symbols:link"
							class="h-5 w-5 mr-2 text-earthy-terracotta-600"
						/>
						External Links
					</h3>

					<ExternalLinks bind:links canEdit={true} />
				</div>

				<!-- Glossary Terms -->
				<div
					class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-6"
				>
					<h3
						class="text-base font-semibold text-gray-900 dark:text-gray-100 mb-4 flex items-center"
					>
						<IconifyIcon
							icon="material-symbols:book"
							class="h-5 w-5 mr-2 text-earthy-terracotta-600"
						/>
						Glossary Terms
					</h3>

					{#if selectedTerms.length > 0}
						<div class="space-y-2 mb-4">
							{#each selectedTerms as term}
								<div
									class="flex items-center justify-between p-3 rounded-lg border border-gray-100 dark:border-gray-700 bg-gray-50 dark:bg-gray-700/30"
								>
									<div class="flex items-center gap-2 min-w-0">
										<IconifyIcon
											icon="material-symbols:book"
											class="w-4 h-4 text-earthy-terracotta-600 dark:text-earthy-terracotta-400 flex-shrink-0"
										/>
										<div class="min-w-0">
											<span class="text-sm font-medium text-gray-900 dark:text-gray-100">
												{term.name}
											</span>
											{#if term.definition}
												<p class="text-xs text-gray-500 dark:text-gray-400 truncate">
													{term.definition}
												</p>
											{/if}
										</div>
									</div>
									<button
										onclick={() => removeTerm(term.id)}
										aria-label="Remove term {term.name}"
										class="p-1 text-gray-400 hover:text-red-600 dark:hover:text-red-400 rounded flex-shrink-0"
									>
										<IconifyIcon icon="material-symbols:close" class="w-4 h-4" />
									</button>
								</div>
							{/each}
						</div>
					{/if}

					<div class="relative">
						<div class="relative">
							<IconifyIcon
								icon="material-symbols:search"
								class="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-400"
							/>
							<input
								type="text"
								value={termSearchQuery}
								oninput={handleTermSearch}
								onfocus={() => (showTermSearch = true)}
								placeholder="Search glossary terms to add..."
								class="w-full pl-9 pr-4 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-earthy-terracotta-600 focus:border-transparent"
							/>
						</div>
						{#if showTermSearch && (termSearchResults.length > 0 || isSearchingTerms)}
							<div
								class="absolute z-10 w-full mt-1 bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg shadow-lg max-h-48 overflow-y-auto"
							>
								{#if isSearchingTerms}
									<div class="px-4 py-3 text-sm text-gray-500 dark:text-gray-400">Searching...</div>
								{:else}
									{#each termSearchResults as term}
										<button
											onclick={() => addTerm(term)}
											class="w-full text-left px-4 py-2.5 hover:bg-gray-50 dark:hover:bg-gray-700/50 transition-colors border-b border-gray-100 dark:border-gray-700 last:border-b-0"
										>
											<div class="text-sm font-medium text-gray-900 dark:text-gray-100">
												{term.name}
											</div>
											{#if term.definition}
												<div class="text-xs text-gray-500 dark:text-gray-400 truncate">
													{term.definition}
												</div>
											{/if}
										</button>
									{/each}
								{/if}
							</div>
						{/if}
					</div>

					<p class="text-xs text-gray-500 dark:text-gray-400 mt-3">
						Add at least one link or glossary term to apply to matched assets.
					</p>
				</div>
			</div>
		{/if}

		<!-- Step 3: Query -->
		{#if currentStep === 3}
			<div
				class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-6"
			>
				<h3 class="text-base font-semibold text-gray-900 dark:text-gray-100 mb-4 flex items-center">
					<IconifyIcon
						icon="material-symbols:filter-list"
						class="h-5 w-5 mr-2 text-earthy-terracotta-600"
					/>
					Query
				</h3>

				<p class="text-sm text-gray-600 dark:text-gray-400 mb-4">
					Define a query to match assets. Enrichments will be automatically applied to all matching
					assets.
				</p>

				<QueryBuilder
					query={queryExpression}
					onQueryChange={(q) => (queryExpression = q)}
					initiallyExpanded={true}
					showRunButton={true}
					runButtonText={previewing ? 'Previewing...' : 'Preview'}
					runButtonIcon={previewing ? 'mdi:loading' : 'material-symbols:visibility'}
					onRunClick={() => handlePreview()}
				/>

				{#if previewCount !== null}
					<div
						class="mt-4 p-3 bg-green-50 dark:bg-green-900/20 border border-green-200 dark:border-green-800 rounded-lg text-sm text-green-800 dark:text-green-200 flex items-center gap-2"
					>
						<IconifyIcon icon="material-symbols:check-circle" class="w-4 h-4" />
						This rule matches {previewCount} asset{previewCount !== 1 ? 's' : ''}
					</div>
				{/if}
			</div>
		{/if}

		<!-- Footer Actions -->
		<div
			class="mt-8 flex items-center justify-between border-t border-gray-200 dark:border-gray-700 pt-6"
		>
			<div>
				{#if currentStep > 1}
					<Button
						variant="clear"
						click={() => currentStep--}
						icon="material-symbols:arrow-back"
						text="Previous"
					/>
				{:else}
					<Button variant="clear" click={() => goto('/asset-rules')} text="Cancel" />
				{/if}
			</div>
			<div>
				{#if currentStep < totalSteps}
					<Button
						variant="filled"
						click={handleNextStep}
						text="Next"
						icon="material-symbols:arrow-forward"
						disabled={currentStep === 1 && !canProceedFromStep1}
					/>
				{:else}
					<Button
						variant="filled"
						click={handleSave}
						text={saving ? 'Creating...' : 'Create Asset Rule'}
						disabled={saving}
						icon="material-symbols:check"
					/>
				{/if}
			</div>
		</div>
	</div>
</div>
