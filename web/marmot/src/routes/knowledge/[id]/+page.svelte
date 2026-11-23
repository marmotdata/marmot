<script lang="ts">
	import '@marmotdata/elements/link-list.js';
	import '@marmotdata/elements/dialog.js';
	import type { MmConfirmDialog } from '@marmotdata/elements/confirm-dialog.js';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { fetchApi } from '#lib/api.ts';
	import { auth } from '#lib/stores/auth.ts';
	import { toasts } from '#lib/stores/toast.ts';
	import { setCrumbs } from '#lib/mm/index.ts';
	import { m } from '#lib/paraglide/messages.js';
	import { formatDate, formatRelativeTime } from '#lib/utils/format.ts';
	import { discoverPath } from '#lib/mrn.ts';
	import {
		kindById,
		knowledgeRelations,
		loadKinds,
		isRequired,
		kindOfType,
		knowledgeKinds,
		relationOf,
		statusOf,
		type KnowledgeField,
		type KnowledgeKind
	} from '#lib/knowledge/kinds.ts';
	import { typeLabel } from '#components/query/catalogSearch.ts';
	import { ruleCoverage, ruleDiscoverHref, ruleLabel } from '#lib/knowledge/rules.ts';
	import {
		allEntryLinks,
		getEntry,
		markReviewed,
		monthsFromToday,
		saveEntry,
		type KnowledgeAssetRef,
		type KnowledgeEntry,
		type KnowledgeLink
	} from '#lib/knowledge/api.ts';
	import {
		draftErrors,
		draftFrom,
		hasValue,
		toInput,
		type EntryDraft
	} from '#lib/knowledge/draft.ts';
	import type { Asset } from '#lib/assets/types.ts';
	import AssetOwnerPicker, { type AssetOwner } from '#components/asset/AssetOwnerPicker.svelte';
	import DocumentationSystem from '#components/docs/DocumentationSystem.svelte';
	import EntryDetailsForm from '#components/knowledge/EntryDetailsForm.svelte';
	import FieldValue from '#components/knowledge/FieldValue.svelte';
	import LinksEditor from '#components/knowledge/LinksEditor.svelte';
	import AssetMark from '#components/asset/AssetMark.svelte';
	import RulesEditor from '#components/knowledge/RulesEditor.svelte';

	const canManage = auth.hasPermission('assets', 'manage');

	let id = $derived(page.params.id ?? '');
	let asset = $state<Asset | null>(null);
	let entry = $state<KnowledgeEntry | null>(null);
	let kind = $state<KnowledgeKind | undefined>();
	let owners = $state<AssetOwner[]>([]);
	let loading = $state(true);
	let failed = $state('');

	let editing = $state(false);
	let draft = $state<EntryDraft | null>(null);
	let summaryDraft = $state('');
	let fieldErrors = $state<Record<string, string>>({});
	let saving = $state(false);
	let saveError = $state('');
	let reviewing = $state(false);
	let deleteDialog = $state<MmConfirmDialog>();
	let deleting = $state(false);

	$effect(() => {
		const current = id;
		if (current) load(current);
	});

	$effect(() => {
		setCrumbs([
			{ label: m.knowledge_heading(), href: resolve('/knowledge') },
			...(kind
				? [{ label: kind.label, href: `${resolve('/knowledge/browse')}?kind=${kind.id}` }]
				: []),
			{ label: asset?.name ?? m.knowledge_entry_loading() }
		]);
	});

	async function load(assetId: string) {
		loading = true;
		failed = '';
		editing = false;
		try {
			const [kinds, assetRes, e] = await Promise.all([
				loadKinds(),
				fetchApi(`/assets/${encodeURIComponent(assetId)}`),
				getEntry(assetId)
			]);
			if (!assetRes.ok)
				throw Object.assign(new Error(m.knowledge_entry_not_found()), { status: assetRes.status });
			asset = await assetRes.json();
			entry = e;
			kind = kindById(e.kind, kinds);
			fetchOwners(assetId);
		} catch (err) {
			const status = (err as { status?: number }).status;
			failed =
				status === 404
					? m.knowledge_entry_not_found()
					: err instanceof Error
						? err.message
						: m.knowledge_entry_load_error();
		} finally {
			loading = false;
		}
	}

	async function fetchOwners(assetId: string) {
		try {
			const res = await fetchApi(`/assets/owners/?asset_id=${encodeURIComponent(assetId)}`);
			owners = res.ok ? ((await res.json()).owners ?? []) : [];
		} catch {
			owners = [];
		}
	}

	async function addOwner(owner: AssetOwner) {
		const res = await fetchApi(`/assets/owners/?asset_id=${id}`, {
			method: 'POST',
			body: JSON.stringify({ owner_type: owner.type, owner_id: owner.id })
		});
		if (res.ok) owners = [...owners, owner];
		else toasts.error(m.asset_page_owner_add_error());
	}

	async function removeOwner(owner: AssetOwner) {
		const res = await fetchApi(
			`/assets/owners/?asset_id=${id}&owner_type=${owner.type}&owner_id=${owner.id}`,
			{
				method: 'DELETE'
			}
		);
		if (res.ok) owners = owners.filter((o) => !(o.id === owner.id && o.type === owner.type));
		else toasts.error(m.asset_page_owner_remove_error());
	}

	async function startEditing() {
		if (!kind || !entry) return;
		const d = draftFrom(kind, entry);
		// The entry carries only its first links, so the rest are read before editing; past a thousand, links are left alone and rules are the way to change coverage.
		if (d.links === null && asset) {
			try {
				d.links = await allEntryLinks(asset.id);
			} catch {
				d.links = null;
			}
		}
		draft = d;
		summaryDraft = asset?.user_description ?? '';
		fieldErrors = {};
		saveError = '';
		editing = true;
	}

	async function save() {
		if (!kind || !draft || !asset) return;
		fieldErrors = draftErrors(kind, draft);
		if (Object.keys(fieldErrors).length) return;
		saving = true;
		saveError = '';
		try {
			const summary = summaryDraft.trim();
			if (summary !== (asset.user_description ?? '')) {
				const res = await fetchApi(`/assets/${asset.id}`, {
					method: 'PUT',
					body: JSON.stringify({ user_description: summary })
				});
				if (!res.ok) throw new Error(m.asset_page_notes_save_error());
				asset.user_description = summary || undefined;
			}
			entry = await saveEntry(asset.id, toInput(draft));
			editing = false;
			toasts.success(m.knowledge_entry_saved());
		} catch (err) {
			saveError = err instanceof Error ? err.message : m.knowledge_entry_save_error();
		} finally {
			saving = false;
		}
	}

	async function review(months: number | null) {
		if (!asset) return;
		reviewing = true;
		try {
			entry = await markReviewed(asset.id, months ? monthsFromToday(months) : undefined);
			toasts.success(m.knowledge_entry_reviewed());
		} catch (err) {
			toasts.error(err instanceof Error ? err.message : m.knowledge_entry_save_error());
		} finally {
			reviewing = false;
		}
	}

	function onAction(e: CustomEvent<{ value: string }>) {
		const v = e.detail.value;
		if (v === 'delete') deleteDialog?.show();
		if (v === 'catalog' && catalogHref) goto(resolve(catalogHref as '/'));
		if (v.startsWith('review-')) review(Number(v.slice(7)) || null);
	}

	async function onDeleteConfirm(e: CustomEvent) {
		e.preventDefault();
		if (!asset) return;
		deleting = true;
		try {
			const res = await fetchApi(`/assets/${asset.id}`, { method: 'DELETE' });
			if (!res.ok) throw new Error(m.asset_delete_error());
			deleteDialog?.close('confirm');
			toasts.success(m.knowledge_entry_deleted({ name: asset.name }));
			goto(resolve('/knowledge'));
		} catch (err) {
			toasts.error(err instanceof Error ? err.message : m.asset_delete_error());
		} finally {
			deleting = false;
		}
	}

	// A linked asset's page: its knowledge page for an entry, its catalog page otherwise. Callers resolve it.
	function refPath(r: KnowledgeAssetRef): string {
		if (kindOfType(r.type, $knowledgeKinds)) return `/knowledge/${encodeURIComponent(r.id)}`;
		return discoverPath(r.mrn) ?? '/discover';
	}

	let status = $derived(entry && kind ? statusOf(kind, entry.status) : undefined);
	let catalogHref = $derived.by(() => {
		const path = discoverPath(asset?.mrn);
		return path ? `${path}?view=asset` : '';
	});
	// The side column shows a handful of links; an entry linked to thousands sends people to its links page instead of growing the column.
	const RAIL_LINKS = 8;
	let shownLinks = $derived((entry?.links ?? []).slice(0, RAIL_LINKS));
	let linkGroups = $derived.by(() => {
		const groups: { relation: string; label: string; links: KnowledgeLink[] }[] = [];
		for (const l of shownLinks) {
			let g = groups.find((x) => x.relation === l.relation);
			if (!g) {
				g = {
					relation: l.relation,
					label: relationOf(l.relation, $knowledgeRelations)?.label ?? l.relation,
					links: []
				};
				groups.push(g);
			}
			g.links.push(l);
		}
		return groups;
	});
	let moreLinks = $derived(entry ? entry.link_count > shownLinks.length : false);

	const SHORT_TYPES = ['text', 'date', 'select', 'url', 'list'];

	// On the entry's own page the status is the subject, so it gets its colour even when all is well: moss in use, slate not yet, neutral no longer.
	const stateTone = { current: 'success', pending: 'info', historical: 'neutral' } as const;

	interface Fact {
		key: string;
		label: string;
		field?: KnowledgeField;
		text?: string;
	}

	// Short facts sit in one row under the header: status, then the kind's short fields that are set (and required ones that are not), then dates. Long text and tables get their own blocks.
	let facts = $derived.by((): Fact[] => {
		if (!kind || !entry) return [];
		const out: Fact[] = [
			{ key: 'status', label: m.knowledge_status_label(), text: status?.label ?? '' }
		];
		for (const f of kind.fields) {
			if (!SHORT_TYPES.includes(f.type)) continue;
			if (hasValue(entry.fields[f.key])) out.push({ key: f.key, label: f.label, field: f });
			else if (isRequired(f, entry.status)) out.push({ key: f.key, label: f.label });
		}
		if (entry.effective_from)
			out.push({
				key: 'effective',
				label: m.knowledge_effective_from_label(),
				text: formatDate(entry.effective_from)
			});
		if (entry.review_by)
			out.push({
				key: 'review',
				label: m.knowledge_review_by_label(),
				text: formatDate(entry.review_by)
			});
		return out;
	});
	let wide = $derived(
		kind && entry
			? kind.fields.filter((f) => !SHORT_TYPES.includes(f.type) && hasValue(entry!.fields[f.key]))
			: []
	);

	let linksOpen = $state(false);
	let openingLinks = $state(false);
	let savingLinks = $state(false);
	let linksError = $state('');
	let linkDraft = $state<EntryDraft | null>(null);

	// Links and rules are edited on their own, without opening every other field, since adding a table to a policy is the most common change.
	async function openLinks() {
		if (!kind || !entry || !asset) return;
		openingLinks = true;
		try {
			const d = draftFrom(kind, entry);
			if (d.links === null) d.links = await allEntryLinks(asset.id).catch(() => null);
			linkDraft = d;
			linksError = '';
			linksOpen = true;
		} finally {
			openingLinks = false;
		}
	}

	async function saveLinks() {
		if (!asset || !linkDraft) return;
		savingLinks = true;
		linksError = '';
		try {
			const input = toInput(linkDraft);
			entry = await saveEntry(asset.id, input);
			linksOpen = false;
			toasts.success(m.knowledge_entry_saved());
		} catch (err) {
			linksError = err instanceof Error ? err.message : m.knowledge_entry_save_error();
		} finally {
			savingLinks = false;
		}
	}
</script>

<svelte:head>
	<title
		>{asset ? m.knowledge_entry_page_title({ name: asset.name }) : m.knowledge_page_title()}</title
	>
</svelte:head>

{#if loading && !asset}
	<mm-skeleton loading count={8}></mm-skeleton>
{:else if failed || !asset || !entry || !kind}
	<div class="mm-stack" style="--mm-gap: var(--space-5)">
		<mm-page-header heading={m.knowledge_heading()}></mm-page-header>
		<mm-callout tone="danger" heading={failed || m.knowledge_entry_load_error()}>
			<mm-button slot="actions" size="sm" href={resolve('/knowledge')}
				>{m.knowledge_entry_back()}</mm-button
			>
		</mm-callout>
	</div>
{:else}
	<div class="mm-stack" style="--mm-gap: var(--space-5)">
		<mm-page-header
			heading={asset.name}
			icon={kind.icon}
			caption={kind.label}
			description={editing ? undefined : asset.user_description}
		>
			{#if editing}
				<div slot="description">
					<mm-textarea
						label={m.knowledge_new_summary_label()}
						help={m.knowledge_new_summary_help()}
						rows={2}
						autosize
						value={summaryDraft}
						onmm-input={(e: CustomEvent<{ value: string }>) => (summaryDraft = e.detail.value)}
					></mm-textarea>
				</div>
			{/if}
			{#each owners as owner (owner.type + owner.id)}
				<mm-owner-chip
					slot="owners"
					name={owner.name}
					kind={owner.type === 'team' ? 'team' : 'person'}
					src={owner.profile_picture}
					email={owner.email}
					removable={canManage}
					onmm-remove={() => removeOwner(owner)}
				></mm-owner-chip>
			{/each}
			{#if canManage}
				<span slot="owners" style="display: contents"
					><AssetOwnerPicker {owners} onadd={addOwner} /></span
				>
			{/if}
			<div slot="actions" class="mm-cluster" style="--mm-gap: var(--space-2)">
				{#if canManage && !editing}
					<mm-button variant="secondary" icon="edit" onclick={startEditing}
						>{m.knowledge_entry_edit()}</mm-button
					>
				{/if}
				<mm-menu-button
					variant="secondary"
					icon="more-horiz"
					icon-only
					label={m.knowledge_entry_more()}
				>
					<mm-menu label={m.knowledge_entry_more()} onmm-select={onAction}>
						{#if canManage && entry.status}
							<mm-menu-group label={m.knowledge_entry_mark_reviewed()}>
								<mm-menu-item value="review-3" icon="task-alt"
									>{m.knowledge_review_again_3_months()}</mm-menu-item
								>
								<mm-menu-item value="review-6" icon="task-alt"
									>{m.knowledge_review_again_6_months()}</mm-menu-item
								>
								<mm-menu-item value="review-12" icon="task-alt"
									>{m.knowledge_review_again_a_year()}</mm-menu-item
								>
							</mm-menu-group>
							<mm-menu-separator></mm-menu-separator>
						{/if}
						{#if catalogHref}
							<mm-menu-item value="catalog" icon="travel-explore"
								>{m.knowledge_entry_open_catalog()}</mm-menu-item
							>
						{/if}
						{#if canManage}
							<mm-menu-separator></mm-menu-separator>
							<mm-menu-item value="delete" icon="delete" danger
								>{m.knowledge_entry_delete()}</mm-menu-item
							>
						{/if}
					</mm-menu>
				</mm-menu-button>
			</div>
		</mm-page-header>

		{#if !editing}
			{#if entry.superseded_by.length && status?.state === 'historical'}
				<mm-callout
					tone="warning"
					icon="swap-horiz"
					heading={m.knowledge_callout_replaced({ kind: kind.label.toLowerCase() })}
				>
					{#each entry.superseded_by as r, i (r.id)}{#if i > 0},
						{/if}<mm-link href={resolve('/knowledge/[id]', { id: r.id })}>{r.name}</mm-link>{/each}
				</mm-callout>
			{:else if entry.state === 'historical'}
				<mm-callout
					tone="warning"
					icon="archive"
					heading={m.knowledge_callout_historical({
						kind: kind.label.toLowerCase(),
						status: (status?.label ?? '').toLowerCase()
					})}
				>
					{m.knowledge_callout_historical_body()}
				</mm-callout>
			{:else if entry.state === 'pending'}
				<mm-callout
					tone="info"
					icon="pending"
					heading={entry.effective_from && status?.state === 'current'
						? m.knowledge_callout_upcoming({ date: formatDate(entry.effective_from) })
						: m.knowledge_callout_pending({
								kind: kind.label.toLowerCase(),
								status: (status?.label ?? '').toLowerCase()
							})}
				>
					{m.knowledge_callout_pending_body()}
				</mm-callout>
			{:else if !entry.status}
				<mm-callout tone="info" heading={m.knowledge_callout_unset()}>
					{m.knowledge_callout_unset_body()}
					{#if canManage}
						<mm-button slot="actions" size="sm" onclick={startEditing}
							>{m.knowledge_entry_edit()}</mm-button
						>
					{/if}
				</mm-callout>
			{/if}
			{#if entry.review_due && entry.state !== 'historical'}
				<mm-callout
					tone="warning"
					icon="schedule"
					compact
					heading={m.knowledge_callout_review_due({ date: formatDate(entry.review_by ?? '') })}
				>
					{m.knowledge_callout_review_due_body()}
					{#if canManage}
						<mm-button slot="actions" size="sm" loading={reviewing} onclick={() => review(6)}
							>{m.knowledge_entry_still_true()}</mm-button
						>
					{/if}
				</mm-callout>
			{/if}
		{/if}

		{#if editing && draft}
			<div class="mm-stack" style="--mm-gap: var(--space-6); max-width: 46rem">
				<EntryDetailsForm {kind} bind:draft errors={fieldErrors} assetId={asset.id} />
				{#if saveError}
					<mm-callout tone="danger" heading={m.knowledge_entry_save_error()}>{saveError}</mm-callout
					>
				{/if}
				<div class="mm-cluster footer">
					<mm-button disabled={saving} onclick={() => (editing = false)}
						>{m.common_cancel()}</mm-button
					>
					<mm-button variant="primary" loading={saving} onclick={save}
						>{m.knowledge_entry_save()}</mm-button
					>
				</div>
			</div>
		{:else}
			{#if facts.length}
				<mm-description-list layout="inline-grid" columns={4} class="facts">
					{#each facts as f (f.key)}
						<mm-description term={f.label}>
							{#if f.key === 'status' && status}
								<mm-status-pill status={stateTone[status.state]} dot>{status.label}</mm-status-pill>
							{:else if f.field}
								<FieldValue field={f.field} value={entry.fields[f.field.key]} />
							{:else if f.text}
								{f.text}
							{:else}
								<mm-text tone="tertiary">{m.knowledge_not_set()}</mm-text>
							{/if}
						</mm-description>
					{/each}
				</mm-description-list>
			{/if}

			<div class="layout">
				<div class="mm-stack main" style="--mm-gap: var(--space-6)">
					{#each wide as f (f.key)}
						<mm-section heading={f.label}>
							<FieldValue field={f} value={entry.fields[f.key]} />
						</mm-section>
					{/each}
					{#if asset.mrn}
						<DocumentationSystem
							entityType="asset"
							entityId={asset.mrn}
							hideTitle={asset.name}
							tabs
						/>
					{/if}
				</div>

				<aside class="rail" aria-label={m.knowledge_rail_label()}>
					<div class="mm-cluster mm-cluster--between">
						<mm-heading level={2} variant="heading-sm">{m.knowledge_group_links()}</mm-heading>
						{#if canManage}
							<mm-button
								size="sm"
								variant="ghost"
								icon="edit"
								loading={openingLinks}
								onclick={openLinks}>{m.knowledge_links_manage()}</mm-button
							>
						{/if}
					</div>

					{#each linkGroups as g (g.relation)}
						<div class="mm-stack" style="--mm-gap: var(--space-2)">
							<mm-text variant="label" tone="secondary" block>{g.label}</mm-text>
							<ul class="refs">
								{#each g.links as l (l.target.id)}
									<li class="link-row">
										<span class="mark"><AssetMark asset={l.target} size={16} /></span>
										<span class="link-text">
											<a
												class="mm-link link-name"
												href={resolve(refPath(l.target) as '/')}
												title={l.target.name}>{l.target.name}</a
											>
											{#if l.target.type}<mm-text variant="caption" tone="secondary" block
													>{typeLabel(l.target.type)}</mm-text
												>{/if}
										</span>
									</li>
								{/each}
							</ul>
						</div>
					{/each}
					{#if moreLinks}
						<mm-link href={resolve('/knowledge/[id]/links', { id: asset.id })}
							>{m.knowledge_links_see_all({ count: entry.link_count })}</mm-link
						>
					{/if}
					{#if entry.rules.length}
						<div class="mm-stack" style="--mm-gap: var(--space-2)">
							<mm-text variant="label" tone="secondary" block>{m.knowledge_group_rules()}</mm-text>
							<ul class="refs">
								{#each entry.rules as r (r.id)}
									<li>
										<span>{ruleLabel(r)}</span>
										<mm-text variant="caption" tone="secondary">
											<mm-link href={resolve(ruleDiscoverHref(r) as '/')}>{ruleCoverage(r)}</mm-link
											>
										</mm-text>
									</li>
								{/each}
							</ul>
						</div>
					{/if}
					{#if !entry.link_count && !entry.rules.length}
						<mm-text tone="secondary" variant="dense">{m.knowledge_links_empty()}</mm-text>
					{/if}

					{#if entry.supersedes}
						<div class="mm-stack" style="--mm-gap: var(--space-2)">
							<mm-text variant="label" tone="secondary" block
								>{m.knowledge_group_replaces()}</mm-text
							>
							<mm-link href={resolve('/knowledge/[id]', { id: entry.supersedes.id })}
								>{entry.supersedes.name}</mm-link
							>
						</div>
					{/if}
					{#if entry.superseded_by.length}
						<div class="mm-stack" style="--mm-gap: var(--space-2)">
							<mm-text variant="label" tone="secondary" block
								>{m.knowledge_replaced_by_label()}</mm-text
							>
							<ul class="refs">
								{#each entry.superseded_by as r (r.id)}
									<li>
										<mm-link href={resolve('/knowledge/[id]', { id: r.id })}>{r.name}</mm-link>
									</li>
								{/each}
							</ul>
						</div>
					{/if}

					<div class="meta">
						{#if entry.last_reviewed_at}
							<mm-text variant="caption" tone="secondary" block
								>{m.knowledge_meta_reviewed({
									when: formatRelativeTime(entry.last_reviewed_at),
									who: entry.last_reviewed_by || m.common_unknown()
								})}</mm-text
							>
						{/if}
						{#if entry.updated_at}
							<mm-text variant="caption" tone="secondary" block
								>{m.knowledge_meta_changed({
									when: formatRelativeTime(entry.updated_at),
									who: entry.updated_by || m.common_unknown()
								})}</mm-text
							>
						{/if}
					</div>
				</aside>
			</div>
		{/if}
	</div>

	<mm-dialog
		heading={m.knowledge_links_manage_heading({ name: asset.name })}
		description={m.knowledge_section_links_description()}
		size="lg"
		open={linksOpen}
		onmm-close={() => (linksOpen = false)}
	>
		{#if linkDraft}
			<div class="mm-stack" style="--mm-gap: var(--space-6)">
				{#if linkDraft.links}
					<LinksEditor
						value={linkDraft.links}
						exclude={asset.id}
						onchange={(v) => linkDraft && (linkDraft = { ...linkDraft, links: v })}
					/>
				{:else}
					<mm-callout tone="info" heading={m.knowledge_links_too_many()}
						>{m.knowledge_links_too_many_body()}</mm-callout
					>
				{/if}
				<RulesEditor
					value={linkDraft.rules}
					onchange={(v) => linkDraft && (linkDraft = { ...linkDraft, rules: v })}
				/>
				{#if linksError}
					<mm-callout tone="danger" heading={m.knowledge_entry_save_error()}
						>{linksError}</mm-callout
					>
				{/if}
			</div>
		{/if}
		<div
			slot="footer"
			class="mm-cluster"
			style="--mm-gap: var(--space-2); justify-content: flex-end"
		>
			<mm-button disabled={savingLinks} onclick={() => (linksOpen = false)}
				>{m.common_cancel()}</mm-button
			>
			<mm-button variant="primary" loading={savingLinks} onclick={saveLinks}
				>{m.knowledge_entry_save()}</mm-button
			>
		</div>
	</mm-dialog>

	<mm-confirm-dialog
		bind:this={deleteDialog}
		heading={m.knowledge_entry_delete_heading({ name: asset.name })}
		body={m.knowledge_entry_delete_body()}
		confirm-label={m.knowledge_entry_delete()}
		tone="danger"
		loading={deleting}
		onmm-confirm={onDeleteConfirm}
	></mm-confirm-dialog>
{/if}

<style>
	.layout {
		display: grid;
		grid-template-columns: minmax(0, 1fr) 18rem;
		gap: var(--space-8);
		align-items: start;
	}
	@media (max-width: 1024px) {
		.layout {
			grid-template-columns: minmax(0, 1fr);
		}
		.rail {
			order: -1;
		}
	}
	.rail {
		display: flex;
		flex-direction: column;
		gap: var(--space-5);
		padding-inline-start: var(--space-5);
		border-inline-start: 1px solid var(--mm-rule);
	}
	@media (max-width: 1024px) {
		.rail {
			padding-inline-start: 0;
			border-inline-start: 0;
		}
	}
	.facts {
		padding-block: var(--space-4);
		border-block: 1px solid var(--mm-rule);
	}
	/* TODO(design-system): a content-sized inline layout for mm-description-list. Equal columns left wide gaps between short values and pushed a fifth one onto its own row. */
	.facts::part(list) {
		display: flex;
		flex-wrap: wrap;
		gap: var(--space-4) var(--space-8);
	}
	.refs li {
		display: flex;
		align-items: baseline;
		flex-wrap: wrap;
		gap: var(--space-2);
	}
	/* A linked asset reads as one row whatever its name's length: the mark, then the name cut short with the full name on hover, then its type underneath. */
	.refs li.link-row {
		display: grid;
		grid-template-columns: auto minmax(0, 1fr);
		align-items: start;
		gap: var(--space-2);
		flex-wrap: nowrap;
	}
	.mark {
		display: flex;
		padding-block-start: 2px;
	}
	.link-text {
		min-width: 0;
	}
	.link-name {
		display: block;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.meta {
		display: flex;
		flex-direction: column;
		gap: var(--space-1);
		padding-block-start: var(--space-4);
		border-block-start: 1px solid var(--mm-rule);
	}
	.refs {
		list-style: none;
		margin: 0;
		padding: 0;
		display: flex;
		flex-direction: column;
		gap: var(--space-1);
	}
	.footer {
		--mm-gap: var(--space-2);
		justify-content: flex-end;
		border-block-start: 1px solid var(--mm-rule);
		padding-block-start: var(--space-4);
	}
</style>
