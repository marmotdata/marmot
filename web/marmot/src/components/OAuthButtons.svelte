<script lang="ts">
	import { onMount } from 'svelte';
	import { fetchApi } from '$lib/api';
	import Button from './Button.svelte';

	let enabledProviders: string[] = [];
	let loading = true;
	let error = '';

	onMount(async () => {
		try {
			const response = await fetchApi('/auth/config');
			if (!response.ok) {
				throw new Error('Failed to fetch auth configuration');
			}
			const data = await response.json();
			enabledProviders = data.enabled_providers;
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load OAuth providers';
		} finally {
			loading = false;
		}
	});

	function handleOktaLogin() {
		const currentPath = window.location.pathname + window.location.search;
		const returnTo = encodeURIComponent(currentPath);
		window.location.href = `/api/v1/auth/okta/login?returnTo=${returnTo}`;
	}
</script>

{#if loading}
	<div class="flex justify-center p-4">
		<div
			class="animate-spin rounded-full h-6 w-6 border-b-2 border-gray-900 dark:border-gray-100"
		></div>
	</div>
{:else if error}
	<div class="text-red-600 dark:text-red-400 text-center p-4">
		{error}
	</div>
{:else if enabledProviders.length > 0}
	<div class="relative my-8">
		<div class="absolute inset-0 flex items-center">
			<div class="w-full border-t border-gray-300 dark:border-gray-600"></div>
		</div>
		<div class="relative flex justify-center text-sm">
			<span class="px-2 bg-white dark:bg-gray-800 text-gray-500 dark:text-gray-400"
				>Or continue with</span
			>
		</div>
	</div>

	<div class="mt-6 space-y-4">
		{#if enabledProviders.includes('okta')}
			<Button
				variant="clear"
				icon="person"
				text="Sign in with Okta"
				class="w-full justify-center border border-gray-300 dark:border-gray-600"
				click={handleOktaLogin}
			/>
		{/if}
	</div>
{/if}
