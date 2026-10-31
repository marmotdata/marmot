<script lang="ts">
	import { onMount, tick } from 'svelte';
	import '@marmotdata/elements/drawer.js';
	import { searchTerms } from '$lib/glossary/api';
	import type { GlossaryTerm } from '$lib/glossary/types';
	import type { FieldAnnotation } from '$lib/assets/types';
	import { m } from '$lib/paraglide/messages';

	type FieldTerm = FieldAnnotation['glossary_terms'][number];

	// Links glossary terms to one schema field: the terms already linked sit apart from the search results and each Link or Unlink saves at once, with Undo in a toast raised by the caller.
	let {
		path,
		terms,
		busy,
		onLink,
		onUnlink,
		onClose
	}: {
		path: string;
		/** The terms linked to the field now, kept current by the caller after each save. */
		terms: FieldTerm[];
		/** The id of the term being saved, if any. */
		busy: string | null;
		onLink: (term: FieldTerm) => Promise<boolean>;
		onUnlink: (term: FieldTerm) => Promise<boolean>;
		onClose: () => void;
	} = $props();

	let search = $state('');
	let results = $state<GlossaryTerm[]>([]);
	let loading = $state(true);
	let body = $state<HTMLElement | null>(null);
	let timer: ReturnType<typeof setTimeout> | undefined;
	let request = 0;

	let linkedIds = $derived(new Set(terms.map((t) => t.id)));
	// Linked terms live in their own list, so results only offer what is not linked yet
	let offered = $derived(results.filter((t) => !linkedIds.has(t.id)));
	let searchingNow = $derived(!!search.trim());

	onMount(runSearch);

	function onSearch(value: string) {
		search = value;
		clearTimeout(timer);
		timer = setTimeout(runSearch, 200);
	}

	// With nothing typed the glossary's first terms are offered, so a small glossary needs no search at all
	async function runSearch() {
		const id = ++request;
		loading = true;
		try {
			const res = await searchTerms(search.trim(), null, 0, 25);
			if (id === request) results = res.terms ?? [];
		} catch {
			if (id === request) results = [];
		} finally {
			if (id === request) loading = false;
		}
	}

	// After a row moves to the other list, keep focus in the list it left so keyboard users can carry on
	async function refocus(list: 'linked' | 'results', index: number) {
		await tick();
		const buttons = body?.querySelectorAll<HTMLElement>(`[data-list="${list}"] mm-button`);
		const next = buttons?.[Math.min(index, (buttons?.length ?? 1) - 1)];
		(next ?? body?.querySelector<HTMLElement>('mm-search-field'))?.focus();
	}

	async function link(term: GlossaryTerm, index: number) {
		if (await onLink({ id: term.id, name: term.name })) refocus('results', index);
	}

	async function unlink(term: FieldTerm, index: number) {
		if (await onUnlink(term)) refocus('linked', index);
	}
</script>

<mm-drawer
	open
	dismissible
	heading={m.schema_field_terms_heading()}
	description={path}
	onmm-close={onClose}
>
	<div bind:this={body} class="mm-stack" style="--mm-gap: var(--space-6)">
		<mm-section heading={m.schema_field_terms_linked()}>
			{#if terms.length === 0}
				<mm-text tone="secondary">{m.schema_field_terms_linked_empty()}</mm-text>
			{:else}
				<mm-list
					aria-label={m.schema_field_terms_linked()}
					data-list="linked"
					style="--mm-list-inset: 0"
				>
					{#each terms as t, i (t.id)}
						<mm-list-item>
							<mm-icon slot="start" name="menu-book" aria-hidden="true"></mm-icon>
							{t.name}
							<mm-button
								slot="end"
								size="sm"
								variant="ghost"
								icon="check"
								pressed="true"
								label={m.glossary_unlink_label({ name: t.name })}
								loading={busy === t.id}
								disabled={busy !== null && busy !== t.id}
								onclick={() => unlink(t, i)}>{m.glossary_linked_button()}</mm-button
							>
						</mm-list-item>
					{/each}
				</mm-list>
			{/if}
		</mm-section>

		<mm-section
			heading={searchingNow
				? m.schema_field_terms_results()
				: m.schema_field_terms_browse_heading()}
		>
			<div class="mm-stack" style="--mm-gap: var(--space-3)">
				<mm-search-field
					label={m.schema_field_terms_search_label()}
					hide-label
					placeholder={m.schema_field_terms_search()}
					value={search}
					autofocus
					onmm-input={(e: CustomEvent<{ value: string }>) => onSearch(e.detail.value)}
				></mm-search-field>
				{#if loading && offered.length === 0}
					<mm-skeleton loading shape="text" count={3}></mm-skeleton>
				{:else if offered.length === 0}
					{#if searchingNow}
						<mm-empty-state
							compact
							heading={m.schema_field_terms_none()}
							description={m.schema_field_terms_none_hint()}
						></mm-empty-state>
					{:else}
						<mm-text tone="secondary">{m.schema_field_terms_browse_none()}</mm-text>
					{/if}
				{:else}
					<mm-list
						aria-label={searchingNow
							? m.schema_field_terms_results()
							: m.schema_field_terms_browse_heading()}
						data-list="results"
						style="--mm-list-inset: 0"
					>
						{#each offered as t, i (t.id)}
							<mm-list-item secondary={t.definition}>
								<mm-icon slot="start" name="menu-book" aria-hidden="true"></mm-icon>
								{t.name}
								<mm-button
									slot="end"
									size="sm"
									icon="add"
									label={m.glossary_link_button_label({ name: t.name })}
									loading={busy === t.id}
									disabled={busy !== null && busy !== t.id}
									onclick={() => link(t, i)}>{m.glossary_link_button()}</mm-button
								>
							</mm-list-item>
						{/each}
					</mm-list>
				{/if}
			</div>
		</mm-section>
	</div>
	<mm-button slot="footer" variant="primary" data-close="done">{m.glossary_link_done()}</mm-button>
</mm-drawer>
