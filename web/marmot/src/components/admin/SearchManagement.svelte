<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { fetchApi } from '$lib/api';
	import { m } from '$lib/paraglide/messages';
	import { formatNumber } from '$lib/utils';
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
				error = p.error ?? m.admin_reindex_failed();
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
				error = data.error || m.admin_reindex_start_error();
				return;
			}
			status = 'running';
			running = true;
			indexed = 0;
			errors = 0;
			total = 0;
		} catch (err) {
			error = err instanceof Error ? err.message : m.admin_reindex_start_error();
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

<div
	class="bg-earthy-brown-50 dark:bg-gray-900 rounded-lg border border-gray-200 dark:border-gray-700"
>
	<div class="p-6">
		<h3 class="text-lg font-medium text-gray-900 dark:text-gray-100 mb-4">
			{m.admin_search_index_heading()}
		</h3>

		{#if loading}
			<div class="flex justify-center p-8">
				<div class="animate-spin rounded-full h-8 w-8 border-b-2 border-earthy-terracotta-700" />
			</div>
		{:else if !esConfigured}
			<div
				class="bg-yellow-50 dark:bg-yellow-900/20 border border-yellow-200 dark:border-yellow-700 rounded-lg p-4 text-yellow-700 dark:text-yellow-300"
			>
				{m.admin_es_not_configured()}
			</div>
		{:else}
			<p class="text-sm text-gray-600 dark:text-gray-400 mb-4">
				{m.admin_reindex_description()}
			</p>

			<button
				class="px-4 py-2 bg-earthy-terracotta-700 dark:bg-earthy-terracotta-700 text-white rounded-md hover:bg-earthy-terracotta-800 dark:hover:bg-earthy-terracotta-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-earthy-terracotta-600 dark:focus:ring-earthy-terracotta-600 disabled:opacity-50 disabled:cursor-not-allowed"
				disabled={running}
				on:click={startReindex}
			>
				{running ? m.admin_reindexing_label() : m.admin_start_reindex()}
			</button>

			{#if status === 'running'}
				<div class="mt-4">
					<div class="flex justify-between text-sm text-gray-600 dark:text-gray-400 mb-1">
						<span>{m.admin_indexing_documents()}</span>
						<span>
							{formatNumber(indexed)}{total > 0 ? ` / ${formatNumber(total)}` : ''}
							{errors > 0 ? m.admin_reindex_errors_suffix({ count: errors }) : ''}
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
				<div
					class="mt-4 bg-green-50 dark:bg-green-900/20 border border-green-200 dark:border-green-700 rounded-lg p-4 text-green-700 dark:text-green-300"
				>
					{errors > 0
						? m.admin_reindex_complete_with_errors({ count: formatNumber(indexed), errors })
						: m.admin_reindex_complete({ count: formatNumber(indexed) })}
				</div>
			{/if}

			{#if status === 'failed'}
				<div
					class="mt-4 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-700 rounded-lg p-4 text-red-700 dark:text-red-300"
				>
					{indexed > 0
						? m.admin_reindex_failed_with_progress({ error, count: formatNumber(indexed) })
						: m.admin_reindex_failed_message({ error })}
				</div>
			{/if}

			{#if error && status !== 'failed'}
				<div
					class="mt-4 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-700 rounded-lg p-4 text-red-700 dark:text-red-300"
				>
					{error}
				</div>
			{/if}
		{/if}
	</div>
</div>
