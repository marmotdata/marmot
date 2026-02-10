<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import IconifyIcon from '@iconify/svelte';
	import Button from '$components/ui/Button.svelte';
	import ConfirmModal from '$components/ui/ConfirmModal.svelte';
	import Tabs, { type Tab } from '$components/ui/Tabs.svelte';
	import QueryBuilder from '$components/query/QueryBuilder.svelte';
	import ExternalLinks from '$components/shared/ExternalLinks.svelte';
	import AssetIcon from '$components/ui/Icon.svelte';
	import { auth } from '$lib/stores/auth';
	import { fetchApi } from '$lib/api';
	import {
		getAssetRule,
		updateAssetRule,
		deleteAssetRule,
		getAssetRuleAssets,
		previewAssetRule
	} from '$lib/assetrules/api';
	import type { AssetRule, ExternalLink, UpdateAssetRuleInput } from '$lib/assetrules/types';
	import { searchTerms, getTerm } from '$lib/glossary/api';
	import type { GlossaryTerm } from '$lib/glossary/types';
	import type { Asset } from '$lib/assets/types';

	let ruleId = $derived($page.params.id);
	let canManage = $derived(auth.hasPermission('assets', 'manage'));

	let rule = $state<AssetRule | null>(null);
	let isLoading = $state(true);
	let loadError = $state<string | null>(null);
	let error = $state<string | null>(null);
	let saving = $state(false);
	let showDeleteModal = $state(false);

	// Form state
	let name = $state('');
	let description = $state('');
	let queryExpression = $state('');
	let links = $state<ExternalLink[]>([]);

	// Glossary term selection
	let selectedTerms = $state<GlossaryTerm[]>([]);
	let termSearchQuery = $state('');
	let termSearchResults = $state<GlossaryTerm[]>([]);
	let isSearchingTerms = $state(false);
	let showTermSearch = $state(false);
	let termSearchTimeout: ReturnType<typeof setTimeout>;

	// Assets tab
	let activeTab = $derived($page.url.searchParams.get('tab') || 'configuration');

	function setActiveTab(tab: string) {
		const url = new URL(window.location.href);
		url.searchParams.set('tab', tab);
		goto(url.toString(), { replaceState: true });
	}
	let assetIds = $state<string[]>([]);
	let assetDetails = $state<Map<string, Asset>>(new Map());
	let assetsTotal = $state(0);
	let isLoadingAssets = $state(false);
	let currentPage = $state(1);
	const PAGE_SIZE = 12;

	// Preview
	let previewing = $state(false);
	let previewCount = $state<number | null>(null);

	const tabs: Tab[] = [
		{ id: 'configuration', label: 'Configuration', icon: 'material-symbols:settings' },
		{ id: 'assets', label: 'Matched Assets', icon: 'material-symbols:database' }
	];

	// Derived pagination
	let totalAssetPages = $derived(Math.ceil(assetsTotal / PAGE_SIZE));

	async function populateForm(r: AssetRule) {
		name = r.name;
		description = r.description || '';
		queryExpression = r.query_expression || '';
		links = r.links?.length ? [...r.links] : [];

		// Load glossary term details for the term IDs
		const terms: GlossaryTerm[] = [];
		for (const termId of r.term_ids || []) {
			try {
				const term = await getTerm(termId);
				terms.push(term);
			} catch {
				// Term may have been deleted
			}
		}
		selectedTerms = terms;
	}

	async function loadRule() {
		isLoading = true;
		loadError = null;
		try {
			rule = await getAssetRule(ruleId);
			await populateForm(rule);
		} catch (e: any) {
			loadError = e.message || 'Failed to load asset rule';
		} finally {
			isLoading = false;
		}
	}

	async function loadAssets() {
		isLoadingAssets = true;
		try {
			const result = await getAssetRuleAssets(ruleId, (currentPage - 1) * PAGE_SIZE, PAGE_SIZE);
			assetIds = result.asset_ids || [];
			assetsTotal = result.total || 0;
			const newDetails = new Map(assetDetails);
			for (const id of assetIds) {
				if (!newDetails.has(id)) {
					try {
						const resp = await fetchApi(`/assets/${id}`);
						if (resp.ok) {
							newDetails.set(id, await resp.json());
						}
					} catch {}
				}
			}
			assetDetails = newDetails;
		} catch (e) {
			console.error('Failed to load assets:', e);
		} finally {
			isLoadingAssets = false;
		}
	}

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

	async function handleSave() {
		error = null;
		if (!name.trim()) {
			error = 'Name is required';
			return;
		}
		const validLinks = links.filter((l) => l.name.trim() && l.url.trim());
		if (validLinks.length === 0 && selectedTerms.length === 0) {
			error = 'At least one link or glossary term is required';
			return;
		}

		saving = true;
		try {
			const input: UpdateAssetRuleInput = {
				name: name.trim(),
				description: description.trim() || undefined,
				links: validLinks,
				term_ids: selectedTerms.map((t) => t.id),
				rule_type: 'query',
				query_expression: queryExpression.trim(),
				priority: 0,
				is_enabled: true
			};
			rule = await updateAssetRule(ruleId, input);
			await populateForm(rule);
		} catch (e: any) {
			error = e.message || 'Failed to update asset rule';
		} finally {
			saving = false;
		}
	}

	async function handleDelete() {
		try {
			await deleteAssetRule(ruleId);
			goto('/asset-rules');
		} catch (e: any) {
			error = e.message || 'Failed to delete asset rule';
			showDeleteModal = false;
		}
	}

	function getAssetUrl(asset: Asset): string {
		if (!asset.mrn) return '#';
		const mrnParts = asset.mrn.replace('mrn://', '').split('/');
		if (mrnParts.length < 3) return '#';
		return `/discover/${encodeURIComponent(mrnParts[0])}/${encodeURIComponent(mrnParts[1])}/${encodeURIComponent(mrnParts.slice(2).join('/'))}`;
	}

	function getIconType(asset: Asset): string {
		if (asset.providers?.length === 1) return asset.providers[0];
		return asset.type || 'unknown';
	}

	function formatDate(d: string): string {
		return new Date(d).toLocaleDateString('en-US', {
			month: 'short',
			day: 'numeric',
			year: 'numeric',
			hour: '2-digit',
			minute: '2-digit'
		});
	}

	async function handleAssetPageChange(newPage: number) {
		currentPage = newPage;
		await loadAssets();
	}

	$effect(() => {
		if (activeTab === 'assets' && rule) {
			loadAssets();
		}
	});

	onMount(() => {
		loadRule();
	});
</script>

<svelte:head>
	<title>{rule?.name || 'Asset Rule'} - Marmot</title>
</svelte:head>

<div class="h-full overflow-y-auto">
	<div class="max-w-7xl mx-auto px-8 py-8">
		<div class="mb-6">
			<button
				onclick={() => goto('/asset-rules')}
				class="inline-flex items-center text-sm text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300"
			>
				<IconifyIcon icon="mdi:arrow-left" class="w-5 h-5 mr-1" />
				Back to Asset Rules
			</button>
		</div>

		{#if isLoading}
			<div class="flex items-center justify-center py-16">
				<div
					class="animate-spin rounded-full h-8 w-8 border-b-2 border-earthy-terracotta-700"
				></div>
			</div>
		{:else if loadError}
			<div
				class="bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-lg p-6 text-center max-w-md mx-auto"
			>
				<IconifyIcon icon="material-symbols:error" class="h-12 w-12 text-red-400 mx-auto mb-3" />
				<p class="text-sm text-red-600 dark:text-red-300 mb-4">{loadError}</p>
				<Button click={() => goto('/asset-rules')} text="Back to Asset Rules" variant="clear" />
			</div>
		{:else if rule}
			<!-- Header -->
			<div class="flex items-start justify-between mb-6">
				<div class="flex items-start gap-5">
					<!-- Rule Icon -->
					<div
						class="w-16 h-16 rounded-xl bg-gradient-to-br from-earthy-terracotta-100 to-earthy-terracotta-200 dark:from-earthy-terracotta-900/50 dark:to-earthy-terracotta-800/30 flex items-center justify-center flex-shrink-0"
					>
						<IconifyIcon
							icon="material-symbols:rule-settings"
							class="text-3xl text-earthy-terracotta-600 dark:text-earthy-terracotta-400"
						/>
					</div>

					<div class="min-w-0 space-y-1">
						<div class="flex items-center gap-3">
							<h1 class="text-2xl font-semibold text-gray-900 dark:text-gray-100">
								{rule.name}
							</h1>
							<span
								class="text-xs px-2 py-0.5 rounded-full {rule.is_enabled
									? 'bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-400'
									: 'bg-gray-100 dark:bg-gray-700 text-gray-500'}"
							>
								{rule.is_enabled ? 'Active' : 'Disabled'}
							</span>
						</div>
						{#if rule.description}
							<p class="text-sm text-gray-500 dark:text-gray-400">{rule.description}</p>
						{/if}
						<div class="flex items-center gap-4 text-sm text-gray-400 dark:text-gray-500">
							{#if rule.links?.length > 0}
								<span class="flex items-center gap-1">
									<IconifyIcon icon="material-symbols:link" class="w-3.5 h-3.5" />
									{rule.links.length} link{rule.links.length !== 1 ? 's' : ''}
								</span>
							{/if}
							{#if rule.term_ids?.length > 0}
								<span class="flex items-center gap-1">
									<IconifyIcon icon="material-symbols:book" class="w-3.5 h-3.5" />
									{rule.term_ids.length} term{rule.term_ids.length !== 1 ? 's' : ''}
								</span>
							{/if}
							<span class="flex items-center gap-1">
								<IconifyIcon icon="material-symbols:database" class="w-3.5 h-3.5" />
								{rule.membership_count || 0} matched asset{rule.membership_count !== 1 ? 's' : ''}
							</span>
							{#if rule.updated_at}
								<span class="flex items-center gap-1">
									<IconifyIcon icon="material-symbols:schedule" class="w-3.5 h-3.5" />
									Updated {formatDate(rule.updated_at)}
								</span>
							{/if}
						</div>
					</div>
				</div>

				{#if canManage}
					<Button
						click={() => (showDeleteModal = true)}
						icon="material-symbols:delete"
						text="Delete"
						variant="clear"
					/>
				{/if}
			</div>

			<Tabs {tabs} {activeTab} onTabChange={setActiveTab} />

			{#if error}
				<div
					class="mt-6 p-3 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-lg text-sm text-red-800 dark:text-red-200"
				>
					{error}
				</div>
			{/if}

			{#if activeTab === 'configuration'}
				<div class="mt-6 grid grid-cols-1 lg:grid-cols-3 gap-6">
					<!-- Left column: Basic Info + Query -->
					<div class="lg:col-span-2 space-y-6">
						<!-- Basic Info -->
						<div
							class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-6 space-y-5"
						>
							<h2
								class="text-sm font-semibold text-gray-600 dark:text-gray-400 uppercase tracking-wider flex items-center gap-2"
							>
								<IconifyIcon icon="material-symbols:info-outline" class="w-4 h-4" />
								Basic Information
							</h2>
							<div>
								<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1.5">
									Name <span class="text-red-500">*</span>
								</label>
								<input
									type="text"
									bind:value={name}
									disabled={!canManage}
									class="w-full px-4 py-2.5 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-earthy-terracotta-600 focus:border-transparent disabled:opacity-50 transition-all"
								/>
							</div>
							<div>
								<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1.5">
									Description
								</label>
								<textarea
									bind:value={description}
									rows="3"
									disabled={!canManage}
									placeholder="What does this rule do?"
									class="w-full px-4 py-2.5 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-earthy-terracotta-600 focus:border-transparent disabled:opacity-50 resize-y transition-all"
								></textarea>
							</div>
						</div>

						<!-- Query -->
						<div
							class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-6 space-y-4"
						>
							<h2
								class="text-sm font-semibold text-gray-600 dark:text-gray-400 uppercase tracking-wider flex items-center gap-2"
							>
								<IconifyIcon icon="material-symbols:filter-list" class="w-4 h-4" />
								Query
							</h2>
							<p class="text-sm text-gray-500 dark:text-gray-400">
								Define a query to match assets. Enrichments will be automatically applied to all
								matching assets.
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
									class="p-3 bg-green-50 dark:bg-green-900/20 border border-green-200 dark:border-green-800 rounded-lg text-sm text-green-800 dark:text-green-200 flex items-center gap-2"
								>
									<IconifyIcon icon="material-symbols:check-circle" class="w-4 h-4" />
									Matches {previewCount} asset{previewCount !== 1 ? 's' : ''}
								</div>
							{/if}
						</div>

						{#if canManage}
							<div class="flex gap-3">
								<Button
									click={handleSave}
									icon={saving ? '' : 'material-symbols:save'}
									text={saving ? 'Saving...' : 'Save Changes'}
									variant="filled"
									disabled={saving}
								/>
							</div>
						{/if}
					</div>

					<!-- Right column: Enrichments -->
					<div class="space-y-6">
						<!-- External Links -->
						<div
							class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-6 space-y-4"
						>
							<h2
								class="text-sm font-semibold text-gray-600 dark:text-gray-400 uppercase tracking-wider flex items-center gap-2"
							>
								<IconifyIcon icon="material-symbols:link" class="w-4 h-4" />
								External Links
							</h2>
							<ExternalLinks bind:links canEdit={canManage} />
						</div>

						<!-- Glossary Terms -->
						<div
							class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-6 space-y-4"
						>
							<h2
								class="text-sm font-semibold text-gray-600 dark:text-gray-400 uppercase tracking-wider flex items-center gap-2"
							>
								<IconifyIcon icon="material-symbols:book" class="w-4 h-4" />
								Glossary Terms
							</h2>

							{#if selectedTerms.length > 0}
								<div class="space-y-2">
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
											{#if canManage}
												<button
													onclick={() => removeTerm(term.id)}
													aria-label="Remove term {term.name}"
													class="p-1 text-gray-400 hover:text-red-600 dark:hover:text-red-400 rounded flex-shrink-0"
												>
													<IconifyIcon icon="material-symbols:close" class="w-4 h-4" />
												</button>
											{/if}
										</div>
									{/each}
								</div>
							{:else if !canManage}
								<p class="text-sm text-gray-400 dark:text-gray-500 italic">No terms attached</p>
							{/if}

							{#if canManage}
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
											placeholder="Search terms..."
											class="w-full pl-9 pr-4 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-earthy-terracotta-600 focus:border-transparent"
										/>
									</div>
									{#if showTermSearch && (termSearchResults.length > 0 || isSearchingTerms)}
										<div
											class="absolute z-10 w-full mt-1 bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg shadow-lg max-h-48 overflow-y-auto"
										>
											{#if isSearchingTerms}
												<div class="px-4 py-3 text-sm text-gray-500 dark:text-gray-400">
													Searching...
												</div>
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
							{/if}
						</div>
					</div>
				</div>
			{/if}

			{#if activeTab === 'assets'}
				<div class="mt-6">
					{#if isLoadingAssets}
						<!-- Skeleton loading grid -->
						<div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
							{#each Array(6) as _}
								<div
									class="bg-gray-50 dark:bg-gray-700/50 rounded-xl border border-gray-200 dark:border-gray-600 p-4 animate-pulse"
								>
									<div class="flex items-start gap-3 mb-3">
										<div class="w-10 h-10 bg-gray-200 dark:bg-gray-600 rounded-lg"></div>
										<div class="flex-1 space-y-2">
											<div class="h-4 bg-gray-200 dark:bg-gray-600 rounded w-3/4"></div>
											<div class="h-3 bg-gray-200 dark:bg-gray-600 rounded w-full"></div>
										</div>
									</div>
									<div class="flex gap-2">
										<div class="h-5 bg-gray-200 dark:bg-gray-600 rounded w-12"></div>
									</div>
								</div>
							{/each}
						</div>
					{:else if assetIds.length === 0}
						<div class="text-center py-16 text-gray-500 dark:text-gray-400">
							<div
								class="w-16 h-16 rounded-2xl bg-gray-100 dark:bg-gray-800 flex items-center justify-center mx-auto mb-4"
							>
								<IconifyIcon icon="material-symbols:database" class="text-3xl opacity-50" />
							</div>
							<p class="font-medium text-gray-900 dark:text-gray-100">No matched assets yet</p>
							<p class="text-sm mt-1">
								Assets will appear here after the next reconciliation cycle
							</p>
						</div>
					{:else}
						<!-- Assets Grid -->
						<div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
							{#each assetIds as assetId}
								{@const asset = assetDetails.get(assetId)}
								<div
									class="group flex flex-col bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 hover:border-earthy-terracotta-300 dark:hover:border-earthy-terracotta-700 hover:shadow-md transition-all overflow-hidden"
								>
									{#if asset}
										<a href={getAssetUrl(asset)} class="flex-1 p-4">
											<div class="flex items-start gap-3">
												<div
													class="flex-shrink-0 p-2 bg-gray-100 dark:bg-gray-700 rounded-lg group-hover:bg-earthy-terracotta-50 dark:group-hover:bg-earthy-terracotta-900/20 transition-colors"
												>
													<AssetIcon name={getIconType(asset)} size="sm" showLabel={false} />
												</div>
												<div class="flex-1 min-w-0">
													<h4
														class="font-semibold text-gray-900 dark:text-gray-100 truncate group-hover:text-earthy-terracotta-700 dark:group-hover:text-earthy-terracotta-400 transition-colors"
													>
														{asset.name}
													</h4>
													<p
														class="text-xs text-gray-500 dark:text-gray-400 truncate font-mono mt-0.5"
													>
														{asset.mrn}
													</p>
												</div>
											</div>
										</a>

										<!-- Footer with type badge -->
										<div
											class="flex items-center px-4 py-2 bg-gray-50 dark:bg-gray-900/50 border-t border-gray-100 dark:border-gray-700 mt-auto"
										>
											<span
												class="text-xs px-2 py-1 rounded-md font-medium bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-300"
											>
												{asset.type?.replace(/_/g, ' ')}
											</span>
										</div>
									{:else}
										<!-- Loading placeholder for individual asset -->
										<div class="p-4 animate-pulse">
											<div class="flex items-start gap-3 mb-3">
												<div class="w-10 h-10 bg-gray-200 dark:bg-gray-600 rounded-lg"></div>
												<div class="flex-1 space-y-2">
													<div class="h-4 bg-gray-200 dark:bg-gray-600 rounded w-3/4"></div>
													<div class="h-3 bg-gray-200 dark:bg-gray-600 rounded w-full"></div>
												</div>
											</div>
										</div>
									{/if}
								</div>
							{/each}
						</div>

						<!-- Pagination Controls -->
						{#if totalAssetPages > 1}
							<div
								class="flex justify-between items-center mt-6 pt-4 border-t border-gray-200 dark:border-gray-700"
							>
								<p class="text-sm text-gray-600 dark:text-gray-400">
									Showing {(currentPage - 1) * PAGE_SIZE + 1}-{Math.min(
										currentPage * PAGE_SIZE,
										assetsTotal
									)} of {assetsTotal} assets
								</p>
								<div class="flex gap-2">
									<button
										onclick={() => handleAssetPageChange(currentPage - 1)}
										disabled={currentPage === 1 || isLoadingAssets}
										class="px-4 py-2 text-sm font-medium rounded-lg border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
									>
										Previous
									</button>
									<button
										onclick={() => handleAssetPageChange(currentPage + 1)}
										disabled={currentPage >= totalAssetPages || isLoadingAssets}
										class="px-4 py-2 text-sm font-medium rounded-lg border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
									>
										Next
									</button>
								</div>
							</div>
						{/if}
					{/if}
				</div>
			{/if}
		{/if}
	</div>
</div>

<ConfirmModal
	bind:show={showDeleteModal}
	title="Delete Asset Rule"
	message="Are you sure you want to delete this asset rule? External links and glossary terms will no longer be automatically applied to matching assets."
	confirmText="Delete"
	variant="danger"
	onConfirm={handleDelete}
	onCancel={() => (showDeleteModal = false)}
/>
