export type RunStatus = 'pending' | 'claimed' | 'running' | 'succeeded' | 'failed' | 'cancelled';

/**
 * Get Tailwind CSS classes for a run/job status badge
 */
export function getStatusColor(status: string): string {
	switch (status) {
		case 'pending':
			return 'bg-blue-100 text-blue-800 dark:bg-blue-900/30 dark:text-blue-300';
		case 'claimed':
			return 'bg-indigo-100 text-indigo-800 dark:bg-indigo-900/30 dark:text-indigo-300';
		case 'running':
			return 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900/30 dark:text-yellow-300';
		case 'succeeded':
		case 'success':
			return 'bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-300';
		case 'failed':
		case 'failure':
		case 'error':
			return 'bg-red-100 text-red-800 dark:bg-red-900/30 dark:text-red-300';
		case 'cancelled':
		case 'canceled':
			return 'bg-gray-100 text-gray-800 dark:bg-gray-900/30 dark:text-gray-300';
		default:
			return 'bg-gray-100 text-gray-800 dark:bg-gray-900/30 dark:text-gray-300';
	}
}

/**
 * Get an icon name for a run/job status
 */
export function getStatusIcon(status: string): string {
	switch (status) {
		case 'pending':
			return 'material-symbols:schedule';
		case 'claimed':
			return 'material-symbols:hourglass-empty';
		case 'running':
			return 'material-symbols:sync';
		case 'succeeded':
		case 'success':
			return 'material-symbols:check-circle';
		case 'failed':
		case 'failure':
		case 'error':
			return 'material-symbols:error';
		case 'cancelled':
		case 'canceled':
			return 'material-symbols:cancel';
		default:
			return 'material-symbols:help';
	}
}

/**
 * Check if a status represents an active/in-progress state
 */
export function isActiveStatus(status: string): boolean {
	return ['pending', 'claimed', 'running'].includes(status);
}

/**
 * Check if a status represents a terminal/completed state
 */
export function isTerminalStatus(status: string): boolean {
	return ['succeeded', 'success', 'failed', 'failure', 'error', 'cancelled', 'canceled'].includes(
		status
	);
}
