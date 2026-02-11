<script lang="ts">
	import { onMount } from 'svelte';
	import { fetchApi } from '$lib/api';
	import Button from '$components/ui/Button.svelte';

	interface Props {
		redirectUri?: string;
	}

	let { redirectUri = '' }: Props = $props();

	let enabledProviders = $state<string[]>([]);
	let loading = $state(true);
	let error = $state('');

	onMount(async () => {
		try {
			const response = await fetchApi('/auth-providers', { skipAuth: true, prefix: '' });
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

	function handleOAuthLogin(provider: string) {
		let url = `/auth/${provider}/login`;

		if (redirectUri) {
			url += `?redirect_uri=${encodeURIComponent(redirectUri)}`;
		} else {
			const currentPath = window.location.pathname + window.location.search;
			const returnTo = encodeURIComponent(currentPath);
			url += `?returnTo=${returnTo}`;
		}

		window.location.href = url;
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

	<div class="mt-6 space-y-3">
		{#if enabledProviders.includes('google')}
			<Button
				variant="clear"
				icon="simple-icons:google"
				text="Sign in with Google"
				class="w-full justify-center border border-gray-300 dark:border-gray-600 hover:border-gray-400 dark:hover:border-gray-500"
				click={() => handleOAuthLogin('google')}
			/>
		{/if}
		{#if enabledProviders.includes('github')}
			<Button
				variant="clear"
				icon="simple-icons:github"
				text="Sign in with GitHub"
				class="w-full justify-center border border-gray-300 dark:border-gray-600 hover:border-gray-400 dark:hover:border-gray-500"
				click={() => handleOAuthLogin('github')}
			/>
		{/if}
		{#if enabledProviders.includes('gitlab')}
			<Button
				variant="clear"
				icon="simple-icons:gitlab"
				text="Sign in with GitLab"
				class="w-full justify-center border border-gray-300 dark:border-gray-600 hover:border-gray-400 dark:hover:border-gray-500"
				click={() => handleOAuthLogin('gitlab')}
			/>
		{/if}
		{#if enabledProviders.includes('keycloak')}
			<Button
				variant="clear"
				icon="simple-icons:keycloak"
				text="Sign in with Keycloak"
				class="w-full justify-center border border-gray-300 dark:border-gray-600 hover:border-gray-400 dark:hover:border-gray-500"
				click={() => handleOAuthLogin('keycloak')}
			/>
		{/if}
		{#if enabledProviders.includes('okta')}
			<Button
				variant="clear"
				icon="simple-icons:okta"
				text="Sign in with Okta"
				class="w-full justify-center border border-gray-300 dark:border-gray-600 hover:border-gray-400 dark:hover:border-gray-500"
				click={() => handleOAuthLogin('okta')}
			/>
		{/if}
		{#if enabledProviders.includes('slack')}
			<Button
				variant="clear"
				icon="simple-icons:slack"
				text="Sign in with Slack"
				class="w-full justify-center border border-gray-300 dark:border-gray-600 hover:border-gray-400 dark:hover:border-gray-500"
				click={() => handleOAuthLogin('slack')}
			/>
		{/if}
		{#if enabledProviders.includes('auth0')}
			<Button
				variant="clear"
				icon="simple-icons:auth0"
				text="Sign in with Auth0"
				class="w-full justify-center border border-gray-300 dark:border-gray-600 hover:border-gray-400 dark:hover:border-gray-500"
				click={() => handleOAuthLogin('auth0')}
			/>
		{/if}
	</div>
{/if}
