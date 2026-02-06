<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import { fetchApi } from '$lib/api';
	import Button from '$components/ui/Button.svelte';
	import IconifyIcon from '@iconify/svelte';
	import Icon from '$components/ui/Icon.svelte';
	import cronstrue from 'cronstrue';
	import { Cron } from 'croner';

	const pipelineId = $page.params.id;

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
	let loadingPipeline = $state(true);
	let selectedPluginId = $state('');
	let name = $state('');
	let cronExpression = $state('');
	let disableSchedule = $state(false);
	let config = $state<Record<string, any>>({});
	let saving = $state(false);
	let error = $state<string | null>(null);
	let pluginSearchQuery = $state('');
	let expandedSections = $state<Record<string, boolean>>({});
	let awsCredentialStatus = $state<{
		available: boolean;
		sources: string[];
		error?: string;
	} | null>(null);
	let loadingAwsStatus = $state(false);

	let selectedPlugin = $derived(plugins.find((p) => p.id === selectedPluginId) || null);

	let configSpec = $derived(selectedPlugin?.config_spec || null);
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

	let isAWSPlugin = $derived(
		selectedPluginId && ['s3', 'sns', 'sqs', 'dynamodb', 'kinesis'].includes(selectedPluginId)
	);

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

	let displayedPlugins = $derived(
		pluginSearchQuery ? filteredPlugins : filteredPlugins.slice(0, 5)
	);

	let hasMorePlugins = $derived(!pluginSearchQuery && filteredPlugins.length > 5);

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

	async function fetchAWSCredentialStatus() {
		try {
			loadingAwsStatus = true;
			const response = await fetchApi('/plugins/aws/credentials/status');
			if (response.ok) {
				awsCredentialStatus = await response.json();
			}
		} catch (err) {
			console.error('Error fetching AWS credential status:', err);
		} finally {
			loadingAwsStatus = false;
		}
	}

	function fillMissingDefaults(fields: ConfigField[], configObj: Record<string, any>) {
		for (const field of fields) {
			if (field.type === 'object' && field.is_array) {
				if (!configObj[field.name] || !Array.isArray(configObj[field.name])) {
					configObj[field.name] = [];
				}
			} else if (field.type === 'object' && field.fields) {
				if (!configObj[field.name]) {
					configObj[field.name] = {};
				}
				fillMissingDefaults(field.fields, configObj[field.name]);
			} else if (field.type === 'multiselect') {
				if (!configObj[field.name] || !Array.isArray(configObj[field.name])) {
					configObj[field.name] = [];
				}
			} else if (
				field.default !== undefined &&
				field.default !== null &&
				configObj[field.name] === undefined
			) {
				configObj[field.name] = field.default;
			}
		}
	}

	async function fetchPipeline() {
		try {
			loadingPipeline = true;
			const response = await fetchApi(`/ingestion/schedules/${pipelineId}`);
			if (!response.ok) throw new Error('Failed to fetch pipeline');
			const pipeline = await response.json();

			name = pipeline.name;
			selectedPluginId = pipeline.plugin_id;
			cronExpression = pipeline.cron_expression || '';
			disableSchedule = !pipeline.enabled && !!pipeline.cron_expression;
			config = pipeline.config || {};

			// Ensure base fields have proper defaults
			if (!config.tags) config.tags = [];
			if (!config.external_links) config.external_links = [];
			if (!config.filter) config.filter = { include: [], exclude: [] };
			if (!config.filter.include) config.filter.include = [];
			if (!config.filter.exclude) config.filter.exclude = [];

			// Auto-expand filter section if patterns exist
			if (config.filter.include.length > 0 || config.filter.exclude.length > 0) {
				expandedSections['asset_filter'] = true;
			}

			const plugin = plugins.find((p) => p.id === pipeline.plugin_id);
			if (plugin && plugin.config_spec) {
				fillMissingDefaults(plugin.config_spec, config);
			}
		} catch (err) {
			console.error('Error fetching pipeline:', err);
			error = 'Failed to load pipeline';
		} finally {
			loadingPipeline = false;
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

			const response = await fetchApi(`/ingestion/schedules/${pipelineId}`, {
				method: 'PUT',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify(body)
			});

			if (!response.ok) {
				const data = await response.json();
				throw new Error(data.error || 'Failed to update pipeline');
			}

			// Navigate back to runs page with pipelines tab
			goto('/runs?tab=pipelines');
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to update pipeline';
		} finally {
			saving = false;
		}
	}

	function handlePluginChange(pluginId: string) {
		selectedPluginId = pluginId;
		// Reset config when plugin changes
		config = {};
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

		// Default collapsed for asset filter
		if (sectionName === 'asset_filter') return false;

		// Default to expanded for all other sections
		return true;
	}

	function shouldHideField(field: ConfigField, configObj: Record<string, any>): boolean {
		// If there's a sibling "use_default" field that's checked, hide all other fields
		if (configObj.use_default === true && field.name !== 'use_default') {
			return true;
		}
		return false;
	}

	onMount(async () => {
		await fetchPlugins();
		await fetchPipeline();
		// Check if this is an AWS plugin and fetch credential status
		if (
			selectedPluginId &&
			['s3', 'sns', 'sqs', 'dynamodb', 'kinesis'].includes(selectedPluginId)
		) {
			fetchAWSCredentialStatus();
		}
	});
</script>

<div class="min-h-screen">
	<!-- Header -->
	<div class="border-b border-gray-200 dark:border-gray-700">
		<div class="container max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-6">
			<div class="flex items-center justify-between">
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
						<h1 class="text-2xl font-bold text-gray-900 dark:text-gray-100">Edit Pipeline</h1>
						<p class="text-sm text-gray-600 dark:text-gray-400 mt-1">
							Update your data ingestion pipeline configuration
						</p>
					</div>
				</div>
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

		<div class="space-y-8">
			<!-- Basic Information -->
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

				<!-- Tags -->
				<div class="mt-6">
					<span class="text-sm font-medium text-gray-700 dark:text-gray-300 mb-2 block">
						Tags
					</span>
					<p class="text-xs text-gray-500 dark:text-gray-400 mb-2">
						Tags to apply to discovered assets
					</p>
					{#if true}
						{@const tagsValue = config.tags || []}
						<div class="space-y-2">
							{#each tagsValue as item, index}
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
										aria-label="Remove tag"
									>
										<IconifyIcon icon="material-symbols:close" class="h-5 w-5" />
									</button>
								</div>
							{/each}
							<div class="flex items-center gap-2">
								<input
									type="text"
									placeholder="Type to add tags..."
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
							<p class="text-xs text-gray-500 dark:text-gray-400">Press Enter to add items</p>
						</div>
					{/if}
				</div>

				<!-- External Links -->
				<div class="mt-6">
					<span class="text-sm font-medium text-gray-700 dark:text-gray-300 mb-2 block">
						External Links
					</span>
					<p class="text-xs text-gray-500 dark:text-gray-400 mb-2">
						External links to show on all assets
					</p>
					{#if true}
						{@const linksValue = config.external_links || []}
						<div class="space-y-3">
							{#each linksValue as item, index}
								<div
									class="border border-gray-200 dark:border-gray-700 rounded-lg p-4 bg-gray-50/50 dark:bg-gray-750/50"
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
													Name
												</span>
												<input
													type="text"
													bind:value={item.name}
													oninput={() => {
														config.external_links = [...linksValue];
													}}
													placeholder="Link name"
													class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-earthy-terracotta-600 focus:border-transparent transition-all"
												/>
											</label>
										</div>
										<div>
											<label class="block">
												<span
													class="text-xs font-medium text-gray-700 dark:text-gray-300 mb-1 block"
												>
													URL
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
								Add External Link
							</button>
						</div>
					{/if}
				</div>
			</div>

			<!-- Plugin Selection (Read-only in edit mode) -->
			<div
				class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-6"
			>
				<h3 class="text-base font-semibold text-gray-900 dark:text-gray-100 mb-4 flex items-center">
					<IconifyIcon
						icon="material-symbols:extension"
						class="h-5 w-5 mr-2 text-earthy-terracotta-600"
					/>
					Data Source
				</h3>

				{#if loadingPlugins || loadingPipeline}
					<div class="flex items-center justify-center py-8">
						<div
							class="animate-spin rounded-full h-8 w-8 border-b-2 border-earthy-terracotta-700"
						></div>
						<span class="ml-3 text-sm text-gray-500">Loading...</span>
					</div>
				{:else if selectedPlugin}
					<div
						class="border-2 border-earthy-terracotta-500 bg-earthy-terracotta-50 dark:bg-earthy-terracotta-900/20 rounded-lg p-5"
					>
						<div class="flex items-center gap-4 mb-3">
							{#if selectedPlugin.icon}
								<div
									class="h-14 w-14 flex items-center justify-center bg-white dark:bg-gray-700 rounded-lg border border-gray-200 dark:border-gray-600 flex-shrink-0"
								>
									<Icon name={selectedPlugin.icon} size="md" showLabel={false} />
								</div>
							{/if}
							<div class="flex-1 min-w-0">
								<h4 class="text-base font-semibold text-gray-900 dark:text-gray-100">
									{selectedPlugin.name}
								</h4>
								{#if selectedPlugin.category}
									<span
										class="inline-block mt-1 px-2 py-0.5 text-xs font-medium rounded-full bg-white dark:bg-gray-700 text-gray-600 dark:text-gray-400"
									>
										{selectedPlugin.category}
									</span>
								{/if}
							</div>
						</div>
						{#if selectedPlugin.description}
							<p class="text-sm text-gray-700 dark:text-gray-300 mb-3">
								{selectedPlugin.description}
							</p>
						{/if}
						<div
							class="flex items-start gap-2 text-xs text-gray-600 dark:text-gray-400 bg-white dark:bg-gray-800/50 rounded px-3 py-2"
						>
							<IconifyIcon icon="material-symbols:lock" class="h-4 w-4 mt-0.5 flex-shrink-0" />
							<p>
								Plugin cannot be changed after creation. Create a new pipeline to use a different
								data source.
							</p>
						</div>
					</div>
				{/if}
			</div>

			<!-- Plugin Configuration -->
			{#if configSpec && configSpec.length > 0}
				<div
					class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-6"
				>
					<h3
						class="text-base font-semibold text-gray-900 dark:text-gray-100 mb-4 flex items-center"
					>
						<IconifyIcon
							icon="material-symbols:settings"
							class="h-5 w-5 mr-2 text-earthy-terracotta-600"
						/>
						Connection Configuration
					</h3>

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
															}}
															class="p-1 text-red-600 dark:text-red-400 hover:bg-red-50 dark:hover:bg-red-900/20 rounded transition-colors"
														>
															<IconifyIcon icon="material-symbols:close" class="h-4 w-4" />
														</button>
													</div>
													<div class="grid grid-cols-1 gap-3">
														{#each field.fields as nestedField}
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
																			// Validate URL fields
																			const input = e.target as HTMLInputElement;
																			if (
																				getFieldType(nestedField) === 'url' &&
																				input.value &&
																				!input.checkValidity()
																			) {
																				input.setCustomValidity(
																					`${nestedField.label} must be a valid URL`
																				);
																			} else {
																				input.setCustomValidity('');
																			}
																		}}
																		placeholder={nestedField.placeholder}
																		required={nestedField.required}
																		class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-earthy-terracotta-600 focus:border-transparent transition-all"
																	/>
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
												}}
												class="w-full px-4 py-2.5 border-2 border-dashed border-gray-300 dark:border-gray-600 rounded-lg text-sm text-gray-600 dark:text-gray-400 hover:border-earthy-terracotta-600 hover:text-earthy-terracotta-600 dark:hover:text-earthy-terracotta-400 transition-colors flex items-center justify-center gap-2"
											>
												<IconifyIcon icon="material-symbols:add" class="h-5 w-5" />
												Add {field.label}
											</button>
										</div>
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
									class="flex items-start p-3 border border-gray-200 dark:border-gray-700 rounded-lg hover:bg-gray-50 dark:hover:bg-gray-750 cursor-pointer transition-colors"
								>
									<input
										type="checkbox"
										bind:checked={configObj[field.name]}
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
									</div>
								</label>
							</div>
						{:else if field.type === 'multiselect'}
							<!-- Array/List field -->
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
														}}
														placeholder={field.placeholder}
														class="flex-1 px-4 py-2.5 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-earthy-terracotta-600 focus:border-transparent transition-all"
													/>
													<button
														type="button"
														onclick={(e) => {
															e.preventDefault();
															e.stopPropagation();
															arrayValue.splice(index, 1);
															configObj[field.name] = [...arrayValue];
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
											}}
											class="w-full px-4 py-2.5 border rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-earthy-terracotta-600 focus:border-transparent transition-all border-gray-300 dark:border-gray-600"
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
											}}
											placeholder={field.placeholder ||
												(field.default ? String(field.default) : '')}
											class="w-full px-4 py-2.5 border rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-earthy-terracotta-600 focus:border-transparent transition-all {field.type ===
											'password'
												? 'font-mono'
												: ''} border-gray-300 dark:border-gray-600"
											required={field.required}
										/>
									{/if}
								</label>
							</div>
						{/if}
					{/snippet}

					<div class="grid grid-cols-1 md:grid-cols-2 gap-4">
						{#each configSpec.filter((f) => !['tags', 'external_links', 'filter'].includes(f.name)) as field}
							{@render renderField(field, field.name, config, 0)}
						{/each}
					</div>
				</div>
			{/if}

			<!-- Schedule & Filtering -->
			<div
				class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-6"
			>
				<h3 class="text-base font-semibold text-gray-900 dark:text-gray-100 mb-4 flex items-center">
					<IconifyIcon
						icon="material-symbols:schedule"
						class="h-5 w-5 mr-2 text-earthy-terracotta-600"
					/>
					Schedule & Filtering
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

					<!-- Filter -->
					<div class="border-t border-gray-200 dark:border-gray-700 pt-5">
						<button
							type="button"
							onclick={() => toggleSection('asset_filter')}
							class="w-full flex items-center justify-between px-4 py-3 rounded-lg border border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-750 hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors group {isExpanded(
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
								<h4 class="text-sm font-medium text-gray-700 dark:text-gray-300">Asset Filter</h4>
								<span class="text-xs text-gray-400 dark:text-gray-500"> — optional </span>
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
									Filter which assets get ingested by name. Supports plain text or <span
										class="font-mono bg-gray-100 dark:bg-gray-700 px-1 py-0.5 rounded text-[11px]"
										>regex</span
									> patterns.
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
												Include
											</span>
										</div>
										<p class="text-xs text-gray-500 dark:text-gray-400 mb-3">
											Only matching assets will be ingested
										</p>
										{#if true}
											{@const includeValue = config.filter?.include || []}
											<div class="space-y-2">
												{#each includeValue as item, index}
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
															aria-label="Remove pattern"
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
														placeholder="^public_.*"
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
													Press Enter to add
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
												Exclude
											</span>
										</div>
										<p class="text-xs text-gray-500 dark:text-gray-400 mb-3">
											Matching assets will be skipped
										</p>
										{#if true}
											{@const excludeValue = config.filter?.exclude || []}
											<div class="space-y-2">
												{#each excludeValue as item, index}
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
															aria-label="Remove pattern"
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
														placeholder="^_tmp_.*"
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
													Press Enter to add
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
		</div>

		<!-- Footer Actions -->
		<div class="mt-8 flex items-center justify-between">
			<div class="text-sm text-gray-500 dark:text-gray-400">
				{#if hasSchedule}
					<IconifyIcon icon="material-symbols:schedule" class="inline h-4 w-4 mr-1" />
					Scheduled pipeline
				{:else}
					<IconifyIcon icon="material-symbols:play-circle-outline" class="inline h-4 w-4 mr-1" />
					Manual-only pipeline
				{/if}
			</div>
			<div class="flex items-center gap-3">
				<Button
					variant="clear"
					click={() => goto('/runs?tab=pipelines')}
					text="Cancel"
					disabled={saving}
				/>
				<Button
					variant="filled"
					click={handleSave}
					text={saving ? 'Saving...' : 'Save Changes'}
					disabled={saving || !name || !selectedPluginId}
				/>
			</div>
		</div>
	</div>
</div>
