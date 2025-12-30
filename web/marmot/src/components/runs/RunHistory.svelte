<script lang="ts">
	import { fetchApi } from '$lib/api';
	import { onMount } from 'svelte';
	import RunHistoryHistogram from './RunHistoryHistogram.svelte';

	export let assetId: string;
	export let minimal = false;
	export let asset: any = null;

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

	function getEventTypeColor(eventType: string): string {
		switch (eventType.toUpperCase()) {
			case 'RUNNING':
				return 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900/30 dark:text-yellow-300';
			case 'COMPLETE':
				return 'bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-300';
			case 'FAIL':
				return 'bg-red-100 text-red-800 dark:bg-red-900/30 dark:text-red-300';
			case 'ABORT':
				return 'bg-gray-100 text-gray-800 dark:bg-gray-900/30 dark:text-gray-300';
			default:
				return 'bg-gray-100 text-gray-800 dark:bg-gray-900/30 dark:text-gray-300';
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
		if (ms <= 0) return '-';

		const seconds = Math.floor(ms / 1000);
		const minutes = Math.floor(seconds / 60);
		const hours = Math.floor(minutes / 60);

		if (hours > 0) {
			return `${hours}h ${minutes % 60}m`;
		} else if (minutes > 0) {
			return `${minutes}m ${seconds % 60}s`;
		} else {
			return `${seconds}s`;
		}
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

<div class="space-y-6">
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
		<div class="text-center {minimal ? 'py-6' : 'py-12'}">
			{#if !minimal}
				<div class="text-gray-400 dark:text-gray-500 mb-4">
					<svg class="mx-auto h-12 w-12" fill="none" stroke="currentColor" viewBox="0 0 24 24">
						<path
							stroke-linecap="round"
							stroke-linejoin="round"
							stroke-width="1.5"
							d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z"
						/>
					</svg>
				</div>
			{/if}
			<h3
				class="text-lg font-medium text-gray-900 dark:text-gray-100 {minimal
					? 'text-base mb-1'
					: 'mb-2'}"
			>
				No Run History
			</h3>
			<p class="text-gray-500 dark:text-gray-400 {minimal ? 'text-sm' : ''}">
				This asset has no recorded job executions.
			</p>
		</div>
	{:else}
		<div>
			{#if minimal}
				<div class="flex justify-between items-center mb-3">
					<h4 class="text-base font-medium text-gray-900 dark:text-gray-100">Recent Runs</h4>
					<a
						href="/discover/{asset?.type?.toLowerCase()}/{encodeURIComponent(
							asset?.name || ''
						)}?tab=run-history"
						class="text-sm text-earthy-terracotta-700 dark:text-earthy-terracotta-700 hover:text-earthy-terracotta-700 dark:hover:text-earthy-terracotta-400"
					>
						View all runs →
					</a>
				</div>
			{:else}
				<h3 class="text-lg font-medium text-gray-900 dark:text-gray-100 mb-4">Recent Runs</h3>

				<div class="mb-4">
					<p class="text-gray-600 dark:text-gray-400">
						Showing {(currentPage - 1) * pageSize + 1} to {Math.min(currentPage * pageSize, total)} of
						{total}
						runs
					</p>
				</div>

				{#if totalPages > 1}
					<div class="flex gap-2 mb-4">
						<button
							onclick={() => goToPage(currentPage - 1)}
							disabled={currentPage === 1}
							class="px-3 py-1 rounded border border-gray-300 dark:border-gray-600 disabled:opacity-50 text-gray-700 dark:text-gray-300"
						>
							Previous
						</button>
						<button
							onclick={() => goToPage(currentPage + 1)}
							disabled={currentPage === totalPages}
							class="px-3 py-1 rounded border border-gray-300 dark:border-gray-600 disabled:opacity-50 text-gray-700 dark:text-gray-300"
						>
							Next
						</button>
					</div>
				{/if}
			{/if}

			<div class="rounded-lg shadow dark:shadow-white/20 overflow-hidden">
				{#each runHistory as run}
					<div
						class="p-{minimal
							? '2'
							: '4'} border-b border-gray-200 dark:border-gray-600 hover:bg-gray-100 dark:hover:bg-gray-600"
					>
						<div class="flex items-center justify-between {minimal ? 'mb-1' : 'mb-3'}">
							<div class="flex items-center space-x-2">
								<h4
									class="font-medium text-gray-900 dark:text-gray-100 {minimal
										? 'text-sm'
										: 'text-lg'}"
								>
									{run.job_name}
								</h4>
								<span
									class="inline-flex px-2 py-1 text-xs font-semibold rounded-full {getEventTypeColor(
										run.status
									)}"
								>
									{run.status}
								</span>
							</div>
							<span class="text-sm text-gray-500 dark:text-gray-400 font-medium">
								{run.duration_ms ? formatDurationMs(run.duration_ms) : '-'}
							</span>
						</div>

						{#if minimal}
							<div class="text-xs text-gray-500 dark:text-gray-400">
								{run.start_time ? formatDateTime(run.start_time) : '-'}
							</div>
						{:else}
							<div class="grid grid-cols-2 md:grid-cols-4 gap-4 text-sm">
								<div>
									<div
										class="text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider"
									>
										Namespace
									</div>
									<div class="mt-1 text-gray-900 dark:text-gray-100">
										{run.job_namespace}
									</div>
								</div>
								<div>
									<div
										class="text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider"
									>
										Type
									</div>
									<div class="mt-1">
										<span
											class="inline-flex px-2 py-1 text-xs font-semibold rounded-full {getTypeColor(
												run.type
											)}"
										>
											{run.type}
										</span>
									</div>
								</div>
								<div>
									<div
										class="text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider"
									>
										Start Time
									</div>
									<div class="mt-1 text-gray-900 dark:text-gray-100">
										{run.start_time ? formatDateTime(run.start_time) : '-'}
									</div>
								</div>
								<div>
									<div
										class="text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider"
									>
										End Time
									</div>
									<div class="mt-1 text-gray-900 dark:text-gray-100">
										{run.end_time ? formatDateTime(run.end_time) : '-'}
									</div>
								</div>
							</div>
						{/if}
					</div>
				{/each}
			</div>

			{#if !minimal && totalPages > 1}
				<div class="flex gap-2 mt-4">
					<button
						onclick={() => goToPage(currentPage - 1)}
						disabled={currentPage === 1}
						class="px-3 py-1 rounded border border-gray-300 dark:border-gray-600 disabled:opacity-50 text-gray-700 dark:text-gray-300"
					>
						Previous
					</button>
					<button
						onclick={() => goToPage(currentPage + 1)}
						disabled={currentPage === totalPages}
						class="px-3 py-1 rounded border border-gray-300 dark:border-gray-600 disabled:opacity-50 text-gray-700 dark:text-gray-300"
					>
						Next
					</button>
				</div>
			{/if}
		</div>
	{/if}
</div>
