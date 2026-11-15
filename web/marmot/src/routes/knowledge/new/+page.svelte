<script lang="ts">
	import '@marmotdata/elements/multiselect.js';
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { fetchApi } from '#lib/api.ts';
	import { userProfile } from '#lib/stores/user.ts';
	import { toasts, parseApiError, isLimitExceeded } from '#lib/stores/toast.ts';
	import { setCrumbs } from '#lib/mm/index.ts';
	import { discoverPath, discoverPathname, type DiscoverPathname } from '#lib/mrn.ts';
	import { m } from '#lib/paraglide/messages.js';
	import { KNOWLEDGE_PROVIDER, knowledgeKindById, knowledgeKinds } from '#lib/knowledge/kinds.ts';

	interface OwnerOption {
		value: string;
		label: string;
		icon: string;
	}

	setCrumbs([
		{ label: m.knowledge_heading(), href: resolve('/knowledge') },
		{ label: m.knowledge_new_heading() }
	]);

	let form = $state<HTMLFormElement>();
	let kindId = $state(knowledgeKindById(page.url.searchParams.get('kind'))?.id ?? '');
	let title = $state('');
	let summary = $state('');
	let owners = $state<string[]>([]);
	let ownerOptions = $state<OwnerOption[]>([]);
	let me = $state<OwnerOption | null>(null);
	let saving = $state(false);
	let error = $state('');
	let errors = $state<{ kind?: string; title?: string }>({});

	let ownerTimer: ReturnType<typeof setTimeout> | undefined;

	let kind = $derived(knowledgeKindById(kindId));

	onMount(() => {
		form?.querySelector<HTMLElement>(kindId ? 'mm-text-field' : 'mm-select')?.focus();
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

	function validate(): boolean {
		errors = {
			kind: kind ? '' : m.knowledge_new_error_kind_required(),
			title: title.trim() ? '' : m.knowledge_new_error_title_required()
		};
		return !errors.kind && !errors.title;
	}

	async function handleSubmit(e: SubmitEvent) {
		e.preventDefault();
		error = '';
		if (!validate() || !kind) return;

		saving = true;
		try {
			// An entry is an asset of the kind's type, so it gets owners, tags, terms and search the same way a table does.
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
				const info = await parseApiError(response);
				if (response.status === 409) {
					errors = { ...errors, title: m.knowledge_new_error_duplicate() };
					return;
				}
				if (isLimitExceeded(info)) toasts.warning(info.message);
				throw new Error(info.message);
			}
			const created: { mrn?: string } = await response.json();
			const path = discoverPath(created.mrn);
			if (!created.mrn || !path) throw new Error(m.knowledge_new_error_create());

			// The outline is the entry's first page. The entry is already saved, so a failure here is reported and the author carries on with an empty page.
			const outline = await fetchApi(
				`/docs/entity/asset/${encodeURIComponent(created.mrn)}/pages`,
				{
					method: 'POST',
					body: JSON.stringify({ parent_id: null, title: title.trim(), content: kind.template })
				}
			).catch(() => null);
			if (!outline?.ok) toasts.warning(m.knowledge_new_outline_failed());

			goto(resolve(`${discoverPathname(path)}?tab=documentation` as DiscoverPathname));
		} catch (err) {
			error = err instanceof Error ? err.message : m.knowledge_new_error_create();
		} finally {
			saving = false;
		}
	}
</script>

<svelte:head>
	<title>{m.knowledge_new_page_title()}</title>
</svelte:head>

<div class="mm-stack" style="--mm-gap: var(--space-6); max-width: 42rem">
	<mm-page-header heading={m.knowledge_new_heading()} description={m.knowledge_new_description()}
	></mm-page-header>

	<form
		bind:this={form}
		class="mm-stack"
		style="--mm-gap: var(--space-5)"
		novalidate
		onsubmit={handleSubmit}
	>
		<mm-select
			label={m.knowledge_new_kind_label()}
			name="kind"
			value={kindId}
			required
			help={kind?.description() ?? ''}
			error={errors.kind ?? ''}
			onmm-change={(e: CustomEvent<{ value: string }>) => {
				kindId = e.detail.value;
				if (errors.kind) errors = { ...errors, kind: '' };
			}}
		>
			{#each knowledgeKinds as k (k.id)}
				<mm-option value={k.id} icon={k.icon}>{k.label()}</mm-option>
			{/each}
		</mm-select>

		<mm-text-field
			label={m.knowledge_new_title_label()}
			name="title"
			value={title}
			required
			help={m.knowledge_new_title_help()}
			error={errors.title ?? ''}
			onmm-input={(e: CustomEvent<{ value: string }>) => {
				title = e.detail.value;
				if (errors.title) errors = { ...errors, title: '' };
			}}
		></mm-text-field>

		<mm-multiselect
			label={m.common_owners()}
			name="owners"
			value={owners}
			required={!!me}
			help={m.knowledge_new_owners_help()}
			onmm-input={(e: CustomEvent<{ query: string }>) => searchOwners(e.detail.query)}
			onmm-change={(e: CustomEvent<{ value: string[] }>) => (owners = e.detail.value)}
		>
			{#each ownerOptions as o (o.value)}
				<mm-option value={o.value} label={o.label} icon={o.icon}></mm-option>
			{/each}
		</mm-multiselect>

		<mm-textarea
			label={m.knowledge_new_summary_label()}
			name="summary"
			optional
			rows={3}
			value={summary}
			help={m.knowledge_new_summary_help()}
			onmm-input={(e: CustomEvent<{ value: string }>) => (summary = e.detail.value)}
		></mm-textarea>

		{#if error}
			<mm-callout tone="danger" heading={m.knowledge_new_error_create()}>{error}</mm-callout>
		{/if}

		<div class="mm-cluster" style="--mm-gap: var(--space-2); justify-content: flex-end">
			<mm-button href={resolve('/knowledge')}>{m.common_cancel()}</mm-button>
			<mm-button variant="primary" type="submit" loading={saving}
				>{m.knowledge_new_create()}</mm-button
			>
		</div>
	</form>
</div>
