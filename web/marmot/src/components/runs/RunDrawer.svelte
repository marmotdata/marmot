<script lang="ts">
	import '@marmotdata/elements/drawer.js';
	import '@marmotdata/elements/timeline.js';
	import '@marmotdata/elements/code-sample.js';
	import { resolve } from '$app/paths';
	import { dump as dumpYaml } from 'js-yaml';
	import { fetchApi } from '$lib/api';
	import { m } from '$lib/paraglide/messages';
	import { formatNumber } from '$lib/utils/format';
	import {
		formatRunDuration,
		formatStamp,
		isRunning,
		pillState,
		runDurationMs,
		startedBy,
		type JobRun
	} from './pipeline';

	interface RunEntity {
		id: string;
		entity_type: string;
		entity_mrn: string;
		entity_name?: string;
		status: string;
		error_message?: string;
		created_at: string;
	}

	interface Props {
		run: JobRun | null;
		open: boolean;
		pluginName?: string;
		onclose: () => void;
	}

	let { run, open, pluginName, onclose }: Props = $props();

	// A run reads as what happened: the outcome and why, the phases it reached, what it changed and its log; machine details fold away
	const ENTITY_PAGE = 10;
	const LOG_TAIL = 200;

	let entities = $state<RunEntity[]>([]);
	let entitiesTotal = $state(0);
	let entitiesPage = $state(1);
	let entitiesError = $state('');
	let entitiesLoading = $state(false);

	let heading = $derived.by(() => {
		if (!run) return '';
		const time = formatStamp(run.created_at);
		if (!run.created_by || run.created_by === 'scheduler')
			return m.plugins_run_heading_scheduled({ time });
		if (run.created_by === 'system') return m.plugins_run_heading_refresh({ time });
		return m.plugins_run_heading_manual({ time });
	});

	let duration = $derived(run ? runDurationMs(run) : null);
	let logLines = $derived(run?.log ? run.log.replace(/\n+$/, '').split('\n') : []);
	let logTail = $derived(logLines.slice(-LOG_TAIL).join('\n'));
	let config = $derived(
		run?.config && Object.keys(run.config).length > 0
			? dumpYaml(run.config, { indent: 2, lineWidth: -1 })
			: ''
	);

	type PhaseState = 'done' | 'running' | 'pending' | 'failed';
	interface Phase {
		heading: string;
		time?: string;
		state: PhaseState;
		label?: string;
	}

	// The phases a run moves through; a failed or cancelled run marks the phase it stopped in. Runs from the scheduler record connecting, extracting and writing; older runs and CLI runs only know when they started and finished
	let phases = $derived.by<Phase[]>(() => {
		if (!run) return [];
		const live = isRunning(run.status);
		const stopped = run.status === 'failed' || run.status === 'cancelled';
		const stopLabel = run.status === 'failed' ? m.runs_status_failed() : m.runs_status_cancelled();
		const tracked =
			!!(run.connecting_at || run.extracting_at || run.writing_at) || (live && !!run.schedule_id);

		if (!tracked) {
			const list: Phase[] = [
				{ heading: m.plugins_phase_queued(), time: run.created_at, state: 'done' }
			];
			if (run.claimed_at)
				list.push({ heading: m.plugins_phase_picked_up(), time: run.claimed_at, state: 'done' });
			if (run.started_at) {
				list.push({ heading: m.plugins_phase_started(), time: run.started_at, state: 'done' });
			} else {
				list.push({
					heading: m.plugins_phase_started(),
					state: 'pending',
					label: live ? m.plugins_phase_waiting() : m.plugins_phase_not_reached()
				});
			}
			if (run.status === 'succeeded') {
				list.push({ heading: m.plugins_phase_finished(), time: run.finished_at, state: 'done' });
			} else if (stopped) {
				list.push({
					heading: m.plugins_phase_finished(),
					time: run.finished_at,
					state: 'failed',
					label: stopLabel
				});
			} else {
				list.push({
					heading: m.plugins_phase_finished(),
					state: run.status === 'running' ? 'running' : 'pending',
					label: run.status === 'running' ? m.runs_status_running() : m.plugins_phase_not_reached()
				});
			}
			return list;
		}

		const steps: { heading: string; time?: string }[] = [
			{ heading: m.plugins_phase_queued(), time: run.created_at },
			{ heading: m.plugins_phase_picked_up(), time: run.claimed_at ?? run.started_at },
			{ heading: m.plugins_phase_connecting(), time: run.connecting_at },
			{ heading: m.plugins_phase_extracting(), time: run.extracting_at },
			{ heading: m.plugins_phase_writing(), time: run.writing_at }
		];
		let reached = -1;
		steps.forEach((step, i) => {
			if (step.time) reached = i;
		});
		const list: Phase[] = steps.map((step, i) => {
			if (i > reached)
				return {
					heading: step.heading,
					state: 'pending',
					label: live ? m.plugins_phase_waiting() : m.plugins_phase_not_reached()
				};
			if (i === reached && stopped)
				return { heading: step.heading, time: step.time, state: 'failed', label: stopLabel };
			if (i === reached && live && i > 0)
				return {
					heading: step.heading,
					time: step.time,
					state: 'running',
					label: m.runs_status_running()
				};
			return { heading: step.heading, time: step.time, state: 'done' };
		});
		// A failed run ends at the phase it stopped in, so only a finished or unfinished run shows the last step
		if (run.status === 'succeeded')
			list.push({ heading: m.plugins_phase_finished(), time: run.finished_at, state: 'done' });
		else if (live)
			list.push({
				heading: m.plugins_phase_finished(),
				state: 'pending',
				label: m.plugins_phase_waiting()
			});
		return list;
	});

	const ENTITY_STATUS: Record<string, () => string> = {
		created: m.plugins_entity_added,
		updated: m.plugins_entity_changed,
		deleted: m.plugins_entity_removed,
		unchanged: m.plugins_entity_unchanged,
		failed: m.runs_status_failed
	};

	const ENTITY_TYPE: Record<string, () => string> = {
		asset: m.plugins_entity_type_asset,
		lineage: m.plugins_entity_type_lineage,
		documentation: m.plugins_entity_type_documentation
	};

	function assetHref(entity: RunEntity): string | undefined {
		if (entity.entity_type !== 'asset' || entity.status === 'deleted' || !entity.entity_mrn)
			return undefined;
		const parts = entity.entity_mrn.replace('mrn://', '').split('/');
		if (parts.length < 3) return undefined;
		return resolve('/discover/[type]/[service]/[name]', {
			type: parts[0],
			service: parts[1],
			name: parts.slice(2).join('/')
		});
	}

	async function loadEntities(id: string, pageNumber: number) {
		entitiesLoading = true;
		entitiesError = '';
		try {
			const params = new URLSearchParams({
				limit: String(ENTITY_PAGE),
				offset: String((pageNumber - 1) * ENTITY_PAGE)
			});
			const res = await fetchApi(`/ingestion/runs/${encodeURIComponent(id)}/entities?${params}`);
			if (!res.ok) throw new Error(m.runs_entities_load_error());
			const data = await res.json();
			entities = data.entities ?? [];
			entitiesTotal = data.total ?? 0;
		} catch (e) {
			entitiesError = e instanceof Error ? e.message : m.runs_entities_load_error();
		} finally {
			entitiesLoading = false;
		}
	}

	let loadedFor = '';
	$effect(() => {
		const id = run?.id;
		if (!open || !id || id === loadedFor) return;
		loadedFor = id;
		entitiesPage = 1;
		loadEntities(id, 1);
	});

	function downloadLog() {
		if (!run?.log) return;
		const url = URL.createObjectURL(new Blob([run.log], { type: 'text/plain' }));
		const a = document.createElement('a');
		a.href = url;
		a.download = `${run.id}.log`;
		a.click();
		URL.revokeObjectURL(url);
	}
</script>

<mm-drawer
	{open}
	size="lg"
	dismissible
	{heading}
	onmm-close={() => {
		loadedFor = '';
		onclose();
	}}
>
	{#if run}
		<div class="mm-stack" style="--mm-gap: var(--space-6)">
			<div class="mm-cluster" style="--mm-gap: var(--space-3)">
				<mm-status-pill state={pillState(run.status)} live={run.status === 'running'}
				></mm-status-pill>
				{#if run.status === 'failed' && run.error_message}
					<mm-text variant="dense">{run.error_message}</mm-text>
				{/if}
				{#if duration !== null}
					<mm-text variant="dense" tone="secondary" numeric>{formatRunDuration(duration)}</mm-text>
				{/if}
			</div>

			<mm-timeline orientation="horizontal" label={m.plugins_run_phases_label()}>
				{#each phases as phase (phase.heading)}
					<mm-timeline-item
						state={phase.state}
						heading={phase.heading}
						time={phase.time}
						state-label={phase.label}
					></mm-timeline-item>
				{/each}
			</mm-timeline>

			{#if run.status === 'succeeded' || run.status === 'failed'}
				<mm-description-list layout="inline-grid" columns={3}>
					<mm-description term={m.plugins_col_assets_added()}
						>{formatNumber(run.assets_created)}</mm-description
					>
					<mm-description term={m.plugins_col_assets_changed()}
						>{formatNumber(run.assets_updated)}</mm-description
					>
					<mm-description term={m.plugins_col_assets_removed()}
						>{formatNumber(run.assets_deleted)}</mm-description
					>
				</mm-description-list>
			{/if}

			{#if logTail}
				<mm-code-sample
					language="text"
					wrap
					label={m.plugins_run_log_label()}
					filename="{run.id}.log"
					code={logTail}
				></mm-code-sample>
			{/if}

			<mm-section heading={m.plugins_run_assets_heading()} level={3}>
				{#if entitiesError}
					<mm-callout tone="danger" compact heading={entitiesError}></mm-callout>
				{:else if entitiesLoading && entities.length === 0}
					<mm-skeleton count={3}></mm-skeleton>
				{:else if entities.length === 0}
					<mm-text variant="dense" tone="secondary">{m.plugins_run_no_assets()}</mm-text>
				{:else}
					<div class="mm-stack" style="--mm-gap: var(--space-3)">
						<mm-list dense aria-label={m.plugins_run_assets_heading()}>
							{#each entities as entity (entity.id)}
								<mm-list-item
									href={assetHref(entity)}
									secondary={ENTITY_TYPE[entity.entity_type]?.() ?? entity.entity_type}
								>
									{entity.entity_name || entity.entity_mrn}
									<mm-text
										slot="end"
										variant="caption"
										tone={entity.status === 'failed' ? 'danger' : 'secondary'}
										>{ENTITY_STATUS[entity.status]?.() ?? entity.status}</mm-text
									>
								</mm-list-item>
							{/each}
						</mm-list>
						{#if entitiesTotal > ENTITY_PAGE}
							<mm-pagination
								variant="compact"
								page={entitiesPage}
								page-size={ENTITY_PAGE}
								total={entitiesTotal}
								page-sizes=""
								label={m.plugins_run_assets_pages()}
								disabled={entitiesLoading}
								onmm-page-change={(e: CustomEvent<{ page: number }>) => {
									entitiesPage = e.detail.page;
									if (run) loadEntities(run.id, entitiesPage);
								}}
							></mm-pagination>
						{/if}
					</div>
				{/if}
			</mm-section>

			<mm-blade-section heading={m.plugins_more_details()} collapsible style="padding: 0">
				<div class="mm-stack" style="--mm-gap: var(--space-4)">
					<mm-description-list>
						<mm-description term={m.plugins_started_by()}
							>{startedBy(run.created_by)}</mm-description
						>
						{#if run.claimed_by}
							<mm-description term={m.plugins_worker()}>{run.claimed_by}</mm-description>
						{/if}
						{#if pluginName}
							<mm-description term={m.plugins_plugin_label()}>{pluginName}</mm-description>
						{/if}
						{#if run.plugin_run_id}
							<mm-description term={m.plugins_plugin_run_id()} mono
								>{run.plugin_run_id}</mm-description
							>
						{/if}
						<mm-description term={m.plugins_run_id()} mono>{run.id}</mm-description>
					</mm-description-list>
					{#if config}
						<mm-code-sample
							language="yaml"
							wrap
							label={m.runs_config_heading()}
							filename="config.yaml"
							code={config}
						></mm-code-sample>
					{/if}
				</div>
			</mm-blade-section>
		</div>
	{/if}
	{#if run?.log}
		<mm-button slot="footer" variant="secondary" icon="download" onclick={downloadLog}
			>{m.plugins_download_log()}</mm-button
		>
	{/if}
</mm-drawer>
