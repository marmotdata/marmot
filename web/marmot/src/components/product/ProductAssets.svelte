<script lang="ts">
	import { untrack } from 'svelte';
	import { resolve } from '$app/paths';
	import '@marmotdata/elements/popularity.js';
	import { toast } from '@marmotdata/elements/toast.js';
	import { fetchApi } from '$lib/api';
	import { m } from '$lib/paraglide/messages';
	import { getProductMembers, type MemberQuery } from '$lib/dataproducts/api';
	import { assetRouteParams, popularityLevel } from '$lib/dataproducts/assets';
	import type { DataProduct, Member, Rule } from '$lib/dataproducts/types';
	import type { Asset } from '$lib/assets/types';
	import AssetPicker from './AssetPicker.svelte';

	const PAGE = 25;

	// Assets are grouped by how they got here: those added manually, which can be removed, then those each rule brings in, which join and leave automatically.
	let {
		product,
		canManage = false,
		onChanged,
		onEditRule
	}: {
		product: DataProduct;
		canManage?: boolean;
		onChanged?: () => void | Promise<void>;
		onEditRule?: (rule: Rule) => void;
	} = $props();

	interface Group {
		/** null for the assets added manually. */
		rule: Rule | null;
		members: Member[];
		total: number;
		page: number;
		loading: boolean;
		failed: boolean;
	}

	let search = $state('');
	let query = $state('');
	let timer: ReturnType<typeof setTimeout> | undefined;
	let manual = $state<Group>(blank(null));
	let ruled = $state<Group[]>([]);
	let picking = $state(false);
	let removing = $state<string | null>(null);
	let seq = 0;

	let empty = $derived(
		!query && (product.asset_count ?? 0) === 0 && product.rules.length === 0 && !manual.loading
	);
	// While searching, rules with nothing that matches drop out so the results read as one list
	let shownRules = $derived(query ? ruled.filter((g) => g.loading || g.total > 0) : ruled);

	function blank(rule: Rule | null): Group {
		return { rule, members: [], total: 0, page: 1, loading: true, failed: false };
	}

	$effect(() => {
		const id = product.id;
		const rules = product.rules;
		const q = query;
		// Loading reads and writes the groups, which must not become dependencies of this effect
		untrack(() => loadAll(id, rules, q));
	});

	function loadAll(id: string, rules: Rule[], q: string) {
		const mine = ++seq;
		manual = blank(null);
		ruled = rules.map((r) => blank(r));
		loadGroup(mine, id, -1, 1, q);
		rules.forEach((_, i) => loadGroup(mine, id, i, 1, q));
	}

	async function loadGroup(mine: number, id: string, index: number, page: number, q: string) {
		const group = index < 0 ? manual : ruled[index];
		if (!group) return;
		const filter: MemberQuery = {
			q,
			offset: (page - 1) * PAGE,
			limit: PAGE,
			...(group.rule ? { ruleId: group.rule.id } : { source: 'manual' })
		};
		const next = { ...group, page, loading: true, failed: false };
		setGroup(index, next);
		try {
			const res = await getProductMembers(id, filter);
			if (mine !== seq) return;
			setGroup(index, { ...next, members: res.members, total: res.total, loading: false });
		} catch {
			if (mine !== seq) return;
			setGroup(index, { ...next, members: [], loading: false, failed: true });
		}
	}

	function setGroup(index: number, group: Group) {
		if (index < 0) manual = group;
		else ruled[index] = group;
	}

	function onSearch(value: string) {
		search = value;
		clearTimeout(timer);
		timer = setTimeout(() => (query = search.trim()), 250);
	}

	async function add(asset: Asset) {
		try {
			const res = await fetchApi(`/products/assets/${product.id}`, {
				method: 'POST',
				body: JSON.stringify({ asset_ids: [asset.id] })
			});
			if (!res.ok) {
				const data = await res.json().catch(() => ({}));
				throw new Error(data.error || m.products_error_add_asset());
			}
			toast({ tone: 'success', message: m.products_asset_added({ name: asset.name }) });
			await onChanged?.();
		} catch (e) {
			toast({
				tone: 'danger',
				message: e instanceof Error ? e.message : m.products_error_add_asset()
			});
		}
	}

	async function remove(member: Member) {
		removing = member.asset_id;
		try {
			const res = await fetchApi(`/products/assets/${product.id}/${member.asset_id}`, {
				method: 'DELETE'
			});
			if (!res.ok) throw new Error(m.products_error_remove_asset());
			toast({
				tone: 'success',
				message: m.products_asset_removed({ name: member.name }),
				action: m.products_undo(),
				onAction: () => add({ id: member.asset_id, name: member.name } as Asset)
			});
			await onChanged?.();
		} catch (e) {
			toast({
				tone: 'danger',
				message: e instanceof Error ? e.message : m.products_error_remove_asset()
			});
		} finally {
			removing = null;
		}
	}

	function caption(member: Member): string {
		const type = member.type.replace(/_/g, ' ');
		const kind = type.charAt(0).toUpperCase() + type.slice(1);
		return [kind, member.owner_names[0] ?? member.providers[0]].filter(Boolean).join(' · ');
	}
</script>

{#snippet assetRow(member: Member, removable: boolean)}
	{@const params = assetRouteParams(member)}
	{@const level = popularityLevel(member.popularity)}
	<mm-list-item
		href={params ? resolve('/discover/[type]/[service]/[name]', params) : undefined}
		secondary={caption(member)}
	>
		<mm-provider-logo
			slot="start"
			provider={member.providers.length === 1 ? member.providers[0].toLowerCase() : member.type}
			size={20}
			decorative
		></mm-provider-logo>
		{member.name}
		{#if level || removable}
			<span slot="end" class="mm-cluster" style="--mm-gap: var(--space-4)">
				{#if level}<mm-popularity {level}></mm-popularity>{/if}
				{#if removable}
					<mm-button
						size="sm"
						variant="ghost"
						loading={removing === member.asset_id}
						label={m.products_remove_named({ name: member.name, product: product.name })}
						onclick={() => remove(member)}>{m.common_remove()}</mm-button
					>
				{/if}
			</span>
		{/if}
	</mm-list-item>
{/snippet}

{#snippet pager(group: Group, index: number, label: string)}
	{#if group.total > PAGE}
		<mm-pagination
			total={group.total}
			page-size={PAGE}
			page={group.page}
			{label}
			onmm-page-change={(e: CustomEvent<{ page: number }>) =>
				loadGroup(seq, product.id, index, e.detail.page, query)}
		></mm-pagination>
	{/if}
{/snippet}

{#if empty && !picking}
	<mm-empty-state
		heading={m.products_no_assets_title()}
		description={canManage
			? m.products_no_assets_hint_manage()
			: m.products_no_assets_hint_readonly()}
		seed={4}
	>
		{#if canManage}
			<mm-button slot="action" icon="add" onclick={() => (picking = true)}
				>{m.products_add_asset()}</mm-button
			>
		{/if}
	</mm-empty-state>
{:else}
	<div class="mm-stack" style="--mm-gap: var(--space-2)">
		<mm-search-field
			class="find"
			label={m.products_find_in_product()}
			hide-label
			placeholder={m.products_find_in_product()}
			value={search}
			onmm-input={(e: CustomEvent<{ value: string }>) => onSearch(e.detail.value)}
		></mm-search-field>

		<mm-section heading={m.products_added_manually()}>
			{#if canManage && !picking}
				<mm-button slot="actions" size="sm" icon="add" onclick={() => (picking = true)}
					>{m.products_add_asset()}</mm-button
				>
			{/if}
			<div class="mm-stack" style="--mm-gap: var(--space-4)">
				{#if picking}
					<AssetPicker
						exclude={manual.members.map((mb) => mb.asset_id)}
						onAdd={add}
						onClose={() => (picking = false)}
					/>
				{/if}
				{#if manual.loading && manual.members.length === 0}
					<mm-skeleton loading count={3}></mm-skeleton>
				{:else if manual.failed}
					<mm-callout tone="danger" compact heading={m.products_assets_load_failed()}></mm-callout>
				{:else if manual.total === 0}
					<mm-text variant="dense" tone="secondary"
						>{query ? m.products_no_matching_assets() : m.products_no_manual_assets()}</mm-text
					>
				{:else}
					<mm-list aria-label={m.products_added_manually()} style="--mm-list-inset: 0">
						{#each manual.members as member (member.asset_id)}
							{@render assetRow(member, canManage)}
						{/each}
					</mm-list>
					{@render pager(manual, -1, m.products_assets_pages_label())}
				{/if}
			</div>
		</mm-section>

		{#if shownRules.length > 0}
			<mm-section heading={m.products_from_rules()} description={m.products_from_rules_hint()}>
				<div class="mm-stack" style="--mm-gap: var(--space-6)">
					{#each shownRules as group (group.rule?.id)}
						{@const rule = group.rule as Rule}
						{@const index = ruled.indexOf(group)}
						<div class="mm-stack" style="--mm-gap: var(--space-1)">
							<div class="mm-cluster mm-cluster--between">
								<mm-text variant="label" tone="secondary">{rule.name}</mm-text>
								{#if canManage && onEditRule}
									<mm-button size="sm" variant="ghost" onclick={() => onEditRule(rule)}
										>{m.products_edit_rule()}</mm-button
									>
								{/if}
							</div>
							{#if group.loading && group.members.length === 0}
								<mm-skeleton loading count={2}></mm-skeleton>
							{:else if group.failed}
								<mm-callout tone="warning" compact heading={m.products_assets_load_failed()}
								></mm-callout>
							{:else if group.total === 0}
								<mm-text variant="dense" tone="secondary"
									>{rule.is_enabled ? m.products_rule_no_matches() : m.products_rule_off()}</mm-text
								>
							{:else}
								<mm-list
									aria-label={m.products_from_rule_named({ name: rule.name })}
									style="--mm-list-inset: 0"
								>
									{#each group.members as member (member.asset_id)}
										{@render assetRow(member, false)}
									{/each}
								</mm-list>
								{@render pager(group, index, m.products_from_rule_named({ name: rule.name }))}
							{/if}
						</div>
					{/each}
				</div>
			</mm-section>
		{/if}
	</div>
{/if}

<style>
	.find {
		width: 22rem;
		max-width: 100%;
	}
</style>
