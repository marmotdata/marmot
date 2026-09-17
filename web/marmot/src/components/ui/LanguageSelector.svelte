<script lang="ts">
	import { fetchApi } from '$lib/api';
	import { m } from '$lib/paraglide/messages';
	import { locales } from '$lib/paraglide/runtime';
	import { changeLocale, locale, type Locale } from '$lib/i18n';
	import { auth } from '$lib/stores/auth';
	import { toasts } from '$lib/stores/toast';

	let saving = false;

	function displayName(value: string): string {
		try {
			const name = new Intl.DisplayNames([value], { type: 'language' }).of(value);
			if (!name) return value;
			return name.charAt(0).toLocaleUpperCase(value) + name.slice(1);
		} catch {
			return value;
		}
	}

	async function handleChange(event: Event) {
		const select = event.currentTarget as HTMLSelectElement;
		const next = select.value as Locale;
		if (next === $locale) return;

		// Switch first so the UI responds immediately, then persist in the background
		changeLocale(next);

		if (!auth.isAuthenticated()) return;
		saving = true;
		try {
			const response = await fetchApi('/users/preferences', {
				method: 'PUT',
				body: JSON.stringify({ preferences: { language: next } })
			});
			if (!response.ok) throw new Error(String(response.status));
		} catch (error) {
			console.error('Failed to update language preference:', error);
			toasts.error(m.profile_language_save_error());
		} finally {
			saving = false;
		}
	}
</script>

<select
	value={$locale}
	onchange={handleChange}
	disabled={locales.length === 1 || saving}
	aria-label={m.profile_language_label()}
	class="rounded-md border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800 px-3 py-2 text-sm text-gray-900 dark:text-gray-100 disabled:opacity-60"
>
	{#each locales as value (value)}
		<option {value}>{displayName(value)}</option>
	{/each}
</select>
