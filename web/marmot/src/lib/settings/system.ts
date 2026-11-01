import { fetchApi } from '$lib/api';
import { parseApiError } from '$lib/stores/toast';
import { getLocale } from '$lib/paraglide/runtime';

export interface SystemSearchIndex {
	documents: number;
	size_bytes: number;
	/** postgres, or elasticsearch when Elasticsearch serves searches from a copy of the index. */
	backend: 'postgres' | 'elasticsearch';
	reindex_running: boolean;
	last_full_reindex?: string;
}

/** What the System page shows about the running instance; admin only, since versions tell an attacker which advisories apply. */
export interface SystemInfo {
	build: {
		version: string;
		commit?: string;
		built_at?: string;
		modified?: boolean;
		go_version: string;
	};
	database: { version: string; size_bytes: number };
	search_index: SystemSearchIndex;
}

export async function getSystemInfo(): Promise<SystemInfo> {
	const res = await fetchApi('/admin/system');
	if (!res.ok) throw new Error((await parseApiError(res)).message);
	return res.json();
}

const UNITS = ['byte', 'kilobyte', 'megabyte', 'gigabyte', 'terabyte'] as const;

/** A size in the largest unit that keeps the number above one, as people read sizes. */
export function formatBytes(bytes: number): string {
	let value = bytes;
	let unit = 0;
	while (value >= 1024 && unit < UNITS.length - 1) {
		value /= 1024;
		unit++;
	}
	return new Intl.NumberFormat(getLocale(), {
		style: 'unit',
		unit: UNITS[unit],
		unitDisplay: 'short',
		maximumFractionDigits: value < 10 && unit > 0 ? 1 : 0
	}).format(value);
}
