<script lang="ts">
	import { onDestroy, onMount, tick } from 'svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { get } from 'svelte/store';
	import { Cron } from 'croner';
	import { fetchApi } from '$lib/api';
	import { auth } from '$lib/stores/auth';
	import { encryptionConfigured } from '$lib/stores/encryption';
	import { toasts } from '$lib/stores/toast';
	import { setCrumbs } from '$lib/mm';
	import { m } from '$lib/paraglide/messages';
	import QueryAccessSection from '$components/dataquery/QueryAccessSection.svelte';
	import ConfigFields from '$components/runs/ConfigFields.svelte';
	import ConnectionTestCallout from '$components/runs/ConnectionTestCallout.svelte';
	import PipelineDetailsFields from '$components/runs/PipelineDetailsFields.svelte';
	import PluginSummary from '$components/runs/PluginSummary.svelte';
	import type { Owner } from '$lib/dataproducts/types';
	import ScheduleFields from '$components/runs/ScheduleFields.svelte';
	import {
		AWS_PLUGINS,
		cleanConfigForSubmit,
		detectSourceType,
		fillMissingDefaults,
		testConnection,
		type ConfigObject,
		type ConnectionTestResult,
		type Pipeline,
		type PluginInfo
	} from '$components/runs/pipeline';

	interface EditConfig extends ConfigObject {
		tags?: string[];
		external_links?: { name: string; url: string }[];
		filter?: { include?: string[]; exclude?: string[] };
	}

	const pipelineId = page.params.id ?? '';

	// Editing uses the create flow's sections on one page, in the same order, since everything already has a value
	let plugins = $state<PluginInfo[]>([]);
	let pluginsStillLoading = $state(false);
	let pluginPollTimer: ReturnType<typeof setTimeout> | null = null;
	let loading = $state(true);
	let loadError = $state('');
	let pluginId = $state('');
	let originalName = $state('');
	let name = $state('');
	let nameError = $state('');
	let description = $state('');
	let owner = $state<Owner | null>(null);
	let connectionTest = $state<ConnectionTestResult | null>(null);
	let testError = $state('');
	let testing = $state(false);
	let config = $state<EditConfig>({});
	let queryEnabled = $state(false);
	let ingestOnQuery = $state(false);
	let scheduled = $state(false);
	let cronExpression = $state('');
	let cronError = $state('');
	let disableSchedule = $state(false);
	let fieldErrors = $state<Record<string, string>>({});
	let saving = $state(false);
	let error = $state('');
	let awsStatus = $state<{ available: boolean; sources: string[]; error?: string } | null>(null);

	let plugin = $derived(plugins.find((p) => p.id === pluginId) ?? null);
	let queryCapable = $derived(plugin?.supports_query === true);

	$effect(() => {
		setCrumbs([
			{ label: m.runs_heading(), href: resolve('/runs') },
			{ label: originalName || pipelineId, href: resolve('/runs/[id]', { id: pipelineId }) },
			{ label: m.common_edit() }
		]);
	});

	async function fetchPlugins() {
		try {
			const res = await fetchApi('/plugins');
			if (!res.ok) throw new Error(m.pipelines_error_load_plugins());
			const data = await res.json();
			plugins = Array.isArray(data?.plugins) ? data.plugins : [];
			pluginsStillLoading = Boolean(data?.loading);
		} catch {
			error = m.pipelines_error_load_plugins();
		} finally {
			if (pluginPollTimer) clearTimeout(pluginPollTimer);
			pluginPollTimer = pluginsStillLoading ? setTimeout(fetchPlugins, 2000) : null;
		}
	}

	async function fetchPipeline() {
		try {
			const res = await fetchApi(`/ingestion/schedules/${encodeURIComponent(pipelineId)}`);
			if (!res.ok) throw new Error(m.pipelines_error_load_pipeline());
			const pipeline: Pipeline = await res.json();
			originalName = pipeline.name;
			name = pipeline.name;
			description = pipeline.description ?? '';
			owner =
				pipeline.owner_type && pipeline.owner_id
					? { id: pipeline.owner_id, type: pipeline.owner_type, name: pipeline.owner_name ?? '' }
					: null;
			pluginId = pipeline.plugin_id;
			cronExpression = pipeline.cron_expression || '';
			scheduled = !!pipeline.cron_expression;
			disableSchedule = !pipeline.enabled && !!pipeline.cron_expression;
			queryEnabled = pipeline.queryable === true;
			ingestOnQuery = pipeline.ingest_on_query === true;

			const next: EditConfig = { ...(pipeline.config ?? {}) };
			next.tags = next.tags ?? [];
			next.external_links = next.external_links ?? [];
			next.filter = { include: next.filter?.include ?? [], exclude: next.filter?.exclude ?? [] };
			const spec = plugins.find((p) => p.id === pipeline.plugin_id)?.config_spec;
			if (spec) fillMissingDefaults(spec, next);
			config = next;
			detectSourceType(spec, config);

			// The Query page's "Turn on" link arrives with ?query=1, so the switch is flipped and shown, ready to save
			if (page.url.searchParams.get('query') === '1' && auth.hasPermission('access', 'manage')) {
				queryEnabled = true;
				ingestOnQuery = true;
				await tick();
				document.getElementById('query-access')?.scrollIntoView({ behavior: 'smooth' });
			}
		} catch (e) {
			loadError = e instanceof Error ? e.message : m.pipelines_error_load_pipeline();
		}
	}

	async function fetchAwsStatus() {
		try {
			const res = await fetchApi('/plugins/aws/credentials/status');
			if (res.ok) awsStatus = await res.json();
		} catch {
			// Without the status the fields simply show
		}
	}

	function onConfigEdit(path: string) {
		if (fieldErrors[path]) {
			const next = { ...fieldErrors };
			delete next[path];
			fieldErrors = next;
		}
		connectionTest = null;
		testError = '';
		detectSourceType(plugin?.config_spec, config);
	}

	// The pipeline's id goes with the test so secrets shown masked are tried with their stored values
	async function runConnectionTest() {
		if (!plugin) return;
		testing = true;
		connectionTest = null;
		testError = '';
		error = '';
		try {
			const res = await testConnection(
				pluginId,
				cleanConfigForSubmit(config, plugin.config_spec ?? []),
				pipelineId
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
		if (!plugin) return true;
		const res = await fetchApi('/ingestion/validate', {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({
				plugin_id: pluginId,
				config: cleanConfigForSubmit(config, plugin.config_spec ?? [])
			})
		});
		if (!res.ok) throw new Error(m.pipelines_error_validate());
		const result = await res.json();
		if (!result.valid && result.errors?.length > 0) {
			const errors: Record<string, string> = {};
			for (const e of result.errors) errors[e.field] = e.message;
			fieldErrors = errors;
			error = m.pipelines_error_check_fields();
			focusField(result.errors[0].field);
			return false;
		}
		return true;
	}

	async function save() {
		error = '';
		fieldErrors = {};
		if (!name.trim()) {
			nameError = m.pipelines_error_name_required();
			focusField('name');
			return;
		}
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
			if (!(await validateConfig())) return;
			const res = await fetchApi(`/ingestion/schedules/${encodeURIComponent(pipelineId)}`, {
				method: 'PUT',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({
					name: name.trim(),
					description: description.trim(),
					owner_type: owner?.type ?? '',
					owner_id: owner?.id ?? '',
					plugin_id: pluginId,
					config: plugin?.config_spec ? cleanConfigForSubmit(config, plugin.config_spec) : config,
					cron_expression: scheduled ? cronExpression.trim() : '',
					// A pipeline without a schedule only runs when started, so it is always enabled
					enabled: scheduled ? !disableSchedule : true,
					queryable: queryCapable && queryEnabled,
					ingest_on_query: queryCapable && queryEnabled && ingestOnQuery
				})
			});
			if (!res.ok) {
				const data = await res.json().catch(() => ({}));
				throw new Error(data.error || m.pipelines_error_update());
			}
			toasts.success(m.pipelines_saved({ name: name.trim() }));
			goto(resolve('/runs/[id]', { id: pipelineId }));
		} catch (e) {
			error = e instanceof Error ? e.message : m.pipelines_error_update();
		} finally {
			saving = false;
		}
	}

	onMount(async () => {
		if (!get(encryptionConfigured)) {
			toasts.error(m.pipelines_error_encryption_not_configured());
			goto(resolve('/runs'));
			return;
		}
		await fetchPlugins();
		await fetchPipeline();
		loading = false;
		if (AWS_PLUGINS.includes(pluginId)) fetchAwsStatus();
	});

	onDestroy(() => {
		if (pluginPollTimer) clearTimeout(pluginPollTimer);
	});
</script>

<svelte:head>
	<title>{m.pipelines_edit_pipeline()}</title>
</svelte:head>

{#if loadError}
	<mm-callout tone="danger" heading={loadError}>
		<mm-button slot="actions" size="sm" variant="secondary" href={resolve('/runs')}
			>{m.plugins_back_to_plugins()}</mm-button
		>
	</mm-callout>
{:else}
	<div class="mm-stack" style="max-inline-size: 72rem">
		<mm-page-header heading={m.pipelines_edit_pipeline()}></mm-page-header>

		{#if loading}
			<mm-skeleton count={6}></mm-skeleton>
		{:else}
			<form
				class="mm-split"
				style="gap: var(--space-12)"
				aria-label={m.pipelines_edit_pipeline()}
				novalidate
				onsubmit={(e) => {
					e.preventDefault();
					save();
				}}
			>
				<div class="mm-stack" style="--mm-gap: var(--space-8)">
					{#if error}
						<mm-callout tone="danger" compact heading={error}></mm-callout>
					{/if}
					{#if pluginsStillLoading}
						<mm-callout tone="info" compact heading={m.pipelines_plugins_still_loading_view()}
						></mm-callout>
					{/if}

					{#if queryCapable}
						<QueryAccessSection bind:queryEnabled bind:ingestOnQuery />
					{/if}

					<mm-section heading={m.pipelines_step_basic_info()}>
						<div class="mm-stack" style="--mm-gap: var(--space-5)">
							<mm-text-field
								label={m.pipelines_name_label()}
								name="name"
								mono
								required
								maxlength={63}
								autocomplete="off"
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

					{#if plugin?.config_spec && plugin.config_spec.length > 0}
						<mm-section heading={m.pipelines_step_configure()}>
							<div class="mm-stack" style="--mm-gap: var(--space-6)">
								<ConnectionTestCallout result={connectionTest} error={testError} />
								<ConfigFields
									{pluginId}
									spec={plugin.config_spec}
									bind:config
									{fieldErrors}
									{awsStatus}
									onedit={onConfigEdit}
								/>
							</div>
						</mm-section>
					{/if}

					<mm-section heading={m.pipelines_step_schedule_filter()}>
						<ScheduleFields
							bind:cronExpression
							bind:scheduled
							bind:config
							bind:disableSchedule
							showPause
							{cronError}
						/>
					</mm-section>

					<hr class="mm-rule" />
					<div class="mm-cluster">
						<mm-button variant="ghost" href={resolve('/runs/[id]', { id: pipelineId })}
							>{m.common_cancel()}</mm-button
						>
						<span class="mm-push"></span>
						<mm-button variant="primary" type="submit" loading={saving} disabled={saving}
							>{m.pipelines_save_changes()}</mm-button
						>
					</div>
				</div>

				<aside
					class="mm-stack"
					style="--mm-gap: var(--space-4)"
					aria-label={m.pipelines_help_label()}
				>
					{#if plugin}
						<PluginSummary {plugin} />
						<mm-text block variant="caption" tone="secondary"
							>{m.pipelines_plugin_locked_hint()}</mm-text
						>
						<mm-text block variant="dense" tone="secondary">{m.pipelines_test_hint()}</mm-text>
						<div>
							<mm-button
								variant="secondary"
								icon="cable"
								loading={testing}
								disabled={testing || saving}
								onclick={runConnectionTest}>{m.pipelines_test_connection()}</mm-button
							>
						</div>
					{/if}
				</aside>
			</form>
		{/if}
	</div>
{/if}
