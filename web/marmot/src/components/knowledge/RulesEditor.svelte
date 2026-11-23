<script lang="ts">
	import '@marmotdata/elements/fieldset.js';
	import { resolve } from '$app/paths';
	import { m } from '#lib/paraglide/messages.js';
	import { knowledgeRelations } from '#lib/knowledge/kinds.ts';
	import type { KnowledgeRule, SelectorType } from '#lib/knowledge/api.ts';
	import { ruleCoverage, ruleDiscoverHref } from '#lib/knowledge/rules.ts';

	// Rules link an entry to every asset with a tag, of a type or from a provider, including assets added later, so a policy can cover thousands of tables without listing them.
	let { value, onchange }: { value: KnowledgeRule[]; onchange: (value: KnowledgeRule[]) => void } =
		$props();

	const selectorLabel: Record<SelectorType, () => string> = {
		tag: m.knowledge_rule_selector_tag,
		type: m.knowledge_rule_selector_type,
		provider: m.knowledge_rule_selector_provider
	};

	function set(i: number, patch: Partial<KnowledgeRule>) {
		const next = value.map((r) => ({ ...r }));
		// A changed selector no longer covers what was counted, so the count goes until the next save.
		next[i] = { ...next[i], ...patch, matches: undefined };
		onchange(next);
	}
</script>

<mm-fieldset label={m.knowledge_group_rules()} description={m.knowledge_rules_help()} optional>
	<div class="mm-stack" style="--mm-gap: var(--space-3)">
		{#each value as r, i (i)}
			<div class="rule">
				<mm-select
					label={m.knowledge_link_relation_new()}
					hide-label
					value={r.relation}
					onmm-change={(e: CustomEvent<{ value: string }>) => set(i, { relation: e.detail.value })}
				>
					{#each $knowledgeRelations as rel (rel.id)}
						<mm-option value={rel.id} label={rel.label}></mm-option>
					{/each}
				</mm-select>
				<mm-select
					label={m.knowledge_rule_selector_label()}
					hide-label
					value={r.selector_type}
					onmm-change={(e: CustomEvent<{ value: string }>) =>
						set(i, { selector_type: e.detail.value as SelectorType })}
				>
					{#each Object.keys(selectorLabel) as s (s)}
						<mm-option value={s} label={selectorLabel[s as SelectorType]()}></mm-option>
					{/each}
				</mm-select>
				<mm-text-field
					label={m.knowledge_rule_value_label()}
					hide-label
					placeholder={r.selector_type === 'tag'
						? 'pii'
						: r.selector_type === 'type'
							? 'table'
							: 'snowflake'}
					value={r.selector_value}
					onmm-input={(e: CustomEvent<{ value: string }>) =>
						set(i, { selector_value: e.detail.value })}
				></mm-text-field>
				<mm-button
					variant="ghost"
					icon="delete"
					label={m.knowledge_rule_remove({ n: i + 1 })}
					onclick={() => onchange(value.filter((_, j) => j !== i))}
				></mm-button>
				{#if r.matches !== undefined}
					<mm-text variant="caption" tone="secondary" class="coverage">
						<mm-link href={resolve(ruleDiscoverHref(r) as '/')}>{ruleCoverage(r)}</mm-link>
					</mm-text>
				{/if}
			</div>
		{/each}
		<div>
			<mm-button
				size="sm"
				variant="ghost"
				icon="add"
				onclick={() =>
					onchange([
						...value,
						{ relation: 'applies_to', selector_type: 'tag', selector_value: '' }
					])}>{m.knowledge_rule_add()}</mm-button
			>
		</div>
	</div>
</mm-fieldset>

<style>
	.rule {
		display: grid;
		grid-template-columns: 9rem 8rem minmax(0, 1fr) auto;
		gap: var(--space-2);
		align-items: center;
	}
	.coverage {
		grid-column: 2 / -1;
	}
	@media (max-width: 640px) {
		.rule {
			grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
		}
	}
</style>
