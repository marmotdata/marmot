<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { SvelteSet } from 'svelte/reactivity';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { fetchApi } from '$lib/api';
	import { m } from '$lib/paraglide/messages';
	import { websocketService, type JobRunEvent } from '$lib/websocket';
	import { auth } from '$lib/stores/auth';
	import { toasts } from '$lib/stores/toast';
	import { encryptionConfigured, allowUnencrypted } from '$lib/stores/encryption';
	import Button from '$components/ui/Button.svelte';
	import IconifyIcon from '@iconify/svelte';
	import IngestionRunModal, { type IngestionRun } from '$components/runs/IngestionRunModal.svelte';
	import PipelineRow from '$components/runs/PipelineRow.svelte';
	import PluginGallery from '$components/runs/PluginGallery.svelte';
	import ConfirmModal from '$components/ui/ConfirmModal.svelte';

	let canManageIngestion = $derived(auth.hasPermission('ingestion', 'manage'));

	// Shell commands are technical tokens, so they live outside the message catalog
	const installCliCommand = 'curl -fsSL get.marmotdata.io | sh';
	const generateKeyCommand = 'marmot generate-encryption-key';

	let unsubscribe: (() => void) | null = null;
	let wsConnected = $state(false);
	let wsCheckInterval: ReturnType<typeof setInterval> | null = null;

	interface IngestionRunsResponse {
		runs: IngestionRun[];
		total: number;
		limit: number;
		offset: number;
	}

	interface Pipeline {
		id: string;
		name: string;
		plugin_id: string;
		config: Record<string, unknown>;
		cron_expression: string;
		enabled: boolean;
		queryable?: boolean;
		ingest_on_query?: boolean;
		last_run_at?: string;
		last_run_status?: string;
		next_run_at?: string;
		created_by?: string;
		created_at: string;
		updated_at: string;
	}

	interface PipelinesResponse {
		schedules: Pipeline[]; // API still uses 'schedules' key
		total: number;
		limit: number;
		offset: number;
	}

	let selectedRun = $state<IngestionRun | null>(null);
	let showRunModal = $state(false);

	let pipelines = $state<Pipeline[]>([]);
	let pipelinesLoading = $state(false);
	let pipelinesError = $state<string | null>(null);
	let pipelinesTotal = $state(0);
	let pipelinesPage = $state(1);
	let pipelinesPageSize = $state(25);

	let showConfirmModal = $state(false);
	let confirmModalTitle = $state('');
	let confirmModalMessage = $state('');
	let confirmModalCheckboxLabel = $state('');
	let confirmModalCheckboxChecked = $state(false);
	let confirmModalAction = $state<((checkboxValue?: boolean) => void) | null>(null);

	let runningPipelines: SvelteSet<string> = new SvelteSet();

	interface PluginInfo {
		id: string;
		name: string;
		description?: string;
		icon?: string;
		category?: string;
		supports_query?: boolean;
	}

	// Every installed plugin, for the gallery and for showing each pipeline's plugin by its proper name
	let plugins = $state<PluginInfo[]>([]);
	let pluginNames = $derived(Object.fromEntries(plugins.map((p) => [p.id, p.name])));
	let connectedCounts = $derived(
		pipelines.reduce<Record<string, number>>((acc, p) => {
			acc[p.plugin_id] = (acc[p.plugin_id] ?? 0) + 1;
			return acc;
		}, {})
	);

	async function fetchPlugins() {
		try {
			const res = await fetchApi('/plugins');
			if (res.ok) plugins = (await res.json()).plugins ?? [];
		} catch {
			// The gallery stays empty; connected pipelines still show
		}
	}

	// A bar in a pipeline's run strip opens that run in the same modal as the history tab
	async function openRun(runId: string) {
		try {
			const res = await fetchApi(`/ingestion/runs/${encodeURIComponent(runId)}`);
			if (res.ok) handleRunClick(await res.json());
		} catch {
			toasts.error(m.plugins_run_load_error());
		}
	}

	function showAvailable() {
		requestAnimationFrame(() =>
			document.getElementById('available')?.scrollIntoView({ behavior: 'smooth', block: 'start' })
		);
	}

	let pipelinesTotalPages = $derived(Math.ceil(pipelinesTotal / pipelinesPageSize));
	let pipelinesOffset = $derived((pipelinesPage - 1) * pipelinesPageSize);

	function handleRunClick(run: IngestionRun) {
		selectedRun = run;
		showRunModal = true;
	}

	function handleModalClose() {
		showRunModal = false;
	}

	async function fetchPipelines() {
		try {
			pipelinesLoading = true;
			pipelinesError = null;

			const params = new URLSearchParams({
				limit: pipelinesPageSize.toString(),
				offset: pipelinesOffset.toString()
			});

			const response = await fetchApi(`/ingestion/schedules?${params}`);
			if (!response.ok) {
				throw new Error(m.runs_pipelines_fetch_error());
			}

			const data: PipelinesResponse = await response.json();
			pipelines = data.schedules || [];
			pipelinesTotal = data.total || 0;
		} catch (err) {
			console.error('Error fetching pipelines:', err);
			pipelinesError = err instanceof Error ? err.message : m.runs_pipelines_load_error();
		} finally {
			pipelinesLoading = false;
		}
	}

	function goToPipelinesPage(page: number) {
		if (page >= 1 && page <= pipelinesTotalPages) {
			pipelinesPage = page;
			fetchPipelines();
		}
	}

	async function handleTriggerPipeline(pipeline: Pipeline) {
		try {
			// Add to running set immediately - SvelteSet is reactive on mutation
			runningPipelines.add(pipeline.id);

			const response = await fetchApi(`/ingestion/schedules/${pipeline.id}/trigger`, {
				method: 'POST'
			});

			if (!response.ok) {
				const data = await response.json();
				// Remove from running set on error
				runningPipelines.delete(pipeline.id);
				throw new Error(data.error || m.runs_trigger_error());
			}

			// Poll for running status
			pollPipelineStatus(pipeline.id);
		} catch (err) {
			const errorMsg = err instanceof Error ? err.message : m.runs_trigger_error();
			toasts.error(errorMsg);
		}
	}

	async function pollPipelineStatus(pipelineId: string) {
		// Poll every 2 seconds for up to 30 seconds
		let attempts = 0;
		const maxAttempts = 15;
		const pollInterval = 2000;

		const poll = async () => {
			try {
				// Fetch latest runs to check status
				const params = new URLSearchParams({
					limit: '1',
					offset: '0',
					pipelines: pipelineId
				});

				const response = await fetchApi(`/runs?${params}`);
				if (!response.ok) {
					throw new Error('Failed to fetch run status');
				}

				const data: IngestionRunsResponse = await response.json();
				const latestRun = data.runs?.[0];

				// If run is no longer running or we've exceeded max attempts, stop polling
				if (latestRun && latestRun.status !== 'running') {
					runningPipelines.delete(pipelineId);

					// Show completion toast
					if (latestRun.status === 'succeeded') {
						toasts.success(m.runs_pipeline_completed_success());
					} else if (latestRun.status === 'failed') {
						toasts.error(
							m.runs_pipeline_failed_error({
								error: latestRun.error_message || m.runs_unknown_error()
							})
						);
					}

					// Refresh pipelines list to update last_run_at
					fetchPipelines();
					return;
				}

				attempts++;
				if (attempts < maxAttempts && runningPipelines.has(pipelineId)) {
					setTimeout(poll, pollInterval);
				} else {
					// Max attempts reached or pipeline removed from running set
					runningPipelines.delete(pipelineId);
				}
			} catch (err) {
				console.error('Error polling pipeline status:', err);
				// Stop polling on error
				runningPipelines.delete(pipelineId);
			}
		};

		// Start polling after initial delay
		setTimeout(poll, pollInterval);
	}

	async function handleDeletePipeline(pipeline: Pipeline) {
		confirmModalTitle = m.runs_delete_pipeline_title();
		confirmModalMessage = m.pipelines_delete_confirm({ name: pipeline.name });
		confirmModalCheckboxLabel = m.runs_delete_teardown_checkbox_label();
		confirmModalCheckboxChecked = false;
		confirmModalAction = async (teardown?: boolean) => {
			showConfirmModal = false;
			try {
				const url = teardown
					? `/ingestion/schedules/${pipeline.id}?teardown=true`
					: `/ingestion/schedules/${pipeline.id}`;

				const response = await fetchApi(url, {
					method: 'DELETE'
				});

				if (!response.ok) {
					const data = await response.json();
					throw new Error(data.error || m.runs_delete_pipeline_error());
				}

				const successMsg = teardown
					? m.runs_delete_teardown_success({ name: pipeline.name })
					: m.runs_delete_success({ name: pipeline.name });
				toasts.success(successMsg);

				// Refresh the list
				fetchPipelines();
			} catch (err) {
				const errorMsg = err instanceof Error ? err.message : m.runs_delete_pipeline_error();
				toasts.error(errorMsg);
			}
		};
		showConfirmModal = true;
	}

	const RUN_STATUSES = [
		'pending',
		'claimed',
		'running',
		'succeeded',
		'failed',
		'cancelled'
	] as const;
	type RunStatus = (typeof RUN_STATUSES)[number];
	function isRunStatus(value: unknown): value is RunStatus {
		return typeof value === 'string' && (RUN_STATUSES as readonly string[]).includes(value);
	}

	interface JobRunPatch {
		id?: string;
		schedule_id?: string;
		status?: RunStatus;
		finished_at?: string;
		updated_at?: string;
	}

	// The websocket only carries a plain payload bag; narrow the fields this page reads instead of casting it to IngestionRun
	function toIngestionRunPatch(payload: Record<string, unknown>): JobRunPatch {
		return {
			id: typeof payload.id === 'string' ? payload.id : undefined,
			schedule_id: typeof payload.schedule_id === 'string' ? payload.schedule_id : undefined,
			status: isRunStatus(payload.status) ? payload.status : undefined,
			finished_at: typeof payload.finished_at === 'string' ? payload.finished_at : undefined,
			updated_at: typeof payload.updated_at === 'string' ? payload.updated_at : undefined
		};
	}

	function handleJobRunEvent(event: JobRunEvent) {
		const jobRun = toIngestionRunPatch(event.payload);

		// Update running pipelines status for pipelines tab
		if (jobRun.schedule_id) {
			if (
				event.type === 'job_run_started' ||
				event.type === 'job_run_claimed' ||
				(event.type === 'job_run_created' && jobRun.status === 'running')
			) {
				// Add to running set - SvelteSet is reactive on mutation
				runningPipelines.add(jobRun.schedule_id);
			} else if (
				event.type === 'job_run_completed' ||
				event.type === 'job_run_cancelled' ||
				jobRun.status === 'succeeded' ||
				jobRun.status === 'failed' ||
				jobRun.status === 'cancelled'
			) {
				// Remove from running set - SvelteSet is reactive on mutation
				runningPipelines.delete(jobRun.schedule_id);

				// Update the pipeline's last_run_status and last_run_at
				pipelines = pipelines.map((p) =>
					p.id === jobRun.schedule_id
						? {
								...p,
								last_run_status: jobRun.status,
								last_run_at: jobRun.finished_at || jobRun.updated_at
							}
						: p
				);
			}
		}

		// Update selected run if it's the one being shown
		if (selectedRun && jobRun.id && selectedRun.id === jobRun.id) {
			selectedRun = {
				...selectedRun,
				status: jobRun.status ?? selectedRun.status,
				finished_at: jobRun.finished_at ?? selectedRun.finished_at
			};
		}
	}

	onMount(() => {
		fetchPipelines();
		fetchPlugins();

		// Subscribe to websocket events
		unsubscribe = websocketService.subscribe(handleJobRunEvent);

		// Check websocket connection status
		wsConnected = websocketService.connected();

		// Poll for connection status
		wsCheckInterval = setInterval(() => {
			const newStatus = websocketService.connected();
			if (newStatus !== wsConnected) {
				wsConnected = newStatus;
			}
		}, 2000);
	});

	onDestroy(() => {
		// Unsubscribe from websocket events
		if (unsubscribe) {
			unsubscribe();
		}
		// Clear websocket status check interval
		if (wsCheckInterval) {
			clearInterval(wsCheckInterval);
		}
	});
</script>

<div class="w-full px-4 py-6 sm:px-6 lg:px-8">
	<div class="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
		<div>
			<h1 class="text-2xl font-semibold tracking-tight text-gray-900 dark:text-gray-100">
				{m.runs_heading()}
			</h1>
			<p class="mt-1 text-sm text-gray-600 dark:text-gray-400">{m.runs_subheading()}</p>
		</div>
		{#if canManageIngestion}
			<button
				type="button"
				onclick={showAvailable}
				disabled={!$encryptionConfigured}
				class="inline-flex flex-shrink-0 items-center gap-2 self-start rounded-lg bg-earthy-terracotta-600 px-4 py-2 text-sm font-semibold text-white shadow-sm transition hover:bg-earthy-terracotta-500 disabled:cursor-not-allowed disabled:opacity-40"
			>
				<IconifyIcon icon="material-symbols:add" class="h-5 w-5" />
				{m.plugins_add()}
			</button>
		{/if}
	</div>

	<div class="mt-6"></div>
	{#if pipelinesLoading}
		<div class="space-y-2">
			{#each Array(5) as _, i (i)}
				<div class="h-16 animate-pulse rounded-xl bg-gray-100 dark:bg-gray-800"></div>
			{/each}
		</div>
	{:else if pipelinesError}
		<div
			class="bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800/50 rounded-lg p-4"
		>
			<div class="flex">
				<IconifyIcon icon="material-symbols:error" class="h-5 w-5 text-red-400 mt-0.5" />
				<div class="ml-3">
					<h3 class="text-sm font-medium text-red-800 dark:text-red-200">{m.common_error()}</h3>
					<p class="mt-1 text-sm text-red-700 dark:text-red-300">{pipelinesError}</p>
				</div>
			</div>
		</div>
	{:else}
		<!-- Unencrypted Mode Warning -->
		{#if $allowUnencrypted}
			<div
				class="mb-6 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800/50 rounded-lg p-4"
			>
				<div class="flex items-start">
					<IconifyIcon
						icon="material-symbols:warning"
						class="h-5 w-5 text-red-500 mt-0.5 flex-shrink-0"
					/>
					<div class="ml-3">
						<h3 class="text-sm font-medium text-red-800 dark:text-red-200">
							{m.runs_unencrypted_mode_heading()}
						</h3>
						<p class="mt-1 text-sm text-red-700 dark:text-red-300">
							{m.runs_unencrypted_mode_hint()}
						</p>
					</div>
				</div>
			</div>
		{:else if !$encryptionConfigured}
			<!-- Encryption Key Not Configured Warning -->
			<div
				class="mb-6 bg-amber-50 dark:bg-amber-900/20 border border-amber-200 dark:border-amber-800/50 rounded-lg p-4"
			>
				<div class="flex items-start">
					<IconifyIcon
						icon="material-symbols:warning"
						class="h-5 w-5 text-amber-500 mt-0.5 flex-shrink-0"
					/>
					<div class="ml-3">
						<h3 class="text-sm font-medium text-amber-800 dark:text-amber-200">
							{m.runs_encryption_not_configured_heading()}
						</h3>
						<p class="mt-1 text-sm text-amber-700 dark:text-amber-300">
							{m.runs_encryption_not_configured_hint()}
						</p>
						<div class="mt-3 text-sm text-amber-700 dark:text-amber-300 space-y-2">
							<p>{m.runs_encryption_get_started_hint()}</p>
							<div
								class="bg-amber-100 dark:bg-amber-900/40 rounded-md px-3 py-2 font-mono text-xs space-y-1"
							>
								<p>{installCliCommand}</p>
								<p>{generateKeyCommand}</p>
							</div>
							<p>
								{m.runs_encryption_set_env_hint({ variable: 'MARMOT_SERVER_ENCRYPTION_KEY' })}
								<a
									href="https://marmotdata.io/docs/Deploy/"
									target="_blank"
									rel="noopener noreferrer"
									class="underline font-medium hover:text-amber-900 dark:hover:text-amber-100"
									>{m.runs_deploy_docs_link()}</a
								>
							</p>
						</div>
					</div>
				</div>
			</div>
		{/if}

		<!-- Connected sources, then everything that could be connected -->
		{#if pipelines.length === 0}
			<div
				class="flex items-center gap-3 rounded-xl border border-dashed border-gray-300 px-5 py-4 text-sm text-gray-600 dark:border-gray-700 dark:text-gray-400"
			>
				<IconifyIcon icon="material-symbols:extension-outline" class="h-5 w-5 text-gray-400" />
				{canManageIngestion ? m.plugins_none_connected() : m.runs_no_pipelines_hint_readonly()}
			</div>
		{:else}
			<ul
				class="divide-y divide-gray-100 rounded-xl border border-gray-200 bg-white dark:divide-gray-700/60 dark:border-gray-700 dark:bg-gray-800"
			>
				{#each pipelines as pipeline (pipeline.id)}
					<PipelineRow
						{pipeline}
						pluginName={pluginNames[pipeline.plugin_id]}
						running={runningPipelines.has(pipeline.id)}
						canRun={$encryptionConfigured}
						canEdit={$encryptionConfigured}
						onRun={() => handleTriggerPipeline(pipeline)}
						onEdit={() => goto(resolve(`/pipelines/${pipeline.id}/edit`))}
						onDelete={() => handleDeletePipeline(pipeline)}
						onOpenRun={openRun}
					/>
				{/each}
			</ul>

			{#if pipelinesTotalPages > 1}
				<div class="mt-3 flex items-center justify-between">
					<div class="text-sm text-gray-600 dark:text-gray-400">
						{m.runs_page_of({ current: pipelinesPage, total: pipelinesTotalPages })}
					</div>
					<div class="flex items-center gap-2">
						<Button
							variant="clear"
							click={() => goToPipelinesPage(pipelinesPage - 1)}
							disabled={pipelinesPage === 1}
							icon="material-symbols:chevron-left"
							text={m.common_previous()}
						/>
						<Button
							variant="clear"
							click={() => goToPipelinesPage(pipelinesPage + 1)}
							disabled={pipelinesPage === pipelinesTotalPages}
							text={m.common_next()}
							icon="material-symbols:chevron-right"
						/>
					</div>
				</div>
			{/if}
		{/if}

		<div class="mt-10">
			<PluginGallery
				{plugins}
				connected={connectedCounts}
				canAdd={canManageIngestion && $encryptionConfigured}
			/>
		</div>
	{/if}
</div>

{#if selectedRun}
	<IngestionRunModal bind:show={showRunModal} run={selectedRun} onClose={handleModalClose} />
{/if}

<ConfirmModal
	bind:show={showConfirmModal}
	title={confirmModalTitle}
	message={confirmModalMessage}
	checkboxLabel={confirmModalCheckboxLabel}
	bind:checkboxChecked={confirmModalCheckboxChecked}
	onConfirm={(checkboxValue) => confirmModalAction?.(checkboxValue)}
/>
