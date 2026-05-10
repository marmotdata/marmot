<script lang="ts">
	import { fetchApi } from '$lib/api';
	import { onMount } from 'svelte';
	import { resolve } from '$app/paths';
	import IconifyIcon from '@iconify/svelte';
	import RunHistoryHistogram from './RunHistoryHistogram.svelte';

	interface AssetRef {
		type?: string;
		name?: string;
	}

	export let assetId: string;
	export let minimal = false;
	export let asset: AssetRef | null = null;

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

	interface RunHistoryResponse {
		run_history: RunHistoryEntry[];
		total: number;
		limit: number;
		offset: number;
	}

	let runHistory: RunHistoryEntry[] = [];
	let loading = true;
	let error: string | null = null;
	let total = 0;
	let currentPage = 1;

	$: pageSize = minimal ? 5 : 10;
	$: totalPages = Math.ceil(total / pageSize);

	function statusKind(status: string): 'success' | 'error' | 'running' | 'other' {
		switch (status.toUpperCase()) {
			case 'COMPLETE':
				return 'success';
			case 'FAIL':
				return 'error';
			case 'RUNNING':
				return 'running';
			default:
				return 'other';
		}
	}

	function getTypeColor(type: string): string {
		switch (type.toUpperCase()) {
			case 'BATCH':
				return 'bg-blue-100 text-blue-800 dark:bg-blue-900/30 dark:text-blue-300';
			case 'STREAMING':
				return 'bg-purple-100 text-purple-800 dark:bg-purple-900/30 dark:text-purple-300';
			default:
				return 'bg-gray-100 text-gray-800 dark:bg-gray-900/30 dark:text-gray-300';
		}
	}

	function formatDateTime(dateString: string): string {
		if (minimal) {
			return new Date(dateString).toLocaleDateString();
		}
		return new Date(dateString).toLocaleString();
	}

	function formatDurationMs(ms: number): string {
		if (ms <= 0) return '—';
		if (ms < 1000) return `${ms}ms`;
		const seconds = ms / 1000;
		if (seconds < 60) return `${seconds.toFixed(2)}s`;
		const minutes = Math.floor(seconds / 60);
		const hours = Math.floor(minutes / 60);
		if (hours > 0) return `${hours}h ${minutes % 60}m`;
		return `${minutes}m ${Math.floor(seconds) % 60}s`;
	}

	async function fetchRunHistory() {
		try {
			loading = true;
			error = null;

			const offset = (currentPage - 1) * pageSize;
			const response = await fetchApi(
				`/assets/run-history/${assetId}?limit=${pageSize}&offset=${offset}`
			);

			if (!response.ok) {
				throw new Error('Failed to fetch run history');
			}

			const data: RunHistoryResponse = await response.json();
			runHistory = data.run_history;
			total = data.total;
		} catch (err) {
			console.error('Error fetching run history:', err);
			error = err instanceof Error ? err.message : 'Failed to load run history';
		} finally {
			loading = false;
		}
	}

	function goToPage(page: number) {
		if (page >= 1 && page <= totalPages) {
			currentPage = page;
			fetchRunHistory();
		}
	}

	let lastAssetId = '';

	onMount(() => {
		if (assetId) {
			lastAssetId = assetId;
			fetchRunHistory();
		}
	});

	$: if (assetId && assetId !== lastAssetId) {
		lastAssetId = assetId;
		currentPage = 1;
		fetchRunHistory();
	}
</script>

<div class="space-y-4">
	<RunHistoryHistogram {assetId} period="30d" minimal={true} />

	{#if loading}
		<div class="flex items-center justify-center {minimal ? 'py-6' : 'py-12'}">
			<div
				class="animate-spin rounded-full {minimal
					? 'h-6 w-6'
					: 'h-8 w-8'} border-b-2 border-earthy-terracotta-700"
			></div>
		</div>
	{:else if error}
		<div
			class="bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800/50 rounded-lg p-4"
		>
			<p class="text-red-600 dark:text-red-400 {minimal ? 'text-sm' : ''}">{error}</p>
		</div>
	{:else if runHistory.length === 0}
		<div
			class="rounded-xl border border-dashed border-gray-300 dark:border-gray-700 p-10 text-center"
		>
			<IconifyIcon
				icon="material-symbols:history"
				class="w-10 h-10 text-gray-300 dark:text-gray-600 mx-auto"
			/>
			<h3 class="mt-2 text-sm font-medium text-gray-900 dark:text-gray-100">No run history</h3>
			<p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
				This asset has no recorded job executions.
			</p>
		</div>
	{:else}
		<div
			class="rounded-xl border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800 overflow-hidden"
		>
			<div
				class="px-5 py-3 border-b border-gray-200 dark:border-gray-700 flex items-center justify-between"
			>
				<div class="text-sm font-medium text-gray-900 dark:text-gray-100">Recent runs</div>
				{#if !minimal}
					<div class="text-xs text-gray-500 dark:text-gray-400">
						{(currentPage - 1) * pageSize + 1}–{Math.min(currentPage * pageSize, total)} of {total}
					</div>
				{:else if asset}
					<a
						href={resolve(
							`/discover/${asset?.type?.toLowerCase()}/${encodeURIComponent(asset?.name || '')}?tab=run-history`
						)}
						class="text-xs text-earthy-terracotta-700 dark:text-earthy-terracotta-500 hover:text-earthy-terracotta-800"
					>
						View all →
					</a>
				{/if}
			</div>

			<ul class="divide-y divide-gray-200 dark:divide-gray-700">
				{#each runHistory as run (run.id)}
					{@const kind = statusKind(run.status)}
					<li class="px-5 py-4 hover:bg-gray-50 dark:hover:bg-gray-700/30 transition-colors">
						<div class="flex items-center justify-between gap-4">
							<div class="flex items-center gap-3 min-w-0">
								{#if kind === 'success'}
									<div
										class="w-7 h-7 rounded-full bg-earthy-green-100 dark:bg-earthy-green-900/30 flex items-center justify-center flex-shrink-0"
									>
										<IconifyIcon
											icon="material-symbols:check-rounded"
											class="w-4 h-4 text-earthy-green-700 dark:text-earthy-green-500"
										/>
									</div>
								{:else if kind === 'error'}
									<div
										class="w-7 h-7 rounded-full bg-red-50 dark:bg-red-900/30 flex items-center justify-center flex-shrink-0"
									>
										<IconifyIcon
											icon="material-symbols:close-rounded"
											class="w-4 h-4 text-red-600 dark:text-red-400"
										/>
									</div>
								{:else if kind === 'running'}
									<div
										class="w-7 h-7 rounded-full bg-amber-50 dark:bg-amber-900/30 flex items-center justify-center flex-shrink-0"
									>
										<div
											class="animate-spin h-3 w-3 border-2 border-amber-600 dark:border-amber-400 border-t-transparent rounded-full"
										></div>
									</div>
								{:else}
									<div
										class="w-7 h-7 rounded-full bg-gray-50 dark:bg-gray-700 flex items-center justify-center flex-shrink-0"
									>
										<IconifyIcon
											icon="material-symbols:radio-button-unchecked"
											class="w-4 h-4 text-gray-400"
										/>
									</div>
								{/if}
								<div class="min-w-0">
									<div class="text-sm font-medium text-gray-900 dark:text-gray-100 truncate">
										{run.job_name}
									</div>
									<div class="text-xs text-gray-500 dark:text-gray-400 truncate">
										{run.start_time ? formatDateTime(run.start_time) : '—'}
										{#if run.job_namespace}
											· <span class="font-mono">{run.job_namespace}</span>
										{/if}
									</div>
								</div>
							</div>
							<div
								class="flex items-center gap-4 flex-shrink-0 text-xs text-gray-500 dark:text-gray-400"
							>
								{#if run.type}
									<span
										class="inline-flex px-2 py-0.5 text-[10px] font-medium rounded-full {getTypeColor(
											run.type
										)}"
									>
										{run.type}
									</span>
								{/if}
								<div>
									<span class="text-gray-400">duration </span>
									<span class="text-gray-900 dark:text-gray-100 font-mono">
										{run.duration_ms ? formatDurationMs(run.duration_ms) : '—'}
									</span>
								</div>
							</div>
						</div>
					</li>
				{/each}
			</ul>

			{#if !minimal && totalPages > 1}
				<div
					class="px-5 py-3 border-t border-gray-200 dark:border-gray-700 flex items-center justify-end gap-2"
				>
					<button
						onclick={() => goToPage(currentPage - 1)}
						disabled={currentPage === 1}
						class="px-3 py-1.5 text-xs font-medium rounded-lg border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800 hover:bg-gray-50 dark:hover:bg-gray-700 disabled:opacity-50 disabled:cursor-not-allowed text-gray-700 dark:text-gray-300 transition-colors"
					>
						Previous
					</button>
					<button
						onclick={() => goToPage(currentPage + 1)}
						disabled={currentPage === totalPages}
						class="px-3 py-1.5 text-xs font-medium rounded-lg border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800 hover:bg-gray-50 dark:hover:bg-gray-700 disabled:opacity-50 disabled:cursor-not-allowed text-gray-700 dark:text-gray-300 transition-colors"
					>
						Next
					</button>
				</div>
			{/if}
		</div>
	{/if}
</div>
