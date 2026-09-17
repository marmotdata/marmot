import { getLocale } from '$lib/paraglide/runtime';
import { m } from '$lib/paraglide/messages';

/**
 * Format a date string as a date in the active UI locale
 */
export function formatDate(dateString: string): string {
	if (!dateString) return m.common_unknown();
	return new Date(dateString).toLocaleDateString(getLocale(), {
		year: 'numeric',
		month: 'short',
		day: 'numeric'
	});
}

/**
 * Format a date string as a date and time in the active UI locale
 */
export function formatDateTime(dateString: string): string {
	if (!dateString) return m.common_unknown();
	return new Date(dateString).toLocaleString(getLocale(), {
		year: 'numeric',
		month: 'short',
		day: 'numeric',
		hour: '2-digit',
		minute: '2-digit'
	});
}

/**
 * Format a date string as a relative time (e.g. "2 hours ago") in the active UI locale
 */
export function formatRelativeTime(dateString: string): string {
	if (!dateString) return m.common_unknown();

	const date = new Date(dateString);
	const now = new Date();
	const diffMs = now.getTime() - date.getTime();
	const diffMins = Math.floor(diffMs / 60000);
	const diffHours = Math.floor(diffMs / 3600000);
	const diffDays = Math.floor(diffMs / 86400000);

	if (diffMins < 1) return m.common_just_now();
	const rtf = new Intl.RelativeTimeFormat(getLocale(), { numeric: 'always' });
	if (diffMins < 60) return rtf.format(-diffMins, 'minute');
	if (diffHours < 24) return rtf.format(-diffHours, 'hour');
	if (diffDays < 7) return rtf.format(-diffDays, 'day');

	return formatDate(dateString);
}

/**
 * Format a number with grouping separators in the active UI locale
 */
export function formatNumber(value: number): string {
	return value.toLocaleString(getLocale());
}

/**
 * Join items into a natural-language list in the active UI locale (e.g. "a, b and c")
 */
export function formatList(items: string[]): string {
	try {
		return new Intl.ListFormat(getLocale(), { style: 'long', type: 'conjunction' }).format(items);
	} catch {
		return items.join(', ');
	}
}

/**
 * Format a duration in milliseconds to a human-readable string
 */
export function formatDuration(ms: number): string {
	if (ms < 1000) return `${ms}ms`;
	if (ms < 60000) return `${(ms / 1000).toFixed(1)}s`;
	if (ms < 3600000) return `${Math.floor(ms / 60000)}m ${Math.floor((ms % 60000) / 1000)}s`;
	return `${Math.floor(ms / 3600000)}h ${Math.floor((ms % 3600000) / 60000)}m`;
}

/**
 * Truncate a string to a maximum length with ellipsis
 */
export function truncate(str: string, maxLength: number): string {
	if (!str || str.length <= maxLength) return str;
	return str.slice(0, maxLength - 3) + '...';
}

/**
 * Capitalize the first letter of a string
 */
export function capitalize(str: string): string {
	if (!str) return str;
	return str.charAt(0).toUpperCase() + str.slice(1);
}
