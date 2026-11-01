<script lang="ts">
	import '@marmotdata/elements/combobox.js';
	import '@marmotdata/elements/multiselect.js';
	import '@marmotdata/elements/key-value-editor.js';
	import { pairsToObject, type KeyValuePair } from '@marmotdata/elements/key-value-editor.js';
	import type { MmOptionData } from '@marmotdata/elements/select.js';
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { fetchApi } from '$lib/api';
	import { auth } from '$lib/stores/auth';
	import { userProfile } from '$lib/stores/user';
	import { toasts, parseApiError, isLimitExceeded } from '$lib/stores/toast';
	import { providerNames, typeNames } from '$lib/assetNames';
	import { setCrumbs } from '$lib/mm';
	import { m } from '$lib/paraglide/messages';

	interface OwnerOption {
		value: string;
		label: string;
		icon: string;
	}

	type Group = 'tags' | 'terms' | 'links' | 'metadata';

	setCrumbs([
		{ label: m.discover_heading(), href: resolve('/discover') },
		{ label: m.assetnew_heading() }
	]);

	const canAddPlugins = auth.hasPermission('ingestion', 'manage');

	// A few common types carry an icon in the list; the rest read by name.
	const typeIcons: Record<string, string> = {
		table: 'table',
		view: 'table-view',
		topic: 'stream',
		dashboard: 'dashboard',
		api: 'api',
		model: 'model-training',
		database: 'database',
		dag: 'account-tree'
	};
	const typeOptions = Object.entries(typeNames)
		.map(([key, label]) => ({ key, label, icon: typeIcons[key] ?? '' }))
		.sort((a, b) => a.label.localeCompare(b.label));
	// Several provider keys share a display name (glue and aws_glue), and the picker is keyed by name, so keep the first of each
	const providerOptions = Object.entries(providerNames)
		.map(([key, label]) => ({ key, label }))
		.filter((p, i, all) => all.findIndex((o) => o.label === p.label) === i)
		.sort((a, b) => a.label.localeCompare(b.label));

	let form = $state<HTMLFormElement>();
	let name = $state('');
	let assetType = $state('');
	let providers = $state<string[]>([]);
	let providerQuery = $state('');
	let owners = $state<string[]>([]);
	let ownerOptions = $state<OwnerOption[]>([]);
	let description = $state('');
	let tags = $state<string[]>([]);
	let tagQuery = $state('');
	let tagSuggestions = $state<string[]>([]);
	let termIds = $state<string[]>([]);
	let links = $state<KeyValuePair[]>([]);
	let metadata = $state<KeyValuePair[]>([]);
	let open = $state<Record<Group, boolean>>({
		tags: false,
		terms: false,
		links: false,
		metadata: false
	});
	let createAnother = $state(false);
	let saving = $state(false);
	let error = $state('');
	let duplicateHref = $state('');
	let errors = $state<Record<string, string>>({});
	let plugins = $state<{ id: string; name: string }[]>([]);
	let me = $state<OwnerOption | null>(null);

	let ownerTimer: ReturnType<typeof setTimeout> | undefined;
	let tagTimer: ReturnType<typeof setTimeout> | undefined;

	onMount(() => {
		form?.querySelector<HTMLElement>('mm-text-field')?.focus();
		// The owner defaults to whoever is adding the asset, since nothing else keeps it up to date.
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
		if (canAddPlugins) {
			fetchApi('/plugins')
				.then((r) => (r.ok ? r.json() : null))
				.then((data) => (plugins = Array.isArray(data?.plugins) ? data.plugins : []))
				.catch(() => {});
		}
	});

	// Known providers plus whatever was typed, so a source Marmot has no logo for can still be named.
	let providerChoices = $derived.by(() => {
		const typed = providerQuery.trim();
		const list = providerOptions.map((p) => ({ value: p.label, label: p.label, provider: p.key }));
		const extra = [...providers, typed].filter(
			(v, i, all) =>
				v && all.indexOf(v) === i && !list.some((p) => p.value.toLowerCase() === v.toLowerCase())
		);
		return [...extra.map((v) => ({ value: v, label: v, provider: '' })), ...list];
	});

	let tagChoices = $derived.by(() => {
		const typed = tagQuery.trim();
		return [...tags, ...tagSuggestions, typed].filter((v, i, all) => v && all.indexOf(v) === i);
	});

	/** The first chosen provider that a plugin catalogs, offered once as a better path. */
	let pluginHint = $derived.by(() => {
		for (const p of providers) {
			const key = providerOptions.find((o) => o.label === p)?.key ?? p.toLowerCase();
			const plugin = plugins.find((x) => x.id === key || x.name.toLowerCase() === p.toLowerCase());
			if (plugin) return plugin;
		}
		return null;
	});

	function validateName(v: string): string {
		if (!v.trim()) return m.assetnew_error_name_required();
		if (v.trim().length < 2) return m.assetnew_error_name_too_short();
		if (!/^[a-zA-Z0-9_\-.]+$/.test(v.trim())) return m.assetnew_error_name_charset();
		return '';
	}

	function validate(field: 'name' | 'type' | 'providers'): string {
		const msg =
			field === 'name'
				? validateName(name)
				: field === 'type'
					? assetType.trim()
						? ''
						: m.assetnew_error_type_required()
					: providers.length
						? ''
						: m.assetnew_error_provider_required();
		errors = { ...errors, [field]: msg };
		return msg;
	}

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

	function searchTags(query: string) {
		tagQuery = query;
		clearTimeout(tagTimer);
		tagTimer = setTimeout(async () => {
			try {
				const res = await fetchApi(
					`/assets/suggestions/tags?${new URLSearchParams({ prefix: query, limit: '10' })}`
				);
				const data: unknown = res.ok ? await res.json() : [];
				tagSuggestions = Array.isArray(data)
					? data.filter((t): t is string => typeof t === 'string')
					: [];
			} catch {
				tagSuggestions = [];
			}
		}, 150);
	}

	async function loadTerms(query: string, signal: AbortSignal): Promise<MmOptionData[]> {
		const res = await fetchApi(`/glossary/search?q=${encodeURIComponent(query)}`, { signal });
		if (!res.ok) return [];
		const data: { terms?: { id: string; name: string; definition?: string }[] } = await res.json();
		return (data.terms ?? []).map((t) => ({
			value: t.id,
			label: t.name,
			description: t.definition,
			icon: 'menu-book'
		}));
	}

	function reveal(group: Group) {
		open = { ...open, [group]: true };
	}

	function assetRoute(type: string, provider: string, assetName: string) {
		return `/discover/${encodeURIComponent(type.toLowerCase())}/${encodeURIComponent(provider.toLowerCase())}/${encodeURIComponent(assetName)}`;
	}

	function reset() {
		name = '';
		description = '';
		tags = [];
		termIds = [];
		links = [];
		metadata = [];
		errors = {};
		open = { tags: false, terms: false, links: false, metadata: false };
		form?.querySelector<HTMLElement>('mm-text-field')?.focus();
	}

	async function handleSubmit(e: SubmitEvent) {
		e.preventDefault();
		error = '';
		duplicateHref = '';
		const problems = [validate('name'), validate('type'), validate('providers')].filter(Boolean);
		if (problems.length) return;

		saving = true;
		try {
			const payload: Record<string, unknown> = {
				name: name.trim(),
				type: assetType.trim(),
				providers
			};
			if (tags.length) payload.tags = tags;
			const meta = metadata.filter((p) => p.key.trim());
			if (meta.length) payload.metadata = pairsToObject(meta);
			const external = links
				.filter((l) => l.key.trim() && l.value.trim())
				.map((l) => ({ name: l.key.trim(), url: l.value.trim() }));
			if (external.length) payload.external_links = external;
			// Owners, the description and glossary terms go in the same request, so the asset is created with all of them or not at all.
			if (description.trim()) payload.user_description = description.trim();
			if (owners.length)
				payload.owners = owners.map((o) => {
					const [type, id] = o.split(':');
					return { type, id };
				});
			if (termIds.length) payload.terms = termIds;

			const response = await fetchApi('/assets/', {
				method: 'POST',
				body: JSON.stringify(payload)
			});
			if (!response.ok) {
				const info = await parseApiError(response);
				if (response.status === 409) {
					duplicateHref = assetRoute(payload.type as string, providers[0], payload.name as string);
					errors = { ...errors, name: m.assetnew_error_duplicate() };
					return;
				}
				if (isLimitExceeded(info)) toasts.warning(info.message);
				throw new Error(info.message);
			}
			const created: { id: string; name: string; type: string; providers: string[] } =
				await response.json();

			if (createAnother) {
				toasts.success(m.assetnew_created_toast({ name: created.name }));
				reset();
				return;
			}
			goto(resolve(assetRoute(created.type, created.providers[0], created.name) as '/'));
		} catch (err) {
			error = err instanceof Error ? err.message : m.assetnew_error_create();
		} finally {
			saving = false;
		}
	}
</script>

<svelte:head>
	<title>{m.assetnew_page_title()}</title>
</svelte:head>

<div class="mm-stack" style="--mm-gap: var(--space-6); max-width: 42rem">
	<mm-page-header heading={m.assetnew_heading()} description={m.assetnew_description()}
	></mm-page-header>

	<form
		bind:this={form}
		class="mm-stack"
		style="--mm-gap: var(--space-5)"
		novalidate
		onsubmit={handleSubmit}
	>
		<mm-text-field
			label={m.common_name()}
			name="name"
			value={name}
			required
			help={m.assetnew_name_help()}
			error={errors.name ?? ''}
			onmm-input={(e: CustomEvent<{ value: string }>) => (name = e.detail.value)}
			onfocusout={() => name && validate('name')}
		></mm-text-field>
		{#if duplicateHref}
			<mm-text variant="caption" tone="secondary" block class="pull-up">
				<mm-link href={resolve(duplicateHref as '/')}>{m.assetnew_open_existing()}</mm-link>
			</mm-text>
		{/if}

		<div class="pair">
			<mm-combobox
				label={m.common_type()}
				name="type"
				value={assetType}
				value-label={assetType}
				allow-custom
				required
				error={errors.type ?? ''}
				onmm-change={(e: CustomEvent<{ value: string }>) => {
					assetType = e.detail.value;
					if (errors.type) validate('type');
				}}
				onfocusout={() => assetType && validate('type')}
			>
				{#each typeOptions as t (t.key)}
					<mm-option value={t.label} label={t.label} icon={t.icon}></mm-option>
				{/each}
			</mm-combobox>
			<mm-multiselect
				label={m.assetnew_where_it_lives()}
				name="providers"
				value={providers}
				required
				error={errors.providers ?? ''}
				onmm-input={(e: CustomEvent<{ query: string }>) => (providerQuery = e.detail.query)}
				onmm-change={(e: CustomEvent<{ value: string[] }>) => {
					providers = e.detail.value;
					providerQuery = '';
					if (errors.providers) validate('providers');
				}}
			>
				{#each providerChoices as p (p.value)}
					<mm-option value={p.value} label={p.label} provider={p.provider}></mm-option>
				{/each}
			</mm-multiselect>
		</div>
		{#if pluginHint}
			<mm-text variant="caption" tone="secondary" block class="pull-up">
				{m.assetnew_plugin_hint({ name: pluginHint.name })}
				<mm-link href={`${resolve('/pipelines/new')}?plugin=${encodeURIComponent(pluginHint.id)}`}
					>{m.assetnew_plugin_connect({ name: pluginHint.name })}</mm-link
				>
			</mm-text>
		{/if}

		<mm-multiselect
			label={m.common_owners()}
			name="owners"
			value={owners}
			required={!!me}
			help={m.assetnew_owners_help()}
			onmm-input={(e: CustomEvent<{ query: string }>) => searchOwners(e.detail.query)}
			onmm-change={(e: CustomEvent<{ value: string[] }>) => (owners = e.detail.value)}
		>
			{#each ownerOptions as o (o.value)}
				<mm-option value={o.value} label={o.label} icon={o.icon}></mm-option>
			{/each}
		</mm-multiselect>

		<mm-textarea
			label={m.common_description()}
			name="description"
			optional
			rows={5}
			value={description}
			help={m.assetnew_description_help()}
			placeholder={m.assetnew_description_placeholder_short()}
			onmm-input={(e: CustomEvent<{ value: string }>) => (description = e.detail.value)}
		></mm-textarea>

		{#if open.tags}
			<mm-section heading={m.common_tags()}>
				<mm-multiselect
					label={m.common_tags()}
					hide-label
					name="tags"
					value={tags}
					onmm-input={(e: CustomEvent<{ query: string }>) => searchTags(e.detail.query)}
					onmm-change={(e: CustomEvent<{ value: string[] }>) => {
						tags = e.detail.value;
						tagQuery = '';
					}}
				>
					{#each tagChoices as t (t)}
						<mm-option value={t} label={t}></mm-option>
					{/each}
				</mm-multiselect>
			</mm-section>
		{/if}

		{#if open.terms}
			<mm-section heading={m.assetnew_terms_heading()}>
				<mm-multiselect
					label={m.assetnew_terms_heading()}
					hide-label
					name="terms"
					value={termIds}
					loader={loadTerms}
					onmm-change={(e: CustomEvent<{ value: string[] }>) => (termIds = e.detail.value)}
				></mm-multiselect>
			</mm-section>
		{/if}

		{#if open.metadata}
			<mm-section
				heading={m.assetnew_metadata_heading()}
				description={m.assetnew_metadata_description()}
			>
				<mm-key-value-editor
					label={m.assetnew_metadata_heading()}
					json
					key-placeholder={m.assetnew_metadata_key_placeholder()}
					value={metadata}
					onmm-change={(e: CustomEvent<{ value: KeyValuePair[] }>) => (metadata = e.detail.value)}
				></mm-key-value-editor>
			</mm-section>
		{/if}

		{#if open.links}
			<mm-section heading={m.assetnew_links_heading()}>
				<mm-key-value-editor
					label={m.assetnew_links_heading()}
					key-label={m.common_name()}
					value-label={m.assetnew_link_url()}
					add-label={m.assetnew_add_link()}
					value={links}
					onmm-change={(e: CustomEvent<{ value: KeyValuePair[] }>) => (links = e.detail.value)}
				></mm-key-value-editor>
			</mm-section>
		{/if}

		{#if !open.tags || !open.terms || !open.links || !open.metadata}
			<div class="mm-cluster more" role="group" aria-label={m.assetnew_add_more_label()}>
				{#if !open.tags}
					<mm-button variant="ghost" size="sm" icon="add" onclick={() => reveal('tags')}
						>{m.common_tags()}</mm-button
					>
				{/if}
				{#if !open.terms}
					<mm-button variant="ghost" size="sm" icon="add" onclick={() => reveal('terms')}
						>{m.assetnew_terms_heading()}</mm-button
					>
				{/if}
				{#if !open.links}
					<mm-button variant="ghost" size="sm" icon="add" onclick={() => reveal('links')}
						>{m.assetnew_links_heading()}</mm-button
					>
				{/if}
				{#if !open.metadata}
					<mm-button variant="ghost" size="sm" icon="add" onclick={() => reveal('metadata')}
						>{m.assetnew_metadata_heading()}</mm-button
					>
				{/if}
			</div>
		{/if}

		{#if error}
			<mm-callout tone="danger" heading={m.assetnew_error_create()}>{error}</mm-callout>
		{/if}

		<div class="mm-cluster mm-cluster--between">
			<mm-checkbox
				label={m.assetnew_create_another()}
				checked={createAnother}
				onmm-change={(e: CustomEvent<{ checked: boolean }>) => (createAnother = e.detail.checked)}
			></mm-checkbox>
			<div class="mm-cluster" style="--mm-gap: var(--space-2)">
				<mm-button href={resolve('/discover')}>{m.common_cancel()}</mm-button>
				<mm-button variant="primary" type="submit" loading={saving}
					>{m.assetnew_create_asset_button()}</mm-button
				>
			</div>
		</div>
	</form>
</div>

<style>
	.pair {
		display: grid;
		grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
		gap: var(--space-4);
		align-items: start;
	}
	@media (max-width: 640px) {
		.pair {
			grid-template-columns: minmax(0, 1fr);
		}
	}
	.pull-up {
		margin-block-start: calc(-1 * var(--space-3));
	}
	.more {
		--mm-gap: var(--space-1);
		border-block: 1px solid var(--mm-rule);
		padding-block: var(--space-2);
	}
</style>
