<script lang="ts">
	import '@marmotdata/elements/textarea.js';
	import { m } from '$lib/paraglide/messages';
	import type { Owner } from '$lib/dataproducts/types';
	import OwnerField from '$components/product/OwnerField.svelte';

	// A pipeline's description and its one owner, shared by the create and edit forms
	let {
		description = $bindable(''),
		owner = $bindable(null)
	}: { description: string; owner: Owner | null } = $props();

	// The owner picker adds to a list; a pipeline has one owner, so the latest choice replaces the earlier one
	let owners = $state<Owner[]>(owner ? [owner] : []);

	$effect(() => {
		if (owners.length > 1) owners = [owners[owners.length - 1]];
		owner = owners[0] ?? null;
	});
</script>

<div class="mm-stack" style="--mm-gap: var(--space-5)">
	<mm-textarea
		label={m.pipelines_description_label()}
		name="description"
		optional
		rows={3}
		value={description}
		onmm-input={(e: CustomEvent<{ value: string }>) => (description = e.detail.value)}
	></mm-textarea>
	<OwnerField bind:owners label={m.pipelines_owner_label()} help={m.pipelines_owner_help()} />
</div>
