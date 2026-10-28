<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { fetchApi } from '$lib/api';
	import { m } from '$lib/paraglide/messages';
	import { formatNumber } from '$lib/utils';
	import { websocketService, type SearchReindexEvent } from '$lib/websocket';
	import IconifyIcon from '@iconify/svelte';
	import Panel from '$components/ui/Panel.svelte';
	import Pill from '$components/ui/Pill.svelte';
	import Alert from '$components/ui/Alert.svelte';
	import Skeleton from '$components/ui/Skeleton.svelte';
	import { BTN_SECONDARY, LABEL } from '$lib/ui/classes';

	type Status = 'idle' | 'running' | 'completed' | 'failed';

	let running = $state(false);
	let esConfigured = $state(false);
	let loading = $state(true);
	let error = $state<string | null>(null);

	let indexed = $state(0);
	let errors = $state(0);
	let total = $state(0);
	let status = $state<Status>('idle');

	const STATUS_TONE: Record<Status, 'neutral' | 'info' | 'success' | 'danger'> = {
		idle: 'neutral',
		running: 'info',
		completed: 'success',
		failed: 'danger'
	};
	let statusLabel = $derived(
		{
			idle: m.admin_search_status_idle(),
			running: m.admin_search_status_running(),
			completed: m.admin_search_status_completed(),
			failed: m.admin_search_status_failed()
		}[status]
	);

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
		} catch {
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

	let progress = $derived(total > 0 ? Math.round((indexed / total) * 100) : 0);
</script>

<Panel
	title={m.admin_search_index_heading()}
	description={esConfigured ? m.admin_reindex_description() : undefined}
	icon="material-symbols:manage-search"
>
	{#snippet action()}
		{#if !loading && esConfigured}
			<button type="button" class={BTN_SECONDARY} disabled={running} onclick={startReindex}>
				<IconifyIcon
					icon="material-symbols:sync"
					class="h-5 w-5 text-gray-400 {running ? 'animate-spin' : ''}"
				/>
				{running ? m.admin_reindexing_label() : m.admin_start_reindex()}
			</button>
		{/if}
	{/snippet}

	{#if loading}
		<Skeleton class="h-16" />
	{:else if !esConfigured}
		<Alert tone="warning">{m.admin_es_not_configured()}</Alert>
	{:else}
		<dl class="grid grid-cols-1 gap-4 sm:grid-cols-3">
			<div>
				<dt class={LABEL}>{m.common_status()}</dt>
				<dd class="mt-1.5">
					<Pill tone={STATUS_TONE[status]}>{statusLabel}</Pill>
				</dd>
			</div>
			<div>
				<dt class={LABEL}>{m.admin_search_indexed_label()}</dt>
				<dd class="mt-1 text-2xl font-semibold tabular-nums text-gray-900 dark:text-gray-100">
					{formatNumber(indexed)}{#if total > 0}<span class="text-sm font-normal text-gray-400">
							/ {formatNumber(total)}</span
						>{/if}
				</dd>
			</div>
			<div>
				<dt class={LABEL}>{m.admin_search_errors_label()}</dt>
				<dd
					class="mt-1 text-2xl font-semibold tabular-nums {errors > 0
						? 'text-red-600 dark:text-red-400'
						: 'text-gray-900 dark:text-gray-100'}"
				>
					{formatNumber(errors)}
				</dd>
			</div>
		</dl>

		{#if status === 'running'}
			<div class="mt-5">
				<div class="mb-1 flex justify-between text-xs text-gray-500 dark:text-gray-400">
					<span>{m.admin_indexing_documents()}</span>
					<span class="tabular-nums">{progress}%</span>
				</div>
				<div class="h-1.5 w-full rounded-full bg-gray-100 dark:bg-gray-700">
					<div
						class="h-1.5 rounded-full bg-earthy-terracotta-600 transition-all duration-300"
						style="width: {progress}%"
					></div>
				</div>
			</div>
		{/if}

		{#if status === 'completed'}
			<div class="mt-5">
				<Alert tone={errors > 0 ? 'warning' : 'success'}>
					{errors > 0
						? m.admin_reindex_complete_with_errors({ count: formatNumber(indexed), errors })
						: m.admin_reindex_complete({ count: formatNumber(indexed) })}
				</Alert>
			</div>
		{/if}

		{#if status === 'failed'}
			<div class="mt-5">
				<Alert>
					{indexed > 0
						? m.admin_reindex_failed_with_progress({
								error: error ?? '',
								count: formatNumber(indexed)
							})
						: m.admin_reindex_failed_message({ error: error ?? '' })}
				</Alert>
			</div>
		{/if}

		{#if error && status !== 'failed'}
			<div class="mt-5"><Alert>{error}</Alert></div>
		{/if}
	{/if}
</Panel>
