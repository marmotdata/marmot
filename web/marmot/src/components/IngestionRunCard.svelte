<script lang="ts">
	import IconifyIcon from '@iconify/svelte';

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

	export let run: IngestionRun;
	export let onClick: () => void = () => {};

	function getStatusColor(status: string): string {
		switch (status) {
			case 'pending':
				return 'bg-blue-100 text-blue-800 dark:bg-blue-900/30 dark:text-blue-300';
			case 'claimed':
				return 'bg-indigo-100 text-indigo-800 dark:bg-indigo-900/30 dark:text-indigo-300';
			case 'running':
				return 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900/30 dark:text-yellow-300';
			case 'succeeded':
				return 'bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-300';
			case 'failed':
				return 'bg-red-100 text-red-800 dark:bg-red-900/30 dark:text-red-300';
			case 'cancelled':
				return 'bg-gray-100 text-gray-800 dark:bg-gray-900/30 dark:text-gray-300';
			default:
				return 'bg-gray-100 text-gray-800 dark:bg-gray-900/30 dark:text-gray-300';
		}
	}

	function getStatusIcon(status: string): string {
		switch (status) {
			case 'pending':
				return 'material-symbols:schedule';
			case 'claimed':
				return 'material-symbols:assignment-ind';
			case 'running':
				return 'material-symbols:sync';
			case 'succeeded':
				return 'material-symbols:check-circle';
			case 'failed':
				return 'material-symbols:error';
			case 'cancelled':
				return 'material-symbols:cancel';
			default:
				return 'material-symbols:help';
		}
	}

	function formatDuration(startedAt: string, completedAt?: string): string {
		const start = new Date(startedAt);
		const end = completedAt ? new Date(completedAt) : new Date();
		const durationMs = end.getTime() - start.getTime();

		const seconds = Math.floor(durationMs / 1000);
		const minutes = Math.floor(seconds / 60);
		const hours = Math.floor(minutes / 60);

		if (hours > 0) {
			return `${hours}h ${minutes % 60}m`;
		} else if (minutes > 0) {
			return `${minutes}m ${seconds % 60}s`;
		} else if (seconds > 0) {
			return `${seconds}s`;
		} else {
			return `${durationMs}ms`;
		}
	}

	function formatTimeAgo(dateString: string): string {
		const now = new Date();
		const date = new Date(dateString);
		const diffMs = now.getTime() - date.getTime();

		const minutes = Math.floor(diffMs / (1000 * 60));
		const hours = Math.floor(diffMs / (1000 * 60 * 60));
		const days = Math.floor(diffMs / (1000 * 60 * 60 * 24));

		if (days > 0) {
			return `${days}d ago`;
		} else if (hours > 0) {
			return `${hours}h ago`;
		} else if (minutes > 0) {
			return `${minutes}m ago`;
		} else {
			return 'Just now';
		}
	}
</script>

<div
	class="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-4 hover:bg-gray-50 dark:hover:bg-gray-750 transition-colors cursor-pointer"
	on:click={onClick}
	role="button"
	tabindex="0"
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
				{formatTimeAgo(run.started_at)}
			</dd>
		</div>

		<div>
			<dt
				class="text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider mb-1"
			>
				Duration
			</dt>
			<dd class="text-gray-900 dark:text-gray-100">
				{formatDuration(run.started_at, run.finished_at)}
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
					<div class="text-lg font-semibold text-earthy-terracotta-700 dark:text-earthy-terracotta-700">
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
