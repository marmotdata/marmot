<script lang="ts">
	import '@marmotdata/elements/search-bar.js';
	import { mmIcon } from '$lib/mm';
	import { m } from '$lib/paraglide/messages';
	import { catalogFields, loadFieldValues, loadMetadataFields } from './catalogSearch';

	// The query editor for rule forms: one search bar whose tokens, text view and in-place query builder all write the same query.
	let {
		query = '',
		onQueryChange,
		showRunButton = true,
		runButtonText = m.query_run_button(),
		runButtonIcon = 'material-symbols:play-arrow',
		onRunClick
	}: {
		query?: string;
		onQueryChange: (query: string) => void;
		/** Kept for callers; the bar is always open now. */
		initiallyExpanded?: boolean;
		showRunButton?: boolean;
		runButtonText?: string;
		runButtonIcon?: string;
		onRunClick?: (query: string) => void;
	} = $props();

	const fields = catalogFields();

	let value = $state('');
	$effect(() => {
		value = query;
	});

	// Callers still pass Iconify names; a spinner name means the run is in flight.
	let busy = $derived(/loading/.test(runButtonIcon));
	let icon = $derived(
		busy ? '' : runButtonIcon.startsWith('mdi:') ? 'play-arrow' : mmIcon(runButtonIcon)
	);

	function change(next: string) {
		value = next;
		onQueryChange(next);
	}
</script>

<div class="mm-stack" style="--mm-gap: var(--space-2)">
	<mm-search-bar
		label={m.query_builder_label()}
		placeholder={m.query_builder_placeholder()}
		{value}
		{fields}
		loadFields={loadMetadataFields}
		loadValues={loadFieldValues}
		onmm-input={(e: CustomEvent<{ value: string }>) => change(e.detail.value)}
		onmm-search={() => onRunClick?.(value)}
	></mm-search-bar>
	{#if showRunButton && onRunClick}
		<div class="mm-cluster">
			<mm-button
				variant="secondary"
				size="sm"
				icon={icon || undefined}
				loading={busy}
				onclick={() => onRunClick?.(value)}>{runButtonText}</mm-button
			>
		</div>
	{/if}
</div>
