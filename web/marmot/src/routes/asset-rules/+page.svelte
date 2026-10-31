<script lang="ts">
	import { onMount } from 'svelte';
	import { resolve } from '$app/paths';
	import { auth } from '$lib/stores/auth';
	import { toasts } from '$lib/stores/toast';
	import { searchAssetRules, updateAssetRule } from '$lib/assetrules/api';
	import { plainQuery } from '$lib/assetrules/plain';
	import type { AssetRule } from '$lib/assetrules/types';
	import { setCrumbs } from '$lib/mm';
	import { m } from '$lib/paraglide/messages';
	import RuleAdds from '$components/assetrule/RuleAdds.svelte';

	const PAGE_SIZE = 50;

	let rules = $state<AssetRule[]>([]);
	let total = $state(0);
	let page = $state(1);
	let isLoading = $state(true);
	let error = $state<string | null>(null);
	let searchQuery = $state('');
	let searchTimeout: ReturnType<typeof setTimeout>;
	let toggling = $state<Record<string, boolean>>({});

	let canManage = $derived(auth.hasPermission('assets', 'manage'));

	setCrumbs([{ label: m.assetrules_heading() }]);

	onMount(() => {
		loadRules();
	});

	async function loadRules() {
		isLoading = true;
		error = null;
		try {
			const result = await searchAssetRules(searchQuery, (page - 1) * PAGE_SIZE, PAGE_SIZE);
			rules = result.asset_rules || [];
			total = result.total ?? rules.length;
		} catch (e: unknown) {
			error = e instanceof Error ? e.message : m.assetrules_load_error();
		} finally {
			isLoading = false;
		}
	}

	function handleSearch(e: Event) {
		searchQuery = (e.target as HTMLInputElement).value;
		clearTimeout(searchTimeout);
		searchTimeout = setTimeout(() => {
			page = 1;
			loadRules();
		}, 300);
	}

	async function toggle(rule: AssetRule, checked: boolean) {
		const previous = rule.is_enabled;
		rule.is_enabled = checked;
		toggling[rule.id] = true;
		try {
			const updated = await updateAssetRule(rule.id, { is_enabled: checked });
			rules = rules.map((r) => (r.id === updated.id ? updated : r));
		} catch (e) {
			rule.is_enabled = previous;
			toasts.error(e instanceof Error ? e.message : m.assetrules_update_error());
		} finally {
			toggling[rule.id] = false;
		}
	}
</script>

<svelte:head>
	<title>{m.assetrules_page_title()}</title>
</svelte:head>

<div class="mm-stack" style="--mm-gap: var(--space-5)">
	<mm-page-header
		heading={m.assetrules_heading()}
		compact
		count={error || isLoading ? undefined : total}
		description={m.assetrules_subheading()}
	>
		{#if canManage}
			<mm-button slot="actions" variant="primary" icon="add" href={resolve('/asset-rules/new')}
				>{m.assetrules_create_rule_button()}</mm-button
			>
		{/if}
	</mm-page-header>

	{#if !isLoading && !error && total === 0 && !searchQuery}
		<mm-empty-state
			heading={m.assetrules_empty_heading()}
			description={m.assetrules_empty_description()}
			seed={13}
		>
			{#if canManage}
				<mm-button slot="action" variant="primary" icon="add" href={resolve('/asset-rules/new')}
					>{m.assetrules_create_rule_button()}</mm-button
				>
			{/if}
		</mm-empty-state>
	{:else}
		<mm-search-field
			class="search"
			label={m.assetrules_search_label()}
			hide-label
			placeholder={m.assetrules_search_placeholder()}
			shortcut="/"
			global-shortcut
			value={searchQuery}
			oninput={handleSearch}
		></mm-search-field>

		{#if error}
			<mm-callout tone="danger" heading={m.assetrules_load_error_heading()}>
				{error}
				<mm-button slot="actions" size="sm" icon="refresh" onclick={loadRules}
					>{m.common_retry()}</mm-button
				>
			</mm-callout>
		{:else if isLoading && rules.length === 0}
			<mm-skeleton loading count={5}></mm-skeleton>
		{:else if rules.length === 0}
			<mm-empty-state
				compact
				heading={m.assetrules_no_matching_heading()}
				description={m.assetrules_no_matching_hint()}
				seed={7}
			></mm-empty-state>
		{:else}
			<ul class="mm-list rules" aria-label={m.assetrules_heading()} aria-busy={isLoading}>
				{#each rules as rule (rule.id)}
					{@const plain = plainQuery(rule.query_expression)}
					<li>
						<div class="mm-stack body" style="--mm-gap: var(--space-2)">
							<a
								class="mm-link mm-link--quiet name"
								href={resolve('/asset-rules/[id]', { id: rule.id })}>{rule.name}</a
							>
							<div class="facts">
								<mm-text variant="caption" tone="secondary">{m.assetrules_matches_label()}</mm-text>
								{#if plain}
									<mm-text variant="dense" block>{plain}</mm-text>
								{:else}
									<code class="query">{rule.query_expression}</code>
								{/if}
								<mm-text variant="caption" tone="secondary">{m.assetrules_adds_label()}</mm-text>
								<RuleAdds termIds={rule.term_ids} links={rule.links} />
							</div>
						</div>
						<div class="mm-stack state" style="--mm-gap: var(--space-2)">
							<mm-switch
								label={m.assetrules_apply_rule_aria({ name: rule.name })}
								hide-label
								checked={rule.is_enabled}
								disabled={!canManage || toggling[rule.id]}
								onmm-change={(e: CustomEvent<{ checked: boolean }>) =>
									toggle(rule, e.detail.checked)}
							></mm-switch>
							<mm-text variant="caption" tone="secondary" numeric>
								{rule.is_enabled
									? m.assetrules_asset_count({ count: rule.membership_count || 0 })
									: m.assetrules_off()}
							</mm-text>
						</div>
					</li>
				{/each}
			</ul>
			{#if total > PAGE_SIZE}
				<mm-pagination
					{total}
					page-size={PAGE_SIZE}
					{page}
					label={m.assetrules_pages_label()}
					onmm-page-change={(e: CustomEvent<{ page: number }>) => {
						page = e.detail.page;
						loadRules();
					}}
				></mm-pagination>
			{/if}
		{/if}
	{/if}
</div>

<style>
	.search {
		width: 22rem;
		max-width: 100%;
	}
	.rules {
		border-block: 1px solid var(--mm-rule);
	}
	.rules > li {
		gap: var(--space-4);
		padding-block: var(--space-4);
		align-items: flex-start;
	}
	.body {
		flex: 1;
		min-width: 0;
	}
	.name {
		font: var(--text-heading-sm);
	}
	.facts {
		display: grid;
		grid-template-columns: 5rem minmax(0, 1fr);
		gap: var(--space-2) var(--space-3);
		align-items: baseline;
	}
	.query {
		font: var(--text-mono-dense);
		color: var(--mm-text-2);
		overflow-wrap: anywhere;
	}
	.state {
		align-items: flex-end;
		flex: none;
	}
</style>
