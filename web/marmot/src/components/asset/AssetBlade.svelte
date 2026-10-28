<script lang="ts">
	import { fetchApi } from '$lib/api';
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import type { Asset } from '$lib/assets/types';
	import type { LineageResponse } from '$lib/lineage/types';
	import Button from '$components/ui/Button.svelte';
	import Blade from '$components/ui/Blade.svelte';
	import LineageViewNode from '$components/lineage/LineageViewNode.svelte';
	import Icon from '$components/ui/Icon.svelte';
	import RunHistory from '$components/runs/RunHistory.svelte';
	import Tags from '$components/shared/Tags.svelte';
	import AssetGlossaryTerms from './AssetGlossaryTerms.svelte';
	import AssetDescriptions from './AssetDescriptions.svelte';
	import OwnerSelector from '$components/shared/OwnerSelector.svelte';
	import IconifyIcon from '@iconify/svelte';
	import { auth } from '$lib/stores/auth';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { SvelteSet } from 'svelte/reactivity';
	import { m } from '$lib/paraglide/messages';
	import { formatDateTime } from '$lib/utils';

	interface Owner {
		id: string;
		name: string;
		type: 'user' | 'team';
		username?: string;
		email?: string;
		profile_picture?: string;
	}

	// assetUrl lets a caller hand us an already-built discover URL; it is expected to be a
	// `/discover/<type>/<service>/<name>` path, matching the shape resolve() needs below.
	type DiscoverAssetRoute = `/discover/${string}/${string}/${string}`;

	interface Props {
		asset?: Asset | null;
		lineage?: LineageResponse | null;
		onClose: () => void;
		staticPlacement?: boolean;
		collapsed?: boolean;
		onToggleCollapse?: () => void;
		assetUrl?: string;
	}

	let {
		asset = $bindable(null),
		lineage = null,
		onClose,
		staticPlacement = false,
		collapsed = false,
		onToggleCollapse,
		assetUrl
	}: Props = $props();

	let currentAssetId: string | null = null;
	let mounted = false;
	let showDeleteModal = $state(false);
	let isDeleting = $state(false);
	let deleteError = $state('');

	const canManageAssets = auth.hasPermission('assets', 'manage');

	let isRunHistoryTab = $derived(page.url.searchParams.get('tab') === 'run-history');
	let shouldHideRunHistory = $derived(staticPlacement && isRunHistoryTab);

	function handleAssetChanged() {
		currentAssetId = asset?.id || null;
		lineage = null;
		expandedAssets.clear();
		showAllUpstream = false;
		showAllDownstream = false;
		owners = [];
		if (asset?.id) {
			fetchOwners();
		}
	}

	let currentIconName = $derived(
		asset
			? Array.isArray(asset.providers) && asset.providers.length === 1
				? asset.providers[0]
				: asset.type
			: ''
	);

	let isVisible = $derived(asset != null);

	// Parse the MRN to extract service and full name: mrn://type/service/full.qualified.name
	const assetRouteParams = $derived.by(() => {
		if (!asset?.mrn || !asset?.type) return null;
		const mrnParts = asset.mrn.replace('mrn://', '').split('/');
		if (mrnParts.length < 3) return null;
		return {
			type: asset.type.toLowerCase(),
			service: mrnParts[1],
			name: encodeURIComponent(mrnParts.slice(2).join('/'))
		};
	});

	let fullViewUrl = $derived(
		assetUrl ??
			(assetRouteParams
				? `/discover/${assetRouteParams.type}/${assetRouteParams.service}/${assetRouteParams.name}`
				: '')
	);

	let loadingLineage = $state(false);
	let lineageError = $state<string | null>(null);
	const expandedAssets = new SvelteSet<string>();
	let showAllUpstream = $state(false);
	let showAllDownstream = $state(false);
	const LINEAGE_COLLAPSED_LIMIT = 5;
	let owners = $state<Owner[]>([]);
	let loadingOwners = $state(false);

	function toggleAssetExpansion(id: string) {
		if (expandedAssets.has(id)) {
			expandedAssets.delete(id);
		} else {
			expandedAssets.add(id);
		}
	}

	async function fetchLineage() {
		if (!asset?.id || loadingLineage) return;

		loadingLineage = true;
		try {
			const response = await fetchApi(`/lineage/assets/${asset.id}?depth=1`);
			if (!response.ok) throw new Error(m.asset_lineage_load_error());
			lineage = await response.json();
		} catch (error) {
			lineageError = error instanceof Error ? error.message : m.asset_lineage_load_error();
		} finally {
			loadingLineage = false;
		}
	}

	async function fetchOwners() {
		if (!asset?.id) return;

		loadingOwners = true;
		try {
			const response = await fetchApi(`/assets/owners/?asset_id=${asset.id}`);
			if (!response.ok) throw new Error('Failed to fetch owners');
			const data = await response.json();
			owners = data.owners || [];
		} catch (error) {
			console.error('Failed to fetch owners:', error);
			owners = [];
		} finally {
			loadingOwners = false;
		}
	}

	let filteredLineage = $derived(
		lineage
			? {
					...lineage,
					nodes: lineage.nodes.filter((node) => !node.asset?.is_stub)
				}
			: null
	);

	let hasNonCurrentNodes = $derived(
		filteredLineage?.nodes && filteredLineage.nodes.filter((n) => n.depth !== 0).length > 0
	);

	let hasDescriptionsOrTerms = $derived(
		asset?.description || asset?.user_description || staticPlacement
	);

	onMount(() => {
		mounted = true;
		if (asset?.id) {
			fetchLineage();
			fetchOwners();
		}
	});

	$effect(() => {
		if (asset?.id !== currentAssetId) {
			handleAssetChanged();
		}
		if (mounted && asset?.id && !lineage && !loadingLineage) {
			fetchLineage();
		}
	});

	async function handleDelete() {
		if (!asset?.id) return;

		isDeleting = true;
		deleteError = '';

		try {
			const response = await fetchApi(`/assets/${asset.id}`, {
				method: 'DELETE'
			});

			if (!response.ok) {
				const errorData = await response.json();
				throw new Error(errorData.error || m.asset_delete_error());
			}

			showDeleteModal = false;

			// Close the blade and redirect to assets list
			if (staticPlacement) {
				goto(resolve('/discover'));
			} else {
				onClose();
				// Trigger a page refresh to update the asset list
				window.location.reload();
			}
		} catch (err) {
			deleteError = err instanceof Error ? err.message : m.asset_delete_error();
		} finally {
			isDeleting = false;
		}
	}
</script>

{#if isVisible && asset}
	<Blade
		title={m.asset_blade_details_heading()}
		{onClose}
		{staticPlacement}
		{collapsed}
		{onToggleCollapse}
		footer={canManageAssets ? deleteFooter : undefined}
	>
		{#snippet actions()}
			<Button
				icon="material-symbols:screenshot-monitor-outline"
				href={fullViewUrl}
				text={m.asset_blade_full_view()}
				variant="filled"
			/>
		{/snippet}

		<div class="space-y-4">
			<!-- Asset Header - only show in non-static mode -->
			{#if !staticPlacement}
				<a
					href={assetUrl
						? resolve(assetUrl as DiscoverAssetRoute)
						: assetRouteParams
							? resolve('/discover/[type]/[service]/[name]', assetRouteParams)
							: '#'}
					class="block bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-5 hover:shadow-md hover:border-earthy-terracotta-300 dark:hover:border-earthy-terracotta-700 transition-all"
				>
					<div class="flex items-start gap-3 mb-3">
						<div class="flex-shrink-0">
							<Icon name={currentIconName} size="md" />
						</div>
						<div class="flex-1 min-w-0">
							<h3 class="font-semibold text-base text-gray-900 dark:text-gray-100 truncate">
								{asset.name || ''}
							</h3>
							<p class="text-xs text-gray-500 dark:text-gray-400 truncate font-mono mt-0.5">
								{asset.mrn || ''}
							</p>
						</div>
					</div>
					<div class="space-y-4">
						<div>
							<div class="flex items-center gap-2 mb-2">
								<IconifyIcon
									icon="material-symbols:label-outline"
									class="w-4 h-4 text-gray-500 dark:text-gray-400"
								/>
								<h4
									class="text-xs font-semibold text-gray-500 dark:text-gray-400 uppercase tracking-wider"
								>
									{m.common_tags()}
								</h4>
							</div>
							<Tags tags={asset.tags ?? []} endpoint="/assets" id={asset.id} canEdit={false} />
						</div>
						<div>
							<div class="flex items-center gap-2 mb-2">
								<IconifyIcon
									icon="material-symbols:person-outline"
									class="w-4 h-4 text-gray-500 dark:text-gray-400"
								/>
								<h4
									class="text-xs font-semibold text-gray-500 dark:text-gray-400 uppercase tracking-wider"
								>
									{m.common_owners()}
								</h4>
							</div>
							{#if loadingOwners}
								<div class="flex items-center justify-center py-4">
									<div
										class="animate-spin h-5 w-5 border-b-2 border-earthy-terracotta-700 rounded-full"
									></div>
								</div>
							{:else}
								<OwnerSelector selectedOwners={owners} onChange={() => {}} disabled={true} />
							{/if}
						</div>
					</div>
				</a>
			{/if}

			<!-- Descriptions and Glossary Terms - hide user description editing in static mode (now in page header) -->
			{#if !staticPlacement && hasDescriptionsOrTerms}
				<div
					class="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-5"
				>
					<div class="space-y-5">
						<AssetDescriptions {asset} editable={false} />
						<AssetGlossaryTerms {asset} editable={false} />
					</div>
				</div>
			{:else if staticPlacement}
				<!-- In static mode, only show technical description and glossary if present -->
				{#if asset.description}
					<div
						class="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-5"
					>
						<div class="flex items-center gap-2 mb-2">
							<h3 class="text-base font-semibold text-gray-900 dark:text-gray-100">
								{m.asset_technical_description()}
							</h3>
							<div
								class="flex items-center justify-center w-5 h-5 rounded-full bg-purple-100 dark:bg-purple-900/30"
								title={m.asset_generated_by_plugin_title()}
							>
								<svg
									class="w-3 h-3 text-purple-700 dark:text-purple-300"
									fill="currentColor"
									viewBox="0 0 20 20"
								>
									<path
										fill-rule="evenodd"
										d="M11.3 1.046A1 1 0 0112 2v5h4a1 1 0 01.82 1.573l-7 10A1 1 0 018 18v-5H4a1 1 0 01-.82-1.573l7-10a1 1 0 011.12-.38z"
										clip-rule="evenodd"
									/>
								</svg>
							</div>
						</div>
						<p class="text-sm text-gray-600 dark:text-gray-400 whitespace-pre-wrap leading-relaxed">
							{asset.description}
						</p>
					</div>
				{/if}
				<AssetGlossaryTerms {asset} editable={true} />
			{/if}

			<!-- Run History -->
			{#if !shouldHideRunHistory && asset.has_run_history}
				<div
					class="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-5"
				>
					<h3 class="text-base font-semibold text-gray-900 dark:text-gray-100 mb-3">
						{m.asset_run_history_heading()}
					</h3>
					<RunHistory assetId={asset.id} minimal={true} {asset} />
				</div>
			{/if}

			<!-- Data Lineage -->
			{#if hasNonCurrentNodes}
				<div
					class="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-5"
				>
					<h3 class="text-base font-semibold text-gray-900 dark:text-gray-100 mb-3">
						{m.asset_data_lineage_heading()}
					</h3>

					{#if loadingLineage}
						<div class="flex items-center justify-center py-8">
							<div
								class="animate-spin h-6 w-6 border-b-2 border-earthy-terracotta-700 rounded-full"
							></div>
						</div>
					{:else if lineageError}
						<div class="text-sm text-red-600 dark:text-red-400">{lineageError}</div>
					{:else if lineage}
						{@const upstreamNodes = filteredLineage?.nodes.filter((n) => n.depth < 0) ?? []}
						{@const downstreamNodes = filteredLineage?.nodes.filter((n) => n.depth > 0) ?? []}
						{@const visibleUpstream = showAllUpstream
							? upstreamNodes
							: upstreamNodes.slice(0, LINEAGE_COLLAPSED_LIMIT)}
						{@const visibleDownstream = showAllDownstream
							? downstreamNodes
							: downstreamNodes.slice(0, LINEAGE_COLLAPSED_LIMIT)}
						{#if upstreamNodes.length > 0}
							<div class="mb-4">
								<h4
									class="text-xs font-semibold text-gray-600 dark:text-gray-400 uppercase tracking-wider mb-2"
								>
									{m.asset_lineage_upstream()}
								</h4>
								<div class="space-y-2">
									{#each visibleUpstream as node (node.id)}
										<LineageViewNode
											{node}
											expanded={expandedAssets.has(node.id)}
											onClick={() => toggleAssetExpansion(node.id)}
											maxMetadataDepth={0}
											compact={true}
										/>
									{/each}
								</div>
								{#if upstreamNodes.length > LINEAGE_COLLAPSED_LIMIT}
									<button
										type="button"
										onclick={() => (showAllUpstream = !showAllUpstream)}
										class="mt-2 text-xs font-medium text-earthy-terracotta-700 dark:text-earthy-terracotta-400 hover:underline focus:outline-none"
									>
										{showAllUpstream
											? m.asset_show_less()
											: m.asset_show_more_count({
													count: upstreamNodes.length - LINEAGE_COLLAPSED_LIMIT
												})}
									</button>
								{/if}
							</div>
						{/if}

						{#if downstreamNodes.length > 0}
							<div>
								<h4
									class="text-xs font-semibold text-gray-600 dark:text-gray-400 uppercase tracking-wider mb-2"
								>
									{m.asset_lineage_downstream()}
								</h4>
								<div class="space-y-2">
									{#each visibleDownstream as node (node.id)}
										<LineageViewNode
											{node}
											expanded={expandedAssets.has(node.id)}
											onClick={() => toggleAssetExpansion(node.id)}
											maxMetadataDepth={0}
											compact={true}
										/>
									{/each}
								</div>
								{#if downstreamNodes.length > LINEAGE_COLLAPSED_LIMIT}
									<button
										type="button"
										onclick={() => (showAllDownstream = !showAllDownstream)}
										class="mt-2 text-xs font-medium text-earthy-terracotta-700 dark:text-earthy-terracotta-400 hover:underline focus:outline-none"
									>
										{showAllDownstream
											? m.asset_show_less()
											: m.asset_show_more_count({
													count: downstreamNodes.length - LINEAGE_COLLAPSED_LIMIT
												})}
									</button>
								{/if}
							</div>
						{/if}
					{/if}
				</div>
			{/if}

			<!-- Additional Details -->
			<div
				class="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-5"
			>
				<h4 class="text-base font-semibold text-gray-900 dark:text-gray-100 mb-3">
					{m.common_details()}
				</h4>
				<dl class="space-y-3">
					<div>
						<dt class="text-xs text-gray-500 dark:text-gray-400">{m.asset_created_by()}</dt>
						<dd class="text-sm text-gray-900 dark:text-gray-100 mt-0.5">
							{asset.created_by || m.common_unknown()}
						</dd>
					</div>
					<div>
						<dt class="text-xs text-gray-500 dark:text-gray-400">{m.asset_created_at()}</dt>
						<dd class="text-sm text-gray-900 dark:text-gray-100 mt-0.5">
							{asset.created_at ? formatDateTime(asset.created_at) : m.common_unknown()}
						</dd>
					</div>
					<div>
						<dt class="text-xs text-gray-500 dark:text-gray-400">{m.asset_last_updated()}</dt>
						<dd class="text-sm text-gray-900 dark:text-gray-100 mt-0.5">
							{asset.updated_at ? formatDateTime(asset.updated_at) : m.common_unknown()}
						</dd>
					</div>
					{#if asset.parent_mrn}
						<div>
							<dt class="text-xs text-gray-500 dark:text-gray-400">
								{m.asset_parent_asset()}
							</dt>
							<dd
								class="text-sm text-gray-900 dark:text-gray-100 mt-0.5 font-mono text-xs break-all"
							>
								{asset.parent_mrn}
							</dd>
						</div>
					{/if}
				</dl>
			</div>
		</div>
	</Blade>
{/if}

{#snippet deleteFooter()}
	<div class="flex justify-end">
		<button
			onclick={() => (showDeleteModal = true)}
			class="inline-flex items-center gap-1.5 px-3 py-2 text-sm font-medium text-red-600 dark:text-red-400 hover:text-red-700 dark:hover:text-red-300 hover:bg-red-50 dark:hover:bg-red-950/30 rounded-lg transition-colors"
		>
			<IconifyIcon icon="material-symbols:delete-outline-rounded" class="w-4 h-4" />
			{m.asset_delete_asset()}
		</button>
	</div>
{/snippet}

<!-- Delete Confirmation Modal -->
{#if showDeleteModal && asset}
	<div
		class="fixed inset-0 bg-black/50 dark:bg-black/70 backdrop-blur-sm z-50 flex items-center justify-center px-4"
		onclick={() => !isDeleting && (showDeleteModal = false)}
		onkeydown={(e) => e.key === 'Escape' && !isDeleting && (showDeleteModal = false)}
		role="button"
		tabindex="-1"
	>
		<div
			class="bg-white dark:bg-gray-800 rounded-xl shadow-2xl max-w-md w-full border border-gray-200 dark:border-gray-700"
			onclick={(e) => e.stopPropagation()}
			onkeydown={(e) => e.stopPropagation()}
			role="dialog"
			tabindex="-1"
		>
			<div class="p-6">
				<div class="flex items-start gap-4">
					<div class="flex-shrink-0">
						<IconifyIcon
							icon="material-symbols:warning-rounded"
							class="w-12 h-12 text-red-600 dark:text-red-400"
						/>
					</div>
					<div class="flex-1">
						<h3 class="text-lg font-semibold text-gray-900 dark:text-gray-100 mb-2">
							{m.asset_delete_asset()}
						</h3>
						<p class="text-sm text-gray-600 dark:text-gray-400 mb-4">
							{m.asset_delete_confirm_message({ name: asset.name })}
						</p>
						{#if deleteError}
							<div
								class="mb-4 p-3 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-lg"
							>
								<p class="text-sm text-red-800 dark:text-red-200">{deleteError}</p>
							</div>
						{/if}
						<div class="flex gap-3 justify-end">
							<button
								onclick={() => (showDeleteModal = false)}
								disabled={isDeleting}
								class="px-4 py-2 text-sm font-medium text-gray-700 dark:text-gray-300 bg-white dark:bg-gray-700 border border-gray-300 dark:border-gray-600 rounded-lg hover:bg-gray-50 dark:hover:bg-gray-600 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
							>
								{m.common_cancel()}
							</button>
							<button
								onclick={handleDelete}
								disabled={isDeleting}
								class="inline-flex items-center gap-2 px-4 py-2 text-sm font-medium text-white bg-red-600 hover:bg-red-700 dark:bg-red-500 dark:hover:bg-red-600 rounded-lg disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
							>
								{#if isDeleting}
									<div class="animate-spin rounded-full h-4 w-4 border-b-2 border-white"></div>
									{m.asset_deleting_progress()}
								{:else}
									<IconifyIcon icon="material-symbols:delete-outline" class="w-5 h-5" />
									{m.asset_delete_asset()}
								{/if}
							</button>
						</div>
					</div>
				</div>
			</div>
		</div>
	</div>
{/if}
