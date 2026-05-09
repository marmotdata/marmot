<script lang="ts">
	import { fetchApi } from '$lib/api';
	import type { Asset } from '$lib/assets/types';
	import IconifyIcon from '@iconify/svelte';
	import RunsActivityChart, { type Bucket } from './RunsActivityChart.svelte';

	let { asset }: { asset: Asset } = $props();

	interface ToolCall {
		ordinal: number;
		tool_name: string;
		target_mrn?: string;
		started_at: string;
		duration_ms?: number;
		status: string;
	}

	interface Run {
		id: string;
		run_id: string;
		started_at: string;
		ended_at?: string;
		duration_ms?: number;
		status: string;
		model?: string;
		tokens_in: number;
		tokens_out: number;
		error?: string;
		tool_calls?: ToolCall[];
	}

	interface Stats {
		run_count: number;
		success_rate: number;
		median_latency_ms: number;
		p95_latency_ms: number;
		tokens_in: number;
		tokens_out: number;
	}

	let stats: Stats | null = $state(null);
	let buckets: Bucket[] = $state([]);
	let runs: Run[] = $state([]);
	let loading = $state(true);
	let error: string | null = $state(null);

	async function load() {
		loading = true;
		error = null;
		try {
			const [statsRes, activityRes, runsRes] = await Promise.all([
				fetchApi(`/agents/${asset.id}/stats?period=24h`),
				fetchApi(`/agents/${asset.id}/activity?period=24h`),
				fetchApi(`/agents/${asset.id}/runs?period=24h&limit=25`)
			]);
			if (!statsRes.ok || !activityRes.ok || !runsRes.ok) {
				throw new Error('Failed to load agent run data');
			}
			stats = await statsRes.json();
			const activity = await activityRes.json();
			buckets = (activity.buckets ?? []).map(
				(b: { hour: string; success: number; error: number }) => ({
					hour: new Date(b.hour),
					success: b.success,
					error: b.error
				})
			);
			const runsBody = await runsRes.json();
			runs = runsBody.runs ?? [];
		} catch (e) {
			error = e instanceof Error ? e.message : String(e);
		} finally {
			loading = false;
		}
	}

	$effect(() => {
		if (asset?.id) {
			load();
		}
	});

	function formatDuration(ms?: number): string {
		if (ms == null) return '—';
		if (ms < 1000) return `${ms}ms`;
		return `${(ms / 1000).toFixed(2)}s`;
	}

	function formatTokens(n: number): string {
		if (n < 1000) return String(n);
		return `${(n / 1000).toFixed(1)}k`;
	}

	function relativeTime(iso: string): string {
		const then = new Date(iso).getTime();
		const diff = Date.now() - then;
		if (diff < 60_000) return 'just now';
		if (diff < 3_600_000) return `${Math.floor(diff / 60_000)} min ago`;
		if (diff < 86_400_000) return `${Math.floor(diff / 3_600_000)} hr ago`;
		return `${Math.floor(diff / 86_400_000)} d ago`;
	}

	function fmtPercent(rate: number): string {
		return `${Math.round(rate * 100)}%`;
	}

	let totalActivity = $derived(buckets.reduce((sum, b) => sum + b.success + b.error, 0));
</script>

<div class="space-y-4">
	{#if loading}
		<div class="flex items-center justify-center py-12">
			<div class="animate-spin rounded-full h-8 w-8 border-b-2 border-earthy-terracotta-700"></div>
		</div>
	{:else if error}
		<div
			class="bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800/50 rounded-lg p-4"
		>
			<p class="text-sm text-red-600 dark:text-red-400">{error}</p>
		</div>
	{:else}
		<!-- Activity chart -->
		<div
			class="rounded-xl border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800 p-4"
		>
			<div class="flex items-center justify-between mb-2">
				<div class="text-[10px] font-semibold uppercase tracking-wider text-gray-400">
					Activity · last 24h
				</div>
				<div class="flex items-center gap-3 text-[11px] text-gray-500 dark:text-gray-400">
					<span class="inline-flex items-center gap-1.5">
						<span class="inline-block w-2.5 h-2.5 rounded-sm bg-earthy-green-700"></span>
						success
					</span>
					<span class="inline-flex items-center gap-1.5">
						<span class="inline-block w-2.5 h-2.5 rounded-sm bg-red-600"></span>
						error
					</span>
				</div>
			</div>
			{#if totalActivity === 0}
				<div class="flex items-center justify-center h-[120px]">
					<p class="text-xs text-gray-400 dark:text-gray-500">No runs in this window</p>
				</div>
			{:else}
				<RunsActivityChart {buckets} />
			{/if}
		</div>

		<!-- Stats -->
		<div class="grid grid-cols-2 md:grid-cols-4 gap-3">
			<div
				class="rounded-lg border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800 p-4"
			>
				<div class="text-[10px] font-semibold uppercase tracking-wider text-gray-400">
					Runs · 24h
				</div>
				<div class="mt-1 text-2xl font-semibold text-gray-900 dark:text-gray-100">
					{stats?.run_count ?? 0}
				</div>
			</div>
			<div
				class="rounded-lg border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800 p-4"
			>
				<div class="text-[10px] font-semibold uppercase tracking-wider text-gray-400">
					Median latency
				</div>
				<div class="mt-1 text-2xl font-semibold text-gray-900 dark:text-gray-100">
					{stats && stats.run_count > 0 ? formatDuration(stats.median_latency_ms) : '—'}
				</div>
			</div>
			<div
				class="rounded-lg border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800 p-4"
			>
				<div class="text-[10px] font-semibold uppercase tracking-wider text-gray-400">
					Tokens · 24h
				</div>
				<div class="mt-1 text-2xl font-semibold text-gray-900 dark:text-gray-100">
					{stats ? formatTokens(stats.tokens_in + stats.tokens_out) : '0'}
				</div>
			</div>
			<div
				class="rounded-lg border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800 p-4"
			>
				<div class="text-[10px] font-semibold uppercase tracking-wider text-gray-400">
					Success rate
				</div>
				<div
					class="mt-1 text-2xl font-semibold {stats && stats.run_count > 0
						? 'text-earthy-green-700 dark:text-earthy-green-500'
						: 'text-gray-400'}"
				>
					{stats && stats.run_count > 0 ? fmtPercent(stats.success_rate) : '—'}
				</div>
			</div>
		</div>

		<!-- Run list -->
		<div
			class="rounded-xl border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800 overflow-hidden"
		>
			<div class="px-5 py-3 border-b border-gray-200 dark:border-gray-700">
				<div class="text-sm font-medium text-gray-900 dark:text-gray-100">Recent invocations</div>
			</div>

			{#if runs.length === 0}
				<div class="px-5 py-12 text-center">
					<IconifyIcon
						icon="material-symbols:bolt-outline"
						class="w-10 h-10 text-gray-300 dark:text-gray-600 mx-auto"
					/>
					<p class="mt-2 text-sm text-gray-500 dark:text-gray-400">
						No runs yet. Invoke this agent with the Marmot LangChain callback attached and runs will
						land here.
					</p>
				</div>
			{:else}
				<ul class="divide-y divide-gray-200 dark:divide-gray-700">
					{#each runs as run}
						<li class="px-5 py-4 hover:bg-gray-50 dark:hover:bg-gray-700/30 transition-colors">
							<div class="flex items-center justify-between gap-4">
								<div class="flex items-center gap-3 min-w-0">
									{#if run.status === 'success'}
										<div
											class="w-7 h-7 rounded-full bg-earthy-green-100 dark:bg-earthy-green-900/30 flex items-center justify-center flex-shrink-0"
										>
											<IconifyIcon
												icon="material-symbols:check-rounded"
												class="w-4 h-4 text-earthy-green-700 dark:text-earthy-green-500"
											/>
										</div>
									{:else}
										<div
											class="w-7 h-7 rounded-full bg-red-50 dark:bg-red-900/30 flex items-center justify-center flex-shrink-0"
										>
											<IconifyIcon
												icon="material-symbols:close-rounded"
												class="w-4 h-4 text-red-600 dark:text-red-400"
											/>
										</div>
									{/if}
									<div class="min-w-0">
										<div class="text-sm font-mono text-gray-900 dark:text-gray-100 truncate">
											{run.run_id.slice(0, 8)}
										</div>
										<div class="text-xs text-gray-500 dark:text-gray-400">
											{relativeTime(run.started_at)}
										</div>
									</div>
								</div>
								<div
									class="flex items-center gap-6 flex-shrink-0 text-xs text-gray-500 dark:text-gray-400"
								>
									<div>
										<span class="text-gray-400">duration </span>
										<span class="text-gray-900 dark:text-gray-100 font-mono">
											{formatDuration(run.duration_ms)}
										</span>
									</div>
									{#if run.tokens_in || run.tokens_out}
										<div>
											<span class="text-gray-400">tokens </span>
											<span class="text-gray-900 dark:text-gray-100 font-mono">
												{run.tokens_in}↓ {run.tokens_out}↑
											</span>
										</div>
									{/if}
								</div>
							</div>
							{#if run.tool_calls && run.tool_calls.length > 0}
								<div class="mt-3 pl-10 flex items-center gap-1 flex-wrap">
									{#each run.tool_calls as call, i}
										<span
											class="inline-flex items-center px-2 py-0.5 rounded text-[11px] font-mono {call.status ===
											'error'
												? 'bg-red-50 text-red-700 dark:bg-red-900/30 dark:text-red-300'
												: 'bg-earthy-terracotta-50 dark:bg-earthy-terracotta-900/30 text-earthy-terracotta-700 dark:text-earthy-terracotta-300'}"
											title={call.target_mrn || ''}
										>
											{call.tool_name}
										</span>
										{#if i < run.tool_calls.length - 1}
											<IconifyIcon
												icon="material-symbols:arrow-right-alt"
												class="w-3.5 h-3.5 text-gray-300 dark:text-gray-600"
											/>
										{/if}
									{/each}
								</div>
							{/if}
							{#if run.error}
								<div
									class="mt-2 pl-10 text-xs font-mono text-red-600 dark:text-red-400 truncate"
									title={run.error}
								>
									{run.error}
								</div>
							{/if}
						</li>
					{/each}
				</ul>
			{/if}
		</div>
	{/if}
</div>
