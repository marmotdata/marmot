<script lang="ts">
	import { fade, fly } from 'svelte/transition';
	import IconifyIcon from '@iconify/svelte';
	import Button from './Button.svelte';
	import MetadataView from './MetadataView.svelte';
	import { fetchApi } from '$lib/api';

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
		status: 'running' | 'success' | 'failed' | 'cancelled';
		started_at: string;
		completed_at?: string;
		error_message?: string;
		config?: any;
		summary?: IngestionRunSummary;
		created_by: string;
	}

	interface RunEntity {
		id: string;
		run_id: string;
		entity_type: string;
		entity_mrn: string;
		entity_name?: string;
		status: string;
		error_message?: string;
		created_at: string;
	}

	interface RunEntitiesResponse {
		entities: RunEntity[];
		total: number;
		limit: number;
		offset: number;
	}

	let {
		run,
		show = $bindable(false),
		onClose
	}: {
		run: IngestionRun;
		show: boolean;
		onClose: () => void;
	} = $props();

	let showRawConfig = $state(false);
	let entities = $state<RunEntity[]>([]);
	let entitiesTotal = $state(0);
	let entitiesPage = $state(1);
	let entitiesLimit = $state(10);
	let entitiesLoading = $state(false);
	let entitiesError = $state<string | null>(null);

	$effect(() => {
		if (show && run?.id) {
			loadEntities();
		}
	});

	async function loadEntities() {
		entitiesLoading = true;
		entitiesError = null;

		try {
			const params = new URLSearchParams({
				limit: entitiesLimit.toString(),
				offset: ((entitiesPage - 1) * entitiesLimit).toString()
			});

			const response = await fetchApi(`/runs/entities/${run.id}?${params}`);
			if (!response.ok) throw new Error('Failed to fetch entities');

			const data: RunEntitiesResponse = await response.json();
			entities = data.entities || [];
			entitiesTotal = data.total || 0;
		} catch (err) {
			entitiesError = err instanceof Error ? err.message : 'Failed to load entities';
		} finally {
			entitiesLoading = false;
		}
	}

	function goToEntitiesPage(page: number) {
		const totalPages = Math.ceil(entitiesTotal / entitiesLimit);
		if (page >= 1 && page <= totalPages) {
			entitiesPage = page;
			loadEntities();
		}
	}

	function getStatusColor(status: string): string {
		const colors: Record<string, string> = {
			running: 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900/30 dark:text-yellow-300',
			success: 'bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-300',
			failed: 'bg-red-100 text-red-800 dark:bg-red-900/30 dark:text-red-300',
			cancelled: 'bg-gray-100 text-gray-800 dark:bg-gray-900/30 dark:text-gray-300',
			created: 'bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-300',
			updated: 'bg-blue-100 text-blue-800 dark:bg-blue-900/30 dark:text-blue-300',
			deleted: 'bg-red-100 text-red-800 dark:bg-red-900/30 dark:text-red-300',
			unchanged: 'bg-cyan-100 text-cyan-800 dark:bg-cyan-900/30 dark:text-cyan-300'
		};
		return colors[status] || 'bg-gray-100 text-gray-800 dark:bg-gray-900/30 dark:text-gray-300';
	}

	function getStatusIcon(status: string): string {
		const icons: Record<string, string> = {
			running: 'material-symbols:sync',
			success: 'material-symbols:check-circle',
			failed: 'material-symbols:error',
			cancelled: 'material-symbols:cancel',
			created: 'material-symbols:add-circle',
			updated: 'material-symbols:edit',
			deleted: 'material-symbols:delete',
			unchanged: 'material-symbols:check'
		};
		return icons[status] || 'material-symbols:help';
	}

	function formatDuration(startedAt: string, completedAt?: string): string {
		const start = new Date(startedAt);
		const end = completedAt ? new Date(completedAt) : new Date();
		const durationMs = end.getTime() - start.getTime();

		const seconds = Math.floor(durationMs / 1000);
		const minutes = Math.floor(seconds / 60);
		const hours = Math.floor(minutes / 60);

		if (hours > 0) {
			return `${hours}h ${minutes % 60}m ${seconds % 60}s`;
		} else if (minutes > 0) {
			return `${minutes}m ${seconds % 60}s`;
		} else if (seconds > 0) {
			return `${seconds}s`;
		} else {
			return `${durationMs}ms`;
		}
	}

	function getAssetUrl(entity: RunEntity): string {
		if (entity.entity_type !== 'asset') return '';

		const parts = entity.entity_mrn.split('://');
		if (parts.length !== 2) return '';

		const [, rest] = parts;
		const pathParts = rest.split('/');
		if (pathParts.length < 3) return '';

		const type = pathParts[0];
		const name = pathParts.slice(2).join('/');

		return `/assets/${encodeURIComponent(type)}/${encodeURIComponent(entity.entity_name)}`;
	}

	function shouldShowAssetLink(entity: RunEntity): boolean {
		return (
			entity.entity_type === 'asset' &&
			(entity.status === 'created' || entity.status === 'updated' || entity.status === 'unchanged')
		);
	}

	function formatDateTime(dateString: string): string {
		return new Date(dateString).toLocaleString();
	}

	function handleBackdropClick(event: MouseEvent) {
		if (event.target === event.currentTarget) {
			show = false;
			onClose();
		}
	}

	function closeModal() {
		show = false;
		onClose();
	}
</script>

{#if show}
	<div
		class="fixed inset-0 bg-black/20 backdrop-blur-lg z-50 flex items-start justify-center p-2 overflow-y-auto"
		style="backdrop-filter: blur(8px);"
		transition:fade={{ duration: 200 }}
		onclick={handleBackdropClick}
		role="dialog"
		aria-modal="true"
	>
		<div
			class="bg-white dark:bg-gray-800 rounded-lg shadow-xl my-8 overflow-hidden relative max-w-7xl w-full max-h-[90vh]"
			transition:fly={{ y: 20, duration: 200 }}
		>
			<div
				class="flex items-center justify-between p-6 border-b border-gray-200 dark:border-gray-700"
			>
				<div class="flex items-center space-x-3">
					<IconifyIcon
						icon="material-symbols:sync"
						class="h-6 w-6 text-orange-600 dark:text-orange-400"
					/>
					<div>
						<div class="flex items-center space-x-2">
							<h2 class="text-xl font-bold text-gray-900 dark:text-gray-100">
								{run.pipeline_name}
							</h2>
							{#if run.source_name === 'destroy'}
								<span
									class="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium bg-red-100 text-red-800 dark:bg-red-900/30 dark:text-red-300"
								>
									<IconifyIcon icon="material-symbols:delete-forever" class="w-3 h-3 mr-1" />
									Teardown
								</span>
							{/if}
						</div>
						<p class="text-sm text-gray-600 dark:text-gray-400">Ingestion Run Details</p>
					</div>
				</div>
				<button
					onclick={closeModal}
					class="p-2 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg transition-colors"
				>
					<IconifyIcon
						icon="material-symbols:close"
						class="h-5 w-5 text-gray-500 dark:text-gray-400"
					/>
				</button>
			</div>

			<div class="overflow-y-auto max-h-[calc(90vh-140px)]">
				<div class="p-6 space-y-6">
					<div class="grid grid-cols-1 lg:grid-cols-3 gap-6">
						<div class="lg:col-span-2 space-y-6">
							<div class="bg-gray-50 dark:bg-gray-900 rounded-lg p-4">
								<h3 class="text-lg font-semibold text-gray-900 dark:text-gray-100 mb-4">
									Run Information
								</h3>
								<div class="grid grid-cols-2 gap-4 text-sm">
									<div>
										<dt class="font-medium text-gray-500 dark:text-gray-400">Status</dt>
										<dd class="mt-1">
											<span
												class="inline-flex items-center px-2 py-1 rounded-full text-xs font-medium {getStatusColor(
													run.status
												)}"
											>
												<IconifyIcon
													icon={getStatusIcon(run.status)}
													class="w-3 h-3 mr-1 {run.status === 'running' ? 'animate-spin' : ''}"
												/>
												{run.status.charAt(0).toUpperCase() + run.status.slice(1)}
											</span>
										</dd>
									</div>
									<div>
										<dt class="font-medium text-gray-500 dark:text-gray-400">Source</dt>
										<dd class="mt-1 text-gray-900 dark:text-gray-100">{run.source_name}</dd>
									</div>
									<div>
										<dt class="font-medium text-gray-500 dark:text-gray-400">Duration</dt>
										<dd class="mt-1 text-gray-900 dark:text-gray-100">
											{formatDuration(run.started_at, run.completed_at)}
										</dd>
									</div>
									<div>
										<dt class="font-medium text-gray-500 dark:text-gray-400">Created By</dt>
										<dd class="mt-1 text-gray-900 dark:text-gray-100">{run.created_by}</dd>
									</div>
								</div>
							</div>

							{#if run.error_message}
								<div
									class="bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800/50 rounded-lg p-4"
								>
									<h3
										class="text-lg font-semibold text-red-800 dark:text-red-200 mb-2 flex items-center"
									>
										<IconifyIcon icon="material-symbols:error" class="h-5 w-5 mr-2" />
										Error Details
									</h3>
									<pre
										class="text-sm text-red-700 dark:text-red-300 whitespace-pre-wrap font-mono bg-red-100 dark:bg-red-900/40 p-3 rounded overflow-x-auto">{run.error_message}</pre>
								</div>
							{/if}
						</div>

						<div class="space-y-6">
							{#if run.summary}
								<div class="bg-gray-50 dark:bg-gray-900 rounded-lg p-4">
									<h3 class="text-lg font-semibold text-gray-900 dark:text-gray-100 mb-4">
										Summary
									</h3>
									<div class="space-y-3">
										<div class="flex justify-between items-center">
											<span class="text-sm text-gray-600 dark:text-gray-400">Created</span>
											<span class="text-lg font-semibold text-green-600 dark:text-green-400"
												>{run.summary.assets_created}</span
											>
										</div>
										<div class="flex justify-between items-center">
											<span class="text-sm text-gray-600 dark:text-gray-400">Updated</span>
											<span class="text-lg font-semibold text-blue-600 dark:text-blue-400"
												>{run.summary.assets_updated}</span
											>
										</div>
										<div class="flex justify-between items-center">
											<span class="text-sm text-gray-600 dark:text-gray-400">Deleted</span>
											<span class="text-lg font-semibold text-orange-600 dark:text-orange-400"
												>{run.summary.assets_deleted}</span
											>
										</div>
										<div class="flex justify-between items-center">
											<span class="text-sm text-gray-600 dark:text-gray-400">Errors</span>
											<span class="text-lg font-semibold text-red-600 dark:text-red-400"
												>{run.summary.errors}</span
											>
										</div>
									</div>
								</div>
							{/if}
						</div>
					</div>

					{#if run.config && Object.keys(run.config).length > 0}
						<div class="bg-gray-50 dark:bg-gray-900 rounded-lg p-4">
							<div class="flex items-center justify-between mb-4">
								<h3
									class="text-lg font-semibold text-gray-900 dark:text-gray-100 flex items-center"
								>
									<IconifyIcon icon="material-symbols:settings" class="h-5 w-5 mr-2" />
									Configuration
								</h3>
								<Button
									variant="clear"
									text={showRawConfig ? 'Hide Raw' : 'Show Raw'}
									icon="code"
									click={() => (showRawConfig = !showRawConfig)}
								/>
							</div>
							<div
								class="bg-white dark:bg-gray-800 rounded border border-gray-200 dark:border-gray-700 p-4 max-h-64 overflow-y-auto"
							>
								{#if showRawConfig}
									<pre
										class="text-sm text-gray-800 dark:text-gray-200 font-mono whitespace-pre-wrap">{JSON.stringify(
											run.config,
											null,
											2
										)}</pre>
								{:else}
									<MetadataView metadata={run.config} maxDepth={3} />
								{/if}
							</div>
						</div>
					{/if}

					<div>
						<div class="flex items-center justify-between mb-4">
							<h3 class="text-lg font-semibold text-gray-900 dark:text-gray-100">
								Entities ({entitiesTotal})
							</h3>
							<div class="flex items-center space-x-2">
								<Button
									variant="clear"
									click={() => goToEntitiesPage(entitiesPage - 1)}
									disabled={entitiesPage === 1}
									icon="chevron-left"
									text="Previous"
								/>
								<span class="text-sm text-gray-600 dark:text-gray-400">
									{entitiesPage} / {Math.ceil(entitiesTotal / entitiesLimit)}
								</span>
								<Button
									variant="clear"
									click={() => goToEntitiesPage(entitiesPage + 1)}
									disabled={entitiesPage === Math.ceil(entitiesTotal / entitiesLimit)}
									text="Next"
									icon="chevron-right"
								/>
							</div>
						</div>

						{#if entitiesLoading}
							<div class="flex items-center justify-center py-8">
								<div class="animate-spin rounded-full h-6 w-6 border-b-2 border-orange-600"></div>
							</div>
						{:else if entitiesError}
							<div
								class="bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800/50 rounded-lg p-4"
							>
								<p class="text-red-700 dark:text-red-300">{entitiesError}</p>
							</div>
						{:else if entities.length === 0}
							<div class="text-center py-8">
								<p class="text-gray-500 dark:text-gray-400">No entities found</p>
							</div>
						{:else}
							<div
								class="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg overflow-hidden"
							>
								<div class="overflow-x-auto">
									<table class="min-w-full divide-y divide-gray-200 dark:divide-gray-700">
										<thead class="bg-gray-50 dark:bg-gray-900">
											<tr>
												<th
													class="px-3 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider"
													>Entity</th
												>
												<th
													class="px-3 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider"
													>Type</th
												>
												<th
													class="px-3 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider"
													>Status</th
												>
												<th
													class="px-3 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider"
													>Time</th
												>
											</tr>
										</thead>
										<tbody
											class="bg-white dark:bg-gray-800 divide-y divide-gray-200 dark:divide-gray-700"
										>
											{#each entities as entity}
												<tr class="hover:bg-gray-50 dark:hover:bg-gray-750">
													<td class="px-3 py-2">
														<div class="max-w-xs flex items-center space-x-2">
															<div class="flex-1 min-w-0">
																<div
																	class="text-sm font-medium text-gray-900 dark:text-gray-100 truncate"
																>
																	{entity.entity_name || entity.entity_mrn}
																</div>
																{#if entity.entity_name}
																	<div
																		class="text-xs text-gray-500 dark:text-gray-400 truncate font-mono"
																	></div>
																{/if}
															</div>
															{#if shouldShowAssetLink(entity)}
																<a
																	href={getAssetUrl(entity)}
																	target="_blank"
																	class="flex-shrink-0 p-1 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300"
																	title="View asset"
																>
																	<IconifyIcon
																		icon="material-symbols:open-in-new"
																		class="w-4 h-4"
																	/>
																</a>
															{/if}
														</div>
													</td>
													<td class="px-3 py-2">
														<span class="text-xs text-gray-600 dark:text-gray-300 capitalize"
															>{entity.entity_type}</span
														>
													</td>
													<td class="px-3 py-2">
														<span
															class="inline-flex items-center px-2 py-1 rounded-full text-xs font-medium {getStatusColor(
																entity.status
															)}"
														>
															<IconifyIcon
																icon={getStatusIcon(entity.status)}
																class="w-3 h-3 mr-1"
															/>
															{entity.status}
														</span>
													</td>
													<td class="px-3 py-2 text-xs text-gray-500 dark:text-gray-400">
														{formatDateTime(entity.created_at)}
													</td>
												</tr>
											{/each}
										</tbody>
									</table>
								</div>
							</div>
						{/if}
					</div>
				</div>
			</div>
		</div>
	</div>
{/if}
