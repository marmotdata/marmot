<script lang="ts">
	import '@marmotdata/elements/filter-pill.js';
	import '@marmotdata/elements/toolbar.js';
	import { resolve } from '$app/paths';
	import { m } from '$lib/paraglide/messages';
	import type { PluginInfo } from './pipeline';

	interface Props {
		plugins: PluginInfo[];
		// How many pipelines already use each plugin, by plugin id
		connected: Record<string, number>;
		canAdd: boolean;
		queryOnly?: boolean;
	}

	let { plugins, connected, canAdd, queryOnly = $bindable(false) }: Props = $props();

	// Every plugin Marmot can connect, grouped by what it connects to; picking one opens the create flow with it chosen
	type NewRoute = `/pipelines/new?${string}`;
	const FOLDED_PER_CATEGORY = 4;

	const CATEGORY_ORDER = [
		'database',
		'data-warehouse',
		'data-lake',
		'streaming',
		'messaging',
		'transformation',
		'etl',
		'orchestration',
		'dashboard',
		'storage',
		'catalog',
		'ml',
		'compute',
		'container',
		'api'
	];

	const CATEGORY_LABELS: Record<string, () => string> = {
		database: m.plugins_category_database,
		'data-warehouse': m.plugins_category_warehouse,
		'data-lake': m.plugins_category_data_lake,
		streaming: m.plugins_category_streaming,
		messaging: m.plugins_category_messaging,
		transformation: m.plugins_category_transformation,
		etl: m.plugins_category_etl,
		orchestration: m.plugins_category_orchestration,
		dashboard: m.plugins_category_dashboard,
		storage: m.plugins_category_storage,
		catalog: m.plugins_category_catalog,
		ml: m.plugins_category_ml,
		compute: m.plugins_category_compute,
		container: m.plugins_category_container,
		api: m.plugins_category_api
	};

	let search = $state('');
	let expanded = $state(false);

	let queryCount = $derived(plugins.filter((p) => p.supports_query).length);
	let matches = $derived.by(() => {
		const q = search.trim().toLowerCase();
		return plugins
			.filter((p) => !queryOnly || p.supports_query)
			.filter(
				(p) =>
					!q ||
					p.name.toLowerCase().includes(q) ||
					p.id.includes(q) ||
					p.description?.toLowerCase().includes(q) ||
					p.catalogs?.some((c) => c.toLowerCase().includes(q))
			)
			.sort((a, b) => a.name.localeCompare(b.name));
	});
	// Searching or filtering shows every match; otherwise each category starts folded to its first few plugins
	let folded = $derived(!expanded && !search.trim() && !queryOnly);
	let categories = $derived.by(() => {
		const byKey: Record<string, PluginInfo[]> = {};
		for (const p of matches) (byKey[p.category ?? 'other'] ??= []).push(p);
		const rank = (k: string) => {
			const i = CATEGORY_ORDER.indexOf(k);
			return i === -1 ? CATEGORY_ORDER.length : i;
		};
		return Object.entries(byKey)
			.sort(([a], [b]) => rank(a) - rank(b) || a.localeCompare(b))
			.map(([key, list]) => ({
				key,
				label: CATEGORY_LABELS[key]?.() ?? m.plugins_category_other(),
				list: folded ? list.slice(0, FOLDED_PER_CATEGORY) : list
			}));
	});
	let hidden = $derived(
		folded &&
			categories.some(
				(c) => c.list.length < matches.filter((p) => (p.category ?? 'other') === c.key).length
			)
	);

	function caption(p: PluginInfo): string {
		const uses = connected[p.id] ?? 0;
		return [
			p.supports_query ? m.plugins_filter_queryable() : '',
			p.produces_lineage ? m.plugins_lineage_tag() : '',
			uses ? m.plugins_connected_uses({ count: uses }) : ''
		]
			.filter(Boolean)
			.join(' · ');
	}
</script>

<mm-section
	id="gallery"
	heading={m.plugins_available_heading()}
	description={m.plugins_available_body()}
	style="scroll-margin-block-start: var(--space-8)"
>
	<div class="mm-stack" style="--mm-gap: var(--space-5)">
		<mm-toolbar label={m.plugins_filter_plugins_label()}>
			<mm-search-field
				label={m.pipelines_search_plugins_placeholder()}
				hide-label
				placeholder={m.pipelines_search_plugins_placeholder()}
				value={search}
				style="inline-size: min(18rem, 100%)"
				oninput={(e: Event) => (search = (e.currentTarget as HTMLInputElement).value)}
			></mm-search-field>
			{#if queryCount > 0}
				<mm-filter-pill
					pressed={queryOnly}
					count={queryCount}
					onmm-change={(e: CustomEvent<{ pressed: boolean }>) => (queryOnly = e.detail.pressed)}
					><mm-icon slot="start" name="terminal"
					></mm-icon>{m.plugins_filter_queryable()}</mm-filter-pill
				>
			{/if}
		</mm-toolbar>

		{#if matches.length === 0}
			<mm-empty-state
				compact
				heading={m.plugins_gallery_no_match_heading()}
				description={m.plugins_gallery_no_match_body()}
			></mm-empty-state>
		{:else}
			<div class="mm-columns" style="columns: 3 20rem">
				{#each categories as c (c.key)}
					<section aria-labelledby="plugin-cat-{c.key}">
						<mm-heading level={3} variant="panel-title" id="plugin-cat-{c.key}"
							>{c.label}</mm-heading
						>
						<mm-list aria-label={c.label}>
							{#each c.list as p (p.id)}
								<mm-list-item
									href={canAdd
										? resolve(`/pipelines/new?plugin=${encodeURIComponent(p.id)}` as NewRoute)
										: undefined}
									secondary={p.description}
								>
									<mm-provider-logo slot="start" provider={p.icon || p.id} size="20" decorative
									></mm-provider-logo>
									{p.name}
									{#if caption(p)}
										<mm-text slot="end" variant="caption" tone="secondary">{caption(p)}</mm-text>
									{/if}
								</mm-list-item>
							{/each}
						</mm-list>
					</section>
				{/each}
			</div>
		{/if}

		<div class="mm-cluster mm-cluster--between">
			<mm-text variant="dense" tone="secondary"
				>{m.plugins_missing_source()}
				<mm-link href="https://marmotdata.io/docs/Develop/creating-plugins" external
					>{m.plugins_write_plugin()}</mm-link
				></mm-text
			>
			{#if hidden}
				<mm-button variant="secondary" icon="expand-more" onclick={() => (expanded = true)}
					>{m.plugins_show_all()}</mm-button
				>
			{/if}
		</div>
	</div>
</mm-section>
