<script lang="ts">
	import { toast } from '@marmotdata/elements/toast.js';
	import { m } from '$lib/paraglide/messages';
	import { createRule, deleteRule, reorderRules, updateRule } from '$lib/dataproducts/api';
	import { plainQuery } from '$lib/dataproducts/plain';
	import type { DataProduct, Rule, RuleInput } from '$lib/dataproducts/types';
	import RuleEditor from './RuleEditor.svelte';

	// Rules read as Matches, Adds and Applied to, in plain words first with the exact query beneath, and keep their on/off switch visible in the list.
	let {
		product,
		canManage = false,
		editing = $bindable(null),
		onChanged
	}: {
		product: DataProduct;
		canManage?: boolean;
		/** The rule open in the editor: a rule id, "new", or null. */
		editing?: string | null;
		onChanged?: () => void | Promise<void>;
	} = $props();

	let form = $state<RuleInput>(empty());
	let saving = $state(false);
	let error = $state('');
	let toggling = $state<string | null>(null);
	let toDelete = $state<Rule | null>(null);
	let deleting = $state(false);
	let lastOpened: string | null = null;

	function empty(): RuleInput {
		return {
			name: '',
			description: '',
			rule_type: 'query',
			query_expression: '',
			is_enabled: true
		};
	}

	// Opening a rule from elsewhere (the Assets tab) only sets `editing`, so the form is filled here.
	$effect(() => {
		if (editing === lastOpened) return;
		lastOpened = editing;
		error = '';
		const rule = product.rules.find((r) => r.id === editing);
		form = rule
			? {
					id: rule.id,
					name: rule.name,
					description: rule.description ?? '',
					rule_type: rule.rule_type,
					query_expression: rule.query_expression ?? '',
					is_enabled: rule.is_enabled
				}
			: empty();
	});

	function payload(input: RuleInput): RuleInput {
		return {
			name: input.name.trim(),
			description: input.description?.trim() || undefined,
			rule_type: input.rule_type,
			query_expression: input.query_expression?.trim(),
			is_enabled: input.is_enabled
		};
	}

	async function save() {
		saving = true;
		error = '';
		try {
			if (editing && editing !== 'new') await updateRule(product.id, editing, payload(form));
			else await createRule(product.id, payload(form));
			editing = null;
			await onChanged?.();
		} catch (e) {
			error = e instanceof Error ? e.message : m.products_error_save_rule();
		} finally {
			saving = false;
		}
	}

	async function toggle(rule: Rule, on: boolean) {
		toggling = rule.id;
		try {
			await updateRule(product.id, rule.id, payload({ ...rule, is_enabled: on }));
			await onChanged?.();
		} catch (e) {
			toast({
				tone: 'danger',
				message: e instanceof Error ? e.message : m.products_error_save_rule()
			});
			await onChanged?.();
		} finally {
			toggling = null;
		}
	}

	async function move(rule: Rule, by: -1 | 1) {
		const ids = product.rules.map((r) => r.id);
		const from = ids.indexOf(rule.id);
		const to = from + by;
		if (from < 0 || to < 0 || to >= ids.length) return;
		[ids[from], ids[to]] = [ids[to], ids[from]];
		try {
			await reorderRules(product.id, ids);
			await onChanged?.();
		} catch (e) {
			toast({
				tone: 'danger',
				message: e instanceof Error ? e.message : m.products_error_reorder_rules()
			});
		}
	}

	function onRuleMenu(rule: Rule, value: string) {
		if (value === 'up') move(rule, -1);
		else if (value === 'down') move(rule, 1);
		else if (value === 'delete') toDelete = rule;
	}

	async function confirmDelete(event: Event) {
		if (!toDelete) return;
		event.preventDefault();
		deleting = true;
		const rule = toDelete;
		try {
			await deleteRule(product.id, rule.id);
			toDelete = null;
			toast({ tone: 'success', message: m.products_rule_deleted({ name: rule.name }) });
			await onChanged?.();
		} catch (e) {
			toast({
				tone: 'danger',
				message: e instanceof Error ? e.message : m.products_error_delete_rule()
			});
		} finally {
			deleting = false;
		}
	}
</script>

{#snippet editor(heading: string, saveLabel: string)}
	<RuleEditor
		bind:rule={form}
		{heading}
		{saveLabel}
		{saving}
		{error}
		onSave={save}
		onCancel={() => (editing = null)}
	/>
{/snippet}

<mm-section heading={m.products_tab_rules()} description={m.products_rules_hint()}>
	{#if canManage && editing === null}
		<mm-button slot="actions" icon="add" onclick={() => (editing = 'new')}
			>{m.products_add_rule()}</mm-button
		>
	{/if}

	<div class="mm-stack" style="--mm-gap: var(--space-4)">
		{#if editing === 'new'}
			{@render editor(m.products_new_rule(), m.products_add_rule())}
		{/if}

		{#if product.rules.length === 0 && editing !== 'new'}
			<mm-empty-state
				compact
				heading={m.products_no_rules_title()}
				description={m.products_no_rules_hint()}
			>
				{#if canManage}
					<mm-button slot="action" icon="add" onclick={() => (editing = 'new')}
						>{m.products_add_rule()}</mm-button
					>
				{/if}
			</mm-empty-state>
		{:else if product.rules.length > 0}
			<ol class="mm-list rules" aria-label={m.products_rules_for({ name: product.name })}>
				{#each product.rules as rule, i (rule.id)}
					{@const plain = plainQuery(rule.query_expression)}
					<li>
						{#if editing === rule.id}
							<div class="grow">{@render editor(rule.name, m.products_save_rule())}</div>
						{:else}
							<div class="mm-stack grow" style="--mm-gap: var(--space-2)">
								<span class="name">{rule.name}</span>
								{#if rule.description}
									<mm-text variant="dense" tone="secondary" block>{rule.description}</mm-text>
								{/if}
								<div class="how">
									<mm-text variant="caption" tone="secondary">{m.products_rule_matches()}</mm-text>
									<div class="mm-stack" style="--mm-gap: var(--space-1); min-width: 0">
										{#if plain}<mm-text variant="dense" block>{plain}</mm-text>{/if}
										<code class="query">{rule.query_expression}</code>
									</div>
									<mm-text variant="caption" tone="secondary">{m.products_rule_adds()}</mm-text>
									<mm-text variant="dense" block>{m.products_rule_adds_value()}</mm-text>
								</div>
							</div>
							<div class="mm-stack side">
								<span class="mm-cluster" style="--mm-gap: var(--space-1); flex-wrap: nowrap">
									<mm-switch
										label={m.products_rule_apply({ name: rule.name })}
										hide-label
										checked={rule.is_enabled}
										disabled={!canManage || toggling === rule.id}
										onmm-change={(e: CustomEvent<{ checked: boolean }>) =>
											toggle(rule, e.detail.checked)}
									></mm-switch>
									{#if canManage}
										<mm-button
											size="sm"
											variant="ghost"
											icon="edit"
											label={m.products_edit_rule_named({ name: rule.name })}
											onclick={() => (editing = rule.id)}
										></mm-button>
										<mm-menu-button
											size="sm"
											variant="ghost"
											icon="more-horiz"
											icon-only
											label={m.products_more_for({ name: rule.name })}
										>
											<mm-menu
												onmm-select={(e: CustomEvent<{ value: string }>) =>
													onRuleMenu(rule, e.detail.value)}
											>
												<mm-menu-item value="up" icon="arrow-upward" disabled={i === 0}
													>{m.products_rule_move_up()}</mm-menu-item
												>
												<mm-menu-item
													value="down"
													icon="arrow-downward"
													disabled={i === product.rules.length - 1}
													>{m.products_rule_move_down()}</mm-menu-item
												>
												<mm-menu-separator></mm-menu-separator>
												<mm-menu-item value="delete" icon="delete" danger
													>{m.products_delete_rule()}</mm-menu-item
												>
											</mm-menu>
										</mm-menu-button>
									{/if}
								</span>
								<mm-text variant="caption" tone="secondary" numeric>
									{rule.is_enabled
										? m.products_assets_count({ count: rule.matched_asset_count ?? 0 })
										: m.products_rule_off()}
								</mm-text>
							</div>
						{/if}
					</li>
				{/each}
			</ol>
		{/if}
	</div>
</mm-section>

<mm-confirm-dialog
	open={!!toDelete}
	heading={m.products_delete_rule_title()}
	body={toDelete ? m.products_delete_rule_confirm({ name: toDelete.name }) : ''}
	confirm-label={m.products_delete_rule()}
	confirm-icon="delete"
	loading={deleting}
	onmm-confirm={confirmDelete}
	onmm-close={() => (toDelete = null)}
></mm-confirm-dialog>

<style>
	.rules {
		border-block: 1px solid var(--mm-rule);
	}
	.rules > li {
		align-items: flex-start;
		gap: var(--space-4);
		padding-block: var(--space-4);
	}
	.grow {
		flex: 1;
		min-width: 0;
	}
	.name {
		font: var(--text-heading-sm);
	}
	.how {
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
	.side {
		--mm-gap: var(--space-2);
		align-items: flex-end;
		flex: none;
	}
</style>
