<script lang="ts">
	import IconifyIcon from '@iconify/svelte';
	import Icon from './Icon.svelte';
	import { auth } from '$lib/stores/auth';

	interface Schedule {
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

	interface Props {
		schedule: Schedule;
		onEdit?: (schedule: Schedule) => void;
		onDelete?: (schedule: Schedule) => void;
		onTrigger?: (schedule: Schedule) => void;
		isRunning?: boolean;
	}

	let { schedule, onEdit, onDelete, onTrigger, isRunning = false }: Props = $props();

	let canManageIngestion = $derived(auth.hasPermission('ingestion', 'manage'));

	function formatSchedule(cronExpression: string): string {
		if (!cronExpression) return 'Manual';
		// Simple cron formatter - you could enhance this
		return cronExpression;
	}

	function formatDate(dateStr?: string): string {
		if (!dateStr) return '—';
		const date = new Date(dateStr);
		const now = new Date();
		const diff = now.getTime() - date.getTime();
		const minutes = Math.floor(diff / 60000);
		const hours = Math.floor(minutes / 60);
		const days = Math.floor(hours / 24);

		if (minutes < 1) return 'Just now';
		if (minutes < 60) return `${minutes}m ago`;
		if (hours < 24) return `${hours}h ago`;
		if (days < 7) return `${days}d ago`;
		return date.toLocaleDateString();
	}

	function formatNextRun(dateStr?: string): string {
		if (!dateStr) return '—';
		const date = new Date(dateStr);
		const now = new Date();
		const diff = date.getTime() - now.getTime();
		const minutes = Math.floor(diff / 60000);
		const hours = Math.floor(minutes / 60);
		const days = Math.floor(hours / 24);

		if (diff < 0) return 'Overdue';
		if (minutes < 1) return 'Now';
		if (minutes < 60) return `In ${minutes}m`;
		if (hours < 24) return `In ${hours}h`;
		return `In ${days}d`;
	}

	function getStatusColor(status?: string): string {
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

	function getStatusIcon(status?: string): string {
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
</script>

<tr
	class="group hover:bg-gray-50 dark:hover:bg-gray-750 transition-colors border-b border-gray-200 dark:border-gray-700 last:border-0"
>
	<!-- Play Button -->
	<td class="px-4 py-4 w-16">
		{#if onTrigger && canManageIngestion}
			<button
				class="p-2 rounded-lg transition-colors {isRunning
					? 'text-gray-400 dark:text-gray-600 cursor-not-allowed'
					: 'text-gray-600 dark:text-gray-400 hover:bg-gray-100 dark:hover:bg-gray-700'}"
				title={isRunning ? 'Running...' : 'Trigger now'}
				onclick={() => !isRunning && onTrigger?.(schedule)}
				disabled={isRunning}
			>
				<IconifyIcon icon="material-symbols:play-arrow" class="h-5 w-5" />
			</button>
		{/if}
	</td>

	<!-- Icon & Name -->
	<td class="px-6 py-4">
		<div class="flex items-center gap-3">
			<div class="flex-shrink-0">
				<Icon name={schedule.plugin_id} size="sm" showLabel={false} />
			</div>
			<div>
				<div class="font-medium text-gray-900 dark:text-gray-100">{schedule.name}</div>
				<div class="text-xs text-gray-500 dark:text-gray-400 capitalize">{schedule.plugin_id}</div>
			</div>
		</div>
	</td>

	<!-- Status -->
	<td class="px-6 py-4">
		{#if isRunning}
			<span
				class="inline-flex items-center px-2.5 py-1 rounded-md text-xs font-medium bg-yellow-100 dark:bg-yellow-900/30 text-yellow-800 dark:text-yellow-300"
			>
				<IconifyIcon icon="material-symbols:sync" class="h-3.5 w-3.5 mr-1.5 animate-spin" />
				Running
			</span>
		{:else if schedule.last_run_status}
			<span
				class="inline-flex items-center px-2.5 py-1 rounded-md text-xs font-medium {getStatusColor(
					schedule.last_run_status
				)}"
			>
				<IconifyIcon icon={getStatusIcon(schedule.last_run_status)} class="h-3.5 w-3.5 mr-1.5" />
				{schedule.last_run_status.charAt(0).toUpperCase() + schedule.last_run_status.slice(1)}
			</span>
		{:else}
			<span class="text-sm text-gray-500 dark:text-gray-400">—</span>
		{/if}
	</td>

	<!-- Schedule -->
	<td class="px-6 py-4">
		<div class="flex items-center gap-2 text-sm text-gray-600 dark:text-gray-400">
			<IconifyIcon icon="material-symbols:schedule" class="h-4 w-4 flex-shrink-0" />
			<span class="font-mono text-xs">{formatSchedule(schedule.cron_expression)}</span>
		</div>
	</td>

	<!-- Last Run -->
	<td class="px-6 py-4">
		<div class="text-sm text-gray-600 dark:text-gray-400">
			{formatDate(schedule.last_run_at)}
		</div>
	</td>

	<!-- Next Run -->
	<td class="px-6 py-4">
		<div class="text-sm text-gray-600 dark:text-gray-400">
			{formatNextRun(schedule.next_run_at)}
		</div>
	</td>

	<!-- Actions -->
	<td class="px-6 py-4">
		<div class="flex items-center justify-end gap-1">
			{#if onEdit && canManageIngestion}
				<button
					class="p-2 text-gray-600 dark:text-gray-400 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg transition-colors"
					title="Edit"
					onclick={() => onEdit?.(schedule)}
				>
					<IconifyIcon icon="material-symbols:edit" class="h-5 w-5" />
				</button>
			{/if}
			{#if onDelete && canManageIngestion}
				<button
					class="p-2 text-red-600 dark:text-red-400 hover:bg-red-50 dark:hover:bg-red-900/20 rounded-lg transition-colors"
					title="Delete"
					onclick={() => onDelete?.(schedule)}
				>
					<IconifyIcon icon="material-symbols:delete" class="h-5 w-5" />
				</button>
			{/if}
		</div>
	</td>
</tr>
