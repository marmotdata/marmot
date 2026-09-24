<script lang="ts">
	import { m } from '$lib/paraglide/messages';
	import { domainsEnabled, loadTree } from '$lib/domains/api';
	import { domainInQuery, withDomain } from '$lib/domains/labels';
	import { domainOptions, type DomainOption } from '$lib/domains/options';
	import DomainPicker from './DomainPicker.svelte';

	let {
		query,
		onQueryChange
	}: {
		query: string;
		onQueryChange: (query: string) => void;
	} = $props();

	let options = $state<DomainOption[] | null>(null);
	const selected = $derived(domainInQuery(query) ?? '');

	$effect(() => {
		let cancelled = false;
		domainsEnabled().then(async (enabled) => {
			if (!enabled || cancelled) return;
			try {
				const forest = await loadTree();
				if (!cancelled) options = domainOptions(forest, { includeUnassigned: true });
			} catch {
				// Without the tree there is nothing to filter by.
			}
		});
		return () => {
			cancelled = true;
		};
	});
</script>

{#if options}
	<div class="mb-4">
		<label
			for="discover-domain-filter"
			class="mb-2 block text-xs font-semibold uppercase tracking-wider text-gray-600 dark:text-gray-400"
		>
			{m.domains_filter_label()}
		</label>
		<DomainPicker
			id="discover-domain-filter"
			value={selected}
			{options}
			label={m.domains_filter_label()}
			noneLabel={m.domains_filter_all()}
			onSelect={(id) => onQueryChange(withDomain(query, id || null))}
		/>
	</div>
{/if}
