<script lang="ts">
	import { tick } from 'svelte';
	import '@marmotdata/elements/combobox.js';
	import '@marmotdata/elements/key-value-editor.js';
	import { objectToPairs, pairsToObject } from '@marmotdata/elements/key-value-editor.js';
	import type { KeyValuePair } from '@marmotdata/elements/classes/key-value-editor.js';
	import type { MmConfirmDialog } from '@marmotdata/elements/classes/confirm-dialog.js';
	import type { MmDialog } from '@marmotdata/elements/classes/dialog.js';
	import { toast } from '@marmotdata/elements/toast.js';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import MarkdownRenderer from '$components/ui/MarkdownRenderer.svelte';
	import TermForm from './TermForm.svelte';
	import LinkAssetsDrawer from './LinkAssetsDrawer.svelte';
	import { auth } from '$lib/stores/auth';
	import { m } from '$lib/paraglide/messages';
	import { formatDate } from '$lib/utils';
	import { deleteTerm, getTerm, updateTerm } from '$lib/glossary/api';
	import {
		assetRouteParams,
		getLinkedAssets,
		groupLink,
		type LinkedAsset,
		type LinkGroup
	} from '$lib/glossary/assets';
	import { possibleParents } from '$lib/glossary/tree';
	import type { CreateTermInput, GlossaryTerm, UpdateTermInput } from '$lib/glossary/types';

	let {
		term,
		allTerms,
		canManage,
		onUpdated,
		onDeleted
	}: {
		term: GlossaryTerm;
		allTerms: GlossaryTerm[];
		canManage: boolean;
		onUpdated: (term: GlossaryTerm) => void;
		onDeleted: () => void;
	} = $props();

	// Linking a term to an asset changes the asset, so it needs asset rights rather than glossary rights
	const canLink = auth.hasPermission('assets', 'manage');
	const LINKED_LIMIT = 12;

	let editing = $state(false);
	let editingMetadata = $state(false);
	let metadataDraft = $state<KeyValuePair[]>([]);
	let savingMetadata = $state(false);
	let linking = $state(false);

	interface LinkedSection {
		group: LinkGroup;
		assets: LinkedAsset[];
	}

	let sections = $state<LinkedSection[]>([]);
	let linkedTotal = $derived(sections.reduce((n, sec) => n + sec.group.count, 0));
	let truncated = $derived(sections.some((sec) => sec.group.count > sec.assets.length));
	let linkedLoading = $state(true);
	let linkedError = $state(false);

	let moveDialog = $state<MmDialog | null>(null);
	let moveParent = $state('');
	let moving = $state(false);
	let deleteDialog = $state<MmConfirmDialog | null>(null);
	let deleting = $state(false);

	let fetchedParent = $state<GlossaryTerm | null>(null);

	let parent = $derived(
		term.parent_term_id
			? (allTerms.find((t) => t.id === term.parent_term_id) ??
					(fetchedParent?.id === term.parent_term_id ? fetchedParent : null))
			: null
	);
	let children = $derived(allTerms.filter((t) => t.parent_term_id === term.id));
	let metadataEntries = $derived(Object.entries(term.metadata ?? {}));

	// Terms the move dialog may offer: never the term itself or anything beneath it
	let moveOptions = $derived(possibleParents(allTerms, term.id));

	// Keyed on the id alone so saving the term does not reset the page or reload its assets
	let termId = $derived(term.id);

	$effect(() => {
		const id = termId;
		editing = false;
		editingMetadata = false;
		loadLinked(id);
	});

	// The parent may be outside the loaded list (a very large glossary), so fetch it when it is missing
	$effect(() => {
		const parentId = term.parent_term_id;
		if (!parentId || allTerms.some((t) => t.id === parentId) || fetchedParent?.id === parentId)
			return;
		getTerm(parentId)
			.then((p) => (fetchedParent = p))
			.catch(() => (fetchedParent = null));
	});

	// Linked assets are grouped by how they got here: linked manually first, then each asset rule that links them
	async function loadLinked(id: string) {
		linkedLoading = true;
		linkedError = false;
		try {
			const manual = await getLinkedAssets(id, LINKED_LIMIT, 'manual');
			const ruled = await Promise.all(
				manual.groups
					.filter((g) => g.source === 'rule')
					.map(async (group) => ({
						group,
						assets: (await getLinkedAssets(id, LINKED_LIMIT, groupLink(group))).assets
					}))
			);
			if (id !== term.id) return;
			const manualGroup = manual.groups.find((g) => g.source === 'manual');
			sections = [
				...(manualGroup ? [{ group: manualGroup, assets: manual.assets }] : []),
				...ruled
			];
		} catch {
			sections = [];
			linkedError = true;
		} finally {
			linkedLoading = false;
		}
	}

	function normalise(t: GlossaryTerm): GlossaryTerm {
		return { ...t, tags: t.tags ?? [], metadata: t.metadata ?? {}, owners: t.owners ?? [] };
	}

	async function saveEdit(input: CreateTermInput): Promise<string | void> {
		try {
			const updated = await updateTerm(term.id, input as UpdateTermInput);
			onUpdated(normalise(updated));
			editing = false;
		} catch (e) {
			return e instanceof Error ? e.message : m.glossary_update_error();
		}
	}

	async function startMetadataEdit() {
		metadataDraft = objectToPairs(term.metadata ?? {});
		editingMetadata = true;
		await tick();
	}

	async function saveMetadata() {
		savingMetadata = true;
		try {
			const updated = await updateTerm(term.id, { metadata: pairsToObject(metadataDraft) });
			onUpdated(normalise(updated));
			editingMetadata = false;
		} catch (e) {
			toast({
				tone: 'danger',
				message: e instanceof Error ? e.message : m.glossary_update_error()
			});
		} finally {
			savingMetadata = false;
		}
	}

	function onMenuSelect(e: CustomEvent<{ value: string }>) {
		switch (e.detail.value) {
			case 'copy':
				copyLink();
				break;
			case 'move':
				moveParent = term.parent_term_id ?? '';
				moveDialog?.show();
				break;
			case 'delete':
				deleteDialog?.show();
				break;
		}
	}

	async function copyLink() {
		const url = new URL(resolve('/glossary/[[id]]', { id: term.id }), page.url.origin);
		try {
			await navigator.clipboard.writeText(url.toString());
			toast({ tone: 'success', message: m.glossary_link_copied() });
		} catch {
			toast({ tone: 'danger', message: m.glossary_link_copy_error() });
		}
	}

	async function move() {
		moving = true;
		try {
			const updated = await updateTerm(term.id, { parent_term_id: moveParent });
			onUpdated(normalise(updated));
			moveDialog?.close('moved');
		} catch (e) {
			toast({
				tone: 'danger',
				message: e instanceof Error ? e.message : m.glossary_update_error()
			});
		} finally {
			moving = false;
		}
	}

	async function confirmDelete(e: Event) {
		// Keep the dialog open while the request runs so a failure can be shown in place
		e.preventDefault();
		deleting = true;
		try {
			await deleteTerm(term.id);
			deleteDialog?.close('confirm');
			toast({ tone: 'success', message: m.glossary_deleted() });
			onDeleted();
		} catch {
			toast({ tone: 'danger', message: m.glossary_delete_error() });
		} finally {
			deleting = false;
		}
	}

	function formatValue(v: unknown): string {
		if (v === null || v === undefined || v === '') return '';
		if (typeof v === 'boolean') return v ? m.common_yes() : m.common_no();
		if (Array.isArray(v)) return v.map((x) => formatValue(x)).join(', ');
		if (typeof v === 'object') return JSON.stringify(v);
		return String(v);
	}

	const discoverHref = $derived(
		resolve(
			`/discover?q=${encodeURIComponent(`@term: "${term.name.replaceAll('"', '\\"')}"`)}` as '/discover'
		)
	);
</script>

{#if editing}
	<TermForm {term} {allTerms} onCancel={() => (editing = false)} onSubmit={saveEdit} />
{:else}
	<article class="mm-page mm-stack">
		<mm-page-header heading={term.name}>
			{#each term.tags ?? [] as tag (tag)}
				<mm-tag
					slot="tags"
					variant={tag === 'pii' ? 'pii' : tag === 'certified' ? 'certified' : 'plain'}
					>{tag === 'pii' ? '' : tag}</mm-tag
				>
			{/each}
			{#if term.owners?.length}
				<span slot="owners" class="mm-cluster">
					{#each term.owners as o (`${o.type}:${o.id}`)}
						<mm-owner-chip
							name={o.name || o.username || o.id}
							kind={o.type === 'team' ? 'team' : 'person'}
							src={o.profile_picture ?? ''}
							email={o.email ?? ''}
							href={o.type === 'team' ? resolve('/teams/[id]', { id: o.id }) : ''}
						></mm-owner-chip>
					{/each}
				</span>
			{/if}
			<mm-menu-button
				slot="actions"
				variant="ghost"
				icon="more-horiz"
				icon-only
				label={m.glossary_more_actions({ name: term.name })}
			>
				<mm-menu onmm-select={onMenuSelect}>
					<mm-menu-item icon="link" value="copy">{m.glossary_copy_link()}</mm-menu-item>
					{#if canManage}
						<mm-menu-item icon="account-tree" value="move">{m.glossary_move_term()}</mm-menu-item>
						<mm-menu-separator></mm-menu-separator>
						<mm-menu-item icon="delete" value="delete" danger
							>{m.glossary_delete_term()}</mm-menu-item
						>
					{/if}
				</mm-menu>
			</mm-menu-button>
			{#if canManage}
				<mm-button slot="actions" icon="edit" onclick={() => (editing = true)}
					>{m.glossary_edit_term()}</mm-button
				>
			{/if}
		</mm-page-header>

		<div class="mm-split">
			<div class="mm-stack" style="--mm-gap: 0">
				<mm-section heading={m.glossary_definition_label()}>
					<div class="mm-prose">
						<p>{term.definition}</p>
						{#if term.description}
							<MarkdownRenderer content={term.description} />
						{/if}
					</div>
				</mm-section>

				<mm-section>
					<span slot="heading"
						>{m.glossary_linked_heading()}
						{#if linkedTotal > 0}<mm-count value={linkedTotal}></mm-count>{/if}</span
					>
					{#if canLink}
						<mm-button slot="actions" size="sm" icon="link" onclick={() => (linking = true)}
							>{m.glossary_link_title()}</mm-button
						>
					{/if}
					{#if linkedLoading}
						<mm-skeleton loading shape="text" count={3}></mm-skeleton>
					{:else if linkedError}
						<mm-callout tone="danger" compact>{m.glossary_linked_error()}</mm-callout>
					{:else if sections.length === 0}
						<mm-empty-state
							compact
							heading={m.glossary_linked_empty_title()}
							description={m.glossary_linked_empty()}
						></mm-empty-state>
					{:else}
						<div class="mm-stack" style="--mm-gap: var(--space-5)">
							{#each sections as sec (groupLink(sec.group))}
								{@const label =
									sec.group.source !== 'rule'
										? m.glossary_linked_manually()
										: sec.group.rule_name
											? m.glossary_linked_from_rule({ rule: sec.group.rule_name })
											: m.glossary_linked_from_deleted_rule()}
								<div class="mm-stack" style="--mm-gap: var(--space-1)">
									<div class="mm-cluster mm-cluster--between">
										<mm-text variant="label" tone="secondary">{label}</mm-text>
										{#if sec.group.source === 'rule' && sec.group.rule_id && sec.group.rule_name}
											<mm-link href={resolve('/asset-rules/[id]', { id: sec.group.rule_id })}
												>{m.glossary_linked_edit_rule()}</mm-link
											>
										{/if}
									</div>
									<mm-list aria-label={label}>
										{#each sec.assets as a (a.id)}
											<mm-list-item
												href={resolve('/discover/[type]/[service]/[name]', assetRouteParams(a))}
											>
												<mm-provider-logo
													slot="start"
													provider={a.providers?.[0] ?? a.type}
													size={16}
													decorative
												></mm-provider-logo>
												{a.name}
												<span slot="secondary"
													>{[a.type, a.providers?.[0]].filter(Boolean).join(' · ')}</span
												>
											</mm-list-item>
										{/each}
									</mm-list>
								</div>
							{/each}
							<div>
								<mm-link href={discoverHref}
									>{truncated
										? m.glossary_linked_view_all()
										: m.glossary_linked_view_discover()}</mm-link
								>
							</div>
						</div>
					{/if}
					{#if canLink && !linkedLoading}
						<p class="mm-muted glossary-rule-hint">
							{m.glossary_linked_rule_hint()}
							<a class="mm-link" href={resolve('/asset-rules/new')}
								>{m.glossary_linked_rule_link()}</a
							>
						</p>
					{/if}
				</mm-section>
			</div>

			<div class="mm-stack" style="--mm-gap: 0">
				<mm-section heading={m.glossary_hierarchy_heading()}>
					<mm-description-list layout="stacked">
						<mm-description term={m.glossary_parent_label()}>
							{#if parent}
								<a
									class="mm-link mm-link--quiet"
									href={resolve('/glossary/[[id]]', { id: parent.id })}>{parent.name}</a
								>
							{/if}
						</mm-description>
						<mm-description term={m.glossary_child_terms()}>
							{#if children.length}
								{#each children as c, i (c.id)}{i ? ', ' : ''}<a
										class="mm-link mm-link--quiet"
										href={resolve('/glossary/[[id]]', { id: c.id })}>{c.name}</a
									>{/each}
							{:else}
								{m.glossary_none()}
							{/if}
						</mm-description>
						<mm-description term={m.glossary_related_terms_label()}>
							{#if term.related_terms?.length}
								{#each term.related_terms as r, i (r.id)}{i ? ', ' : ''}<a
										class="mm-link mm-link--quiet"
										href={resolve('/glossary/[[id]]', { id: r.id })}>{r.name}</a
									>{/each}
							{:else}
								{m.glossary_none()}
							{/if}
						</mm-description>
					</mm-description-list>
				</mm-section>

				<mm-section heading={m.glossary_metadata_heading()}>
					{#if canManage && !editingMetadata}
						<mm-button
							slot="actions"
							size="sm"
							variant="ghost"
							icon="edit"
							label={m.glossary_edit_metadata()}
							onclick={startMetadataEdit}>{m.common_edit()}</mm-button
						>
					{/if}
					{#if editingMetadata}
						<div class="mm-stack" style="--mm-gap: var(--space-3)">
							<mm-key-value-editor
								label={m.glossary_metadata_heading()}
								json
								value={metadataDraft}
								onmm-change={(e: CustomEvent<{ value: KeyValuePair[] }>) =>
									(metadataDraft = e.detail.value)}
							></mm-key-value-editor>
							<div class="mm-cluster">
								<mm-button
									size="sm"
									disabled={savingMetadata}
									onclick={() => (editingMetadata = false)}>{m.common_cancel()}</mm-button
								>
								<mm-button
									size="sm"
									variant="primary"
									loading={savingMetadata}
									onclick={saveMetadata}>{m.common_save()}</mm-button
								>
							</div>
						</div>
					{:else}
						<mm-description-list layout="stacked">
							{#each metadataEntries as [key, value] (key)}
								<mm-description term={key}>{formatValue(value)}</mm-description>
							{/each}
							<mm-description term={m.glossary_created_label()}
								>{term.created_by_name
									? m.glossary_created_by({
											date: formatDate(term.created_at),
											name: term.created_by_name
										})
									: formatDate(term.created_at)}</mm-description
							>
							<mm-description term={m.glossary_last_updated_label()}
								>{formatDate(term.updated_at)}</mm-description
							>
						</mm-description-list>
					{/if}
				</mm-section>
			</div>
		</div>
	</article>
{/if}

{#if linking}
	<LinkAssetsDrawer
		termId={term.id}
		termName={term.name}
		onClose={() => {
			linking = false;
			loadLinked(term.id);
		}}
	/>
{/if}

<mm-dialog
	bind:this={moveDialog}
	heading={m.glossary_move_heading()}
	description={m.glossary_move_description()}
>
	<mm-combobox
		label={m.glossary_parent_label()}
		value={moveParent}
		onmm-change={(e: CustomEvent<{ value: string }>) => (moveParent = e.detail.value)}
	>
		{#each moveOptions as t (t.id)}
			<mm-option value={t.id} label={t.name}></mm-option>
		{/each}
	</mm-combobox>
	<mm-button slot="footer" data-close="cancel" disabled={moving}>{m.common_cancel()}</mm-button>
	<mm-button slot="footer" variant="primary" loading={moving} onclick={move}
		>{m.glossary_move_button()}</mm-button
	>
</mm-dialog>

<mm-confirm-dialog
	bind:this={deleteDialog}
	heading={m.glossary_delete_heading()}
	body={m.glossary_delete_confirm({ name: term.name })}
	confirm-label={m.glossary_delete_term()}
	confirm-icon="delete"
	loading={deleting}
	onmm-confirm={confirmDelete}
></mm-confirm-dialog>

<style>
	.glossary-rule-hint {
		margin: var(--space-4) 0 0;
		font: var(--text-caption);
	}
</style>
