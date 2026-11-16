<script lang="ts">
	import '@marmotdata/elements/link-list.js';
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
	import { discoverPath, discoverPathname } from '#lib/mrn.ts';
	import { kindById, loadKinds, statusOf, type KnowledgeKind } from '#lib/knowledge/kinds.ts';
	import {
		getEntry,
		markReviewed,
		monthsFromToday,
		saveEntry,
		type KnowledgeAssetRef,
		type KnowledgeEntry
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
	import StatusPill from '#components/knowledge/StatusPill.svelte';

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
			...(kind ? [{ label: kind.label, href: `${resolve('/knowledge')}?kind=${kind.id}` }] : []),
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

	function startEditing() {
		if (!kind || !entry) return;
		draft = draftFrom(kind, entry);
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

	function refHref(r: KnowledgeAssetRef): string {
		const path = discoverPath(r.mrn);
		return path ? resolve(discoverPathname(path)) : '#';
	}

	let status = $derived(entry && kind ? statusOf(kind, entry.status) : undefined);
	let catalogHref = $derived.by(() => {
		const path = discoverPath(asset?.mrn);
		return path ? `${path}?view=asset` : '';
	});
	let filled = $derived(
		kind && entry ? kind.fields.filter((f) => hasValue(entry!.fields[f.key])) : []
	);
	let missing = $derived(
		kind && entry ? kind.fields.filter((f) => !hasValue(entry!.fields[f.key]) && f.required) : []
	);
	let knowledgeRefs = $derived((refs: KnowledgeAssetRef[]) => refs.filter((r) => r.id));
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
			<span slot="tags" style="display: contents"
				><StatusPill
					{kind}
					status={entry.status}
					state={entry.state}
					reviewDue={entry.review_due}
				/></span
			>
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
			<div class="layout">
				<div class="mm-stack main" style="--mm-gap: var(--space-6)">
					{#if filled.length || missing.length}
						<mm-section heading={m.knowledge_section_details({ kind: kind.label })}>
							<div class="mm-stack" style="--mm-gap: var(--space-5)">
								{#each filled as f (f.key)}
									<div class="field">
										<mm-text variant="label" tone="secondary" block>{f.label}</mm-text>
										<FieldValue field={f} value={entry.fields[f.key]} />
									</div>
								{/each}
								{#each missing as f (f.key)}
									<div class="field">
										<mm-text variant="label" tone="secondary" block>{f.label}</mm-text>
										<mm-text tone="tertiary">{m.knowledge_not_set()}</mm-text>
									</div>
								{/each}
							</div>
						</mm-section>
					{/if}
					<mm-section heading={m.knowledge_section_writeup()}>
						{#if asset.mrn}
							<DocumentationSystem entityType="asset" entityId={asset.mrn} />
						{/if}
					</mm-section>
				</div>

				<aside class="rail" aria-label={m.knowledge_rail_label()}>
					<mm-description-list layout="stacked">
						<mm-description term={m.knowledge_status_label()}
							>{status?.label ?? m.knowledge_not_set()}</mm-description
						>
						<mm-description term={m.knowledge_effective_from_label()}
							>{entry.effective_from ? formatDate(entry.effective_from) : ''}</mm-description
						>
						<mm-description term={m.knowledge_review_by_label()}
							>{entry.review_by ? formatDate(entry.review_by) : ''}</mm-description
						>
						<mm-description term={m.knowledge_last_reviewed_label()}
							>{entry.last_reviewed_at
								? m.knowledge_last_reviewed_value({
										when: formatRelativeTime(entry.last_reviewed_at),
										who: entry.last_reviewed_by || m.common_unknown()
									})
								: ''}</mm-description
						>
						{#if entry.updated_at}
							<mm-description term={m.knowledge_updated_label()}
								>{m.knowledge_last_reviewed_value({
									when: formatRelativeTime(entry.updated_at),
									who: entry.updated_by || m.common_unknown()
								})}</mm-description
							>
						{/if}
					</mm-description-list>

					<div class="mm-stack" style="--mm-gap: var(--space-2)">
						<mm-text variant="label" tone="secondary" block>{m.knowledge_group_applies()}</mm-text>
						{#if entry.applies_to.length}
							<ul class="refs">
								{#each knowledgeRefs(entry.applies_to) as r (r.id)}
									<li><mm-link href={refHref(r)}>{r.name}</mm-link></li>
								{/each}
							</ul>
						{:else}
							<mm-text tone="tertiary" variant="dense">{m.knowledge_applies_none()}</mm-text>
						{/if}
					</div>

					{#if entry.supersedes || entry.superseded_by.length}
						<div class="mm-stack" style="--mm-gap: var(--space-2)">
							{#if entry.supersedes}
								<mm-text variant="label" tone="secondary" block
									>{m.knowledge_group_replaces()}</mm-text
								>
								<mm-link href={resolve('/knowledge/[id]', { id: entry.supersedes.id })}
									>{entry.supersedes.name}</mm-link
								>
							{/if}
							{#if entry.superseded_by.length}
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
							{/if}
						</div>
					{/if}
				</aside>
			</div>
		{/if}
	</div>

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
	.field {
		display: flex;
		flex-direction: column;
		gap: var(--space-1);
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
