<script lang="ts">
	import '@marmotdata/elements/stepper.js';
	import '@marmotdata/elements/filter-pill.js';
	import { onDestroy, onMount, tick } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { resolve } from '$app/paths';
	import { get } from 'svelte/store';
	import { Cron } from 'croner';
	import { fetchApi } from '$lib/api';
	import { auth } from '$lib/stores/auth';
	import { encryptionConfigured } from '$lib/stores/encryption';
	import { toasts } from '$lib/stores/toast';
	import { setCrumbs } from '$lib/mm';
	import { formatNumber } from '$lib/utils/format';
	import { m } from '$lib/paraglide/messages';
	import { getLocale } from '$lib/paraglide/runtime';
	import type { Owner } from '$lib/dataproducts/types';
	import QueryAccessSection from '$components/dataquery/QueryAccessSection.svelte';
	import ConfigFields from '$components/runs/ConfigFields.svelte';
	import ConnectionTestCallout from '$components/runs/ConnectionTestCallout.svelte';
	import PipelineDetailsFields from '$components/runs/PipelineDetailsFields.svelte';
	import PluginSummary from '$components/runs/PluginSummary.svelte';
	import ScheduleFields from '$components/runs/ScheduleFields.svelte';
	import {
		AWS_PLUGINS,
		cleanConfigForSubmit,
		detectSourceType,
		formatList,
		initializeConfigDefaults,
		testConnection,
		type ConfigObject,
		type ConnectionTestResult,
		type Pipeline,
		type PluginInfo
	} from '$components/runs/pipeline';

	type Step = 'plugin' | 'basics' | 'configure' | 'schedule';
	const STEPS: Step[] = ['plugin', 'basics', 'configure', 'schedule'];

	interface NewConfig extends ConfigObject {
		tags?: string[];
		external_links?: { name: string; url: string }[];
		filter?: { include?: string[]; exclude?: string[] };
	}

	setCrumbs([
		{ label: m.runs_heading(), href: resolve('/runs') },
		{ label: m.pipelines_new_pipeline() }
	]);

	// Creating a pipeline is the stepper's four steps; the stepper names each step and shows what the finished ones chose
	let step = $state<Step>('plugin');
	let stepContent = $state<HTMLDivElement>();
	let shownStep: Step | null = null;

	// A new step replaces the controls that had focus, so focus would fall to the page; move it to the step's first field instead, the way a new page would start
	$effect(() => {
		const current = step;
		if (shownStep !== null && shownStep !== current) {
			tick().then(() => {
				const first = stepContent?.querySelector<HTMLElement>(
					'mm-search-field, mm-text-field, mm-textarea, mm-select, mm-combobox, mm-multiselect, mm-switch, mm-cron-field, mm-list, mm-checkbox, input, select, textarea, mm-button, button'
				);
				first?.focus();
			});
		}
		shownStep = current;
	});
	let plugins = $state<PluginInfo[]>([]);
	let existing = $state<Pipeline[]>([]);
	let loadingPlugins = $state(true);
	let pluginsStillLoading = $state(false);
	let pluginPollTimer: ReturnType<typeof setTimeout> | null = null;
	let pluginSearch = $state('');
	let queryOnly = $state(false);

	let selectedPluginId = $state('');
	let name = $state('');
	let nameError = $state('');
	let description = $state('');
	let owner = $state<Owner | null>(null);
	let config = $state<NewConfig>({
		tags: [],
		external_links: [],
		filter: { include: [], exclude: [] }
	});
	let queryEnabled = $state(false);
	let ingestOnQuery = $state(false);
	let scheduled = $state(false);
	let cronExpression = $state('');
	let cronError = $state('');

	let fieldErrors = $state<Record<string, string>>({});
	let configValidated = $state(false);
	let validating = $state(false);
	let saving = $state(false);
	let error = $state('');
	let awsStatus = $state<{ available: boolean; sources: string[]; error?: string } | null>(null);
	// The last connection test, cleared whenever the settings change so it never describes other settings
	let connectionTest = $state<ConnectionTestResult | null>(null);
	let testError = $state('');
	let testing = $state(false);

	let index = $derived(STEPS.indexOf(step));
	let selectedPlugin = $derived(plugins.find((p) => p.id === selectedPluginId) ?? null);
	let queryCapable = $derived(selectedPlugin?.supports_query === true);
	let queryCount = $derived(plugins.filter((p) => p.supports_query).length);
	let connectedHere = $derived(
		existing.filter((p) => p.plugin_id === selectedPluginId).map((p) => p.name)
	);
	let shownPlugins = $derived.by(() => {
		const q = pluginSearch.trim().toLowerCase();
		return plugins
			.filter((p) => !queryOnly || p.supports_query)
			.filter(
				(p) =>
					!q ||
					p.name.toLowerCase().includes(q) ||
					p.id.toLowerCase().includes(q) ||
					p.description?.toLowerCase().includes(q)
			)
			.sort((a, b) => a.name.localeCompare(b.name));
	});

	let stepLabels = $derived<Record<Step, string>>({
		plugin: m.pipelines_step_choose_plugin(),
		basics: m.pipelines_step_basic_info(),
		configure: m.pipelines_step_configure(),
		schedule: m.pipelines_step_schedule_filter()
	});

	// What the person will see once it exists, so the side column only says what is still to come
	let firstRun = $derived.by(() => {
		if (queryCapable && queryEnabled) return m.pipelines_first_run_now();
		if (!scheduled) return m.pipelines_first_run_manual();
		try {
			const next = new Cron(cronExpression).nextRun();
			return next
				? next.toLocaleString(getLocale(), {
						weekday: 'short',
						day: 'numeric',
						month: 'short',
						hour: '2-digit',
						minute: '2-digit'
					})
				: m.pipelines_first_run_manual();
		} catch {
			return m.pipelines_first_run_manual();
		}
	});

	async function fetchPlugins() {
		try {
			loadingPlugins = true;
			const res = await fetchApi('/plugins');
			if (!res.ok) throw new Error(m.pipelines_error_load_plugins());
			const data = await res.json();
			plugins = Array.isArray(data?.plugins) ? data.plugins : [];
			pluginsStillLoading = Boolean(data?.loading);
		} catch {
			error = m.pipelines_error_load_plugins();
		} finally {
			loadingPlugins = false;
			if (pluginPollTimer) clearTimeout(pluginPollTimer);
			pluginPollTimer = pluginsStillLoading ? setTimeout(fetchPlugins, 2000) : null;
		}
	}

	async function fetchExisting() {
		try {
			const res = await fetchApi('/ingestion/schedules?limit=200');
			if (res.ok) existing = (await res.json()).schedules ?? [];
		} catch {
			// Only the side column's "Already connected" line uses this
		}
	}

	async function fetchAwsStatus() {
		try {
			const res = await fetchApi('/plugins/aws/credentials/status');
			if (!res.ok) return;
			awsStatus = await res.json();
			// Credentials the server already has are the simplest choice, so they start selected
			if (awsStatus?.available) {
				config = {
					...config,
					credentials: { ...((config.credentials as ConfigObject) ?? {}), use_default: true }
				};
			}
		} catch {
			// Without the status the fields simply show
		}
	}

	function choosePlugin(pluginId: string) {
		if (pluginId === selectedPluginId) return;
		selectedPluginId = pluginId;
		const plugin = plugins.find((p) => p.id === pluginId);
		// The plugin's own settings start from its defaults; tags, links and filters carry over
		config = {
			...(plugin?.config_spec ? initializeConfigDefaults(plugin.config_spec) : {}),
			tags: config.tags ?? [],
			external_links: config.external_links ?? [],
			filter: config.filter ?? { include: [], exclude: [] }
		};
		fieldErrors = {};
		configValidated = false;
		connectionTest = null;
		testError = '';
		awsStatus = null;
		error = '';

		// Querying is on by default for plugins that support it, for people allowed to turn it on
		const canQuery = plugin?.supports_query === true && auth.hasPermission('access', 'manage');
		queryEnabled = canQuery;
		ingestOnQuery = canQuery;

		if (AWS_PLUGINS.includes(pluginId)) fetchAwsStatus();
	}

	function onConfigEdit(path: string) {
		if (fieldErrors[path]) {
			const next = { ...fieldErrors };
			delete next[path];
			fieldErrors = next;
		}
		configValidated = false;
		connectionTest = null;
		testError = '';
		detectSourceType(selectedPlugin?.config_spec, config);
	}

	async function runConnectionTest() {
		if (!selectedPlugin) return;
		testing = true;
		connectionTest = null;
		testError = '';
		error = '';
		try {
			const res = await testConnection(
				selectedPluginId,
				cleanConfigForSubmit(config, selectedPlugin.config_spec ?? [])
			);
			if (!res.valid && res.errors?.length) {
				const errors: Record<string, string> = {};
				for (const e of res.errors) errors[e.field] = e.message;
				fieldErrors = errors;
				testError = m.pipelines_test_invalid();
				focusField(res.errors[0].field);
				return;
			}
			connectionTest = res.result ?? null;
		} catch (e) {
			testError = e instanceof Error ? e.message : m.pipelines_test_error();
		} finally {
			testing = false;
		}
	}

	async function focusField(path: string) {
		await tick();
		const el = document.querySelector<HTMLElement>(`[data-field-path="${CSS.escape(path)}"]`);
		el?.scrollIntoView({ behavior: 'smooth', block: 'center' });
		el?.focus();
	}

	async function validateConfig(): Promise<boolean> {
		if (!selectedPlugin) return false;
		validating = true;
		fieldErrors = {};
		error = '';
		try {
			const res = await fetchApi('/ingestion/validate', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({
					plugin_id: selectedPluginId,
					config: cleanConfigForSubmit(config, selectedPlugin.config_spec ?? [])
				})
			});
			if (!res.ok) throw new Error(m.pipelines_error_validate());
			const result = await res.json();
			if (!result.valid && result.errors?.length > 0) {
				const errors: Record<string, string> = {};
				for (const e of result.errors) errors[e.field] = e.message;
				fieldErrors = errors;
				error = m.pipelines_error_check_fields();
				configValidated = false;
				focusField(result.errors[0].field);
				return false;
			}
			configValidated = true;
			return true;
		} catch (e) {
			error = e instanceof Error ? e.message : m.pipelines_error_validate();
			configValidated = false;
			return false;
		} finally {
			validating = false;
		}
	}

	async function next() {
		error = '';
		if (step === 'plugin') {
			if (!selectedPluginId) {
				error = m.pipelines_error_select_plugin();
				return;
			}
			step = 'basics';
		} else if (step === 'basics') {
			if (!name.trim()) {
				nameError = m.pipelines_error_name_required();
				focusField('name');
				return;
			}
			nameError = '';
			step = 'configure';
		} else if (step === 'configure') {
			if (await validateConfig()) step = 'schedule';
		} else {
			await create();
		}
		window.scrollTo({ top: 0 });
	}

	function back() {
		error = '';
		if (index > 0) step = STEPS[index - 1];
	}

	async function create() {
		if (scheduled) {
			try {
				new Cron(cronExpression);
				cronError = '';
			} catch {
				cronError = m.pipelines_cron_invalid();
				return;
			}
		}
		saving = true;
		try {
			const res = await fetchApi('/ingestion/schedules', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({
					name: name.trim(),
					description: description.trim(),
					owner_type: owner?.type ?? '',
					owner_id: owner?.id ?? '',
					plugin_id: selectedPluginId,
					config: selectedPlugin?.config_spec
						? cleanConfigForSubmit(config, selectedPlugin.config_spec)
						: config,
					cron_expression: scheduled ? cronExpression.trim() : '',
					enabled: true,
					queryable: queryCapable && queryEnabled,
					ingest_on_query: queryCapable && queryEnabled && ingestOnQuery
				})
			});
			if (!res.ok) {
				const data = await res.json().catch(() => ({}));
				throw new Error(data.error || m.pipelines_error_create());
			}
			const created: { id?: string } = await res.json().catch(() => ({}));

			// A source that answers queries is only useful once its tables are cataloged, so its first run starts straight away
			if (created.id && queryCapable && queryEnabled) {
				const trigger = await fetchApi(
					`/ingestion/schedules/${encodeURIComponent(created.id)}/trigger`,
					{ method: 'POST' }
				).catch(() => null);
				if (trigger?.ok) toasts.success(m.pipelines_first_run_started({ name: name.trim() }));
			}

			goto(created.id ? resolve('/runs/[id]', { id: created.id }) : resolve('/runs'));
		} catch (e) {
			error = e instanceof Error ? e.message : m.pipelines_error_create();
		} finally {
			saving = false;
		}
	}

	onMount(() => {
		if (!get(encryptionConfigured)) {
			toasts.error(m.pipelines_error_encryption_not_configured());
			goto(resolve('/runs'));
			return;
		}
		// Links such as the gallery and the Query page's empty state can preselect a plugin or the querying filter
		const params = page.url.searchParams;
		if (params.get('query') === '1') queryOnly = true;
		fetchExisting();
		fetchPlugins().then(() => {
			const preset = params.get('plugin');
			if (preset && plugins.some((p) => p.id === preset)) {
				choosePlugin(preset);
				step = 'basics';
			}
		});
	});

	onDestroy(() => {
		if (pluginPollTimer) clearTimeout(pluginPollTimer);
	});
</script>

<svelte:head>
	<title>{m.pipelines_new_pipeline()}</title>
</svelte:head>

<div class="mm-stack" style="max-inline-size: 72rem">
	<mm-page-header heading={m.pipelines_new_pipeline()}></mm-page-header>

	<mm-stepper
		label={m.pipelines_new_pipeline()}
		value={step}
		navigable
		onmm-step-change={(e: CustomEvent<{ value: string }>) => {
			error = '';
			step = e.detail.value as Step;
		}}
	>
		<mm-step
			value="plugin"
			label={stepLabels.plugin}
			description={index > 0 ? (selectedPlugin?.name ?? '') : ''}
		></mm-step>
		<mm-step value="basics" label={stepLabels.basics} description={index > 1 ? name : ''}></mm-step>
		<mm-step
			value="configure"
			label={stepLabels.configure}
			description={index > 2 && configValidated
				? connectionTest?.succeeded
					? m.pipelines_step_connection_tested()
					: m.pipelines_step_settings_checked()
				: ''}
			error={step === 'configure' &&
				(Object.keys(fieldErrors).length > 0 ||
					(connectionTest !== null && !connectionTest.succeeded && !connectionTest.timed_out))}
		></mm-step>
		<mm-step value="schedule" label={stepLabels.schedule}></mm-step>
	</mm-stepper>

	<form
		class="mm-split"
		style="gap: var(--space-12)"
		aria-label={m.pipelines_form_label({ step: stepLabels[step] })}
		novalidate
		onsubmit={(e) => {
			e.preventDefault();
			next();
		}}
	>
		<div class="mm-stack" bind:this={stepContent}>
			{#if error}
				<mm-callout tone="danger" compact heading={error}></mm-callout>
			{/if}

			{#if step === 'plugin'}
				<mm-section description={m.pipelines_choose_plugin_hint()}>
					<div class="mm-stack" style="--mm-gap: var(--space-4)">
						{#if pluginsStillLoading}
							<mm-callout tone="info" compact heading={m.pipelines_plugins_still_loading_list()}
							></mm-callout>
						{/if}
						<div class="mm-cluster" style="--mm-gap: var(--space-3)">
							<mm-search-field
								label={m.pipelines_search_plugins_placeholder()}
								hide-label
								placeholder={m.pipelines_search_plugins_placeholder()}
								value={pluginSearch}
								style="flex: 1; min-inline-size: 12rem"
								oninput={(e: Event) => (pluginSearch = (e.currentTarget as HTMLInputElement).value)}
								onmm-search={(e: CustomEvent<{ value: string }>) => {
									e.preventDefault();
									if (shownPlugins.length === 1) choosePlugin(shownPlugins[0].id);
								}}
							></mm-search-field>
							{#if queryCount > 0}
								<mm-filter-pill
									pressed={queryOnly}
									count={queryCount}
									onmm-change={(e: CustomEvent<{ pressed: boolean }>) =>
										(queryOnly = e.detail.pressed)}
									><mm-icon slot="start" name="terminal"
									></mm-icon>{m.plugins_filter_queryable()}</mm-filter-pill
								>
							{/if}
						</div>
						{#if loadingPlugins && plugins.length === 0}
							<mm-skeleton count={6}></mm-skeleton>
						{:else if shownPlugins.length === 0}
							<mm-empty-state
								compact
								heading={m.plugins_gallery_no_match_heading()}
								description={m.plugins_gallery_no_match_body()}
							></mm-empty-state>
						{:else}
							<mm-list aria-label={m.pipelines_available_plugins_aria()}>
								{#each shownPlugins as p (p.id)}
									<mm-list-item
										interactive
										selected={p.id === selectedPluginId}
										secondary={p.description}
										onmm-activate={() => choosePlugin(p.id)}
									>
										<mm-provider-logo slot="start" provider={p.icon || p.id} size="20" decorative
										></mm-provider-logo>
										{p.name}
										{#if p.supports_query}
											<span slot="end" class="mm-cluster" style="--mm-gap: var(--space-1)">
												<mm-icon name="terminal" aria-hidden="true"></mm-icon>
												<mm-text variant="caption">{m.plugins_filter_queryable()}</mm-text>
											</span>
										{/if}
									</mm-list-item>
								{/each}
							</mm-list>
						{/if}
					</div>
				</mm-section>
			{:else if step === 'basics'}
				<mm-section>
					<div class="mm-stack" style="--mm-gap: var(--space-5)">
						<mm-text-field
							label={m.pipelines_name_label()}
							name="name"
							mono
							required
							maxlength={63}
							autocomplete="off"
							placeholder={m.pipelines_name_placeholder()}
							help={m.pipelines_name_help()}
							error={nameError}
							value={name}
							data-field-path="name"
							onmm-input={(e: CustomEvent<{ value: string }>) => {
								name = e.detail.value;
								if (nameError && name.trim()) nameError = '';
							}}
						></mm-text-field>
						<PipelineDetailsFields bind:description bind:owner />
					</div>
				</mm-section>
			{:else if step === 'configure'}
				<div class="mm-stack" style="--mm-gap: var(--space-6)">
					{#if queryCapable}
						<QueryAccessSection bind:queryEnabled bind:ingestOnQuery />
					{/if}
					<ConnectionTestCallout result={connectionTest} error={testError} />
					{#if selectedPlugin?.config_spec && selectedPlugin.config_spec.length > 0}
						<ConfigFields
							pluginId={selectedPluginId}
							spec={selectedPlugin.config_spec}
							bind:config
							{fieldErrors}
							{awsStatus}
							onedit={onConfigEdit}
						/>
					{:else}
						<mm-text block variant="dense" tone="secondary">{m.pipelines_no_config_hint()}</mm-text>
					{/if}
				</div>
			{:else}
				<ScheduleFields bind:cronExpression bind:scheduled bind:config {cronError} />
			{/if}

			<hr class="mm-rule" />
			<div class="mm-cluster">
				<mm-button variant="ghost" href={resolve('/runs')}>{m.common_cancel()}</mm-button>
				<span class="mm-push"></span>
				{#if index > 0}
					<mm-button variant="secondary" icon="arrow-back" onclick={back}
						>{m.common_back()}</mm-button
					>
				{/if}
				<mm-button
					variant="primary"
					type="submit"
					loading={validating || saving}
					disabled={validating || saving || (step === 'plugin' && !selectedPluginId)}
					>{step === 'schedule'
						? m.pipelines_create_pipeline()
						: step === 'configure'
							? m.pipelines_check_and_continue()
							: m.pipelines_continue()}</mm-button
				>
			</div>
		</div>

		<aside class="mm-stack" style="--mm-gap: var(--space-4)" aria-label={m.pipelines_help_label()}>
			{#if step === 'schedule'}
				<mm-heading level={2} variant="heading-sm">{m.pipelines_when_created()}</mm-heading>
				<mm-description-list layout="stacked">
					<mm-description term={m.pipelines_first_run()}>{firstRun}</mm-description>
					{#if connectionTest?.succeeded}
						<mm-description term={m.pipelines_tables_visible()}
							>{formatNumber(connectionTest.tables)}</mm-description
						>
					{/if}
					{#if queryCapable}
						<mm-description term={m.plugins_query_card_title()}
							>{queryEnabled ? m.pipelines_answers_queries_yes() : m.common_no()}</mm-description
						>
					{/if}
				</mm-description-list>
			{:else if selectedPlugin}
				<PluginSummary plugin={selectedPlugin} />
				{#if step === 'plugin'}
					<mm-description-list layout="stacked">
						{#if selectedPlugin.catalogs?.length}
							<mm-description term={m.pipelines_catalogs_label()}
								>{formatList(selectedPlugin.catalogs)}</mm-description
							>
						{/if}
						<mm-description term={m.pipelines_lineage_label()}
							>{selectedPlugin.produces_lineage
								? m.pipelines_lineage_yes()
								: m.pipelines_lineage_no()}</mm-description
						>
						{#if connectedHere.length > 0}
							<mm-description term={m.pipelines_already_connected()}
								>{connectedHere.join(', ')}</mm-description
							>
						{/if}
					</mm-description-list>
				{:else if step === 'configure'}
					<mm-text block variant="dense" tone="secondary">{m.pipelines_test_hint()}</mm-text>
					<div>
						<mm-button
							variant="secondary"
							icon="cable"
							loading={testing}
							disabled={testing || validating}
							onclick={runConnectionTest}>{m.pipelines_test_connection()}</mm-button
						>
					</div>
				{/if}
			{/if}
		</aside>
	</form>
</div>
