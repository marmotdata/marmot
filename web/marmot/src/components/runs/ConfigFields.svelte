<script lang="ts">
	import '@marmotdata/elements/fieldset.js';
	import '@marmotdata/elements/number-field.js';
	import { m } from '$lib/paraglide/messages';
	import { formatList } from '$lib/utils/format';
	import ListInput from './ListInput.svelte';
	import {
		AWS_PLUGINS,
		FOLDED_GROUPS,
		groupFields,
		shouldHideField,
		type ConfigField,
		type ConfigObject,
		type ConfigValue
	} from './pipeline';

	interface AwsStatus {
		available: boolean;
		sources: string[];
		error?: string;
	}

	interface Props {
		pluginId: string;
		spec: ConfigField[];
		config: ConfigObject;
		fieldErrors: Record<string, string>;
		awsStatus?: AwsStatus | null;
		// Called after any edit, with the path of the field that changed
		onedit: (path: string) => void;
	}

	let { pluginId, spec, config = $bindable(), fieldErrors, awsStatus, onedit }: Props = $props();

	// A plugin's settings in the sections its spec names: the connection essentials first, then Authentication and Discovery, then the rarely touched groups folded
	let layout = $derived(groupFields(spec));

	function hasError(name: string): boolean {
		return Object.keys(fieldErrors).some(
			(path) => path === name || path.startsWith(`${name}.`) || path.startsWith(`${name}[`)
		);
	}

	function set(obj: ConfigObject, name: string, value: ConfigValue, path: string) {
		obj[name] = value;
		onedit(path);
	}

	function inputType(field: ConfigField): 'password' | 'url' | 'text' {
		if (field.type === 'password' || field.sensitive) return 'password';
		if (field.name.toLowerCase() === 'url') return 'url';
		return 'text';
	}

	const isNumber = (field: ConfigField) =>
		field.type === 'int' || field.type === 'number' || field.type === 'integer';
	const isBool = (field: ConfigField) => field.type === 'bool' || field.type === 'boolean';
	// Switches, lists and nested groups take a whole row; short inputs sit side by side
	const isWide = (field: ConfigField) =>
		isBool(field) || field.type === 'multiselect' || field.type === 'object';
</script>

{#snippet fieldGrid(fields: ConfigField[], obj: ConfigObject, prefix: string)}
	<div class="config-grid">
		{#each fields as field (field.name)}
			{#if !shouldHideField(field, obj, config)}
				<div class={isWide(field) ? 'wide' : undefined}>
					{@render renderField(field, prefix ? `${prefix}.${field.name}` : field.name, obj)}
				</div>
			{/if}
		{/each}
	</div>
{/snippet}

{#snippet renderField(field: ConfigField, path: string, obj: ConfigObject)}
	{#if field.type === 'object' && field.is_array && field.fields}
		{@const items = (obj[field.name] as ConfigObject[] | undefined) ?? []}
		<mm-fieldset
			label={field.label}
			description={field.description}
			error={fieldErrors[path]}
			data-field-path={path}
		>
			<div class="mm-stack" style="--mm-gap: var(--space-4)">
				{#each items as item, index (index)}
					<div class="mm-stack" style="--mm-gap: var(--space-3)">
						{@render fieldGrid(field.fields, item, `${path}[${index}]`)}
						<div>
							<mm-button
								size="sm"
								variant="ghost"
								icon="delete"
								onclick={() =>
									set(
										obj,
										field.name,
										items.filter((_, i) => i !== index),
										path
									)}>{m.pipelines_remove_item({ label: field.label })}</mm-button
							>
						</div>
					</div>
				{/each}
				<div>
					<mm-button
						size="sm"
						variant="secondary"
						icon="add"
						onclick={() => {
							const next: ConfigObject = {};
							for (const f of field.fields ?? []) next[f.name] = (f.default as ConfigValue) ?? '';
							set(obj, field.name, [...items, next], path);
						}}>{m.pipelines_add_field_item({ label: field.label })}</mm-button
					>
				</div>
			</div>
		</mm-fieldset>
	{:else if field.type === 'object' && field.fields}
		{@const nested = (obj[field.name] as ConfigObject | undefined) ?? {}}
		<mm-fieldset label={field.label} description={field.description} data-field-path={path}>
			{@render fieldGrid(field.fields, nested, path)}
		</mm-fieldset>
	{:else if isBool(field)}
		<mm-switch
			label={field.label}
			description={field.description}
			checked={obj[field.name] === true}
			error={fieldErrors[path]}
			data-field-path={path}
			onmm-change={(e: CustomEvent<{ checked: boolean }>) =>
				set(obj, field.name, e.detail.checked, path)}
		></mm-switch>
	{:else if field.type === 'multiselect'}
		<ListInput
			label={field.label}
			help={field.description}
			placeholder={field.placeholder}
			mono
			required={field.required}
			error={fieldErrors[path]}
			fieldPath={path}
			values={(obj[field.name] as string[] | undefined) ?? []}
			onchange={(values) => set(obj, field.name, values, path)}
		/>
	{:else if field.options && field.options.length > 0}
		<mm-select
			label={field.label}
			help={field.description}
			required={field.required}
			optional={!field.required}
			error={fieldErrors[path]}
			value={String(obj[field.name] ?? '')}
			data-field-path={path}
			style="inline-size: 100%"
			onmm-change={(e: CustomEvent<{ value: string }>) =>
				set(obj, field.name, e.detail.value, path)}
		>
			{#each field.options as option (option.value)}
				<mm-option value={option.value}>{option.label}</mm-option>
			{/each}
		</mm-select>
	{:else if isNumber(field)}
		<mm-number-field
			label={field.label}
			help={field.description}
			required={field.required}
			optional={!field.required}
			min={field.validation?.min}
			max={field.validation?.max}
			placeholder={field.placeholder ??
				(field.default !== undefined && field.default !== null ? String(field.default) : '')}
			error={fieldErrors[path]}
			value={obj[field.name] === undefined || obj[field.name] === null
				? ''
				: String(obj[field.name])}
			data-field-path={path}
			onmm-input={(e: CustomEvent<{ value: string }>) =>
				set(obj, field.name, e.detail.value === '' ? undefined : Number(e.detail.value), path)}
		></mm-number-field>
	{:else}
		{@const type = inputType(field)}
		<mm-text-field
			{type}
			label={field.label}
			help={field.description}
			required={field.required}
			optional={!field.required}
			mono={type !== 'password'}
			autocomplete={type === 'password' ? 'new-password' : 'off'}
			placeholder={field.placeholder ??
				(field.default !== undefined && field.default !== null ? String(field.default) : '')}
			error={fieldErrors[path]}
			value={String(obj[field.name] ?? '')}
			data-field-path={path}
			onmm-input={(e: CustomEvent<{ value: string }>) => set(obj, field.name, e.detail.value, path)}
		></mm-text-field>
	{/if}
{/snippet}

<div class="mm-stack" style="--mm-gap: var(--space-6)">
	{#if AWS_PLUGINS.includes(pluginId) && awsStatus}
		{#if awsStatus.available}
			<mm-callout tone="info" heading={m.pipelines_aws_creds_detected_heading()}>
				{m.pipelines_aws_creds_sources({ sources: formatList(awsStatus.sources) })}
				{m.pipelines_aws_creds_detected_hint()}
			</mm-callout>
		{:else if awsStatus.error}
			<mm-callout tone="warning" heading={m.pipelines_aws_creds_not_detected_heading()}>
				{m.pipelines_aws_creds_not_detected_hint()}
			</mm-callout>
		{/if}
	{/if}

	{#if layout.essentials.length > 0}
		<mm-fieldset>
			<mm-heading slot="label" level={3} variant="heading-sm"
				>{m.pipelines_group_connection()}</mm-heading
			>
			{@render fieldGrid(layout.essentials, config, '')}
		</mm-fieldset>
	{/if}

	{#each layout.groups as group (group.name)}
		{#if FOLDED_GROUPS.has(group.name)}
			<mm-disclosure
				summary={group.name}
				open={group.fields.some((f) => hasError(f.name))}
				heading-level={3}
			>
				{@render fieldGrid(group.fields, config, '')}
			</mm-disclosure>
		{:else}
			<mm-fieldset>
				<mm-heading slot="label" level={3} variant="heading-sm">{group.name}</mm-heading>
				{@render fieldGrid(group.fields, config, '')}
			</mm-fieldset>
		{/if}
	{/each}
</div>

<style>
	.config-grid {
		display: grid;
		grid-template-columns: repeat(auto-fill, minmax(min(16rem, 100%), 1fr));
		gap: var(--space-4);
		align-items: start;
	}

	.config-grid > .wide {
		grid-column: 1 / -1;
	}
</style>
