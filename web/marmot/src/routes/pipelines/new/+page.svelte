<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { fetchApi } from '$lib/api';
	import Button from '../../../components/Button.svelte';
	import IconifyIcon from '@iconify/svelte';
	import Icon from '../../../components/Icon.svelte';
	import cronstrue from 'cronstrue';
	import { Cron } from 'croner';

	interface ConfigField {
		name: string;
		type: string;
		label: string;
		description: string;
		required: boolean;
		default?: any;
		options?: { label: string; value: string }[];
		sensitive: boolean;
		placeholder?: string;
		fields?: ConfigField[];
		is_array?: boolean;
		validation?: {
			pattern?: string;
			min?: number;
			max?: number;
		};
	}

	interface Plugin {
		id: string;
		name: string;
		description?: string;
		icon?: string;
		category?: string;
		config_spec?: ConfigField[];
	}

	let plugins = $state<Plugin[]>([]);
	let loadingPlugins = $state(true);
	let selectedPluginId = $state('');
	let name = $state('');
	let cronExpression = $state('');
	let disableSchedule = $state(false);
	let config = $state<Record<string, any>>({});
	let saving = $state(false);
	let error = $state<string | null>(null);
	let pluginSearchQuery = $state('');
	let currentStep = $state(1);
	let validating = $state(false);
	let fieldErrors = $state<Record<string, string>>({});
	let expandedSections = $state<Record<string, boolean>>({});
	let awsCredentialStatus = $state<{
		available: boolean;
		sources: string[];
		error?: string;
	} | null>(null);
	let loadingAwsStatus = $state(false);

	const steps = [
		{ number: 1, title: 'Basic Info', icon: 'material-symbols:info-outline' },
		{ number: 2, title: 'Choose Plugin', icon: 'material-symbols:extension' },
		{ number: 3, title: 'Configure', icon: 'material-symbols:settings' },
		{ number: 4, title: 'Schedule', icon: 'material-symbols:schedule' }
	];

	let canProceedToStep2 = $derived(name.trim() !== '');
	let canProceedToStep3 = $derived(selectedPluginId !== '');
	let configValidated = $state(false);
	let canProceedToStep4 = $derived(configValidated); // Must validate config first

	let selectedPlugin = $derived(plugins.find((p) => p.id === selectedPluginId) || null);

	let configSpec = $derived(selectedPlugin?.config_spec || null);

	let isAWSPlugin = $derived(
		selectedPluginId && ['s3', 'sns', 'sqs', 'dynamodb', 'kinesis'].includes(selectedPluginId)
	);
	let hasSchedule = $derived(cronExpression.trim() !== '');

	let cronDescription = $derived.by(() => {
		if (!cronExpression.trim()) return null;
		try {
			return cronstrue.toString(cronExpression, { verbose: true });
		} catch (e) {
			return null;
		}
	});

	let cronNextRuns = $derived.by(() => {
		if (!cronExpression.trim()) return [];
		try {
			const cron = Cron(cronExpression);
			const runs: Date[] = [];
			const now = new Date();
			for (let i = 0; i < 5; i++) {
				const next = cron.next(i === 0 ? now : runs[i - 1]);
				if (next) runs.push(next);
			}
			return runs;
		} catch (e) {
			return [];
		}
	});

	let filteredPlugins = $derived(
		plugins.filter((plugin) => {
			const searchLower = pluginSearchQuery.toLowerCase();
			return (
				plugin.name.toLowerCase().includes(searchLower) ||
				plugin.description?.toLowerCase().includes(searchLower) ||
				plugin.id.toLowerCase().includes(searchLower)
			);
		})
	);

	// Show all plugins
	let displayedPlugins = $derived(filteredPlugins);

	async function fetchPlugins() {
		try {
			loadingPlugins = true;
			const response = await fetchApi('/plugins');
			if (!response.ok) throw new Error('Failed to fetch plugins');
			const data = await response.json();
			plugins = Array.isArray(data) ? data : [];
		} catch (err) {
			console.error('Error fetching plugins:', err);
			error = 'Failed to load plugins';
		} finally {
			loadingPlugins = false;
		}
	}

	async function handleSave() {
		try {
			saving = true;
			error = null;

			// If no schedule, pipeline is manual-only and always enabled
			// If schedule is provided, enabled depends on disableSchedule checkbox
			const enabled = cronExpression.trim() === '' ? true : !disableSchedule;

			const body = {
				name,
				plugin_id: selectedPluginId,
				config,
				cron_expression: cronExpression,
				enabled
			};

			const response = await fetchApi('/ingestion/schedules', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify(body)
			});

			if (!response.ok) {
				const data = await response.json();
				throw new Error(data.error || 'Failed to create pipeline');
			}

			// Navigate back to runs page with pipelines tab
			goto('/runs?tab=pipelines');
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to create pipeline';
		} finally {
			saving = false;
		}
	}

	function initializeConfigDefaults(fields: ConfigField[], configObj: Record<string, any> = {}) {
		for (const field of fields) {
			if (field.type === 'object' && field.is_array) {
				// Initialize array of objects as empty array
				configObj[field.name] = [];
			} else if (field.type === 'object' && field.fields) {
				// Initialize nested object
				configObj[field.name] = {};
				initializeConfigDefaults(field.fields, configObj[field.name]);
			} else if (field.type === 'multiselect') {
				// Initialize multiselect as empty array
				configObj[field.name] = [];
			} else if (field.default !== undefined && field.default !== null) {
				configObj[field.name] = field.default;
			}
		}
		return configObj;
	}

	async function fetchAWSCredentialStatus() {
		try {
			loadingAwsStatus = true;
			const response = await fetchApi('/plugins/aws/credentials/status');
			if (response.ok) {
				awsCredentialStatus = await response.json();
				// If credentials are available, set use_default to true by default
				if (awsCredentialStatus?.available) {
					// Ensure credentials object exists
					if (!config.credentials) {
						config.credentials = {};
					}
					// Set use_default to true (reactive assignment)
					config = {
						...config,
						credentials: {
							...config.credentials,
							use_default: true
						}
					};
				}
			}
		} catch (err) {
			console.error('Error fetching AWS credential status:', err);
		} finally {
			loadingAwsStatus = false;
		}
	}

	function handlePluginChange(pluginId: string) {
		selectedPluginId = pluginId;
		// Reset config and pre-populate with default values
		config = {};
		fieldErrors = {};
		configValidated = false;
		awsCredentialStatus = null;

		// Find selected plugin and populate defaults
		const plugin = plugins.find((p) => p.id === pluginId);
		if (plugin && plugin.config_spec) {
			config = initializeConfigDefaults(plugin.config_spec);
		}

		// Check if this is an AWS plugin and fetch credential status
		if (['s3', 'sns', 'sqs', 'dynamodb', 'kinesis'].includes(pluginId)) {
			fetchAWSCredentialStatus();
		}
	}

	function getNestedValue(obj: any, path: string): any {
		const keys = path.split('.');
		let current = obj;
		for (const key of keys) {
			if (current === undefined || current === null) return undefined;
			current = current[key];
		}
		return current;
	}

	function setNestedValue(obj: any, path: string, value: any) {
		const keys = path.split('.');
		let current = obj;

		for (let i = 0; i < keys.length - 1; i++) {
			const key = keys[i];
			if (!(key in current) || typeof current[key] !== 'object') {
				current[key] = {};
			}
			current = current[key];
		}

		current[keys[keys.length - 1]] = value;
	}

	function clearFieldError(fieldName: string) {
		const newErrors = { ...fieldErrors };
		delete newErrors[fieldName];
		fieldErrors = newErrors;
		configValidated = false;
	}

	async function validateConfig() {
		if (!selectedPluginId) return true;

		try {
			validating = true;
			fieldErrors = {};
			error = null;

			const response = await fetchApi('/ingestion/validate', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({
					plugin_id: selectedPluginId,
					config: config
				})
			});

			if (!response.ok) {
				throw new Error('Failed to validate configuration');
			}

			const result = await response.json();

			if (!result.valid && result.errors && result.errors.length > 0) {
				// Convert array of errors to field map
				const errors: Record<string, string> = {};
				for (const err of result.errors) {
					errors[err.field] = err.message;
				}
				fieldErrors = errors;
				configValidated = false;

				// Build descriptive error message using the first error message
				const errorCount = result.errors.length;
				error =
					errorCount === 1
						? result.errors[0].message
						: `${errorCount} validation errors found: ${result.errors[0].message}`;

				// Scroll to the first errored field
				setTimeout(() => {
					const firstErrorField = result.errors[0].field;
					const element = document.querySelector(`[data-field-path="${firstErrorField}"]`);
					if (element) {
						element.scrollIntoView({ behavior: 'smooth', block: 'center' });
						// Add visual highlight
						element.classList.add('ring-2', 'ring-red-500', 'ring-offset-2');
						setTimeout(() => {
							element.classList.remove('ring-2', 'ring-red-500', 'ring-offset-2');
						}, 2000);
					} else {
						// Fallback: scroll to top if we can't find the field
						window.scrollTo({ top: 0, behavior: 'smooth' });
					}
				}, 100);

				return false;
			}

			configValidated = true;
			return true;
		} catch (err) {
			console.error('Error validating config:', err);
			error = err instanceof Error ? err.message : 'Failed to validate configuration';
			configValidated = false;

			// Scroll to top to show error message
			window.scrollTo({ top: 0, behavior: 'smooth' });

			return false;
		} finally {
			validating = false;
		}
	}

	async function handleNextStep() {
		// Step 1: Basic Info - validate name
		if (currentStep === 1) {
			if (!name.trim()) {
				error = 'Pipeline name is required';
				window.scrollTo({ top: 0, behavior: 'smooth' });
				return;
			}
			error = null;
			currentStep++;
			return;
		}

		// Step 2: Plugin Selection - validate plugin selected
		if (currentStep === 2) {
			if (!selectedPluginId) {
				error = 'Please select a plugin';
				window.scrollTo({ top: 0, behavior: 'smooth' });
				return;
			}
			error = null;
			currentStep++;
			return;
		}

		// Step 3: Configuration - validate config
		if (currentStep === 3) {
			// Check for client-side validation errors first
			if (Object.keys(fieldErrors).length > 0) {
				error = 'Please fix validation errors before proceeding';
				window.scrollTo({ top: 0, behavior: 'smooth' });
				return;
			}

			const isValid = await validateConfig();
			if (!isValid) {
				// Error message and scroll already handled in validateConfig
				return;
			}
			error = null;
			currentStep++;
			return;
		}

		// Step 4: already at last step
	}

	function getFieldType(field: ConfigField): string {
		if (field.type === 'bool' || field.type === 'boolean') return 'checkbox';
		if (field.type === 'int' || field.type === 'number' || field.type === 'integer')
			return 'number';
		if (field.type === 'password' || field.sensitive) return 'password';
		// Check if field name is 'url' to use URL input type for validation
		if (field.name.toLowerCase() === 'url') return 'url';
		return 'text';
	}

	function toggleSection(sectionName: string) {
		expandedSections[sectionName] = !expandedSections[sectionName];
	}

	function isExpanded(sectionName: string): boolean {
		// If the section has been explicitly toggled, use that state
		if (expandedSections[sectionName] !== undefined) {
			return expandedSections[sectionName];
		}

		// Default to expanded for all sections
		return true;
	}

	function shouldHideField(field: ConfigField, configObj: Record<string, any>): boolean {
		// If there's a sibling "use_default" field that's checked, hide all other fields
		if (configObj.use_default === true && field.name !== 'use_default') {
			return true;
		}
		return false;
	}

	onMount(() => {
		fetchPlugins();
	});
</script>

<div class="min-h-screen">
	<!-- Header -->
	<div class="border-b border-gray-200 dark:border-gray-700">
		<div class="container max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-6">
			<div class="flex items-center gap-4">
				<button
					onclick={() => goto('/runs?tab=pipelines')}
					class="p-2 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg transition-colors"
				>
					<IconifyIcon
						icon="material-symbols:arrow-back"
						class="h-6 w-6 text-gray-600 dark:text-gray-400"
					/>
				</button>
				<div>
					<h1 class="text-2xl font-bold text-gray-900 dark:text-gray-100">Create Pipeline</h1>
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
			<div class="flex items-center justify-between">
				{#each steps as step, index}
					<div class="flex items-center {index < steps.length - 1 ? 'flex-1' : ''}">
						<button
							onclick={() => {
								if (
									step.number < currentStep ||
									(step.number === 2 && canProceedToStep2) ||
									(step.number === 3 && canProceedToStep3) ||
									(step.number === 4 && canProceedToStep4)
								) {
									currentStep = step.number;
								}
							}}
							class="flex items-center gap-3 {currentStep === step.number
								? ''
								: 'opacity-60 hover:opacity-80'} transition-opacity"
						>
							<div
								class="flex items-center justify-center w-10 h-10 rounded-full {currentStep ===
								step.number
									? 'bg-earthy-terracotta-600 text-white'
									: currentStep > step.number
										? 'bg-green-600 text-white'
										: 'bg-gray-200 dark:bg-gray-700 text-gray-500 dark:text-gray-400'}"
							>
								{#if currentStep > step.number}
									<IconifyIcon icon="material-symbols:check" class="h-5 w-5" />
								{:else}
									<IconifyIcon icon={step.icon} class="h-5 w-5" />
								{/if}
							</div>
							<span class="text-sm font-medium text-gray-900 dark:text-gray-100 hidden sm:block">
								{step.title}
							</span>
						</button>
						{#if index < steps.length - 1}
							<div
								class="flex-1 h-0.5 mx-4 {currentStep > step.number
									? 'bg-green-600'
									: 'bg-gray-200 dark:bg-gray-700'}"
							></div>
						{/if}
					</div>
				{/each}
			</div>
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
				<div>
					<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
						Pipeline Name <span class="text-red-500">*</span>
					</label>
					<input
						type="text"
						bind:value={name}
						placeholder="e.g., daily-postgres-sync"
						class="w-full px-4 py-2.5 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-earthy-terracotta-600 focus:border-transparent transition-all"
						required
					/>
				</div>
			</div>
		{/if}

		<!-- Step 2: Plugin Selection -->
		{#if currentStep === 2}
			<div
				class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-6"
			>
				<h3 class="text-base font-semibold text-gray-900 dark:text-gray-100 mb-4 flex items-center">
					<IconifyIcon
						icon="material-symbols:extension"
						class="h-5 w-5 mr-2 text-earthy-terracotta-600"
					/>
					Choose Data Source <span class="text-red-500 ml-1">*</span>
				</h3>

				{#if loadingPlugins}
					<div class="flex items-center justify-center py-12">
						<div
							class="animate-spin rounded-full h-8 w-8 border-b-2 border-earthy-terracotta-700"
						></div>
						<span class="ml-3 text-sm text-gray-500">Loading plugins...</span>
					</div>
				{:else}
					<!-- Search Bar -->
					<div class="mb-6">
						<div class="relative">
							<IconifyIcon
								icon="material-symbols:search"
								class="absolute left-4 top-1/2 -translate-y-1/2 h-5 w-5 text-gray-400"
							/>
							<input
								type="text"
								bind:value={pluginSearchQuery}
								placeholder="Search plugins..."
								class="w-full pl-12 pr-4 py-3 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-earthy-terracotta-600 focus:border-transparent transition-all"
							/>
						</div>
						{#if pluginSearchQuery && filteredPlugins.length === 0}
							<p class="mt-3 text-sm text-gray-500 dark:text-gray-400">
								No plugins found matching "{pluginSearchQuery}"
							</p>
						{/if}
					</div>

					<!-- Plugin Grid -->
					<div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
						{#each displayedPlugins as plugin}
							<button
								type="button"
								onclick={() => handlePluginChange(plugin.id)}
								class="relative flex flex-col p-5 border-2 rounded-lg transition-all text-left {selectedPluginId ===
								plugin.id
									? 'border-earthy-terracotta-500 bg-earthy-terracotta-50 dark:bg-earthy-terracotta-900/20 shadow-md'
									: 'border-gray-200 dark:border-gray-700 hover:border-gray-300 dark:hover:border-gray-600 bg-white dark:bg-gray-800 hover:shadow-sm'}"
							>
								<div class="flex items-start gap-3 mb-3">
									{#if plugin.icon}
										<div
											class="h-12 w-12 flex items-center justify-center bg-white dark:bg-gray-700 rounded-lg border border-gray-200 dark:border-gray-600 flex-shrink-0"
										>
											<Icon name={plugin.icon} size="sm" showLabel={false} />
										</div>
									{/if}
									<div class="flex-1 min-w-0">
										<h4 class="text-sm font-semibold text-gray-900 dark:text-gray-100 truncate">
											{plugin.name}
										</h4>
										{#if plugin.category}
											<span
												class="inline-block mt-1 px-2 py-0.5 text-xs font-medium rounded-full bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-400"
											>
												{plugin.category}
											</span>
										{/if}
									</div>
								</div>
								{#if plugin.description}
									<p class="text-xs text-gray-600 dark:text-gray-400 line-clamp-2">
										{plugin.description}
									</p>
								{/if}
								{#if selectedPluginId === plugin.id}
									<IconifyIcon
										icon="material-symbols:check-circle"
										class="h-6 w-6 text-earthy-terracotta-600 absolute top-4 right-4"
									/>
								{/if}
							</button>
						{/each}
					</div>
				{/if}
			</div>
		{/if}

		<!-- Step 3: Plugin Configuration -->
		{#if currentStep === 3}
			{#if configSpec && configSpec.length > 0}
				<div
					class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-6"
				>
					<div class="flex items-center justify-between mb-4">
						<h3 class="text-base font-semibold text-gray-900 dark:text-gray-100 flex items-center">
							<IconifyIcon
								icon="material-symbols:settings"
								class="h-5 w-5 mr-2 text-earthy-terracotta-600"
							/>
							Connection Configuration
						</h3>
						{#if Object.keys(fieldErrors).length > 0}
							<span class="text-sm text-red-600 dark:text-red-400 flex items-center">
								<IconifyIcon icon="material-symbols:error" class="h-4 w-4 mr-1" />
								{Object.keys(fieldErrors).length} error{Object.keys(fieldErrors).length === 1
									? ''
									: 's'}
							</span>
						{/if}
					</div>

					<!-- AWS Credential Status Banner -->
					{#if isAWSPlugin && awsCredentialStatus}
						{#if awsCredentialStatus.available}
							<div
								class="mb-6 bg-green-50 dark:bg-green-900/20 border border-green-200 dark:border-green-800/50 rounded-lg p-4"
							>
								<div class="flex items-start">
									<IconifyIcon
										icon="material-symbols:check-circle"
										class="h-5 w-5 text-green-600 dark:text-green-400 mt-0.5 flex-shrink-0"
									/>
									<div class="ml-3 flex-1">
										<h4 class="text-sm font-semibold text-green-900 dark:text-green-100">
											AWS Credentials Detected
										</h4>
										<p class="text-sm text-green-700 dark:text-green-300 mt-1">
											Credentials found from: {awsCredentialStatus.sources.join(', ')}
										</p>
										<p class="text-xs text-green-600 dark:text-green-400 mt-2">
											You don't need to enter credentials manually. The system will use your
											existing AWS configuration. If you want to use different credentials, uncheck
											"Use default credentials" below.
										</p>
									</div>
								</div>
							</div>
						{:else if awsCredentialStatus.error}
							<div
								class="mb-6 bg-amber-50 dark:bg-amber-900/20 border border-amber-200 dark:border-amber-800/50 rounded-lg p-4"
							>
								<div class="flex items-start">
									<IconifyIcon
										icon="material-symbols:warning"
										class="h-5 w-5 text-amber-600 dark:text-amber-400 mt-0.5 flex-shrink-0"
									/>
									<div class="ml-3 flex-1">
										<h4 class="text-sm font-semibold text-amber-900 dark:text-amber-100">
											AWS Credentials Not Detected
										</h4>
										<p class="text-sm text-amber-700 dark:text-amber-300 mt-1">
											Please provide AWS credentials below to connect to your AWS account.
										</p>
									</div>
								</div>
							</div>
						{/if}
					{:else if isAWSPlugin && loadingAwsStatus}
						<div
							class="mb-6 bg-gray-50 dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg p-4"
						>
							<div class="flex items-center">
								<div
									class="animate-spin rounded-full h-4 w-4 border-b-2 border-earthy-terracotta-700"
								></div>
								<span class="ml-3 text-sm text-gray-600 dark:text-gray-400"
									>Checking for AWS credentials...</span
								>
							</div>
						</div>
					{/if}
					{#snippet renderField(
						field: ConfigField,
						fieldPath: string,
						configObj: Record<string, any>,
						depth: number = 0
					)}
						{#if field.type === 'object' && field.is_array && field.fields}
							<!-- Array of objects (e.g., external_links) -->
							<div class="md:col-span-2">
								<div class="block">
									<span class="text-sm font-medium text-gray-700 dark:text-gray-300 mb-2 block">
										{field.label}
										{#if field.required}
											<span class="text-red-500">*</span>
										{/if}
									</span>
									{#if field.description}
										<p class="text-xs text-gray-500 dark:text-gray-400 mb-2">
											{field.description}
										</p>
									{/if}
									{#if true}
										{@const arrayValue = configObj[field.name] || []}
										<div class="space-y-3">
											{#each arrayValue as item, index}
												<div
													class="border border-gray-200 dark:border-gray-700 rounded-lg p-4 bg-gray-50/50 dark:bg-gray-750/50"
												>
													<div class="flex items-end justify-end mb-3">
														<button
															type="button"
															onclick={(e) => {
																e.preventDefault();
																arrayValue.splice(index, 1);
																configObj[field.name] = [...arrayValue];
																clearFieldError(fieldPath);
															}}
															class="p-1 text-red-600 dark:text-red-400 hover:bg-red-50 dark:hover:bg-red-900/20 rounded transition-colors"
														>
															<IconifyIcon icon="material-symbols:close" class="h-4 w-4" />
														</button>
													</div>
													<div class="grid grid-cols-1 gap-3">
														{#each field.fields as nestedField}
															{@const nestedItemPath = `${fieldPath}[${index}].${nestedField.name}`}
															<div>
																<label class="block">
																	<span
																		class="text-xs font-medium text-gray-700 dark:text-gray-300 mb-1 block"
																	>
																		{nestedField.label}
																		{#if nestedField.required}
																			<span class="text-red-500">*</span>
																		{/if}
																	</span>
																	<input
																		type={getFieldType(nestedField)}
																		bind:value={item[nestedField.name]}
																		oninput={(e) => {
																			configObj[field.name] = [...arrayValue];
																			clearFieldError(nestedItemPath);
																		}}
																		placeholder={nestedField.placeholder}
																		required={nestedField.required}
																		data-field-path={nestedItemPath}
																		class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-earthy-terracotta-600 focus:border-transparent transition-all {fieldErrors[
																			nestedItemPath
																		]
																			? 'border-red-500 dark:border-red-500'
																			: ''}"
																	/>
																	{#if fieldErrors[nestedItemPath]}
																		<p
																			class="mt-1.5 text-sm text-red-600 dark:text-red-400 flex items-start"
																		>
																			<IconifyIcon
																				icon="material-symbols:error"
																				class="h-4 w-4 mr-1 mt-0.5 flex-shrink-0"
																			/>
																			{fieldErrors[nestedItemPath]}
																		</p>
																	{/if}
																</label>
															</div>
														{/each}
													</div>
												</div>
											{/each}
											<button
												type="button"
												onclick={(e) => {
													e.preventDefault();
													const newItem: Record<string, any> = {};
													// Initialize with default values
													field.fields?.forEach((f) => {
														newItem[f.name] = f.default || '';
													});
													arrayValue.push(newItem);
													configObj[field.name] = [...arrayValue];
													clearFieldError(fieldPath);
												}}
												class="w-full px-4 py-2.5 border-2 border-dashed border-gray-300 dark:border-gray-600 rounded-lg text-sm text-gray-600 dark:text-gray-400 hover:border-earthy-terracotta-600 hover:text-earthy-terracotta-600 dark:hover:text-earthy-terracotta-400 transition-colors flex items-center justify-center gap-2"
											>
												<IconifyIcon icon="material-symbols:add" class="h-5 w-5" />
												Add {field.label}
											</button>
										</div>
									{/if}
									{#if fieldErrors[fieldPath]}
										<p class="mt-1.5 text-sm text-red-600 dark:text-red-400 flex items-start">
											<IconifyIcon
												icon="material-symbols:error"
												class="h-4 w-4 mr-1 mt-0.5 flex-shrink-0"
											/>
											{fieldErrors[fieldPath]}
										</p>
									{/if}
								</div>
							</div>
						{:else if field.type === 'object' && field.fields}
							<div class="md:col-span-2">
								<div class="border border-gray-200 dark:border-gray-700 rounded-lg overflow-hidden">
									<button
										type="button"
										onclick={() => toggleSection(fieldPath)}
										class="w-full flex items-center justify-between p-3 hover:bg-gray-50 dark:hover:bg-gray-750 transition-colors text-left"
									>
										<div class="flex items-center">
											<IconifyIcon
												icon={isExpanded(fieldPath)
													? 'material-symbols:expand-more'
													: 'material-symbols:chevron-right'}
												class="h-5 w-5 text-gray-500 dark:text-gray-400 transition-transform"
											/>
											<span class="ml-2 text-sm font-medium text-gray-700 dark:text-gray-300">
												{field.label}
												{#if field.required}
													<span class="text-red-500 ml-1">*</span>
												{/if}
											</span>
										</div>
										{#if field.description}
											<span class="text-xs text-gray-500 dark:text-gray-400 ml-2 truncate"
												>{field.description}</span
											>
										{/if}
									</button>
									{#if isExpanded(fieldPath)}
										<div
											class="px-4 pb-4 border-t border-gray-200 dark:border-gray-700 bg-gray-50/50 dark:bg-gray-750/50"
										>
											<div class="grid grid-cols-1 md:grid-cols-2 gap-4 pt-4">
												{#each field.fields as nestedField}
													{@const nestedPath = `${fieldPath}.${nestedField.name}`}
													{@const nestedConfigObj = configObj[field.name] || {}}
													{#if !shouldHideField(nestedField, nestedConfigObj)}
														{@render renderField(
															nestedField,
															nestedPath,
															nestedConfigObj,
															depth + 1
														)}
													{/if}
												{/each}
											</div>
										</div>
									{/if}
								</div>
							</div>
						{:else if field.type === 'bool' || field.type === 'boolean'}
							<div class="md:col-span-2">
								<label
									class="flex items-start p-3 border rounded-lg hover:bg-gray-50 dark:hover:bg-gray-750 cursor-pointer transition-colors {fieldErrors[
										fieldPath
									]
										? 'border-red-500 dark:border-red-500'
										: 'border-gray-200 dark:border-gray-700'}"
									data-field-path={fieldPath}
								>
									<input
										type="checkbox"
										bind:checked={configObj[field.name]}
										onchange={() => clearFieldError(fieldPath)}
										class="h-4 w-4 mt-0.5 text-earthy-terracotta-700 focus:ring-earthy-terracotta-600 border-gray-300 rounded"
									/>
									<div class="ml-3 flex-1">
										<span class="text-sm font-medium text-gray-700 dark:text-gray-300">
											{field.label}
										</span>
										{#if field.description}
											<p class="text-xs text-gray-500 dark:text-gray-400 mt-0.5">
												{field.description}
											</p>
										{/if}
										{#if fieldErrors[fieldPath]}
											<p class="mt-1.5 text-sm text-red-600 dark:text-red-400 flex items-start">
												<IconifyIcon
													icon="material-symbols:error"
													class="h-4 w-4 mr-1 mt-0.5 flex-shrink-0"
												/>
												{fieldErrors[fieldPath]}
											</p>
										{/if}
									</div>
								</label>
							</div>
						{:else if field.type === 'multiselect'}
							<!-- Array/List field -->
							<div class="md:col-span-2">
								<div class="block" data-field-path={fieldPath}>
									<span class="text-sm font-medium text-gray-700 dark:text-gray-300 mb-2 block">
										{field.label}
										{#if field.required}
											<span class="text-red-500">*</span>
										{/if}
									</span>
									{#if field.description}
										<p class="text-xs text-gray-500 dark:text-gray-400 mb-2">
											{field.description}
										</p>
									{/if}
									{#if true}
										{@const arrayValue = configObj[field.name] || []}
										{@const newItemKey = `${fieldPath}_new_item`}
										<div class="space-y-2">
											{#each arrayValue as item, index}
												<div class="flex items-center gap-2">
													<input
														type="text"
														value={item}
														oninput={(e) => {
															const target = e.target as HTMLInputElement;
															arrayValue[index] = target.value;
															configObj[field.name] = [...arrayValue];
															clearFieldError(fieldPath);
														}}
														placeholder={field.placeholder}
														class="flex-1 px-4 py-2.5 border rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-earthy-terracotta-600 focus:border-transparent transition-all {fieldErrors[
															fieldPath
														]
															? 'border-red-500 dark:border-red-500'
															: 'border-gray-300 dark:border-gray-600'}"
													/>
													<button
														type="button"
														onclick={(e) => {
															e.preventDefault();
															e.stopPropagation();
															arrayValue.splice(index, 1);
															configObj[field.name] = [...arrayValue];
															clearFieldError(fieldPath);
														}}
														class="p-2 text-red-600 dark:text-red-400 hover:bg-red-50 dark:hover:bg-red-900/20 rounded-lg transition-colors"
														aria-label="Remove item"
													>
														<IconifyIcon icon="material-symbols:close" class="h-5 w-5" />
													</button>
												</div>
											{/each}
											<div class="flex items-center gap-2">
												<input
													type="text"
													placeholder={`Type to add ${field.label.toLowerCase()}...`}
													onkeydown={(e) => {
														if (e.key === 'Enter') {
															e.preventDefault();
															const target = e.target as HTMLInputElement;
															const value = target.value.trim();
															if (value) {
																const newArray = [...arrayValue, value];
																configObj[field.name] = newArray;
																target.value = '';
																clearFieldError(fieldPath);
															}
														}
													}}
													class="flex-1 px-4 py-2.5 border-2 border-dashed border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-earthy-terracotta-600 focus:border-earthy-terracotta-600 transition-all"
												/>
											</div>
											<p class="text-xs text-gray-500 dark:text-gray-400">
												Press Enter to add items
											</p>
										</div>
									{/if}
									{#if fieldErrors[fieldPath]}
										<p class="mt-1.5 text-sm text-red-600 dark:text-red-400 flex items-start">
											<IconifyIcon
												icon="material-symbols:error"
												class="h-4 w-4 mr-1 mt-0.5 flex-shrink-0"
											/>
											{fieldErrors[fieldPath]}
										</p>
									{/if}
								</div>
							</div>
						{:else}
							<div>
								<label class="block">
									<span class="text-sm font-medium text-gray-700 dark:text-gray-300 mb-2 block">
										{field.label}
										{#if field.required}
											<span class="text-red-500">*</span>
										{/if}
									</span>
									{#if field.description}
										<p class="text-xs text-gray-500 dark:text-gray-400 mb-2">
											{field.description}
										</p>
									{/if}
									{#if field.options && field.options.length > 0}
										<select
											value={configObj[field.name] || ''}
											onchange={(e) => {
												configObj[field.name] = (e.target as HTMLSelectElement).value;
												clearFieldError(fieldPath);
											}}
											data-field-path={fieldPath}
											class="w-full px-4 py-2.5 border rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-earthy-terracotta-600 focus:border-transparent transition-all {fieldErrors[
												fieldPath
											]
												? 'border-red-500 dark:border-red-500'
												: 'border-gray-300 dark:border-gray-600'}"
											required={field.required}
										>
											<option value="">Select...</option>
											{#each field.options as option}
												<option value={option.value}>{option.label}</option>
											{/each}
										</select>
									{:else}
										<input
											type={getFieldType(field)}
											value={configObj[field.name] || ''}
											oninput={(e) => {
												const target = e.target as HTMLInputElement;
												configObj[field.name] =
													field.type === 'int' || field.type === 'number'
														? Number(target.value)
														: target.value;
												clearFieldError(fieldPath);
											}}
											placeholder={field.placeholder ||
												(field.default ? String(field.default) : '')}
											data-field-path={fieldPath}
											class="w-full px-4 py-2.5 border rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-earthy-terracotta-600 focus:border-transparent transition-all {field.type ===
											'password'
												? 'font-mono'
												: ''} {fieldErrors[fieldPath]
												? 'border-red-500 dark:border-red-500'
												: 'border-gray-300 dark:border-gray-600'}"
											required={field.required}
										/>
									{/if}
									{#if fieldErrors[fieldPath]}
										<p class="mt-1.5 text-sm text-red-600 dark:text-red-400 flex items-start">
											<IconifyIcon
												icon="material-symbols:error"
												class="h-4 w-4 mr-1 mt-0.5 flex-shrink-0"
											/>
											{fieldErrors[fieldPath]}
										</p>
									{/if}
								</label>
							</div>
						{/if}
					{/snippet}

					<div class="grid grid-cols-1 md:grid-cols-2 gap-4">
						{#each configSpec as field}
							{@render renderField(field, field.name, config, 0)}
						{/each}
					</div>
				</div>
			{:else}
				<div
					class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-6 text-center py-12"
				>
					<IconifyIcon
						icon="material-symbols:check-circle"
						class="h-12 w-12 mx-auto text-green-600 mb-4"
					/>
					<h3 class="text-lg font-semibold text-gray-900 dark:text-gray-100 mb-2">
						No Configuration Needed
					</h3>
					<p class="text-sm text-gray-600 dark:text-gray-400 mb-6">
						This plugin doesn't require additional configuration
					</p>
				</div>
			{/if}
		{/if}

		<!-- Step 4: Schedule Configuration -->
		{#if currentStep === 4}
			<div
				class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-6"
			>
				<h3 class="text-base font-semibold text-gray-900 dark:text-gray-100 mb-4 flex items-center">
					<IconifyIcon
						icon="material-symbols:schedule"
						class="h-5 w-5 mr-2 text-earthy-terracotta-600"
					/>
					Schedule Configuration
				</h3>
				<div class="space-y-5">
					<div>
						<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
							Cron Expression
							<span class="text-xs font-normal text-gray-500 ml-1">(Optional)</span>
						</label>
						<input
							type="text"
							bind:value={cronExpression}
							placeholder="0 2 * * *"
							class="w-full px-4 py-2.5 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-earthy-terracotta-600 focus:border-transparent font-mono text-sm transition-all"
						/>
						<div class="mt-2 flex items-start">
							<IconifyIcon
								icon="material-symbols:info-outline"
								class="h-4 w-4 text-gray-400 mt-0.5 flex-shrink-0"
							/>
							<p class="ml-2 text-xs text-gray-500 dark:text-gray-400">
								Leave empty for manual-only pipeline.
							</p>
						</div>

						{#if cronDescription}
							<div
								class="mt-3 p-3 bg-green-50 dark:bg-green-900/10 border border-green-200 dark:border-green-800 rounded-lg"
							>
								<div class="flex items-start">
									<IconifyIcon
										icon="material-symbols:check-circle"
										class="h-5 w-5 text-green-600 dark:text-green-400 mt-0.5 flex-shrink-0"
									/>
									<div class="ml-2 flex-1">
										<p class="text-sm font-medium text-green-800 dark:text-green-300">
											{cronDescription}
										</p>
										{#if cronNextRuns.length > 0}
											<div class="mt-2">
												<p class="text-xs font-medium text-green-700 dark:text-green-400 mb-1">
													Next 5 runs:
												</p>
												<ul class="text-xs text-green-700 dark:text-green-400 space-y-0.5">
													{#each cronNextRuns as run}
														<li class="font-mono">
															{run.toLocaleString('en-US', {
																weekday: 'short',
																year: 'numeric',
																month: 'short',
																day: 'numeric',
																hour: '2-digit',
																minute: '2-digit',
																second: '2-digit',
																hour12: false
															})}
														</li>
													{/each}
												</ul>
											</div>
										{/if}
									</div>
								</div>
							</div>
						{:else if cronExpression.trim()}
							<div
								class="mt-3 p-3 bg-red-50 dark:bg-red-900/10 border border-red-200 dark:border-red-800 rounded-lg"
							>
								<div class="flex items-start">
									<IconifyIcon
										icon="material-symbols:error"
										class="h-5 w-5 text-red-600 dark:text-red-400 mt-0.5 flex-shrink-0"
									/>
									<p class="ml-2 text-sm text-red-800 dark:text-red-300">Invalid cron expression</p>
								</div>
							</div>
						{/if}
					</div>

					{#if hasSchedule}
						<div
							class="bg-amber-50 dark:bg-amber-900/20 border border-amber-200 dark:border-amber-800/50 rounded-lg p-4"
						>
							<label class="flex items-start cursor-pointer">
								<input
									type="checkbox"
									bind:checked={disableSchedule}
									class="h-4 w-4 mt-0.5 text-earthy-terracotta-700 focus:ring-earthy-terracotta-600 border-gray-300 rounded"
								/>
								<div class="ml-3">
									<span class="text-sm font-medium text-gray-900 dark:text-gray-100">
										Disable scheduled runs
									</span>
									<p class="text-xs text-gray-600 dark:text-gray-400 mt-1">
										Pipeline can still be triggered manually, but won't run automatically on the
										schedule above
									</p>
								</div>
							</label>
						</div>
					{/if}
				</div>
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
					<Button variant="clear" click={() => goto('/runs?tab=pipelines')} text="Cancel" />
				{/if}
			</div>
			<div class="flex items-center gap-3">
				{#if currentStep < 4}
					<Button
						variant="filled"
						click={handleNextStep}
						text={currentStep === 3 && validating ? 'Validating...' : 'Next'}
						icon="material-symbols:arrow-forward"
						disabled={validating ||
							(currentStep === 1 && !canProceedToStep2) ||
							(currentStep === 2 && !canProceedToStep3)}
					/>
				{:else}
					<div class="text-sm text-gray-500 dark:text-gray-400 mr-4">
						{#if hasSchedule}
							<IconifyIcon icon="material-symbols:schedule" class="inline h-4 w-4 mr-1" />
							Scheduled pipeline
						{:else}
							<IconifyIcon
								icon="material-symbols:play-circle-outline"
								class="inline h-4 w-4 mr-1"
							/>
							Manual-only pipeline
						{/if}
					</div>
					<Button
						variant="filled"
						click={handleSave}
						text={saving ? 'Creating...' : 'Create Pipeline'}
						disabled={saving || !name || !selectedPluginId}
						icon="material-symbols:check"
					/>
				{/if}
			</div>
		</div>
	</div>
</div>
