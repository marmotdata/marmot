<script lang="ts">
	import '@marmotdata/elements/kpi-tile.js';
	import { onDestroy, onMount } from 'svelte';
	import { SvelteURLSearchParams } from 'svelte/reactivity';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { html } from 'lit';
	import type { DataTableColumn } from '@marmotdata/elements/classes/data-table.js';
	import RunDrawer from '$components/runs/RunDrawer.svelte';
	import {
		BASE_FIELDS,
		describeCron,
		formatRunDuration,
		formatStamp,
		isRunning,
		pillState,
		runDurationMs,
		startedBy,
		type JobRun,
		type Pipeline,
		type PluginInfo
	} from '$components/runs/pipeline';
	import { fetchApi } from '$lib/api';
	import { setCrumbs } from '$lib/mm';
	import { auth } from '$lib/stores/auth';
	import { encryptionConfigured } from '$lib/stores/encryption';
	import { toasts } from '$lib/stores/toast';
	import { websocketService, type JobRunEvent } from '$lib/websocket';
	import { m } from '$lib/paraglide/messages';
	import { getLocale } from '$lib/paraglide/runtime';
	import { formatDate, formatDateTime, formatNumber } from '$lib/utils/format';

	type StatusFilter = 'all' | 'succeeded' | 'failed' | 'running' | 'cancelled';

	// One connected source: how it has been doing, every run it has made and how it is set up
	const PAGE_SIZE = 20;
	const DAY = 86_400_000;
	const canManage = auth.hasPermission('ingestion', 'manage');
	const id = page.params.id ?? '';

	let pipeline = $state<Pipeline | null>(null);
	let plugin = $state<PluginInfo | null>(null);
	let loadError = $state('');
	let runs = $state<JobRun[]>([]);
	let runsTotal = $state(0);
	let runsError = $state('');
	let runsLoading = $state(true);
	let pageNumber = $state(1);
	let status = $state<StatusFilter>('all');
	let recent = $state<JobRun[]>([]);
	let triggered = $state(false);
	let selectedRun = $state<JobRun | null>(null);
	let drawerOpen = $state(false);
	let deleteOpen = $state(false);
	let teardown = $state(false);
	let deleting = $state(false);
	let unsubscribe: (() => void) | null = null;
	let refreshTimer: ReturnType<typeof setTimeout> | undefined;
	// The source's queries over the last week; left out when the caller may not see query activity
	let queryStats = $state<{ queries: number; refused: number } | null>(null);

	$effect(() => {
		setCrumbs([
			{ label: m.runs_heading(), href: resolve('/runs') },
			{ label: pipeline?.name ?? id }
		]);
	});

	let managed = $derived(!!pipeline?.managed_by);
	let running = $derived(triggered || recent.some((r) => isRunning(r.status)));
	let schedule = $derived(describeCron(pipeline?.cron_expression));
	let pluginName = $derived(plugin?.name ?? pipeline?.plugin_id ?? '');
	let latest = $derived(recent[0]);

	let caption = $derived(
		pipeline
			? [
					pluginName,
					schedule,
					pipeline.cron_expression && !pipeline.enabled ? m.plugins_schedule_paused() : '',
					pipeline.asset_count ? m.plugins_asset_count({ count: pipeline.asset_count }) : '',
					pipeline.queryable ? m.plugins_filter_queryable() : '',
					managed ? m.runs_operator_managed_title() : ''
				]
					.filter(Boolean)
					.join(' · ')
			: ''
	);

	// The last 30 days of runs drive the headline numbers
	let window30 = $derived(
		recent.filter((r) => Date.now() - new Date(r.created_at).getTime() < 30 * DAY)
	);
	let finished = $derived(
		window30.filter((r) => r.status === 'succeeded' || r.status === 'failed')
	);
	let successRate = $derived(
		finished.length === 0
			? null
			: Math.round(
					(finished.filter((r) => r.status === 'succeeded').length / finished.length) * 100
				)
	);
	let durations = $derived(
		[...window30]
			.reverse()
			.map((r) => (r.finished_at ? runDurationMs(r) : null))
			.filter((d): d is number => d !== null)
	);
	let avgDuration = $derived(
		durations.length === 0 ? null : durations.reduce((n, d) => n + d, 0) / durations.length
	);
	let lastSuccess = $derived(recent.find((r) => r.status === 'succeeded'));

	let nextRun = $derived.by(() => {
		if (!pipeline?.next_run_at || !pipeline.enabled) return null;
		const d = new Date(pipeline.next_run_at);
		const today = new Date();
		const tomorrow = new Date(today.getTime() + DAY);
		const sameDay = (a: Date, b: Date) => a.toDateString() === b.toDateString();
		return {
			value: d.toLocaleTimeString(getLocale(), { hour: '2-digit', minute: '2-digit' }),
			context: sameDay(d, today)
				? m.plugins_today()
				: sameDay(d, tomorrow)
					? m.plugins_tomorrow()
					: formatDate(pipeline.next_run_at)
		};
	});

	// Settings shown at a glance: the plugin's set fields, secrets masked, the essentials first
	let settings = $derived.by(() => {
		const spec = (plugin?.config_spec ?? []).filter(
			(f) => !BASE_FIELDS.has(f.name) && !f.hidden && f.type !== 'object'
		);
		const config = pipeline?.config ?? {};
		return spec
			.filter((f) => {
				const v = config[f.name];
				return v !== undefined && v !== null && v !== '' && !(Array.isArray(v) && v.length === 0);
			})
			.sort((a, b) => Number(!!a.group) - Number(!!b.group))
			.map((f) => ({
				label: f.label,
				secret: f.sensitive || f.type === 'password',
				mono: f.type !== 'bool' && f.type !== 'boolean',
				value: display(config[f.name])
			}));
	});

	let include = $derived(
		(pipeline?.config?.filter as { include?: string[] } | undefined)?.include ?? []
	);
	let exclude = $derived(
		(pipeline?.config?.filter as { exclude?: string[] } | undefined)?.exclude ?? []
	);
	let tags = $derived((pipeline?.config?.tags as string[] | undefined) ?? []);

	function display(v: unknown): string {
		if (typeof v === 'boolean') return v ? m.common_yes() : m.common_no();
		if (Array.isArray(v)) return v.join(', ');
		return String(v);
	}

	const outcomeCell = (r: JobRun) => {
		if (r.status === 'succeeded')
			return html`<mm-text variant="dense">${m.runs_status_succeeded()}</mm-text>`;
		return html`<mm-status-pill
			state=${pillState(r.status)}
			?live=${r.status === 'running'}
		></mm-status-pill>`;
	};

	const runColumns: DataTableColumn<JobRun>[] = [
		{
			key: 'created_at',
			header: m.plugins_col_started(),
			width: 200,
			value: (r) => formatStamp(r.created_at)
		},
		{ key: 'status', header: m.plugins_col_outcome(), width: 130, render: outcomeCell },
		{
			key: 'duration',
			header: m.plugins_col_duration(),
			type: 'number',
			width: 110,
			value: (r) => (r.finished_at ? formatRunDuration(runDurationMs(r)) : '')
		},
		{ key: 'assets_created', header: m.plugins_col_assets_added(), type: 'number', width: 120 },
		{ key: 'assets_updated', header: m.plugins_col_assets_changed(), type: 'number', width: 130 },
		{ key: 'assets_deleted', header: m.plugins_col_assets_removed(), type: 'number', width: 130 },
		{
			key: 'created_by',
			header: m.plugins_started_by(),
			minWidth: 140,
			value: (r) => startedBy(r.created_by)
		}
	];

	async function loadPipeline() {
		try {
			const res = await fetchApi(`/ingestion/schedules/${encodeURIComponent(id)}`);
			if (!res.ok) throw new Error(m.plugins_detail_load_error());
			pipeline = await res.json();
			if (pipeline?.queryable) loadQueryStats(pipeline.name);
			const plugins = await fetchApi('/plugins');
			if (plugins.ok) {
				const list: PluginInfo[] = (await plugins.json()).plugins ?? [];
				plugin = list.find((p) => p.id === pipeline?.plugin_id) ?? null;
			}
		} catch (e) {
			loadError = e instanceof Error ? e.message : m.plugins_detail_load_error();
		}
	}

	async function loadQueryStats(name: string) {
		try {
			const res = await fetchApi(`/query/sources/${encodeURIComponent(name)}/stats`);
			if (!res.ok) return;
			const data = await res.json();
			queryStats = { queries: data.queries ?? 0, refused: data.refused ?? 0 };
		} catch {
			// The section still says how the source answers queries
		}
	}

	async function loadRuns() {
		runsLoading = true;
		runsError = '';
		const params = new SvelteURLSearchParams({
			schedule_id: id,
			limit: String(PAGE_SIZE),
			offset: String((pageNumber - 1) * PAGE_SIZE)
		});
		if (status !== 'all') params.set('status', status);
		try {
			const res = await fetchApi(`/ingestion/runs?${params}`);
			if (!res.ok) throw new Error(m.plugins_runs_load_error());
			const data = await res.json();
			runs = data.runs ?? [];
			runsTotal = data.total ?? 0;
		} catch (e) {
			runsError = e instanceof Error ? e.message : m.plugins_runs_load_error();
		} finally {
			runsLoading = false;
		}
	}

	async function loadRecent() {
		try {
			const res = await fetchApi(`/ingestion/runs?schedule_id=${encodeURIComponent(id)}&limit=200`);
			if (!res.ok) return;
			recent = (await res.json()).runs ?? [];
			if (!recent.some((r) => isRunning(r.status))) triggered = false;
		} catch {
			// The headline numbers stay as they were
		}
	}

	async function runNow() {
		triggered = true;
		try {
			const res = await fetchApi(`/ingestion/schedules/${encodeURIComponent(id)}/trigger`, {
				method: 'POST'
			});
			if (!res.ok)
				throw new Error((await res.json().catch(() => ({}))).error || m.runs_trigger_error());
			scheduleRefresh();
		} catch (e) {
			triggered = false;
			toasts.error(e instanceof Error ? e.message : m.runs_trigger_error());
		}
	}

	async function deletePipeline(e: CustomEvent) {
		if (!pipeline) return;
		e.preventDefault();
		deleting = true;
		try {
			const res = await fetchApi(
				`/ingestion/schedules/${encodeURIComponent(id)}${teardown ? '?teardown=true' : ''}`,
				{ method: 'DELETE' }
			);
			if (!res.ok)
				throw new Error(
					(await res.json().catch(() => ({}))).error || m.runs_delete_pipeline_error()
				);
			toasts.success(
				teardown
					? m.runs_delete_teardown_success({ name: pipeline.name })
					: m.runs_delete_success({ name: pipeline.name })
			);
			deleteOpen = false;
			goto(resolve('/runs'));
		} catch (err) {
			toasts.error(err instanceof Error ? err.message : m.runs_delete_pipeline_error());
		} finally {
			deleting = false;
		}
	}

	async function openRun(runId: string) {
		const known = runs.find((r) => r.id === runId) ?? recent.find((r) => r.id === runId);
		if (known) {
			selectedRun = known;
			drawerOpen = true;
		}
		// The list leaves out the log, so the full run is fetched for the drawer
		try {
			const res = await fetchApi(`/ingestion/runs/${encodeURIComponent(runId)}`);
			if (!res.ok) throw new Error();
			selectedRun = await res.json();
			drawerOpen = true;
		} catch {
			if (!known) toasts.error(m.plugins_run_load_error());
		}
	}

	function pipelineAction(action: string) {
		if (action === 'copy-id') {
			navigator.clipboard?.writeText(id).then(() => toasts.success(m.plugins_id_copied()));
		} else if (action === 'delete') {
			teardown = false;
			deleteOpen = true;
		}
	}

	// Live updates arrive over the websocket; a short debounce folds bursts of progress events into one reload
	function scheduleRefresh() {
		clearTimeout(refreshTimer);
		refreshTimer = setTimeout(() => {
			loadRuns();
			loadRecent();
			if (drawerOpen && selectedRun) openRun(selectedRun.id);
		}, 400);
	}

	function onRunEvent(event: JobRunEvent) {
		if (event.payload.schedule_id !== id) return;
		scheduleRefresh();
	}

	onMount(() => {
		loadPipeline();
		loadRuns();
		loadRecent();
		unsubscribe = websocketService.subscribe(onRunEvent);
	});

	onDestroy(() => {
		unsubscribe?.();
		clearTimeout(refreshTimer);
	});
</script>

<svelte:head>
	<title>{pipeline ? m.plugins_detail_title({ name: pipeline.name }) : m.pagetitle_runs()}</title>
</svelte:head>

{#if loadError}
	<mm-callout tone="danger" heading={loadError}>
		<mm-button slot="actions" size="sm" variant="secondary" href={resolve('/runs')}
			>{m.plugins_back_to_plugins()}</mm-button
		>
	</mm-callout>
{:else if !pipeline}
	<div class="mm-stack">
		<mm-skeleton shape="heading"></mm-skeleton>
		<mm-skeleton count={4}></mm-skeleton>
	</div>
{:else}
	<div class="mm-stack">
		<mm-page-header
			heading={pipeline.name}
			provider={pipeline.plugin_id}
			{caption}
			description={pipeline.description ?? ''}
		>
			{#if canManage && !managed}
				<mm-menu-button
					slot="actions"
					variant="secondary"
					icon="more-horiz"
					icon-only
					label={m.plugins_more_pipeline_actions()}
				>
					<mm-menu
						label={m.plugins_pipeline_actions()}
						onmm-select={(e: CustomEvent<{ value: string }>) => pipelineAction(e.detail.value)}
					>
						<mm-menu-item icon="content-copy" value="copy-id">{m.plugins_copy_id()}</mm-menu-item>
						<mm-menu-separator></mm-menu-separator>
						<mm-menu-item icon="delete" value="delete" danger
							>{m.runs_delete_pipeline_title()}</mm-menu-item
						>
					</mm-menu>
				</mm-menu-button>
				<mm-button
					slot="actions"
					variant="secondary"
					icon="edit"
					href={resolve('/pipelines/[id]/edit', { id })}
					disabled={!$encryptionConfigured}>{m.pipelines_edit_pipeline()}</mm-button
				>
				<mm-button
					slot="actions"
					variant="primary"
					icon="play-arrow"
					loading={running}
					disabled={running || !$encryptionConfigured}
					onclick={runNow}>{running ? m.plugins_running() : m.plugins_run_now()}</mm-button
				>
			{/if}
		</mm-page-header>

		{#if latest?.status === 'failed'}
			<mm-callout tone="danger" heading={m.plugins_last_run_failed()}>
				{latest.error_message ?? ''}
				<mm-button
					slot="actions"
					size="sm"
					variant="secondary"
					icon="article"
					onclick={() => openRun(latest.id)}>{m.plugins_view_run()}</mm-button
				>
			</mm-callout>
		{/if}

		{#if recent.length > 0}
			<mm-kpi-row label={m.plugins_health_label()}>
				<mm-kpi-tile
					label={m.plugins_stat_success()}
					value={successRate ?? '—'}
					unit={successRate === null ? undefined : '%'}
					context={m.runs_period_30d()}
				></mm-kpi-tile>
				<mm-kpi-tile
					label={m.plugins_runs_heading()}
					value={window30.length}
					context={m.runs_period_30d()}
				></mm-kpi-tile>
				<mm-kpi-tile
					label={m.plugins_stat_duration()}
					value={avgDuration === null ? '—' : formatRunDuration(avgDuration)}
					spark={durations.length >= 6 ? durations.map((d) => Math.round(d / 1000)) : undefined}
				></mm-kpi-tile>
				<mm-kpi-tile
					label={m.plugins_stat_last_success()}
					value={lastSuccess ? formatDate(lastSuccess.created_at) : '—'}
				></mm-kpi-tile>
				{#if nextRun}
					<mm-kpi-tile
						label={m.plugins_stat_next_run()}
						value={nextRun.value}
						context={nextRun.context}
					></mm-kpi-tile>
				{/if}
			</mm-kpi-row>
		{:else if nextRun}
			<!-- Before the first run only the next run is known; the empty runs table already says there are none -->
			<mm-kpi-row label={m.plugins_health_label()}>
				<mm-kpi-tile
					label={m.plugins_stat_next_run()}
					value={nextRun.value}
					context={nextRun.context}
				></mm-kpi-tile>
			</mm-kpi-row>
		{/if}

		<mm-section heading={m.plugins_runs_heading()}>
			<mm-select
				slot="actions"
				label={m.common_status()}
				hide-label
				value={status}
				style="inline-size: 11rem"
				onmm-change={(e: CustomEvent<{ value: string }>) => {
					status = e.detail.value as StatusFilter;
					pageNumber = 1;
					loadRuns();
				}}
			>
				<mm-option value="all">{m.plugins_all_statuses()}</mm-option>
				<mm-option value="failed">{m.runs_status_failed()}</mm-option>
				<mm-option value="running">{m.runs_status_running()}</mm-option>
				<mm-option value="succeeded">{m.runs_status_succeeded()}</mm-option>
				<mm-option value="cancelled">{m.runs_status_cancelled()}</mm-option>
			</mm-select>
			<div class="mm-stack" style="--mm-gap: var(--space-3)">
				<mm-data-table
					label={m.plugins_runs_of({ name: pipeline.name })}
					density="compact"
					row-key="id"
					columns={runColumns}
					rows={runs}
					loading={runsLoading}
					error={runsError}
					selected={drawerOpen && selectedRun ? [selectedRun.id] : []}
					empty-heading={status === 'all'
						? m.plugins_no_runs_heading()
						: m.runs_no_runs_filtered_hint()}
					empty-text={status === 'all'
						? pipeline.cron_expression && pipeline.enabled
							? m.plugins_no_runs_scheduled()
							: m.plugins_no_runs()
						: m.plugins_no_runs_filtered_body()}
					onmm-row-activate={(e: CustomEvent<{ row: object }>) =>
						openRun((e.detail.row as JobRun).id)}
					onmm-retry={loadRuns}
				></mm-data-table>
				{#if runsTotal > PAGE_SIZE}
					<mm-pagination
						page={pageNumber}
						page-size={PAGE_SIZE}
						total={runsTotal}
						page-sizes=""
						label={m.plugins_runs_pages()}
						onmm-page-change={(e: CustomEvent<{ page: number }>) => {
							pageNumber = e.detail.page;
							loadRuns();
						}}
					></mm-pagination>
				{/if}
			</div>
		</mm-section>

		<div class="mm-split">
			<mm-section heading={m.plugins_settings_heading()}>
				<mm-description-list>
					<mm-description term={m.plugins_plugin_label()}>{pluginName}</mm-description>
					{#each settings as row (row.label)}
						<mm-description term={row.label} mono={row.mono && !row.secret}
							>{row.secret ? '••••••••' : row.value}</mm-description
						>
					{/each}
					<mm-description term={m.plugins_col_schedule()}
						>{schedule}
						{#if pipeline.cron_expression}
							<mm-text variant="mono-dense" tone="secondary">{pipeline.cron_expression}</mm-text>
						{/if}</mm-description
					>
					{#if include.length > 0}
						<mm-description term={m.pipelines_filter_include_label()} mono
							>{include.join(', ')}</mm-description
						>
					{/if}
					{#if exclude.length > 0}
						<mm-description term={m.pipelines_filter_exclude_label()} mono
							>{exclude.join(', ')}</mm-description
						>
					{/if}
					{#if tags.length > 0}
						<mm-description term={m.common_tags()}>{tags.join(' · ')}</mm-description>
					{/if}
					{#if managed}
						<mm-description term={m.plugins_runs_on()}
							>{m.plugins_runs_on_kubernetes()}</mm-description
						>
					{/if}
					{#if pipeline.owner_name}
						<mm-description term={m.pipelines_owner_label()}
							><mm-owner-chip
								name={pipeline.owner_name}
								kind={pipeline.owner_type === 'team' ? 'team' : 'person'}
							></mm-owner-chip></mm-description
						>
					{/if}
					{#if pipeline.created_by}
						<mm-description term={m.plugins_created_by()}>{pipeline.created_by}</mm-description>
					{/if}
					<mm-description term={m.plugins_created_label()}
						>{formatDateTime(pipeline.created_at)}</mm-description
					>
				</mm-description-list>
			</mm-section>

			{#if pipeline.queryable}
				<mm-section
					heading={m.plugins_query_card_title()}
					description={m.plugins_query_card_body()}
				>
					<div class="mm-stack" style="--mm-gap: var(--space-4)">
						<mm-description-list>
							{#if queryStats}
								<mm-description term={m.plugins_queries_7d()}
									>{formatNumber(queryStats.queries)}</mm-description
								>
								<mm-description term={m.plugins_refused_7d()}
									>{formatNumber(queryStats.refused)}</mm-description
								>
							{/if}
							<mm-description term={m.pipelines_ingest_on_query_label()}
								>{pipeline.ingest_on_query ? m.common_yes() : m.common_no()}</mm-description
							>
						</mm-description-list>
						<div class="mm-cluster">
							<mm-button
								size="sm"
								variant="secondary"
								icon="terminal"
								href={resolve(
									`/query?source=${encodeURIComponent(pipeline.name)}` as `/query?${string}`
								)}>{m.plugins_open_in_query()}</mm-button
							>
							<mm-button size="sm" variant="ghost" icon="policy" href={resolve('/query-audit')}
								>{m.plugins_view_query_audit()}</mm-button
							>
						</div>
					</div>
				</mm-section>
			{:else if plugin?.supports_query && canManage && !managed}
				<mm-section heading={m.plugins_query_card_title()} description={m.plugins_query_off_body()}>
					<div class="mm-cluster">
						<mm-button
							size="sm"
							variant="secondary"
							icon="terminal"
							href={resolve(
								`/pipelines/${id}/edit?query=1` as `/pipelines/${string}/edit?${string}`
							)}>{m.plugins_turn_on_querying()}</mm-button
						>
					</div>
				</mm-section>
			{/if}
		</div>
	</div>
{/if}

<RunDrawer run={selectedRun} open={drawerOpen} {pluginName} onclose={() => (drawerOpen = false)} />

<mm-confirm-dialog
	open={deleteOpen}
	heading={m.plugins_delete_heading({ name: pipeline?.name ?? '' })}
	confirm-label={m.runs_delete_pipeline_title()}
	loading={deleting}
	onmm-confirm={deletePipeline}
	onmm-close={() => (deleteOpen = false)}
>
	<div class="mm-stack" style="--mm-gap: var(--space-4)">
		<mm-text block variant="dense">{m.plugins_delete_body()}</mm-text>
		<mm-checkbox
			label={m.runs_delete_teardown_checkbox_label()}
			checked={teardown}
			onmm-change={(e: CustomEvent<{ checked: boolean }>) => (teardown = e.detail.checked)}
		></mm-checkbox>
	</div>
</mm-confirm-dialog>
