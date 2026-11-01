<script lang="ts">
	import '@marmotdata/elements/combobox.js';
	import { tick } from 'svelte';
	import { fetchApi } from '$lib/api';
	import { m } from '$lib/paraglide/messages';

	// Tags are free text, so the combobox accepts anything typed and suggests tags already in use to keep spellings consistent.
	let { tags = $bindable([]), label }: { tags: string[]; label: string } = $props();

	type Combobox = HTMLElement & { value: string };
	let combobox = $state<Combobox | null>(null);

	async function loader(query: string, signal: AbortSignal) {
		const res = await fetchApi(
			`/assets/suggestions/tags?prefix=${encodeURIComponent(query.trim())}&limit=10`,
			{ signal }
		);
		if (!res.ok) throw new Error(res.statusText);
		const data = ((await res.json()) as string[]) || [];
		return data.filter((t) => !tags.includes(t)).map((t) => ({ value: t, label: t }));
	}

	async function add(event: CustomEvent<{ value: string }>) {
		const tag = event.detail.value.trim();
		if (tag && !tags.includes(tag)) tags = [...tags, tag];
		await tick();
		if (combobox) combobox.value = '';
	}

	function remove(tag: string) {
		tags = tags.filter((t) => t !== tag);
		combobox?.focus();
	}
</script>

<div class="mm-stack" style="--mm-gap: var(--space-2)">
	<mm-combobox
		bind:this={combobox}
		{label}
		placeholder={m.products_tags_placeholder()}
		allow-custom
		{loader}
		onmm-change={add}
	></mm-combobox>
	{#if tags.length > 0}
		<ul class="mm-cluster chosen" aria-label={m.products_tags_chosen()}>
			{#each tags as tag (tag)}
				<li><mm-tag removable onmm-remove={() => remove(tag)}>{tag}</mm-tag></li>
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
