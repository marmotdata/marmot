<script lang="ts">
	import { m } from '$lib/paraglide/messages';
	import { domainsEnabled, flatten, loadTree } from '$lib/domains/api';
	import { domainInQuery, domainName, withDomain } from '$lib/domains/labels';

	let {
		query,
		onQueryChange
	}: {
		query: string;
		onQueryChange: (query: string) => void;
	} = $props();

	let options = $state<{ id: string; label: string }[] | null>(null);
	const selected = $derived(domainInQuery(query) ?? '');

	$effect(() => {
		let cancelled = false;
		domainsEnabled().then(async (enabled) => {
			if (!enabled || cancelled) return;
			try {
				const entries = flatten(await loadTree());
				if (!cancelled) {
					options = entries.map((e) => ({
						id: e.domain.id,
						label: e.domain.parent_id ? e.label : domainName(e.domain)
					}));
				}
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
		<select
			id="discover-domain-filter"
			class="w-full rounded-md border-gray-300 py-1.5 text-sm dark:border-gray-600 dark:bg-gray-800 dark:text-gray-200"
			value={selected}
			onchange={(e) => onQueryChange(withDomain(query, e.currentTarget.value || null))}
		>
			<option value="">{m.domains_filter_all()}</option>
			{#each options as option (option.id)}
				<option value={option.id}>{option.label}</option>
			{/each}
		</select>
	</div>
{/if}
