import { browser } from '$app/environment';
import { readonly, writable } from 'svelte/store';
import {
	baseLocale,
	extractLocaleFromNavigator,
	isLocale,
	locales,
	localStorageKey,
	overwriteGetLocale,
	setLocale
} from '$lib/paraglide/runtime';

export type Locale = (typeof locales)[number];

let instanceDefault: string | null = null;

// Every message call goes through getLocale, so the answer is resolved once and cached rather than re-read from storage each time
let current: Locale = baseLocale;

const localeStore = writable<Locale>(current);

// Bumped on every change so markup wrapped in {#key $locale} re-renders with the new messages
export const locale = readonly(localeStore);

// The account preference is written into the stored slot at login, so it wins wherever the user signs in
function computeLocale(): Locale {
	try {
		const stored = localStorage.getItem(localStorageKey);
		if (stored && isLocale(stored)) return stored;
	} catch {
		// unavailable in private windows, fall through
	}
	if (instanceDefault && isLocale(instanceDefault)) return instanceDefault;
	const fromNavigator = extractLocaleFromNavigator();
	if (fromNavigator) return fromNavigator;
	return baseLocale;
}

function apply(next: Locale): void {
	current = next;
	document.documentElement.lang = next;
	localeStore.set(next);
}

// Awaited from the root layout load so every message call sees the resolved locale from the first paint
export async function initI18n(): Promise<void> {
	if (!browser) return;
	try {
		// Capped so a stalled request cannot hold up the first paint indefinitely
		const response = await fetch('/api/v1/ui/config', { signal: AbortSignal.timeout(3000) });
		if (response.ok) {
			const config = await response.json();
			if (typeof config.default_language === 'string' && config.default_language) {
				instanceDefault = config.default_language;
			}
		}
	} catch {
		// no instance default, resolution falls through to the browser language
	}
	overwriteGetLocale(() => current);
	apply(computeLocale());
}

// Switches without reloading, which keeps the app shell on screen instead of flashing a blank document
export function changeLocale(next: Locale): void {
	if (!browser || next === current) return;
	setLocale(next, { reload: false });
	apply(next);
}

// Takes effect on the next document load, which is why login navigates with a full page load
export function applyStoredLanguage(language: unknown): void {
	if (!browser || typeof language !== 'string' || !isLocale(language)) return;
	try {
		localStorage.setItem(localStorageKey, language);
	} catch {
		// unavailable in private windows, browser detection applies instead
	}
}
