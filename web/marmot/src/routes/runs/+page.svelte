<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { browser } from '$app/environment';
	import { fetchApi } from '$lib/api';
	import { websocketService, type JobRunEvent } from '$lib/websocket';
	import { auth } from '$lib/stores/auth';
	import Button from '../../components/Button.svelte';
	import IconifyIcon from '@iconify/svelte';
	import IngestionRunCard from '../../components/IngestionRunCard.svelte';
	import IngestionRunModal from '../../components/IngestionRunModal.svelte';
	import GettingStarted from '../../components/GettingStarted.svelte';
	import ScheduleCard from '../../components/ScheduleCard.svelte';
	import ConfirmModal from '../../components/ConfirmModal.svelte';
	import Toast from '../../components/Toast.svelte';

	let canManageIngestion = $derived(auth.hasPermission('ingestion', 'manage'));

	let unsubscribe: (() => void) | null = null;
	let fetchRunsTimeout: ReturnType<typeof setTimeout> | null = null;
	let wsConnected = $state(false);
	let wsCheckInterval: ReturnType<typeof setInterval> | null = null;

	type Tab = 'history' | 'pipelines';

	interface IngestionRunSummary {
		assets_created: number;
		assets_updated: number;
		assets_deleted: number;
		errors: number;
	}

	interface IngestionRun {
		id: string;
		schedule_id?: string;
		status: 'pending' | 'claimed' | 'running' | 'succeeded' | 'failed' | 'cancelled';
		claimed_by?: string;
		claimed_at?: string;
		started_at?: string;
		finished_at?: string;
		error_message?: string;
		assets_created: number;
		assets_updated: number;
		assets_deleted: number;
		lineage_created: number;
		documentation_added: number;
		created_at: string;
		updated_at: string;
	}

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
		config: any;
		cron_expression: string;
		enabled: boolean;
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

	let runs = $state<IngestionRun[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let total = $state(0);
	let currentPage = $state(1);
	let pageSize = $state(10);
	let selectedStatuses = $state<string[]>([]);
	let selectedRun = $state<IngestionRun | null>(null);
	let showRunModal = $state(false);
	let showStatusDropdown = $state(false);
	let activeTab = $state<Tab>('pipelines');

	// Pipelines state
	let pipelines = $state<Pipeline[]>([]);
	let pipelinesLoading = $state(false);
	let pipelinesError = $state<string | null>(null);
	let pipelinesTotal = $state(0);
	let pipelinesPage = $state(1);
	let pipelinesPageSize = $state(10);

	// Confirmation modal state
	let showConfirmModal = $state(false);
	let confirmModalTitle = $state('');
	let confirmModalMessage = $state('');
	let confirmModalCheckboxLabel = $state('');
	let confirmModalCheckboxChecked = $state(false);
	let confirmModalAction = $state<((checkboxValue?: boolean) => void) | null>(null);

	// Toast state
	let showToast = $state(false);
	let toastMessage = $state('');
	let toastVariant = $state<'success' | 'error' | 'info'>('info');

	// Running pipelines tracking
	let runningPipelines = $state<Set<string>>(new Set());

	let totalPages = $derived(Math.ceil(total / pageSize));
	let offset = $derived((currentPage - 1) * pageSize);
	let pipelinesTotalPages = $derived(Math.ceil(pipelinesTotal / pipelinesPageSize));
	let pipelinesOffset = $derived((pipelinesPage - 1) * pipelinesPageSize);

	let showGettingStarted = $derived(
		!loading && runs && runs.length === 0 && selectedStatuses.length === 0
	);

	const availableStatuses = ['pending', 'claimed', 'running', 'succeeded', 'failed', 'cancelled'];

	$effect(() => {
		if (browser) {
			const urlParams = $page.url.searchParams;
			const tabParam = urlParams.get('tab');
			const pageParam = urlParams.get('page');
			const statusesParam = urlParams.get('statuses');
			const runParam = urlParams.get('run');

			if (tabParam === 'history') {
				activeTab = 'history';
			} else if (tabParam === 'pipelines') {
				activeTab = 'pipelines';
			} else {
				activeTab = 'pipelines';
			}

			if (pageParam) {
				const pageNum = parseInt(pageParam);
				if (pageNum > 0) currentPage = pageNum;
			}

			if (statusesParam) {
				selectedStatuses = statusesParam.split(',').filter((s) => s.trim());
			}

			if (runParam && runs.length > 0) {
				const foundRun = runs.find((r) => r.id === runParam);
				if (foundRun && (!selectedRun || selectedRun.id !== foundRun.id)) {
					selectedRun = foundRun;
					showRunModal = true;
				}
			} else if (showRunModal && !runParam) {
				showRunModal = false;
				selectedRun = null;
			}
		}
	});

	function switchTab(tab: Tab) {
		activeTab = tab;
		const url = new URL($page.url);
		if (tab === 'pipelines') {
			url.searchParams.set('tab', 'pipelines');
			// Fetch pipelines when switching to pipelines tab
			fetchPipelines();
		} else {
			url.searchParams.set('tab', 'history');
			// Fetch runs when switching to history tab
			fetchRuns();
		}
		goto(url.toString(), { replaceState: true, noScroll: true });
	}

	function updateUrl() {
		if (!browser) return;

		const url = new URL($page.url);

		if (currentPage > 1) {
			url.searchParams.set('page', currentPage.toString());
		} else {
			url.searchParams.delete('page');
		}

		if (selectedStatuses.length > 0) {
			url.searchParams.set('statuses', selectedStatuses.join(','));
		} else {
			url.searchParams.delete('statuses');
		}

		goto(url.toString(), { replaceState: true, noScroll: true });
	}

	async function fetchRuns(showLoading: boolean = true) {
		try {
			if (showLoading) {
				loading = true;
			}
			error = null;

			const params = new URLSearchParams({
				limit: pageSize.toString(),
				offset: offset.toString()
			});

			if (selectedStatuses.length > 0) {
				params.append('status', selectedStatuses.join(','));
			}

			const response = await fetchApi(`/ingestion/runs?${params}`);
			if (!response.ok) {
				throw new Error('Failed to fetch job runs');
			}

			const data: IngestionRunsResponse = await response.json();
			runs = data.runs || [];
			total = data.total || 0;
		} catch (err) {
			console.error('Error fetching ingestion runs:', err);
			error = err instanceof Error ? err.message : 'Failed to load ingestion runs';
		} finally {
			if (showLoading) {
				loading = false;
			}
		}
	}

	function goToPage(page: number) {
		if (page >= 1 && page <= totalPages) {
			currentPage = page;
			updateUrl();
			fetchRuns();
		}
	}

	function handleStatusToggle(status: string) {
		if (selectedStatuses.includes(status)) {
			selectedStatuses = selectedStatuses.filter((s) => s !== status);
		} else {
			selectedStatuses = [...selectedStatuses, status];
		}
		currentPage = 1;
		updateUrl();
		fetchRuns();
	}

	function resetFilters() {
		selectedStatuses = [];
		currentPage = 1;
		updateUrl();
		fetchRuns();
	}

	function handleRunClick(run: IngestionRun) {
		selectedRun = run;
		showRunModal = true;
		const url = new URL($page.url);
		url.searchParams.set('run', run.id);
		goto(url.toString(), { replaceState: true, noScroll: true });
	}

	function handleModalClose() {
		const url = new URL($page.url);
		url.searchParams.delete('run');
		goto(url.toString(), { replaceState: true, noScroll: true });
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
				throw new Error('Failed to fetch pipelines');
			}

			const data: PipelinesResponse = await response.json();
			pipelines = data.schedules || [];
			pipelinesTotal = data.total || 0;
		} catch (err) {
			console.error('Error fetching pipelines:', err);
			pipelinesError = err instanceof Error ? err.message : 'Failed to load pipelines';
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
			// Add to running set immediately - create new Set for reactivity
			const newSet = new Set(runningPipelines);
			newSet.add(pipeline.id);
			runningPipelines = newSet;
			console.log('Manually triggered pipeline:', pipeline.id);

			const response = await fetchApi(`/ingestion/schedules/${pipeline.id}/trigger`, {
				method: 'POST'
			});

			if (!response.ok) {
				const data = await response.json();
				// Remove from running set on error - create new Set for reactivity
				const errorSet = new Set(runningPipelines);
				errorSet.delete(pipeline.id);
				runningPipelines = errorSet;
				throw new Error(data.error || 'Failed to trigger pipeline');
			}

			// Poll for running status
			pollPipelineStatus(pipeline.id);
		} catch (err) {
			const errorMsg = err instanceof Error ? err.message : 'Failed to trigger pipeline';
			toastMessage = errorMsg;
			toastVariant = 'error';
			showToast = true;
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
					const newSet = new Set(runningPipelines);
					newSet.delete(pipelineId);
					runningPipelines = newSet;

					// Show completion toast
					if (latestRun.status === 'succeeded') {
						toastMessage = `Pipeline completed successfully!`;
						toastVariant = 'success';
					} else if (latestRun.status === 'failed') {
						toastMessage = `Pipeline failed: ${latestRun.error_message || 'Unknown error'}`;
						toastVariant = 'error';
					}
					showToast = true;

					// Refresh pipelines list to update last_run_at
					fetchPipelines();
					return;
				}

				attempts++;
				if (attempts < maxAttempts && runningPipelines.has(pipelineId)) {
					setTimeout(poll, pollInterval);
				} else {
					// Max attempts reached or pipeline removed from running set
					const newSet = new Set(runningPipelines);
					newSet.delete(pipelineId);
					runningPipelines = newSet;
				}
			} catch (err) {
				console.error('Error polling pipeline status:', err);
				// Stop polling on error
				const newSet = new Set(runningPipelines);
				newSet.delete(pipelineId);
				runningPipelines = newSet;
			}
		};

		// Start polling after initial delay
		setTimeout(poll, pollInterval);
	}

	async function handleDeletePipeline(pipeline: Pipeline) {
		confirmModalTitle = 'Delete Pipeline';
		confirmModalMessage = `Are you sure you want to delete pipeline "${pipeline.name}"? This action cannot be undone.`;
		confirmModalCheckboxLabel = 'Delete all resources created by this pipeline';
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
					throw new Error(data.error || 'Failed to delete pipeline');
				}

				toastMessage = teardown
					? `Pipeline "${pipeline.name}" and all its assets deleted successfully`
					: `Pipeline "${pipeline.name}" deleted successfully`;
				toastVariant = 'success';
				showToast = true;

				// Refresh the list
				fetchPipelines();
			} catch (err) {
				const errorMsg = err instanceof Error ? err.message : 'Failed to delete pipeline';
				toastMessage = errorMsg;
				toastVariant = 'error';
				showToast = true;
			}
		};
		showConfirmModal = true;
	}

	function handleJobRunEvent(event: JobRunEvent) {
		console.log('Received job run event:', event.type, event.payload);

		const jobRun = event.payload;

		// Update running pipelines status for pipelines tab
		if (jobRun.schedule_id) {
			if (
				event.type === 'job_run_started' ||
				event.type === 'job_run_claimed' ||
				(event.type === 'job_run_created' && jobRun.status === 'running')
			) {
				// Add to running set - create new Set to trigger reactivity in Svelte 5
				const newSet = new Set(runningPipelines);
				newSet.add(jobRun.schedule_id);
				runningPipelines = newSet;
				console.log(
					'Pipeline marked as running:',
					jobRun.schedule_id,
					'Total running:',
					runningPipelines.size
				);
			} else if (
				event.type === 'job_run_completed' ||
				event.type === 'job_run_cancelled' ||
				jobRun.status === 'succeeded' ||
				jobRun.status === 'failed' ||
				jobRun.status === 'cancelled'
			) {
				// Remove from running set - create new Set to trigger reactivity in Svelte 5
				const newSet = new Set(runningPipelines);
				newSet.delete(jobRun.schedule_id);
				runningPipelines = newSet;
				console.log(
					'Pipeline completed:',
					jobRun.schedule_id,
					'Status:',
					jobRun.status,
					'Total running:',
					runningPipelines.size
				);

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

		// Only update runs list if we're on the history tab
		if (activeTab !== 'history') return;

		switch (event.type) {
			case 'job_run_created':
				// Only add if not already in list (prevent duplicates)
				if (!runs.some((r) => r.id === jobRun.id)) {
					runs = [jobRun, ...runs];
					total = total + 1;
				}
				break;

			case 'job_run_updated':
			case 'job_run_claimed':
			case 'job_run_started':
			case 'job_run_progress':
			case 'job_run_completed':
			case 'job_run_cancelled':
				// Update existing run
				const index = runs.findIndex((r) => r.id === jobRun.id);
				if (index !== -1) {
					// Create new array with updated run to trigger reactivity
					runs = [...runs.slice(0, index), jobRun, ...runs.slice(index + 1)];
				} else {
					// Debounce refresh if run not in current list
					if (fetchRunsTimeout) {
						clearTimeout(fetchRunsTimeout);
					}
					fetchRunsTimeout = setTimeout(() => {
						fetchRuns(false);
					}, 1000);
				}
				break;
		}

		// Update selected run if it's the one being shown
		if (selectedRun && selectedRun.id === jobRun.id) {
			selectedRun = jobRun;
		}
	}

	onMount(() => {
		console.log('[Runs Page] Component mounted, active tab:', activeTab);

		if (activeTab === 'history') {
			fetchRuns();
		} else if (activeTab === 'pipelines') {
			fetchPipelines();
		}

		// Subscribe to websocket events
		console.log('[Runs Page] Subscribing to websocket events');
		unsubscribe = websocketService.subscribe(handleJobRunEvent);

		// Check websocket connection status
		wsConnected = websocketService.connected();
		console.log('[Runs Page] Websocket connected:', wsConnected);

		// Poll for connection status
		wsCheckInterval = setInterval(() => {
			const newStatus = websocketService.connected();
			if (newStatus !== wsConnected) {
				wsConnected = newStatus;
				console.log('[Runs Page] Websocket status changed:', wsConnected);
			}
		}, 2000);
	});

	onDestroy(() => {
		// Unsubscribe from websocket events
		if (unsubscribe) {
			unsubscribe();
		}
		// Clear pending fetch timeout
		if (fetchRunsTimeout) {
			clearTimeout(fetchRunsTimeout);
		}
		// Clear websocket status check interval
		if (wsCheckInterval) {
			clearInterval(wsCheckInterval);
		}
	});
</script>

<div class="container max-w-7xl mx-auto py-6 px-4 sm:px-6 lg:px-8">
	<div class="mb-6">
		<h1 class="text-2xl font-bold text-gray-900 dark:text-gray-100">Runs</h1>
		<p class="text-gray-600 dark:text-gray-400 mt-1">Monitor ingestion runs and manage pipelines</p>
	</div>

	<!-- Tab Navigation -->
	<div class="border-b border-gray-200 dark:border-gray-700 mb-6">
		<nav class="-mb-px flex space-x-8">
			<button
				onclick={() => switchTab('pipelines')}
				class="whitespace-nowrap pb-4 px-1 border-b-2 font-medium text-sm transition-colors {activeTab ===
				'pipelines'
					? 'border-earthy-terracotta-700 text-earthy-terracotta-700'
					: 'border-transparent text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300 hover:border-gray-300 dark:hover:border-gray-600'}"
			>
				<IconifyIcon
					icon="material-symbols:account-tree"
					class="inline-block h-5 w-5 mr-2 -mt-0.5"
				/>
				Pipelines
			</button>
			<button
				onclick={() => switchTab('history')}
				class="whitespace-nowrap pb-4 px-1 border-b-2 font-medium text-sm transition-colors {activeTab ===
				'history'
					? 'border-earthy-terracotta-700 text-earthy-terracotta-700'
					: 'border-transparent text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300 hover:border-gray-300 dark:hover:border-gray-600'}"
			>
				<IconifyIcon icon="material-symbols:history" class="inline-block h-5 w-5 mr-2 -mt-0.5" />
				Run History
			</button>
		</nav>
	</div>

	<!-- Tab Content -->
	{#if activeTab === 'pipelines'}
		{#if pipelinesLoading}
			<div class="flex items-center justify-center py-12">
				<div
					class="animate-spin rounded-full h-8 w-8 border-b-2 border-earthy-terracotta-700"
				></div>
			</div>
		{:else if pipelinesError}
			<div
				class="bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800/50 rounded-lg p-4"
			>
				<div class="flex">
					<IconifyIcon icon="material-symbols:error" class="h-5 w-5 text-red-400 mt-0.5" />
					<div class="ml-3">
						<h3 class="text-sm font-medium text-red-800 dark:text-red-200">Error</h3>
						<p class="mt-1 text-sm text-red-700 dark:text-red-300">{pipelinesError}</p>
					</div>
				</div>
			</div>
		{:else}
			<!-- Header Actions -->
			{#if canManageIngestion}
				<div class="flex justify-end items-center mb-6">
					<Button
						variant="filled"
						click={() => goto('/pipelines/new')}
						icon="material-symbols:add"
						text="Create Pipeline"
					/>
				</div>
			{/if}

			{#if pipelines.length === 0}
				<div class="text-center py-12">
					<IconifyIcon
						icon="material-symbols:account-tree"
						class="mx-auto h-12 w-12 text-gray-400 mb-4"
					/>
					<h3 class="text-lg font-medium text-gray-900 dark:text-gray-100 mb-2">No Pipelines</h3>
					<p class="text-gray-500 dark:text-gray-400 mb-6">
						{#if canManageIngestion}
							Create a pipeline to ingest data - run on a schedule or trigger manually
						{:else}
							No pipelines have been configured yet
						{/if}
					</p>
					{#if canManageIngestion}
						<Button
							variant="filled"
							click={() => goto('/pipelines/new')}
							icon="material-symbols:add"
							text="Create Pipeline"
						/>
					{/if}
				</div>
			{:else}
				<div class="mb-4">
					<p class="text-gray-600 dark:text-gray-400">
						Showing {pipelines.length} of {pipelinesTotal} pipelines
					</p>
				</div>

				<div
					class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 overflow-hidden mb-6"
				>
					<table class="w-full">
						<thead
							class="bg-gray-50 dark:bg-gray-900 border-b border-gray-200 dark:border-gray-700"
						>
							<tr>
								<th class="px-4 py-3 w-16"></th>
								<th
									class="px-6 py-3 text-left text-xs font-semibold text-gray-700 dark:text-gray-300 uppercase tracking-wider"
								>
									Pipeline
								</th>
								<th
									class="px-6 py-3 text-left text-xs font-semibold text-gray-700 dark:text-gray-300 uppercase tracking-wider"
								>
									Status
								</th>
								<th
									class="px-6 py-3 text-left text-xs font-semibold text-gray-700 dark:text-gray-300 uppercase tracking-wider"
								>
									Schedule
								</th>
								<th
									class="px-6 py-3 text-left text-xs font-semibold text-gray-700 dark:text-gray-300 uppercase tracking-wider"
								>
									Last Run
								</th>
								<th
									class="px-6 py-3 text-left text-xs font-semibold text-gray-700 dark:text-gray-300 uppercase tracking-wider"
								>
									Next Run
								</th>
								<th
									class="px-6 py-3 text-right text-xs font-semibold text-gray-700 dark:text-gray-300 uppercase tracking-wider"
								>
								</th>
							</tr>
						</thead>
						<tbody class="divide-y divide-gray-200 dark:divide-gray-700">
							{#each pipelines as pipeline}
								<ScheduleCard
									schedule={pipeline}
									onEdit={(s) => goto(`/pipelines/${s.id}/edit`)}
									onDelete={handleDeletePipeline}
									onTrigger={handleTriggerPipeline}
									isRunning={runningPipelines.has(pipeline.id)}
								/>
							{/each}
						</tbody>
					</table>
				</div>

				<!-- Pagination -->
				{#if pipelinesTotalPages > 1}
					<div class="flex items-center justify-between">
						<div class="text-sm text-gray-600 dark:text-gray-400">
							Page {pipelinesPage} of {pipelinesTotalPages}
						</div>

						<div class="flex items-center gap-2">
							<Button
								variant="clear"
								click={() => goToPipelinesPage(pipelinesPage - 1)}
								disabled={pipelinesPage === 1}
								icon="material-symbols:chevron-left"
								text="Previous"
							/>
							<Button
								variant="clear"
								click={() => goToPipelinesPage(pipelinesPage + 1)}
								disabled={pipelinesPage === pipelinesTotalPages}
								text="Next"
								icon="material-symbols:chevron-right"
							/>
						</div>
					</div>
				{/if}
			{/if}
		{/if}
	{:else if activeTab === 'history'}
		{#if loading}
			<div class="flex items-center justify-center py-12">
				<div
					class="animate-spin rounded-full h-8 w-8 border-b-2 border-earthy-terracotta-700"
				></div>
			</div>
		{:else if error}
			<div
				class="bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800/50 rounded-lg p-4"
			>
				<div class="flex">
					<IconifyIcon icon="material-symbols:error" class="h-5 w-5 text-red-400 mt-0.5" />
					<div class="ml-3">
						<h3 class="text-sm font-medium text-red-800 dark:text-red-200">Error</h3>
						<p class="mt-1 text-sm text-red-700 dark:text-red-300">{error}</p>
					</div>
				</div>
			</div>
		{:else if showGettingStarted}
			<GettingStarted />
		{:else}
			<!-- Filters -->
			<div
				class="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-4 mb-6"
			>
				<div class="grid grid-cols-1 md:grid-cols-2 gap-4">
					<!-- Status Filter -->
					<div class="relative">
						<button
							class="w-full flex items-center justify-between px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:outline-none focus:ring-1 focus:ring-earthy-terracotta-600 focus:border-earthy-terracotta-700"
							onclick={() => (showStatusDropdown = !showStatusDropdown)}
						>
							<span class="flex items-center">
								<IconifyIcon icon="material-symbols:filter-list" class="h-4 w-4 mr-2" />
								{selectedStatuses.length === 0
									? 'All Statuses'
									: selectedStatuses.length === 1
										? selectedStatuses[0].charAt(0).toUpperCase() + selectedStatuses[0].slice(1)
										: `${selectedStatuses.length} Statuses`}
							</span>
							<IconifyIcon icon="material-symbols:expand-more" class="h-4 w-4" />
						</button>

						{#if showStatusDropdown}
							<div
								class="absolute z-10 mt-1 w-full bg-white dark:bg-gray-700 shadow-lg max-h-60 rounded-md py-1 text-base ring-1 ring-black ring-opacity-5 overflow-auto"
							>
								{#each availableStatuses as status}
									<div
										class="cursor-default select-none relative py-2 pl-3 pr-9 hover:bg-gray-100 dark:hover:bg-gray-600"
										onclick={() => handleStatusToggle(status)}
										onkeydown={(e) => e.key === 'Enter' && handleStatusToggle(status)}
										role="button"
										tabindex="0"
									>
										<div class="flex items-center">
											<input
												type="checkbox"
												checked={selectedStatuses.includes(status)}
												class="h-4 w-4 text-earthy-terracotta-700 focus:ring-earthy-terracotta-600 border-gray-300 rounded"
												readonly
											/>
											<span class="ml-3 text-gray-900 dark:text-gray-100 capitalize">{status}</span>
										</div>
									</div>
								{/each}
							</div>
						{/if}
					</div>

					<!-- Clear Filters -->
					<Button variant="clear" click={resetFilters} text="Clear Filters" class="w-full" />
				</div>
			</div>

			<!-- Top Pagination -->
			{#if totalPages > 1}
				<div class="flex items-center justify-between mb-6">
					<div class="text-sm text-gray-600 dark:text-gray-400">
						Page {currentPage} of {totalPages}
					</div>

					<div class="flex items-center gap-2">
						{#if currentPage > 2}
							<Button variant="clear" click={() => goToPage(1)} text="1" />
							{#if currentPage > 3}
								<span class="text-gray-500">...</span>
							{/if}
						{/if}

						{#if currentPage > 1}
							<Button
								variant="clear"
								click={() => goToPage(currentPage - 1)}
								text={(currentPage - 1).toString()}
							/>
						{/if}

						<Button variant="filled" text={currentPage.toString()} disabled />

						{#if currentPage < totalPages}
							<Button
								variant="clear"
								click={() => goToPage(currentPage + 1)}
								text={(currentPage + 1).toString()}
							/>
						{/if}

						{#if currentPage < totalPages - 1}
							{#if currentPage < totalPages - 2}
								<span class="text-gray-500">...</span>
							{/if}
							<Button
								variant="clear"
								click={() => goToPage(totalPages)}
								text={totalPages.toString()}
							/>
						{/if}
					</div>

					<div class="flex items-center gap-2">
						<Button
							variant="clear"
							click={() => goToPage(currentPage - 1)}
							disabled={currentPage === 1}
							icon="material-symbols:chevron-left"
							text="Previous"
						/>
						<Button
							variant="clear"
							click={() => goToPage(currentPage + 1)}
							disabled={currentPage === totalPages}
							text="Next"
							icon="material-symbols:chevron-right"
						/>
					</div>
				</div>
			{/if}

			{#if runs.length === 0}
				<div class="text-center py-12">
					<IconifyIcon icon="material-symbols:sync" class="mx-auto h-12 w-12 text-gray-400 mb-4" />
					<h3 class="text-lg font-medium text-gray-900 dark:text-gray-100 mb-2">
						No Ingestion Runs
					</h3>
					<p class="text-gray-500 dark:text-gray-400">
						{selectedStatuses.length > 0
							? 'No runs match your current filters'
							: 'No ingestion runs have been executed yet'}
					</p>
				</div>
			{:else}
				<div class="mb-4">
					<p class="text-gray-600 dark:text-gray-400">
						Showing {runs.length} of {total} runs
						{selectedStatuses.length > 0 ? `with selected statuses` : ''}
					</p>
				</div>

				<div class="grid gap-4 mb-6">
					{#each runs as run}
						<IngestionRunCard {run} onClick={() => handleRunClick(run)} />
					{/each}
				</div>

				<!-- Bottom Pagination -->
				{#if totalPages > 1}
					<div class="flex items-center justify-between">
						<div class="text-sm text-gray-600 dark:text-gray-400">
							Page {currentPage} of {totalPages}
						</div>

						<div class="flex items-center gap-2">
							{#if currentPage > 2}
								<Button variant="clear" click={() => goToPage(1)} text="1" />
								{#if currentPage > 3}
									<span class="text-gray-500">...</span>
								{/if}
							{/if}

							{#if currentPage > 1}
								<Button
									variant="clear"
									click={() => goToPage(currentPage - 1)}
									text={(currentPage - 1).toString()}
								/>
							{/if}

							<Button variant="filled" text={currentPage.toString()} disabled />

							{#if currentPage < totalPages}
								<Button
									variant="clear"
									click={() => goToPage(currentPage + 1)}
									text={(currentPage + 1).toString()}
								/>
							{/if}

							{#if currentPage < totalPages - 1}
								{#if currentPage < totalPages - 2}
									<span class="text-gray-500">...</span>
								{/if}
								<Button
									variant="clear"
									click={() => goToPage(totalPages)}
									text={totalPages.toString()}
								/>
							{/if}
						</div>

						<div class="flex items-center gap-2">
							<Button
								variant="clear"
								click={() => goToPage(currentPage - 1)}
								disabled={currentPage === 1}
								icon="material-symbols:chevron-left"
								text="Previous"
							/>
							<Button
								variant="clear"
								click={() => goToPage(currentPage + 1)}
								disabled={currentPage === totalPages}
								text="Next"
								icon="material-symbols:chevron-right"
							/>
						</div>
					</div>
				{/if}
			{/if}

			{#if showStatusDropdown}
				<div
					class="fixed inset-0 z-5"
					onclick={() => {
						showStatusDropdown = false;
					}}
					onkeydown={(e) => e.key === 'Escape' && (showStatusDropdown = false)}
					role="button"
					tabindex="-1"
				></div>
			{/if}
		{/if}
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

<Toast bind:show={showToast} message={toastMessage} variant={toastVariant} />
