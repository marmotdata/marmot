<script lang="ts">
	import { fade, fly } from 'svelte/transition';
	import IconifyIcon from '@iconify/svelte';
	import Button from '$components/ui/Button.svelte';
	import MetadataView from '$components/shared/MetadataView.svelte';
	import CodeBlock from '$components/editor/CodeBlock.svelte';
	import Icon from '$components/ui/Icon.svelte';
	import { fetchApi } from '$lib/api';
	import yaml from 'js-yaml';

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
		status: 'pending' | 'claimed' | 'running' | 'succeeded' | 'failed' | 'cancelled';
		started_at: string;
		finished_at?: string;
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

			const response = await fetchApi(`/ingestion/runs/${run.id}/entities?${params}`);
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
			pending: 'bg-blue-100 text-blue-800 dark:bg-blue-900/30 dark:text-blue-300',
			claimed: 'bg-indigo-100 text-indigo-800 dark:bg-indigo-900/30 dark:text-indigo-300',
			running: 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900/30 dark:text-yellow-300',
			succeeded: 'bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-300',
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
			pending: 'material-symbols:schedule',
			claimed: 'material-symbols:assignment-ind',
			running: 'material-symbols:sync',
			succeeded: 'material-symbols:check-circle',
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
		if (entity.entity_type !== 'asset' || !entity.entity_mrn) return '';

		// Parse MRN: mrn://type/service/full.qualified.name
		const mrnParts = entity.entity_mrn.replace('mrn://', '').split('/');
		if (mrnParts.length < 3) return '';
		const type = mrnParts[0];
		const service = mrnParts[1];
		const fullName = mrnParts.slice(2).join('/');
		return `/discover/${encodeURIComponent(type)}/${encodeURIComponent(service)}/${encodeURIComponent(fullName)}`;
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
		onkeydown={(e) => e.key === 'Escape' && onClose()}
		role="dialog"
		aria-modal="true"
		tabindex="-1"
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
						class="h-6 w-6 text-earthy-terracotta-700 dark:text-earthy-terracotta-700"
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
					<!-- Run Info Cards -->
					<div class="grid grid-cols-2 sm:grid-cols-4 gap-4">
						<div
							class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-4"
						>
							<dt
								class="text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wide mb-2"
							>
								Status
							</dt>
							<dd>
								<span
									class="inline-flex items-center px-2.5 py-1 rounded-full text-xs font-medium {getStatusColor(
										run.status
									)}"
								>
									<IconifyIcon
										icon={getStatusIcon(run.status)}
										class="w-3.5 h-3.5 mr-1.5 {run.status === 'running' ? 'animate-spin' : ''}"
									/>
									{run.status.charAt(0).toUpperCase() + run.status.slice(1)}
								</span>
							</dd>
						</div>
						<div
							class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-4"
						>
							<dt
								class="text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wide mb-2"
							>
								Duration
							</dt>
							<dd class="text-lg font-semibold text-gray-900 dark:text-gray-100">
								{formatDuration(run.started_at, run.finished_at)}
							</dd>
						</div>
						<div
							class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-4"
						>
							<dt
								class="text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wide mb-2"
							>
								Source
							</dt>
							<dd class="flex items-center gap-2">
								<Icon name={run.source_name} size="xs" showLabel={false} />
								<span class="text-sm font-medium text-gray-900 dark:text-gray-100 truncate">
									{run.source_name}
								</span>
							</dd>
						</div>
						<div
							class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-4"
						>
							<dt
								class="text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wide mb-2"
							>
								Created By
							</dt>
							<dd class="text-sm font-medium text-gray-900 dark:text-gray-100 truncate">
								{run.created_by}
							</dd>
						</div>
					</div>

					<!-- Summary Stats -->
					{#if run.summary}
						<div class="grid grid-cols-4 gap-4">
							<div
								class="bg-green-50 dark:bg-green-900/20 border border-green-200 dark:border-green-800/50 rounded-xl p-4 text-center"
							>
								<div class="text-2xl font-bold text-green-600 dark:text-green-400">
									{run.summary.assets_created}
								</div>
								<div
									class="text-xs font-medium text-green-700 dark:text-green-300 uppercase tracking-wide mt-1"
								>
									Created
								</div>
							</div>
							<div
								class="bg-blue-50 dark:bg-blue-900/20 border border-blue-200 dark:border-blue-800/50 rounded-xl p-4 text-center"
							>
								<div class="text-2xl font-bold text-blue-600 dark:text-blue-400">
									{run.summary.assets_updated}
								</div>
								<div
									class="text-xs font-medium text-blue-700 dark:text-blue-300 uppercase tracking-wide mt-1"
								>
									Updated
								</div>
							</div>
							<div
								class="bg-orange-50 dark:bg-orange-900/20 border border-orange-200 dark:border-orange-800/50 rounded-xl p-4 text-center"
							>
								<div class="text-2xl font-bold text-orange-600 dark:text-orange-400">
									{run.summary.assets_deleted}
								</div>
								<div
									class="text-xs font-medium text-orange-700 dark:text-orange-300 uppercase tracking-wide mt-1"
								>
									Deleted
								</div>
							</div>
							<div
								class="bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800/50 rounded-xl p-4 text-center"
							>
								<div class="text-2xl font-bold text-red-600 dark:text-red-400">
									{run.summary.errors}
								</div>
								<div
									class="text-xs font-medium text-red-700 dark:text-red-300 uppercase tracking-wide mt-1"
								>
									Errors
								</div>
							</div>
						</div>
					{/if}

					<!-- Error Message -->
					{#if run.error_message}
						<div
							class="bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800/50 rounded-xl p-4"
						>
							<h3
								class="text-sm font-semibold text-red-800 dark:text-red-200 mb-2 flex items-center"
							>
								<IconifyIcon icon="material-symbols:error" class="h-4 w-4 mr-2" />
								Error Details
							</h3>
							<pre
								class="text-sm text-red-700 dark:text-red-300 whitespace-pre-wrap font-mono bg-red-100 dark:bg-red-900/40 p-3 rounded-lg overflow-x-auto">{run.error_message}</pre>
						</div>
					{/if}

					<!-- Configuration -->
					{#if run.config && Object.keys(run.config).length > 0}
						<div
							class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 overflow-hidden"
						>
							<div
								class="flex items-center justify-between px-4 py-3 bg-gray-50 dark:bg-gray-900 border-b border-gray-200 dark:border-gray-700"
							>
								<h3
									class="text-sm font-semibold text-gray-900 dark:text-gray-100 flex items-center"
								>
									<IconifyIcon
										icon="material-symbols:settings"
										class="h-4 w-4 mr-2 text-gray-500 dark:text-gray-400"
									/>
									Configuration
								</h3>
								<Button
									variant="clear"
									text={showRawConfig ? 'Structured View' : 'Raw YAML'}
									icon={showRawConfig ? 'material-symbols:view-list' : 'material-symbols:code'}
									click={() => (showRawConfig = !showRawConfig)}
								/>
							</div>
							<div class="p-4">
								{#if showRawConfig}
									<div class="max-h-72 overflow-y-auto rounded-lg">
										<CodeBlock
											code={yaml.dump(run.config, { indent: 2, lineWidth: -1 })}
											language="yaml"
										/>
									</div>
								{:else}
									<div class="max-h-64 overflow-y-auto">
										<MetadataView metadata={run.config} readOnly={true} maxDepth={3} />
									</div>
								{/if}
							</div>
						</div>
					{/if}

					<!-- Entities -->
					<div
						class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 overflow-hidden"
					>
						<div
							class="flex items-center justify-between px-4 py-3 bg-gray-50 dark:bg-gray-900 border-b border-gray-200 dark:border-gray-700"
						>
							<h3 class="text-sm font-semibold text-gray-900 dark:text-gray-100">
								Entities
								<span class="ml-1.5 text-xs font-normal text-gray-500 dark:text-gray-400">
									({entitiesTotal})
								</span>
							</h3>
							{#if entitiesTotal > 0}
								<div class="flex items-center space-x-2">
									<Button
										variant="clear"
										click={() => goToEntitiesPage(entitiesPage - 1)}
										disabled={entitiesPage === 1}
										icon="chevron-left"
										text="Previous"
									/>
									<span class="text-xs text-gray-500 dark:text-gray-400 tabular-nums">
										{entitiesPage} / {Math.max(1, Math.ceil(entitiesTotal / entitiesLimit))}
									</span>
									<Button
										variant="clear"
										click={() => goToEntitiesPage(entitiesPage + 1)}
										disabled={entitiesPage >= Math.ceil(entitiesTotal / entitiesLimit)}
										text="Next"
										icon="chevron-right"
									/>
								</div>
							{/if}
						</div>

						<div class="p-4">
							{#if entitiesLoading}
								<div class="flex items-center justify-center py-8">
									<div
										class="animate-spin rounded-full h-6 w-6 border-b-2 border-earthy-terracotta-700"
									></div>
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
							{/if}
						</div>
					</div>
				</div>
			</div>
		</div>
	</div>
{/if}
