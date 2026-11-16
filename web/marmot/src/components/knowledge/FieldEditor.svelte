<script lang="ts">
	import '@marmotdata/elements/fieldset.js';
	import { m } from '#lib/paraglide/messages.js';
	import type { KnowledgeField } from '#lib/knowledge/kinds.ts';
	import type { FieldRow, FieldValue } from '#lib/knowledge/api.ts';

	// One typed field of a knowledge kind, as an input. Lists are edited a line at a time and tables a row at a time, so neither needs a format to learn.
	let {
		field,
		value,
		required = false,
		error = '',
		onchange
	}: {
		field: KnowledgeField;
		value: FieldValue | undefined;
		required?: boolean;
		error?: string;
		onchange: (value: FieldValue | undefined) => void;
	} = $props();

	let text = $derived(typeof value === 'string' ? value : '');
	let items = $derived(
		Array.isArray(value) ? (value as unknown[]).map((v) => String(v ?? '')) : []
	);
	let rows = $derived(
		Array.isArray(value)
			? (value as unknown[]).filter((r): r is FieldRow => typeof r === 'object' && r !== null)
			: []
	);

	function setItem(i: number, v: string) {
		const next = [...items];
		next[i] = v;
		onchange(next);
	}

	function removeItem(i: number) {
		const next = items.filter((_, j) => j !== i);
		onchange(next.length ? next : undefined);
	}

	function setCell(i: number, key: string, v: string) {
		const next = rows.map((r) => ({ ...r }));
		next[i] = { ...next[i], [key]: v };
		onchange(next);
	}

	function removeRow(i: number) {
		const next = rows.filter((_, j) => j !== i);
		onchange(next.length ? next : undefined);
	}

	let canAddRow = $derived(!field.max_rows || rows.length < field.max_rows);
</script>

{#snippet scalar(
	f: KnowledgeField,
	v: string,
	set: (v: string) => void,
	hideLabel: boolean,
	req: boolean,
	err: string
)}
	{#if f.type === 'long_text'}
		<mm-textarea
			label={f.label}
			hide-label={hideLabel}
			help={hideLabel ? '' : (f.help ?? '')}
			placeholder={f.placeholder ?? ''}
			rows={3}
			autosize
			required={req}
			optional={!req && !hideLabel}
			error={err}
			value={v}
			onmm-input={(e: CustomEvent<{ value: string }>) => set(e.detail.value)}
		></mm-textarea>
	{:else if f.type === 'select'}
		<mm-select
			label={f.label}
			hide-label={hideLabel}
			help={hideLabel ? '' : (f.help ?? '')}
			placeholder={m.knowledge_field_choose()}
			required={req}
			error={err}
			value={v}
			onmm-change={(e: CustomEvent<{ value: string }>) => set(e.detail.value)}
		>
			{#each f.options ?? [] as o (o.value)}
				<mm-option value={o.value} label={o.label}></mm-option>
			{/each}
		</mm-select>
	{:else}
		<mm-text-field
			type={f.type === 'date' ? 'date' : f.type === 'url' ? 'url' : 'text'}
			label={f.label}
			hide-label={hideLabel}
			help={hideLabel ? '' : (f.help ?? '')}
			placeholder={f.type === 'url' ? 'https://' : (f.placeholder ?? '')}
			required={req}
			optional={!req && !hideLabel}
			error={err}
			value={v}
			onmm-input={(e: CustomEvent<{ value: string }>) => set(e.detail.value)}
		></mm-text-field>
	{/if}
{/snippet}

{#if field.type === 'list'}
	<mm-fieldset
		label={field.label}
		description={field.help ?? ''}
		{required}
		optional={!required}
		{error}
	>
		<div class="mm-stack" style="--mm-gap: var(--space-2)">
			{#each items as item, i (i)}
				<div class="line">
					<mm-text-field
						label={m.knowledge_field_item_label({ field: field.label, n: i + 1 })}
						hide-label
						value={item}
						onmm-input={(e: CustomEvent<{ value: string }>) => setItem(i, e.detail.value)}
					></mm-text-field>
					<mm-button
						variant="ghost"
						icon="close"
						label={m.knowledge_field_remove_item({ field: field.label, n: i + 1 })}
						onclick={() => removeItem(i)}
					></mm-button>
				</div>
			{/each}
			<div>
				<mm-button size="sm" variant="ghost" icon="add" onclick={() => onchange([...items, ''])}
					>{m.knowledge_field_add_item()}</mm-button
				>
			</div>
		</div>
	</mm-fieldset>
{:else if field.type === 'table'}
	<mm-fieldset
		label={field.label}
		description={field.help ?? ''}
		{required}
		optional={!required}
		{error}
	>
		<div class="mm-stack" style="--mm-gap: var(--space-3)">
			{#each rows as row, i (i)}
				<div class="row">
					<div class="cells">
						{#each field.columns ?? [] as c (c.key)}
							<div class:wide={c.type === 'long_text'}>
								{@render scalar(c, row[c.key] ?? '', (v) => setCell(i, c.key, v), false, false, '')}
							</div>
						{/each}
					</div>
					<mm-button
						variant="ghost"
						icon="delete"
						label={m.knowledge_field_remove_row({ field: field.label, n: i + 1 })}
						onclick={() => removeRow(i)}
					></mm-button>
				</div>
			{/each}
			{#if canAddRow}
				<div>
					<mm-button size="sm" variant="ghost" icon="add" onclick={() => onchange([...rows, {}])}
						>{m.knowledge_field_add_row()}</mm-button
					>
				</div>
			{/if}
		</div>
	</mm-fieldset>
{:else}
	{@render scalar(field, text, (v) => onchange(v || undefined), false, required, error)}
{/if}

<style>
	.line {
		display: grid;
		grid-template-columns: minmax(0, 1fr) auto;
		gap: var(--space-2);
		align-items: center;
	}
	.row {
		display: grid;
		grid-template-columns: minmax(0, 1fr) auto;
		gap: var(--space-2);
		align-items: start;
		padding: var(--space-3);
		border: 1px solid var(--mm-rule);
		border-radius: var(--radius-md);
	}
	.cells {
		display: grid;
		grid-template-columns: repeat(auto-fit, minmax(10rem, 1fr));
		gap: var(--space-3);
	}
	.wide {
		grid-column: 1 / -1;
	}
</style>
