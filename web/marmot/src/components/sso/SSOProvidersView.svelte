<script lang="ts">
	import { onMount } from 'svelte';
	import IconifyIcon from '@iconify/svelte';
	import Panel from '$components/ui/Panel.svelte';
	import Pill from '$components/ui/Pill.svelte';
	import Alert from '$components/ui/Alert.svelte';
	import Skeleton from '$components/ui/Skeleton.svelte';
	import EmptyState from '$components/ui/EmptyState.svelte';
	import { listSSOProviders } from '$lib/sso/api';
	import { m } from '$lib/paraglide/messages';
	import type { SSOProvider } from '$lib/sso/types';

	// The sign-in providers the server was started with, as read-only cards, since they are defined in the server config rather than here
	let providers = $state<SSOProvider[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);

	async function fetchAll() {
		try {
			loading = true;
			error = null;
			providers = await listSSOProviders();
		} catch (err) {
			error = err instanceof Error ? err.message : m.sso_load_error();
		} finally {
			loading = false;
		}
	}

	onMount(fetchAll);
</script>

<Panel
	title={m.sso_providers_heading()}
	description={m.sso_config_note_body({ file: 'config.yaml', section: 'auth.*' })}
	icon="material-symbols:login"
>
	{#snippet action()}
		<Pill>
			<IconifyIcon icon="material-symbols:description-outline" class="h-3.5 w-3.5" />
			{m.sso_config_note_title()}
		</Pill>
	{/snippet}

	{#if loading}
		<div class="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
			<Skeleton class="h-20" />
			<Skeleton class="h-20" />
		</div>
	{:else if error}
		<Alert>{error}</Alert>
	{:else if providers.length === 0}
		<EmptyState icon="material-symbols:login" title={m.sso_none_configured()} />
	{:else}
		<ul class="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
			{#each providers as provider (provider.name)}
				<li
					class="flex items-start gap-3 rounded-xl border border-gray-200 bg-white p-4 dark:border-gray-700 dark:bg-gray-800"
				>
					<span
						class="flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-lg border border-gray-100 bg-gray-50 dark:border-gray-700 dark:bg-gray-900"
					>
						<IconifyIcon icon="material-symbols:passkey-outline" class="h-5 w-5 text-gray-500" />
					</span>
					<span class="min-w-0 flex-1">
						<span class="flex items-center gap-2">
							<span class="truncate text-sm font-semibold text-gray-900 dark:text-gray-100"
								>{provider.name}</span
							>
							<Pill>{provider.type}</Pill>
						</span>
						<span
							class="mt-1 block truncate font-mono text-xs text-gray-500 dark:text-gray-400"
							title={provider.issuer_url}
						>
							{provider.issuer_url || '-'}
						</span>
					</span>
				</li>
			{/each}
		</ul>
	{/if}
</Panel>
