<script lang="ts">
	import '@marmotdata/elements/run-strip.js';
	import type { Run as StripRun } from '@marmotdata/elements/classes/run-strip.js';
	import { resolve } from '$app/paths';
	import { fetchApi } from '$lib/api';
	import { m } from '$lib/paraglide/messages';
	import { formatDateTime } from '$lib/utils/format';
	import { formatRunDuration } from './pipeline';

	interface AssetRef {
		type?: string;
		name?: string;
		mrn?: string;
		providers?: string[];
	}

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

	interface Props {
		assetId: string;
		minimal?: boolean;
		asset?: AssetRef | null;
	}

	let { assetId, minimal = false, asset = null }: Props = $props();

	// An asset's job runs from OpenLineage: a strip of recent outcomes on the full page, then the runs themselves; the blade shows only the last few
	const STRIP = 30;

	let runs = $state<RunHistoryEntry[]>([]);
	let strip = $state<StripRun[]>([]);
	let loading = $state(true);
	let error = $state('');
	let total = $state(0);
	let pageNumber = $state(1);

	let pageSize = $derived(minimal ? 5 : 10);

	let assetHref = $derived.by(() => {
		if (!asset) return undefined;
		const parts = (asset.mrn ?? '').replace('mrn://', '').split('/');
		const params =
			parts.length >= 3
				? { type: parts[0], service: parts[1], name: parts.slice(2).join('/') }
				: { type: asset.type ?? '', service: asset.providers?.[0] ?? '', name: asset.name ?? '' };
		if (!params.type || !params.service || !params.name) return undefined;
		return `${resolve('/discover/[type]/[service]/[name]', params)}?tab=run-history`;
	});

	function stripStatus(status: string): StripRun['status'] {
		switch (status.toUpperCase()) {
			case 'COMPLETE':
				return 'succeeded';
			case 'FAIL':
				return 'failed';
			case 'RUNNING':
			case 'START':
				return 'running';
			default:
				return 'skipped';
		}
	}

	function pill(status: string): 'failed' | 'running' | 'cancelled' | null {
		switch (status.toUpperCase()) {
			case 'FAIL':
				return 'failed';
			case 'RUNNING':
			case 'START':
				return 'running';
			case 'ABORT':
				return 'cancelled';
			default:
				return null;
		}
	}

	async function fetchRuns(id: string, pageNo: number, size: number) {
		loading = true;
		error = '';
		try {
			const offset = (pageNo - 1) * size;
			const res = await fetchApi(`/assets/run-history/${id}?limit=${size}&offset=${offset}`);
			if (!res.ok) throw new Error(m.runs_history_fetch_error());
			const data = await res.json();
			runs = data.run_history ?? [];
			total = data.total ?? 0;
		} catch (e) {
			error = e instanceof Error ? e.message : m.runs_history_load_error();
		} finally {
			loading = false;
		}
	}

	async function fetchStrip(id: string) {
		try {
			const res = await fetchApi(`/assets/run-history/${id}?limit=${STRIP}&offset=0`);
			if (!res.ok) return;
			const list: RunHistoryEntry[] = (await res.json()).run_history ?? [];
			strip = list
				.map((r) => ({
					id: r.run_id || r.id,
					status: stripStatus(r.status),
					startedAt: r.start_time ?? r.event_time,
					durationMs: r.duration_ms || undefined
				}))
				.reverse();
		} catch {
			// The strip is a summary; the list still shows
		}
	}

	$effect(() => {
		if (!assetId) return;
		pageNumber = 1;
		fetchRuns(assetId, 1, pageSize);
		if (!minimal) fetchStrip(assetId);
	});
</script>

<div class="mm-stack" style="--mm-gap: var(--space-4)">
	{#if !minimal && strip.length > 0}
		<mm-run-strip runs={strip} label={m.runs_history_strip_label()}></mm-run-strip>
	{/if}

	{#if error}
		<mm-callout tone="danger" compact heading={error}></mm-callout>
	{:else if loading && runs.length === 0}
		<mm-skeleton count={minimal ? 3 : 5}></mm-skeleton>
	{:else if runs.length === 0}
		<mm-empty-state
			compact
			heading={m.runs_history_empty_heading()}
			description={m.runs_history_empty_hint()}
		></mm-empty-state>
	{:else}
		<mm-list dense={minimal} aria-label={m.runs_history_recent_heading()}>
			{#each runs as run (run.id)}
				{@const state = pill(run.status)}
				<mm-list-item
					secondary={[run.start_time ? formatDateTime(run.start_time) : '', run.job_namespace]
						.filter(Boolean)
						.join(' · ')}
				>
					{run.job_name}
					<span slot="end" class="mm-cluster" style="--mm-gap: var(--space-2); flex-wrap: nowrap">
						{#if state}
							<mm-status-pill {state} live={state === 'running'}></mm-status-pill>
						{/if}
						{#if run.duration_ms}
							<mm-text variant="caption" tone="secondary" numeric
								>{formatRunDuration(run.duration_ms)}</mm-text
							>
						{/if}
					</span>
				</mm-list-item>
			{/each}
		</mm-list>
		{#if minimal && assetHref && total > pageSize}
			<mm-link href={assetHref}>{m.runs_history_view_all()}</mm-link>
		{:else if !minimal && total > pageSize}
			<mm-pagination
				page={pageNumber}
				page-size={pageSize}
				{total}
				page-sizes=""
				label={m.runs_history_pages()}
				disabled={loading}
				onmm-page-change={(e: CustomEvent<{ page: number }>) => {
					pageNumber = e.detail.page;
					fetchRuns(assetId, pageNumber, pageSize);
				}}
			></mm-pagination>
		{/if}
	{/if}
</div>
