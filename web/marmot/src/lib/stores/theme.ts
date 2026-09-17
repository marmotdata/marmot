import { writable } from 'svelte/store';
import { browser } from '$app/environment';

type Theme = 'light' | 'dark' | 'auto';

// The same decision runs in src/app.html before the first paint. Keep both in sync.
function isDarkTheme(theme: Theme): boolean {
	return (
		theme === 'dark' ||
		(theme === 'auto' && window.matchMedia('(prefers-color-scheme: dark)').matches)
	);
}

function readStoredTheme(): Theme {
	if (!browser) return 'auto';
	const saved = localStorage.getItem('theme');
	return saved === 'light' || saved === 'dark' || saved === 'auto' ? saved : 'auto';
}

function createThemeStore() {
	let current: Theme = readStoredTheme();
	const { subscribe, set: internalSet } = writable<Theme>(current);

	function applyTheme(theme: Theme) {
		if (!browser) return;
		const dark = isDarkTheme(theme);
		document.documentElement.classList.toggle('dark', dark);
		document.documentElement.style.colorScheme = dark ? 'dark' : 'light';
	}

	if (browser) {
		// Registered once. Doing this per update would stack up listeners.
		window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', () => {
			if (current === 'auto') applyTheme('auto');
		});
	}

	function set(theme: Theme) {
		current = theme;
		internalSet(theme);
		if (browser) {
			localStorage.setItem('theme', theme);
		}
		applyTheme(theme);
	}

	return {
		subscribe,
		set,
		initialize: () => set(readStoredTheme()),
		getCurrentTheme: (): Theme => current
	};
}

export const theme = createThemeStore();
