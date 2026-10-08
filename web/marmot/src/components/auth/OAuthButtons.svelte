<script lang="ts">
	import { onMount } from 'svelte';
	import { fetchApi } from '$lib/api';
	import { m } from '$lib/paraglide/messages';
	import Button from '$components/ui/Button.svelte';

	let enabledProviders = $state<string[]>([]);
	let loading = $state(true);
	let error = $state('');

	onMount(async () => {
		try {
			const response = await fetchApi('/auth-providers', { skipAuth: true, prefix: '' });
			if (!response.ok) {
				throw new Error(m.oauth_fetch_config_error());
			}
			const data = await response.json();
			enabledProviders = data.enabled_providers;
		} catch (err) {
			error = err instanceof Error ? err.message : m.oauth_load_error();
		} finally {
			loading = false;
		}
	});

	function handleOAuthLogin(provider: string) {
		window.location.href = `/auth/${provider}/login`;
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
				>{m.oauth_continue_with()}</span
			>
		</div>
	</div>

	<div class="mt-6 space-y-3">
		{#if enabledProviders.includes('google')}
			<Button
				variant="clear"
				icon="simple-icons:google"
				text={m.oauth_signin_with({ provider: 'Google' })}
				class="w-full justify-center border border-gray-300 dark:border-gray-600 hover:border-gray-400 dark:hover:border-gray-500"
				click={() => handleOAuthLogin('google')}
			/>
		{/if}
		{#if enabledProviders.includes('github')}
			<Button
				variant="clear"
				icon="simple-icons:github"
				text={m.oauth_signin_with({ provider: 'GitHub' })}
				class="w-full justify-center border border-gray-300 dark:border-gray-600 hover:border-gray-400 dark:hover:border-gray-500"
				click={() => handleOAuthLogin('github')}
			/>
		{/if}
		{#if enabledProviders.includes('gitlab')}
			<Button
				variant="clear"
				icon="simple-icons:gitlab"
				text={m.oauth_signin_with({ provider: 'GitLab' })}
				class="w-full justify-center border border-gray-300 dark:border-gray-600 hover:border-gray-400 dark:hover:border-gray-500"
				click={() => handleOAuthLogin('gitlab')}
			/>
		{/if}
		{#if enabledProviders.includes('keycloak')}
			<Button
				variant="clear"
				icon="simple-icons:keycloak"
				text={m.oauth_signin_with({ provider: 'Keycloak' })}
				class="w-full justify-center border border-gray-300 dark:border-gray-600 hover:border-gray-400 dark:hover:border-gray-500"
				click={() => handleOAuthLogin('keycloak')}
			/>
		{/if}
		{#if enabledProviders.includes('okta')}
			<Button
				variant="clear"
				icon="simple-icons:okta"
				text={m.oauth_signin_with({ provider: 'Okta' })}
				class="w-full justify-center border border-gray-300 dark:border-gray-600 hover:border-gray-400 dark:hover:border-gray-500"
				click={() => handleOAuthLogin('okta')}
			/>
		{/if}
		{#if enabledProviders.includes('slack')}
			<Button
				variant="clear"
				icon="simple-icons:slack"
				text={m.oauth_signin_with({ provider: 'Slack' })}
				class="w-full justify-center border border-gray-300 dark:border-gray-600 hover:border-gray-400 dark:hover:border-gray-500"
				click={() => handleOAuthLogin('slack')}
			/>
		{/if}
		{#if enabledProviders.includes('auth0')}
			<Button
				variant="clear"
				icon="simple-icons:auth0"
				text={m.oauth_signin_with({ provider: 'Auth0' })}
				class="w-full justify-center border border-gray-300 dark:border-gray-600 hover:border-gray-400 dark:hover:border-gray-500"
				click={() => handleOAuthLogin('auth0')}
			/>
		{/if}
		{#if enabledProviders.includes('generic_oidc')}
			<Button
				variant="clear"
				icon="mdi:shield-key-outline"
				text={m.oauth_signin_sso()}
				class="w-full justify-center border border-gray-300 dark:border-gray-600 hover:border-gray-400 dark:hover:border-gray-500"
				click={() => handleOAuthLogin('generic_oidc')}
			/>
		{/if}
	</div>
{/if}
