// Catalog views and searches are daily rollups served for at most 30 days, so both Insights views share these three windows
export type InsightsRange = '24h' | '7d' | '30d';

export const RANGE_HOURS: Record<InsightsRange, number> = {
	'24h': 24,
	'7d': 7 * 24,
	'30d': 30 * 24
};

export function isRange(value: string | null): value is InsightsRange {
	return value === '24h' || value === '7d' || value === '30d';
}

/** The start of the window, rounded to the minute so re-renders do not ask for a new one. */
export function rangeStart(range: InsightsRange): Date {
	return new Date(Math.floor((Date.now() - RANGE_HOURS[range] * 3_600_000) / 60_000) * 60_000);
}

/** A share as "87%" or "87.5%", or an en dash when there is nothing to divide. */
export function percent(part: number, whole: number): string {
	if (!whole) return '–';
	return `${((part / whole) * 100).toFixed(1).replace(/\.0$/, '')}%`;
}
