<script lang="ts">
	import { untrack } from 'svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { SvelteMap, SvelteURLSearchParams } from 'svelte/reactivity';
	import { fetchApi } from '$lib/api';
	import type { Owner, RuleInput } from '$lib/dataproducts/types';
	import type { Asset } from '$lib/assets/types';
	import Button from '$components/ui/Button.svelte';
	import IconifyIcon from '@iconify/svelte';
	import OwnerSelector from '$components/shared/OwnerSelector.svelte';
	import RichTextEditor from '$components/editor/RichTextEditor.svelte';
	import Tags from '$components/shared/Tags.svelte';
	import MetadataView from '$components/shared/MetadataView.svelte';
	import AssetIcon from '$components/ui/Icon.svelte';
	import QueryBuilder from '$components/query/QueryBuilder.svelte';
	import DataProductSummary from '$components/product/DataProductSummary.svelte';
	import ConfirmModal from '$components/ui/ConfirmModal.svelte';
	import { m } from '$lib/paraglide/messages';
	import { formatList } from '$lib/utils';
	import { useUnsavedChangesGuard } from '$lib/utils/unsavedChanges.svelte';

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
		initialAssetDetails = new SvelteMap(),
		existingRuleIds = new SvelteMap(),
		resolvedAssets = { manual: [], dynamic: [], all: [] }
	}: {
		mode?: FormMode;
		productId?: string;
		initialName?: string;
		initialDescription?: string;
		initialDocumentation?: string;
		initialOwners?: Owner[];
		initialTags?: string[];
		initialMetadata?: Record<string, unknown>;
		initialRules?: RuleInput[];
		initialManualAssetIds?: string[];
		initialAssetDetails?: SvelteMap<string, Asset>;
		existingRuleIds?: SvelteMap<number, string>;
		resolvedAssets?: { manual: string[]; dynamic: string[]; all: string[] };
	} = $props();

	// Form state
	let name = $state(untrack(() => initialName));
	let description = $state(untrack(() => initialDescription));
	let documentation = $state(untrack(() => initialDocumentation));
	let owners = $state<Owner[]>(untrack(() => initialOwners));
	let tags = $state<string[]>(untrack(() => initialTags));
	let metadata = $state<Record<string, unknown>>(untrack(() => initialMetadata));

	// Assets state
	let manualAssetIds = $state<string[]>(untrack(() => initialManualAssetIds));
	let assetSearchQuery = $state('');
	let assetSearchResults = $state<Asset[]>([]);
	let isSearchingAssets = $state(false);
	let assetDetails = $state<SvelteMap<string, Asset>>(untrack(() => initialAssetDetails));

	// Rules state
	interface PreviewResult {
		id?: string;
		name: string;
		type: string;
		description?: string;
		metadata?: {
			mrn?: string;
			type?: string;
			providers?: string[];
			[key: string]: unknown;
		};
	}

	let rules = $state<RuleInput[]>(untrack(() => initialRules));
	let showRuleForm = $state(false);
	let editingRuleIndex = $state<number | null>(null);
	let ruleForm = $state<RuleInput>(getEmptyRuleForm());
	let rulePreviewResults = $state<PreviewResult[]>([]);
	let rulePreviewTotal = $state(0);
	let isPreviewLoading = $state(false);

	// Submission state
	let saving = $state(false);
	let error = $state<string | null>(null);

	// Validation
	let canSave = $derived(name.trim() !== '');
	let nameInput = $state<HTMLInputElement | null>(null);
	let showNameError = $state(false);

	// Set once a save succeeds so the unsaved changes guard does not block the redirect that follows it.
	let submitted = false;

	function snapshot(values: {
		name: string;
		description: string;
		documentation: string;
		owners: Owner[];
		tags: string[];
		metadata: Record<string, unknown>;
		rules: RuleInput[];
		manualAssetIds: string[];
	}): string {
		return JSON.stringify({
			name: values.name.trim(),
			description: values.description.trim(),
			documentation: values.documentation.trim(),
			owners: values.owners.map((o) => `${o.type}:${o.id}`),
			tags: values.tags,
			metadata: values.metadata,
			rules: values.rules.map((r) => [r.name, r.description, r.query_expression, r.is_enabled]),
			manualAssetIds: values.manualAssetIds
		});
	}

	let baseline = $derived(
		snapshot({
			name: initialName,
			description: initialDescription,
			documentation: initialDocumentation,
			owners: initialOwners,
			tags: initialTags,
			metadata: initialMetadata,
			rules: initialRules,
			manualAssetIds: initialManualAssetIds
		})
	);
	let isDirty = $derived(
		(showRuleForm && (ruleForm.name.trim() !== '' || !!ruleForm.query_expression?.trim())) ||
			snapshot({
				name,
				description,
				documentation,
				owners,
				tags,
				metadata,
				rules,
				manualAssetIds
			}) !== baseline
	);

	const unsavedGuard = useUnsavedChangesGuard(() => !submitted && !saving && isDirty);

	// Sync initial values when they change (for edit mode loading)
	$effect(() => {
		name = initialName;
		description = initialDescription;
		documentation = initialDocumentation;
		owners = initialOwners;
		tags = initialTags;
		metadata = initialMetadata;
		rules = initialRules;
		manualAssetIds = initialManualAssetIds;
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
			assetDetails = new SvelteMap(assetDetails);
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
			const params = new SvelteURLSearchParams({
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
			rulePreviewResults = (data.results || []).filter((r: PreviewResult) => r.type === 'asset');
			rulePreviewTotal = data.total || rulePreviewResults.length;
		} catch (err) {
			console.error('Failed to preview rule:', err);
		} finally {
			isPreviewLoading = false;
		}
	}

	function getPreviewIconType(result: PreviewResult): string {
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

	function getPreviewAssetParams(
		result: PreviewResult
	): { type: string; service: string; name: string } | undefined {
		const mrn = result.metadata?.mrn;
		if (!mrn) return undefined;
		const mrnParts = mrn.replace('mrn://', '').split('/');
		if (mrnParts.length < 3) return undefined;
		return { type: mrnParts[0], service: mrnParts[1], name: mrnParts.slice(2).join('/') };
	}

	async function handleSave() {
		if (!name.trim()) {
			showNameError = true;
			error = m.products_name_required();
			nameInput?.focus();
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
			error =
				err instanceof Error
					? err.message
					: mode === 'create'
						? m.products_error_create()
						: m.products_error_update();
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
			throw new Error(errorData.error || m.products_error_create());
		}

		const created = await response.json();

		// Add manual assets if any
		if (manualAssetIds.length > 0) {
			await fetchApi(`/products/assets/${created.id}`, {
				method: 'POST',
				body: JSON.stringify({ asset_ids: manualAssetIds })
			});
		}

		submitted = true;
		goto(resolve('/products/[id]', { id: created.id }));
	}

	async function handleUpdate() {
		if (!productId) throw new Error('Product ID is required for updates');

		// Update the data product
		const updateBody: Record<string, unknown> = {
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
			throw new Error(errorData.error || m.products_error_update());
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

		submitted = true;
		goto(resolve('/products'));
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

	const pageDescription = $derived(
		mode === 'create' ? m.products_form_create_description() : m.products_form_edit_description()
	);
	const pageTitle = $derived(
		mode === 'create' ? m.products_form_create_title() : m.products_form_edit_title()
	);
	const saveButtonText = $derived(
		saving
			? mode === 'create'
				? m.products_creating()
				: m.products_saving()
			: mode === 'create'
				? m.products_form_create_title()
				: m.products_save_changes()
	);
</script>

{#snippet previewResultBody(result: PreviewResult)}
	<div class="flex items-start gap-3">
		<div class="flex-shrink-0">
			<AssetIcon name={getPreviewIconType(result)} showLabel={false} size="sm" />
		</div>
		<div class="min-w-0 flex-1">
			<div class="flex items-center justify-between gap-2">
				<h4
					class="truncate text-sm font-medium text-gray-900 transition-colors group-hover:text-earthy-terracotta-700 dark:text-gray-100 dark:group-hover:text-earthy-terracotta-400"
				>
					{result.name}
				</h4>
				<span
					class="flex-shrink-0 rounded bg-gray-100 px-2 py-0.5 text-xs text-gray-600 dark:bg-gray-700 dark:text-gray-300"
				>
					{result.metadata?.type?.replace(/_/g, ' ')}
				</span>
			</div>
			<p class="mt-0.5 truncate font-mono text-xs text-gray-500 dark:text-gray-400">
				{result.metadata?.mrn}
			</p>
			{#if result.description}
				<p class="mt-1 line-clamp-1 text-xs text-gray-600 dark:text-gray-400">
					{result.description}
				</p>
			{/if}
		</div>
	</div>
{/snippet}

<div class="w-full px-4 pt-6 sm:px-6 lg:px-8">
	<header class="mb-6">
		<a
			href={resolve('/products')}
			class="inline-flex items-center gap-1 text-sm text-gray-500 hover:text-gray-800 dark:text-gray-400 dark:hover:text-gray-200"
		>
			<IconifyIcon icon="material-symbols:arrow-back" class="h-4 w-4" />
			{m.products_back_to_products()}
		</a>
		<h1 class="mt-3 text-2xl font-semibold tracking-tight text-gray-900 dark:text-gray-100">
			{pageTitle}
		</h1>
		<p class="mt-1 text-sm text-gray-600 dark:text-gray-400">{pageDescription}</p>
	</header>

	<div>
		<div class="grid grid-cols-1 gap-6 lg:grid-cols-3 lg:items-start">
			<div class="space-y-6 lg:col-span-2">
				<!-- Basic information -->
				<section
					class="rounded-xl border border-gray-200 bg-white p-6 dark:border-gray-700 dark:bg-gray-800"
				>
					<h2 class="text-sm font-semibold text-gray-900 dark:text-gray-100">
						{m.products_basic_information()}
					</h2>
					<p class="mt-1 text-sm text-gray-600 dark:text-gray-400">
						{m.products_section_basic_hint()}
					</p>

					<div class="mt-5 space-y-5">
						<div>
							<label
								for="product-name"
								class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300"
							>
								{m.common_name()} <span class="text-red-500">*</span>
							</label>
							<input
								id="product-name"
								type="text"
								bind:value={name}
								bind:this={nameInput}
								onkeydown={(e) => {
									if (e.key === 'Enter') handleSave();
								}}
								placeholder={m.products_name_placeholder()}
								aria-invalid={showNameError && !canSave}
								aria-describedby={showNameError && !canSave ? 'product-name-error' : undefined}
								class="w-full rounded-lg border bg-white px-4 py-2.5 text-gray-900 transition-all focus:border-transparent focus:ring-2 focus:ring-earthy-terracotta-600 dark:bg-gray-700 dark:text-gray-100 {showNameError &&
								!canSave
									? 'border-red-400 dark:border-red-500'
									: 'border-gray-300 dark:border-gray-600'}"
								required
							/>
							{#if showNameError && !canSave}
								<p id="product-name-error" class="mt-1.5 text-sm text-red-600 dark:text-red-400">
									{m.products_name_required()}
								</p>
							{/if}
						</div>

						<div>
							<label
								for="product-description"
								class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300"
							>
								{m.common_description()}
							</label>
							<textarea
								id="product-description"
								bind:value={description}
								placeholder={m.products_description_placeholder()}
								rows="2"
								class="w-full resize-none rounded-lg border border-gray-300 bg-white px-4 py-2.5 text-gray-900 transition-all focus:border-transparent focus:ring-2 focus:ring-earthy-terracotta-600 dark:border-gray-600 dark:bg-gray-700 dark:text-gray-100"
							></textarea>
						</div>

						<div class="grid grid-cols-1 gap-5 md:grid-cols-2">
							<div>
								<span class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
									{m.common_tags()}
								</span>
								<Tags bind:tags canEdit={true} />
							</div>

							<div>
								<span class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
									{m.common_owners()}
								</span>
								<OwnerSelector bind:selectedOwners={owners} />
							</div>
						</div>
					</div>
				</section>

				<!-- Manual assets -->
				<section
					class="rounded-xl border border-gray-200 bg-white p-6 dark:border-gray-700 dark:bg-gray-800"
				>
					<h2 class="text-sm font-semibold text-gray-900 dark:text-gray-100">
						{m.products_manual_assets_heading()}
						<span class="ml-1 text-xs font-normal text-gray-500"
							>{m.products_optional_suffix()}</span
						>
					</h2>
					<p class="mt-1 text-sm text-gray-600 dark:text-gray-400">
						{m.products_section_assets_hint()}
					</p>

					{#if mode === 'edit' && resolvedAssets.dynamic.length > 0}
						<div
							class="mt-4 rounded-lg border border-blue-200 bg-blue-50 p-3 dark:border-blue-800/50 dark:bg-blue-900/20"
						>
							<div class="flex items-center gap-2 text-sm text-blue-700 dark:text-blue-300">
								<IconifyIcon icon="material-symbols:auto-awesome" class="h-4 w-4" />
								<span>
									{m.products_assets_matched_by_rules({ count: resolvedAssets.dynamic.length })}
								</span>
							</div>
						</div>
					{/if}

					<div class="mt-5">
						<div class="relative">
							<IconifyIcon
								icon="material-symbols:search"
								class="absolute left-4 top-1/2 h-5 w-5 -translate-y-1/2 text-gray-400"
							/>
							<input
								type="text"
								value={assetSearchQuery}
								oninput={handleAssetSearchInput}
								placeholder={m.products_form_asset_search_placeholder()}
								aria-label={m.products_form_asset_search_placeholder()}
								class="w-full rounded-lg border border-gray-300 bg-white py-2.5 pl-12 pr-4 text-gray-900 transition-all focus:border-transparent focus:ring-2 focus:ring-earthy-terracotta-600 dark:border-gray-600 dark:bg-gray-700 dark:text-gray-100"
							/>
							{#if isSearchingAssets}
								<div class="absolute right-4 top-1/2 -translate-y-1/2">
									<div
										class="h-5 w-5 animate-spin rounded-full border-b-2 border-earthy-terracotta-700"
									></div>
								</div>
							{/if}
						</div>

						{#if assetSearchResults.length > 0}
							<div
								class="mt-2 max-h-64 overflow-y-auto rounded-lg border border-gray-200 dark:border-gray-700"
							>
								{#each assetSearchResults as asset (asset.id)}
									<button
										type="button"
										onclick={() => addAsset(asset)}
										disabled={manualAssetIds.includes(asset.id)}
										class="flex w-full items-center justify-between border-b border-gray-100 p-3 text-left transition-colors last:border-b-0 hover:bg-gray-50 disabled:cursor-not-allowed disabled:opacity-50 dark:border-gray-700 dark:hover:bg-gray-700/50"
									>
										<div class="flex min-w-0 flex-1 items-center gap-3">
											<AssetIcon name={getIconType(asset)} size="sm" showLabel={false} />
											<div class="min-w-0 flex-1">
												<div class="truncate text-sm font-medium text-gray-900 dark:text-gray-100">
													{asset.name}
												</div>
												<div class="truncate font-mono text-xs text-gray-500 dark:text-gray-400">
													{asset.mrn}
												</div>
											</div>
										</div>
										{#if manualAssetIds.includes(asset.id)}
											<span class="text-xs text-green-600 dark:text-green-400">
												{m.products_asset_added_badge()}
											</span>
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

					<div class="mt-5">
						{#if manualAssetIds.length > 0}
							<h3 class="mb-3 text-sm font-medium text-gray-700 dark:text-gray-300">
								{mode === 'edit'
									? m.products_manual_assets_count({ count: manualAssetIds.length })
									: m.products_selected_assets_count({ count: manualAssetIds.length })}
							</h3>
							<ul class="space-y-2">
								{#each manualAssetIds as assetId (assetId)}
									{@const asset = assetDetails.get(assetId)}
									<li
										class="flex items-center justify-between rounded-lg bg-gray-50 p-3 dark:bg-gray-700/50"
									>
										<div class="flex min-w-0 flex-1 items-center gap-3">
											{#if asset}
												<AssetIcon name={getIconType(asset)} size="sm" showLabel={false} />
												<div class="min-w-0 flex-1">
													<div
														class="truncate text-sm font-medium text-gray-900 dark:text-gray-100"
													>
														{asset.name}
													</div>
													<div class="truncate font-mono text-xs text-gray-500 dark:text-gray-400">
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
											class="rounded p-1.5 text-red-600 transition-colors hover:bg-red-50 dark:hover:bg-red-900/20"
											title={m.products_remove_from_product()}
											aria-label={m.products_remove_from_product()}
										>
											<IconifyIcon icon="material-symbols:close" class="h-4 w-4" />
										</button>
									</li>
								{/each}
							</ul>
						{:else}
							<div
								class="rounded-lg border-2 border-dashed border-gray-200 py-6 text-center text-gray-500 dark:border-gray-700 dark:text-gray-400"
							>
								<p class="text-sm font-medium">
									{mode === 'edit'
										? m.products_no_manual_assets_added()
										: m.products_no_assets_added_yet()}
								</p>
								<p class="mt-1 text-sm">
									{mode === 'edit'
										? m.products_no_manual_assets_hint_edit()
										: m.products_section_assets_empty_hint()}
								</p>
							</div>
						{/if}
					</div>
				</section>

				<!-- Dynamic rules -->
				<section
					class="rounded-xl border border-gray-200 bg-white p-6 dark:border-gray-700 dark:bg-gray-800"
				>
					<div class="flex items-start justify-between gap-4">
						<div>
							<h2 class="text-sm font-semibold text-gray-900 dark:text-gray-100">
								{m.products_dynamic_rules()}
								<span class="ml-1 text-xs font-normal text-gray-500"
									>{m.products_optional_suffix()}</span
								>
							</h2>
							<p class="mt-1 text-sm text-gray-600 dark:text-gray-400">
								{m.products_dynamic_rules_hint()}
							</p>
						</div>
						{#if !showRuleForm && rules.length > 0}
							<Button
								variant="clear"
								click={() => openRuleForm()}
								icon="material-symbols:add"
								text={m.products_add_rule()}
							/>
						{/if}
					</div>

					<div class="mt-5">
						{#if showRuleForm}
							<div class="overflow-hidden rounded-lg border border-gray-200 dark:border-gray-700">
								<div
									class="border-b border-gray-200 bg-gray-50 px-4 py-3 dark:border-gray-700 dark:bg-gray-800/50"
								>
									<h3 class="text-sm font-semibold text-gray-900 dark:text-gray-100">
										{editingRuleIndex !== null
											? m.products_edit_rule()
											: m.products_create_new_rule()}
									</h3>
								</div>

								<div class="space-y-6 p-4">
									<div class="grid grid-cols-1 gap-4 md:grid-cols-2">
										<div>
											<label
												for="rule-name"
												class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300"
											>
												{m.products_rule_name_field_label()} <span class="text-red-500">*</span>
											</label>
											<input
												id="rule-name"
												type="text"
												bind:value={ruleForm.name}
												placeholder={m.products_rule_name_input_placeholder()}
												class="w-full rounded-lg border border-gray-300 bg-white px-4 py-2.5 text-gray-900 transition-all focus:border-transparent focus:ring-2 focus:ring-earthy-terracotta-600 dark:border-gray-600 dark:bg-gray-700 dark:text-gray-100"
											/>
										</div>
										<div>
											<label
												for="rule-description"
												class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300"
											>
												{m.common_description()}
											</label>
											<input
												id="rule-description"
												type="text"
												bind:value={ruleForm.description}
												placeholder={m.products_rule_description_input_placeholder()}
												class="w-full rounded-lg border border-gray-300 bg-white px-4 py-2.5 text-gray-900 transition-all focus:border-transparent focus:ring-2 focus:ring-earthy-terracotta-600 dark:border-gray-600 dark:bg-gray-700 dark:text-gray-100"
											/>
										</div>
									</div>

									<div>
										<span class="mb-3 block text-sm font-medium text-gray-700 dark:text-gray-300">
											{m.products_query_expression_field_label()}
										</span>
										<QueryBuilder
											query={ruleForm.query_expression || ''}
											onQueryChange={(q) => (ruleForm.query_expression = q)}
											initiallyExpanded={true}
											showRunButton={true}
											runButtonText={isPreviewLoading
												? m.products_searching()
												: m.products_preview()}
											runButtonIcon={isPreviewLoading
												? 'mdi:loading'
												: 'material-symbols:visibility'}
											onRunClick={(q) => previewRule(q)}
										/>
									</div>

									{#if isPreviewLoading}
										<div class="space-y-2">
											{#each Array(3) as _, i (i)}
												<div
													class="animate-pulse rounded-lg border border-gray-200 bg-white p-3 dark:border-gray-700 dark:bg-gray-800"
												>
													<div class="flex items-center gap-2">
														<div class="h-6 w-6 rounded bg-gray-200 dark:bg-gray-600"></div>
														<div class="flex-1 space-y-1.5">
															<div class="h-4 w-48 rounded bg-gray-200 dark:bg-gray-600"></div>
															<div class="h-3 w-64 rounded bg-gray-200 dark:bg-gray-600"></div>
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
													<span class="font-medium">
														{m.products_asset_matched_count({ count: rulePreviewTotal })}
													</span>
												</div>
												{#if rulePreviewTotal > rulePreviewResults.length}
													<span class="text-xs text-gray-500 dark:text-gray-400">
														{m.products_showing_first({ count: rulePreviewResults.length })}
													</span>
												{/if}
											</div>
											<div class="max-h-80 space-y-2 overflow-y-auto">
												{#each rulePreviewResults as result, i (result.metadata?.mrn ?? i)}
													{@const params = getPreviewAssetParams(result)}
													{#if params}
														<a
															href={resolve('/discover/[type]/[service]/[name]', params)}
															target="_blank"
															rel="noopener"
															class="group block rounded-lg border border-gray-200 bg-white p-3 transition-all hover:border-earthy-terracotta-300 hover:shadow-md dark:border-gray-700 dark:bg-gray-800 dark:hover:border-earthy-terracotta-700"
														>
															{@render previewResultBody(result)}
														</a>
													{:else}
														<div
															class="rounded-lg border border-gray-200 bg-white p-3 dark:border-gray-700 dark:bg-gray-800"
														>
															{@render previewResultBody(result)}
														</div>
													{/if}
												{/each}
											</div>
										</div>
									{:else}
										<div class="py-6 text-center text-gray-500 dark:text-gray-400">
											<IconifyIcon
												icon="material-symbols:search"
												class="mx-auto mb-2 h-10 w-10 opacity-40"
											/>
											<p class="text-sm">{m.products_preview_hint()}</p>
										</div>
									{/if}

									<div
										class="flex items-center justify-end gap-3 border-t border-gray-200 pt-4 dark:border-gray-700"
									>
										<Button variant="clear" click={closeRuleForm} text={m.common_cancel()} />
										<Button
											variant="filled"
											click={saveRule}
											text={editingRuleIndex !== null
												? m.products_update_rule()
												: m.products_add_rule()}
											icon="material-symbols:check"
											disabled={!ruleForm.name.trim()}
										/>
									</div>
								</div>
							</div>
						{:else if rules.length > 0}
							<ul class="space-y-3">
								{#each rules as rule, index (rule.id ?? index)}
									<li
										class="rounded-lg border border-gray-200 p-4 transition-shadow hover:shadow-md dark:border-gray-700"
									>
										<div class="flex items-start justify-between">
											<div class="min-w-0 flex-1">
												<div class="flex items-center gap-3">
													<div
														class="flex h-8 w-8 items-center justify-center rounded-lg bg-blue-100 text-blue-600 dark:bg-blue-900/30 dark:text-blue-400"
													>
														<IconifyIcon icon="mdi:database-search" class="h-4 w-4" />
													</div>
													<div>
														<span class="text-sm font-semibold text-gray-900 dark:text-gray-100">
															{rule.name}
														</span>
														{#if rule.description}
															<p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
																{rule.description}
															</p>
														{/if}
													</div>
												</div>
												{#if rule.query_expression}
													<div
														class="mt-3 truncate rounded-lg bg-gray-50 px-3 py-2 font-mono text-xs text-gray-600 dark:bg-gray-700/50 dark:text-gray-400"
													>
														{rule.query_expression}
													</div>
												{/if}
											</div>
											<div class="ml-4 flex items-center gap-1">
												<button
													type="button"
													onclick={() => openRuleForm(index)}
													class="rounded-lg p-2 text-gray-500 transition-colors hover:bg-gray-100 hover:text-gray-700 dark:hover:bg-gray-700"
													title={m.products_edit_rule_tooltip()}
													aria-label={m.products_edit_rule_tooltip()}
												>
													<IconifyIcon icon="material-symbols:edit" class="h-4 w-4" />
												</button>
												<button
													type="button"
													onclick={() => deleteRule(index)}
													class="rounded-lg p-2 text-gray-500 transition-colors hover:bg-red-50 hover:text-red-600 dark:hover:bg-red-900/20"
													title={m.products_delete_rule_tooltip()}
													aria-label={m.products_delete_rule_tooltip()}
												>
													<IconifyIcon icon="material-symbols:delete" class="h-4 w-4" />
												</button>
											</div>
										</div>
									</li>
								{/each}
							</ul>
						{:else}
							<button
								type="button"
								onclick={() => openRuleForm()}
								class="group w-full rounded-lg border-2 border-dashed border-gray-300 p-6 transition-all hover:border-earthy-terracotta-500 hover:bg-earthy-terracotta-50 dark:border-gray-600 dark:hover:bg-earthy-terracotta-900/10"
							>
								<div class="text-center">
									<div
										class="mx-auto flex h-10 w-10 items-center justify-center rounded-full bg-gray-100 transition-colors group-hover:bg-earthy-terracotta-100 dark:bg-gray-700 dark:group-hover:bg-earthy-terracotta-900/30"
									>
										<IconifyIcon
											icon="material-symbols:add"
											class="h-5 w-5 text-gray-400 transition-colors group-hover:text-earthy-terracotta-600"
										/>
									</div>
									<p
										class="mt-3 text-sm font-medium text-gray-600 group-hover:text-earthy-terracotta-700 dark:text-gray-400 dark:group-hover:text-earthy-terracotta-400"
									>
										{m.products_add_first_rule_prompt()}
									</p>
									<p class="mt-1 text-xs text-gray-500 dark:text-gray-500">
										{m.products_add_first_rule_hint()}
									</p>
								</div>
							</button>
						{/if}
					</div>
				</section>

				<!-- Documentation and metadata -->
				<section
					class="rounded-xl border border-gray-200 bg-white p-6 dark:border-gray-700 dark:bg-gray-800"
				>
					<h2 class="text-sm font-semibold text-gray-900 dark:text-gray-100">
						{m.common_details()}
						<span class="ml-1 text-xs font-normal text-gray-500"
							>{m.products_optional_suffix()}</span
						>
					</h2>
					<p class="mt-1 text-sm text-gray-600 dark:text-gray-400">
						{m.products_section_details_hint()}
					</p>

					<div class="mt-5 space-y-5">
						<div>
							<span class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300">
								{m.common_documentation()}
							</span>
							<p class="mb-2 text-xs text-gray-500 dark:text-gray-400">
								{m.products_documentation_hint()}
							</p>
							<RichTextEditor
								bind:value={documentation}
								placeholder={m.products_documentation_placeholder()}
							/>
						</div>

						<MetadataView bind:metadata readOnly={false} maxDepth={2} />
					</div>
				</section>
			</div>

			<div class="lg:sticky lg:top-6">
				<DataProductSummary
					{mode}
					{name}
					{description}
					{owners}
					{tags}
					{manualAssetIds}
					{assetDetails}
					{rules}
				/>
			</div>
		</div>

		<div
			class="sticky bottom-0 z-10 -mx-4 mt-8 border-t border-gray-200 bg-white/90 px-4 py-3 backdrop-blur sm:-mx-6 sm:px-6 lg:-mx-8 lg:px-8 dark:border-gray-700 dark:bg-gray-900/90"
		>
			<div class="flex flex-wrap items-center justify-end gap-3">
				{#if error}
					<p
						role="alert"
						class="mr-auto inline-flex items-center gap-1.5 text-sm text-red-600 dark:text-red-400"
					>
						<IconifyIcon icon="material-symbols:error" class="h-4 w-4 flex-shrink-0" />
						{error}
					</p>
				{:else}
					<p class="mr-auto hidden text-sm text-gray-500 sm:block dark:text-gray-400">
						{formatList([
							m.products_manual_asset_count({ count: manualAssetIds.length }),
							m.products_rule_count({ count: rules.length })
						])}
					</p>
				{/if}
				<button
					type="button"
					onclick={() => goto(resolve('/products'))}
					class="rounded-lg px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-gray-800"
				>
					{m.common_cancel()}
				</button>
				<button
					type="button"
					onclick={handleSave}
					disabled={saving}
					class="rounded-lg bg-earthy-terracotta-600 px-4 py-2 text-sm font-semibold text-white hover:bg-earthy-terracotta-500 disabled:opacity-50"
				>
					{saveButtonText}
				</button>
			</div>
		</div>
	</div>
</div>

<ConfirmModal
	show={unsavedGuard.showConfirm}
	title={m.common_discard_prompt()}
	message={m.products_unsaved_changes_confirm()}
	confirmText={m.common_discard()}
	cancelText={m.common_keep_editing()}
	variant="warning"
	onConfirm={unsavedGuard.confirmDiscard}
	onCancel={unsavedGuard.cancelDiscard}
/>
