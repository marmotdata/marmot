<script lang="ts">
	import { resolve } from '$app/paths';
	import IconifyIcon from '@iconify/svelte';
	import AssetIcon from '$components/ui/Icon.svelte';
	import { fetchApi } from '$lib/api';
	import { previewAssetRule } from '$lib/assetrules/api';
	import type { Asset } from '$lib/assets/types';
	import { m } from '$lib/paraglide/messages';

	let { query }: { query: string } = $props();

	const SAMPLE_SIZE = 5;
	const DEBOUNCE_MS = 400;

	let loading = $state(false);
	let error = $state<string | null>(null);
	let count = $state<number | null>(null);
	let sample = $state<Asset[]>([]);

	let trimmed = $derived(query.trim());

	// A sequence number lets a slow response for an older query be discarded once the user has typed further
	let requestSeq = 0;

	async function runPreview(expression: string, seq: number) {
		try {
			const result = await previewAssetRule({
				rule_type: 'query',
				query_expression: expression,
				limit: SAMPLE_SIZE
			});
			const ids = (result.asset_ids || []).slice(0, SAMPLE_SIZE);
			const assets = await Promise.all(
				ids.map(async (id) => {
					try {
						const resp = await fetchApi(`/assets/${id}`);
						return resp.ok ? ((await resp.json()) as Asset) : null;
					} catch {
						return null;
					}
				})
			);
			if (seq !== requestSeq) return;
			count = result.asset_count;
			sample = assets.filter((a): a is Asset => a !== null);
			error = result.errors?.length ? result.errors.join(' ') : null;
		} catch (e) {
			if (seq !== requestSeq) return;
			count = null;
			sample = [];
			error = e instanceof Error ? e.message : m.assetrules_preview_error();
		} finally {
			if (seq === requestSeq) loading = false;
		}
	}

	$effect(() => {
		const expression = trimmed;
		const seq = ++requestSeq;
		if (!expression) {
			loading = false;
			error = null;
			count = null;
			sample = [];
			return;
		}
		loading = true;
		const timer = setTimeout(() => runPreview(expression, seq), DEBOUNCE_MS);
		return () => clearTimeout(timer);
	});

	function assetParams(asset: Asset) {
		const parts = (asset.mrn ?? '').replace('mrn://', '').split('/');
		if (parts.length >= 3)
			return { type: parts[0], service: parts[1], name: parts.slice(2).join('/') };
		return { type: asset.type, service: asset.providers?.[0] ?? '', name: asset.name };
	}

	function getIconType(asset: Asset): string {
		if (asset.providers?.length === 1) return asset.providers[0];
		return asset.type || 'unknown';
	}
</script>

<div
	class="rounded-xl border border-gray-200 bg-white p-6 dark:border-gray-700 dark:bg-gray-800"
	aria-live="polite"
>
	<div class="flex items-center justify-between gap-2">
		<h2 class="text-sm font-semibold text-gray-900 dark:text-gray-100">
			{m.assetrules_preview_panel_heading()}
		</h2>
		{#if loading}
			<IconifyIcon icon="mdi:loading" class="h-4 w-4 animate-spin text-gray-400" />
		{/if}
	</div>

	{#if !trimmed}
		<div class="mt-6 flex flex-col items-center text-center">
			<div
				class="flex h-12 w-12 items-center justify-center rounded-xl bg-gray-100 dark:bg-gray-700"
			>
				<IconifyIcon icon="material-symbols:filter-list" class="h-6 w-6 text-gray-400" />
			</div>
			<p class="mt-3 text-sm font-medium text-gray-900 dark:text-gray-100">
				{m.assetrules_preview_empty_heading()}
			</p>
			<p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
				{m.assetrules_preview_empty_hint()}
			</p>
		</div>
	{:else if error}
		<div
			class="mt-4 flex items-start gap-2 rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-700 dark:border-red-800/50 dark:bg-red-900/20 dark:text-red-300"
		>
			<IconifyIcon icon="material-symbols:error" class="mt-0.5 h-4 w-4 flex-shrink-0" />
			<span>{error}</span>
		</div>
	{:else if count === null}
		<p class="mt-4 text-sm text-gray-500 dark:text-gray-400">{m.assetrules_preview_loading()}</p>
	{:else}
		<p
			class="mt-3 text-lg font-semibold text-gray-900 dark:text-gray-100 {loading
				? 'opacity-60'
				: ''}"
		>
			{m.assetrules_rule_matches_count({ count })}
		</p>
		{#if count === 0}
			<p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
				{m.assetrules_preview_no_matches_hint()}
			</p>
		{:else if sample.length > 0}
			<ul class="mt-4 divide-y divide-gray-100 dark:divide-gray-700 {loading ? 'opacity-60' : ''}">
				{#each sample as asset (asset.id)}
					<li class="py-2">
						<a
							href={resolve('/discover/[type]/[service]/[name]', assetParams(asset))}
							target="_blank"
							rel="noopener"
							class="group flex items-center gap-3"
						>
							<span class="flex-shrink-0 rounded-md bg-gray-100 p-1.5 dark:bg-gray-700">
								<AssetIcon name={getIconType(asset)} size="xs" showLabel={false} />
							</span>
							<span class="min-w-0">
								<span
									class="block truncate text-sm font-medium text-gray-900 group-hover:text-earthy-terracotta-700 dark:text-gray-100 dark:group-hover:text-earthy-terracotta-400"
								>
									{asset.name}
								</span>
								<span class="block truncate font-mono text-xs text-gray-500 dark:text-gray-400">
									{asset.mrn}
								</span>
							</span>
						</a>
					</li>
				{/each}
			</ul>
			{#if count > sample.length}
				<p class="mt-3 text-xs text-gray-500 dark:text-gray-400">
					{m.assetrules_preview_showing({ shown: sample.length, total: count })}
				</p>
			{/if}
		{/if}
	{/if}
</div>
