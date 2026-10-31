<script lang="ts">
	import { fetchApi } from '$lib/api';
	import type { Asset } from '$lib/assets/types';
	import { m } from '$lib/paraglide/messages';
	import { getLocale } from '$lib/paraglide/runtime';
	import { formatMs } from './assetFormat';
	import '@marmotdata/elements/run-strip.js';
	import '@marmotdata/elements/line-chart.js';
	import type { Run } from '@marmotdata/elements/classes/run-strip.js';
	import type { DataTableColumn } from '@marmotdata/elements/classes/data-table.js';

	// The Run history tab: the last 30 runs as a strip and a build-time line, then every run in a paged table.
	let { asset }: { asset: Asset } = $props();

	interface RunHistoryEntry {
		id: string;
		run_id: string;
		job_name: string;
		job_namespace: string;
		status: string;
		start_time?: string;
		end_time?: string;
		duration_ms?: number;
		type: string;
		event_time: string;
	}

	interface Row {
		id: string;
		run: string;
		status: string;
		job: string;
		started: string;
		duration: number | null;
		type: string;
	}

	const PAGE_SIZE = 25;
	const STRIP_SIZE = 30;

	let recent = $state<RunHistoryEntry[]>([]);
	let pageRuns = $state<RunHistoryEntry[]>([]);
	let total = $state(0);
	let currentPage = $state(1);
	let loading = $state(true);
	let error = $state('');

	function status(s: string): Run['status'] {
		switch (s.toUpperCase()) {
			case 'COMPLETE':
				return 'succeeded';
			case 'FAIL':
				return 'failed';
			case 'RUNNING':
				return 'running';
			case 'ABORT':
				return 'skipped';
			default:
				return 'queued';
		}
	}

	let strip = $derived<Run[]>(
		[...recent].reverse().map((r) => ({
			id: r.run_id,
			status: status(r.status),
			startedAt: r.start_time || r.event_time,
			durationMs: r.duration_ms
		}))
	);

	let timed = $derived(
		strip.filter((r) => r.status === 'succeeded' && r.durationMs != null && r.durationMs > 0)
	);

	let rows = $derived<Row[]>(
		pageRuns.map((r) => ({
			id: r.id,
			run: r.run_id.slice(0, 12),
			status: status(r.status),
			job: r.job_namespace ? `${r.job_namespace} / ${r.job_name}` : r.job_name,
			started: r.start_time || r.event_time,
			duration: r.duration_ms && r.duration_ms > 0 ? r.duration_ms : null,
			type: r.type ? r.type.charAt(0) + r.type.slice(1).toLowerCase() : ''
		}))
	);

	const columns: DataTableColumn<Row>[] = [
		{ key: 'run', header: m.asset_page_runs_col_run(), type: 'mono', width: 140 },
		{ key: 'status', header: m.asset_page_runs_col_status(), type: 'status', width: 120 },
		{ key: 'job', header: m.asset_page_runs_col_job() },
		{
			key: 'started',
			header: m.asset_page_runs_col_started(),
			width: 180,
			value: (r) =>
				new Date(r.started).toLocaleString(getLocale(), {
					month: 'short',
					day: 'numeric',
					hour: '2-digit',
					minute: '2-digit'
				})
		},
		{
			key: 'duration',
			header: m.asset_page_runs_col_duration(),
			type: 'number',
			width: 110,
			value: (r) => (r.duration == null ? '' : formatMs(r.duration)),
			empty: ''
		},
		{ key: 'type', header: m.asset_page_runs_col_type(), width: 110 }
	];

	async function fetchPage(offset: number, limit: number) {
		const response = await fetchApi(
			`/assets/run-history/${asset.id}?limit=${limit}&offset=${offset}`
		);
		if (!response.ok) throw new Error(m.runs_history_fetch_error());
		return (await response.json()) as { run_history: RunHistoryEntry[]; total: number };
	}

	async function load() {
		loading = true;
		error = '';
		try {
			const [stripData, pageData] = await Promise.all([
				fetchPage(0, STRIP_SIZE),
				fetchPage((currentPage - 1) * PAGE_SIZE, PAGE_SIZE)
			]);
			recent = stripData.run_history ?? [];
			pageRuns = pageData.run_history ?? [];
			total = pageData.total ?? 0;
		} catch (e) {
			error = e instanceof Error ? e.message : m.runs_history_load_error();
		} finally {
			loading = false;
		}
	}

	async function goTo(p: number) {
		currentPage = p;
		loading = true;
		try {
			const data = await fetchPage((p - 1) * PAGE_SIZE, PAGE_SIZE);
			pageRuns = data.run_history ?? [];
			total = data.total ?? 0;
		} catch (e) {
			error = e instanceof Error ? e.message : m.runs_history_load_error();
		} finally {
			loading = false;
		}
	}

	$effect(() => {
		if (asset.id) {
			currentPage = 1;
			load();
		}
	});
</script>

{#if !loading && !error && total === 0}
	<mm-empty-state
		heading={m.asset_page_runs_empty_heading()}
		description={m.asset_page_runs_empty_body()}
		seed={5}
	></mm-empty-state>
{:else}
	<div class="mm-stack" style="--mm-gap: var(--space-6)">
		{#if strip.length > 0}
			<mm-run-strip label={m.asset_page_runs_strip_label({ name: asset.name })} runs={strip}
			></mm-run-strip>
		{/if}
		{#if timed.length > 1}
			<mm-line-chart
				label={m.asset_page_runs_chart_label()}
				x={timed.map((r) => r.startedAt)}
				series={[
					{
						name: m.asset_page_runs_chart_series(),
						values: timed.map((r) => Math.round((r.durationMs ?? 0) / 100) / 10),
						key: true
					}
				]}
				unit="s"
				tone="info"
				height={200}
			></mm-line-chart>
		{/if}
		<div class="mm-stack" style="--mm-gap: var(--space-3)">
			<mm-data-table
				label={m.asset_page_runs_table_label({ name: asset.name })}
				density="compact"
				row-key="id"
				{columns}
				{rows}
				{loading}
				error={error || undefined}
				onmm-retry={load}
			></mm-data-table>
			{#if total > PAGE_SIZE}
				<mm-pagination
					page={currentPage}
					page-size={PAGE_SIZE}
					{total}
					page-sizes=""
					disabled={loading}
					onmm-page-change={(e: CustomEvent<{ page: number }>) => goTo(e.detail.page)}
				></mm-pagination>
			{/if}
		</div>
	</div>
{/if}
