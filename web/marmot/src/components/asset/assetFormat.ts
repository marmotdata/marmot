import { resolve } from '$app/paths';
import type { Asset } from '$lib/assets/types';
import { mmIcon } from '$lib/mm';

/** The resolved asset page href for an asset, or undefined when its MRN cannot be read. */
export function assetHref(a: Pick<Asset, 'mrn' | 'type'>): string | undefined {
	if (!a?.mrn || !a?.type) return undefined;
	const parts = a.mrn.replace('mrn://', '').split('/');
	if (parts.length < 3) return undefined;
	return resolve('/discover/[type]/[service]/[name]', {
		type: a.type.toLowerCase(),
		service: parts[1],
		name: encodeURIComponent(parts.slice(2).join('/'))
	});
}

/** The provider key for the header logo: the single provider when there is one, otherwise the type so the logo element can fall back. */
export function providerKey(a: Pick<Asset, 'providers' | 'type'>): string {
	if (Array.isArray(a.providers) && a.providers.length > 0) return a.providers[0].toLowerCase();
	return (a.type || '').toLowerCase();
}

/** A duration with a non-breaking space before the unit, as the design system writes them ("412 ms", "6.2 s"). */
export function formatMs(ms?: number | null): string {
	if (ms == null || ms < 0) return '';
	if (ms < 1000) return `${Math.round(ms)}\u00a0ms`;
	const s = ms / 1000;
	if (s < 60) return `${s.toFixed(1)}\u00a0s`;
	const mins = Math.floor(s / 60);
	if (mins < 60) return `${mins}\u00a0min ${Math.floor(s % 60)}\u00a0s`;
	return `${Math.floor(mins / 60)}\u00a0h ${mins % 60}\u00a0min`;
}

/** External links store an Iconify name; only Material Symbols map onto mm-icon, so anything else falls back to the plain link icon. */
export function linkIcon(icon?: string): string {
	if (icon && icon.startsWith('material-symbols:')) return mmIcon(icon);
	return 'link';
}
