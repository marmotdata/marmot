<script lang="ts">
	import { tick } from 'svelte';
	import IconifyIcon from '@iconify/svelte';
	import { resolve } from '$app/paths';
	import Icon from '$components/ui/Icon.svelte';
	import MarkdownRenderer from '$components/ui/MarkdownRenderer.svelte';
	import RichTextEditor from '$components/editor/RichTextEditor.svelte';
	import OwnerSelector from '$components/shared/OwnerSelector.svelte';
	import Tags from '$components/shared/Tags.svelte';
	import MetadataView from '$components/shared/MetadataView.svelte';
	import LinkAssetsModal from './LinkAssetsModal.svelte';
	import { auth } from '$lib/stores/auth';
	import { fetchApi } from '$lib/api';
	import { m } from '$lib/paraglide/messages';
	import { formatDate } from '$lib/utils';
	import type { GlossaryTerm, UpdateTermInput } from '$lib/glossary/types';

	interface LinkedAsset {
		id: string;
		name: string;
		type: string;
		mrn?: string;
		providers?: string[];
		description?: string;
	}

	let {
		term,
		canManage,
		onUpdated,
		onDelete
	}: {
		term: GlossaryTerm;
		canManage: boolean;
		onUpdated: (term: GlossaryTerm) => void;
		onDelete: () => void;
	} = $props();

	// Each part of the term has its own Edit button with Save and Cancel; the name and definition edit together because they are the term's heading
	type Section = 'heading' | 'description';
	let editing = $state<Section | null>(null);
	let draft = $state('');
	let draftName = $state('');
	let draftDefinition = $state('');
	let saving = $state(false);
	let error = $state('');
	let menuOpen = $state(false);
	let showMetadata = $state(false);
	let input = $state<HTMLInputElement | HTMLTextAreaElement | null>(null);

	let linked = $state<LinkedAsset[]>([]);
	let linkedTotal = $state(0);
	let linkedLoading = $state(true);
	let linking = $state(false);

	// Linking a term to an asset changes the asset, so it needs asset rights rather than glossary rights
	const canLink = auth.hasPermission('assets', 'manage');

	let hasMetadata = $derived(Object.keys(term.metadata ?? {}).length > 0);

	$effect(() => {
		const id = term.id;
		editing = null;
		error = '';
		showMetadata = false;
		loadLinked(id);
	});

	async function loadLinked(id: string) {
		linkedLoading = true;
		try {
			const res = await fetchApi(`/assets/by-glossary-term/${encodeURIComponent(id)}?limit=12`);
			if (res.ok) {
				const data = await res.json();
				linked = data.assets ?? [];
				linkedTotal = data.total ?? linked.length;
			} else {
				linked = [];
				linkedTotal = 0;
			}
		} catch {
			linked = [];
			linkedTotal = 0;
		} finally {
			linkedLoading = false;
		}
	}

	async function save(update: UpdateTermInput) {
		saving = true;
		error = '';
		try {
			const res = await fetchApi(`/glossary/${term.id}`, {
				method: 'PUT',
				body: JSON.stringify(update)
			});
			if (!res.ok) {
				const data = await res.json().catch(() => ({}));
				throw new Error(data.error || m.glossary_update_error());
			}
			const updated: GlossaryTerm = await res.json();
			updated.tags = updated.tags ?? [];
			updated.metadata = updated.metadata ?? {};
			onUpdated(updated);
			return true;
		} catch (e) {
			error = e instanceof Error ? e.message : m.glossary_update_error();
			return false;
		} finally {
			saving = false;
		}
	}

	async function startEdit(section: Section) {
		if (!canManage) return;
		error = '';
		editing = section;
		if (section === 'heading') {
			draftName = term.name;
			draftDefinition = term.definition;
		} else {
			draft = term.description ?? '';
		}
		await tick();
		input?.focus();
	}

	function cancelEdit() {
		editing = null;
		error = '';
	}

	async function saveHeading() {
		const name = draftName.trim();
		const definition = draftDefinition.trim();
		if (!name || !definition) {
			error = m.glossary_error_name_definition_required();
			return;
		}
		const update: UpdateTermInput = {};
		if (name !== term.name) update.name = name;
		if (definition !== term.definition) update.definition = definition;
		if (Object.keys(update).length === 0 || (await save(update))) editing = null;
	}

	async function saveDescription() {
		if (draft === (term.description ?? '') || (await save({ description: draft || null }))) {
			editing = null;
		}
	}

	function onHeadingKeydown(e: KeyboardEvent) {
		if (e.key === 'Escape') {
			e.preventDefault();
			cancelEdit();
		} else if (
			e.key === 'Enter' &&
			(e.metaKey || e.ctrlKey || e.currentTarget instanceof HTMLInputElement)
		) {
			e.preventDefault();
			saveHeading();
		}
	}

	async function unlink(asset: LinkedAsset) {
		try {
			const res = await fetchApi(`/assets/terms/${encodeURIComponent(asset.id)}`, {
				method: 'DELETE',
				body: JSON.stringify({ term_id: term.id })
			});
			if (!res.ok) throw new Error(m.glossary_unlink_error());
			loadLinked(term.id);
		} catch (e) {
			error = e instanceof Error ? e.message : m.glossary_unlink_error();
		}
	}

	let discoverQuery = $derived(
		`q=${encodeURIComponent(`@term: "${term.name.replaceAll('"', '\\"')}"`)}`
	);

	function saveOwners(owners: { id: string; type: 'user' | 'team' }[]) {
		save({ owners: owners.map((o) => ({ id: o.id, type: o.type })) });
	}

	function assetParams(a: LinkedAsset) {
		const parts = (a.mrn ?? '').replace('mrn://', '').split('/');
		if (parts.length >= 3) {
			return { type: parts[0], service: parts[1], name: parts.slice(2).join('/') };
		}
		return { type: a.type, service: a.providers?.[0] ?? '', name: a.name };
	}
</script>

<svelte:window onclick={() => (menuOpen = false)} />

{#snippet propertyLabel(text: string)}
	<h3
		class="mb-2 text-[11px] font-semibold uppercase tracking-wider text-gray-500 dark:text-gray-400"
	>
		{text}
	</h3>
{/snippet}

<article class="mx-auto grid max-w-6xl gap-10 lg:grid-cols-[minmax(0,1fr)_17rem]">
	<div class="min-w-0">
		<!-- What this page is, the term and its definition -->
		<header>
			<div class="flex items-start justify-between gap-4">
				{#if editing === 'heading'}
					<input
						bind:this={input}
						bind:value={draftName}
						onkeydown={onHeadingKeydown}
						disabled={saving}
						aria-label={m.common_name()}
						class="w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-gray-900 focus:border-earthy-terracotta-500 focus:outline-none focus:ring-2 focus:ring-earthy-terracotta-500/30 dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100 text-2xl font-semibold tracking-tight"
					/>
				{:else}
					<h1 class="text-3xl font-semibold tracking-tight text-gray-900 dark:text-gray-100">
						{term.name}
					</h1>
				{/if}
				{#if canManage}
					<div class="relative -my-1 flex-shrink-0">
						<button
							type="button"
							onclick={(e) => {
								e.stopPropagation();
								menuOpen = !menuOpen;
							}}
							aria-label={m.glossary_more_actions()}
							aria-expanded={menuOpen}
							class="rounded-lg p-2 text-gray-500 transition hover:bg-gray-100 hover:text-gray-800 dark:hover:bg-gray-800 dark:hover:text-gray-200"
						>
							<IconifyIcon icon="material-symbols:more-horiz" class="h-5 w-5" />
						</button>
						{#if menuOpen}
							<div
								class="absolute right-0 z-20 mt-1 w-44 overflow-hidden rounded-lg border border-gray-200 bg-white py-1 shadow-lg dark:border-gray-700 dark:bg-gray-800"
								role="menu"
							>
								<button
									type="button"
									role="menuitem"
									onclick={onDelete}
									class="flex w-full items-center gap-2 px-3 py-2 text-left text-sm text-red-600 hover:bg-red-50 dark:text-red-400 dark:hover:bg-red-950/30"
								>
									<IconifyIcon icon="material-symbols:delete-outline" class="h-4 w-4" />
									{m.common_delete()}
								</button>
							</div>
						{/if}
					</div>
				{/if}
			</div>

			<div class="mt-5 border-l-2 border-earthy-terracotta-400 pl-4">
				<div class="mb-1 flex items-center justify-between">
					<p
						class="text-[11px] font-semibold uppercase tracking-wider text-gray-500 dark:text-gray-400"
					>
						{m.glossary_definition_label()}
					</p>
					{#if canManage && editing !== 'heading'}
						<button
							type="button"
							onclick={() => startEdit('heading')}
							class="inline-flex items-center gap-1 rounded-md px-2 py-1 text-xs font-medium text-gray-600 hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-gray-800"
						>
							<IconifyIcon icon="material-symbols:edit-outline" class="h-3.5 w-3.5" />
							{m.common_edit()}
						</button>
					{/if}
				</div>
				{#if editing === 'heading'}
					<textarea
						bind:value={draftDefinition}
						onkeydown={onHeadingKeydown}
						disabled={saving}
						rows="3"
						aria-label={m.glossary_definition_label()}
						class="w-full resize-none rounded-lg border border-gray-300 bg-white px-3 py-2 text-gray-900 focus:border-earthy-terracotta-500 focus:outline-none focus:ring-2 focus:ring-earthy-terracotta-500/30 dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100 text-base"
					></textarea>
					<div class="mt-3 flex justify-end gap-2">
						<button
							type="button"
							onclick={cancelEdit}
							class="rounded-lg px-3 py-1.5 text-sm font-medium text-gray-700 hover:bg-gray-100 dark:text-gray-200 dark:hover:bg-gray-800"
							>{m.common_cancel()}</button
						>
						<button
							type="button"
							onclick={saveHeading}
							disabled={saving}
							class="rounded-lg bg-earthy-terracotta-600 px-3 py-1.5 text-sm font-semibold text-white hover:bg-earthy-terracotta-500 disabled:opacity-50"
							>{m.common_save()}</button
						>
					</div>
				{:else}
					<p class="max-w-3xl text-lg leading-relaxed text-gray-800 dark:text-gray-200">
						{term.definition}
					</p>
				{/if}
			</div>
			{#if error}
				<p class="mt-3 text-sm text-red-600 dark:text-red-400">{error}</p>
			{/if}
			{#if saving}
				<p class="mt-3 text-xs text-gray-400">{m.glossary_saving()}</p>
			{/if}
		</header>

		<div class="mt-10 space-y-10">
			<!-- Description -->
			<section>
				<div class="mb-2 flex items-center justify-between">
					<h2
						class="text-[11px] font-semibold uppercase tracking-wider text-gray-500 dark:text-gray-400"
					>
						{m.common_description()}
					</h2>
					{#if canManage && editing !== 'description' && term.description}
						<button
							type="button"
							onclick={() => startEdit('description')}
							class="inline-flex items-center gap-1 rounded-md px-2 py-1 text-xs font-medium text-gray-600 hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-gray-800"
						>
							<IconifyIcon icon="material-symbols:edit-outline" class="h-3.5 w-3.5" />
							{m.common_edit()}
						</button>
					{/if}
				</div>
				{#if editing === 'description'}
					<RichTextEditor bind:value={draft} disabled={saving} />
					<div class="mt-3 flex justify-end gap-2">
						<button
							type="button"
							onclick={cancelEdit}
							class="rounded-lg px-3 py-1.5 text-sm font-medium text-gray-700 hover:bg-gray-100 dark:text-gray-200 dark:hover:bg-gray-800"
							>{m.common_cancel()}</button
						>
						<button
							type="button"
							onclick={saveDescription}
							disabled={saving}
							class="rounded-lg bg-earthy-terracotta-600 px-3 py-1.5 text-sm font-semibold text-white hover:bg-earthy-terracotta-500 disabled:opacity-50"
							>{m.common_save()}</button
						>
					</div>
				{:else if term.description}
					<div class="prose prose-sm max-w-none dark:prose-invert">
						<MarkdownRenderer content={term.description} />
					</div>
				{:else if canManage}
					<button
						type="button"
						onclick={() => startEdit('description')}
						class="w-full rounded-xl border border-dashed border-gray-300 px-4 py-6 text-left text-sm text-gray-500 transition hover:border-earthy-terracotta-300 hover:text-gray-700 dark:border-gray-600 dark:text-gray-400 dark:hover:text-gray-200"
					>
						<IconifyIcon
							icon="material-symbols:add"
							class="mr-1 inline h-4 w-4 align-text-bottom"
						/>
						{m.glossary_description_prompt()}
					</button>
				{:else}
					<p class="text-sm text-gray-500 dark:text-gray-400">{m.glossary_no_description()}</p>
				{/if}
			</section>

			<!-- Assets that use this term -->
			<section>
				<div class="mb-2 flex items-center justify-between">
					<h2
						class="text-[11px] font-semibold uppercase tracking-wider text-gray-500 dark:text-gray-400"
					>
						{m.glossary_linked_heading()}
						{#if linkedTotal > 0}
							<span
								class="ml-1 rounded-full bg-gray-100 px-1.5 text-xs font-medium tabular-nums text-gray-600 dark:bg-gray-700 dark:text-gray-300"
								>{linkedTotal}</span
							>
						{/if}
					</h2>
					{#if canLink && linked.length > 0}
						<button
							type="button"
							onclick={() => (linking = true)}
							class="inline-flex items-center gap-1 rounded-md px-2 py-1 text-xs font-medium text-gray-600 hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-gray-800"
						>
							<IconifyIcon icon="material-symbols:add-link" class="h-3.5 w-3.5" />
							{m.glossary_link_title()}
						</button>
					{/if}
				</div>
				{#if linkedLoading}
					<div class="space-y-2">
						{#each Array(3) as _, i (i)}
							<div class="h-12 animate-pulse rounded-lg bg-gray-100 dark:bg-gray-800"></div>
						{/each}
					</div>
				{:else if linked.length === 0}
					<div
						class="rounded-xl border border-dashed border-gray-300 px-5 py-6 text-center dark:border-gray-600"
					>
						<IconifyIcon
							icon="material-symbols:link"
							class="mx-auto h-6 w-6 text-gray-400 dark:text-gray-500"
						/>
						<p class="mt-2 text-sm font-medium text-gray-900 dark:text-gray-100">
							{m.glossary_linked_empty_title()}
						</p>
						<p class="mx-auto mt-1 max-w-sm text-sm text-gray-500 dark:text-gray-400">
							{m.glossary_linked_empty()}
						</p>
						{#if canLink}
							<div class="mt-4 flex flex-wrap justify-center gap-2">
								<button
									type="button"
									onclick={() => (linking = true)}
									class="inline-flex items-center gap-1.5 rounded-lg bg-earthy-terracotta-600 px-3 py-1.5 text-sm font-semibold text-white transition hover:bg-earthy-terracotta-500"
								>
									<IconifyIcon icon="material-symbols:add-link" class="h-4 w-4" />
									{m.glossary_link_title()}
								</button>
								<a
									href={resolve('/asset-rules/new')}
									class="inline-flex items-center gap-1.5 rounded-lg border border-gray-200 bg-white px-3 py-1.5 text-sm font-medium text-gray-700 hover:bg-gray-50 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-200 dark:hover:bg-gray-700"
								>
									<IconifyIcon icon="material-symbols:rule-settings" class="h-4 w-4" />
									{m.glossary_linked_rule()}
								</a>
							</div>
						{/if}
					</div>
				{:else}
					<ul
						class="divide-y divide-gray-100 overflow-hidden rounded-xl border border-gray-200 bg-white dark:divide-gray-700/60 dark:border-gray-700 dark:bg-gray-800"
					>
						{#each linked as a (a.id)}
							<li class="group flex items-center">
								<a
									href={resolve('/discover/[type]/[service]/[name]', assetParams(a))}
									class="flex min-w-0 flex-1 items-center gap-3 px-4 py-2.5 transition hover:bg-gray-50 dark:hover:bg-gray-700/30"
								>
									<Icon name={a.providers?.[0] ?? a.type} size="xs" showLabel={false} />
									<span class="min-w-0 flex-1">
										<span
											class="block truncate text-sm font-medium text-gray-900 group-hover:text-earthy-terracotta-700 dark:text-gray-100 dark:group-hover:text-earthy-terracotta-400"
											>{a.name}</span
										>
										{#if a.description}
											<span class="block truncate text-xs text-gray-500 dark:text-gray-400"
												>{a.description}</span
											>
										{/if}
									</span>
									<span class="text-xs text-gray-400">{a.type}</span>
								</a>
								{#if canLink}
									<button
										type="button"
										onclick={() => unlink(a)}
										class="mr-2 rounded-md p-1.5 text-gray-400 opacity-0 transition hover:bg-gray-100 hover:text-red-600 focus:opacity-100 group-hover:opacity-100 dark:hover:bg-gray-700"
										title={m.glossary_unlink()}
										aria-label={m.glossary_unlink()}
									>
										<IconifyIcon icon="material-symbols:link-off" class="h-4 w-4" />
									</button>
								{/if}
							</li>
						{/each}
						<li>
							<a
								href={resolve(`/discover?${discoverQuery}`)}
								class="flex items-center justify-center gap-1 bg-gray-50 px-4 py-2.5 text-sm font-medium text-earthy-terracotta-700 transition hover:bg-gray-100 dark:bg-gray-800/60 dark:text-earthy-terracotta-400 dark:hover:bg-gray-700/50"
							>
								{linkedTotal > linked.length
									? m.glossary_linked_view_all({ count: linkedTotal })
									: m.glossary_linked_view_discover()}
								<IconifyIcon icon="material-symbols:arrow-forward" class="h-4 w-4" />
							</a>
						</li>
					</ul>
					{#if canLink}
						<div
							class="mt-4 flex flex-col gap-3 rounded-xl border border-gray-200 bg-gray-50 px-4 py-3 sm:flex-row sm:items-center dark:border-gray-700 dark:bg-gray-800/60"
						>
							<span
								class="flex h-8 w-8 flex-shrink-0 items-center justify-center rounded-lg bg-white text-gray-500 shadow-sm dark:bg-gray-700 dark:text-gray-300"
							>
								<IconifyIcon icon="material-symbols:rule-settings" class="h-4 w-4" />
							</span>
							<span class="min-w-0 flex-1">
								<span class="block text-sm font-medium text-gray-900 dark:text-gray-100"
									>{m.glossary_linked_rule_nudge_title()}</span
								>
								<span class="block text-xs text-gray-500 dark:text-gray-400"
									>{m.glossary_linked_rule_nudge()}</span
								>
							</span>
							<a
								href={resolve('/asset-rules/new')}
								class="inline-flex flex-shrink-0 items-center gap-1.5 self-start rounded-lg border border-gray-200 bg-white px-3 py-1.5 text-sm font-medium text-gray-700 hover:bg-gray-50 sm:self-auto dark:border-gray-600 dark:bg-gray-800 dark:text-gray-200 dark:hover:bg-gray-700"
							>
								{m.glossary_linked_rule_nudge_link()}
							</a>
						</div>
					{/if}
				{/if}
			</section>
		</div>
	</div>

	<!-- Properties sit beside the content without a card, divided by a rule -->
	<aside
		class="h-fit space-y-6 border-t border-gray-200 pt-6 lg:sticky lg:top-6 lg:border-l lg:border-t-0 lg:pl-8 lg:pt-1 dark:border-gray-700"
	>
		<div>
			{@render propertyLabel(m.common_owners())}
			<OwnerSelector selectedOwners={term.owners} onChange={saveOwners} disabled={!canManage} />
		</div>
		<div>
			{@render propertyLabel(m.common_tags())}
			<Tags tags={term.tags ?? []} endpoint="/glossary" id={term.id} canEdit={canManage} />
		</div>
		<div>
			{@render propertyLabel(m.glossary_metadata_heading())}
			{#if hasMetadata || showMetadata}
				<MetadataView metadata={term.metadata} endpoint="/glossary" id={term.id} maxDepth={2} />
			{:else if canManage}
				<button
					type="button"
					onclick={() => (showMetadata = true)}
					class="inline-flex items-center gap-1 text-sm font-medium text-earthy-terracotta-700 hover:underline dark:text-earthy-terracotta-400"
				>
					<IconifyIcon icon="material-symbols:add" class="h-4 w-4" />
					{m.glossary_add_field()}
				</button>
			{:else}
				<p class="text-sm text-gray-400">{m.glossary_none()}</p>
			{/if}
		</div>
		<dl class="grid grid-cols-2 gap-3 border-t border-gray-100 pt-4 text-xs dark:border-gray-700">
			<div>
				<dt class="text-gray-500 dark:text-gray-400">{m.glossary_created_label()}</dt>
				<dd class="mt-0.5 text-gray-900 dark:text-gray-100">{formatDate(term.created_at)}</dd>
			</div>
			<div>
				<dt class="text-gray-500 dark:text-gray-400">{m.glossary_last_updated_label()}</dt>
				<dd class="mt-0.5 text-gray-900 dark:text-gray-100">{formatDate(term.updated_at)}</dd>
			</div>
		</dl>
	</aside>
</article>

{#if linking}
	<LinkAssetsModal
		termId={term.id}
		termName={term.name}
		linkedIds={linked.map((a) => a.id)}
		onClose={() => (linking = false)}
		onLinked={() => loadLinked(term.id)}
	/>
{/if}
