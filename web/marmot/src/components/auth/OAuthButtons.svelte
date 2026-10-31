<script lang="ts">
	import { onMount } from 'svelte';
	import { fetchApi } from '$lib/api';
	import { m } from '$lib/paraglide/messages';
	import { signInProviders } from './providers';

	let enabled = $state<string[]>([]);

	const providers = $derived(signInProviders.filter((p) => enabled.includes(p.id)));

	// A failed lookup only hides the provider buttons; the username and password form still works
	onMount(async () => {
		try {
			const response = await fetchApi('/auth-providers', { skipAuth: true, prefix: '' });
			if (!response.ok) return;
			const data = await response.json();
			enabled = data.enabled_providers ?? [];
		} catch {
			enabled = [];
		}
	});

	function signInWith(provider: string) {
		window.location.href = `/auth/${provider}/login`;
	}
</script>

{#if providers.length}
	<div class="mm-stack" style="--mm-gap: var(--space-2)">
		{#each providers as p (p.id)}
			<mm-button class="block" onclick={() => signInWith(p.id)}>
				<mm-provider-logo slot="icon" provider={p.logo} size={16} decorative></mm-provider-logo>
				{p.name ? m.oauth_signin_with({ provider: p.name }) : m.oauth_signin_sso()}
			</mm-button>
		{/each}
	</div>
	<div class="divider">
		<hr class="mm-rule" />
		<mm-text variant="caption" tone="secondary">{m.oauth_continue_with()}</mm-text>
		<hr class="mm-rule" />
	</div>
{/if}

<style>
	.block {
		display: flex;
	}
	.divider {
		display: grid;
		grid-template-columns: 1fr auto 1fr;
		align-items: center;
		gap: var(--space-3);
	}
</style>
