<script lang="ts">
	import { goto } from '$app/navigation';
	import { fetchApi } from '$lib/api';
	import Button from '$components/ui/Button.svelte';
	import IconifyIcon from '@iconify/svelte';
	import Icon from '$components/ui/Icon.svelte';
	import Stepper from '$components/ui/Stepper.svelte';
	import Step from '$components/ui/Step.svelte';
	import TagsInput from '$components/shared/TagsInput.svelte';
	import { providerIconMap, typeIconMap } from '$lib/iconloader';

	let name = $state('');
	let assetType = $state('');
	let providers = $state<string[]>([]);
	let userDescription = $state('');
	let tags = $state<string[]>([]);
	let saving = $state(false);
	let error = $state<string | null>(null);
	let currentStep = $state(1);
	let totalSteps = $state(0);

	// Field-level validation
	let fieldErrors = $state<Record<string, string>>({});
	let touched = $state<Record<string, boolean>>({});

	function validateName(value: string): string | null {
		if (!value.trim()) return 'Asset name is required';
		if (value.trim().length < 2) return 'Name must be at least 2 characters';
		if (!/^[a-zA-Z0-9_\-.]+$/.test(value.trim()))
			return 'Name can only contain letters, numbers, underscores, hyphens, and dots';
		return null;
	}

	function validateType(value: string): string | null {
		if (!value.trim()) return 'Asset type is required';
		return null;
	}

	function validateProviders(value: string[]): string | null {
		if (value.length === 0) return 'At least one provider is required';
		return null;
	}

	function validateField(field: string) {
		touched[field] = true;
		if (field === 'name') {
			const err = validateName(name);
			if (err) fieldErrors['name'] = err;
			else delete fieldErrors['name'];
		} else if (field === 'type') {
			const err = validateType(assetType);
			if (err) fieldErrors['type'] = err;
			else delete fieldErrors['type'];
		} else if (field === 'providers') {
			const err = validateProviders(providers);
			if (err) fieldErrors['providers'] = err;
			else delete fieldErrors['providers'];
		}
		fieldErrors = { ...fieldErrors };
	}

	function clearFieldError(field: string) {
		delete fieldErrors[field];
		fieldErrors = { ...fieldErrors };
	}

	// Search/dropdown state
	let typeSearch = $state('');
	let providerSearch = $state('');
	let showTypeDropdown = $state(false);
	let showProviderDropdown = $state(false);
	let selectedTypeIndex = $state(-1);
	let selectedProviderIndex = $state(-1);
	let typeDropdownElement = $state<HTMLDivElement>();
	let providerDropdownElement = $state<HTMLDivElement>();

	const steps = [{ title: 'Basic Info' }, { title: 'Type & Providers' }, { title: 'Details' }];

	let canProceedToStep2 = $derived(validateName(name) === null);
	let canProceedToStep3 = $derived(
		validateType(assetType) === null && validateProviders(providers) === null
	);

	function canNavigateToStep(stepNumber: number): boolean {
		if (stepNumber === 2) return canProceedToStep2;
		if (stepNumber === 3) return canProceedToStep3;
		return false;
	}

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

	function selectType(typeObj: { key: string; display: string }) {
		assetType = typeObj.display;
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
				assetType = typeSearch.trim();
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
				if (!providers.includes(providerSearch.trim())) {
					providers = [...providers, providerSearch.trim()];
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
		if (providers.includes(displayName)) {
			providers = providers.filter((p) => p !== displayName);
		} else {
			providers = [...providers, displayName];
		}
		providerSearch = '';
	}

	function removeProvider(provider: string) {
		providers = providers.filter((p) => p !== provider);
	}

	async function handleSave() {
		if (!name.trim() || !assetType.trim() || providers.length === 0) {
			error = 'Name, type, and at least one provider are required';
			return;
		}

		try {
			saving = true;
			error = null;

			const payload: Record<string, unknown> = {
				name: name.trim(),
				type: assetType.trim(),
				providers
			};

			if (userDescription.trim()) {
				payload.user_description = userDescription.trim();
			}
			if (tags.length > 0) {
				payload.tags = tags;
			}

			const response = await fetchApi('/assets/', {
				method: 'POST',
				body: JSON.stringify(payload)
			});

			if (!response.ok) {
				const errorData = await response.json();
				throw new Error(errorData.error || 'Failed to create asset');
			}

			const data = await response.json();
			// URL format: /discover/type/provider/name
			const type = encodeURIComponent(data.type.toLowerCase());
			const provider = encodeURIComponent(data.providers[0].toLowerCase());
			const assetName = encodeURIComponent(data.name);
			goto(`/discover/${type}/${provider}/${assetName}`);
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to create asset';
		} finally {
			saving = false;
		}
	}

	function handleNextStep() {
		if (currentStep === 1) {
			validateField('name');
			const nameErr = validateName(name);
			if (nameErr) {
				error = nameErr;
				return;
			}
			error = null;
			currentStep++;
			return;
		}

		if (currentStep === 2) {
			validateField('type');
			validateField('providers');
			const typeErr = validateType(assetType);
			const providersErr = validateProviders(providers);
			if (typeErr) {
				error = typeErr;
				return;
			}
			if (providersErr) {
				error = providersErr;
				return;
			}
			error = null;
			currentStep++;
			return;
		}
	}
</script>

<div class="min-h-screen">
	<!-- Header -->
	<div class="border-b border-gray-200 dark:border-gray-700">
		<div class="container max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-6">
			<div class="flex items-center gap-4">
				<button
					onclick={() => goto('/discover')}
					class="p-2 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg transition-colors"
				>
					<IconifyIcon
						icon="material-symbols:arrow-back"
						class="h-6 w-6 text-gray-600 dark:text-gray-400"
					/>
				</button>
				<div>
					<h1 class="text-2xl font-bold text-gray-900 dark:text-gray-100">Create Asset</h1>
					<p class="text-sm text-gray-600 dark:text-gray-400 mt-1">
						Step {currentStep} of {steps.length} — {steps[currentStep - 1].title}
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
				<Step title="Type & Providers" icon="material-symbols:category" />
				<Step title="Details" icon="material-symbols:description" />
			</Stepper>
		</div>
	</div>

	<!-- Main Content -->
	<div class="container max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
		<!-- Tip Banner -->
		<div
			class="mb-6 bg-gradient-to-r from-green-50 to-emerald-50 dark:from-green-900/20 dark:to-emerald-900/20 border border-green-200 dark:border-green-800/50 rounded-lg p-5"
		>
			<div class="flex items-start gap-3">
				<IconifyIcon
					icon="material-symbols:auto-awesome"
					class="h-5 w-5 text-green-600 dark:text-green-400 mt-0.5 flex-shrink-0"
				/>
				<div class="flex-1">
					<h4 class="text-sm font-semibold text-green-900 dark:text-green-100">
						Automate asset discovery
					</h4>
					<p class="text-sm text-green-700 dark:text-green-300 mt-1">
						Instead of adding assets manually, set up a Pipeline to automatically discover and sync
						assets from your data sources.
					</p>
					<a
						href="/runs?tab=pipelines"
						class="inline-flex items-center gap-2 mt-3 px-4 py-2 bg-green-600 hover:bg-green-700 text-white text-sm font-medium rounded-lg shadow-sm transition-all hover:shadow-md"
					>
						<IconifyIcon icon="material-symbols:rocket-launch" class="h-4 w-4" />
						Set up a Pipeline
					</a>
				</div>
			</div>
		</div>

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
				<div>
					<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
						Asset Name <span class="text-red-500">*</span>
					</label>
					<input
						type="text"
						bind:value={name}
						placeholder="e.g., user_events, orders_table"
						oninput={() => {
							if (touched['name']) validateField('name');
							else clearFieldError('name');
						}}
						onblur={() => validateField('name')}
						onkeydown={(e) => {
							if (e.key === 'Enter' && canProceedToStep2) {
								e.preventDefault();
								handleNextStep();
							}
						}}
						class="w-full px-4 py-2.5 border rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-earthy-terracotta-600 focus:border-transparent transition-all {fieldErrors[
							'name'
						] && touched['name']
							? 'border-red-500 dark:border-red-500'
							: 'border-gray-300 dark:border-gray-600'}"
						required
					/>
					{#if fieldErrors['name'] && touched['name']}
						<p class="mt-1.5 text-sm text-red-600 dark:text-red-400 flex items-center">
							<IconifyIcon icon="material-symbols:error" class="h-4 w-4 mr-1 flex-shrink-0" />
							{fieldErrors['name']}
						</p>
					{:else}
						<p class="mt-2 text-xs text-gray-500 dark:text-gray-400">
							A unique identifier for this asset in your data catalog
						</p>
					{/if}
				</div>
			</div>
		{/if}

		<!-- Step 2: Type & Providers -->
		{#if currentStep === 2}
			<div
				class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-6"
			>
				<h3 class="text-base font-semibold text-gray-900 dark:text-gray-100 mb-4 flex items-center">
					<IconifyIcon
						icon="material-symbols:category"
						class="h-5 w-5 mr-2 text-earthy-terracotta-600"
					/>
					Type & Providers
				</h3>

				<div class="space-y-6">
					<!-- Type -->
					<div>
						<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
							Type <span class="text-red-500">*</span>
						</label>
						<div class="relative">
							<input
								type="text"
								bind:value={typeSearch}
								oninput={() => {
									assetType = typeSearch;
									showTypeDropdown = true;
									if (touched['type']) validateField('type');
									else clearFieldError('type');
								}}
								onfocus={() => (showTypeDropdown = true)}
								onblur={() => {
									setTimeout(() => (showTypeDropdown = false), 200);
									validateField('type');
								}}
								onkeydown={handleTypeKeydown}
								placeholder="e.g., Table, Queue, Topic, Database..."
								class="w-full px-4 py-2.5 border rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-earthy-terracotta-600 focus:border-transparent transition-all font-mono {fieldErrors[
									'type'
								] && touched['type']
									? 'border-red-500 dark:border-red-500'
									: 'border-gray-300 dark:border-gray-600'}"
							/>
							{#if showTypeDropdown && filteredTypes.length > 0}
								<div
									bind:this={typeDropdownElement}
									class="absolute z-10 w-full mt-1 bg-white dark:bg-gray-700 border border-gray-200 dark:border-gray-600 rounded-lg shadow-lg max-h-60 overflow-y-auto"
								>
									{#each filteredTypes as typeObj, index (typeObj.key)}
										<button
											type="button"
											onclick={() => {
												selectType(typeObj);
												clearFieldError('type');
											}}
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
						{#if fieldErrors['type'] && touched['type']}
							<p class="mt-1.5 text-sm text-red-600 dark:text-red-400 flex items-center">
								<IconifyIcon icon="material-symbols:error" class="h-4 w-4 mr-1 flex-shrink-0" />
								{fieldErrors['type']}
							</p>
						{:else}
							<p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
								The type of asset: Table, Queue, Topic, Database, View, DAG, etc.
							</p>
						{/if}
					</div>

					<!-- Providers -->
					<div>
						<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
							Providers <span class="text-red-500">*</span>
						</label>

						{#if providers.length > 0}
							<div class="flex flex-wrap gap-2 mb-2">
								{#each providers as provider, index (index)}
									<span
										class="inline-flex items-center gap-2 px-3 py-1.5 bg-earthy-terracotta-50 dark:bg-earthy-terracotta-900/30 text-earthy-terracotta-700 dark:text-earthy-terracotta-100 rounded-lg border border-earthy-terracotta-200 dark:border-earthy-terracotta-800"
									>
										<span class="text-sm font-medium">{provider}</span>
										<button
											type="button"
											onclick={() => {
												removeProvider(provider);
												validateField('providers');
											}}
											class="text-earthy-terracotta-700 dark:text-earthy-terracotta-700 hover:text-earthy-terracotta-700 dark:hover:text-earthy-terracotta-200 transition-colors"
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
								onblur={() => {
									setTimeout(() => (showProviderDropdown = false), 200);
									validateField('providers');
								}}
								onkeydown={handleProviderKeydown}
								placeholder="e.g., Kafka, Snowflake, PostgreSQL, Airflow..."
								class="w-full px-4 py-2.5 border rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-earthy-terracotta-600 focus:border-transparent transition-all font-mono {fieldErrors[
									'providers'
								] &&
								touched['providers'] &&
								providers.length === 0
									? 'border-red-500 dark:border-red-500'
									: 'border-gray-300 dark:border-gray-600'}"
							/>
							{#if showProviderDropdown && filteredProviders.length > 0}
								<div
									bind:this={providerDropdownElement}
									class="absolute z-10 w-full mt-1 bg-white dark:bg-gray-700 border border-gray-200 dark:border-gray-600 rounded-lg shadow-lg max-h-60 overflow-y-auto"
								>
									{#each filteredProviders as providerObj, index (providerObj.key)}
										<button
											type="button"
											onclick={() => {
												toggleProvider(providerObj);
												clearFieldError('providers');
											}}
											class="w-full px-4 py-3 flex items-center gap-3 hover:bg-gray-50 dark:hover:bg-gray-600 transition-colors text-left {index ===
											selectedProviderIndex
												? 'bg-blue-50 dark:bg-blue-900/30'
												: providers.includes(providerObj.display)
													? 'bg-earthy-terracotta-50 dark:bg-earthy-terracotta-900/20'
													: ''}"
										>
											<Icon name={providerObj.key} showLabel={false} size="md" />
											<span class="text-gray-900 dark:text-gray-100 flex-1"
												>{providerObj.display}</span
											>
											{#if providers.includes(providerObj.display)}
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
						{#if fieldErrors['providers'] && touched['providers']}
							<p class="mt-1.5 text-sm text-red-600 dark:text-red-400 flex items-center">
								<IconifyIcon icon="material-symbols:error" class="h-4 w-4 mr-1 flex-shrink-0" />
								{fieldErrors['providers']}
							</p>
						{:else}
							<p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
								The data platform or service: Kafka, Snowflake, PostgreSQL, Airflow, dbt, S3, etc.
							</p>
						{/if}
					</div>
				</div>
			</div>
		{/if}

		<!-- Step 3: Details -->
		{#if currentStep === 3}
			<div
				class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-6"
			>
				<h3 class="text-base font-semibold text-gray-900 dark:text-gray-100 mb-4 flex items-center">
					<IconifyIcon
						icon="material-symbols:description"
						class="h-5 w-5 mr-2 text-earthy-terracotta-600"
					/>
					Additional Details
					<span class="ml-2 text-xs font-normal text-gray-500">(Optional)</span>
				</h3>

				<div class="space-y-6">
					<!-- Description -->
					<div>
						<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
							Description
						</label>
						<textarea
							bind:value={userDescription}
							placeholder="Add a description to provide context about this asset..."
							rows="4"
							class="w-full px-4 py-2.5 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-earthy-terracotta-600 focus:border-transparent transition-all resize-none"
						></textarea>
						<p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
							Help others understand what this asset is used for
						</p>
					</div>

					<!-- Tags -->
					<div>
						<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
							Tags
						</label>
						<TagsInput bind:tags placeholder="Type a tag and press Enter..." />
						<p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
							Add tags for better organization and discovery
						</p>
					</div>
				</div>
			</div>

			<!-- Summary Card -->
			<div
				class="mt-6 bg-gray-50 dark:bg-gray-800/50 rounded-xl border border-gray-200 dark:border-gray-700 p-6"
			>
				<h4 class="text-sm font-semibold text-gray-900 dark:text-gray-100 mb-4 flex items-center">
					<IconifyIcon icon="material-symbols:summarize" class="h-5 w-5 mr-2 text-gray-500" />
					Summary
				</h4>
				<dl class="grid grid-cols-1 sm:grid-cols-2 gap-4 text-sm">
					<div>
						<dt class="text-gray-500 dark:text-gray-400">Name</dt>
						<dd class="font-medium text-gray-900 dark:text-gray-100 font-mono">{name}</dd>
					</div>
					<div>
						<dt class="text-gray-500 dark:text-gray-400">Type</dt>
						<dd class="font-medium text-gray-900 dark:text-gray-100">{assetType}</dd>
					</div>
					<div class="sm:col-span-2">
						<dt class="text-gray-500 dark:text-gray-400">Providers</dt>
						<dd class="flex flex-wrap gap-1.5 mt-1">
							{#each providers as provider}
								<span
									class="inline-flex items-center px-2 py-0.5 rounded-md text-xs font-medium bg-gray-200 dark:bg-gray-700 text-gray-700 dark:text-gray-300"
								>
									{provider}
								</span>
							{/each}
						</dd>
					</div>
					{#if userDescription}
						<div class="sm:col-span-2">
							<dt class="text-gray-500 dark:text-gray-400">Description</dt>
							<dd class="font-medium text-gray-900 dark:text-gray-100">{userDescription}</dd>
						</div>
					{/if}
					{#if tags.length > 0}
						<div class="sm:col-span-2">
							<dt class="text-gray-500 dark:text-gray-400">Tags</dt>
							<dd class="flex flex-wrap gap-1.5 mt-1">
								{#each tags as tag}
									<span
										class="inline-flex items-center px-2 py-0.5 rounded-md text-xs font-medium bg-blue-100 dark:bg-blue-900/30 text-blue-700 dark:text-blue-300"
									>
										{tag}
									</span>
								{/each}
							</dd>
						</div>
					{/if}
				</dl>
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
					<Button variant="clear" click={() => goto('/discover')} text="Cancel" />
				{/if}
			</div>
			<div class="flex items-center gap-3">
				{#if currentStep < totalSteps}
					<Button
						variant="filled"
						click={handleNextStep}
						text="Next"
						icon="material-symbols:arrow-forward"
						disabled={(currentStep === 1 && !canProceedToStep2) ||
							(currentStep === 2 && !canProceedToStep3)}
					/>
				{:else}
					<Button
						variant="filled"
						click={handleSave}
						text={saving ? 'Creating...' : 'Create Asset'}
						disabled={saving || !name.trim() || !assetType.trim() || providers.length === 0}
						icon="material-symbols:check"
					/>
				{/if}
			</div>
		</div>
	</div>
</div>
