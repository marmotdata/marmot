<script lang="ts">
	import { auth } from '$lib/stores/auth';
	import { m } from '$lib/paraglide/messages';
	import { domainsEnabled, flatten, loadTree } from '$lib/domains/api';
	import { isUnassigned } from '$lib/domains/labels';

	let {
		value = $bindable(''),
		id = 'domain-select'
	}: {
		/** Selected domain id; empty keeps the entity in Unassigned. */
		value?: string;
		id?: string;
	} = $props();

	let options = $state<{ id: string; label: string }[] | null>(null);

	$effect(() => {
		let cancelled = false;
		if (!auth.hasPermission('domains', 'manage')) return;
		domainsEnabled().then(async (enabled) => {
			if (!enabled || cancelled) return;
			try {
				const entries = flatten(await loadTree()).filter((e) => !isUnassigned(e.domain));
				if (!cancelled) options = entries.map((e) => ({ id: e.domain.id, label: e.label }));
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
		<select
			{id}
			bind:value
			class="w-full rounded-md border-gray-300 text-sm dark:border-gray-600 dark:bg-gray-800 dark:text-gray-200"
		>
			<option value="">{m.domains_unassigned()}</option>
			{#each options as option (option.id)}
				<option value={option.id}>{option.label}</option>
			{/each}
		</select>
	</div>
{/if}
