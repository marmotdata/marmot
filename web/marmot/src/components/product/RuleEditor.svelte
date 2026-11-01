<script lang="ts">
	import '@marmotdata/elements/query-input.js';
	import { createQuerySuggest } from '@marmotdata/elements/query-input.js';
	import { untrack } from 'svelte';
	import { SvelteMap } from 'svelte/reactivity';
	import { m } from '$lib/paraglide/messages';
	import { previewRule } from '$lib/dataproducts/api';
	import { plainQuery } from '$lib/dataproducts/plain';
	import { assetCaption, assetProvider, loadAssets } from '$lib/dataproducts/assets';
	import type { RuleInput } from '$lib/dataproducts/types';
	import type { Asset } from '$lib/assets/types';

	const SAMPLE = 3;

	// One rule edited in place: a name, what it matches (with the query read back in plain words) and a live preview of the assets it would bring in.
	let {
		rule = $bindable(),
		heading,
		saveLabel = '',
		saving = false,
		error = '',
		onSave,
		onCancel,
		onRemove
	}: {
		rule: RuleInput;
		heading: string;
		saveLabel?: string;
		saving?: boolean;
		error?: string;
		onSave?: () => void;
		onCancel?: () => void;
		onRemove?: () => void;
	} = $props();

	let nameError = $state('');
	let queryError = $state('');
	let matchCount = $state<number | null>(null);
	let previewFailed = $state(false);
	let checking = $state(false);
	let sampleIds = $state<string[]>([]);
	const details = new SvelteMap<string, Asset>();
	let token = 0;

	const suggest = createQuerySuggest({
		fields: [
			{ value: 'type', description: m.products_plain_label_type() },
			{ value: 'provider', description: m.products_plain_label_provider() },
			{ value: 'name', description: m.products_plain_label_name() },
			{ value: 'term', description: m.products_plain_label_term() },
			{ value: 'metadata.', description: m.products_rule_field_metadata() }
		]
	});

	let plain = $derived(plainQuery(rule.query_expression));

	// Previews are debounced and tagged so a slow answer for an older query never replaces the current one.
	$effect(() => {
		const query = (rule.query_expression ?? '').trim();
		const mine = ++token;
		if (!query) {
			untrack(() => {
				matchCount = null;
				sampleIds = [];
				checking = false;
			});
			return;
		}
		untrack(() => (checking = true));
		const timer = setTimeout(async () => {
			try {
				const preview = await previewRule(
					{
						name: rule.name || 'preview',
						rule_type: 'query',
						query_expression: query,
						is_enabled: true
					},
					SAMPLE
				);
				if (mine !== token) return;
				previewFailed = (preview.errors?.length ?? 0) > 0;
				matchCount = preview.asset_count ?? 0;
				sampleIds = (preview.asset_ids ?? []).slice(0, SAMPLE);
				await loadAssets(sampleIds, details);
			} catch {
				if (mine !== token) return;
				previewFailed = true;
				matchCount = null;
				sampleIds = [];
			} finally {
				if (mine === token) checking = false;
			}
		}, 400);
		return () => clearTimeout(timer);
	});

	function validate(): boolean {
		nameError = rule.name.trim() ? '' : m.products_rule_name_required();
		queryError = rule.query_expression?.trim() ? '' : m.products_rule_query_required();
		return !nameError && !queryError;
	}

	function save() {
		if (validate()) onSave?.();
	}

	/** Lets a parent form check this rule before it submits. */
	export function check(): boolean {
		return validate();
	}
</script>

<mm-panel {heading} icon="rule">
	{#if onRemove}
		<mm-button
			slot="actions"
			variant="ghost"
			size="sm"
			icon="close"
			label={m.products_rule_remove()}
			onclick={onRemove}
		></mm-button>
	{/if}
	<div class="mm-stack" style="--mm-gap: var(--space-4)">
		<mm-text-field
			label={m.products_rule_name_label()}
			required
			value={rule.name}
			error={nameError}
			placeholder={m.products_rule_name_placeholder()}
			oninput={(e: Event) => (rule.name = (e.target as HTMLInputElement).value)}
			onmm-change={() => rule.name.trim() && (nameError = '')}
		></mm-text-field>
		<div class="mm-stack" style="--mm-gap: var(--space-2)">
			<mm-query-input
				label={m.products_rule_matches()}
				value={rule.query_expression ?? ''}
				error={queryError}
				shortcut=""
				placeholder={m.products_rule_query_placeholder()}
				{suggest}
				onmm-input={(e: CustomEvent<{ value: string }>) => {
					rule.query_expression = e.detail.value;
					if (e.detail.value.trim()) queryError = '';
				}}
				onmm-search={(e: Event) => e.preventDefault()}
			></mm-query-input>
			{#if plain}
				<mm-text variant="dense" block>{plain}</mm-text>
			{/if}
		</div>
		<mm-text-field
			label={m.common_description()}
			optional
			value={rule.description ?? ''}
			oninput={(e: Event) => (rule.description = (e.target as HTMLInputElement).value)}
		></mm-text-field>

		{#if rule.query_expression?.trim()}
			<div class="mm-stack" style="--mm-gap: var(--space-2)" aria-live="polite">
				{#if checking && matchCount === null}
					<mm-text variant="dense" tone="secondary">{m.products_rule_checking()}</mm-text>
				{:else if previewFailed}
					<mm-callout tone="warning" compact heading={m.products_rule_preview_failed()}
					></mm-callout>
				{:else if matchCount !== null}
					<mm-text variant="dense" tone="secondary" numeric
						>{m.products_rule_match_count({ count: matchCount })}</mm-text
					>
					{#if sampleIds.length > 0}
						<mm-list dense aria-label={m.products_rule_sample()} style="--mm-list-inset: 0">
							{#each sampleIds as id (id)}
								{@const asset = details.get(id)}
								{#if asset}
									<mm-list-item secondary={assetCaption(asset)}>
										<mm-provider-logo
											slot="start"
											provider={assetProvider(asset)}
											size={16}
											decorative
										></mm-provider-logo>
										{asset.name}
									</mm-list-item>
								{/if}
							{/each}
						</mm-list>
					{/if}
				{/if}
			</div>
		{/if}

		{#if error}
			<mm-callout tone="danger" compact heading={error}></mm-callout>
		{/if}

		{#if onSave || onCancel}
			<div class="mm-cluster" style="--mm-gap: var(--space-2); justify-content: flex-end">
				{#if onCancel}
					<mm-button onclick={onCancel} disabled={saving}>{m.common_cancel()}</mm-button>
				{/if}
				{#if onSave}
					<mm-button variant="primary" loading={saving} onclick={save}>{saveLabel}</mm-button>
				{/if}
			</div>
		{/if}
	</div>
</mm-panel>
