<script lang="ts">
	import '@marmotdata/elements/multiselect.js';
	import '@marmotdata/elements/link-list.js';
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { fetchApi } from '#lib/api.ts';
	import { userProfile } from '#lib/stores/user.ts';
	import { toasts, parseApiError, isLimitExceeded } from '#lib/stores/toast.ts';
	import { setCrumbs } from '#lib/mm/index.ts';
	import { m } from '#lib/paraglide/messages.js';
	import { KNOWLEDGE_PROVIDER, kindById, knowledgeKinds, loadKinds } from '#lib/knowledge/kinds.ts';
	import { saveEntry } from '#lib/knowledge/api.ts';
	import { draftErrors, newDraft, toInput, type EntryDraft } from '#lib/knowledge/draft.ts';
	import EntryDetailsForm from '#components/knowledge/EntryDetailsForm.svelte';

	interface OwnerOption {
		value: string;
		label: string;
		icon: string;
	}

	let loadFailed = $state(false);
	let kind = $derived(kindById(page.url.searchParams.get('kind'), $knowledgeKinds));

	let form = $state<HTMLFormElement>();
	let title = $state('');
	let summary = $state('');
	let owners = $state<string[]>([]);
	let ownerOptions = $state<OwnerOption[]>([]);
	let me = $state<OwnerOption | null>(null);
	let draft = $state<EntryDraft | null>(null);
	let saving = $state(false);
	let error = $state('');
	let titleError = $state('');
	let existingHref = $state('');
	let fieldErrors = $state<Record<string, string>>({});
	let ownerTimer: ReturnType<typeof setTimeout> | undefined;

	// A new kind starts a fresh draft, so switching kind never carries fields the new kind does not have.
	$effect(() => {
		if (kind) draft = newDraft(kind);
	});

	$effect(() => {
		setCrumbs([
			{ label: m.knowledge_heading(), href: resolve('/knowledge') },
			kind
				? { label: m.knowledge_new_kind_heading({ kind: kind.label }) }
				: { label: m.knowledge_new_heading() }
		]);
	});

	let outline = $derived(
		(kind?.template ?? '')
			.split('\n')
			.filter((l) => l.startsWith('## '))
			.map((l) => l.slice(3))
	);

	onMount(() => {
		loadKinds().catch(() => (loadFailed = true));
		// The owner defaults to whoever is writing the entry, since nothing else keeps it true.
		userProfile.load().then((profile) => {
			if (!profile) return;
			me = {
				value: `user:${profile.id}`,
				label: m.assetnew_owner_you({ name: profile.name || profile.username }),
				icon: 'person'
			};
			ownerOptions = [me, ...ownerOptions.filter((o) => o.value !== me!.value)];
			if (!owners.length) owners = [me.value];
		});
	});

	function searchOwners(query: string) {
		clearTimeout(ownerTimer);
		if (query.trim().length < 2) return;
		ownerTimer = setTimeout(async () => {
			try {
				const res = await fetchApi(`/owners/search?q=${encodeURIComponent(query)}&limit=20`);
				if (!res.ok) return;
				const data: { owners?: { id: string; name: string; type: 'user' | 'team' }[] } =
					await res.json();
				const found = (data.owners ?? []).map((o) => ({
					value: `${o.type}:${o.id}`,
					label: o.name,
					icon: o.type === 'team' ? 'groups' : 'person'
				}));
				const kept = ownerOptions.filter((o) => owners.includes(o.value) || o.value === me?.value);
				ownerOptions = [...kept, ...found.filter((f) => !kept.some((k) => k.value === f.value))];
			} catch {
				// Suggestions are a convenience; the field still works without them.
			}
		}, 200);
	}

	async function handleSubmit(e: SubmitEvent) {
		e.preventDefault();
		if (!kind || !draft) return;
		error = '';
		existingHref = '';
		titleError = title.trim() ? '' : m.knowledge_new_error_title_required();
		fieldErrors = draftErrors(kind, draft);
		if (titleError || Object.keys(fieldErrors).length) {
			form?.querySelector<HTMLElement>('[error]:not([error=""])')?.focus();
			return;
		}

		saving = true;
		let createdId = '';
		try {
			const payload: Record<string, unknown> = {
				name: title.trim(),
				type: kind.type,
				providers: [KNOWLEDGE_PROVIDER]
			};
			if (summary.trim()) payload.user_description = summary.trim();
			if (owners.length)
				payload.owners = owners.map((o) => {
					const [type, id] = o.split(':');
					return { type, id };
				});

			const response = await fetchApi('/assets/', {
				method: 'POST',
				body: JSON.stringify(payload)
			});
			if (!response.ok) {
				const copy = response.clone();
				const info = await parseApiError(response);
				if (response.status === 409) {
					const existing: { existing_id?: string } = await copy.json().catch(() => ({}));
					titleError = m.knowledge_new_error_duplicate({ kind: kind.label.toLowerCase() });
					if (existing.existing_id) existingHref = `/knowledge/${existing.existing_id}`;
					return;
				}
				if (isLimitExceeded(info)) toasts.warning(info.message);
				throw new Error(info.message);
			}
			const created: { id: string; mrn?: string } = await response.json();
			createdId = created.id;

			try {
				await saveEntry(created.id, toInput(draft));
			} catch (err) {
				// The details did not save, so the half-made entry is removed rather than left without a status.
				await fetchApi(`/assets/${created.id}`, { method: 'DELETE' }).catch(() => {});
				createdId = '';
				throw err;
			}

			if (kind.template && created.mrn) {
				const outlineRes = await fetchApi(
					`/docs/entity/asset/${encodeURIComponent(created.mrn)}/pages`,
					{
						method: 'POST',
						body: JSON.stringify({ parent_id: null, title: title.trim(), content: kind.template })
					}
				).catch(() => null);
				if (!outlineRes?.ok) toasts.warning(m.knowledge_new_outline_failed());
			}

			toasts.success(m.knowledge_new_created({ name: title.trim() }));
			goto(resolve('/knowledge/[id]', { id: created.id }));
		} catch (err) {
			error = err instanceof Error ? err.message : m.knowledge_new_error_create();
		} finally {
			if (!createdId) saving = false;
		}
	}
</script>

<svelte:head>
	<title
		>{kind
			? m.knowledge_new_kind_page_title({ kind: kind.label })
			: m.knowledge_new_page_title()}</title
	>
</svelte:head>

{#if !kind}
	<div class="mm-stack" style="--mm-gap: var(--space-6); max-width: 48rem">
		<mm-page-header heading={m.knowledge_new_heading()} description={m.knowledge_new_description()}
		></mm-page-header>
		{#if loadFailed}
			<mm-callout tone="danger" heading={m.knowledge_load_failed()}></mm-callout>
		{:else if $knowledgeKinds.length === 0}
			<mm-skeleton loading count={6}></mm-skeleton>
		{:else}
			<mm-link-list columns="2">
				{#each $knowledgeKinds as k (k.id)}
					<mm-link-list-item
						href={`${resolve('/knowledge/new')}?kind=${k.id}`}
						icon={k.icon}
						description={k.description}>{k.label}</mm-link-list-item
					>
				{/each}
			</mm-link-list>
		{/if}
	</div>
{:else if draft}
	<div class="mm-stack" style="--mm-gap: var(--space-6); max-width: 46rem">
		<mm-page-header
			heading={m.knowledge_new_kind_heading({ kind: kind.label })}
			icon={kind.icon}
			description={kind.description}
		>
			<mm-button slot="actions" variant="ghost" size="sm" href={resolve('/knowledge/new')}
				>{m.knowledge_new_change_kind()}</mm-button
			>
		</mm-page-header>

		<form
			bind:this={form}
			class="mm-stack"
			style="--mm-gap: var(--space-6)"
			novalidate
			onsubmit={handleSubmit}
		>
			<div class="mm-stack" style="--mm-gap: var(--space-5)">
				<mm-text-field
					label={m.knowledge_new_title_label()}
					name="title"
					value={title}
					required
					autofocus
					help={m.knowledge_new_title_help({ kind: kind.label.toLowerCase() })}
					error={titleError}
					onmm-input={(e: CustomEvent<{ value: string }>) => {
						title = e.detail.value;
						if (titleError) titleError = '';
					}}
				></mm-text-field>
				{#if existingHref}
					<mm-text variant="caption" tone="secondary" block class="pull-up">
						<mm-link href={resolve(existingHref as '/')}>{m.knowledge_new_open_existing()}</mm-link>
					</mm-text>
				{/if}

				<mm-textarea
					label={m.knowledge_new_summary_label()}
					name="summary"
					optional
					rows={2}
					autosize
					value={summary}
					help={m.knowledge_new_summary_help()}
					onmm-input={(e: CustomEvent<{ value: string }>) => (summary = e.detail.value)}
				></mm-textarea>

				<mm-multiselect
					label={m.common_owners()}
					name="owners"
					value={owners}
					help={m.knowledge_new_owners_help()}
					onmm-input={(e: CustomEvent<{ query: string }>) => searchOwners(e.detail.query)}
					onmm-change={(e: CustomEvent<{ value: string[] }>) => (owners = e.detail.value)}
				>
					{#each ownerOptions as o (o.value)}
						<mm-option value={o.value} label={o.label} icon={o.icon}></mm-option>
					{/each}
				</mm-multiselect>
			</div>

			<EntryDetailsForm {kind} bind:draft errors={fieldErrors} compact />

			{#if outline.length}
				<mm-section
					heading={m.knowledge_new_writeup_heading()}
					description={m.knowledge_new_writeup_description()}
				>
					<ol class="outline">
						{#each outline as h (h)}
							<li>{h}</li>
						{/each}
					</ol>
				</mm-section>
			{/if}

			{#if error}
				<mm-callout tone="danger" heading={m.knowledge_new_error_create()}>{error}</mm-callout>
			{/if}

			<div class="mm-cluster footer">
				<mm-button href={resolve('/knowledge')}>{m.common_cancel()}</mm-button>
				<mm-button variant="primary" type="submit" loading={saving}
					>{m.knowledge_new_create_kind({ kind: kind.label.toLowerCase() })}</mm-button
				>
			</div>
		</form>
	</div>
{/if}

<style>
	.pull-up {
		margin-block-start: calc(-1 * var(--space-3));
	}
	.outline {
		margin: 0;
		padding-inline-start: var(--space-5);
		color: var(--mm-text-2);
		font: var(--text-dense);
	}
	.outline li + li {
		margin-block-start: var(--space-1);
	}
	.footer {
		--mm-gap: var(--space-2);
		justify-content: flex-end;
		border-block-start: 1px solid var(--mm-rule);
		padding-block-start: var(--space-4);
	}
</style>
