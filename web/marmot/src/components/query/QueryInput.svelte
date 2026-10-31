<script lang="ts">
	import '@marmotdata/elements/query-input.js';
	import { createQuerySuggest } from '@marmotdata/elements/query-input.js';
	import { m } from '$lib/paraglide/messages';
	import { catalogFields, loadFieldValues } from './catalogSearch';

	// A single search box that highlights Marmot's query syntax and completes fields, operators and values as people type.
	let {
		value = '',
		placeholder = m.query_input_placeholder(),
		isLoading = false,
		onQueryChange = () => {},
		onSubmit = () => {},
		autofocus = false,
		label = m.query_input_label()
	}: {
		value?: string;
		placeholder?: string;
		isLoading?: boolean;
		onQueryChange?: (query: string) => void;
		onSubmit?: () => void;
		/** Kept for callers; suggestions open on their own now. */
		showDropdown?: boolean;
		autofocus?: boolean;
		/** Kept for callers; the box always uses the design-system field. */
		plain?: boolean;
		label?: string;
	} = $props();

	const suggest = createQuerySuggest({
		fields: catalogFields().map((f) => ({
			value: f.value,
			description: f.label,
			values: f.values?.map((v) => (typeof v === 'string' ? v : v.value))
		})),
		values: loadFieldValues
	});

	let input = $state<HTMLElement>();
	$effect(() => {
		if (autofocus) input?.focus();
	});
</script>

<mm-query-input
	bind:this={input}
	{label}
	hide-label
	{value}
	{placeholder}
	loading={isLoading}
	shortcut=""
	{suggest}
	onmm-input={(e: CustomEvent<{ value: string }>) => onQueryChange(e.detail.value)}
	onmm-search={(e: CustomEvent<{ query: string }>) => {
		e.preventDefault();
		onSubmit();
	}}
></mm-query-input>
