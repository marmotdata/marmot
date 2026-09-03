<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { SvelteSet, SvelteURLSearchParams } from 'svelte/reactivity';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { browser } from '$app/environment';
	import { fetchApi } from '$lib/api';
	import { websocketService, type JobRunEvent } from '$lib/websocket';
	import { auth } from '$lib/stores/auth';
	import { toasts } from '$lib/stores/toast';
	import { encryptionConfigured, allowUnencrypted } from '$lib/stores/encryption';
	import Button from '$components/ui/Button.svelte';
	import IconifyIcon from '@iconify/svelte';
	import IngestionRunModal from '$components/runs/IngestionRunModal.svelte';
	import ScheduleCard from '$components/runs/ScheduleCard.svelte';
	import ConfirmModal from '$components/ui/ConfirmModal.svelte';
	import SessionsView from '$components/gateway/SessionsView.svelte';
	import AuditView from '$components/gateway/AuditView.svelte';
	import { getStatusColor, getStatusIcon } from '$lib/utils/status';
	import { formatRelativeTime } from '$lib/utils/format';

	let canManageIngestion = $derived(auth.hasPermission('ingestion', 'manage'));
	let canViewGateway = $derived(auth.hasPermission('gateway', 'view'));

	let unsubscribe: (() => void) | null = null;
	let fetchRunsTimeout: ReturnType<typeof setTimeout> | null = null;
	let wsConnected = $state(false);
	let wsCheckInterval: ReturnType<typeof setInterval> | null = null;

	type Tab = 'pipelines' | 'sessions' | 'audit';

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
		config: Record<string, unknown>;
		cron_expression: string;
		enabled: boolean;
		queryable?: boolean;
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

	let pipelines = $state<Pipeline[]>([]);
	let pipelinesLoading = $state(false);
	let pipelinesError = $state<string | null>(null);
	let pipelinesTotal = $state(0);
	let pipelinesPage = $state(1);
	let pipelinesPageSize = $state(10);

	let showConfirmModal = $state(false);
	let confirmModalTitle = $state('');
	let confirmModalMessage = $state('');
	let confirmModalCheckboxLabel = $state('');
	let confirmModalCheckboxChecked = $state(false);
	let confirmModalAction = $state<((checkboxValue?: boolean) => void) | null>(null);

	let runningPipelines: SvelteSet<string> = new SvelteSet();

	// Per-source run history, revealed by expanding a source row.
	let expandedSourceId = $state<string | null>(null);
	let sourceRuns = $state<Record<string, IngestionRun[]>>({});
	let sourceRunsLoading = $state<Record<string, boolean>>({});

	async function toggleSourceExpand(schedule: { id: string }) {
		if (expandedSourceId === schedule.id) {
			expandedSourceId = null;
			return;
		}
		expandedSourceId = schedule.id;
		await fetchSourceRuns(schedule.id);
	}

	async function fetchSourceRuns(scheduleId: string) {
		sourceRunsLoading = { ...sourceRunsLoading, [scheduleId]: true };
		try {
			const response = await fetchApi(`/ingestion/runs?schedule_id=${scheduleId}&limit=8`);
			if (!response.ok) throw new Error('Failed to fetch runs');
			const data: IngestionRunsResponse = await response.json();
			sourceRuns = { ...sourceRuns, [scheduleId]: data.runs || [] };
		} catch (err) {
			console.error('Error fetching source runs:', err);
			sourceRuns = { ...sourceRuns, [scheduleId]: [] };
		} finally {
			sourceRunsLoading = { ...sourceRunsLoading, [scheduleId]: false };
		}
	}

	let totalPages = $derived(Math.ceil(total / pageSize));
	let offset = $derived((currentPage - 1) * pageSize);
	let pipelinesTotalPages = $derived(Math.ceil(pipelinesTotal / pipelinesPageSize));
	let pipelinesOffset = $derived((pipelinesPage - 1) * pipelinesPageSize);

	const availableStatuses = ['pending', 'claimed', 'running', 'succeeded', 'failed', 'cancelled'];

	$effect(() => {
		if (browser) {
			const urlParams = $page.url.searchParams;
			const tabParam = urlParams.get('tab');
			const pageParam = urlParams.get('page');
			const statusesParam = urlParams.get('statuses');
			const runParam = urlParams.get('run');

			if (
				tabParam === 'pipelines' ||
				tabParam === 'sessions' ||
				tabParam === 'audit'
			) {
				activeTab = tabParam;
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
		url.searchParams.set('tab', tab);
		if (tab === 'pipelines') {
			fetchPipelines();
		}
		goto(resolve(`/runs${url.search}`), { replaceState: true, noScroll: true });
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

		goto(resolve(`/runs${url.search}`), { replaceState: true, noScroll: true });
	}

	async function fetchRuns(showLoading: boolean = true) {
		try {
			if (showLoading) {
				loading = true;
			}
			error = null;

			const params = new SvelteURLSearchParams({
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
		goto(resolve(`/runs${url.search}`), { replaceState: true, noScroll: true });
	}

	function handleModalClose() {
		const url = new URL($page.url);
		url.searchParams.delete('run');
		goto(resolve(`/runs${url.search}`), { replaceState: true, noScroll: true });
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
			// Add to running set immediately - SvelteSet is reactive on mutation
			runningPipelines.add(pipeline.id);

			const response = await fetchApi(`/ingestion/schedules/${pipeline.id}/trigger`, {
				method: 'POST'
			});

			if (!response.ok) {
				const data = await response.json();
				// Remove from running set on error
				runningPipelines.delete(pipeline.id);
				throw new Error(data.error || 'Failed to trigger pipeline');
			}

			// Poll for running status
			pollPipelineStatus(pipeline.id);
		} catch (err) {
			const errorMsg = err instanceof Error ? err.message : 'Failed to trigger pipeline';
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
						toasts.success('Pipeline completed successfully!');
					} else if (latestRun.status === 'failed') {
						toasts.error(`Pipeline failed: ${latestRun.error_message || 'Unknown error'}`);
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

				const successMsg = teardown
					? `Pipeline "${pipeline.name}" and all its assets deleted successfully`
					: `Pipeline "${pipeline.name}" deleted successfully`;
				toasts.success(successMsg);

				// Refresh the list
				fetchPipelines();
			} catch (err) {
				const errorMsg = err instanceof Error ? err.message : 'Failed to delete pipeline';
				toasts.error(errorMsg);
			}
		};
		showConfirmModal = true;
	}

	function handleJobRunEvent(event: JobRunEvent) {
		const jobRun = event.payload;

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

		// Keep the expanded source's run list fresh as its jobs progress.
		if (jobRun.schedule_id && expandedSourceId === jobRun.schedule_id) {
			fetchSourceRuns(jobRun.schedule_id);
		}

		// The rest maintains the legacy flat runs list, unused by the current UI.
		if (activeTab !== 'pipelines') return;

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
			case 'job_run_cancelled': {
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
		}

		// Update selected run if it's the one being shown
		if (selectedRun && selectedRun.id === jobRun.id) {
			selectedRun = jobRun;
		}
	}

	onMount(() => {
		if (activeTab === 'pipelines') {
			fetchPipelines();
		}

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
		<h1 class="text-2xl font-bold text-gray-900 dark:text-gray-100">Plugins</h1>
		<p class="text-gray-600 dark:text-gray-400 mt-1">
			Sources Marmot catalogs and lets agents query, and the agent activity that flows through them
		</p>
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
					icon="material-symbols:hub-outline"
					class="inline-block h-5 w-5 mr-2 -mt-0.5"
				/>
				Sources
			</button>
			{#if canViewGateway}
				<button
					onclick={() => switchTab('sessions')}
					class="whitespace-nowrap pb-4 px-1 border-b-2 font-medium text-sm transition-colors {activeTab ===
					'sessions'
						? 'border-earthy-terracotta-700 text-earthy-terracotta-700'
						: 'border-transparent text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300 hover:border-gray-300 dark:hover:border-gray-600'}"
				>
					<IconifyIcon
						icon="material-symbols:badge-outline"
						class="inline-block h-5 w-5 mr-2 -mt-0.5"
					/>
					Sessions
				</button>
				<button
					onclick={() => switchTab('audit')}
					class="whitespace-nowrap pb-4 px-1 border-b-2 font-medium text-sm transition-colors {activeTab ===
					'audit'
						? 'border-earthy-terracotta-700 text-earthy-terracotta-700'
						: 'border-transparent text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300 hover:border-gray-300 dark:hover:border-gray-600'}"
				>
					<IconifyIcon
						icon="material-symbols:receipt-long-outline"
						class="inline-block h-5 w-5 mr-2 -mt-0.5"
					/>
					Query Audit
				</button>
			{/if}
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
							<h3 class="text-sm font-medium text-red-800 dark:text-red-200">Unencrypted mode</h3>
							<p class="mt-1 text-sm text-red-700 dark:text-red-300">
								Pipeline credentials are stored in plaintext. This should only be used for
								development.
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
								Encryption key not configured
							</h3>
							<p class="mt-1 text-sm text-amber-700 dark:text-amber-300">
								Pipeline creation, editing, and triggering are disabled until an encryption key is
								set.
							</p>
							<div class="mt-3 text-sm text-amber-700 dark:text-amber-300 space-y-2">
								<p>To get started, install the CLI and generate a key:</p>
								<div
									class="bg-amber-100 dark:bg-amber-900/40 rounded-md px-3 py-2 font-mono text-xs space-y-1"
								>
									<p>curl -fsSL get.marmotdata.io | sh</p>
									<p>marmot generate-encryption-key</p>
								</div>
								<p>
									Then set <code
										class="px-1 py-0.5 bg-amber-100 dark:bg-amber-900/40 rounded text-xs font-mono"
										>MARMOT_SERVER_ENCRYPTION_KEY</code
									>
									and restart the server. See the
									<a
										href="https://marmotdata.io/docs/Deploy/"
										target="_blank"
										rel="noopener noreferrer"
										class="underline font-medium hover:text-amber-900 dark:hover:text-amber-100"
										>deploy docs</a
									> for details.
								</p>
							</div>
						</div>
					</div>
				</div>
			{/if}

			<!-- Header Actions -->
			{#if canManageIngestion}
				<div class="flex justify-end items-center mb-6">
					<Button
						variant="filled"
						click={() => goto(resolve('/pipelines/new'))}
						icon="material-symbols:add"
						text="Add source"
						disabled={!$encryptionConfigured}
					/>
				</div>
			{/if}

			{#if pipelines.length === 0}
				<div class="text-center py-12">
					<IconifyIcon
						icon="material-symbols:account-tree"
						class="mx-auto h-12 w-12 text-gray-400 mb-4"
					/>
					<h3 class="text-lg font-medium text-gray-900 dark:text-gray-100 mb-2">No sources yet</h3>
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
							click={() => goto(resolve('/pipelines/new'))}
							icon="material-symbols:add"
							text="Add source"
							disabled={!$encryptionConfigured}
						/>
					{/if}
				</div>
			{:else}
				<div class="mb-4">
					<p class="text-gray-600 dark:text-gray-400">
						Showing {pipelines.length} of {pipelinesTotal} sources
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
									Source
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
							{#each pipelines as pipeline (pipeline.id)}
								<ScheduleCard
									schedule={pipeline}
									onEdit={$encryptionConfigured
										? (s) => goto(resolve(`/pipelines/${s.id}/edit`))
										: undefined}
									onDelete={handleDeletePipeline}
									onTrigger={$encryptionConfigured ? handleTriggerPipeline : undefined}
									onToggleExpand={toggleSourceExpand}
									expanded={expandedSourceId === pipeline.id}
									isRunning={runningPipelines.has(pipeline.id)}
								/>
								{#if expandedSourceId === pipeline.id}
									<tr class="bg-gray-50 dark:bg-gray-900/40">
										<td colspan="7" class="px-6 py-4">
											<div class="flex items-center justify-between mb-3">
												<h4
													class="text-xs font-semibold uppercase tracking-wider text-gray-500 dark:text-gray-400"
												>
													Recent runs
												</h4>
											</div>
											{#if sourceRunsLoading[pipeline.id]}
												<div class="flex items-center justify-center py-6">
													<div
														class="animate-spin rounded-full h-5 w-5 border-b-2 border-earthy-terracotta-700"
													></div>
												</div>
											{:else if (sourceRuns[pipeline.id] || []).length === 0}
												<p class="text-sm text-gray-500 dark:text-gray-400 py-2">
													No runs yet. Trigger one with the play button, or it will run on its schedule.
												</p>
											{:else}
												<div class="space-y-1">
													{#each sourceRuns[pipeline.id] as run (run.id)}
														<button
															class="w-full flex items-center gap-4 text-left px-3 py-2 rounded-lg hover:bg-white dark:hover:bg-gray-800 transition-colors"
															onclick={() => handleRunClick(run)}
														>
															<span
																class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium {getStatusColor(
																	run.status
																)}"
															>
																<IconifyIcon
																	icon={getStatusIcon(run.status)}
																	class="h-3.5 w-3.5 mr-1.5"
																/>
																{run.status.charAt(0).toUpperCase() + run.status.slice(1)}
															</span>
															<span class="text-sm text-gray-600 dark:text-gray-400 tabular-nums">
																{formatRelativeTime(run.created_at)}
															</span>
															<span class="text-xs text-gray-500 dark:text-gray-500 ml-auto">
																{run.assets_created} created · {run.assets_updated} updated
															</span>
														</button>
													{/each}
												</div>
											{/if}
										</td>
									</tr>
								{/if}
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
	{:else if activeTab === 'sessions'}
		<SessionsView />
	{:else if activeTab === 'audit'}
		<AuditView />
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
