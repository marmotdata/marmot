<script lang="ts">
	import { onMount } from 'svelte';
	import { Info } from 'lucide-svelte';
	import { listSSOProviders } from '$lib/sso/api';
	import type { SSOProvider } from '$lib/sso/types';

	let providers: SSOProvider[] = [];
	let loading = false;
	let error: string | null = null;

	async function fetchAll() {
		try {
			loading = true;
			error = null;
			providers = await listSSOProviders();
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load SSO providers';
		} finally {
			loading = false;
		}
	}

	onMount(fetchAll);
</script>

<div
	class="bg-earthy-brown-50 dark:bg-gray-900 rounded-lg border border-gray-200 dark:border-gray-700"
>
	<div class="p-6 space-y-4">
		<div
			class="flex gap-3 rounded-md border border-earthy-brown-200 dark:border-gray-700 bg-earthy-brown-100 dark:bg-gray-800 p-4"
		>
			<Info
				class="h-5 w-5 text-earthy-terracotta-700 dark:text-earthy-terracotta-500 shrink-0 mt-0.5"
			/>
			<div class="text-sm text-gray-700 dark:text-gray-300">
				<p class="font-medium text-gray-900 dark:text-gray-100">Configured via server config</p>
				<p class="mt-1">
					SSO providers for human sign-in are defined in <code
						class="px-1 py-0.5 rounded bg-earthy-brown-200 dark:bg-gray-700 text-xs"
						>config.yaml</code
					>
					under
					<code class="px-1 py-0.5 rounded bg-earthy-brown-200 dark:bg-gray-700 text-xs"
						>auth.*</code
					>. Restart Marmot after changes.
				</p>
			</div>
		</div>

		{#if loading}
			<div class="flex justify-center p-8">
				<div
					class="animate-spin rounded-full h-8 w-8 border-b-2 border-earthy-terracotta-700"
				></div>
			</div>
		{:else if error}
			<div class="bg-red-50 border border-red-200 rounded-lg p-4 text-red-700">
				{error}
			</div>
		{:else if providers.length === 0}
			<p class="text-sm text-gray-500 dark:text-gray-400 text-center py-8">
				No SSO providers configured
			</p>
		{:else}
			<div class="overflow-x-auto">
				<table class="min-w-full">
					<thead>
						<tr>
							<th
								class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider bg-earthy-brown-100 dark:bg-gray-800"
								>Name</th
							>
							<th
								class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider bg-earthy-brown-100 dark:bg-gray-800"
								>Type</th
							>
							<th
								class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider bg-earthy-brown-100 dark:bg-gray-800"
								>Issuer URL</th
							>
						</tr>
					</thead>
					<tbody class="divide-y divide-earthy-brown-100 bg-earthy-brown-50 dark:bg-gray-900">
						{#each providers as provider (provider.name)}
							<tr>
								<td class="px-6 py-4 whitespace-nowrap">
									<span class="text-sm font-medium text-gray-900 dark:text-gray-100"
										>{provider.name}</span
									>
								</td>
								<td class="px-6 py-4 whitespace-nowrap">
									<span
										class="inline-flex items-center rounded-full bg-earthy-brown-200 dark:bg-gray-700 px-2 py-0.5 text-xs font-medium text-gray-800 dark:text-gray-200"
									>
										{provider.type}
									</span>
								</td>
								<td class="px-6 py-4 whitespace-nowrap">
									{#if provider.issuer_url}
										<span class="text-sm text-gray-600 dark:text-gray-400 font-mono"
											>{provider.issuer_url}</span
										>
									{:else}
										<span class="text-sm text-gray-400 dark:text-gray-500">—</span>
									{/if}
								</td>
							</tr>
						{/each}
					</tbody>
				</table>
			</div>
		{/if}
	</div>
</div>
