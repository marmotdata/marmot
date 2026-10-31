<script lang="ts">
	import '@marmotdata/elements/run-strip.js';
	import '@marmotdata/elements/toolbar.js';
	import { onMount, onDestroy } from 'svelte';
	import { SvelteSet, SvelteURLSearchParams } from 'svelte/reactivity';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { html, nothing } from 'lit';
	import type { DataTableColumn } from '@marmotdata/elements/classes/data-table.js';
	import type { Run as StripRun } from '@marmotdata/elements/classes/run-strip.js';
	import { fetchApi } from '$lib/api';
	import { m } from '$lib/paraglide/messages';
	import { websocketService, type JobRunEvent } from '$lib/websocket';
	import { auth } from '$lib/stores/auth';
	import { toasts } from '$lib/stores/toast';
	import { encryptionConfigured, allowUnencrypted } from '$lib/stores/encryption';
	import { setCrumbs } from '$lib/mm';
	import { formatRelativeTime } from '$lib/utils/format';
	import PluginGallery from '$components/runs/PluginGallery.svelte';
	import RunDrawer from '$components/runs/RunDrawer.svelte';
	import {
		describeCron,
		isRunning,
		pillState,
		toStripRun,
		type JobRun,
		type Pipeline,
		type PluginInfo
	} from '$components/runs/pipeline';

	// Plugins: the pipelines already connected first, then the gallery of every plugin that could be connected
	const PAGE_SIZE = 25;
	const STRIP = 7;
	const canManage = auth.hasPermission('ingestion', 'manage');

	type StatusFilter = 'all' | 'failed' | 'running' | 'succeeded' | 'never';

	interface Row extends Pipeline {
		pluginName: string;
		running: boolean;
		strip: StripRun[];
	}

	setCrumbs([{ label: m.runs_heading() }]);

	let pipelines = $state<Pipeline[]>([]);
	let total = $state(0);
	let pageNumber = $state(1);
	let loading = $state(true);
	let loadError = $state('');
	let search = $state('');
	let status = $state<StatusFilter>('all');
	let plugins = $state<PluginInfo[]>([]);
	let strips = $state<Record<string, StripRun[]>>({});
	let queryOnly = $state(page.url.searchParams.get('query') === '1');
	const runningPipelines = new SvelteSet<string>();

	let selectedRun = $state<JobRun | null>(null);
	let drawerOpen = $state(false);
	let deleteTarget = $state<Pipeline | null>(null);
	let deleteOpen = $state(false);
	let teardown = $state(false);
	let deleting = $state(false);

	let unsubscribe: (() => void) | null = null;

	// Every pipeline counted by plugin across all pages, so the gallery's connected counts ignore the filters
	let connectedCounts = $state<Record<string, number>>({});
	let searchTimer: ReturnType<typeof setTimeout> | undefined;
	let filtered = $derived(search.trim() !== '' || status !== 'all');

	let pluginNames = $derived(Object.fromEntries(plugins.map((p) => [p.id, p.name])));

	// The server searches and filters by status, so the rows are the page it returned
	let rows = $derived.by<Row[]>(() =>
		pipelines.map((p) => ({
			...p,
			pluginName: pluginNames[p.plugin_id] ?? p.plugin_id,
			running: runningPipelines.has(p.id) || isRunning(p.last_run_status),
			strip: strips[p.id] ?? []
		}))
	);

	const lastRunCell = (r: Row) => {
		if (r.running) return html`<mm-status-pill state="running" live></mm-status-pill>`;
		if (!r.last_run_at)
			return html`<mm-text variant="dense" tone="tertiary">${m.plugins_never_run()}</mm-text>`;
		const state = pillState(r.last_run_status);
		return html`<span class="mm-cluster" style="--mm-gap:var(--space-2);flex-wrap:nowrap"
			>${state === 'succeeded' ? nothing : html`<mm-status-pill state=${state}></mm-status-pill>`}<mm-text
				variant="dense"
				tone=${state === 'succeeded' ? 'default' : 'secondary'}
				>${formatRelativeTime(r.last_run_at)}</mm-text
			></span
		>`;
	};

	const actionsCell = (r: Row) => {
		if (r.managed_by)
			return html`<mm-text variant="caption" tone="secondary"
				>${m.runs_operator_managed_title()}</mm-text
			>`;
		if (!canManage) return nothing;
		return html`<span
			class="mm-cluster"
			style="--mm-gap:var(--space-1);justify-content:flex-end;flex-wrap:nowrap"
		>
			<mm-button
				size="sm"
				variant="ghost"
				icon="play-arrow"
				?disabled=${r.running || !$encryptionConfigured}
				@click=${() => triggerPipeline(r)}
				>${m.plugins_run_now()}</mm-button
			>
			<mm-menu-button
				size="sm"
				variant="ghost"
				icon="more-horiz"
				icon-only
				label=${m.plugins_more_actions_for({ name: r.name })}
			>
				<mm-menu
					label=${m.plugins_actions_for({ name: r.name })}
					@mm-select=${(e: CustomEvent<{ value: string }>) => rowAction(r, e.detail.value)}
				>
					<mm-menu-item icon="edit" value="edit" ?disabled=${!$encryptionConfigured}
						>${m.pipelines_edit_pipeline()}</mm-menu-item
					>
					<mm-menu-item icon="history" value="runs">${m.plugins_view_runs()}</mm-menu-item>
					<mm-menu-separator></mm-menu-separator>
					<mm-menu-item icon="delete" value="delete" danger
						>${m.runs_delete_pipeline_title()}</mm-menu-item
					>
				</mm-menu>
			</mm-menu-button>
		</span>`;
	};

	let columns = $derived<DataTableColumn<Row>[]>([
		{
			key: 'name',
			header: m.plugins_col_pipeline(),
			width: 240,
			sortable: true,
			href: (r) => resolve('/runs/[id]', { id: r.id }),
			render: (r) =>
				html`<mm-provider-logo provider=${r.plugin_id} size="16" decorative></mm-provider-logo
					>&nbsp;&nbsp;${r.name}`
		},
		{
			key: 'last_run_at',
			header: m.plugins_stat_last_run(),
			width: 180,
			sortable: true,
			render: lastRunCell
		},
		{
			key: 'strip',
			header: m.plugins_recent_runs(),
			width: 140,
			render: (r) =>
				r.strip.length
					? html`<mm-run-strip
							.runs=${r.strip}
							label=${m.plugins_strip_label({ name: r.name })}
							?live=${r.running}
							selectable
							@mm-select=${(e: CustomEvent<{ run: StripRun }>) => openRun(e.detail.run.id)}
						></mm-run-strip>`
					: nothing
		},
		{
			key: 'cron_expression',
			header: m.plugins_col_schedule(),
			minWidth: 160,
			render: (r) =>
				html`<span class="mm-cluster" style="--mm-gap:var(--space-2);flex-wrap:nowrap"
					>${describeCron(r.cron_expression)}${
						r.cron_expression && !r.enabled
							? html`<mm-status-pill status="neutral"
									>${m.plugins_schedule_paused()}</mm-status-pill
								>`
							: nothing
					}</span
				>`
		},
		{
			key: 'asset_count',
			header: m.plugins_col_assets(),
			type: 'number',
			width: 96,
			sortable: true,
			value: (r) => r.asset_count ?? 0
		},
		{
			key: 'actions',
			header: m.plugins_col_actions(),
			width: 170,
			minWidth: 170,
			align: 'end',
			render: actionsCell
		}
	]);

	async function fetchPlugins() {
		try {
			const res = await fetchApi('/plugins');
			if (res.ok) plugins = (await res.json()).plugins ?? [];
		} catch {
			// The gallery stays empty; connected pipelines still show
		}
	}

	async function fetchPipelines() {
		loading = true;
		loadError = '';
		try {
			const params = new SvelteURLSearchParams({
				limit: String(PAGE_SIZE),
				offset: String((pageNumber - 1) * PAGE_SIZE)
			});
			if (search.trim()) params.set('search', search.trim());
			if (status !== 'all') params.set('status', status);
			const res = await fetchApi(`/ingestion/schedules?${params}`);
			if (!res.ok) throw new Error(m.runs_pipelines_fetch_error());
			const data = await res.json();
			// The API still names pipelines schedules
			pipelines = data.schedules ?? [];
			total = data.total ?? 0;
			connectedCounts = data.plugin_counts ?? {};
			loadStrips(pipelines);
		} catch (err) {
			loadError = err instanceof Error ? err.message : m.runs_pipelines_load_error();
		} finally {
			loading = false;
		}
	}

	// Each pipeline's last few runs, oldest first, for its run strip
	async function loadStrip(id: string) {
		try {
			const res = await fetchApi(
				`/ingestion/runs?schedule_id=${encodeURIComponent(id)}&limit=${STRIP}`
			);
			if (!res.ok) return;
			const runs: JobRun[] = (await res.json()).runs ?? [];
			strips = { ...strips, [id]: runs.map(toStripRun).reverse() };
		} catch {
			// The strip is a summary; the row still works without it
		}
	}

	function loadStrips(list: Pipeline[]) {
		for (const p of list) loadStrip(p.id);
	}

	async function openRun(runId: string) {
		try {
			const res = await fetchApi(`/ingestion/runs/${encodeURIComponent(runId)}`);
			if (!res.ok) throw new Error();
			selectedRun = await res.json();
			drawerOpen = true;
		} catch {
			toasts.error(m.plugins_run_load_error());
		}
	}

	// Typing waits a moment before asking the server, so a search is one request rather than one per key
	function onSearch(value: string) {
		search = value;
		clearTimeout(searchTimer);
		searchTimer = setTimeout(() => {
			pageNumber = 1;
			fetchPipelines();
		}, 250);
	}

	function onStatus(value: StatusFilter) {
		status = value;
		pageNumber = 1;
		fetchPipelines();
	}

	function rowAction(p: Pipeline, action: string) {
		if (action === 'edit') goto(resolve('/pipelines/[id]/edit', { id: p.id }));
		else if (action === 'runs') goto(resolve('/runs/[id]', { id: p.id }));
		else if (action === 'delete') {
			deleteTarget = p;
			teardown = false;
			deleteOpen = true;
		}
	}

	async function triggerPipeline(p: Pipeline) {
		runningPipelines.add(p.id);
		try {
			const res = await fetchApi(`/ingestion/schedules/${encodeURIComponent(p.id)}/trigger`, {
				method: 'POST'
			});
			if (!res.ok) {
				const data = await res.json().catch(() => ({}));
				throw new Error(data.error || m.runs_trigger_error());
			}
			watchRun(p.id);
		} catch (err) {
			runningPipelines.delete(p.id);
			toasts.error(err instanceof Error ? err.message : m.runs_trigger_error());
		}
	}

	// Without the websocket a triggered run would look busy forever, so its newest run is checked for a while too
	async function watchRun(id: string, attempt = 0) {
		await loadStrip(id);
		const latest = strips[id]?.at(-1);
		if (latest && latest.status !== 'running' && latest.status !== 'queued' && attempt > 0) {
			runningPipelines.delete(id);
			fetchPipelines();
			return;
		}
		if (attempt < 20 && runningPipelines.has(id)) setTimeout(() => watchRun(id, attempt + 1), 3000);
	}

	async function deletePipeline(e: CustomEvent) {
		const p = deleteTarget;
		if (!p) return;
		e.preventDefault();
		deleting = true;
		try {
			const url = `/ingestion/schedules/${encodeURIComponent(p.id)}${teardown ? '?teardown=true' : ''}`;
			const res = await fetchApi(url, { method: 'DELETE' });
			if (!res.ok) {
				const data = await res.json().catch(() => ({}));
				throw new Error(data.error || m.runs_delete_pipeline_error());
			}
			toasts.success(
				teardown
					? m.runs_delete_teardown_success({ name: p.name })
					: m.runs_delete_success({ name: p.name })
			);
			deleteOpen = false;
			fetchPipelines();
		} catch (err) {
			toasts.error(err instanceof Error ? err.message : m.runs_delete_pipeline_error());
		} finally {
			deleting = false;
		}
	}

	// Run events arrive over the websocket, so rows, strips and an open run update without a reload
	function handleJobRunEvent(event: JobRunEvent) {
		const payload = event.payload;
		const scheduleId = typeof payload.schedule_id === 'string' ? payload.schedule_id : undefined;
		const runStatus = typeof payload.status === 'string' ? payload.status : undefined;
		const runId = typeof payload.id === 'string' ? payload.id : undefined;

		if (scheduleId) {
			const started =
				event.type === 'job_run_started' ||
				event.type === 'job_run_claimed' ||
				(event.type === 'job_run_created' && runStatus === 'running');
			const ended =
				event.type === 'job_run_completed' ||
				event.type === 'job_run_cancelled' ||
				runStatus === 'succeeded' ||
				runStatus === 'failed' ||
				runStatus === 'cancelled';
			if (started) runningPipelines.add(scheduleId);
			if (ended) {
				const watched = runningPipelines.has(scheduleId);
				runningPipelines.delete(scheduleId);
				const finished =
					(typeof payload.finished_at === 'string' && payload.finished_at) ||
					(typeof payload.updated_at === 'string' && payload.updated_at) ||
					undefined;
				pipelines = pipelines.map((p) =>
					p.id === scheduleId
						? { ...p, last_run_status: runStatus, last_run_at: finished ?? p.last_run_at }
						: p
				);
				const name = pipelines.find((p) => p.id === scheduleId)?.name;
				if (name && watched) {
					if (runStatus === 'succeeded') toasts.success(m.plugins_run_finished({ name }));
					else if (runStatus === 'failed') toasts.error(m.plugins_run_failed({ name }));
				}
			}
			if (started || ended) loadStrip(scheduleId);
		}

		if (selectedRun && runId === selectedRun.id) openRun(runId);
	}

	onMount(() => {
		fetchPipelines();
		fetchPlugins();
		unsubscribe = websocketService.subscribe(handleJobRunEvent);
	});

	onDestroy(() => {
		unsubscribe?.();
		clearTimeout(searchTimer);
	});

	function showGallery() {
		document.getElementById('gallery')?.scrollIntoView({ behavior: 'smooth', block: 'start' });
	}
</script>

<svelte:head>
	<title>{m.pagetitle_runs()}</title>
</svelte:head>

<div class="mm-stack">
	<mm-page-header heading={m.runs_heading()}>
		{#if canManage}
			<mm-button
				slot="actions"
				variant="primary"
				icon="add"
				disabled={!$encryptionConfigured}
				onclick={showGallery}>{m.plugins_add()}</mm-button
			>
		{/if}
	</mm-page-header>

	{#if !$encryptionConfigured}
		<mm-callout tone="warning" heading={m.plugins_encryption_missing_heading()}>
			{m.plugins_encryption_missing_body()}
			<mm-button
				slot="actions"
				size="sm"
				variant="secondary"
				href="https://marmotdata.io/docs/Configure#server"
				external>{m.plugins_encryption_guide()}</mm-button
			>
		</mm-callout>
	{:else if $allowUnencrypted && total > 0}
		<mm-callout tone="warning" heading={m.runs_unencrypted_mode_heading()}>
			{m.runs_unencrypted_mode_hint()}
		</mm-callout>
	{/if}

	<div class="mm-stack" style="--mm-gap: var(--space-8)">
		<mm-section heading={m.plugins_pipelines_heading()}>
			{#if !loading && !loadError && total === 0 && !filtered}
				<mm-empty-state
					compact
					heading={m.plugins_none_connected_heading()}
					description={canManage ? m.plugins_none_connected() : m.runs_no_pipelines_hint_readonly()}
				></mm-empty-state>
			{:else}
				<div class="mm-stack" style="--mm-gap: var(--space-3)">
					<mm-toolbar label={m.plugins_filter_pipelines_label()}>
						<mm-search-field
							label={m.plugins_search_pipelines()}
							hide-label
							placeholder={m.plugins_search_pipelines()}
							shortcut="/"
							value={search}
							style="inline-size: min(18rem, 100%)"
							oninput={(e: Event) => onSearch((e.currentTarget as HTMLInputElement).value)}
						></mm-search-field>
						<mm-select
							label={m.common_status()}
							hide-label
							value={status}
							style="inline-size: 11rem"
							onmm-change={(e: CustomEvent<{ value: string }>) =>
								onStatus(e.detail.value as StatusFilter)}
						>
							<mm-option value="all">{m.plugins_all_statuses()}</mm-option>
							<mm-option value="failed">{m.runs_status_failed()}</mm-option>
							<mm-option value="running">{m.runs_status_running()}</mm-option>
							<mm-option value="succeeded">{m.runs_status_succeeded()}</mm-option>
							<mm-option value="never">{m.plugins_never_run()}</mm-option>
						</mm-select>
					</mm-toolbar>
					<mm-data-table
						label={m.plugins_pipelines_table_label()}
						row-key="id"
						{columns}
						{rows}
						{loading}
						error={loadError}
						sort={{ key: 'name', direction: 'ascending' }}
						empty-heading={m.plugins_no_match_heading()}
						empty-text={m.plugins_no_match_body()}
						onmm-retry={fetchPipelines}
					></mm-data-table>
					{#if total > PAGE_SIZE}
						<mm-pagination
							page={pageNumber}
							page-size={PAGE_SIZE}
							{total}
							page-sizes=""
							label={m.plugins_pipelines_pages()}
							onmm-page-change={(e: CustomEvent<{ page: number }>) => {
								pageNumber = e.detail.page;
								fetchPipelines();
							}}
						></mm-pagination>
					{/if}
				</div>
			{/if}
		</mm-section>

		<PluginGallery
			{plugins}
			connected={connectedCounts}
			canAdd={canManage && $encryptionConfigured}
			bind:queryOnly
		/>
	</div>
</div>

<RunDrawer
	run={selectedRun}
	open={drawerOpen}
	pluginName={selectedRun
		? pluginNames[pipelines.find((p) => p.id === selectedRun?.schedule_id)?.plugin_id ?? '']
		: undefined}
	onclose={() => (drawerOpen = false)}
/>

<mm-confirm-dialog
	open={deleteOpen}
	heading={m.plugins_delete_heading({ name: deleteTarget?.name ?? '' })}
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
