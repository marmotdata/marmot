<script lang="ts">
	import { onMount, tick, untrack } from 'svelte';
	import { SvelteMap, SvelteSet } from 'svelte/reactivity';
	import '@marmotdata/elements/combobox.js';
	import '@marmotdata/elements/multiselect.js';
	import { fetchApi } from '$lib/api';
	import { userProfile } from '$lib/stores/user';
	import { m } from '$lib/paraglide/messages';
	import { possibleParents } from '$lib/glossary/tree';
	import type { CreateTermInput, GlossaryTerm, Owner } from '$lib/glossary/types';

	let {
		term = undefined,
		allTerms,
		initialName = '',
		initialParent = '',
		cancelHref = undefined,
		onCancel = undefined,
		onSubmit
	}: {
		/** The term being edited; leave out to create one. */
		term?: GlossaryTerm;
		allTerms: GlossaryTerm[];
		initialName?: string;
		initialParent?: string;
		cancelHref?: string;
		onCancel?: () => void;
		/** Saves the input and resolves with an error message, or nothing when it worked. */
		onSubmit: (input: CreateTermInput) => Promise<string | void>;
	} = $props();

	const formId = $props.id();
	// The form is a draft: it starts from the props once and the parent remounts it for another term
	const start = untrack(() => ({ term, name: initialName, parent: initialParent }));
	const editing = !!start.term;

	let name = $state(start.term?.name ?? start.name);
	let definition = $state(start.term?.definition ?? '');
	let description = $state(start.term?.description ?? '');
	let showDescription = $state(!!start.term?.description);
	let parent = $state(start.term?.parent_term_id ?? start.parent);
	let owners = $state<Owner[]>([...(start.term?.owners ?? [])]);
	let tags = $state<string[]>([...(start.term?.tags ?? [])]);
	let related = $state<string[]>((start.term?.related_terms ?? []).map((t) => t.id));

	let nameError = $state('');
	let definitionError = $state('');
	let formError = $state('');
	let saving = $state(false);

	let ownerResults = $state<Owner[]>([]);
	let tagQuery = $state('');
	let descriptionField = $state<HTMLElement | null>(null);

	const ownerKey = (o: Pick<Owner, 'id' | 'type'>) => `${o.type}:${o.id}`;

	// A term cannot sit under itself or one of its own descendants, so those are left out of the parent picker
	let parentOptions = $derived(possibleParents(allTerms, term?.id));
	// Related terms sit beside this one, so any other term qualifies
	let relatedOptions = $derived(allTerms.filter((t) => t.id !== term?.id));

	let ownerOptions = $derived.by(() => {
		const seen = new SvelteMap<string, Owner>();
		for (const o of [...owners, ...ownerResults]) seen.set(ownerKey(o), o);
		return [...seen.values()];
	});

	// Tags are free-form, so the typed text is offered as an option alongside the tags other terms already use
	let tagOptions = $derived.by(() => {
		const known = new SvelteSet<string>(tags);
		for (const t of allTerms) for (const tag of t.tags ?? []) known.add(tag);
		const typed = tagQuery.trim();
		return { known: [...known].sort(), typed: typed && !known.has(typed) ? typed : '' };
	});

	onMount(async () => {
		// New terms start owned by the person creating them
		if (editing || owners.length) return;
		const me = await userProfile.load();
		if (me && owners.length === 0) {
			owners = [{ id: me.id, name: me.name || me.username, username: me.username, type: 'user' }];
		}
	});

	let ownerRequest = 0;
	async function searchOwners(query: string) {
		const id = ++ownerRequest;
		const q = query.trim();
		if (q.length < 2) {
			ownerResults = [];
			return;
		}
		try {
			const res = await fetchApi(`/owners/search?q=${encodeURIComponent(q)}&limit=20`);
			const data = res.ok ? await res.json() : { owners: [] };
			if (id === ownerRequest) ownerResults = data.owners ?? [];
		} catch {
			if (id === ownerRequest) ownerResults = [];
		}
	}

	function setOwners(keys: string[]) {
		const byKey = new Map(ownerOptions.map((o) => [ownerKey(o), o]));
		owners = keys.map((k) => byKey.get(k)).filter((o): o is Owner => !!o);
	}

	function validateName() {
		nameError = name.trim() ? '' : m.glossary_error_name_required();
	}
	function validateDefinition() {
		definitionError = definition.trim() ? '' : m.glossary_error_definition_required();
	}

	async function addDescription() {
		showDescription = true;
		await tick();
		descriptionField?.focus();
	}

	async function submit(e?: Event) {
		e?.preventDefault();
		if (saving) return;
		validateName();
		validateDefinition();
		if (nameError || definitionError) return;
		saving = true;
		formError = '';
		const input: CreateTermInput = {
			name: name.trim(),
			definition: definition.trim(),
			owners: owners.map((o) => ({ id: o.id, type: o.type })),
			tags
		};
		const desc = description.trim();
		if (desc || editing) input.description = desc;
		// An empty parent on an edit moves the term to the top level
		if (parent || editing) input.parent_term_id = parent;
		// On an edit an empty list clears the related terms, so it is always sent
		if (related.length || editing) input.related_term_ids = related;
		const failure = await onSubmit(input);
		saving = false;
		if (failure) formError = failure;
	}
</script>

<div class="mm-page mm-stack">
	<mm-page-header heading={editing ? (term?.name ?? '') : m.glossary_new_term_heading()}>
		<mm-button slot="actions" href={cancelHref} onclick={onCancel} disabled={saving}
			>{m.common_cancel()}</mm-button
		>
		<mm-button slot="actions" variant="primary" loading={saving} onclick={() => submit()}
			>{editing ? m.glossary_save_button() : m.glossary_create_term_button()}</mm-button
		>
	</mm-page-header>

	{#if formError}
		<mm-callout tone="danger">{formError}</mm-callout>
	{/if}

	<form id={formId} class="mm-split" onsubmit={submit} novalidate>
		<div class="mm-stack" style="--mm-gap: var(--space-5)">
			<!-- svelte-ignore a11y_autofocus -->
			<mm-text-field
				label={m.common_name()}
				name="name"
				required
				autofocus
				value={name}
				error={nameError}
				oninput={(e: Event) => {
					name = (e.currentTarget as HTMLInputElement).value;
					if (nameError) validateName();
				}}
				onfocusout={validateName}
			></mm-text-field>
			<mm-textarea
				label={m.glossary_definition_label()}
				name="definition"
				rows={3}
				required
				placeholder={m.glossary_definition_placeholder()}
				value={definition}
				error={definitionError}
				oninput={(e: Event) => {
					definition = (e.currentTarget as HTMLTextAreaElement).value;
					if (definitionError) validateDefinition();
				}}
				onfocusout={validateDefinition}
			></mm-textarea>
			{#if showDescription}
				<mm-textarea
					bind:this={descriptionField}
					label={m.glossary_description_label()}
					name="description"
					rows={10}
					help={m.glossary_description_help()}
					placeholder={m.glossary_description_placeholder()}
					value={description}
					oninput={(e: Event) => (description = (e.currentTarget as HTMLTextAreaElement).value)}
				></mm-textarea>
			{:else}
				<div>
					<mm-button size="sm" variant="ghost" icon="add" onclick={addDescription}
						>{m.glossary_add_description()}</mm-button
					>
				</div>
			{/if}
		</div>

		<div class="mm-stack" style="--mm-gap: var(--space-5)">
			<mm-combobox
				label={m.glossary_parent_label()}
				name="parent"
				value={parent}
				onmm-change={(e: CustomEvent<{ value: string }>) => (parent = e.detail.value)}
			>
				{#each parentOptions as t (t.id)}
					<mm-option value={t.id} label={t.name}></mm-option>
				{/each}
			</mm-combobox>
			<mm-multiselect
				label={m.glossary_related_terms_label()}
				name="related"
				placeholder={m.glossary_related_terms_placeholder()}
				value={related}
				onmm-change={(e: CustomEvent<{ value: string[] }>) => (related = e.detail.value)}
			>
				{#each relatedOptions as t (t.id)}
					<mm-option value={t.id} label={t.name}></mm-option>
				{/each}
			</mm-multiselect>
			<mm-multiselect
				label={m.common_owners()}
				name="owners"
				placeholder={m.glossary_owners_placeholder()}
				value={owners.map(ownerKey)}
				onmm-input={(e: CustomEvent<{ query: string }>) => searchOwners(e.detail.query)}
				onmm-change={(e: CustomEvent<{ value: string[] }>) => setOwners(e.detail.value)}
			>
				{#each ownerOptions as o (ownerKey(o))}
					<mm-option
						value={ownerKey(o)}
						label={o.name || o.username || o.id}
						icon={o.type === 'team' ? 'groups' : 'person'}
					></mm-option>
				{/each}
			</mm-multiselect>
			<mm-multiselect
				label={m.common_tags()}
				name="tags"
				value={tags}
				onmm-input={(e: CustomEvent<{ query: string }>) => (tagQuery = e.detail.query)}
				onmm-change={(e: CustomEvent<{ value: string[] }>) => {
					tags = e.detail.value;
					tagQuery = '';
				}}
			>
				{#if tagOptions.typed}
					<mm-option
						value={tagOptions.typed}
						label={tagOptions.typed}
						description={m.glossary_tag_create()}
						icon="add"
					></mm-option>
				{/if}
				{#each tagOptions.known as tag (tag)}
					<mm-option value={tag} label={tag}></mm-option>
				{/each}
			</mm-multiselect>
		</div>
	</form>
</div>
