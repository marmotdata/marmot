<script lang="ts">
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { fetchApi } from '$lib/api';
	import type { Asset } from '$lib/assets/types';
	import AssetBlade from '$components/asset/AssetBlade.svelte';
	import Button from '$components/ui/Button.svelte';
	import DocumentationSystem from '$components/docs/DocumentationSystem.svelte';
	import AssetSources from '$components/asset/AssetSources.svelte';
	import MetadataView from '$components/shared/MetadataView.svelte';
	import Lineage from '$components/lineage/Lineage.svelte';
	import SchemaEditor from '$components/schema/SchemaEditor.svelte';
	import AssetEnvironmentsView from '$components/asset/AssetEnvironmentsView.svelte';
	import RunHistory from '$components/runs/RunHistory.svelte';
	import CodeBlock from '$components/editor/CodeBlock.svelte';
	import Tabs, { type Tab } from '$components/ui/Tabs.svelte';
	import Icon from '$components/ui/Icon.svelte';
	import IconifyIcon from '@iconify/svelte';
	import Tags from '$components/shared/Tags.svelte';
	import OwnerSelector from '$components/shared/OwnerSelector.svelte';
	import { auth } from '$lib/stores/auth';

	interface Owner {
		id: string;
		name: string;
		type: 'user' | 'team';
		username?: string;
		email?: string;
		profile_picture?: string;
	}

	let asset: Asset | null = $state(null);
	let loading = $state(true);
	let error: string | null = $state(null);
	let bladeCollapsed = $state(false);

	let owners: Owner[] = $state([]);
	let loadingOwners = $state(false);

	let userDescription = $state('');
	let isEditingDescription = $state(false);
	let savingDescription = $state(false);

	let canManageAssets = $derived(auth.hasPermission('assets', 'manage'));

	let activeTab = $derived($page.url.searchParams.get('tab') || 'metadata');
	let assetType = $derived($page.params.type);
	let assetService = $derived($page.params.service);
	let assetName = $derived($page.params.name);

	async function fetchAsset() {
		try {
			loading = true;
			error = null;
			const response = await fetchApi(
				`/assets/lookup/${assetType}/${assetService}/${encodeURIComponent(assetName)}`
			);
			if (!response.ok) {
				throw new Error('Failed to fetch asset');
			}
			asset = await response.json();
		} catch (err) {
			console.error('Error fetching asset:', err);
			error = err instanceof Error ? err.message : 'Failed to load asset';
		} finally {
			loading = false;
		}
	}

	function setActiveTab(tab: string) {
		const url = new URL(window.location.href);
		url.searchParams.set('tab', tab);
		goto(url.toString(), { replaceState: true });
	}

	function handleBack() {
		window.history.back();
	}

	function getIconName(asset: Asset): string {
		if (Array.isArray(asset.providers) && asset.providers.length === 1) {
			return asset.providers[0];
		}
		return asset.type || '';
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

	async function handleOwnersChange(newOwners: Owner[]) {
		if (!asset?.id || !canManageAssets) return;

		const currentOwnerKeys = new Set(owners.map((o) => `${o.type}-${o.id}`));
		const newOwnerKeys = new Set(newOwners.map((o) => `${o.type}-${o.id}`));

		const added = newOwners.filter((o) => !currentOwnerKeys.has(`${o.type}-${o.id}`));
		const removed = owners.filter((o) => !newOwnerKeys.has(`${o.type}-${o.id}`));

		try {
			for (const owner of added) {
				const response = await fetchApi(`/assets/owners/?asset_id=${asset.id}`, {
					method: 'POST',
					body: JSON.stringify({ owner_type: owner.type, owner_id: owner.id })
				});
				if (!response.ok) throw new Error('Failed to add owner');
			}

			for (const owner of removed) {
				const response = await fetchApi(
					`/assets/owners/?asset_id=${asset.id}&owner_type=${owner.type}&owner_id=${owner.id}`,
					{ method: 'DELETE' }
				);
				if (!response.ok) throw new Error('Failed to remove owner');
			}

			owners = newOwners;
		} catch (error) {
			console.error('Failed to update owners:', error);
			await fetchOwners();
		}
	}

	function startEditingDescription() {
		userDescription = asset?.user_description || '';
		isEditingDescription = true;
	}

	function cancelEditingDescription() {
		userDescription = asset?.user_description || '';
		isEditingDescription = false;
	}

	async function saveDescription(valueToSave?: string) {
		if (!asset?.id) return;

		const finalValue = valueToSave !== undefined ? valueToSave : userDescription;

		savingDescription = true;
		try {
			const response = await fetchApi(`/assets/${asset.id}`, {
				method: 'PUT',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ user_description: finalValue.trim() })
			});

			if (response.ok) {
				const trimmedValue = finalValue.trim();
				if (trimmedValue) {
					asset.user_description = trimmedValue;
				} else {
					delete asset.user_description;
				}
				userDescription = trimmedValue;
				isEditingDescription = false;
			}
		} catch (error) {
			console.error('Error saving description:', error);
		} finally {
			savingDescription = false;
		}
	}

	const allTabs: Tab[] = [
		{ id: 'documentation', label: 'Documentation', icon: 'material-symbols:description' },
		{ id: 'metadata', label: 'Metadata', icon: 'material-symbols:data-object' },
		{ id: 'query', label: 'Query', icon: 'material-symbols:code' },
		{ id: 'environments', label: 'Environments', icon: 'material-symbols:deployed-code' },
		{ id: 'schema', label: 'Schema', icon: 'material-symbols:table' },
		{ id: 'run-history', label: 'Run History', icon: 'material-symbols:history' },
		{ id: 'lineage', label: 'Lineage', icon: 'material-symbols:account-tree' }
	];

	let visibleTabs = $derived(
		allTabs.filter((tab) => {
			if (
				tab.id === 'environments' &&
				(!asset?.environments || Object.keys(asset.environments).length === 0)
			)
				return false;
			if (tab.id === 'query' && !asset?.query) return false;
			if (tab.id === 'run-history' && !asset?.has_run_history) return false;
			return true;
		})
	);

	$effect(() => {
		if (assetType && assetService && assetName) {
			fetchAsset();
		}
	});

	$effect(() => {
		if (asset?.id) {
			fetchOwners();
			userDescription = asset.user_description || '';
		}
	});
</script>

<div class="h-full flex">
	{#if loading}
		<div class="flex items-center justify-center w-full">
			<div class="animate-spin rounded-full h-8 w-8 border-b-2 border-earthy-terracotta-700"></div>
		</div>
	{:else if error}
		<div class="flex items-center justify-center w-full">
			<div
				class="bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800/50 rounded-lg p-4 text-red-600 dark:text-red-400"
			>
				{error}
			</div>
		</div>
	{:else}
		<div class="flex-1 flex flex-col min-w-0">
			<div class="flex-none p-8">
				<div class="mb-6">
					<button
						onclick={handleBack}
						class="inline-flex items-center text-sm text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300"
					>
						<svg class="w-5 h-5 mr-1" fill="none" stroke="currentColor" viewBox="0 0 24 24">
							<path
								stroke-linecap="round"
								stroke-linejoin="round"
								stroke-width="2"
								d="M10 19l-7-7m0 0l7-7m-7 7h18"
							/>
						</svg>
						Back
					</button>
				</div>

				{#if asset}
					<div class="mb-4 flex items-start gap-4">
						<!-- Asset Icon -->
						<div class="flex-shrink-0">
							<Icon name={getIconName(asset)} size="lg" />
						</div>

						<!-- Title, description, and metadata -->
						<div class="flex-1 min-w-0 space-y-1">
							<div class="flex items-center gap-3">
								<h1 class="text-2xl font-semibold text-gray-900 dark:text-gray-100">
									{asset.name}
								</h1>
								{#if asset.external_links && asset.external_links.length > 0}
									<div class="flex gap-1">
										{#each asset.external_links as link}
											<Button
												icon={link.icon}
												text={link.name}
												variant="clear"
												href={link.url}
												target="_blank"
											/>
										{/each}
									</div>
								{/if}
							</div>

							<p class="text-xs text-gray-500 dark:text-gray-400 font-mono">{asset.mrn}</p>

							<!-- Description (user notes) -->
							{#if isEditingDescription}
								<div class="space-y-2 max-w-2xl pt-1">
									<textarea
										bind:value={userDescription}
										placeholder="Add your notes..."
										rows="2"
										class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-earthy-terracotta-600 focus:border-transparent resize-y"
									></textarea>
									<div class="flex justify-between gap-2">
										<div>
											{#if asset.user_description}
												<button
													onclick={() => saveDescription('')}
													disabled={savingDescription}
													class="px-2 py-1 text-sm text-red-600 dark:text-red-400 hover:bg-red-50 dark:hover:bg-red-900/20 rounded disabled:opacity-50"
												>
													Delete
												</button>
											{/if}
										</div>
										<div class="flex gap-2">
											<button
												onclick={cancelEditingDescription}
												disabled={savingDescription}
												class="px-2 py-1 text-sm text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-800 rounded disabled:opacity-50"
											>
												Cancel
											</button>
											<button
												onclick={() => saveDescription()}
												disabled={savingDescription}
												class="px-2 py-1 text-sm bg-earthy-terracotta-700 text-white rounded hover:bg-earthy-terracotta-800 disabled:opacity-50 flex items-center gap-1"
											>
												{#if savingDescription}
													<div
														class="animate-spin rounded-full h-3 w-3 border-b-2 border-white"
													></div>
												{/if}
												Save
											</button>
										</div>
									</div>
								</div>
							{:else}
								<div class="flex items-start gap-2 max-w-2xl group pt-1">
									{#if asset.user_description}
										<p class="text-sm text-gray-500 dark:text-gray-400">{asset.user_description}</p>
									{:else if canManageAssets}
										<span class="text-sm text-gray-400 dark:text-gray-500 italic">No notes</span>
									{/if}
									{#if canManageAssets}
										<button
											onclick={startEditingDescription}
											class="flex-shrink-0 p-0.5 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 opacity-0 group-hover:opacity-100 transition-opacity"
											title="Edit notes"
										>
											<IconifyIcon icon="material-symbols:edit" class="w-3.5 h-3.5" />
										</button>
									{/if}
								</div>
							{/if}

							<!-- Tags and Owners -->
							<div class="flex items-start gap-6 pt-2">
								<div>
									<div class="flex items-center gap-1.5 mb-1">
										<IconifyIcon
											icon="material-symbols:label-outline"
											class="w-3.5 h-3.5 text-gray-400"
										/>
										<span class="text-xs font-medium text-gray-400 uppercase tracking-wide"
											>Tags</span
										>
									</div>
									<Tags
										tags={asset.tags ?? []}
										endpoint="/assets"
										id={asset.id}
										canEdit={canManageAssets}
									/>
								</div>
								<div>
									<div class="flex items-center gap-1.5 mb-1">
										<IconifyIcon
											icon="material-symbols:person-outline"
											class="w-3.5 h-3.5 text-gray-400"
										/>
										<span class="text-xs font-medium text-gray-400 uppercase tracking-wide"
											>Owners</span
										>
									</div>
									{#if loadingOwners}
										<div class="flex items-center py-1">
											<div
												class="animate-spin h-4 w-4 border-b-2 border-earthy-terracotta-700 rounded-full"
											></div>
										</div>
									{:else}
										<OwnerSelector
											selectedOwners={owners}
											onChange={handleOwnersChange}
											disabled={!canManageAssets}
										/>
									{/if}
								</div>
							</div>
						</div>
					</div>

					<Tabs tabs={visibleTabs} bind:activeTab onTabChange={setActiveTab} />
				{/if}
			</div>

			<div class="flex-1 overflow-y-auto overflow-x-auto px-8">
				<div class="pb-16 max-w-7xl mx-auto">
					<div class="rounded-lg max-w-full overflow-x-auto">
						{#if !asset}
							<div class="bg-gray-50 dark:bg-gray-800 rounded-lg p-4">
								<p class="text-gray-500 dark:text-gray-400">Loading asset information...</p>
							</div>
						{:else if activeTab === 'metadata'}
							<div class="mt-6">
								<MetadataView {asset} />
								{#if asset.sources && Array.isArray(asset.sources) && asset.sources.length > 0}
									<h3 class="pt-4 text-lg font-medium text-gray-900 dark:text-gray-100 mb-4">
										Asset Sources
									</h3>
									<AssetSources sources={asset.sources} />
								{/if}
							</div>
						{:else if activeTab === 'query'}
							<div class="mt-6">
								{#if asset.query}
									{#if asset.query_language}
										<div class="text-xs text-gray-500 dark:text-gray-400 mb-2 uppercase">
											{asset.query_language}
										</div>
									{/if}
									<CodeBlock code={asset.query} language={asset.query_language || 'sql'} />
								{:else}
									<div class="p-4 bg-gray-50 dark:bg-gray-800 rounded-lg">
										<p class="text-gray-500 dark:text-gray-400 italic">No query available</p>
									</div>
								{/if}
							</div>
						{:else if activeTab === 'environments'}
							<div class="mt-6">
								{#if asset.environments && Object.keys(asset.environments).length > 0}
									<AssetEnvironmentsView environments={asset.environments} />
								{:else}
									<div class="p-4 bg-gray-50 dark:bg-gray-800 rounded-lg">
										<p class="text-gray-500 dark:text-gray-400 italic">No environments available</p>
									</div>
								{/if}
							</div>
						{:else if activeTab === 'schema'}
							<div class="mt-6">
								<SchemaEditor {asset} />
							</div>
						{:else if activeTab === 'documentation'}
							<div class="mt-6" style="height: calc(100vh - 320px); min-height: 400px;">
								<DocumentationSystem entityType="asset" entityId={asset.mrn} />
							</div>
						{:else if activeTab === 'run-history'}
							<div class="mt-6">
								<RunHistory assetId={asset.id} />
							</div>
						{:else if activeTab === 'lineage'}
							<div class="mt-6">
								<Lineage currentAsset={asset} />
							</div>
						{:else}
							<div class="mt-6">
								<p class="text-gray-500 dark:text-gray-400">
									{activeTab.charAt(0).toUpperCase() + activeTab.slice(1)} coming soon.
								</p>
							</div>
						{/if}
					</div>
				</div>
			</div>
		</div>

		<div
			class="border-l border-gray-200 dark:border-gray-700 overflow-hidden transition-all duration-300 {bladeCollapsed
				? 'w-12'
				: 'w-[36rem]'}"
		>
			<AssetBlade
				{asset}
				staticPlacement={true}
				collapsed={bladeCollapsed}
				onToggleCollapse={() => (bladeCollapsed = !bladeCollapsed)}
				onClose={() => {}}
			/>
		</div>
	{/if}
</div>
