<script lang="ts">
	import { auth } from '$lib/stores/auth';
	import { m } from '$lib/paraglide/messages';
	import { domainsEnabled, loadTree } from '$lib/domains/api';
	import { domainOptions, type DomainOption } from '$lib/domains/options';
	import DomainPicker from './DomainPicker.svelte';

	let {
		value = $bindable(''),
		id = 'domain-select'
	}: {
		/** Selected domain id; empty keeps the entity in Unassigned. */
		value?: string;
		id?: string;
	} = $props();

	let options = $state<DomainOption[] | null>(null);

	$effect(() => {
		let cancelled = false;
		if (!auth.hasPermission('domains', 'manage')) return;
		domainsEnabled().then(async (enabled) => {
			if (!enabled || cancelled) return;
			try {
				const forest = await loadTree();
				if (!cancelled) options = domainOptions(forest);
			} catch {
				// Without the tree the entity is created in Unassigned.
			}
		});
		return () => {
			cancelled = true;
		};
	});
</script>

{#if options}
	<div>
		<label for={id} class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
			{m.domains_chip_label()}
		</label>
		<DomainPicker
			{id}
			bind:value
			{options}
			label={m.domains_chip_label()}
			noneLabel={m.domains_unassigned()}
		/>
	</div>
{/if}
