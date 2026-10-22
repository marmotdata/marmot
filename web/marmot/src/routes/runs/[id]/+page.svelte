<script lang="ts">
	import { onDestroy, onMount, type ComponentProps } from 'svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import IconifyIcon from '@iconify/svelte';
	import cronstrue from 'cronstrue/i18n';
	import Icon from '$components/ui/Icon.svelte';
	import Sparkline from '$components/charts/Sparkline.svelte';
	import ConfirmModal from '$components/ui/ConfirmModal.svelte';
	import IngestionRunModal from '$components/runs/IngestionRunModal.svelte';
	import { fetchApi } from '$lib/api';
	import { auth } from '$lib/stores/auth';
	import { encryptionConfigured } from '$lib/stores/encryption';
	import { toasts } from '$lib/stores/toast';
	import { websocketService, type JobRunEvent } from '$lib/websocket';
	import { m } from '$lib/paraglide/messages';
	import { getLocale } from '$lib/paraglide/runtime';
	import {
		formatDateTime,
		formatDuration,
		formatNumber,
		formatRelativeTime
	} from '$lib/utils/format';

	interface Pipeline {
		id: string;
		name: string;
		plugin_id: string;
		config: Record<string, unknown>;
		cron_expression: string;
		enabled: boolean;
		queryable?: boolean;
		last_run_at?: string;
		next_run_at?: string;
		managed_by?: string;
		created_at: string;
	}

	interface SpecField {
		name: string;
		label: string;
		type: string;
		sensitive?: boolean;
		hidden?: boolean;
		group?: string;
	}

	interface Plugin {
		id: string;
		name: string;
		config_spec?: SpecField[];
	}

	interface Run {
		id: string;
		status: 'pending' | 'claimed' | 'running' | 'succeeded' | 'failed' | 'cancelled';
		started_at?: string;
		finished_at?: string;
		created_at: string;
		error_message?: string;
		assets_created: number;
		assets_updated: number;
		assets_deleted: number;
	}

	type ModalRun = ComponentProps<typeof IngestionRunModal>['run'];
	type StatusFilter = '' | 'succeeded' | 'failed' | 'running';

	// One connected source: how it has been doing, every run it has made and how it is set up
	const PAGE = 20;
	const BASE_FIELDS = new Set(['tags', 'external_links', 'filter']);
	const canManage = auth.hasPermission('ingestion', 'manage');
	const id = page.params.id ?? '';

	let pipeline = $state<Pipeline | null>(null);
	let plugin = $state<Plugin | null>(null);
	let loadError = $state('');
	let runs = $state<Run[]>([]);
	let runsTotal = $state(0);
	let offset = $state(0);
	let status = $state<StatusFilter>('');
	let recent = $state<Run[]>([]);
	let running = $state(false);
	let showAllSettings = $state(false);
	let menuOpen = $state(false);
	let selectedRun = $state<ModalRun | null>(null);
	let showRun = $state(false);
	let confirmDelete = $state(false);
	let teardown = $state(false);
	let unsubscribe: (() => void) | null = null;
	let refreshTimer: ReturnType<typeof setTimeout> | undefined;

	let managed = $derived(!!pipeline?.managed_by);
	let schedule = $derived.by(() => {
		if (!pipeline?.cron_expression) return m.runs_schedule_manual();
		try {
			return cronstrue.toString(pipeline.cron_expression, { locale: getLocale() });
		} catch {
			return pipeline.cron_expression;
		}
	});

	// The last 30 days of runs drive the headline numbers
	let window30 = $derived(
		recent.filter((r) => Date.now() - new Date(r.created_at).getTime() < 30 * 86_400_000)
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
			.filter((r) => r.started_at && r.finished_at)
			.map((r) => ({
				label: formatRelativeTime(r.created_at),
				value: (new Date(r.finished_at!).getTime() - new Date(r.started_at!).getTime()) / 1000
			}))
	);
	let avgDuration = $derived(
		durations.length === 0
			? null
			: (durations.reduce((n, d) => n + d.value, 0) / durations.length) * 1000
	);
	let latest = $derived(recent[0]);

	// Settings shown at a glance: the ungrouped fields that are set, then the rest behind a toggle
	let settings = $derived.by(() => {
		const spec = (plugin?.config_spec ?? []).filter(
			(f) => !BASE_FIELDS.has(f.name) && !f.hidden && f.type !== 'object'
		);
		const config = pipeline?.config ?? {};
		const rows = spec
			.filter((f) => {
				const v = config[f.name];
				return v !== undefined && v !== null && v !== '' && !(Array.isArray(v) && v.length === 0);
			})
			.map((f) => ({
				label: f.label,
				group: f.group,
				value: f.sensitive || f.type === 'password' ? '••••••••' : display(config[f.name])
			}));
		return { main: rows.filter((r) => !r.group), extra: rows.filter((r) => r.group) };
	});

	function display(v: unknown): string {
		if (typeof v === 'boolean') return v ? m.common_yes() : m.common_no();
		if (Array.isArray(v)) return v.join(', ');
		return String(v);
	}

	function durationOf(r: Run): string {
		if (!r.started_at || !r.finished_at) return '—';
		return formatDuration(new Date(r.finished_at).getTime() - new Date(r.started_at).getTime());
	}

	const STATUS: Record<Run['status'], { label: () => string; dot: string; text: string }> = {
		succeeded: {
			label: m.runs_status_succeeded,
			dot: 'bg-earthy-green-500',
			text: 'text-earthy-green-800 dark:text-earthy-green-300'
		},
		failed: {
			label: m.runs_status_failed,
			dot: 'bg-red-500',
			text: 'text-red-700 dark:text-red-300'
		},
		running: {
			label: m.runs_status_running,
			dot: 'bg-earthy-blue-500 animate-pulse',
			text: 'text-earthy-blue-800 dark:text-earthy-blue-300'
		},
		claimed: {
			label: m.runs_status_claimed,
			dot: 'bg-earthy-blue-500 animate-pulse',
			text: 'text-earthy-blue-800 dark:text-earthy-blue-300'
		},
		pending: {
			label: m.runs_status_pending,
			dot: 'bg-gray-400',
			text: 'text-gray-600 dark:text-gray-300'
		},
		cancelled: {
			label: m.runs_status_cancelled,
			dot: 'bg-gray-300 dark:bg-gray-600',
			text: 'text-gray-500 dark:text-gray-400'
		}
	};

	const FILTERS: { id: StatusFilter; label: () => string }[] = [
		{ id: '', label: m.plugins_filter_all },
		{ id: 'succeeded', label: m.runs_status_succeeded },
		{ id: 'failed', label: m.runs_status_failed },
		{ id: 'running', label: m.runs_status_running }
	];

	async function loadPipeline() {
		try {
			const res = await fetchApi(`/ingestion/schedules/${encodeURIComponent(id)}`);
			if (!res.ok) throw new Error(m.plugins_detail_load_error());
			pipeline = await res.json();
			const plugins = await fetchApi('/plugins');
			if (plugins.ok) {
				const list: Plugin[] = (await plugins.json()).plugins ?? [];
				plugin = list.find((p) => p.id === pipeline?.plugin_id) ?? null;
			}
		} catch (e) {
			loadError = e instanceof Error ? e.message : m.plugins_detail_load_error();
		}
	}

	async function loadRuns() {
		const params = `schedule_id=${encodeURIComponent(id)}&limit=${PAGE}&offset=${offset}${status ? `&status=${status}` : ''}`;
		try {
			const res = await fetchApi(`/ingestion/runs?${params}`);
			if (!res.ok) return;
			const data = await res.json();
			runs = data.runs ?? [];
			runsTotal = data.total ?? 0;
		} catch {
			// The list keeps what it had
		}
	}

	async function loadRecent() {
		try {
			const res = await fetchApi(`/ingestion/runs?schedule_id=${encodeURIComponent(id)}&limit=200`);
			if (!res.ok) return;
			recent = (await res.json()).runs ?? [];
			running = recent.some((r) => r.status === 'running' || r.status === 'claimed');
		} catch {
			// Stats stay as they were
		}
	}

	function setFilter(next: StatusFilter) {
		status = next;
		offset = 0;
		loadRuns();
	}

	function pageBy(delta: number) {
		offset = Math.max(0, offset + delta * PAGE);
		loadRuns();
	}

	async function runNow() {
		if (!pipeline) return;
		running = true;
		try {
			const res = await fetchApi(`/ingestion/schedules/${encodeURIComponent(id)}/trigger`, {
				method: 'POST'
			});
			if (!res.ok) throw new Error((await res.json()).error || m.runs_trigger_error());
			scheduleRefresh();
		} catch (e) {
			running = false;
			toasts.error(e instanceof Error ? e.message : m.runs_trigger_error());
		}
	}

	async function deletePipeline(withTeardown?: boolean) {
		confirmDelete = false;
		if (!pipeline) return;
		try {
			const res = await fetchApi(
				`/ingestion/schedules/${encodeURIComponent(id)}${withTeardown ? '?teardown=true' : ''}`,
				{ method: 'DELETE' }
			);
			if (!res.ok) throw new Error((await res.json()).error || m.runs_delete_pipeline_error());
			toasts.success(
				withTeardown
					? m.runs_delete_teardown_success({ name: pipeline.name })
					: m.runs_delete_success({ name: pipeline.name })
			);
			goto(resolve('/runs'));
		} catch (e) {
			toasts.error(e instanceof Error ? e.message : m.runs_delete_pipeline_error());
		}
	}

	function openRun(run: Run) {
		selectedRun = run as unknown as ModalRun;
		showRun = true;
	}

	// Live updates for this pipeline's runs arrive over the websocket; a short debounce folds bursts of progress events into one reload
	function scheduleRefresh() {
		clearTimeout(refreshTimer);
		refreshTimer = setTimeout(() => {
			loadRuns();
			loadRecent();
		}, 400);
	}

	function onRunEvent(event: JobRunEvent) {
		if (event.payload.schedule_id !== id) return;
		if (event.type === 'job_run_started' || event.type === 'job_run_claimed') running = true;
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

<svelte:window onclick={() => (menuOpen = false)} />

{#snippet stat(label: string, value: string, hint?: string)}
	<div class="rounded-xl border border-gray-200 bg-white p-4 dark:border-gray-700 dark:bg-gray-800">
		<p class="text-xs font-medium text-gray-500 dark:text-gray-400">{label}</p>
		<p class="mt-1 text-2xl font-semibold tabular-nums text-gray-900 dark:text-gray-100">{value}</p>
		{#if hint}
			<p class="mt-0.5 truncate text-xs text-gray-500 dark:text-gray-400">{hint}</p>
		{/if}
	</div>
{/snippet}

<div class="w-full px-4 py-6 sm:px-6 lg:px-8">
	<a
		href={resolve('/runs')}
		class="inline-flex items-center gap-1 text-sm text-gray-500 hover:text-gray-900 dark:text-gray-400 dark:hover:text-gray-100"
	>
		<IconifyIcon icon="material-symbols:arrow-back" class="h-4 w-4" />
		{m.runs_heading()}
	</a>

	{#if loadError}
		<p
			class="mt-6 rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700 dark:border-red-800 dark:bg-red-900/20 dark:text-red-200"
		>
			{loadError}
		</p>
	{:else if !pipeline}
		<div class="mt-6 space-y-4">
			<div class="h-14 w-80 animate-pulse rounded-xl bg-gray-100 dark:bg-gray-800"></div>
			<div class="grid grid-cols-2 gap-4 lg:grid-cols-4">
				{#each Array(4) as _, i (i)}
					<div class="h-24 animate-pulse rounded-xl bg-gray-100 dark:bg-gray-800"></div>
				{/each}
			</div>
		</div>
	{:else}
		<!-- Header -->
		<div class="mt-4 flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
			<div class="flex min-w-0 items-center gap-4">
				<span
					class="flex h-12 w-12 flex-shrink-0 items-center justify-center rounded-xl border border-gray-200 bg-white dark:border-gray-700 dark:bg-gray-800"
				>
					<Icon name={pipeline.plugin_id} size="sm" showLabel={false} />
				</span>
				<div class="min-w-0">
					<div class="flex flex-wrap items-center gap-2">
						<h1
							class="truncate text-2xl font-semibold tracking-tight text-gray-900 dark:text-gray-100"
						>
							{pipeline.name}
						</h1>
						{#if pipeline.queryable}
							<span
								class="inline-flex items-center gap-0.5 rounded-full bg-earthy-terracotta-100 px-2 py-0.5 text-xs font-medium text-earthy-terracotta-800 dark:bg-earthy-terracotta-900/40 dark:text-earthy-terracotta-200"
							>
								<IconifyIcon icon="material-symbols:bolt" class="h-3.5 w-3.5" />
								{m.runs_queryable_badge()}
							</span>
						{/if}
						{#if managed}
							<span
								class="inline-flex items-center gap-0.5 rounded-full bg-earthy-blue-100 px-2 py-0.5 text-xs font-medium text-earthy-blue-800 dark:bg-earthy-blue-900/40 dark:text-earthy-blue-200"
								title={m.runs_operator_managed_title()}
							>
								<IconifyIcon icon="mdi:kubernetes" class="h-3.5 w-3.5" />
								{m.runs_operator_badge()}
							</span>
						{/if}
					</div>
					<p class="mt-0.5 flex items-center gap-1.5 text-sm text-gray-500 dark:text-gray-400">
						{plugin?.name ?? pipeline.plugin_id}
						<span aria-hidden="true">·</span>
						<IconifyIcon icon="material-symbols:schedule-outline" class="h-4 w-4" />
						<span title={pipeline.cron_expression || undefined}>{schedule}</span>
					</p>
				</div>
			</div>

			{#if canManage && !managed}
				<div class="flex flex-shrink-0 items-center gap-2">
					<button
						type="button"
						onclick={runNow}
						disabled={running || !$encryptionConfigured}
						class="inline-flex items-center gap-2 rounded-lg bg-earthy-terracotta-600 px-4 py-2 text-sm font-semibold text-white shadow-sm transition hover:bg-earthy-terracotta-500 disabled:cursor-not-allowed disabled:opacity-50"
					>
						<IconifyIcon
							icon={running
								? 'material-symbols:progress-activity'
								: 'material-symbols:play-arrow-rounded'}
							class="h-5 w-5 {running ? 'animate-spin' : ''}"
						/>
						{running ? m.plugins_running() : m.plugins_run_now()}
					</button>
					<a
						href={resolve('/pipelines/[id]/edit', { id })}
						class="inline-flex items-center gap-1.5 rounded-lg border border-gray-200 bg-white px-3 py-2 text-sm font-medium text-gray-700 transition hover:bg-gray-50 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-200 dark:hover:bg-gray-700"
					>
						<IconifyIcon icon="material-symbols:edit-outline" class="h-4 w-4" />
						{m.common_edit()}
					</a>
					<div class="relative">
						<button
							type="button"
							onclick={(e) => {
								e.stopPropagation();
								menuOpen = !menuOpen;
							}}
							aria-haspopup="menu"
							aria-expanded={menuOpen}
							aria-label={m.plugins_more_actions()}
							class="rounded-lg border border-gray-200 bg-white p-2 text-gray-500 transition hover:bg-gray-50 hover:text-gray-800 dark:border-gray-700 dark:bg-gray-800 dark:hover:bg-gray-700 dark:hover:text-gray-200"
						>
							<IconifyIcon icon="material-symbols:more-horiz" class="h-5 w-5" />
						</button>
						{#if menuOpen}
							<div
								class="absolute right-0 z-20 mt-1 w-44 overflow-hidden rounded-lg border border-gray-200 bg-white py-1 shadow-lg dark:border-gray-700 dark:bg-gray-800"
								role="menu"
							>
								<button
									type="button"
									role="menuitem"
									onclick={() => {
										teardown = false;
										confirmDelete = true;
									}}
									class="flex w-full items-center gap-2 px-3 py-2 text-left text-sm text-red-600 hover:bg-red-50 dark:text-red-400 dark:hover:bg-red-900/20"
								>
									<IconifyIcon icon="material-symbols:delete-outline" class="h-4 w-4" />
									{m.common_delete()}
								</button>
							</div>
						{/if}
					</div>
				</div>
			{/if}
		</div>

		<!-- Headline numbers over the last 30 days -->
		<div class="mt-6 grid grid-cols-2 gap-4 lg:grid-cols-4">
			{@render stat(
				m.plugins_stat_success(),
				successRate === null ? '—' : `${successRate}%`,
				m.plugins_stat_runs({ count: window30.length })
			)}
			<div
				class="rounded-xl border border-gray-200 bg-white p-4 dark:border-gray-700 dark:bg-gray-800"
			>
				<p class="text-xs font-medium text-gray-500 dark:text-gray-400">
					{m.plugins_stat_duration()}
				</p>
				<p class="mt-1 text-2xl font-semibold tabular-nums text-gray-900 dark:text-gray-100">
					{avgDuration === null ? '—' : formatDuration(avgDuration)}
				</p>
				{#if durations.length >= 6}
					<div class="mt-2"><Sparkline points={durations} height={28} /></div>
				{/if}
			</div>
			{@render stat(
				m.plugins_stat_last_run(),
				latest ? formatRelativeTime(latest.created_at) : '—',
				latest
					? m.plugins_run_counts({
							created: formatNumber(latest.assets_created),
							updated: formatNumber(latest.assets_updated)
						})
					: m.plugins_never_run()
			)}
			{@render stat(
				m.plugins_stat_next_run(),
				pipeline.next_run_at ? formatRelativeTime(pipeline.next_run_at) : '—',
				pipeline.next_run_at ? formatDateTime(pipeline.next_run_at) : m.runs_schedule_manual()
			)}
		</div>

		<div class="mt-6 grid gap-6 lg:grid-cols-[minmax(0,1fr)_20rem]">
			<!-- Runs -->
			<section
				class="overflow-hidden rounded-xl border border-gray-200 bg-white dark:border-gray-700 dark:bg-gray-800"
			>
				<div
					class="flex flex-wrap items-center justify-between gap-3 border-b border-gray-100 px-5 py-3 dark:border-gray-700"
				>
					<h2 class="text-sm font-semibold text-gray-900 dark:text-gray-100">
						{m.plugins_runs_heading()}
						<span class="ml-1 font-normal tabular-nums text-gray-400"
							>{formatNumber(runsTotal)}</span
						>
					</h2>
					<div class="flex flex-wrap gap-1.5" role="group">
						{#each FILTERS as f (f.id)}
							<button
								type="button"
								onclick={() => setFilter(f.id)}
								aria-pressed={status === f.id}
								class="rounded-full border px-2.5 py-0.5 text-xs font-medium transition {status ===
								f.id
									? 'border-gray-900 bg-gray-900 text-white dark:border-gray-100 dark:bg-gray-100 dark:text-gray-900'
									: 'border-gray-200 text-gray-600 hover:border-gray-300 dark:border-gray-700 dark:text-gray-300'}"
								>{f.label()}</button
							>
						{/each}
					</div>
				</div>

				{#if runs.length === 0}
					<p class="px-5 py-12 text-center text-sm text-gray-500 dark:text-gray-400">
						{status ? m.runs_no_runs_filtered_hint() : m.plugins_no_runs()}
					</p>
				{:else}
					<table class="w-full text-sm">
						<thead
							class="text-left text-[11px] font-semibold uppercase tracking-wider text-gray-500 dark:text-gray-400"
						>
							<tr class="border-b border-gray-100 dark:border-gray-700">
								<th class="px-5 py-2.5">{m.common_status()}</th>
								<th class="px-3 py-2.5">{m.plugins_col_started()}</th>
								<th class="px-3 py-2.5">{m.plugins_col_duration()}</th>
								<th class="px-5 py-2.5 text-right">{m.plugins_col_assets()}</th>
							</tr>
						</thead>
						<tbody class="divide-y divide-gray-100 dark:divide-gray-700/60">
							{#each runs as run (run.id)}
								{@const s = STATUS[run.status] ?? STATUS.pending}
								<tr
									class="cursor-pointer transition hover:bg-gray-50 dark:hover:bg-gray-700/30"
									onclick={() => openRun(run)}
								>
									<td class="px-5 py-3 align-top">
										<span class="inline-flex items-center gap-2 font-medium {s.text}">
											<span class="h-2 w-2 rounded-full {s.dot}"></span>
											{s.label()}
										</span>
										{#if run.status === 'failed' && run.error_message}
											<p
												class="mt-1 line-clamp-2 max-w-xl font-mono text-xs text-red-700 dark:text-red-300"
											>
												{run.error_message}
											</p>
										{/if}
									</td>
									<td
										class="px-3 py-3 align-top text-gray-700 dark:text-gray-300"
										title={formatDateTime(run.created_at)}
									>
										{formatRelativeTime(run.created_at)}
									</td>
									<td class="px-3 py-3 align-top tabular-nums text-gray-700 dark:text-gray-300">
										{durationOf(run)}
									</td>
									<td class="px-5 py-3 text-right align-top tabular-nums">
										<span class="text-earthy-green-700 dark:text-earthy-green-300"
											>+{formatNumber(run.assets_created)}</span
										>
										<span class="ml-2 text-gray-500">~{formatNumber(run.assets_updated)}</span>
										{#if run.assets_deleted > 0}
											<span class="ml-2 text-red-600 dark:text-red-400"
												>−{formatNumber(run.assets_deleted)}</span
											>
										{/if}
									</td>
								</tr>
							{/each}
						</tbody>
					</table>
					{#if runsTotal > PAGE}
						<div
							class="flex items-center justify-between border-t border-gray-100 px-5 py-2.5 text-sm text-gray-500 dark:border-gray-700 dark:text-gray-400"
						>
							<span class="tabular-nums"
								>{m.plugins_runs_range({
									from: offset + 1,
									to: Math.min(offset + PAGE, runsTotal),
									total: runsTotal
								})}</span
							>
							<div class="flex gap-1">
								<button
									type="button"
									onclick={() => pageBy(-1)}
									disabled={offset === 0}
									aria-label={m.common_previous()}
									class="rounded-md p-1.5 hover:bg-gray-100 disabled:opacity-40 dark:hover:bg-gray-700"
								>
									<IconifyIcon icon="material-symbols:chevron-left" class="h-5 w-5" />
								</button>
								<button
									type="button"
									onclick={() => pageBy(1)}
									disabled={offset + PAGE >= runsTotal}
									aria-label={m.common_next()}
									class="rounded-md p-1.5 hover:bg-gray-100 disabled:opacity-40 dark:hover:bg-gray-700"
								>
									<IconifyIcon icon="material-symbols:chevron-right" class="h-5 w-5" />
								</button>
							</div>
						</div>
					{/if}
				{/if}
			</section>

			<!-- Setup at a glance -->
			<aside class="space-y-4">
				{#if pipeline.queryable}
					<a
						href={resolve(
							`/query?source=${encodeURIComponent(pipeline.name)}` as `/query?${string}`
						)}
						class="flex items-center gap-3 rounded-xl border border-earthy-terracotta-200 bg-earthy-terracotta-50/60 p-4 transition hover:border-earthy-terracotta-300 dark:border-earthy-terracotta-800 dark:bg-earthy-terracotta-900/10"
					>
						<span
							class="flex h-9 w-9 items-center justify-center rounded-lg bg-earthy-terracotta-600 text-white"
						>
							<IconifyIcon icon="material-symbols:bolt" class="h-5 w-5" />
						</span>
						<span class="min-w-0 flex-1">
							<span class="block text-sm font-semibold text-gray-900 dark:text-gray-100"
								>{m.plugins_query_card_title()}</span
							>
							<span class="block text-xs text-gray-600 dark:text-gray-400"
								>{m.plugins_query_card_body()}</span
							>
						</span>
						<IconifyIcon icon="material-symbols:arrow-forward" class="h-4 w-4 text-gray-400" />
					</a>
				{/if}

				<section
					class="rounded-xl border border-gray-200 bg-white p-5 dark:border-gray-700 dark:bg-gray-800"
				>
					<h2 class="text-sm font-semibold text-gray-900 dark:text-gray-100">
						{m.plugins_settings_heading()}
					</h2>
					<dl class="mt-3 space-y-2.5 text-sm">
						{#each settings.main as row (row.label)}
							<div class="flex justify-between gap-3">
								<dt class="text-gray-500 dark:text-gray-400">{row.label}</dt>
								<dd class="truncate text-right font-medium text-gray-800 dark:text-gray-200">
									{row.value}
								</dd>
							</div>
						{/each}
						{#if showAllSettings}
							{#each settings.extra as row (row.label)}
								<div class="flex justify-between gap-3">
									<dt class="text-gray-500 dark:text-gray-400">{row.label}</dt>
									<dd class="truncate text-right text-gray-700 dark:text-gray-300">{row.value}</dd>
								</div>
							{/each}
						{/if}
					</dl>
					{#if settings.extra.length > 0}
						<button
							type="button"
							onclick={() => (showAllSettings = !showAllSettings)}
							class="mt-3 text-xs font-medium text-gray-500 hover:text-gray-800 hover:underline dark:text-gray-400 dark:hover:text-gray-200"
						>
							{showAllSettings
								? m.plugins_settings_fewer()
								: m.plugins_settings_more({ count: settings.extra.length })}
						</button>
					{/if}
					<p
						class="mt-4 border-t border-gray-100 pt-3 text-xs text-gray-500 dark:border-gray-700 dark:text-gray-400"
					>
						{m.plugins_created({ date: formatDateTime(pipeline.created_at) })}
					</p>
				</section>
			</aside>
		</div>
	{/if}
</div>

{#if selectedRun}
	<IngestionRunModal
		bind:show={showRun}
		run={selectedRun}
		onClose={() => {
			showRun = false;
			selectedRun = null;
		}}
	/>
{/if}

<ConfirmModal
	bind:show={confirmDelete}
	title={m.runs_delete_pipeline_title()}
	message={m.pipelines_delete_confirm({ name: pipeline?.name ?? '' })}
	checkboxLabel={m.runs_delete_teardown_checkbox_label()}
	bind:checkboxChecked={teardown}
	onConfirm={deletePipeline}
/>
