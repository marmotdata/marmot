<script lang="ts">
	import '@marmotdata/elements/combobox.js';
	import { tick } from 'svelte';
	import { fetchApi } from '$lib/api';
	import { m } from '$lib/paraglide/messages';
	import type { Owner } from '$lib/dataproducts/types';

	// Chosen owners sit in their own list with Remove, apart from the search that adds more, so nobody has to untick a result to remove an owner.
	let {
		owners = $bindable([]),
		label,
		help = '',
		error = ''
	}: { owners: Owner[]; label: string; help?: string; error?: string } = $props();

	type Combobox = HTMLElement & { value: string };
	let combobox = $state<Combobox | null>(null);
	// A lookup from option value back to the owner record, not rendered state.
	const found: Record<string, Owner> = {};
	const key = (o: { id: string; type: string }) => `${o.type}:${o.id}`;

	async function loader(query: string, signal: AbortSignal) {
		if (query.trim().length < 2) return [];
		const res = await fetchApi(`/owners/search?q=${encodeURIComponent(query)}&limit=20`, {
			signal
		});
		if (!res.ok) throw new Error(res.statusText);
		const data = await res.json();
		const chosen = new Set(owners.map(key));
		return ((data.owners as Owner[]) || [])
			.filter((o) => !chosen.has(key(o)))
			.map((o) => {
				found[key(o)] = o;
				return {
					value: key(o),
					label: o.name || o.username || o.id,
					description: o.type === 'user' ? o.email || o.username : undefined,
					icon: o.type === 'team' ? 'groups' : 'person'
				};
			});
	}

	async function add(event: CustomEvent<{ value: string }>) {
		const owner = found[event.detail.value];
		if (!owner || owners.some((o) => key(o) === key(owner))) return;
		owners = [...owners, owner];
		await tick();
		if (combobox) combobox.value = '';
	}

	function remove(owner: Owner) {
		owners = owners.filter((o) => key(o) !== key(owner));
		combobox?.focus();
	}
</script>

<div class="mm-stack" style="--mm-gap: var(--space-2)">
	<mm-combobox
		bind:this={combobox}
		{label}
		{help}
		{error}
		placeholder={m.products_owners_placeholder()}
		{loader}
		onmm-change={add}
	></mm-combobox>
	{#if owners.length > 0}
		<ul class="mm-cluster chosen" aria-label={m.products_owners_chosen()}>
			{#each owners as owner (key(owner))}
				<li>
					<mm-owner-chip
						name={owner.name || owner.username || owner.id}
						kind={owner.type === 'team' ? 'team' : 'person'}
						src={owner.profile_picture ?? ''}
						removable
						onmm-remove={() => remove(owner)}
					></mm-owner-chip>
				</li>
			{/each}
		</ul>
	{/if}
</div>

<style>
	.chosen {
		--mm-gap: var(--space-2);
		margin: 0;
		padding: 0;
		list-style: none;
	}
</style>
