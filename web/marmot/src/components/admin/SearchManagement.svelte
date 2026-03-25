<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { fetchApi } from '$lib/api';
	import { websocketService, type SearchReindexEvent } from '$lib/websocket';

	let running = false;
	let esConfigured = false;
	let loading = true;
	let error: string | null = null;

	let indexed = 0;
	let errors = 0;
	let total = 0;
	let status: 'idle' | 'running' | 'completed' | 'failed' = 'idle';

	let unsubscribe: (() => void) | null = null;

	function handleReindexEvent(event: SearchReindexEvent) {
		const p = event.payload;
		switch (event.type) {
			case 'search_reindex_started':
				status = 'running';
				running = true;
				total = p.total ?? 0;
				indexed = 0;
				errors = 0;
				error = null;
				break;
			case 'search_reindex_progress':
				status = 'running';
				indexed = p.indexed ?? 0;
				errors = p.errors ?? 0;
				total = p.total ?? total;
				break;
			case 'search_reindex_completed':
				status = 'completed';
				running = false;
				indexed = p.indexed ?? indexed;
				errors = p.errors ?? errors;
				total = p.total ?? total;
				break;
			case 'search_reindex_failed':
				status = 'failed';
				running = false;
				indexed = p.indexed ?? indexed;
				errors = p.errors ?? errors;
				total = p.total ?? total;
				error = p.error ?? 'Reindex failed';
				break;
		}
	}

	async function fetchStatus() {
		try {
			const response = await fetchApi('/admin/search/reindex');
			const data = await response.json();
			running = data.running;
			esConfigured = data.es_configured;
			if (running) {
				status = 'running';
			}
		} catch (err) {
			// Non-critical - we'll still show the UI
		} finally {
			loading = false;
		}
	}

	async function startReindex() {
		error = null;
		try {
			const response = await fetchApi('/admin/search/reindex', { method: 'POST' });
			if (!response.ok) {
				const data = await response.json();
				error = data.error || 'Failed to start reindex';
				return;
			}
			status = 'running';
			running = true;
			indexed = 0;
			errors = 0;
			total = 0;
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to start reindex';
		}
	}

	onMount(() => {
		fetchStatus();
		unsubscribe = websocketService.subscribeToReindex(handleReindexEvent);
	});

	onDestroy(() => {
		if (unsubscribe) unsubscribe();
	});

	$: progress = total > 0 ? Math.round((indexed / total) * 100) : 0;
</script>

<div class="bg-earthy-brown-50 dark:bg-gray-900 rounded-lg border border-gray-200 dark:border-gray-700">
	<div class="p-6">
		<h3 class="text-lg font-medium text-gray-900 dark:text-gray-100 mb-4">Search Index</h3>

		{#if loading}
			<div class="flex justify-center p-8">
				<div class="animate-spin rounded-full h-8 w-8 border-b-2 border-earthy-terracotta-700" />
			</div>
		{:else if !esConfigured}
			<div class="bg-yellow-50 dark:bg-yellow-900/20 border border-yellow-200 dark:border-yellow-700 rounded-lg p-4 text-yellow-700 dark:text-yellow-300">
				Elasticsearch is not configured. Search reindexing is unavailable.
			</div>
		{:else}
			<p class="text-sm text-gray-600 dark:text-gray-400 mb-4">
				Rebuild the search index from the database. This is useful if the search index has become out of sync.
			</p>

			<button
				class="px-4 py-2 bg-earthy-terracotta-700 dark:bg-earthy-terracotta-700 text-white rounded-md hover:bg-earthy-terracotta-800 dark:hover:bg-earthy-terracotta-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-earthy-terracotta-600 dark:focus:ring-earthy-terracotta-600 disabled:opacity-50 disabled:cursor-not-allowed"
				disabled={running}
				on:click={startReindex}
			>
				{running ? 'Reindexing...' : 'Start Reindex'}
			</button>

			{#if status === 'running'}
				<div class="mt-4">
					<div class="flex justify-between text-sm text-gray-600 dark:text-gray-400 mb-1">
						<span>Indexing documents...</span>
						<span>
							{indexed.toLocaleString()}{total > 0 ? ` / ${total.toLocaleString()}` : ''}
							{errors > 0 ? ` (${errors} errors)` : ''}
						</span>
					</div>
					<div class="w-full bg-gray-200 dark:bg-gray-700 rounded-full h-2.5">
						<div
							class="bg-earthy-terracotta-700 h-2.5 rounded-full transition-all duration-300"
							style="width: {progress}%"
						/>
					</div>
				</div>
			{/if}

			{#if status === 'completed'}
				<div class="mt-4 bg-green-50 dark:bg-green-900/20 border border-green-200 dark:border-green-700 rounded-lg p-4 text-green-700 dark:text-green-300">
					Reindex complete: {indexed.toLocaleString()} documents indexed{errors > 0 ? `, ${errors} errors` : ''}.
				</div>
			{/if}

			{#if status === 'failed'}
				<div class="mt-4 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-700 rounded-lg p-4 text-red-700 dark:text-red-300">
					Reindex failed: {error}
					{indexed > 0 ? ` (${indexed.toLocaleString()} documents indexed before failure)` : ''}
				</div>
			{/if}

			{#if error && status !== 'failed'}
				<div class="mt-4 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-700 rounded-lg p-4 text-red-700 dark:text-red-300">
					{error}
				</div>
			{/if}
		{/if}
	</div>
</div>
