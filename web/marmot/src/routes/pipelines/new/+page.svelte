<script lang="ts">
	import { onDestroy, onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { get } from 'svelte/store';
	import { fetchApi } from '$lib/api';
	import { encryptionConfigured } from '$lib/stores/encryption';
	import { toasts } from '$lib/stores/toast';
	import Button from '$components/ui/Button.svelte';
	import PipelineDomain from '$components/domain/PipelineDomain.svelte';
	import { assignPipeline, errorMessage as domainErrorMessage } from '$lib/domains/api';
	import IconifyIcon from '@iconify/svelte';
	import Icon from '$components/ui/Icon.svelte';
	import Stepper from '$components/ui/Stepper.svelte';
	import Step from '$components/ui/Step.svelte';
	import cronstrue from 'cronstrue/i18n';
	import { Cron } from 'croner';
	import { m } from '$lib/paraglide/messages';
	import { getLocale } from '$lib/paraglide/runtime';
	import { formatList } from '$lib/utils';

	// Example filter patterns are regexes, so they are technical tokens rather than copy
	const includePatternExample = '^public_.*';
	const excludePatternExample = '^_tmp_.*';

	interface ConfigField {
		name: string;
		type: string;
		label: string;
		description: string;
		required: boolean;
		default?: unknown;
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
		show_when?: { field: string; value: string };
		hidden?: boolean;
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
	let pluginsStillLoading = $state(false);
	let pluginPollTimer: ReturnType<typeof setTimeout> | null = null;
	let selectedPluginId = $state('');
	let name = $state('');
	let pipelineDomain = $state('');
	let cronExpression = $state('');
	let disableSchedule = $state(false);
	type ConfigValue =
		| string
		| number
		| boolean
		| null
		| undefined
		| ConfigValue[]
		| { [key: string]: ConfigValue };
	interface PipelineConfig {
		tags?: string[];
		external_links?: { name: string; url: string }[];
		filter?: { include?: string[]; exclude?: string[] };
		credentials?: Record<string, ConfigValue>;
		source_type?: string;
		[key: string]: ConfigValue | undefined;
	}
	let config = $state<PipelineConfig>({
		tags: [],
		external_links: [],
		filter: { include: [], exclude: [] }
	});
	let saving = $state(false);
	let error = $state<string | null>(null);
	let pluginSearchQuery = $state('');
	let currentStep = $state(1);
	const steps = [
		{ title: m.pipelines_step_basic_info() },
		{ title: m.pipelines_step_choose_plugin() },
		{ title: m.pipelines_step_configure() },
		{ title: m.pipelines_step_schedule_filter() }
	];
	let validating = $state(false);
	let fieldErrors = $state<Record<string, string>>({});
	let expandedSections = $state<Record<string, boolean>>({});
	let awsCredentialStatus = $state<{
		available: boolean;
		sources: string[];
		error?: string;
	} | null>(null);
	let loadingAwsStatus = $state(false);

	let totalSteps = $state(0);

	let canProceedToStep2 = $derived(name.trim() !== '');
	let canProceedToStep3 = $derived(selectedPluginId !== '');
	let configValidated = $state(false);
	let canProceedToStep4 = $derived(configValidated); // Must validate config first

	function canNavigateToStep(stepNumber: number): boolean {
		if (stepNumber === 2) return canProceedToStep2;
		if (stepNumber === 3) return canProceedToStep3;
		if (stepNumber === 4) return canProceedToStep4;
		return false;
	}

	let selectedPlugin = $derived(plugins.find((p) => p.id === selectedPluginId) || null);

	let configSpec = $derived(selectedPlugin?.config_spec || null);

	let isAWSPlugin = $derived(
		selectedPluginId && ['s3', 'sns', 'sqs', 'dynamodb', 'kinesis'].includes(selectedPluginId)
	);
	let hasSchedule = $derived(cronExpression.trim() !== '');

	let cronDescription = $derived.by(() => {
		if (!cronExpression.trim()) return null;
		try {
			return cronstrue.toString(cronExpression, { verbose: true, locale: getLocale() });
		} catch (e) {
			try {
				return cronstrue.toString(cronExpression, { verbose: true });
			} catch {
				return null;
			}
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
		plugins
			.filter((plugin) => {
				const searchLower = pluginSearchQuery.toLowerCase();
				return (
					plugin.name.toLowerCase().includes(searchLower) ||
					plugin.description?.toLowerCase().includes(searchLower) ||
					plugin.id.toLowerCase().includes(searchLower)
				);
			})
			.sort((a, b) => a.name.localeCompare(b.name))
	);

	let displayedPlugins = $derived(filteredPlugins);

	async function fetchPlugins() {
		try {
			loadingPlugins = true;
			const response = await fetchApi('/plugins');
			if (!response.ok) throw new Error('Failed to fetch plugins');
			const data = await response.json();
			plugins = Array.isArray(data?.plugins) ? data.plugins : [];
			pluginsStillLoading = Boolean(data?.loading);
		} catch (err) {
			console.error('Error fetching plugins:', err);
			error = m.pipelines_error_load_plugins();
		} finally {
			loadingPlugins = false;
			if (pluginPollTimer) {
				clearTimeout(pluginPollTimer);
				pluginPollTimer = null;
			}
			if (pluginsStillLoading) {
				pluginPollTimer = setTimeout(fetchPlugins, 2000);
			}
		}
	}

	async function handleSave() {
		try {
			saving = true;
			error = null;

			const enabled = cronExpression.trim() === '' ? true : !disableSchedule;
			const cleanedConfig = selectedPlugin?.config_spec
				? cleanConfigForSubmit(config, selectedPlugin.config_spec)
				: config;

			const body = {
				name,
				plugin_id: selectedPluginId,
				config: cleanedConfig,
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
				throw new Error(data.error || m.pipelines_error_create());
			}

			// The pipeline exists either way; a failed assignment leaves it in Unassigned.
			if (pipelineDomain) {
				const created = await response.json();
				await assignPipeline(created.id, pipelineDomain, false).catch((error) =>
					toasts.error(domainErrorMessage(error))
				);
			}

			goto(resolve('/runs?tab=pipelines'));
		} catch (err) {
			error = err instanceof Error ? err.message : m.pipelines_error_create();
		} finally {
			saving = false;
		}
	}

	function initializeConfigDefaults(
		fields: ConfigField[],
		configObj: Record<string, ConfigValue> = {}
	) {
		for (const field of fields) {
			if (field.type === 'object' && field.is_array) {
				configObj[field.name] = [];
			} else if (field.type === 'object' && field.fields) {
				const nested: Record<string, ConfigValue> = {};
				configObj[field.name] = nested;
				initializeConfigDefaults(field.fields, nested);
			} else if (field.type === 'multiselect') {
				configObj[field.name] = [];
			} else if (field.default !== undefined && field.default !== null) {
				configObj[field.name] = field.default as ConfigValue;
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
		// Reset config but preserve base fields (tags, external_links, filter)
		const savedTags = config.tags || [];
		const savedLinks = config.external_links || [];
		const savedFilter = config.filter || { include: [], exclude: [] };
		config = {};
		fieldErrors = {};
		configValidated = false;
		awsCredentialStatus = null;

		// Find selected plugin and populate defaults
		const plugin = plugins.find((p) => p.id === pluginId);
		if (plugin && plugin.config_spec) {
			config = initializeConfigDefaults(plugin.config_spec);
		}

		// Restore base fields
		config.tags = savedTags;
		config.external_links = savedLinks;
		config.filter = savedFilter;

		// Check if this is an AWS plugin and fetch credential status
		if (['s3', 'sns', 'sqs', 'dynamodb', 'kinesis'].includes(pluginId)) {
			fetchAWSCredentialStatus();
		}
	}

	function clearFieldError(fieldName: string) {
		const newErrors = { ...fieldErrors };
		delete newErrors[fieldName];
		fieldErrors = newErrors;
		configValidated = false;
	}

	function cleanConfigForSubmit(
		configObj: Record<string, ConfigValue>,
		fields: ConfigField[]
	): Record<string, ConfigValue> {
		const cleaned = { ...configObj };
		for (const field of fields) {
			if (field.show_when) {
				const currentValue = cleaned[field.show_when.field];
				if (currentValue !== field.show_when.value) {
					delete cleaned[field.name];
				}
			}
		}
		return cleaned;
	}

	async function validateConfig() {
		if (!selectedPluginId) return true;

		try {
			validating = true;
			fieldErrors = {};
			error = null;

			const cleanedConfig = selectedPlugin?.config_spec
				? cleanConfigForSubmit(config, selectedPlugin.config_spec)
				: config;

			const response = await fetchApi('/ingestion/validate', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({
					plugin_id: selectedPluginId,
					config: cleanedConfig
				})
			});

			if (!response.ok) {
				throw new Error(m.pipelines_error_validate());
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
						: m.pipelines_error_validation_summary({
								count: errorCount,
								message: result.errors[0].message
							});

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
			error = err instanceof Error ? err.message : m.pipelines_error_validate();
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
				error = m.pipelines_error_name_required();
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
				error = m.pipelines_error_select_plugin();
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
				error = m.pipelines_error_fix_validation();
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

		// Default collapsed for the asset filter and credentials sections.
		// These are optional and most users leave them at their defaults,
		// so collapsing keeps the form tidy and avoids a large box jumping
		// into view when the step opens.
		if (sectionName === 'asset_filter' || sectionName === 'credentials') return false;

		// Default to expanded for all other sections
		return true;
	}

	function shouldHideField(
		field: ConfigField,
		configObj: Record<string, ConfigValue>,
		rootConfig?: Record<string, ConfigValue>
	): boolean {
		// If there's a sibling "use_default" field that's checked, hide all other fields
		if (configObj.use_default === true && field.name !== 'use_default') {
			return true;
		}
		// Check show_when condition against the root config (for top-level sibling fields)
		if (field.show_when) {
			const checkObj = rootConfig || configObj;
			const currentValue = checkObj[field.show_when.field];
			if (currentValue !== field.show_when.value) {
				return true;
			}
		}
		return false;
	}

	function autoDetectSourceType() {
		if (!configSpec) return;
		const hasSourceType = configSpec.some((f) => f.name === 'source_type');
		if (!hasSourceType) return;
		// Scan all string values in config for s3:// or git:: prefixes
		let detected = 'local';
		for (const field of configSpec) {
			if (field.hidden || field.type !== 'string') continue;
			const val = config[field.name];
			if (typeof val === 'string') {
				if (val.startsWith('s3://')) {
					detected = 's3';
					break;
				}
				if (val.startsWith('git::')) {
					detected = 'git';
					break;
				}
			}
		}
		config.source_type = detected;
	}

	onMount(() => {
		if (!get(encryptionConfigured)) {
			toasts.error(m.pipelines_error_encryption_not_configured());
			goto(resolve('/runs'));
			return;
		}
		fetchPlugins();
	});

	onDestroy(() => {
		if (pluginPollTimer) clearTimeout(pluginPollTimer);
	});
</script>

<div class="min-h-screen">
	<!-- Header -->
	<div class="border-b border-gray-200 dark:border-gray-700">
		<div class="container max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-6">
			<div class="flex items-center gap-4">
				<button
					onclick={() => goto(resolve('/runs?tab=pipelines'))}
					class="p-2 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg transition-colors"
				>
					<IconifyIcon
						icon="material-symbols:arrow-back"
						class="h-6 w-6 text-gray-600 dark:text-gray-400"
					/>
				</button>
				<div>
					<h1 class="text-2xl font-bold text-gray-900 dark:text-gray-100">
						{m.pipelines_create_pipeline()}
					</h1>
					<p class="text-sm text-gray-600 dark:text-gray-400 mt-1">
						{m.pipelines_step_indicator({
							current: currentStep,
							total: steps.length,
							title: steps[currentStep - 1].title
						})}
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
				<Step title={m.pipelines_step_basic_info()} icon="material-symbols:info-outline" />
				<Step title={m.pipelines_step_choose_plugin()} icon="material-symbols:extension" />
				<Step title={m.pipelines_step_configure()} icon="material-symbols:settings" />
				<Step title={m.pipelines_step_schedule_filter()} icon="material-symbols:schedule" />
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
					{m.pipelines_basic_info_heading()}
				</h3>
				<div>
					<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
						{m.pipelines_name_label()} <span class="text-red-500">*</span>
					</label>
					<input
						type="text"
						bind:value={name}
						placeholder={m.pipelines_name_placeholder()}
						onkeydown={(e) => {
							if (e.key === 'Enter' && canProceedToStep2) {
								e.preventDefault();
								handleNextStep();
							}
						}}
						class="w-full px-4 py-2.5 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-earthy-terracotta-600 focus:border-transparent transition-all"
						required
					/>
				</div>

				<PipelineDomain bind:value={pipelineDomain} />

				<!-- Tags -->
				<div class="mt-6">
					<span class="text-sm font-medium text-gray-700 dark:text-gray-300 mb-2 block">
						{m.common_tags()}
					</span>
					<p class="text-xs text-gray-500 dark:text-gray-400 mb-2">
						{m.pipelines_tags_hint()}
					</p>
					{#if true}
						{@const tagsValue = config.tags || []}
						<div class="space-y-2">
							{#each tagsValue as item, index (index)}
								<div class="flex items-center gap-2">
									<input
										type="text"
										value={item}
										oninput={(e) => {
											const target = e.target as HTMLInputElement;
											tagsValue[index] = target.value;
											config.tags = [...tagsValue];
										}}
										class="flex-1 px-4 py-2.5 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-earthy-terracotta-600 focus:border-transparent transition-all"
									/>
									<button
										type="button"
										onclick={(e) => {
											e.preventDefault();
											tagsValue.splice(index, 1);
											config.tags = [...tagsValue];
										}}
										class="p-2 text-red-600 dark:text-red-400 hover:bg-red-50 dark:hover:bg-red-900/20 rounded-lg transition-colors"
										aria-label={m.pipelines_remove_tag_aria()}
									>
										<IconifyIcon icon="material-symbols:close" class="h-5 w-5" />
									</button>
								</div>
							{/each}
							<div class="flex items-center gap-2">
								<input
									type="text"
									placeholder={m.pipelines_tags_typeahead_placeholder()}
									onkeydown={(e) => {
										if (e.key === 'Enter') {
											e.preventDefault();
											const target = e.target as HTMLInputElement;
											const value = target.value.trim();
											if (value) {
												config.tags = [...(config.tags || []), value];
												target.value = '';
											}
										}
									}}
									class="flex-1 px-4 py-2.5 border-2 border-dashed border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-earthy-terracotta-600 focus:border-earthy-terracotta-600 transition-all"
								/>
							</div>
							<p class="text-xs text-gray-500 dark:text-gray-400">
								{m.pipelines_press_enter_hint()}
							</p>
						</div>
					{/if}
				</div>

				<!-- External Links -->
				<div class="mt-6">
					<span class="text-sm font-medium text-gray-700 dark:text-gray-300 mb-2 block">
						{m.pipelines_external_links_label()}
					</span>
					<p class="text-xs text-gray-500 dark:text-gray-400 mb-2">
						{m.pipelines_external_links_hint()}
					</p>
					{#if true}
						{@const linksValue = config.external_links || []}
						<div class="space-y-3">
							{#each linksValue as item, index (index)}
								<div
									class="border border-gray-200 dark:border-gray-700 rounded-lg p-4 bg-gray-50/50 dark:bg-gray-800/50"
								>
									<div class="flex items-end justify-end mb-3">
										<button
											type="button"
											onclick={(e) => {
												e.preventDefault();
												linksValue.splice(index, 1);
												config.external_links = [...linksValue];
											}}
											class="p-1 text-red-600 dark:text-red-400 hover:bg-red-50 dark:hover:bg-red-900/20 rounded transition-colors"
										>
											<IconifyIcon icon="material-symbols:close" class="h-4 w-4" />
										</button>
									</div>
									<div class="grid grid-cols-1 gap-3">
										<div>
											<label class="block">
												<span
													class="text-xs font-medium text-gray-700 dark:text-gray-300 mb-1 block"
												>
													{m.common_name()}
												</span>
												<input
													type="text"
													bind:value={item.name}
													oninput={() => {
														config.external_links = [...linksValue];
													}}
													placeholder={m.pipelines_link_name_placeholder()}
													class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-earthy-terracotta-600 focus:border-transparent transition-all"
												/>
											</label>
										</div>
										<div>
											<label class="block">
												<span
													class="text-xs font-medium text-gray-700 dark:text-gray-300 mb-1 block"
												>
													{m.pipelines_url_label()}
												</span>
												<input
													type="url"
													bind:value={item.url}
													oninput={() => {
														config.external_links = [...linksValue];
													}}
													placeholder="https://..."
													class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-earthy-terracotta-600 focus:border-transparent transition-all"
												/>
											</label>
										</div>
									</div>
								</div>
							{/each}
							<button
								type="button"
								onclick={(e) => {
									e.preventDefault();
									config.external_links = [...(config.external_links || []), { name: '', url: '' }];
								}}
								class="w-full px-4 py-2.5 border-2 border-dashed border-gray-300 dark:border-gray-600 rounded-lg text-sm text-gray-600 dark:text-gray-400 hover:border-earthy-terracotta-600 hover:text-earthy-terracotta-600 dark:hover:text-earthy-terracotta-400 transition-colors flex items-center justify-center gap-2"
							>
								<IconifyIcon icon="material-symbols:add" class="h-5 w-5" />
								{m.pipelines_add_external_link()}
							</button>
						</div>
					{/if}
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
					{m.pipelines_choose_data_source_heading()} <span class="text-red-500 ml-1">*</span>
				</h3>

				{#if pluginsStillLoading}
					<div
						class="mb-4 flex items-start gap-3 rounded-lg border border-amber-200 bg-amber-50 dark:border-amber-800 dark:bg-amber-900/20 p-3 text-sm text-amber-800 dark:text-amber-200"
						role="status"
					>
						<IconifyIcon icon="material-symbols:info-outline" class="h-5 w-5 flex-shrink-0" />
						<span>{m.pipelines_plugins_still_loading_list()}</span>
					</div>
				{/if}

				{#if loadingPlugins}
					<div class="flex items-center justify-center py-12">
						<div
							class="animate-spin rounded-full h-8 w-8 border-b-2 border-earthy-terracotta-700"
						></div>
						<span class="ml-3 text-sm text-gray-500">{m.pipelines_loading_plugins()}</span>
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
								placeholder={m.pipelines_search_plugins_placeholder()}
								onkeydown={(e) => {
									if (e.key === 'Enter' && displayedPlugins.length === 1) {
										e.preventDefault();
										handlePluginChange(displayedPlugins[0].id);
									}
								}}
								class="w-full pl-12 pr-4 py-3 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-earthy-terracotta-600 focus:border-transparent transition-all"
							/>
						</div>
						{#if pluginSearchQuery && filteredPlugins.length === 0}
							<p class="mt-3 text-sm text-gray-500 dark:text-gray-400">
								{m.pipelines_no_plugins_match({ query: pluginSearchQuery })}
							</p>
						{/if}
					</div>

					<!-- Plugin Grid -->
					<div
						class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4"
						role="listbox"
						aria-label={m.pipelines_available_plugins_aria()}
					>
						{#each displayedPlugins as plugin (plugin.id)}
							<button
								type="button"
								onclick={() => handlePluginChange(plugin.id)}
								onkeydown={(e) => {
									if (e.key === 'Enter' || e.key === ' ') {
										e.preventDefault();
										handlePluginChange(plugin.id);
									}
								}}
								role="option"
								aria-selected={selectedPluginId === plugin.id}
								class="relative flex flex-col p-5 border-2 rounded-lg transition-all text-left focus:outline-none focus:ring-2 focus:ring-earthy-terracotta-500 focus:ring-offset-2 {selectedPluginId ===
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
							{m.pipelines_connection_config_heading()}
						</h3>
						{#if Object.keys(fieldErrors).length > 0}
							<span class="text-sm text-red-600 dark:text-red-400 flex items-center">
								<IconifyIcon icon="material-symbols:error" class="h-4 w-4 mr-1" />
								{m.pipelines_error_count({ count: Object.keys(fieldErrors).length })}
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
											{m.pipelines_aws_creds_detected_heading()}
										</h4>
										<p class="text-sm text-green-700 dark:text-green-300 mt-1">
											{m.pipelines_aws_creds_sources({
												sources: formatList(awsCredentialStatus.sources)
											})}
										</p>
										<p class="text-xs text-green-600 dark:text-green-400 mt-2">
											{m.pipelines_aws_creds_detected_hint()}
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
											{m.pipelines_aws_creds_not_detected_heading()}
										</h4>
										<p class="text-sm text-amber-700 dark:text-amber-300 mt-1">
											{m.pipelines_aws_creds_not_detected_hint()}
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
									>{m.pipelines_aws_creds_checking()}</span
								>
							</div>
						</div>
					{/if}
					{#snippet renderField(
						field: ConfigField,
						fieldPath: string,
						configObj: Record<string, ConfigValue>,
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
										{@const arrayValue =
											(configObj[field.name] as Record<string, ConfigValue>[] | undefined) || []}
										<div class="space-y-3">
											{#each arrayValue as item, index (index)}
												<div
													class="border border-gray-200 dark:border-gray-700 rounded-lg p-4 bg-gray-50/50 dark:bg-gray-800/50"
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
														{#each field.fields as nestedField (nestedField.name)}
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
																		oninput={() => {
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
													const newItem: Record<string, ConfigValue> = {};
													// Initialize with default values
													field.fields?.forEach((f) => {
														newItem[f.name] = (f.default as ConfigValue) || '';
													});
													arrayValue.push(newItem);
													configObj[field.name] = [...arrayValue];
													clearFieldError(fieldPath);
												}}
												class="w-full px-4 py-2.5 border-2 border-dashed border-gray-300 dark:border-gray-600 rounded-lg text-sm text-gray-600 dark:text-gray-400 hover:border-earthy-terracotta-600 hover:text-earthy-terracotta-600 dark:hover:text-earthy-terracotta-400 transition-colors flex items-center justify-center gap-2"
											>
												<IconifyIcon icon="material-symbols:add" class="h-5 w-5" />
												{m.pipelines_add_field_item({ label: field.label })}
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
										class="w-full flex items-center justify-between p-3 hover:bg-gray-50 dark:hover:bg-gray-800 transition-colors text-left"
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
											class="px-4 pb-4 border-t border-gray-200 dark:border-gray-700 bg-gray-50/50 dark:bg-gray-800/50"
										>
											<div class="grid grid-cols-1 md:grid-cols-2 gap-4 pt-4">
												{#each field.fields as nestedField (nestedField.name)}
													{@const nestedPath = `${fieldPath}.${nestedField.name}`}
													{@const nestedConfigObj =
														(configObj[field.name] as Record<string, ConfigValue> | undefined) ||
														{}}
													{#if !shouldHideField(nestedField, nestedConfigObj, config)}
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
									class="flex items-start p-3 border rounded-lg hover:bg-gray-50 dark:hover:bg-gray-800 cursor-pointer transition-colors {fieldErrors[
										fieldPath
									]
										? 'border-red-500 dark:border-red-500'
										: 'border-gray-200 dark:border-gray-700'}"
									data-field-path={fieldPath}
								>
									<input
										type="checkbox"
										checked={configObj[field.name] === true}
										onchange={(e) => {
											configObj[field.name] = (e.currentTarget as HTMLInputElement).checked;
											clearFieldError(fieldPath);
										}}
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
										{@const arrayValue = (configObj[field.name] as string[] | undefined) || []}
										<div class="space-y-2">
											{#each arrayValue as item, index (index)}
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
														aria-label={m.pipelines_remove_item_aria()}
													>
														<IconifyIcon icon="material-symbols:close" class="h-5 w-5" />
													</button>
												</div>
											{/each}
											<div class="flex items-center gap-2">
												<input
													type="text"
													placeholder={m.pipelines_typeahead_placeholder()}
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
												{m.pipelines_press_enter_hint()}
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
											<option value="">{m.pipelines_select_placeholder()}</option>
											{#each field.options as option (option.value)}
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
												autoDetectSourceType();
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
						{#each configSpec.filter((f) => !['tags', 'external_links', 'filter'].includes(f.name) && !f.hidden) as field (field.name)}
							{#if !shouldHideField(field, config)}
								{@render renderField(field, field.name, config, 0)}
							{/if}
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
						{m.pipelines_no_config_heading()}
					</h3>
					<p class="text-sm text-gray-600 dark:text-gray-400 mb-6">
						{m.pipelines_no_config_hint()}
					</p>
				</div>
			{/if}
		{/if}

		<!-- Step 4: Schedule Configuration -->
		{#if currentStep === totalSteps}
			<div
				class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-6"
			>
				<h3 class="text-base font-semibold text-gray-900 dark:text-gray-100 mb-4 flex items-center">
					<IconifyIcon
						icon="material-symbols:schedule"
						class="h-5 w-5 mr-2 text-earthy-terracotta-600"
					/>
					{m.pipelines_schedule_heading()}
				</h3>
				<div class="space-y-5">
					<div>
						<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
							{m.pipelines_cron_label()}
							<span class="text-xs font-normal text-gray-500 ml-1"
								>{m.pipelines_cron_optional()}</span
							>
						</label>
						<input
							type="text"
							bind:value={cronExpression}
							placeholder="0 2 * * *"
							onkeydown={(e) => {
								if (e.key === 'Enter' && !saving && name && selectedPluginId) {
									e.preventDefault();
									handleSave();
								}
							}}
							class="w-full px-4 py-2.5 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-earthy-terracotta-600 focus:border-transparent font-mono text-sm transition-all"
						/>
						<div class="mt-2 flex items-start">
							<IconifyIcon
								icon="material-symbols:info-outline"
								class="h-4 w-4 text-gray-400 mt-0.5 flex-shrink-0"
							/>
							<p class="ml-2 text-xs text-gray-500 dark:text-gray-400">
								{m.pipelines_cron_hint()}
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
													{m.pipelines_cron_next_runs_heading()}
												</p>
												<ul class="text-xs text-green-700 dark:text-green-400 space-y-0.5">
													{#each cronNextRuns as run, i (i)}
														<li class="font-mono">
															{run.toLocaleString(getLocale(), {
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
									<p class="ml-2 text-sm text-red-800 dark:text-red-300">
										{m.pipelines_cron_invalid()}
									</p>
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
										{m.pipelines_disable_schedule_label()}
									</span>
									<p class="text-xs text-gray-600 dark:text-gray-400 mt-1">
										{m.pipelines_disable_schedule_hint()}
									</p>
								</div>
							</label>
						</div>
					{/if}

					<!-- Filter -->
					<div class="border-t border-gray-200 dark:border-gray-700 pt-5">
						<button
							type="button"
							onclick={() => toggleSection('asset_filter')}
							class="w-full flex items-center justify-between px-4 py-3 rounded-lg border border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-800 hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors group {isExpanded(
								'asset_filter'
							)
								? 'rounded-b-none border-b-0'
								: ''}"
						>
							<div class="flex items-center gap-2">
								<IconifyIcon
									icon="material-symbols:filter-alt"
									class="h-4 w-4 text-gray-500 dark:text-gray-400"
								/>
								<h4 class="text-sm font-medium text-gray-700 dark:text-gray-300">
									{m.pipelines_asset_filter_heading()}
								</h4>
								<span class="text-xs text-gray-400 dark:text-gray-500">
									{m.pipelines_filter_optional_suffix()}
								</span>
							</div>
							<IconifyIcon
								icon={isExpanded('asset_filter')
									? 'material-symbols:expand-less'
									: 'material-symbols:expand-more'}
								class="h-5 w-5 text-gray-400 dark:text-gray-500 group-hover:text-gray-600 dark:group-hover:text-gray-300 transition-colors"
							/>
						</button>

						{#if isExpanded('asset_filter')}
							<div
								class="px-4 py-4 rounded-b-lg border border-t-0 border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800"
							>
								<p class="text-xs text-gray-500 dark:text-gray-400 mb-4">
									{m.pipelines_filter_description()}
								</p>

								<div class="grid grid-cols-1 md:grid-cols-2 gap-4">
									<!-- Include patterns -->
									<div
										class="rounded-lg border border-green-200 dark:border-green-800/50 bg-green-50/50 dark:bg-green-900/10 p-4"
									>
										<div class="flex items-center gap-2 mb-3">
											<IconifyIcon
												icon="material-symbols:check-circle-outline"
												class="h-4 w-4 text-green-600 dark:text-green-400"
											/>
											<span class="text-sm font-medium text-gray-800 dark:text-gray-200">
												{m.pipelines_filter_include_label()}
											</span>
										</div>
										<p class="text-xs text-gray-500 dark:text-gray-400 mb-3">
											{m.pipelines_filter_include_hint()}
										</p>
										{#if true}
											{@const includeValue = config.filter?.include || []}
											<div class="space-y-2">
												{#each includeValue as item, index (index)}
													<div class="flex items-center gap-1.5">
														<span
															class="text-gray-400 dark:text-gray-500 font-mono text-sm select-none"
															>/</span
														>
														<input
															type="text"
															value={item}
															oninput={(e) => {
																const target = e.target as HTMLInputElement;
																includeValue[index] = target.value;
																config.filter = { ...config.filter, include: [...includeValue] };
															}}
															class="flex-1 px-3 py-2 border border-gray-300 dark:border-gray-600 rounded bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-green-500 focus:border-transparent transition-all font-mono text-sm"
														/>
														<span
															class="text-gray-400 dark:text-gray-500 font-mono text-sm select-none"
															>/</span
														>
														<button
															type="button"
															onclick={(e) => {
																e.preventDefault();
																includeValue.splice(index, 1);
																config.filter = { ...config.filter, include: [...includeValue] };
															}}
															class="p-1.5 text-gray-400 hover:text-red-600 dark:hover:text-red-400 hover:bg-red-50 dark:hover:bg-red-900/20 rounded transition-colors"
															aria-label={m.pipelines_remove_pattern_aria()}
														>
															<IconifyIcon icon="material-symbols:close" class="h-4 w-4" />
														</button>
													</div>
												{/each}
												<div class="flex items-center gap-1.5">
													<span
														class="text-gray-400 dark:text-gray-500 font-mono text-sm select-none"
														>/</span
													>
													<input
														type="text"
														placeholder={includePatternExample}
														onkeydown={(e) => {
															if (e.key === 'Enter') {
																e.preventDefault();
																const target = e.target as HTMLInputElement;
																const value = target.value.trim();
																if (value) {
																	if (!config.filter) config.filter = {};
																	config.filter = {
																		...config.filter,
																		include: [...(config.filter.include || []), value]
																	};
																	target.value = '';
																}
															}
														}}
														class="flex-1 px-3 py-2 border-2 border-dashed border-gray-300 dark:border-gray-600 rounded bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-green-500 focus:border-green-500 transition-all font-mono text-sm"
													/>
													<span
														class="text-gray-400 dark:text-gray-500 font-mono text-sm select-none"
														>/</span
													>
													<div class="w-[30px]"></div>
												</div>
												<p class="text-[11px] text-gray-400 dark:text-gray-500 pl-3">
													{m.pipelines_press_enter_short_hint()}
												</p>
											</div>
										{/if}
									</div>

									<!-- Exclude patterns -->
									<div
										class="rounded-lg border border-amber-200 dark:border-amber-800/50 bg-amber-50/50 dark:bg-amber-900/10 p-4"
									>
										<div class="flex items-center gap-2 mb-3">
											<IconifyIcon
												icon="material-symbols:block"
												class="h-4 w-4 text-amber-600 dark:text-amber-400"
											/>
											<span class="text-sm font-medium text-gray-800 dark:text-gray-200">
												{m.pipelines_filter_exclude_label()}
											</span>
										</div>
										<p class="text-xs text-gray-500 dark:text-gray-400 mb-3">
											{m.pipelines_filter_exclude_hint()}
										</p>
										{#if true}
											{@const excludeValue = config.filter?.exclude || []}
											<div class="space-y-2">
												{#each excludeValue as item, index (index)}
													<div class="flex items-center gap-1.5">
														<span
															class="text-gray-400 dark:text-gray-500 font-mono text-sm select-none"
															>/</span
														>
														<input
															type="text"
															value={item}
															oninput={(e) => {
																const target = e.target as HTMLInputElement;
																excludeValue[index] = target.value;
																config.filter = { ...config.filter, exclude: [...excludeValue] };
															}}
															class="flex-1 px-3 py-2 border border-gray-300 dark:border-gray-600 rounded bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-amber-500 focus:border-transparent transition-all font-mono text-sm"
														/>
														<span
															class="text-gray-400 dark:text-gray-500 font-mono text-sm select-none"
															>/</span
														>
														<button
															type="button"
															onclick={(e) => {
																e.preventDefault();
																excludeValue.splice(index, 1);
																config.filter = { ...config.filter, exclude: [...excludeValue] };
															}}
															class="p-1.5 text-gray-400 hover:text-red-600 dark:hover:text-red-400 hover:bg-red-50 dark:hover:bg-red-900/20 rounded transition-colors"
															aria-label={m.pipelines_remove_pattern_aria()}
														>
															<IconifyIcon icon="material-symbols:close" class="h-4 w-4" />
														</button>
													</div>
												{/each}
												<div class="flex items-center gap-1.5">
													<span
														class="text-gray-400 dark:text-gray-500 font-mono text-sm select-none"
														>/</span
													>
													<input
														type="text"
														placeholder={excludePatternExample}
														onkeydown={(e) => {
															if (e.key === 'Enter') {
																e.preventDefault();
																const target = e.target as HTMLInputElement;
																const value = target.value.trim();
																if (value) {
																	if (!config.filter) config.filter = {};
																	config.filter = {
																		...config.filter,
																		exclude: [...(config.filter.exclude || []), value]
																	};
																	target.value = '';
																}
															}
														}}
														class="flex-1 px-3 py-2 border-2 border-dashed border-gray-300 dark:border-gray-600 rounded bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-amber-500 focus:border-amber-500 transition-all font-mono text-sm"
													/>
													<span
														class="text-gray-400 dark:text-gray-500 font-mono text-sm select-none"
														>/</span
													>
													<div class="w-[30px]"></div>
												</div>
												<p class="text-[11px] text-gray-400 dark:text-gray-500 pl-3">
													{m.pipelines_press_enter_short_hint()}
												</p>
											</div>
										{/if}
									</div>
								</div>
							</div>
						{/if}
					</div>
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
						text={m.common_previous()}
					/>
				{:else}
					<Button
						variant="clear"
						click={() => goto(resolve('/runs?tab=pipelines'))}
						text={m.common_cancel()}
					/>
				{/if}
			</div>
			<div class="flex items-center gap-3">
				{#if currentStep < totalSteps}
					<Button
						variant="filled"
						click={handleNextStep}
						text={currentStep === 3 && validating ? m.pipelines_validating() : m.common_next()}
						icon="material-symbols:arrow-forward"
						disabled={validating ||
							(currentStep === 1 && !canProceedToStep2) ||
							(currentStep === 2 && !canProceedToStep3)}
					/>
				{:else}
					<div class="text-sm text-gray-500 dark:text-gray-400 mr-4">
						{#if hasSchedule}
							<IconifyIcon icon="material-symbols:schedule" class="inline h-4 w-4 mr-1" />
							{m.pipelines_scheduled_pipeline()}
						{:else}
							<IconifyIcon
								icon="material-symbols:play-circle-outline"
								class="inline h-4 w-4 mr-1"
							/>
							{m.pipelines_manual_only_pipeline()}
						{/if}
					</div>
					<Button
						variant="filled"
						click={handleSave}
						text={saving ? m.pipelines_creating() : m.pipelines_create_pipeline()}
						disabled={saving || !name || !selectedPluginId}
						icon="material-symbols:check"
					/>
				{/if}
			</div>
		</div>
	</div>
</div>
