<script lang="ts">
	import IconifyIcon from '@iconify/svelte';
	import { getStatusColor, getStatusIcon } from '$lib/utils/status';
	import { formatRelativeTime, formatDuration } from '$lib/utils/format';

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
		config?: unknown;
		summary?: IngestionRunSummary;
		created_by: string;
	}

	export let run: IngestionRun;
	export let onClick: () => void = () => {};

	function getRunDuration(startedAt: string, completedAt?: string): string {
		const start = new Date(startedAt);
		const end = completedAt ? new Date(completedAt) : new Date();
		return formatDuration(end.getTime() - start.getTime());
	}
</script>

<div
	class="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-4 hover:bg-gray-50 dark:hover:bg-gray-750 transition-colors cursor-pointer"
	on:click={onClick}
	role="button"
	tabindex="0"
	aria-label="View run details for {run.pipeline_name}"
	on:keydown={(e) => e.key === 'Enter' && onClick()}
>
	<div class="flex items-start justify-between mb-3">
		<div class="flex items-center space-x-3">
			<div class="flex-shrink-0">
				<IconifyIcon
					icon={run.source_name === 'destroy'
						? 'material-symbols:delete-forever'
						: 'material-symbols:sync'}
					class="h-6 w-6 {run.source_name === 'destroy'
						? 'text-red-600 dark:text-red-400'
						: 'text-earthy-terracotta-700 dark:text-earthy-terracotta-700'}"
				/>
			</div>
			<div class="min-w-0 flex-1">
				<div class="flex items-center space-x-2">
					<h3 class="text-lg font-semibold text-gray-900 dark:text-gray-100 truncate">
						{run.pipeline_name}
					</h3>
					{#if run.source_name === 'destroy'}
						<span
							class="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium bg-red-100 text-red-800 dark:bg-red-900/30 dark:text-red-300"
						>
							<IconifyIcon icon="material-symbols:delete-forever" class="w-3 h-3 mr-1" />
							Teardown
						</span>
					{/if}
				</div>
				<p class="text-sm text-gray-600 dark:text-gray-400 truncate">
					Source: {run.source_name}
				</p>
			</div>
		</div>

		<div class="flex items-center space-x-2">
			<span
				class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium {getStatusColor(
					run.status
				)}"
			>
				<IconifyIcon
					icon={getStatusIcon(run.status)}
					class="w-3 h-3 mr-1 {run.status === 'running' ? 'animate-spin' : ''}"
				/>
				{run.status.charAt(0).toUpperCase() + run.status.slice(1)}
			</span>
		</div>
	</div>

	<div class="grid grid-cols-2 md:grid-cols-4 gap-4 text-sm">
		<div>
			<dt
				class="text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider mb-1"
			>
				Started
			</dt>
			<dd class="text-gray-900 dark:text-gray-100">
				{formatRelativeTime(run.started_at)}
			</dd>
		</div>

		<div>
			<dt
				class="text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider mb-1"
			>
				Duration
			</dt>
			<dd class="text-gray-900 dark:text-gray-100">
				{getRunDuration(run.started_at, run.finished_at)}
			</dd>
		</div>

		<div>
			<dt
				class="text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider mb-1"
			>
				Created By
			</dt>
			<dd class="text-gray-900 dark:text-gray-100 truncate">
				{run.created_by}
			</dd>
		</div>

		<div>
			<dt
				class="text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider mb-1"
			>
				Run ID
			</dt>
			<dd class="text-gray-900 dark:text-gray-100 font-mono text-xs truncate">
				{run.run_id}
			</dd>
		</div>
	</div>

	{#if run.summary}
		<div class="mt-4 pt-3 border-t border-gray-200 dark:border-gray-600">
			<div class="grid grid-cols-2 md:grid-cols-4 gap-4 text-sm">
				<div class="text-center">
					<div class="text-lg font-semibold text-green-600 dark:text-green-400">
						{run.summary.assets_created}
					</div>
					<div class="text-xs text-gray-500 dark:text-gray-400">Created</div>
				</div>
				<div class="text-center">
					<div class="text-lg font-semibold text-blue-600 dark:text-blue-400">
						{run.summary.assets_updated}
					</div>
					<div class="text-xs text-gray-500 dark:text-gray-400">Updated</div>
				</div>
				<div class="text-center">
					<div
						class="text-lg font-semibold text-earthy-terracotta-700 dark:text-earthy-terracotta-700"
					>
						{run.summary.assets_deleted}
					</div>
					<div class="text-xs text-gray-500 dark:text-gray-400">Deleted</div>
				</div>
				<div class="text-center">
					<div class="text-lg font-semibold text-red-600 dark:text-red-400">
						{run.summary.errors}
					</div>
					<div class="text-xs text-gray-500 dark:text-gray-400">Errors</div>
				</div>
			</div>
		</div>
	{/if}

	{#if run.error_message}
		<div
			class="mt-2 p-2 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800/50 rounded text-xs"
		>
			<div class="flex">
				<IconifyIcon
					icon="material-symbols:error"
					class="h-3 w-3 text-red-400 mt-0.5 flex-shrink-0"
				/>
				<p class="ml-2 text-red-700 dark:text-red-300 break-words">
					{run.error_message}
				</p>
			</div>
		</div>
	{/if}
</div>
