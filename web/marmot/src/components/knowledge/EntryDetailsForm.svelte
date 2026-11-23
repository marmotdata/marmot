<script lang="ts">
	import '@marmotdata/elements/fieldset.js';
	import '@marmotdata/elements/filter-pill.js';
	import { SvelteSet } from 'svelte/reactivity';
	import { m } from '#lib/paraglide/messages.js';
	import { formatDate } from '#lib/utils/format.ts';
	import { monthsFromToday } from '#lib/knowledge/api.ts';
	import { isRequired, statusOf, type KnowledgeKind } from '#lib/knowledge/kinds.ts';
	import { visibleFields, type EntryDraft } from '#lib/knowledge/draft.ts';
	import FieldEditor from './FieldEditor.svelte';
	import AssetLinks from './AssetLinks.svelte';
	import LinksEditor from './LinksEditor.svelte';
	import RulesEditor from './RulesEditor.svelte';

	// The typed half of an entry, in the order the entry page shows it: the kind's own fields, then status and review, then what it is linked to. When writing a new entry only the required fields show at first, and the rest are a click away, so a first draft stays short.
	let {
		kind,
		draft = $bindable(),
		errors = {},
		assetId = '',
		compact = false
	}: {
		kind: KnowledgeKind;
		draft: EntryDraft;
		errors?: Record<string, string>;
		/** The entry's own asset ID, never offered as a link. */
		assetId?: string;
		/** Hides optional fields and groups until asked for, as the create page does. */
		compact?: boolean;
	} = $props();

	type Group = 'review' | 'links' | 'rules' | 'replaces';

	const opened = new SvelteSet<string>();

	let fields = $derived(compact ? visibleFields(kind, draft, opened) : kind.fields);
	let hiddenFields = $derived(kind.fields.filter((f) => !fields.includes(f)));
	let status = $derived(statusOf(kind, draft.status));

	let show = $derived({
		review: !compact || opened.has('review') || !!draft.review_by || !!draft.effective_from,
		links: !compact || opened.has('links') || (draft.links?.length ?? 0) > 0,
		rules: !compact || opened.has('rules') || draft.rules.length > 0,
		replaces: !compact || opened.has('replaces') || draft.supersedes.length > 0
	});

	const groupLabel: Record<Group, () => string> = {
		review: m.knowledge_group_review,
		links: m.knowledge_group_links,
		rules: m.knowledge_group_rules,
		replaces: m.knowledge_group_replaces
	};
	let hiddenGroups = $derived((Object.keys(show) as Group[]).filter((g) => !show[g]));

	const presets = [
		{ value: '3', label: m.knowledge_review_in_3_months },
		{ value: '6', label: m.knowledge_review_in_6_months },
		{ value: '12', label: m.knowledge_review_in_a_year }
	];

	function stateHelp(): string {
		switch (status?.state) {
			case 'pending':
				return m.knowledge_state_pending_help();
			case 'historical':
				return m.knowledge_state_historical_help();
			default:
				return m.knowledge_state_current_help();
		}
	}
</script>

<div class="mm-stack" style="--mm-gap: var(--space-6)">
	<mm-section
		heading={m.knowledge_section_details({ kind: kind.label })}
		description={kind.description}
	>
		<div class="mm-stack" style="--mm-gap: var(--space-5)">
			{#each fields as f (f.key)}
				<FieldEditor
					field={f}
					value={draft.fields[f.key]}
					required={isRequired(f, draft.status)}
					error={errors[f.key] ?? ''}
					onchange={(v) => {
						const next = { ...draft.fields };
						if (v === undefined) delete next[f.key];
						else next[f.key] = v;
						draft = { ...draft, fields: next };
					}}
				/>
			{/each}
			{#if fields.length === 0}
				<mm-text tone="secondary" variant="dense">{m.knowledge_details_none_required()}</mm-text>
			{/if}
		</div>
	</mm-section>

	<mm-section heading={m.knowledge_section_status()}>
		<div class="mm-stack" style="--mm-gap: var(--space-5)">
			<mm-select
				label={m.knowledge_status_label()}
				help={stateHelp()}
				value={draft.status}
				onmm-change={(e: CustomEvent<{ value: string }>) =>
					(draft = { ...draft, status: e.detail.value })}
			>
				{#each kind.statuses as s (s.value)}
					<mm-option value={s.value} label={s.label}></mm-option>
				{/each}
			</mm-select>

			{#if show.review}
				<div class="pair">
					<mm-text-field
						type="date"
						label={m.knowledge_effective_from_label()}
						help={m.knowledge_effective_from_help()}
						optional
						value={draft.effective_from}
						onmm-input={(e: CustomEvent<{ value: string }>) =>
							(draft = { ...draft, effective_from: e.detail.value })}
					></mm-text-field>
					<div class="mm-stack" style="--mm-gap: var(--space-2)">
						<mm-text-field
							type="date"
							label={m.knowledge_review_by_label()}
							help={draft.review_by
								? m.knowledge_review_by_set({ date: formatDate(draft.review_by) })
								: m.knowledge_review_by_help()}
							optional
							value={draft.review_by}
							onmm-input={(e: CustomEvent<{ value: string }>) =>
								(draft = { ...draft, review_by: e.detail.value })}
						></mm-text-field>
						<div class="mm-cluster" style="--mm-gap: var(--space-1)">
							{#each presets as p (p.value)}
								<mm-filter-pill
									pressed={draft.review_by === monthsFromToday(Number(p.value))}
									onmm-change={() =>
										(draft = { ...draft, review_by: monthsFromToday(Number(p.value)) })}
									>{p.label()}</mm-filter-pill
								>
							{/each}
						</div>
					</div>
				</div>
			{/if}
		</div>
	</mm-section>

	{#if show.links || show.rules || show.replaces}
		<mm-section
			heading={m.knowledge_section_links()}
			description={m.knowledge_section_links_description()}
		>
			<div class="mm-stack" style="--mm-gap: var(--space-6)">
				{#if show.links}
					{#if draft.links}
						<LinksEditor
							value={draft.links}
							exclude={assetId}
							onchange={(v) => (draft = { ...draft, links: v })}
						/>
					{:else}
						<mm-callout tone="info" heading={m.knowledge_links_too_many()}>
							{m.knowledge_links_too_many_body()}
						</mm-callout>
					{/if}
				{/if}
				{#if show.rules}
					<RulesEditor value={draft.rules} onchange={(v) => (draft = { ...draft, rules: v })} />
				{/if}
				{#if show.replaces}
					<AssetLinks
						label={m.knowledge_group_replaces()}
						help={m.knowledge_replaces_help({ kind: kind.label.toLowerCase() })}
						value={draft.supersedes}
						max={1}
						assetType={kind.type}
						exclude={assetId}
						onchange={(v) => (draft = { ...draft, supersedes: v })}
					/>
				{/if}
			</div>
		</mm-section>
	{/if}

	{#if compact && (hiddenFields.length || hiddenGroups.length)}
		<div class="mm-cluster more" role="group" aria-label={m.knowledge_add_more_label()}>
			{#each hiddenFields as f (f.key)}
				<mm-button size="sm" variant="ghost" icon="add" onclick={() => opened.add(f.key)}
					>{f.label}</mm-button
				>
			{/each}
			{#each hiddenGroups as g (g)}
				<mm-button size="sm" variant="ghost" icon="add" onclick={() => opened.add(g)}
					>{groupLabel[g]()}</mm-button
				>
			{/each}
		</div>
	{/if}
</div>

<style>
	.pair {
		display: grid;
		grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
		gap: var(--space-4);
		align-items: start;
	}
	@media (max-width: 640px) {
		.pair {
			grid-template-columns: minmax(0, 1fr);
		}
	}
	.more {
		--mm-gap: var(--space-1);
		border-block: 1px solid var(--mm-rule);
		padding-block: var(--space-2);
	}
</style>
