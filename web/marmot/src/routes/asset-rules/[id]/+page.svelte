<script lang="ts">
	import { html, nothing } from 'lit';
	import type { DataTableColumn } from '@marmotdata/elements/data-table.js';
	import type { MmDataTable } from '@marmotdata/elements/classes/data-table.js';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { auth } from '$lib/stores/auth';
	import { toasts } from '$lib/stores/toast';
	import {
		createAssetRule,
		deleteAssetRule,
		getAssetRule,
		getAssetRuleAssets,
		updateAssetRule
	} from '$lib/assetrules/api';
	import { assetHref } from '$lib/assetrules/assets';
	import { plainQuery, providerLabel } from '$lib/assetrules/plain';
	import type { AssetRule } from '$lib/assetrules/types';
	import { setCrumbs } from '$lib/mm';
	import { m } from '$lib/paraglide/messages';
	import { formatDateTime } from '$lib/utils';
	import RuleAdds from '$components/assetrule/RuleAdds.svelte';

	const PAGE_SIZE = 25;

	interface MatchedRow {
		id: string;
		name: string;
		type: string;
		provider: string;
		href: string;
	}

	let ruleId = $derived(page.params.id ?? '');
	let canManage = $derived(auth.hasPermission('assets', 'manage'));

	let rule = $state<AssetRule | null>(null);
	let isLoading = $state(true);
	let loadError = $state<string | null>(null);
	let toggling = $state(false);
	let deleting = $state(false);
	let deleteOpen = $state(false);

	let rows = $state<MatchedRow[]>([]);
	let assetsTotal = $state(0);
	let assetsPage = $state(1);
	let assetsLoading = $state(false);
	let assetsError = $state('');
	let table = $state<MmDataTable | null>(null);
	let assetsFilter = $state('');
	let filterTimer: ReturnType<typeof setTimeout> | undefined;
	// Drops a slow page or filter response once a newer one has been asked for
	let assetsSeq = 0;

	let plain = $derived(plainQuery(rule?.query_expression));

	$effect(() => {
		setCrumbs([
			{ label: m.assetrules_heading(), href: resolve('/asset-rules') },
			{ label: rule?.name ?? m.assetrules_fallback_name() }
		]);
	});

	$effect(() => {
		const id = ruleId;
		loadRule(id);
	});

	async function loadRule(id: string) {
		isLoading = true;
		loadError = null;
		try {
			rule = await getAssetRule(id);
			assetsPage = 1;
			loadAssets(id);
		} catch (e) {
			loadError = e instanceof Error ? e.message : m.assetrules_load_one_error();
		} finally {
			isLoading = false;
		}
	}

	async function loadAssets(id: string) {
		const seq = ++assetsSeq;
		assetsLoading = true;
		assetsError = '';
		try {
			const result = await getAssetRuleAssets(
				id,
				(assetsPage - 1) * PAGE_SIZE,
				PAGE_SIZE,
				assetsFilter
			);
			if (seq !== assetsSeq) return;
			assetsTotal = result.total || 0;
			rows = (result.assets ?? []).map((a) => ({
				id: a.id,
				name: a.name,
				type: (a.type ?? '').replace(/_/g, ' '),
				provider: a.providers?.[0] ?? '',
				href: assetHref(a)
			}));
		} catch (e) {
			if (seq === assetsSeq)
				assetsError = e instanceof Error ? e.message : m.assetrules_assets_load_error();
		} finally {
			if (seq === assetsSeq) assetsLoading = false;
		}
	}

	function onFilterInput(value: string) {
		assetsFilter = value;
		clearTimeout(filterTimer);
		filterTimer = setTimeout(() => {
			assetsPage = 1;
			if (rule) loadAssets(rule.id);
		}, 250);
	}

	// The asset cell carries the provider logo beside the name, as on Discover
	const assetCell = (row: MatchedRow) =>
		html`<span
			style="display: inline-flex; align-items: center; gap: var(--space-2); white-space: nowrap"
			>${
				row.provider
					? html`<mm-provider-logo
							provider=${row.provider}
							size="16"
							decorative
						></mm-provider-logo>`
					: nothing
			}${row.name}</span
		>`;

	const columns: DataTableColumn<MatchedRow>[] = [
		{
			key: 'name',
			header: m.assetrules_col_asset(),
			width: 260,
			render: assetCell,
			href: (r) => r.href
		},
		{ key: 'type', header: m.common_type(), width: 120 },
		{
			key: 'provider',
			header: m.assetrules_col_source(),
			width: 160,
			value: (r) => (r.provider ? providerLabel(r.provider) : '')
		}
	];

	// Rows and columns are JS-only properties on the table
	$effect(() => {
		if (!table) return;
		table.columns = columns;
		table.rows = rows;
	});

	async function toggle(checked: boolean) {
		if (!rule) return;
		const previous = rule.is_enabled;
		rule.is_enabled = checked;
		toggling = true;
		try {
			rule = await updateAssetRule(rule.id, { is_enabled: checked });
			loadAssets(rule.id);
		} catch (e) {
			if (rule) rule.is_enabled = previous;
			toasts.error(e instanceof Error ? e.message : m.assetrules_update_error());
		} finally {
			toggling = false;
		}
	}

	async function duplicate() {
		if (!rule) return;
		try {
			const copy = await createAssetRule({
				name: m.assetrules_copy_name({ name: rule.name }),
				description: rule.description,
				links: rule.links,
				term_ids: rule.term_ids,
				rule_type: rule.rule_type,
				query_expression: rule.query_expression,
				metadata_field: rule.metadata_field,
				pattern_type: rule.pattern_type,
				pattern_value: rule.pattern_value,
				priority: rule.priority,
				// A copy starts off so it never applies the same things twice by surprise
				is_enabled: false
			});
			goto(resolve('/asset-rules/[id]/edit', { id: copy.id }));
		} catch (e) {
			toasts.error(e instanceof Error ? e.message : m.assetrules_create_error());
		}
	}

	async function handleDelete(e: Event) {
		e.preventDefault();
		if (!rule) return;
		deleting = true;
		try {
			await deleteAssetRule(rule.id);
			deleteOpen = false;
			goto(resolve('/asset-rules'));
		} catch (err) {
			deleteOpen = false;
			toasts.error(err instanceof Error ? err.message : m.assetrules_delete_error());
		} finally {
			deleting = false;
		}
	}

	function onMenu(e: CustomEvent<{ value: string }>) {
		if (e.detail.value === 'duplicate') duplicate();
		if (e.detail.value === 'delete') deleteOpen = true;
	}
</script>

<svelte:head>
	<title
		>{m.assetrules_detail_page_title({ name: rule?.name || m.assetrules_fallback_name() })}</title
	>
</svelte:head>

{#if isLoading && !rule}
	<mm-skeleton loading count={6}></mm-skeleton>
{:else if loadError || !rule}
	<mm-callout tone="danger" heading={m.assetrules_load_one_error()}>
		{loadError}
		<mm-button slot="actions" size="sm" href={resolve('/asset-rules')}
			>{m.assetrules_back_to_rules()}</mm-button
		>
	</mm-callout>
{:else}
	<div class="mm-stack" style="--mm-gap: var(--space-6)">
		<mm-page-header heading={rule.name} icon="rule" description={rule.description ?? ''}>
			<mm-switch
				slot="actions"
				label={m.assetrules_apply_rule_label()}
				state-label
				checked={rule.is_enabled}
				disabled={!canManage || toggling}
				onmm-change={(e: CustomEvent<{ checked: boolean }>) => toggle(e.detail.checked)}
			></mm-switch>
			{#if canManage}
				<mm-menu-button
					slot="actions"
					variant="ghost"
					icon="more-horiz"
					icon-only
					label={m.assetrules_more_actions({ name: rule.name })}
				>
					<mm-menu onmm-select={onMenu}>
						<mm-menu-item value="duplicate" icon="content-copy"
							>{m.assetrules_duplicate_rule()}</mm-menu-item
						>
						<mm-menu-separator></mm-menu-separator>
						<mm-menu-item value="delete" icon="delete" danger
							>{m.assetrules_delete_rule()}</mm-menu-item
						>
					</mm-menu>
				</mm-menu-button>
				<mm-button
					slot="actions"
					icon="edit"
					href={resolve('/asset-rules/[id]/edit', { id: rule.id })}
					>{m.assetrules_edit_rule_button()}</mm-button
				>
			{/if}
		</mm-page-header>

		<mm-panel heading={m.assetrules_how_it_works_heading()}>
			<div class="how">
				<div class="mm-stack step" style="--mm-gap: var(--space-2)">
					<mm-text variant="label" tone="secondary">{m.assetrules_matches_label()}</mm-text>
					<mm-text variant="dense" block>{plain ?? m.assetrules_plain_custom()}</mm-text>
					<code class="query">{rule.query_expression}</code>
				</div>
				<div class="mm-stack step" style="--mm-gap: var(--space-2)">
					<mm-text variant="label" tone="secondary">{m.assetrules_adds_label()}</mm-text>
					<RuleAdds termIds={rule.term_ids} links={rule.links} />
				</div>
				<div class="mm-stack step" style="--mm-gap: var(--space-2)">
					<mm-text variant="label" tone="secondary">{m.assetrules_applied_to_label()}</mm-text>
					<mm-text variant="dense" block numeric>
						{rule.is_enabled
							? m.assetrules_asset_count({ count: rule.membership_count || 0 })
							: m.assetrules_off()}
					</mm-text>
					<mm-text variant="caption" tone="secondary" block>{m.assetrules_applied_hint()}</mm-text>
				</div>
			</div>
		</mm-panel>

		<mm-section heading={m.assetrules_matched_assets_heading()}>
			<div class="mm-stack" style="--mm-gap: var(--space-4)">
				<mm-search-field
					class="filter"
					label={m.assetrules_filter_matched_label()}
					hide-label
					placeholder={m.assetrules_filter_matched_placeholder()}
					value={assetsFilter}
					onmm-input={(e: CustomEvent<{ value: string }>) => onFilterInput(e.detail.value)}
				></mm-search-field>
				<mm-data-table
					bind:this={table}
					label={m.assetrules_matched_assets_heading()}
					row-key="id"
					loading={assetsLoading}
					error={assetsError}
					empty-heading={assetsFilter.trim()
						? m.assetrules_no_filtered_assets_heading()
						: m.assetrules_no_matched_assets_heading()}
					empty-text={assetsFilter.trim()
						? m.assetrules_no_filtered_assets_hint()
						: m.assetrules_no_matched_assets_hint()}
				></mm-data-table>
				{#if assetsTotal > PAGE_SIZE}
					<mm-pagination
						total={assetsTotal}
						page-size={PAGE_SIZE}
						page={assetsPage}
						label={m.assetrules_matched_pages_label()}
						onmm-page-change={(e: CustomEvent<{ page: number }>) => {
							assetsPage = e.detail.page;
							if (rule) loadAssets(rule.id);
						}}
					></mm-pagination>
				{/if}
			</div>
		</mm-section>

		<mm-section heading={m.assetrules_more_details_heading()}>
			<mm-description-list>
				<mm-description term={m.assetrules_created_label()}
					>{formatDateTime(rule.created_at)}</mm-description
				>
				{#if rule.created_by_name}
					<mm-description term={m.assetrules_created_by_label()}
						>{rule.created_by_name}</mm-description
					>
				{/if}
				<mm-description term={m.assetrules_last_changed_label()}
					>{formatDateTime(rule.updated_at)}</mm-description
				>
				{#if rule.updated_by_name}
					<mm-description term={m.assetrules_last_changed_by_label()}
						>{rule.updated_by_name}</mm-description
					>
				{/if}
				<mm-description term={m.assetrules_last_checked_label()}
					>{rule.last_reconciled_at
						? formatDateTime(rule.last_reconciled_at)
						: m.common_never()}</mm-description
				>
			</mm-description-list>
		</mm-section>
	</div>

	<mm-confirm-dialog
		open={deleteOpen}
		heading={m.assetrules_delete_title({ name: rule.name })}
		body={m.assetrules_delete_confirm_message()}
		confirm-label={m.assetrules_delete_rule()}
		confirm-icon="delete"
		loading={deleting}
		onmm-confirm={handleDelete}
		onmm-close={() => (deleteOpen = false)}
	></mm-confirm-dialog>
{/if}

<style>
	.how {
		display: grid;
		grid-template-columns: repeat(auto-fit, minmax(14rem, 1fr));
	}
	.step {
		padding: var(--space-4);
	}
	.step + .step {
		border-inline-start: 1px solid var(--mm-rule);
	}
	.filter {
		width: 22rem;
		max-width: 100%;
	}
	.query {
		font: var(--text-mono-dense);
		color: var(--mm-text-2);
		overflow-wrap: anywhere;
	}
</style>
