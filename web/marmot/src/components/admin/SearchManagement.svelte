<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { fetchApi } from '$lib/api';
	import { m } from '$lib/paraglide/messages';
	import { formatDateTime, formatNumber } from '$lib/utils';
	import { websocketService, type SearchReindexEvent } from '$lib/websocket';
	import '@marmotdata/elements/progress.js';

	import { formatBytes, type SystemSearchIndex } from '$lib/settings/system';

	// The index's size comes from the System page's admin-only read; the reindex progress comes over the websocket
	let { index, onReindexed }: { index?: SystemSearchIndex; onReindexed?: () => void } = $props();

	type Status = 'idle' | 'running' | 'completed' | 'failed';

	let running = $state(false);
	let esConfigured = $state(false);
	let loading = $state(true);
	let error = $state<string | null>(null);

	let indexed = $state(0);
	let errors = $state(0);
	let total = $state(0);
	let status = $state<Status>('idle');

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
				onReindexed?.();
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

	// Counts only mean something once a reindex has reported them, so an idle index shows just its status
	let counted = $derived(status !== 'idle' || total > 0 || indexed > 0);
</script>

<mm-section heading={m.admin_search_index_heading()} description={m.admin_reindex_description()}>
	{#if !loading && esConfigured}
		<mm-button
			slot="actions"
			icon="refresh"
			loading={running}
			disabled={running}
			onclick={startReindex}
			>{running ? m.admin_reindexing_label() : m.admin_start_reindex()}</mm-button
		>
	{/if}

	{#if index}
		<mm-description-list layout="inline-grid" columns={4}>
			<mm-description term={m.admin_search_documents()}>
				<span class="mm-num">{formatNumber(index.documents)}</span>
			</mm-description>
			<mm-description term={m.admin_search_size()}>
				<span class="mm-num">{formatBytes(index.size_bytes)}</span>
			</mm-description>
			{#if esConfigured}
				<mm-description term={m.admin_search_last_full_reindex()}>
					{index.last_full_reindex
						? formatDateTime(index.last_full_reindex)
						: m.admin_search_never_reindexed()}
				</mm-description>
			{/if}
			<mm-description term={m.common_status()}>
				{#if status === 'failed'}
					<mm-status-pill status="danger" icon="error">{statusLabel}</mm-status-pill>
				{:else if running}
					{m.admin_search_status_running()}
				{:else}
					{m.admin_search_up_to_date()}
				{/if}
			</mm-description>
		</mm-description-list>
	{/if}

	{#if loading}
		<mm-skeleton loading count={2}></mm-skeleton>
	{:else if !esConfigured}
		{#if !index}
			<mm-callout tone="info" compact heading={m.admin_es_not_configured()}></mm-callout>
		{/if}
	{:else}
		<div class="mm-stack" style="--mm-gap: var(--space-5)">
			{#if !index || counted}
				<mm-description-list layout="inline-grid" columns={3}>
					{#if !index}
						<mm-description term={m.common_status()}>
							{#if status === 'failed'}
								<mm-status-pill status="danger" icon="error">{statusLabel}</mm-status-pill>
							{:else}
								{statusLabel}
							{/if}
						</mm-description>
					{/if}
					{#if counted}
						<mm-description term={m.admin_search_indexed_label()}>
							<span class="mm-num">{formatNumber(indexed)}</span>
						</mm-description>
						<mm-description term={m.admin_search_errors_label()}>
							<span class="mm-num">{formatNumber(errors)}</span>
						</mm-description>
					{/if}
				</mm-description-list>
			{/if}

			{#if status === 'running'}
				<mm-progress
					label={m.admin_indexing_documents()}
					value={indexed}
					max={total || 1}
					indeterminate={total === 0}
					unit={m.admin_search_documents_unit()}
				></mm-progress>
			{/if}

			{#if status === 'completed' && errors > 0}
				<mm-callout tone="warning" compact heading={m.admin_reindex_finished_with_errors()}
				></mm-callout>
			{/if}

			{#if status === 'failed' || error}
				<mm-callout
					tone="danger"
					compact
					heading={m.admin_reindex_failed_message({ error: error ?? m.admin_reindex_failed() })}
				></mm-callout>
			{/if}
		</div>
	{/if}
</mm-section>
