<script lang="ts">
	import '@marmotdata/elements/search-bar.js';
	import '@marmotdata/elements/key-value-editor.js';
	import '@marmotdata/elements/multiselect.js';
	import type { MmSearchBar, SearchBarField } from '@marmotdata/elements/search-bar.js';
	import type { KeyValuePair } from '@marmotdata/elements/key-value-editor.js';
	import { onMount, untrack } from 'svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { fetchApi } from '$lib/api';
	import { createAssetRule, previewAssetRule, updateAssetRule } from '$lib/assetrules/api';
	import { fieldLabel, providerLabel } from '$lib/assetrules/plain';
	import { loadTerms } from '$lib/assetrules/terms';
	import type { AssetRule, ExternalLink } from '$lib/assetrules/types';
	import type { MetadataFieldSuggestion, MetadataValueSuggestion } from '$lib/assets/types';
	import { searchTerms } from '$lib/glossary/api';
	import type { GlossaryTerm } from '$lib/glossary/types';
	import { m } from '$lib/paraglide/messages';
	import { useUnsavedChangesGuard } from '$lib/utils/unsavedChanges.svelte';
	import RulePreviewPanel from './RulePreviewPanel.svelte';

	let { rule = null, cancelHref }: { rule?: AssetRule | null; cancelHref: string } = $props();

	// The form starts from the rule as loaded; the edit page remounts the editor for a different rule
	const initial = untrack(() => ({
		name: rule?.name ?? '',
		description: rule?.description ?? '',
		enabled: rule?.is_enabled ?? true,
		query: rule?.query_expression ?? '',
		links: (rule?.links ?? []).map((l) => ({ key: l.name, value: l.url })),
		termIds: rule?.term_ids ?? []
	}));

	let name = $state(initial.name);
	let description = $state(initial.description);
	let isEnabled = $state(initial.enabled);
	let queryExpression = $state(initial.query);
	let linkPairs = $state<KeyValuePair[]>(initial.links);
	let termIds = $state<string[]>([...initial.termIds]);

	// Chosen terms stay as options so their tags keep their names while search results change
	let chosenTerms = $state<GlossaryTerm[]>([]);
	let termResults = $state<GlossaryTerm[]>([]);
	let termOptions = $derived([
		...chosenTerms.filter((t) => termIds.includes(t.id)),
		...termResults.filter((t) => !chosenTerms.some((c) => c.id === t.id))
	]);
	let termTimeout: ReturnType<typeof setTimeout>;

	let saving = $state(false);
	let error = $state<string | null>(null);
	// Field errors wait for the first submit so the form does not shout at people before they have typed anything
	let submitted = $state(false);
	let done = false;

	let previewError = $state<string | null>(null);
	let draftCount = $state<number | undefined>(undefined);
	let searchBar = $state<MmSearchBar | null>(null);
	let fields = $state<SearchBarField[]>([
		{ value: 'type', label: fieldLabel('type') },
		{ value: 'provider', label: fieldLabel('provider') },
		{ value: 'kind', label: fieldLabel('kind') },
		{ value: 'name', label: fieldLabel('name') },
		{ value: 'term', label: fieldLabel('term'), kind: 'term' }
	]);

	let validLinks = $derived(
		linkPairs
			.filter((p) => p.key.trim() && p.value.trim())
			.map((p): ExternalLink => {
				const before = rule?.links?.find((l) => l.url === p.value.trim());
				return { name: p.key.trim(), url: p.value.trim(), icon: before?.icon };
			})
	);
	let nameError = $derived(name.trim() ? '' : m.assetrules_validation_name_required());
	let queryError = $derived(
		queryExpression.trim() ? (previewError ?? '') : m.assetrules_validation_query_required()
	);
	let enrichmentError = $derived(
		validLinks.length > 0 || termIds.length > 0 ? '' : m.assetrules_validation_enrichment_required()
	);

	let isDirty = $derived(
		name !== initial.name ||
			description !== initial.description ||
			isEnabled !== initial.enabled ||
			queryExpression !== initial.query ||
			JSON.stringify(linkPairs.filter((p) => p.key || p.value)) !== JSON.stringify(initial.links) ||
			termIds.join(',') !== initial.termIds.join(',')
	);

	const unsavedGuard = useUnsavedChangesGuard(() => !done && !saving && isDirty);

	onMount(async () => {
		chosenTerms = await loadTerms(initial.termIds);
		try {
			const resp = await fetchApi('/assets/summary');
			if (!resp.ok) return;
			const summary = (await resp.json()) as {
				types?: Record<string, number>;
				providers?: Record<string, number>;
			};
			fields = fields.map((f) => {
				if (f.value === 'type') return { ...f, values: Object.keys(summary.types ?? {}).sort() };
				if (f.value === 'provider')
					return {
						...f,
						values: Object.keys(summary.providers ?? {})
							.sort()
							.map((p) => ({ value: p, label: providerLabel(p) }))
					};
				return f;
			});
		} catch {
			// Known values are a convenience; the bar still takes typed values without them
		}
	});

	async function loadFields(prefix: string): Promise<string[]> {
		try {
			const resp = await fetchApi('/assets/suggestions/metadata/fields');
			if (!resp.ok) return [];
			const data = ((await resp.json()) ?? []) as MetadataFieldSuggestion[];
			const p = prefix.replace(/^metadata\./, '').toLowerCase();
			return data
				.map((d) => `metadata.${d.field}`)
				.filter((f) => f.toLowerCase().includes(p))
				.slice(0, 20);
		} catch {
			return [];
		}
	}

	async function loadValues(field: string, prefix: string): Promise<string[]> {
		if (!field.startsWith('metadata.')) return [];
		try {
			const params = new URLSearchParams({
				field: field.replace(/^metadata\./, ''),
				prefix,
				limit: '10'
			});
			const resp = await fetchApi(`/assets/suggestions/metadata/values?${params}`);
			if (!resp.ok) return [];
			const data = ((await resp.json()) ?? []) as MetadataValueSuggestion[];
			return data.map((d) => d.value);
		} catch {
			return [];
		}
	}

	// Properties the elements take only as JS values, not attributes
	$effect(() => {
		if (!searchBar) return;
		searchBar.fields = fields;
		searchBar.loadFields = loadFields;
		searchBar.loadValues = loadValues;
	});

	let draftSeq = 0;
	async function onDraft(value: string) {
		const seq = ++draftSeq;
		if (!value.trim()) {
			draftCount = undefined;
			return;
		}
		try {
			const r = await previewAssetRule({ rule_type: 'query', query_expression: value, limit: 1 });
			if (seq === draftSeq) draftCount = r.errors?.length ? undefined : r.asset_count;
		} catch {
			if (seq === draftSeq) draftCount = undefined;
		}
	}

	function onTermQuery(q: string) {
		clearTimeout(termTimeout);
		termTimeout = setTimeout(async () => {
			try {
				const result = await searchTerms(q, null, 0, 20);
				termResults = result.terms || [];
			} catch {
				termResults = [];
			}
		}, 250);
	}

	function onTermsChange(ids: string[]) {
		const known = [...chosenTerms, ...termResults];
		chosenTerms = ids
			.map((id) => known.find((t) => t.id === id))
			.filter((t): t is GlossaryTerm => !!t);
		termIds = ids;
	}

	async function handleSubmit(e: Event) {
		e.preventDefault();
		error = null;
		submitted = true;
		if (nameError || queryError || enrichmentError) return;

		saving = true;
		try {
			const input = {
				name: name.trim(),
				description: rule ? description.trim() : description.trim() || undefined,
				links: validLinks,
				term_ids: termIds,
				rule_type: 'query' as const,
				query_expression: queryExpression.trim(),
				priority: rule?.priority ?? 0,
				is_enabled: isEnabled
			};
			const saved = rule ? await updateAssetRule(rule.id, input) : await createAssetRule(input);
			done = true;
			goto(resolve('/asset-rules/[id]', { id: saved.id }));
		} catch (err) {
			error =
				err instanceof Error
					? err.message
					: rule
						? m.assetrules_update_error()
						: m.assetrules_create_error();
		} finally {
			saving = false;
		}
	}
</script>

<div class="mm-split editor">
	<form class="mm-stack" style="--mm-gap: 0" novalidate onsubmit={handleSubmit}>
		<mm-section heading={m.assetrules_section_name_heading()}>
			<div class="mm-stack" style="--mm-gap: var(--space-5)">
				<!-- svelte-ignore a11y_autofocus -->
				<mm-text-field
					label={m.common_name()}
					name="name"
					required
					autofocus
					value={name}
					placeholder={m.assetrules_name_placeholder()}
					error={submitted ? nameError : ''}
					oninput={(e: Event) => (name = (e.target as HTMLInputElement).value)}
				></mm-text-field>
				<mm-textarea
					label={m.common_description()}
					name="description"
					rows={2}
					optional
					value={description}
					oninput={(e: Event) => (description = (e.target as HTMLTextAreaElement).value)}
				></mm-textarea>
				<mm-switch
					label={m.assetrules_apply_rule_label()}
					description={m.assetrules_enabled_hint()}
					state-label
					checked={isEnabled}
					onmm-change={(e: CustomEvent<{ checked: boolean }>) => (isEnabled = e.detail.checked)}
				></mm-switch>
			</div>
		</mm-section>

		<mm-section
			heading={m.assetrules_section_match_heading()}
			description={m.assetrules_section_match_hint()}
		>
			<div class="mm-stack" style="--mm-gap: var(--space-2)">
				<mm-search-bar
					bind:this={searchBar}
					label={m.assetrules_query_label()}
					placeholder={m.assetrules_query_placeholder()}
					value={queryExpression}
					draft-count={draftCount}
					onmm-input={(e: CustomEvent<{ value: string }>) => (queryExpression = e.detail.value)}
					onmm-draft={(e: CustomEvent<{ value: string }>) => onDraft(e.detail.value)}
				></mm-search-bar>
				{#if previewError || (submitted && queryError)}
					<mm-text variant="caption" tone="danger" block role="alert">
						{previewError ?? queryError}
					</mm-text>
				{/if}
			</div>
		</mm-section>

		<mm-section
			heading={m.assetrules_section_apply_heading()}
			description={m.assetrules_section_apply_hint()}
		>
			<div class="mm-stack" style="--mm-gap: var(--space-6)">
				<mm-key-value-editor
					label={m.assetrules_links_label()}
					key-label={m.assetrules_link_title_label()}
					value-label={m.assetrules_link_url_label()}
					value-placeholder="https://"
					add-label={m.assetrules_add_link()}
					value={linkPairs}
					onmm-change={(e: CustomEvent<{ value: KeyValuePair[] }>) => (linkPairs = e.detail.value)}
				></mm-key-value-editor>
				<mm-multiselect
					label={m.assetrules_glossary_terms_label()}
					name="terms"
					placeholder={m.assetrules_term_search_placeholder()}
					value={termIds}
					onmm-input={(e: CustomEvent<{ query: string }>) => onTermQuery(e.detail.query)}
					onmm-change={(e: CustomEvent<{ value: string[] }>) => onTermsChange(e.detail.value)}
				>
					{#each termOptions as term (term.id)}
						<mm-option value={term.id} label={term.name}></mm-option>
					{/each}
				</mm-multiselect>
				{#if submitted && enrichmentError}
					<mm-text variant="caption" tone="danger" block role="alert">{enrichmentError}</mm-text>
				{/if}
			</div>
		</mm-section>

		<div class="mm-stack" style="--mm-gap: var(--space-4)">
			{#if error}
				<mm-callout
					tone="danger"
					heading={rule ? m.assetrules_update_error() : m.assetrules_create_error()}
					>{error}</mm-callout
				>
			{/if}
			<div class="mm-cluster">
				<mm-button variant="primary" type="submit" loading={saving} disabled={saving}>
					{rule ? m.assetrules_save_rule_button() : m.assetrules_create_rule_button()}
				</mm-button>
				<mm-button href={cancelHref}>{m.common_cancel()}</mm-button>
			</div>
		</div>
	</form>

	<aside class="preview">
		<RulePreviewPanel query={queryExpression} onresult={(r) => (previewError = r.error)} />
	</aside>
</div>

<mm-confirm-dialog
	open={unsavedGuard.showConfirm}
	tone="default"
	heading={m.common_discard_prompt()}
	body={m.assetrules_unsaved_changes_confirm()}
	confirm-label={m.common_discard()}
	cancel-label={m.common_keep_editing()}
	onmm-confirm={unsavedGuard.confirmDiscard}
	onmm-close={(e: CustomEvent<{ returnValue?: string }>) => {
		if (e.detail?.returnValue !== 'confirm') unsavedGuard.cancelDiscard();
	}}
></mm-confirm-dialog>

<style>
	.editor {
		align-items: start;
	}
	.preview {
		position: sticky;
		top: 0;
	}
</style>
