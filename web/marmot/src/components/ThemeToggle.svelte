<script lang="ts">
	import { onMount } from 'svelte';
	import { fetchApi } from '$lib/api';
	import Button from './Button.svelte';
	import { theme } from '$lib/stores/theme';

	type ThemeType = 'light' | 'dark' | 'auto';

	let loading = true;
	let currentTheme: ThemeType;

	// Initialize currentTheme from the store on page load
	theme.initialize();

	// Subscribe to the theme store for updates
	theme.subscribe((value) => {
		currentTheme = value;
	});

	onMount(async () => {
		try {
			const response = await fetchApi('/users/me');
			if (response.ok) {
				const user = await response.json();
				if (user.preferences?.theme) {
					// Only update if the API returns a different theme
					if (user.preferences.theme !== currentTheme) {
						theme.set(user.preferences.theme);
					}
				}
			}
		} catch (error) {
			console.error('Failed to fetch theme preference:', error);
		} finally {
			loading = false;
		}
	});

	const handleThemeChange = (newTheme: ThemeType) => () => {
		theme.set(newTheme); // Update the theme in the store

		// API call to update user preference (in the background)
		fetchApi('/users/preferences', {
			method: 'PUT',
			body: JSON.stringify({
				preferences: { theme: newTheme }
			})
		})
			.then((response) => {
				if (!response.ok) {
					console.error('Failed to update theme preference:', response.statusText);
				}
			})
			.catch((error) => {
				console.error('Failed to update theme preference:', error);
			});
	};
</script>

<div class="inline-flex rounded-md shadow-sm">
	<Button
		variant={currentTheme === 'light' ? 'filled' : 'clear'}
		class="rounded-r-none border-r-0"
		click={handleThemeChange('light')}
		disabled={loading}
		icon="material-symbols:sunny"
		text="Light"
	/>

	<Button
		variant={currentTheme === 'dark' ? 'filled' : 'clear'}
		class="rounded-none border-x"
		click={handleThemeChange('dark')}
		disabled={loading}
		icon="material-symbols:moon-stars"
		text="Dark"
	/>

	<Button
		variant={currentTheme === 'auto' ? 'filled' : 'clear'}
		class="rounded-l-none border-l-0"
		click={handleThemeChange('auto')}
		disabled={loading}
		icon="material-symbols:wand-stars-rounded"
		text="Auto"
	/>
</div>
