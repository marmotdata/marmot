<script lang="ts">
	import { m } from '$lib/paraglide/messages';

	interface Props {
		label: string;
		values: string[];
		help?: string;
		placeholder?: string;
		mono?: boolean;
		required?: boolean;
		error?: string;
		fieldPath?: string;
		onchange?: (values: string[]) => void;
	}

	let {
		label,
		values,
		help,
		placeholder,
		mono = false,
		required = false,
		error,
		fieldPath,
		onchange
	}: Props = $props();

	// A short list of free values: type one and press Enter, and each shows as a removable tag below the field
	let draft = $state('');

	function add() {
		const value = draft.trim();
		if (!value || values.includes(value)) {
			draft = '';
			return;
		}
		onchange?.([...values, value]);
		draft = '';
	}
</script>

<div class="mm-stack" style="--mm-gap: var(--space-2)" data-field-path={fieldPath}>
	<mm-text-field
		{label}
		{help}
		{error}
		{mono}
		required={required && values.length === 0}
		optional={!required}
		placeholder={placeholder ?? m.pipelines_press_enter_hint()}
		value={draft}
		oninput={(e: Event) => (draft = (e.currentTarget as HTMLInputElement).value)}
		onkeydowncapture={(e: KeyboardEvent) => {
			// Enter adds the value rather than submitting the pipeline form: caught on the way down so it never reaches the field's own implicit submission
			if (e.key === 'Enter' && !e.isComposing) {
				e.preventDefault();
				e.stopPropagation();
				add();
			}
		}}
		onmm-change={add}
	></mm-text-field>
	{#if values.length > 0}
		<div class="mm-cluster" style="--mm-gap: var(--space-2)">
			{#each values as value (value)}
				<mm-tag
					removable
					onmm-remove={() => onchange?.(values.filter((v) => v !== value))}
					class={mono ? 'mm-mono' : undefined}>{value}</mm-tag
				>
			{/each}
		</div>
	{/if}
</div>
