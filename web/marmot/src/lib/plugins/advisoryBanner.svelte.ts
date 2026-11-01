import { browser } from '$app/environment';
import { getPluginUpdatesStatus } from './api';
import { severityRank } from './advisories';
import type { PluginAdvisory } from './types';

export interface AdvisoryBannerEntry {
	id: string;
	name: string;
	version: string;
	advisory: PluginAdvisory;
}

const DISMISSED_KEY = 'plugin-advisory-dismissed';

function readDismissed(): string[] {
	if (!browser) return [];
	try {
		const raw = localStorage.getItem(DISMISSED_KEY);
		return raw ? JSON.parse(raw) : [];
	} catch {
		return [];
	}
}

// Read once per app load: the banner only needs to know whether anything is wrong, and a stale
// answer for a few minutes is harmless since the Plugins settings page always has the live state.
function createAdvisoryBannerStore() {
	let entries = $state<AdvisoryBannerEntry[]>([]);
	let dismissed = $state<string[]>(readDismissed());
	let requested = false;

	async function load(): Promise<void> {
		if (requested) return;
		requested = true;
		try {
			const status = await getPluginUpdatesStatus();
			// Every advisory against every plugin's running version, not just one per plugin, so the
			// single most severe advisory anywhere is the one that can win below.
			entries = status.plugins.flatMap((p) =>
				p.advisories.map((advisory) => ({ id: p.id, name: p.name, version: p.version, advisory }))
			);
		} catch {
			// The banner is a convenience notice; a failed or disabled check just means it never appears this session
		}
	}

	function dismiss(id: string) {
		dismissed = [...dismissed, id];
		try {
			localStorage.setItem(DISMISSED_KEY, JSON.stringify(dismissed));
		} catch {
			// Private browsing or a full quota just means the banner reappears next load
		}
	}

	// The single most severe advisory across every plugin, ties broken by plugin name, skipping
	// whichever the admin has already dismissed this session or before.
	const current = $derived(
		[...entries]
			.filter((e) => !dismissed.includes(e.advisory.id))
			.sort(
				(a, b) =>
					severityRank(b.advisory.severity) - severityRank(a.advisory.severity) ||
					a.name.localeCompare(b.name)
			)[0]
	);

	return {
		get current(): AdvisoryBannerEntry | undefined {
			return current;
		},
		load,
		dismiss
	};
}

export const advisoryBanner = createAdvisoryBannerStore();
