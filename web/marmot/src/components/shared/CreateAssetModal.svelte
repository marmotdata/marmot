<script lang="ts">
	import { fetchApi } from '$lib/api';
	import IconifyIcon from '@iconify/svelte';
	import Icon from '$components/ui/Icon.svelte';
	import TagsInput from './TagsInput.svelte';
	import Button from '$components/ui/Button.svelte';
	import { providerIconMap, typeIconMap } from '$lib/iconloader';

	let { show = $bindable(false), onSuccess }: { show: boolean; onSuccess?: () => void } = $props();

	let isCreating = $state(false);
	let createError = $state('');
	let newAssetName = $state('');
	let newAssetType = $state('');
	let newAssetProviders = $state<string[]>([]);
	let newAssetUserDescription = $state('');
	let newAssetTags = $state<string[]>([]);
	let typeSearch = $state('');
	let providerSearch = $state('');
	let showTypeDropdown = $state(false);
	let showProviderDropdown = $state(false);
	let selectedTypeIndex = $state(-1);
	let selectedProviderIndex = $state(-1);
	let typeDropdownElement = $state<HTMLDivElement>();
	let providerDropdownElement = $state<HTMLDivElement>();

	// Get unique suggestions with proper casing
	let filteredTypes = $derived(
		Object.keys(typeIconMap)
			.filter((type) =>
				typeIconMap[type].displayName.toLowerCase().includes(typeSearch.toLowerCase())
			)
			.map((type) => ({ key: type, display: typeIconMap[type].displayName }))
	);

	let filteredProviders = $derived(
		Object.keys(providerIconMap)
			.filter((provider) =>
				providerIconMap[provider].displayName.toLowerCase().includes(providerSearch.toLowerCase())
			)
			.map((provider) => ({ key: provider, display: providerIconMap[provider].displayName }))
	);

	// Reset selected index when filtered list changes
	$effect(() => {
		if (filteredTypes) selectedTypeIndex = -1;
	});
	$effect(() => {
		if (filteredProviders) selectedProviderIndex = -1;
	});

	function resetForm() {
		newAssetName = '';
		newAssetType = '';
		newAssetProviders = [];
		newAssetUserDescription = '';
		newAssetTags = [];
		typeSearch = '';
		providerSearch = '';
		createError = '';
		showTypeDropdown = false;
		showProviderDropdown = false;
	}

	function selectType(typeObj: { key: string; display: string }) {
		newAssetType = typeObj.display;
		typeSearch = typeObj.display;
		showTypeDropdown = false;
	}

	function handleTypeKeydown(event: KeyboardEvent) {
		if (!showTypeDropdown && event.key !== 'Escape') {
			showTypeDropdown = true;
		}

		if (event.key === 'ArrowDown') {
			event.preventDefault();
			selectedTypeIndex = Math.min(selectedTypeIndex + 1, filteredTypes.length - 1);
			scrollToSelectedType();
		} else if (event.key === 'ArrowUp') {
			event.preventDefault();
			selectedTypeIndex = Math.max(selectedTypeIndex - 1, -1);
			scrollToSelectedType();
		} else if (event.key === 'Enter') {
			event.preventDefault();
			if (selectedTypeIndex >= 0 && filteredTypes[selectedTypeIndex]) {
				selectType(filteredTypes[selectedTypeIndex]);
			} else if (typeSearch.trim()) {
				newAssetType = typeSearch.trim();
				showTypeDropdown = false;
			}
		} else if (event.key === 'Escape') {
			event.preventDefault();
			showTypeDropdown = false;
			selectedTypeIndex = -1;
		}
	}

	function scrollToSelectedType() {
		if (typeDropdownElement && selectedTypeIndex >= 0) {
			const buttons = typeDropdownElement.querySelectorAll('button');
			if (buttons[selectedTypeIndex]) {
				buttons[selectedTypeIndex].scrollIntoView({ block: 'nearest', behavior: 'smooth' });
			}
		}
	}

	function handleProviderKeydown(event: KeyboardEvent) {
		if (!showProviderDropdown && event.key !== 'Escape') {
			showProviderDropdown = true;
		}

		if (event.key === 'ArrowDown') {
			event.preventDefault();
			selectedProviderIndex = Math.min(selectedProviderIndex + 1, filteredProviders.length - 1);
			scrollToSelectedProvider();
		} else if (event.key === 'ArrowUp') {
			event.preventDefault();
			selectedProviderIndex = Math.max(selectedProviderIndex - 1, -1);
			scrollToSelectedProvider();
		} else if (event.key === 'Enter') {
			event.preventDefault();
			if (selectedProviderIndex >= 0 && filteredProviders[selectedProviderIndex]) {
				toggleProvider(filteredProviders[selectedProviderIndex]);
				selectedProviderIndex = -1;
			} else if (providerSearch.trim()) {
				if (!newAssetProviders.includes(providerSearch.trim())) {
					newAssetProviders = [...newAssetProviders, providerSearch.trim()];
				}
				providerSearch = '';
			}
		} else if (event.key === 'Escape') {
			event.preventDefault();
			showProviderDropdown = false;
			selectedProviderIndex = -1;
		}
	}

	function scrollToSelectedProvider() {
		if (providerDropdownElement && selectedProviderIndex >= 0) {
			const buttons = providerDropdownElement.querySelectorAll('button');
			if (buttons[selectedProviderIndex]) {
				buttons[selectedProviderIndex].scrollIntoView({ block: 'nearest', behavior: 'smooth' });
			}
		}
	}

	function toggleProvider(providerObj: { key: string; display: string }) {
		const displayName = providerObj.display;
		if (newAssetProviders.includes(displayName)) {
			newAssetProviders = newAssetProviders.filter((p) => p !== displayName);
		} else {
			newAssetProviders = [...newAssetProviders, displayName];
		}
		providerSearch = '';
	}

	function removeProvider(provider: string) {
		newAssetProviders = newAssetProviders.filter((p) => p !== provider);
	}

	async function createAsset() {
		if (!newAssetName || !newAssetType || newAssetProviders.length === 0) {
			createError = 'Name, type, and at least one provider are required';
			return;
		}

		isCreating = true;
		createError = '';

		try {
			const payload: any = {
				name: newAssetName,
				type: newAssetType,
				providers: newAssetProviders
			};

			if (newAssetUserDescription) {
				payload.user_description = newAssetUserDescription;
			}
			if (newAssetTags.length > 0) {
				payload.tags = newAssetTags;
			}

			const response = await fetchApi('/assets/', {
				method: 'POST',
				body: JSON.stringify(payload)
			});

			if (!response.ok) {
				const errorData = await response.json();
				throw new Error(errorData.error || 'Failed to create asset');
			}

			show = false;
			resetForm();
			if (onSuccess) {
				onSuccess();
			}
		} catch (err) {
			createError = err instanceof Error ? err.message : 'Failed to create asset';
		} finally {
			isCreating = false;
		}
	}

	function handleClose() {
		if (!isCreating) {
			show = false;
			resetForm();
		}
	}

	// Reset form when modal is opened
	$effect(() => {
		if (show) {
			resetForm();
		}
	});
</script>

{#if show}
	<div class="fixed inset-0 z-50 flex items-start justify-center pt-[10vh] px-4 overflow-y-auto">
		<div
			class="fixed inset-0 bg-black/50 dark:bg-black/70 backdrop-blur-sm transition-opacity"
			onclick={handleClose}
			onkeydown={(e) => e.key === 'Enter' && handleClose()}
			role="button"
			tabindex="0"
		></div>

		<div
			class="relative bg-white dark:bg-gray-800 rounded-xl shadow-2xl max-w-3xl w-full z-10 border border-gray-200 dark:border-gray-700 mb-10"
		>
			<!-- Header -->
			<div
				class="flex items-center justify-between px-6 py-5 border-b border-gray-200 dark:border-gray-700"
			>
				<div>
					<h3 class="text-xl font-semibold text-gray-900 dark:text-gray-100">Create New Asset</h3>
					<p class="text-sm text-gray-500 dark:text-gray-400 mt-1">
						Add a new asset to your data catalog
					</p>
				</div>
				<button
					onclick={handleClose}
					disabled={isCreating}
					class="text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 transition-colors disabled:opacity-50"
				>
					<IconifyIcon icon="material-symbols:close" class="w-6 h-6" />
				</button>
			</div>

			{#if createError}
				<div
					class="mx-6 mt-5 rounded-lg bg-red-50 dark:bg-red-900/20 p-4 border border-red-200 dark:border-red-800"
				>
					<div class="flex items-start gap-3">
						<IconifyIcon
							icon="material-symbols:error"
							class="w-6 h-6 text-red-600 dark:text-red-400"
						/>
						<p class="text-sm text-red-800 dark:text-red-200">{createError}</p>
					</div>
				</div>
			{/if}

			<form
				onsubmit={(e) => {
					e.preventDefault();
					createAsset();
				}}
				class="p-6"
			>
				<div class="space-y-6">
					<!-- Basic Information Section -->
					<div class="space-y-4">
						<!-- Name -->
						<div>
							<label
								for="asset-name"
								class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2"
							>
								Name <span class="text-red-500">*</span>
							</label>
							<input
								id="asset-name"
								type="text"
								bind:value={newAssetName}
								disabled={isCreating}
								placeholder="e.g., user_events"
								class="w-full px-4 py-3 border border-gray-300 dark:border-gray-600 rounded-lg shadow-sm focus:ring-2 focus:ring-earthy-terracotta-600 focus:border-earthy-terracotta-700 dark:bg-gray-700 dark:text-gray-100 disabled:opacity-50 transition-all"
								required
							/>
						</div>

						<!-- Type - Searchable with Free Text -->
						<div>
							<label
								for="asset-type"
								class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2"
							>
								Type <span class="text-red-500">*</span>
							</label>
							<div class="relative">
								<input
									type="text"
									bind:value={typeSearch}
									oninput={() => {
										newAssetType = typeSearch;
										showTypeDropdown = true;
									}}
									onfocus={() => (showTypeDropdown = true)}
									onblur={() => setTimeout(() => (showTypeDropdown = false), 200)}
									onkeydown={handleTypeKeydown}
									disabled={isCreating}
									placeholder="e.g., Table, Queue, Topic, Database..."
									class="w-full px-4 py-3 border border-gray-300 dark:border-gray-600 rounded-lg shadow-sm focus:ring-2 focus:ring-earthy-terracotta-600 focus:border-earthy-terracotta-700 dark:bg-gray-700 dark:text-gray-100 disabled:opacity-50 transition-all font-mono"
									required
								/>
								{#if showTypeDropdown && filteredTypes.length > 0}
									<div
										bind:this={typeDropdownElement}
										class="absolute z-10 w-full mt-1 bg-white dark:bg-gray-700 border border-gray-200 dark:border-gray-600 rounded-lg shadow-lg max-h-60 overflow-y-auto"
									>
										{#each filteredTypes as typeObj, index (typeObj.key)}
											<button
												type="button"
												onclick={() => selectType(typeObj)}
												class="w-full px-4 py-3 flex items-center gap-3 hover:bg-gray-50 dark:hover:bg-gray-600 transition-colors text-left {index ===
												selectedTypeIndex
													? 'bg-earthy-terracotta-50 dark:bg-earthy-terracotta-900/30'
													: ''}"
											>
												<Icon name={typeObj.key} showLabel={false} size="md" />
												<span class="text-gray-900 dark:text-gray-100">{typeObj.display}</span>
											</button>
										{/each}
									</div>
								{/if}
							</div>
							<p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
								The type of asset. Table, Queue, Topic, Database, View, DAG, etc.
							</p>
						</div>

						<!-- Providers - Multi-select with Free Text -->
						<div>
							<label
								for="asset-providers"
								class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2"
							>
								Providers <span class="text-red-500">*</span>
							</label>

							{#if newAssetProviders.length > 0}
								<div class="flex flex-wrap gap-2 mb-2">
									{#each newAssetProviders as provider, index (index)}
										<span
											class="inline-flex items-center gap-2 px-3 py-1.5 bg-earthy-terracotta-50 dark:bg-earthy-terracotta-900/30 text-earthy-terracotta-700 dark:text-earthy-terracotta-100 rounded-lg border border-earthy-terracotta-200 dark:border-earthy-terracotta-800"
										>
											<span class="text-sm font-medium">{provider}</span>
											<button
												type="button"
												onclick={() => removeProvider(provider)}
												disabled={isCreating}
												class="text-earthy-terracotta-700 dark:text-earthy-terracotta-700 hover:text-earthy-terracotta-700 dark:hover:text-earthy-terracotta-200 transition-colors disabled:opacity-50"
											>
												<IconifyIcon icon="material-symbols:close" class="w-4 h-4" />
											</button>
										</span>
									{/each}
								</div>
							{/if}

							<div class="relative">
								<input
									type="text"
									bind:value={providerSearch}
									onfocus={() => (showProviderDropdown = true)}
									onblur={() => setTimeout(() => (showProviderDropdown = false), 200)}
									onkeydown={handleProviderKeydown}
									disabled={isCreating}
									placeholder="e.g., Kafka, Snowflake, PostgreSQL, Airflow..."
									class="w-full px-4 py-3 border border-gray-300 dark:border-gray-600 rounded-lg shadow-sm focus:ring-2 focus:ring-earthy-terracotta-600 focus:border-earthy-terracotta-700 dark:bg-gray-700 dark:text-gray-100 disabled:opacity-50 transition-all font-mono"
								/>
								{#if showProviderDropdown && filteredProviders.length > 0}
									<div
										bind:this={providerDropdownElement}
										class="absolute z-10 w-full mt-1 bg-white dark:bg-gray-700 border border-gray-200 dark:border-gray-600 rounded-lg shadow-lg max-h-60 overflow-y-auto"
									>
										{#each filteredProviders as providerObj, index (providerObj.key)}
											<button
												type="button"
												onclick={() => toggleProvider(providerObj)}
												class="w-full px-4 py-3 flex items-center gap-3 hover:bg-gray-50 dark:hover:bg-gray-600 transition-colors text-left {index ===
												selectedProviderIndex
													? 'bg-blue-50 dark:bg-blue-900/30'
													: newAssetProviders.includes(providerObj.display)
														? 'bg-earthy-terracotta-50 dark:bg-earthy-terracotta-900/20'
														: ''}"
											>
												<Icon name={providerObj.key} showLabel={false} size="md" />
												<span class="text-gray-900 dark:text-gray-100 flex-1"
													>{providerObj.display}</span
												>
												{#if newAssetProviders.includes(providerObj.display)}
													<IconifyIcon
														icon="material-symbols:check"
														class="w-5 h-5 text-earthy-terracotta-700 dark:text-earthy-terracotta-700"
													/>
												{/if}
											</button>
										{/each}
									</div>
								{/if}
							</div>
							<p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
								The asset Provider. Kafka, Snowflake, PostgreSQL, Airflow, dbt, S3, etc.
							</p>
						</div>
					</div>

					<!-- Description Section -->
					<div class="space-y-4 pt-4 border-t border-gray-200 dark:border-gray-700">
						<h4
							class="text-sm font-semibold text-gray-900 dark:text-gray-100 uppercase tracking-wider"
						>
							Description
						</h4>

						<div>
							<label
								for="asset-user-description"
								class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2"
							>
								Description
							</label>
							<textarea
								id="asset-user-description"
								bind:value={newAssetUserDescription}
								disabled={isCreating}
								placeholder="Add your own description to provide context..."
								rows="3"
								class="w-full px-4 py-3 border border-gray-300 dark:border-gray-600 rounded-lg shadow-sm focus:ring-2 focus:ring-earthy-terracotta-600 focus:border-earthy-terracotta-700 dark:bg-gray-700 dark:text-gray-100 disabled:opacity-50 transition-all resize-none"
							></textarea>
							<p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
								Your custom description for this asset
							</p>
						</div>
					</div>

					<!-- Tags Section -->
					<div class="space-y-4 pt-4 border-t border-gray-200 dark:border-gray-700">
						<h4
							class="text-sm font-semibold text-gray-900 dark:text-gray-100 uppercase tracking-wider"
						>
							Tags
						</h4>

						<div>
							<TagsInput
								bind:tags={newAssetTags}
								disabled={isCreating}
								placeholder="Type a tag and press Enter..."
							/>
							<p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
								Press Enter to add tags for better organization
							</p>
						</div>
					</div>
				</div>

				<div class="flex justify-end gap-3 pt-6 mt-6 border-t border-gray-200 dark:border-gray-700">
					<Button
						type="button"
						click={handleClose}
						disabled={isCreating}
						text="Cancel"
						variant="clear"
					/>
					<Button
						type="submit"
						disabled={isCreating ||
							!newAssetName ||
							!newAssetType ||
							newAssetProviders.length === 0}
						loading={isCreating}
						icon="material-symbols:add"
						text={isCreating ? 'Creating...' : 'Create Asset'}
						variant="filled"
					/>
				</div>
			</form>
		</div>
	</div>
{/if}
