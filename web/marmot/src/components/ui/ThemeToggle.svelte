<script lang="ts">
	import { onMount } from 'svelte';
	import { fetchApi } from '$lib/api';
	import { theme } from '$lib/stores/theme';
	import { m } from '$lib/paraglide/messages';
	import SegmentedControl from './SegmentedControl.svelte';

	type ThemeType = 'light' | 'dark' | 'auto';

	theme.initialize();
	let currentTheme = $state<ThemeType>('auto');
	theme.subscribe((value) => {
		currentTheme = value as ThemeType;
	});

	const options: { id: ThemeType; label: string; icon: string }[] = [
		{ id: 'light', label: m.ui_theme_light(), icon: 'material-symbols:sunny-outline' },
		{ id: 'dark', label: m.ui_theme_dark(), icon: 'material-symbols:dark-mode-outline' },
		{ id: 'auto', label: m.ui_theme_auto(), icon: 'material-symbols:brightness-auto-outline' }
	];

	onMount(async () => {
		try {
			const response = await fetchApi('/users/me');
			if (response.ok) {
				const user = await response.json();
				// The saved preference wins over whatever this browser last used
				if (user.preferences?.theme && user.preferences.theme !== currentTheme) {
					theme.set(user.preferences.theme);
				}
			}
		} catch (error) {
			console.error('Failed to fetch theme preference:', error);
		}
	});

	function change(next: ThemeType) {
		theme.set(next);
		// Saved in the background; the theme has already changed on screen
		fetchApi('/users/preferences', {
			method: 'PUT',
			body: JSON.stringify({ preferences: { theme: next } })
		})
			.then((response) => {
				if (!response.ok) console.error('Failed to update theme preference:', response.statusText);
			})
			.catch((error) => console.error('Failed to update theme preference:', error));
	}
</script>

<SegmentedControl
	{options}
	value={currentTheme}
	onChange={change}
	label={m.profile_theme_label()}
/>
