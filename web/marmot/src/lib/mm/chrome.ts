import { writable, get } from 'svelte/store';
import { page } from '$app/state';

export interface Crumb {
	label: string;
	href?: string;
}

interface Chrome {
	/** The path the crumbs belong to, so a page that never sets them shows the default for its route. */
	path: string;
	crumbs: Crumb[] | null;
	/** Shows a lone crumb in the top bar, for a page whose own title doesn't already name its section. */
	showLoneCrumb?: boolean;
	/** Full-bleed pages (Query, Glossary, Lineage) manage their own padding. */
	bleed: boolean;
}

export const chrome = writable<Chrome>({ path: '', crumbs: null, bleed: false });

/** Sets the top bar's breadcrumbs for the current page; the last crumb is the page itself. Call from a page's $effect so it follows the data. */
export function setCrumbs(crumbs: Crumb[], opts: { showLoneCrumb?: boolean } = {}): void {
	chrome.update((c) => ({
		...c,
		path: page.url.pathname,
		crumbs,
		showLoneCrumb: opts.showLoneCrumb ?? false
	}));
}

/** Drops the main area's gutters so the page can fill the shell edge to edge. */
export function setBleed(bleed: boolean): void {
	const c = get(chrome);
	if (c.bleed !== bleed || c.path !== page.url.pathname)
		chrome.set({
			...c,
			path: page.url.pathname,
			crumbs: c.path === page.url.pathname ? c.crumbs : null,
			showLoneCrumb: c.path === page.url.pathname ? c.showLoneCrumb : false,
			bleed
		});
}
