<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { browser } from '$app/environment';
	import { fetchApi } from '$lib/api';
	import Button from '../../components/Button.svelte';
	import IconifyIcon from '@iconify/svelte';
	import IngestionRunCard from '../../components/IngestionRunCard.svelte';
	import IngestionRunModal from '../../components/IngestionRunModal.svelte';
	import GettingStarted from '../../components/GettingStarted.svelte';

	interface IngestionRunSummary {
		assets_created: number;
		assets_updated: number;
		assets_deleted: number;
		errors: number;
	}

	interface IngestionRun {
		id: string;
		pipeline_name: string;
		source_name: string;
		run_id: string;
		status: 'running' | 'completed' | 'failed' | 'cancelled';
		started_at: string;
		completed_at?: string;
		error_message?: string;
		config?: any;
		summary?: IngestionRunSummary;
		created_by: string;
	}

	interface IngestionRunsResponse {
		runs: IngestionRun[];
		total: number;
		limit: number;
		offset: number;
		pipelines: string[];
	}

	let runs = $state<IngestionRun[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let total = $state(0);
	let currentPage = $state(1);
	let pageSize = $state(10);
	let selectedPipelines = $state<string[]>([]);
	let selectedStatuses = $state<string[]>([]);
	let pipelineSearchQuery = $state('');
	let selectedRun = $state<IngestionRun | null>(null);
	let showRunModal = $state(false);
	let showPipelineDropdown = $state(false);
	let showStatusDropdown = $state(false);
	let availablePipelines = $state<string[]>([]);

	let totalPages = $derived(Math.ceil(total / pageSize));
	let offset = $derived((currentPage - 1) * pageSize);

	let filteredPipelines = $derived(
		availablePipelines.filter(
			(pipeline) =>
				!pipelineSearchQuery || pipeline.toLowerCase().includes(pipelineSearchQuery.toLowerCase())
		)
	);

	let showGettingStarted = $derived(
		!loading &&
			runs &&
			runs.length === 0 &&
			selectedPipelines.length === 0 &&
			selectedStatuses.length === 0
	);

	const availableStatuses = ['running', 'completed', 'failed', 'cancelled'];

	$effect(() => {
		if (browser) {
			const urlParams = $page.url.searchParams;
			const pageParam = urlParams.get('page');
			const pipelinesParam = urlParams.get('pipelines');
			const statusesParam = urlParams.get('statuses');
			const runParam = urlParams.get('run');

			if (pageParam) {
				const pageNum = parseInt(pageParam);
				if (pageNum > 0) currentPage = pageNum;
			}

			if (pipelinesParam) {
				selectedPipelines = pipelinesParam.split(',').filter((p) => p.trim());
			}

			if (statusesParam) {
				selectedStatuses = statusesParam.split(',').filter((s) => s.trim());
			}

			if (runParam && runs.length > 0) {
				const foundRun = runs.find((r) => r.id === runParam || r.run_id === runParam);
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

	function updateUrl() {
		if (!browser) return;

		const url = new URL($page.url);

		if (currentPage > 1) {
			url.searchParams.set('page', currentPage.toString());
		} else {
			url.searchParams.delete('page');
		}

		if (selectedPipelines.length > 0) {
			url.searchParams.set('pipelines', selectedPipelines.join(','));
		} else {
			url.searchParams.delete('pipelines');
		}

		if (selectedStatuses.length > 0) {
			url.searchParams.set('statuses', selectedStatuses.join(','));
		} else {
			url.searchParams.delete('statuses');
		}

		goto(url.toString(), { replaceState: true, noScroll: true });
	}

	async function fetchRuns() {
		try {
			loading = true;
			error = null;

			const params = new URLSearchParams({
				limit: pageSize.toString(),
				offset: offset.toString()
			});

			if (selectedPipelines.length > 0) {
				params.append('pipelines', selectedPipelines.join(','));
			}

			if (selectedStatuses.length > 0) {
				params.append('statuses', selectedStatuses.join(','));
			}

			const response = await fetchApi(`/runs?${params}`);
			if (!response.ok) {
				throw new Error('Failed to fetch ingestion runs');
			}

			const data: IngestionRunsResponse = await response.json();
			runs = data.runs || [];
			total = data.total || 0;
			availablePipelines = data.pipelines || [];
		} catch (err) {
			console.error('Error fetching ingestion runs:', err);
			error = err instanceof Error ? err.message : 'Failed to load ingestion runs';
		} finally {
			loading = false;
		}
	}

	function goToPage(page: number) {
		if (page >= 1 && page <= totalPages) {
			currentPage = page;
			updateUrl();
			fetchRuns();
		}
	}

	function handlePipelineToggle(pipeline: string) {
		if (selectedPipelines.includes(pipeline)) {
			selectedPipelines = selectedPipelines.filter((p) => p !== pipeline);
		} else {
			selectedPipelines = [...selectedPipelines, pipeline];
		}
		currentPage = 1;
		updateUrl();
		fetchRuns();
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
		selectedPipelines = [];
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

	onMount(() => {
		fetchRuns();
	});
</script>

<div class="container max-w-7xl mx-auto py-6 px-4 sm:px-6 lg:px-8">
	<div class="flex justify-between items-center mb-8">
		<div>
			<h1 class="text-2xl font-bold text-gray-900 dark:text-gray-100">Runs</h1>
			<p class="text-gray-600 dark:text-gray-400 mt-1">
				Monitor and track Marmot ingest jobs running from the CLI
			</p>
		</div>

		{#if !showGettingStarted}
			<div class="flex items-center gap-4">
				<Button
					variant="clear"
					click={resetFilters}
					icon="material-symbols:refresh"
					text="Refresh"
				/>
			</div>
		{/if}
	</div>

	{#if loading}
		<div class="flex items-center justify-center py-12">
			<div class="animate-spin rounded-full h-8 w-8 border-b-2 border-earthy-terracotta-700"></div>
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
			<div class="grid grid-cols-1 md:grid-cols-3 gap-4">
				<!-- Pipeline Filter -->
				<div class="relative">
					<button
						class="w-full flex items-center justify-between px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:outline-none focus:ring-1 focus:ring-earthy-terracotta-600 focus:border-earthy-terracotta-700"
						onclick={() => (showPipelineDropdown = !showPipelineDropdown)}
					>
						<span class="flex items-center">
							<IconifyIcon icon="material-symbols:data-exploration" class="h-4 w-4 mr-2" />
							{selectedPipelines.length === 0
								? 'All Pipelines'
								: selectedPipelines.length === 1
									? selectedPipelines[0]
									: `${selectedPipelines.length} Pipelines`}
						</span>
						<IconifyIcon icon="material-symbols:expand-more" class="h-4 w-4" />
					</button>

					{#if showPipelineDropdown}
						<div
							class="absolute z-10 mt-1 w-full bg-white dark:bg-gray-700 shadow-lg max-h-60 rounded-md py-1 text-base ring-1 ring-black ring-opacity-5 overflow-auto"
						>
							<div class="px-3 py-2">
								<input
									type="text"
									placeholder="Search pipelines..."
									bind:value={pipelineSearchQuery}
									class="w-full px-2 py-1 text-sm border border-gray-300 dark:border-gray-600 rounded bg-white dark:bg-gray-600 text-gray-900 dark:text-gray-100"
								/>
							</div>
							{#each filteredPipelines as pipeline}
								<div
									class="cursor-default select-none relative py-2 pl-3 pr-9 hover:bg-gray-100 dark:hover:bg-gray-600"
									onclick={() => handlePipelineToggle(pipeline)}
								>
									<div class="flex items-center">
										<input
											type="checkbox"
											checked={selectedPipelines.includes(pipeline)}
											class="h-4 w-4 text-earthy-terracotta-700 focus:ring-earthy-terracotta-600 border-gray-300 rounded"
											readonly
										/>
										<span class="ml-3 text-gray-900 dark:text-gray-100">{pipeline}</span>
									</div>
								</div>
							{/each}
						</div>
					{/if}
				</div>

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
				<h3 class="text-lg font-medium text-gray-900 dark:text-gray-100 mb-2">No Ingestion Runs</h3>
				<p class="text-gray-500 dark:text-gray-400">
					{selectedStatuses.length > 0 || selectedPipelines.length > 0
						? 'No runs match your current filters'
						: 'No ingestion runs have been executed yet'}
				</p>
			</div>
		{:else}
			<div class="mb-4">
				<p class="text-gray-600 dark:text-gray-400">
					Showing {runs.length} of {total} runs
					{selectedPipelines.length > 0 ? `for selected pipelines` : ''}
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
	{/if}

	{#if showPipelineDropdown || showStatusDropdown}
		<div
			class="fixed inset-0 z-5"
			onclick={() => {
				showPipelineDropdown = false;
				showStatusDropdown = false;
			}}
		></div>
	{/if}
</div>

{#if selectedRun}
	<IngestionRunModal bind:show={showRunModal} run={selectedRun} onClose={handleModalClose} />
{/if}
