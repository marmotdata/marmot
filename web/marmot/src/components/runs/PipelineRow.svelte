<script lang="ts">
	import IconifyIcon from '@iconify/svelte';
	import { resolve } from '$app/paths';
	import cronstrue from 'cronstrue/i18n';
	import Icon from '$components/ui/Icon.svelte';
	import { fetchApi } from '$lib/api';
	import { auth } from '$lib/stores/auth';
	import { m } from '$lib/paraglide/messages';
	import { getLocale } from '$lib/paraglide/runtime';
	import { formatDuration, formatNumber, formatRelativeTime } from '$lib/utils/format';

	interface Pipeline {
		id: string;
		name: string;
		plugin_id: string;
		cron_expression: string;
		enabled: boolean;
		queryable?: boolean;
		last_run_at?: string;
		last_run_status?: string;
		next_run_at?: string;
		managed_by?: string;
	}

	interface Run {
		id: string;
		status: string;
		started_at?: string;
		finished_at?: string;
		created_at: string;
		error_message?: string;
		assets_created: number;
		assets_updated: number;
	}

	interface Props {
		pipeline: Pipeline;
		pluginName?: string;
		running?: boolean;
		canRun?: boolean;
		canEdit?: boolean;
		onRun: () => void;
		onEdit: () => void;
		onDelete: () => void;
		onOpenRun: (runId: string) => void;
	}

	let {
		pipeline,
		pluginName,
		running = false,
		canRun = true,
		canEdit = true,
		onRun,
		onEdit,
		onDelete,
		onOpenRun
	}: Props = $props();

	// One connected source: its last ten runs as a strip, what the latest did, its schedule in words and its actions, with the error inline when the last run failed
	const STRIP = 10;
	const canManage = auth.hasPermission('ingestion', 'manage');

	let runs = $state<Run[]>([]);
	let menuOpen = $state(false);

	let managed = $derived(!!pipeline.managed_by);
	let latest = $derived(runs[0]);
	let failed = $derived(!running && latest?.status === 'failed');
	let strip = $derived([...runs].reverse());
	let duration = $derived(
		latest?.started_at && latest.finished_at
			? new Date(latest.finished_at).getTime() - new Date(latest.started_at).getTime()
			: null
	);

	let schedule = $derived.by(() => {
		if (!pipeline.cron_expression) return m.runs_schedule_manual();
		try {
			return cronstrue.toString(pipeline.cron_expression, { locale: getLocale() });
		} catch {
			return pipeline.cron_expression;
		}
	});

	async function loadRuns() {
		try {
			const res = await fetchApi(
				`/ingestion/runs?schedule_id=${encodeURIComponent(pipeline.id)}&limit=${STRIP}`
			);
			if (res.ok) runs = (await res.json()).runs ?? [];
		} catch {
			// The strip is a summary; the row still works without it
		}
	}

	// Reload whenever the pipeline reports a new run, which the page updates live
	$effect(() => {
		void pipeline.last_run_at;
		void running;
		loadRuns();
	});

	const STATUS_LABELS: Record<string, () => string> = {
		pending: m.runs_status_pending,
		claimed: m.runs_status_claimed,
		running: m.runs_status_running,
		succeeded: m.runs_status_succeeded,
		failed: m.runs_status_failed,
		cancelled: m.runs_status_cancelled
	};
	const statusLabel = (status: string) => STATUS_LABELS[status]?.() ?? status;

	function dotClass(status: string): string {
		if (status === 'succeeded') return 'bg-earthy-green-500';
		if (status === 'failed') return 'bg-red-500';
		if (status === 'running' || status === 'claimed' || status === 'pending')
			return 'bg-earthy-blue-500 animate-pulse';
		return 'bg-gray-300 dark:bg-gray-600';
	}
</script>

<svelte:window onclick={() => (menuOpen = false)} />

<li class="group px-5 py-4 transition hover:bg-gray-50/70 dark:hover:bg-gray-800/40">
	<div class="flex flex-col gap-4 lg:flex-row lg:items-center">
		<!-- Identity, which opens the pipeline's own page -->
		<a
			href={resolve('/runs/[id]', { id: pipeline.id })}
			class="group/name flex min-w-0 items-center gap-3 lg:w-80 lg:flex-shrink-0"
		>
			<span
				class="flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-lg border border-gray-200 bg-white dark:border-gray-700 dark:bg-gray-900"
			>
				<Icon name={pipeline.plugin_id} size="sm" showLabel={false} />
			</span>
			<div class="min-w-0">
				<div class="flex items-center gap-2">
					<span
						class="truncate font-medium text-gray-900 group-hover/name:text-earthy-terracotta-700 group-hover/name:underline dark:text-gray-100 dark:group-hover/name:text-earthy-terracotta-300"
						>{pipeline.name}</span
					>
					{#if pipeline.queryable}
						<span
							class="inline-flex items-center gap-0.5 rounded-full bg-earthy-terracotta-100 px-1.5 py-0.5 text-[10px] font-medium text-earthy-terracotta-800 dark:bg-earthy-terracotta-900/40 dark:text-earthy-terracotta-200"
							title={m.runs_queryable_title()}
						>
							<IconifyIcon icon="material-symbols:bolt" class="h-3 w-3" />
							{m.runs_queryable_badge()}
						</span>
					{/if}
					{#if managed}
						<span
							class="inline-flex items-center gap-0.5 rounded-full bg-earthy-blue-100 px-1.5 py-0.5 text-[10px] font-medium text-earthy-blue-800 dark:bg-earthy-blue-900/40 dark:text-earthy-blue-200"
							title={m.runs_operator_managed_title()}
						>
							<IconifyIcon icon="mdi:kubernetes" class="h-3 w-3" />
							{m.runs_operator_badge()}
						</span>
					{/if}
				</div>
				<p class="truncate text-xs text-gray-500 dark:text-gray-400">
					{pluginName ?? pipeline.plugin_id}
				</p>
			</div>
		</a>

		<!-- Recent runs -->
		<div
			class="flex items-center gap-1 lg:w-44 lg:flex-shrink-0"
			aria-label={m.plugins_recent_runs()}
		>
			{#each Array(STRIP - strip.length) as _, i (i)}
				<span class="h-5 w-2 rounded-sm bg-gray-100 dark:bg-gray-800"></span>
			{/each}
			{#each strip as run (run.id)}
				<button
					type="button"
					onclick={() => onOpenRun(run.id)}
					class="h-5 w-2 rounded-sm transition hover:scale-y-125 {dotClass(run.status)}"
					title="{statusLabel(run.status)} · {formatRelativeTime(run.created_at)}"
					aria-label="{statusLabel(run.status)} · {formatRelativeTime(run.created_at)}"
				></button>
			{/each}
		</div>

		<!-- Last run -->
		<div class="min-w-0 flex-1 text-sm">
			{#if running}
				<span
					class="inline-flex items-center gap-1.5 font-medium text-earthy-blue-700 dark:text-earthy-blue-300"
				>
					<IconifyIcon icon="material-symbols:progress-activity" class="h-4 w-4 animate-spin" />
					{m.plugins_running()}
				</span>
			{:else if latest}
				<p class="text-gray-800 dark:text-gray-200">
					{formatRelativeTime(latest.created_at)}{#if duration !== null}<span class="text-gray-400">
							· {formatDuration(duration)}</span
						>{/if}
				</p>
				{#if !failed}
					<p class="text-xs text-gray-500 dark:text-gray-400">
						{m.plugins_run_counts({
							created: formatNumber(latest.assets_created),
							updated: formatNumber(latest.assets_updated)
						})}
					</p>
				{/if}
			{:else}
				<p class="text-gray-400">{m.plugins_never_run()}</p>
			{/if}
		</div>

		<!-- Schedule -->
		<div
			class="flex items-center gap-1.5 text-sm text-gray-600 lg:w-56 lg:flex-shrink-0 dark:text-gray-400"
		>
			<IconifyIcon
				icon="material-symbols:schedule-outline"
				class="h-4 w-4 flex-shrink-0 text-gray-400"
			/>
			<span class="truncate" title={pipeline.cron_expression || undefined}>{schedule}</span>
		</div>

		<!-- Actions -->
		{#if canManage && !managed}
			<div class="flex items-center gap-1 lg:justify-end">
				<button
					type="button"
					onclick={onRun}
					disabled={running || !canRun}
					class="inline-flex items-center gap-1.5 rounded-lg border border-gray-200 bg-white px-3 py-1.5 text-sm font-medium text-gray-700 transition hover:border-gray-300 hover:bg-gray-50 disabled:cursor-not-allowed disabled:opacity-50 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-200 dark:hover:bg-gray-700"
				>
					<IconifyIcon icon="material-symbols:play-arrow-rounded" class="h-4 w-4" />
					{m.plugins_run_now()}
				</button>
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
						class="rounded-lg p-1.5 text-gray-400 transition hover:bg-gray-100 hover:text-gray-700 dark:hover:bg-gray-700 dark:hover:text-gray-200"
					>
						<IconifyIcon icon="material-symbols:more-horiz" class="h-5 w-5" />
					</button>
					{#if menuOpen}
						<div
							class="absolute right-0 z-20 mt-1 w-40 overflow-hidden rounded-lg border border-gray-200 bg-white py-1 shadow-lg dark:border-gray-700 dark:bg-gray-800"
							role="menu"
						>
							<button
								type="button"
								role="menuitem"
								onclick={onEdit}
								disabled={!canEdit}
								class="flex w-full items-center gap-2 px-3 py-2 text-left text-sm text-gray-700 hover:bg-gray-50 disabled:opacity-50 dark:text-gray-200 dark:hover:bg-gray-700"
							>
								<IconifyIcon icon="material-symbols:edit-outline" class="h-4 w-4" />
								{m.common_edit()}
							</button>
							<button
								type="button"
								role="menuitem"
								onclick={onDelete}
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

	{#if failed && latest?.error_message}
		<button
			type="button"
			onclick={() => onOpenRun(latest.id)}
			class="mt-3 flex w-full items-start gap-2 rounded-lg bg-red-50 px-3 py-2 text-left text-xs text-red-800 transition hover:bg-red-100 lg:ml-[3.25rem] lg:w-[calc(100%-3.25rem)] dark:bg-red-900/20 dark:text-red-200 dark:hover:bg-red-900/30"
		>
			<IconifyIcon icon="material-symbols:error-outline" class="mt-0.5 h-4 w-4 flex-shrink-0" />
			<span class="line-clamp-2 font-mono">{latest.error_message}</span>
		</button>
	{/if}
</li>
