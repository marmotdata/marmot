<script lang="ts">
	import { SvelteMap } from 'svelte/reactivity';
	import IconifyIcon from '@iconify/svelte';
	import { fetchApi } from '$lib/api';
	import { previewRule } from '$lib/dataproducts/api';
	import type { Owner, RuleInput } from '$lib/dataproducts/types';
	import type { Asset } from '$lib/assets/types';
	import AssetIcon from '$components/ui/Icon.svelte';
	import { m } from '$lib/paraglide/messages';

	const SAMPLE_SIZE = 5;

	let {
		mode = 'create',
		name,
		description,
		owners,
		tags,
		manualAssetIds,
		assetDetails,
		rules
	}: {
		mode?: 'create' | 'edit';
		name: string;
		description: string;
		owners: Owner[];
		tags: string[];
		manualAssetIds: string[];
		assetDetails: SvelteMap<string, Asset>;
		rules: RuleInput[];
	} = $props();

	interface RuleMatch {
		count: number;
		ids: string[];
		error: boolean;
	}

	let ruleMatches = $state<(RuleMatch | null)[]>([]);
	let isChecking = $state(false);
	const fetchedAssets = new SvelteMap<string, Asset | null>();
	let previewToken = 0;

	function isPreviewable(rule: RuleInput): boolean {
		return rule.is_enabled && !!rule.query_expression?.trim();
	}

	// Rule previews are debounced and tagged with a token so a slow response for an older rule set never overwrites the current one.
	$effect(() => {
		const snapshot = rules.map((r) => ({
			name: r.name,
			rule_type: r.rule_type,
			query_expression: r.query_expression,
			is_enabled: r.is_enabled
		}));
		const token = ++previewToken;

		if (!snapshot.some(isPreviewable)) {
			ruleMatches = snapshot.map(() => null);
			isChecking = false;
			return;
		}

		isChecking = true;
		const timer = setTimeout(async () => {
			const results = await Promise.all(
				snapshot.map(async (rule): Promise<RuleMatch | null> => {
					if (!isPreviewable(rule)) return null;
					try {
						const preview = await previewRule(rule);
						return {
							count: preview.asset_count ?? 0,
							ids: preview.asset_ids ?? [],
							error: (preview.errors?.length ?? 0) > 0
						};
					} catch {
						return { count: 0, ids: [], error: true };
					}
				})
			);
			if (token !== previewToken) return;
			ruleMatches = results;
			isChecking = false;
		}, 400);

		return () => clearTimeout(timer);
	});

	let resolvedIds = $derived.by(() => {
		return Array.from(
			new Set([...manualAssetIds, ...ruleMatches.flatMap((match) => match?.ids ?? [])])
		);
	});

	// The preview endpoint caps the IDs it returns, so when any rule has more matches than IDs the overlap is unknown and only an upper bound can be shown.
	let isExact = $derived(ruleMatches.every((r) => !r || r.ids.length >= r.count));
	let upperBound = $derived(
		manualAssetIds.length + ruleMatches.reduce((sum, r) => sum + (r?.count ?? 0), 0)
	);
	let totalLabel = $derived(
		isExact
			? m.products_summary_assets_total({ count: resolvedIds.length })
			: m.products_summary_assets_up_to({ count: upperBound })
	);
	let sampleIds = $derived(resolvedIds.slice(0, SAMPLE_SIZE));
	let remaining = $derived((isExact ? resolvedIds.length : upperBound) - sampleIds.length);

	$effect(() => {
		for (const id of sampleIds) {
			if (assetDetails.has(id) || fetchedAssets.has(id)) continue;
			fetchedAssets.set(id, null);
			fetchApi(`/assets/${id}`)
				.then(async (res) => {
					if (res.ok) fetchedAssets.set(id, await res.json());
				})
				.catch(() => {});
		}
	});

	function sampleAsset(id: string): Asset | null | undefined {
		return assetDetails.get(id) ?? fetchedAssets.get(id);
	}

	function iconFor(asset: Asset): string {
		if (asset.providers && asset.providers.length === 1) return asset.providers[0];
		return asset.type || 'unknown';
	}
</script>

<aside
	class="rounded-xl border border-gray-200 bg-white p-6 dark:border-gray-700 dark:bg-gray-800"
	aria-live="polite"
>
	<h2 class="text-sm font-semibold text-gray-900 dark:text-gray-100">
		{m.products_summary_heading()}
	</h2>
	<p class="mt-1 text-sm text-gray-600 dark:text-gray-400">
		{mode === 'create' ? m.products_summary_create_hint() : m.products_summary_edit_hint()}
	</p>

	<div class="mt-5 space-y-5">
		<div>
			<p
				class="break-words text-base font-semibold {name.trim()
					? 'text-gray-900 dark:text-gray-100'
					: 'text-gray-400 dark:text-gray-500'}"
			>
				{name.trim() || m.products_summary_untitled()}
			</p>
			{#if description.trim()}
				<p class="mt-1 line-clamp-3 text-sm text-gray-600 dark:text-gray-400">
					{description.trim()}
				</p>
			{/if}
			{#if tags.length > 0}
				<div class="mt-2 flex flex-wrap gap-1.5">
					{#each tags as tag (tag)}
						<span
							class="rounded-full bg-gray-100 px-2 py-0.5 text-xs text-gray-700 dark:bg-gray-700 dark:text-gray-300"
						>
							{tag}
						</span>
					{/each}
				</div>
			{/if}
		</div>

		<div>
			<h3 class="text-xs font-medium uppercase tracking-wide text-gray-500 dark:text-gray-400">
				{m.common_owners()}
			</h3>
			{#if owners.length > 0}
				<ul class="mt-2 space-y-1.5">
					{#each owners as owner (owner.type + owner.id)}
						<li class="flex items-center gap-2 text-sm text-gray-800 dark:text-gray-200">
							<IconifyIcon
								icon={owner.type === 'team' ? 'material-symbols:group' : 'material-symbols:person'}
								class="h-4 w-4 flex-shrink-0 text-gray-400"
							/>
							<span class="truncate">{owner.name || owner.username || owner.id}</span>
						</li>
					{/each}
				</ul>
			{:else}
				<p class="mt-2 text-sm text-gray-500 dark:text-gray-400">
					{m.products_summary_no_owners()}
				</p>
			{/if}
		</div>

		<div>
			<div class="flex items-center justify-between gap-2">
				<h3 class="text-xs font-medium uppercase tracking-wide text-gray-500 dark:text-gray-400">
					{m.products_assets()}
				</h3>
				{#if isChecking}
					<span class="inline-flex items-center gap-1 text-xs text-gray-500 dark:text-gray-400">
						<IconifyIcon icon="mdi:loading" class="h-3.5 w-3.5 animate-spin" />
						{m.products_summary_checking()}
					</span>
				{/if}
			</div>

			{#if manualAssetIds.length === 0 && rules.length === 0}
				<p class="mt-2 text-sm text-gray-500 dark:text-gray-400">
					{m.products_summary_no_assets()}
				</p>
			{:else}
				<p class="mt-2 text-2xl font-semibold tabular-nums text-gray-900 dark:text-gray-100">
					{totalLabel}
				</p>
				<ul class="mt-2 space-y-1 text-sm text-gray-600 dark:text-gray-400">
					<li class="flex items-center justify-between gap-2">
						<span>{m.products_manual_assets_heading()}</span>
						<span class="tabular-nums">{manualAssetIds.length}</span>
					</li>
					{#each rules as rule, i (rule.id ?? i)}
						{@const match = ruleMatches[i]}
						<li class="flex items-center justify-between gap-2">
							<span class="truncate">{rule.name}</span>
							<span class="flex-shrink-0 tabular-nums">
								{#if !rule.is_enabled}
									{m.products_summary_rule_disabled()}
								{:else if match?.error}
									<span
										class="text-red-600 dark:text-red-400"
										title={m.products_summary_rule_error()}
									>
										<IconifyIcon icon="material-symbols:error-outline" class="inline h-4 w-4" />
										<span class="sr-only">{m.products_summary_rule_error()}</span>
									</span>
								{:else if match}
									{m.products_matched_count({ count: match.count })}
								{:else}
									&ndash;
								{/if}
							</span>
						</li>
					{/each}
				</ul>

				{#if sampleIds.length > 0}
					<h4 class="mt-4 text-xs font-medium text-gray-500 dark:text-gray-400">
						{m.products_summary_sample_heading()}
					</h4>
					<ul class="mt-2 space-y-1.5">
						{#each sampleIds as id (id)}
							{@const asset = sampleAsset(id)}
							<li class="flex min-w-0 items-center gap-2 text-sm">
								{#if asset}
									<AssetIcon name={iconFor(asset)} size="xs" showLabel={false} />
									<span class="truncate text-gray-800 dark:text-gray-200">{asset.name}</span>
								{:else}
									<span class="h-4 w-full animate-pulse rounded bg-gray-100 dark:bg-gray-700"
									></span>
								{/if}
							</li>
						{/each}
					</ul>
					{#if remaining > 0}
						<p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
							{m.products_summary_more_assets({ count: remaining })}
						</p>
					{/if}
				{/if}
			{/if}
		</div>
	</div>
</aside>
