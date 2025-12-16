<script lang="ts">
	import { goto } from '$app/navigation';
	import { fetchApi } from '$lib/api';
	import type { Owner, RuleInput } from '$lib/dataproducts/types';
	import type { Asset } from '$lib/assets/types';
	import Button from './Button.svelte';
	import IconifyIcon from '@iconify/svelte';
	import OwnerSelector from './OwnerSelector.svelte';
	import RichTextEditor from './RichTextEditor.svelte';
	import Tags from './Tags.svelte';
	import MetadataView from './MetadataView.svelte';
	import AssetIcon from './Icon.svelte';
	import Stepper from './Stepper.svelte';
	import Step from './Step.svelte';
	import QueryBuilder from './QueryBuilder.svelte';

	type FormMode = 'create' | 'edit';

	let {
		mode = 'create',
		productId = undefined,
		initialName = '',
		initialDescription = '',
		initialDocumentation = '',
		initialOwners = [],
		initialTags = [],
		initialMetadata = {},
		initialRules = [],
		initialManualAssetIds = [],
		initialAssetDetails = new Map(),
		existingRuleIds = new Map(),
		resolvedAssets = { manual: [], dynamic: [], all: [] }
	}: {
		mode?: FormMode;
		productId?: string;
		initialName?: string;
		initialDescription?: string;
		initialDocumentation?: string;
		initialOwners?: Owner[];
		initialTags?: string[];
		initialMetadata?: Record<string, any>;
		initialRules?: RuleInput[];
		initialManualAssetIds?: string[];
		initialAssetDetails?: Map<string, Asset>;
		existingRuleIds?: Map<number, string>;
		resolvedAssets?: { manual: string[]; dynamic: string[]; all: string[] };
	} = $props();

	// Step state
	let currentStep = $state(1);
	let totalSteps = $state(0);

	// Form state
	let name = $state(initialName);
	let description = $state(initialDescription);
	let documentation = $state(initialDocumentation);
	let owners = $state<Owner[]>(initialOwners);
	let tags = $state<string[]>(initialTags);
	let metadata = $state<Record<string, any>>(initialMetadata);

	// Assets state
	let manualAssetIds = $state<string[]>(initialManualAssetIds);
	let assetSearchQuery = $state('');
	let assetSearchResults = $state<Asset[]>([]);
	let isSearchingAssets = $state(false);
	let assetDetails = $state<Map<string, Asset>>(initialAssetDetails);

	// Rules state
	let rules = $state<RuleInput[]>(initialRules);
	let showRuleForm = $state(false);
	let editingRuleIndex = $state<number | null>(null);
	let ruleForm = $state<RuleInput>(getEmptyRuleForm());
	let rulePreviewResults = $state<any[]>([]);
	let rulePreviewTotal = $state(0);
	let isPreviewLoading = $state(false);

	// Submission state
	let saving = $state(false);
	let error = $state<string | null>(null);

	// Validation
	let canProceedFromStep1 = $derived(name.trim() !== '');

	// Step navigation validation - step 1 requires name, all others just need step 1 complete
	function canNavigateToStep(stepNumber: number): boolean {
		if (stepNumber === 1) return true;
		return canProceedFromStep1;
	}

	// Sync initial values when they change (for edit mode loading)
	$effect(() => {
		name = initialName;
	});
	$effect(() => {
		description = initialDescription;
	});
	$effect(() => {
		documentation = initialDocumentation;
	});
	$effect(() => {
		owners = initialOwners;
	});
	$effect(() => {
		tags = initialTags;
	});
	$effect(() => {
		metadata = initialMetadata;
	});
	$effect(() => {
		rules = initialRules;
	});
	$effect(() => {
		manualAssetIds = initialManualAssetIds;
	});
	$effect(() => {
		assetDetails = initialAssetDetails;
	});

	function getEmptyRuleForm(): RuleInput {
		return {
			name: '',
			description: '',
			rule_type: 'query',
			query_expression: '',
			is_enabled: true
		};
	}

	function getIconType(asset: Asset): string {
		if (asset.providers && Array.isArray(asset.providers) && asset.providers.length === 1) {
			return asset.providers[0];
		}
		return asset.type || 'unknown';
	}

	async function searchAssets() {
		if (!assetSearchQuery.trim()) {
			assetSearchResults = [];
			return;
		}

		isSearchingAssets = true;
		try {
			const response = await fetchApi(
				`/assets/search?q=${encodeURIComponent(assetSearchQuery)}&limit=20`
			);
			if (response.ok) {
				const data = await response.json();
				assetSearchResults = data.assets || [];
			}
		} catch (err) {
			console.error('Failed to search assets:', err);
		} finally {
			isSearchingAssets = false;
		}
	}

	function addAsset(asset: Asset) {
		if (!manualAssetIds.includes(asset.id)) {
			manualAssetIds = [...manualAssetIds, asset.id];
			assetDetails.set(asset.id, asset);
			assetDetails = new Map(assetDetails);
		}
	}

	function removeAsset(assetId: string) {
		manualAssetIds = manualAssetIds.filter((id) => id !== assetId);
	}

	function openRuleForm(index: number | null = null) {
		if (index !== null && rules[index]) {
			ruleForm = { ...rules[index] };
			editingRuleIndex = index;
		} else {
			ruleForm = getEmptyRuleForm();
			editingRuleIndex = null;
		}
		rulePreviewResults = [];
		showRuleForm = true;
	}

	function closeRuleForm() {
		showRuleForm = false;
		ruleForm = getEmptyRuleForm();
		editingRuleIndex = null;
		rulePreviewResults = [];
	}

	function saveRule() {
		if (!ruleForm.name.trim()) return;

		if (editingRuleIndex !== null) {
			// Preserve the existing rule ID if it has one
			const existingId = existingRuleIds.get(editingRuleIndex);
			rules[editingRuleIndex] = { ...ruleForm, id: existingId };
			rules = [...rules];
		} else {
			rules = [...rules, { ...ruleForm }];
		}
		closeRuleForm();
	}

	function deleteRule(index: number) {
		existingRuleIds.delete(index);
		rules = rules.filter((_, i) => i !== index);
	}

	async function previewRule(queryExpression?: string) {
		isPreviewLoading = true;
		rulePreviewResults = [];
		rulePreviewTotal = 0;

		const query = queryExpression || ruleForm.query_expression || '';
		if (!query.trim()) {
			isPreviewLoading = false;
			return;
		}

		try {
			const params = new URLSearchParams({
				q: query,
				limit: '10'
			});
			params.append('types[]', 'asset');

			const response = await fetchApi(`/search?${params}`);

			if (!response.ok) {
				const errorData = await response.json();
				throw new Error(errorData.error || 'Failed to preview rule');
			}

			const data = await response.json();
			rulePreviewResults = (data.results || []).filter((r: any) => r.type === 'asset');
			rulePreviewTotal = data.total || rulePreviewResults.length;
		} catch (err) {
			console.error('Failed to preview rule:', err);
		} finally {
			isPreviewLoading = false;
		}
	}

	function getPreviewIconType(result: any): string {
		const metadata = result.metadata;
		if (
			metadata?.providers &&
			Array.isArray(metadata.providers) &&
			metadata.providers.length === 1
		) {
			return metadata.providers[0];
		}
		return metadata?.type || 'unknown';
	}

	function getPreviewAssetUrl(result: any): string {
		const metadata = result.metadata;
		if (!metadata?.mrn) return '#';
		const mrnParts = metadata.mrn.replace('mrn://', '').split('/');
		if (mrnParts.length < 3) return '#';
		const type = mrnParts[0];
		const service = mrnParts[1];
		const fullName = mrnParts.slice(2).join('/');
		return `/discover/${encodeURIComponent(type)}/${encodeURIComponent(service)}/${encodeURIComponent(fullName)}`;
	}

	function handleNextStep() {
		if (currentStep === 1) {
			if (!name.trim()) {
				error = 'Name is required';
				return;
			}
		}

		if (currentStep < totalSteps) {
			error = null;
			currentStep++;
		}
	}

	async function handleSave() {
		if (!name.trim()) {
			error = 'Name is required';
			currentStep = 1;
			return;
		}

		saving = true;
		error = null;

		try {
			if (mode === 'create') {
				await handleCreate();
			} else {
				await handleUpdate();
			}
		} catch (err) {
			error = err instanceof Error ? err.message : `Failed to ${mode} data product`;
		} finally {
			saving = false;
		}
	}

	async function handleCreate() {
		const body = {
			name: name.trim(),
			description: description.trim() || undefined,
			documentation: documentation.trim() || undefined,
			owners: owners.length > 0 ? owners.map((o) => ({ id: o.id, type: o.type })) : undefined,
			tags: tags,
			metadata: Object.keys(metadata).length > 0 ? metadata : undefined,
			rules:
				rules.length > 0
					? rules.map((r) => ({
							name: r.name,
							description: r.description,
							rule_type: r.rule_type,
							query_expression: r.query_expression,
							is_enabled: r.is_enabled
						}))
					: undefined
		};

		const response = await fetchApi('/products/', {
			method: 'POST',
			body: JSON.stringify(body)
		});

		if (!response.ok) {
			const errorData = await response.json();
			throw new Error(errorData.error || 'Failed to create data product');
		}

		const created = await response.json();

		// Add manual assets if any
		if (manualAssetIds.length > 0) {
			await fetchApi(`/products/assets/${created.id}`, {
				method: 'POST',
				body: JSON.stringify({ asset_ids: manualAssetIds })
			});
		}

		goto(`/products/${created.id}`);
	}

	async function handleUpdate() {
		if (!productId) throw new Error('Product ID is required for updates');

		// Update the data product
		const updateBody: any = {
			name: name.trim(),
			description: description.trim() || null,
			documentation: documentation.trim() || null,
			owners: owners.map((o) => ({ id: o.id, type: o.type })),
			tags: tags,
			metadata: metadata
		};

		const updateResponse = await fetchApi(`/products/${productId}`, {
			method: 'PUT',
			body: JSON.stringify(updateBody)
		});

		if (!updateResponse.ok) {
			const errorData = await updateResponse.json();
			throw new Error(errorData.error || 'Failed to update data product');
		}

		// Handle rules - delete removed rules, update existing, create new
		const currentRuleIds = new Set(rules.filter((r) => r.id).map((r) => r.id));

		// Delete rules that were removed
		for (const [_, existingId] of existingRuleIds) {
			if (!currentRuleIds.has(existingId)) {
				await fetchApi(`/products/rules/${productId}/${existingId}`, {
					method: 'DELETE'
				});
			}
		}

		// Update or create rules
		for (const rule of rules) {
			const ruleBody = {
				name: rule.name,
				description: rule.description,
				rule_type: rule.rule_type,
				query_expression: rule.query_expression,
				is_enabled: rule.is_enabled
			};

			if (rule.id) {
				// Update existing rule
				await fetchApi(`/products/rules/${productId}/${rule.id}`, {
					method: 'PUT',
					body: JSON.stringify(ruleBody)
				});
			} else {
				// Create new rule
				await fetchApi(`/products/rules/${productId}`, {
					method: 'POST',
					body: JSON.stringify(ruleBody)
				});
			}
		}

		// Handle manual assets
		const assetsToAdd = manualAssetIds.filter((id) => !resolvedAssets.manual.includes(id));
		const assetsToRemove = resolvedAssets.manual.filter((id) => !manualAssetIds.includes(id));

		// Add new assets
		if (assetsToAdd.length > 0) {
			await fetchApi(`/products/assets/${productId}`, {
				method: 'POST',
				body: JSON.stringify({ asset_ids: assetsToAdd })
			});
		}

		// Remove assets
		for (const assetId of assetsToRemove) {
			await fetchApi(`/products/assets/${productId}/${assetId}`, {
				method: 'DELETE'
			});
		}

		goto('/products');
	}

	let searchDebounceTimer: ReturnType<typeof setTimeout>;
	function handleAssetSearchInput(e: Event) {
		const target = e.target as HTMLInputElement;
		assetSearchQuery = target.value;
		clearTimeout(searchDebounceTimer);
		searchDebounceTimer = setTimeout(() => {
			searchAssets();
		}, 300);
	}

	const pageTitle = $derived(mode === 'create' ? 'Create Data Product' : 'Edit Data Product');
	const saveButtonText = $derived(
		saving ? (mode === 'create' ? 'Creating...' : 'Saving...') : mode === 'create' ? 'Create Data Product' : 'Save Changes'
	);
</script>

<div class="min-h-screen">
	<!-- Header -->
	<div class="border-b border-gray-200 dark:border-gray-700">
		<div class="container max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-6">
			<div class="flex items-center gap-4">
				<button
					onclick={() => goto('/products')}
					class="p-2 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg transition-colors"
				>
					<IconifyIcon
						icon="material-symbols:arrow-back"
						class="h-6 w-6 text-gray-600 dark:text-gray-400"
					/>
				</button>
				<div>
					<h1 class="text-2xl font-bold text-gray-900 dark:text-gray-100">
						{pageTitle}
					</h1>
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
				<Step title="Details" icon="material-symbols:description" />
				<Step title="Assets" icon="material-symbols:database" />
				<Step title="Rules" icon="material-symbols:filter-list" />
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
				<h3
					class="text-base font-semibold text-gray-900 dark:text-gray-100 mb-4 flex items-center"
				>
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
							placeholder="e.g., Customer Analytics"
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
							placeholder="A brief description of this data product..."
							rows="2"
							class="w-full px-4 py-2.5 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-earthy-terracotta-600 focus:border-transparent transition-all resize-none"
						></textarea>
					</div>

					<div class="grid grid-cols-1 md:grid-cols-2 gap-5">
						<div>
							<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
								Tags
							</label>
							<Tags bind:tags canEdit={true} />
						</div>

						<div>
							<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
								Owners
							</label>
							<OwnerSelector bind:selectedOwners={owners} />
						</div>
					</div>
				</div>
			</div>
		{/if}

		<!-- Step 2: Details (Documentation & Metadata) -->
		{#if currentStep === 2}
			<div
				class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-6"
			>
				<h3
					class="text-base font-semibold text-gray-900 dark:text-gray-100 mb-4 flex items-center"
				>
					<IconifyIcon
						icon="material-symbols:description"
						class="h-5 w-5 mr-2 text-earthy-terracotta-600"
					/>
					Details
				</h3>

				<div class="space-y-5">
					<div>
						<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
							Documentation
						</label>
						<p class="text-xs text-gray-500 dark:text-gray-400 mb-2">
							Rich documentation with formatting, images, and links.
						</p>
						<RichTextEditor
							bind:value={documentation}
							placeholder="Add detailed documentation for this data product..."
						/>
					</div>

					<MetadataView bind:metadata readOnly={false} maxDepth={2} />
				</div>
			</div>
		{/if}

		<!-- Step 3: Assets -->
		{#if currentStep === 3}
			<div
				class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-6"
			>
				<h3
					class="text-base font-semibold text-gray-900 dark:text-gray-100 mb-4 flex items-center"
				>
					<IconifyIcon
						icon="material-symbols:database"
						class="h-5 w-5 mr-2 text-earthy-terracotta-600"
					/>
					Manual Assets
					<span class="text-xs font-normal text-gray-500 ml-2">(Optional)</span>
				</h3>
				<p class="text-sm text-gray-600 dark:text-gray-400 mb-4">
					Search and add specific assets to this data product. You can also add dynamic rules in
					the next step.
				</p>

				<!-- Dynamic assets info (edit mode only) -->
				{#if mode === 'edit' && resolvedAssets.dynamic.length > 0}
					<div
						class="mb-4 p-3 bg-blue-50 dark:bg-blue-900/20 border border-blue-200 dark:border-blue-800/50 rounded-lg"
					>
						<div class="flex items-center gap-2 text-sm text-blue-700 dark:text-blue-300">
							<IconifyIcon icon="material-symbols:auto-awesome" class="h-4 w-4" />
							<span>{resolvedAssets.dynamic.length} assets matched by rules</span>
						</div>
					</div>
				{/if}

				<!-- Asset Search -->
				<div class="mb-6">
					<div class="relative">
						<IconifyIcon
							icon="material-symbols:search"
							class="absolute left-4 top-1/2 -translate-y-1/2 h-5 w-5 text-gray-400"
						/>
						<input
							type="text"
							value={assetSearchQuery}
							oninput={handleAssetSearchInput}
							placeholder="Search assets by name, MRN, or type..."
							class="w-full pl-12 pr-4 py-3 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-earthy-terracotta-600 focus:border-transparent transition-all"
						/>
						{#if isSearchingAssets}
							<div class="absolute right-4 top-1/2 -translate-y-1/2">
								<div
									class="animate-spin rounded-full h-5 w-5 border-b-2 border-earthy-terracotta-700"
								></div>
							</div>
						{/if}
					</div>

					<!-- Search Results -->
					{#if assetSearchResults.length > 0}
						<div
							class="mt-2 border border-gray-200 dark:border-gray-700 rounded-lg max-h-64 overflow-y-auto"
						>
							{#each assetSearchResults as asset}
								<button
									type="button"
									onclick={() => addAsset(asset)}
									disabled={manualAssetIds.includes(asset.id)}
									class="w-full flex items-center justify-between p-3 hover:bg-gray-50 dark:hover:bg-gray-700/50 transition-colors text-left border-b border-gray-100 dark:border-gray-700 last:border-b-0 disabled:opacity-50 disabled:cursor-not-allowed"
								>
									<div class="flex items-center gap-3 flex-1 min-w-0">
										<AssetIcon name={getIconType(asset)} size="sm" showLabel={false} />
										<div class="flex-1 min-w-0">
											<div
												class="text-sm font-medium text-gray-900 dark:text-gray-100 truncate"
											>
												{asset.name}
											</div>
											<div class="text-xs text-gray-500 dark:text-gray-400 truncate font-mono">
												{asset.mrn}
											</div>
										</div>
									</div>
									{#if manualAssetIds.includes(asset.id)}
										<span class="text-xs text-green-600 dark:text-green-400">Added</span>
									{:else}
										<IconifyIcon
											icon="material-symbols:add"
											class="h-5 w-5 text-earthy-terracotta-600"
										/>
									{/if}
								</button>
							{/each}
						</div>
					{/if}
				</div>

				<!-- Selected Assets -->
				{#if manualAssetIds.length > 0}
					<div>
						<h4 class="text-sm font-medium text-gray-700 dark:text-gray-300 mb-3">
							{mode === 'edit' ? 'Manual Assets' : 'Selected Assets'} ({manualAssetIds.length})
						</h4>
						<div class="space-y-2">
							{#each manualAssetIds as assetId}
								{@const asset = assetDetails.get(assetId)}
								<div
									class="flex items-center justify-between p-3 bg-gray-50 dark:bg-gray-700/50 rounded-lg"
								>
									<div class="flex items-center gap-3 flex-1 min-w-0">
										{#if asset}
											<AssetIcon name={getIconType(asset)} size="sm" showLabel={false} />
											<div class="flex-1 min-w-0">
												<div
													class="text-sm font-medium text-gray-900 dark:text-gray-100 truncate"
												>
													{asset.name}
												</div>
												<div
													class="text-xs text-gray-500 dark:text-gray-400 truncate font-mono"
												>
													{asset.mrn}
												</div>
											</div>
										{:else}
											<div class="text-sm text-gray-500">{assetId}</div>
										{/if}
									</div>
									<button
										type="button"
										onclick={() => removeAsset(assetId)}
										class="p-1.5 text-red-600 hover:bg-red-50 dark:hover:bg-red-900/20 rounded transition-colors"
									>
										<IconifyIcon icon="material-symbols:close" class="h-4 w-4" />
									</button>
								</div>
							{/each}
						</div>
					</div>
				{:else}
					<div
						class="text-center py-8 text-gray-500 dark:text-gray-400 border-2 border-dashed border-gray-200 dark:border-gray-700 rounded-lg"
					>
						<IconifyIcon
							icon="material-symbols:database"
							class="h-12 w-12 mx-auto mb-3 opacity-50"
						/>
						<p>{mode === 'edit' ? 'No manual assets added' : 'No assets added yet'}</p>
						<p class="text-sm">
							Search and add assets above, or {mode === 'edit'
								? 'use rules for dynamic matching'
								: 'add rules in the next step'}
						</p>
					</div>
				{/if}
			</div>
		{/if}

		<!-- Step 4: Rules -->
		{#if currentStep === 4}
			<div class="space-y-6">
				<!-- Header -->
				<div
					class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-6"
				>
					<div class="flex items-center justify-between">
						<div>
							<h3
								class="text-base font-semibold text-gray-900 dark:text-gray-100 flex items-center"
							>
								<IconifyIcon
									icon="material-symbols:auto-awesome"
									class="h-5 w-5 mr-2 text-earthy-terracotta-600"
								/>
								Dynamic Rules
								<span class="text-xs font-normal text-gray-500 ml-2">(Optional)</span>
							</h3>
							<p class="text-sm text-gray-600 dark:text-gray-400 mt-1">
								Automatically include assets that match your criteria. Rules are evaluated
								continuously to keep your data product up to date.
							</p>
						</div>
						{#if !showRuleForm && rules.length > 0}
							<Button
								variant="clear"
								click={() => openRuleForm()}
								icon="material-symbols:add"
								text="Add Rule"
							/>
						{/if}
					</div>
				</div>

				<!-- Rule Form -->
				{#if showRuleForm}
					<div
						class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 overflow-hidden"
					>
						<div
							class="px-6 py-4 border-b border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-800/50"
						>
							<h4 class="text-sm font-semibold text-gray-900 dark:text-gray-100">
								{editingRuleIndex !== null ? 'Edit Rule' : 'Create New Rule'}
							</h4>
						</div>

						<div class="p-6 space-y-6">
							<!-- Basic Info -->
							<div class="grid grid-cols-1 md:grid-cols-2 gap-4">
								<div>
									<label
										class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2"
									>
										Rule Name <span class="text-red-500">*</span>
									</label>
									<input
										type="text"
										bind:value={ruleForm.name}
										placeholder="e.g., All Customer Tables"
										class="w-full px-4 py-2.5 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-earthy-terracotta-600 focus:border-transparent transition-all"
									/>
								</div>
								<div>
									<label
										class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2"
									>
										Description
									</label>
									<input
										type="text"
										bind:value={ruleForm.description}
										placeholder="Brief description of what this rule matches..."
										class="w-full px-4 py-2.5 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-earthy-terracotta-600 focus:border-transparent transition-all"
									/>
								</div>
							</div>

							<!-- Query Builder with integrated Preview button -->
							<div>
								<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-3">
									Query Expression
								</label>
								<QueryBuilder
									query={ruleForm.query_expression || ''}
									onQueryChange={(q) => (ruleForm.query_expression = q)}
									initiallyExpanded={true}
									showRunButton={true}
									runButtonText={isPreviewLoading ? 'Searching...' : 'Preview'}
									runButtonIcon={isPreviewLoading
										? 'mdi:loading'
										: 'material-symbols:visibility'}
									onRunClick={(q) => previewRule(q)}
								/>
							</div>

							<!-- Preview Results -->
							{#if isPreviewLoading}
								<div class="space-y-2">
									{#each Array(3) as _}
										<div
											class="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-3 animate-pulse"
										>
											<div class="flex items-center gap-2 mb-2">
												<div class="h-6 w-6 bg-gray-200 dark:bg-gray-600 rounded"></div>
												<div class="space-y-1.5 flex-1">
													<div class="h-4 bg-gray-200 dark:bg-gray-600 rounded w-48"></div>
													<div class="h-3 bg-gray-200 dark:bg-gray-600 rounded w-64"></div>
												</div>
											</div>
										</div>
									{/each}
								</div>
							{:else if rulePreviewResults.length > 0}
								<div class="space-y-3">
									<div class="flex items-center justify-between">
										<div
											class="flex items-center gap-2 text-sm text-green-700 dark:text-green-400"
										>
											<IconifyIcon icon="material-symbols:check-circle" class="h-4 w-4" />
											<span class="font-medium"
												>{rulePreviewTotal} asset{rulePreviewTotal !== 1 ? 's' : ''} matched</span
											>
										</div>
										{#if rulePreviewTotal > rulePreviewResults.length}
											<span class="text-xs text-gray-500 dark:text-gray-400">
												Showing first {rulePreviewResults.length}
											</span>
										{/if}
									</div>
									<div class="space-y-2 max-h-80 overflow-y-auto">
										{#each rulePreviewResults as result}
											<a
												href={getPreviewAssetUrl(result)}
												target="_blank"
												class="block bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-3 hover:shadow-md hover:border-earthy-terracotta-300 dark:hover:border-earthy-terracotta-700 transition-all group"
											>
												<div class="flex items-start gap-3">
													<div class="flex-shrink-0">
														<AssetIcon
															name={getPreviewIconType(result)}
															showLabel={false}
															size="sm"
														/>
													</div>
													<div class="flex-1 min-w-0">
														<div class="flex items-center justify-between gap-2">
															<h4
																class="font-medium text-sm text-gray-900 dark:text-gray-100 truncate group-hover:text-earthy-terracotta-700 dark:group-hover:text-earthy-terracotta-400 transition-colors"
															>
																{result.name}
															</h4>
															<span
																class="flex-shrink-0 text-xs bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-300 px-2 py-0.5 rounded"
															>
																{result.metadata?.type?.replace(/_/g, ' ')}
															</span>
														</div>
														<p
															class="text-xs text-gray-500 dark:text-gray-400 truncate font-mono mt-0.5"
														>
															{result.metadata?.mrn}
														</p>
														{#if result.description}
															<p
																class="text-xs text-gray-600 dark:text-gray-400 mt-1 line-clamp-1"
															>
																{result.description}
															</p>
														{/if}
													</div>
												</div>
											</a>
										{/each}
									</div>
								</div>
							{:else}
								<div class="text-center py-8 text-gray-500 dark:text-gray-400">
									<IconifyIcon
										icon="material-symbols:search"
										class="h-10 w-10 mx-auto mb-2 opacity-40"
									/>
									<p class="text-sm">Click "Preview" to see which assets match this rule</p>
								</div>
							{/if}

							<!-- Actions -->
							<div
								class="flex items-center justify-end pt-4 border-t border-gray-200 dark:border-gray-700"
							>
								<div class="flex gap-3">
									<Button variant="clear" click={closeRuleForm} text="Cancel" />
									<Button
										variant="filled"
										click={saveRule}
										text={editingRuleIndex !== null ? 'Update Rule' : 'Add Rule'}
										icon="material-symbols:check"
										disabled={!ruleForm.name.trim()}
									/>
								</div>
							</div>
						</div>
					</div>
				{/if}

				<!-- Rules List -->
				{#if rules.length > 0 && !showRuleForm}
					<div class="space-y-3">
						{#each rules as rule, index}
							<div
								class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-4 hover:shadow-md transition-shadow"
							>
								<div class="flex items-start justify-between">
									<div class="flex-1 min-w-0">
										<div class="flex items-center gap-3">
											<div
												class="w-8 h-8 rounded-lg flex items-center justify-center bg-blue-100 dark:bg-blue-900/30 text-blue-600 dark:text-blue-400"
											>
												<IconifyIcon icon="mdi:database-search" class="h-4 w-4" />
											</div>
											<div>
												<span
													class="text-sm font-semibold text-gray-900 dark:text-gray-100"
												>
													{rule.name}
												</span>
												{#if rule.description}
													<p class="text-xs text-gray-500 dark:text-gray-400 mt-0.5">
														{rule.description}
													</p>
												{/if}
											</div>
										</div>
										{#if rule.query_expression}
											<div
												class="mt-3 px-3 py-2 bg-gray-50 dark:bg-gray-700/50 rounded-lg font-mono text-xs text-gray-600 dark:text-gray-400 truncate"
											>
												{rule.query_expression}
											</div>
										{/if}
									</div>
									<div class="flex items-center gap-1 ml-4">
										<button
											type="button"
											onclick={() => openRuleForm(index)}
											class="p-2 text-gray-500 hover:text-gray-700 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg transition-colors"
											title="Edit rule"
										>
											<IconifyIcon icon="material-symbols:edit" class="h-4 w-4" />
										</button>
										<button
											type="button"
											onclick={() => deleteRule(index)}
											class="p-2 text-gray-500 hover:text-red-600 hover:bg-red-50 dark:hover:bg-red-900/20 rounded-lg transition-colors"
											title="Delete rule"
										>
											<IconifyIcon icon="material-symbols:delete" class="h-4 w-4" />
										</button>
									</div>
								</div>
							</div>
						{/each}
					</div>
				{:else if !showRuleForm}
					<button
						type="button"
						onclick={() => openRuleForm()}
						class="w-full bg-white dark:bg-gray-800 rounded-xl border-2 border-dashed border-gray-300 dark:border-gray-600 p-8 hover:border-earthy-terracotta-500 hover:bg-earthy-terracotta-50 dark:hover:bg-earthy-terracotta-900/10 transition-all group"
					>
						<div class="text-center">
							<div
								class="w-12 h-12 mx-auto rounded-full bg-gray-100 dark:bg-gray-700 flex items-center justify-center group-hover:bg-earthy-terracotta-100 dark:group-hover:bg-earthy-terracotta-900/30 transition-colors"
							>
								<IconifyIcon
									icon="material-symbols:add"
									class="h-6 w-6 text-gray-400 group-hover:text-earthy-terracotta-600 transition-colors"
								/>
							</div>
							<p
								class="mt-3 text-sm font-medium text-gray-600 dark:text-gray-400 group-hover:text-earthy-terracotta-700 dark:group-hover:text-earthy-terracotta-400"
							>
								Add your first rule
							</p>
							<p class="mt-1 text-xs text-gray-500 dark:text-gray-500">
								Rules automatically include assets that match your criteria
							</p>
						</div>
					</button>
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
					<Button variant="clear" click={() => goto('/products')} text="Cancel" />
				{/if}
			</div>
			<div class="flex items-center gap-3">
				{#if currentStep < totalSteps}
					<Button
						variant="filled"
						click={handleNextStep}
						text="Next"
						icon="material-symbols:arrow-forward"
						disabled={currentStep === 1 && !canProceedFromStep1}
					/>
				{:else}
					<div class="text-sm text-gray-500 dark:text-gray-400 mr-4">
						<IconifyIcon icon="material-symbols:inventory-2" class="inline h-4 w-4 mr-1" />
						{manualAssetIds.length} manual asset{manualAssetIds.length !== 1 ? 's' : ''}, {rules.length}
						rule{rules.length !== 1 ? 's' : ''}
					</div>
					<Button
						variant="filled"
						click={handleSave}
						text={saveButtonText}
						disabled={saving || !name.trim()}
						icon="material-symbols:check"
					/>
				{/if}
			</div>
		</div>
	</div>
</div>
