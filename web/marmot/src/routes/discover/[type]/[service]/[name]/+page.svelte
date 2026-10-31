<script lang="ts">
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { fetchApi, fetchAssetPreview, type AssetPreviewResponse } from '$lib/api';
	import type { Asset, AssetTabCounts, EnrichedExternalLink } from '$lib/assets/types';
	import { setCrumbs, BladeSlot } from '$lib/mm';
	import DocumentationSystem from '$components/docs/DocumentationSystem.svelte';
	import AssetMetadata from '$components/asset/AssetMetadata.svelte';
	import Lineage from '$components/lineage/Lineage.svelte';
	import AssetContents from '$components/asset/AssetContents.svelte';
	import SchemaEditor from '$components/schema/SchemaEditor.svelte';
	import AssetEnvironmentsView from '$components/asset/AssetEnvironmentsView.svelte';
	import AssetRunHistory from '$components/asset/AssetRunHistory.svelte';
	import AgentRunsView from '$components/asset/AgentRunsView.svelte';
	import AssetQueryPanel from '$components/dataquery/AssetQueryPanel.svelte';
	import DataPreviewTable from '$components/asset/DataPreviewTable.svelte';
	import SubscribeButton from '$components/asset/SubscribeButton.svelte';
	import AssetPageBlade from '$components/asset/AssetPageBlade.svelte';
	import AssetOwnerPicker, { type AssetOwner } from '$components/asset/AssetOwnerPicker.svelte';
	import AssetLinksEditor from '$components/asset/AssetLinksEditor.svelte';
	import { linkIcon, providerKey } from '$components/asset/assetFormat';
	import { auth } from '$lib/stores/auth';
	import { tablePreviewEnabled } from '$lib/stores/features';
	import { toasts } from '$lib/stores/toast';
	import { formatRelativeTime } from '$lib/utils';
	import { formatNumber } from '$lib/utils/format';
	import { m } from '$lib/paraglide/messages';
	import type { MenuSelectDetail } from '@marmotdata/elements/classes/menu.js';
	import type { MmConfirmDialog } from '@marmotdata/elements/classes/confirm-dialog.js';
	import '@marmotdata/elements/code-sample.js';

	type TabId =
		| 'documentation'
		| 'metadata'
		| 'schema'
		| 'lineage'
		| 'runs'
		| 'run-history'
		| 'queries'
		| 'query'
		| 'preview'
		| 'environments'
		| 'contents';

	interface TabDef {
		id: TabId;
		label: string;
		icon: string;
		count?: number;
	}

	let asset = $state<Asset | null>(null);
	let enrichedLinks = $state<EnrichedExternalLink[]>([]);
	let loading = $state(true);
	let error: string | null = $state(null);

	let owners: AssetOwner[] = $state([]);
	let counts = $state<AssetTabCounts | null>(null);
	let children: Asset[] = $state([]);
	let parents: Asset[] = $state([]);
	let loadingChildren = $state(false);

	let editingNotes = $state(false);
	let notesDraft = $state('');
	let savingNotes = $state(false);

	let tagDraft = $state('');
	let tagPopoverOpen = $state(false);

	let deleteDialog = $state<MmConfirmDialog>();
	let deleting = $state(false);

	// The blade starts folded on tabs that need the width; a fold or unfold by hand holds until the tab changes.
	let bladeOverride = $state<boolean | null>(null);

	let previewData: AssetPreviewResponse | null = $state(null);
	let previewLoading = $state(false);
	let previewError: string | null = $state(null);

	let canManageAssets = $derived(auth.hasPermission('assets', 'manage'));
	const canQueryData = auth.hasPermission('assets', 'query');

	let assetType = $derived($page.params.type ?? '');
	let assetService = $derived($page.params.service ?? '');
	let assetName = $derived($page.params.name ?? '');

	let isAgent = $derived((asset?.type || '').toLowerCase() === 'agent');
	let isTable = $derived(
		['table', 'view', 'dataset'].some((k) => (asset?.type || '').toLowerCase().includes(k))
	);
	let manualLinks = $derived(asset?.external_links ?? []);
	let ruleLinks = $derived(enrichedLinks.filter((l) => l.source !== 'asset'));

	// Providers whose assets form a drive tree worth browsing as folders; other providers nest too, but their structure reads better as lineage.
	const driveProviders = new Set(['googledrive', 'sharepoint']);

	let tabs = $derived.by((): TabDef[] => {
		if (!asset) return [];
		const a = asset;
		const envCount = a.environments ? Object.keys(a.environments).length : 0;
		const all: (TabDef & { show: boolean })[] = [
			{
				id: 'documentation',
				label: m.common_documentation(),
				icon: 'description',
				show: true
			},
			{
				id: 'runs',
				label: m.discover_tab_runs(),
				icon: 'play-arrow',
				count: counts?.agent_runs || undefined,
				show: isAgent
			},
			{ id: 'metadata', label: m.discover_tab_metadata(), icon: 'data-object', show: !isAgent },
			{
				id: 'schema',
				label: m.discover_tab_schema(),
				icon: 'view-column',
				count: a.column_count || undefined,
				show: !isAgent
			},
			{
				id: 'lineage',
				label: m.discover_tab_lineage(),
				icon: 'account-tree',
				count: counts?.lineage || undefined,
				show: true
			},
			{
				id: 'run-history',
				label: m.asset_page_tab_run_history(),
				icon: 'history',
				count: counts?.runs || undefined,
				show: !!a.has_run_history
			},
			{
				id: 'queries',
				label: m.discover_tab_queries(),
				icon: 'query-stats',
				count: counts?.queries || undefined,
				show: canQueryData && isTable && !!a.mrn
			},
			{ id: 'query', label: m.discover_tab_query(), icon: 'code', show: !!a.query },
			{
				id: 'preview',
				label: m.discover_tab_preview(),
				icon: 'table-view',
				show: isTable && $tablePreviewEnabled
			},
			{
				id: 'environments',
				label: m.discover_tab_environments(),
				icon: 'layers',
				count: envCount,
				show: envCount > 0
			},
			{
				id: 'contents',
				label: m.discover_tab_contents(),
				icon: 'folder-open',
				count: children.length,
				show:
					driveProviders.has((a.providers?.[0] ?? '').toLowerCase()) &&
					(children.length > 0 || parents.length > 0)
			},
			{ id: 'metadata', label: m.discover_tab_metadata(), icon: 'data-object', show: isAgent }
		];
		return all.filter((t) => t.show);
	});

	let requestedTab = $derived($page.url.searchParams.get('tab'));
	let activeTab = $derived.by((): TabId => {
		const ids = tabs.map((t) => t.id);
		if (requestedTab && ids.includes(requestedTab as TabId)) return requestedTab as TabId;
		return isAgent ? 'runs' : 'documentation';
	});

	const wideTabs = new Set<TabId>([
		'schema',
		'lineage',
		'runs',
		'run-history',
		'queries',
		'preview'
	]);
	let bladeCollapsed = $derived(bladeOverride ?? wideTabs.has(activeTab));

	$effect(() => {
		// Reading activeTab resets the manual fold whenever the tab changes.
		void activeTab;
		bladeOverride = null;
	});

	$effect(() => {
		const typeLabel = assetType ? assetType.charAt(0).toUpperCase() + assetType.slice(1) : '';
		setCrumbs([
			{ label: m.asset_page_crumb_discover(), href: resolve('/discover') },
			...(typeLabel
				? [
						{
							label: typeLabel,
							href: `${resolve('/discover')}?types=${encodeURIComponent(assetType)}`
						}
					]
				: []),
			{ label: asset?.name ?? assetName }
		]);
	});

	function tabHref(id: TabId): string {
		// Docs keep their page in ?page=, which belongs to the documentation tab only.
		const kept = [...$page.url.searchParams].filter(([k]) => k !== 'tab' && k !== 'page');
		return `?${new URLSearchParams([...kept, ['tab', id]]).toString()}`;
	}

	async function fetchAsset() {
		try {
			loading = true;
			error = null;
			const response = await fetchApi(
				`/assets/lookup/${assetType}/${assetService}/${encodeURIComponent(assetName)}`
			);
			if (!response.ok) {
				throw new Error(m.discover_asset_fetch_error());
			}
			const data = await response.json();
			enrichedLinks = data.enriched_external_links || [];
			asset = data;
		} catch (err) {
			console.error('Error fetching asset:', err);
			error = err instanceof Error ? err.message : m.discover_asset_load_error();
		} finally {
			loading = false;
		}
	}

	// Containment is stored as CONTAINS lineage, so an asset's children are the immediate targets of its CONTAINS edges.
	async function fetchChildren() {
		if (!asset?.id || !asset?.mrn) {
			children = [];
			parents = [];
			return;
		}
		const self = asset.mrn;
		loadingChildren = true;
		try {
			const response = await fetchApi(`/lineage/assets/${asset.id}?limit=1`);
			if (!response.ok) throw new Error('Failed to fetch contents');
			const data = await response.json();
			const contains = (data.edges || []).filter(
				(edge: { type?: string }) => (edge.type || '').toUpperCase() === 'CONTAINS'
			) as { source: string; target: string }[];
			const childMrns = new Set(contains.filter((e) => e.source === self).map((e) => e.target));
			const parentMrns = new Set(contains.filter((e) => e.target === self).map((e) => e.source));
			const assets: Asset[] = (data.nodes || [])
				.map((node: { asset?: Asset }) => node.asset)
				.filter((a: Asset | undefined): a is Asset => !!a);
			children = assets.filter((a) => childMrns.has(a.mrn));
			parents = assets.filter((a) => parentMrns.has(a.mrn));
		} catch (err) {
			console.error('Failed to fetch contents:', err);
			children = [];
			parents = [];
		} finally {
			loadingChildren = false;
		}
	}

	// Tab counts are a nicety, so a failure leaves the tabs without numbers rather than showing an error.
	async function fetchCounts() {
		if (!asset?.id) return;
		const id = asset.id;
		try {
			const response = await fetchApi(`/assets/counts/${encodeURIComponent(id)}`);
			if (response.ok && asset?.id === id) counts = await response.json();
		} catch {
			counts = null;
		}
	}

	async function fetchOwners() {
		if (!asset?.id) return;
		try {
			const response = await fetchApi(`/assets/owners/?asset_id=${asset.id}`);
			if (!response.ok) throw new Error('Failed to fetch owners');
			const data = await response.json();
			owners = data.owners || [];
		} catch (err) {
			console.error('Failed to fetch owners:', err);
			owners = [];
		}
	}

	async function addOwner(owner: AssetOwner) {
		if (!asset?.id || !canManageAssets) return;
		const response = await fetchApi(`/assets/owners/?asset_id=${asset.id}`, {
			method: 'POST',
			body: JSON.stringify({ owner_type: owner.type, owner_id: owner.id })
		});
		if (response.ok) {
			owners = [...owners, owner];
		} else {
			toasts.error(m.asset_page_owner_add_error());
		}
	}

	async function removeOwner(owner: AssetOwner) {
		if (!asset?.id || !canManageAssets) return;
		const response = await fetchApi(
			`/assets/owners/?asset_id=${asset.id}&owner_type=${owner.type}&owner_id=${owner.id}`,
			{ method: 'DELETE' }
		);
		if (response.ok) {
			owners = owners.filter((o) => !(o.id === owner.id && o.type === owner.type));
		} else {
			toasts.error(m.asset_page_owner_remove_error());
		}
	}

	async function saveTags(tags: string[]): Promise<boolean> {
		if (!asset?.id) return false;
		const response = await fetchApi(`/assets/${asset.id}`, {
			method: 'PUT',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ tags })
		});
		if (!response.ok) {
			toasts.error(m.asset_page_tags_save_error());
			return false;
		}
		asset.tags = tags;
		return true;
	}

	async function addTag(e: SubmitEvent) {
		e.preventDefault();
		const tag = tagDraft.trim();
		if (!asset || !tag) return;
		if ((asset.tags ?? []).includes(tag)) {
			tagDraft = '';
			return;
		}
		if (await saveTags([...(asset.tags ?? []), tag])) {
			tagDraft = '';
			tagPopoverOpen = false;
		}
	}

	function startEditingNotes() {
		notesDraft = asset?.user_description || '';
		editingNotes = true;
	}

	async function saveNotes(value: string) {
		if (!asset?.id) return;
		const trimmed = value.trim();
		savingNotes = true;
		try {
			const response = await fetchApi(`/assets/${asset.id}`, {
				method: 'PUT',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ user_description: trimmed })
			});
			if (!response.ok) throw new Error();
			if (trimmed) asset.user_description = trimmed;
			else delete asset.user_description;
			editingNotes = false;
		} catch {
			toasts.error(m.asset_page_notes_save_error());
		} finally {
			savingNotes = false;
		}
	}

	function onMoreSelect(e: CustomEvent<MenuSelectDetail>) {
		if (e.detail.value === 'edit-notes') startEditingNotes();
		if (e.detail.value === 'delete') deleteDialog?.show();
	}

	async function onDeleteConfirm(e: CustomEvent) {
		// Keep the dialog open with a spinner until the API answers.
		e.preventDefault();
		if (!asset?.id) return;
		deleting = true;
		try {
			const response = await fetchApi(`/assets/${asset.id}`, { method: 'DELETE' });
			if (!response.ok) {
				const data = await response.json().catch(() => ({}));
				throw new Error(data.error || m.asset_delete_error());
			}
			deleteDialog?.close('confirm');
			goto(resolve('/discover'));
		} catch (err) {
			toasts.error(err instanceof Error ? err.message : m.asset_delete_error());
		} finally {
			deleting = false;
		}
	}

	async function fetchPreviewData() {
		if (!asset?.id) return;
		previewLoading = true;
		previewError = null;
		previewData = null;
		try {
			previewData = await fetchAssetPreview(asset.id);
		} catch (err) {
			let errorMsg: string = m.discover_preview_fetch_error();
			const status =
				typeof err === 'object' && err !== null && 'status' in err
					? (err as { status?: number }).status
					: undefined;
			if (status === 403) {
				errorMsg = m.discover_preview_permission_error();
			} else if (err instanceof Error) {
				errorMsg = err.message;
			}
			previewError = errorMsg;
		} finally {
			previewLoading = false;
		}
	}

	// Size first, then freshness, as in "1,284 rows · Updated 5 hours ago".
	let caption = $derived.by(() => {
		if (!asset) return '';
		const parts: string[] = [];
		if (asset.row_count != null) {
			parts.push(
				m.asset_size_rows({ count: asset.row_count, formatted: formatNumber(asset.row_count) })
			);
		}
		if (asset.updated_at) {
			parts.push(m.asset_page_caption_updated({ when: formatRelativeTime(asset.updated_at) }));
		}
		return parts.join(' · ');
	});

	let queryHref = $derived(
		asset
			? `${resolve('/query')}?${new URLSearchParams({ q: `SELECT *\nFROM ${asset.name}\nLIMIT 10` }).toString()}`
			: ''
	);

	$effect(() => {
		if (assetType && assetService && assetName) {
			fetchAsset();
		}
	});

	$effect(() => {
		if (asset?.id) {
			counts = null;
			fetchOwners();
			fetchChildren();
			fetchCounts();
			editingNotes = false;
		}
	});

	// Preview is fetched only when its tab is opened, since it runs a query against the source.
	$effect(() => {
		if (activeTab === 'preview' && asset?.id && !previewData && !previewLoading && !previewError) {
			fetchPreviewData();
		}
	});
</script>

{#if loading && !asset}
	<mm-loading active caption={m.discover_asset_loading()}></mm-loading>
{:else if error || !asset}
	<div class="mm-stack" style="--mm-gap: var(--space-5)">
		<mm-page-header heading={assetName}></mm-page-header>
		<mm-callout tone="danger" heading={m.asset_page_load_error_heading()}>
			{error ?? m.discover_asset_load_error()}
			<mm-button slot="actions" size="sm" variant="secondary" icon="refresh" onclick={fetchAsset}
				>{m.common_retry()}</mm-button
			>
		</mm-callout>
	</div>
{:else}
	<div class="mm-stack" style="--mm-gap: var(--space-5)">
		<mm-page-header
			heading={asset.name}
			provider={providerKey(asset)}
			caption={caption || undefined}
			mrn={asset.mrn}
			description={editingNotes ? undefined : asset.user_description}
		>
			{#if editingNotes}
				<div slot="description" class="mm-stack" style="--mm-gap: var(--space-2)">
					<mm-textarea
						label={m.asset_page_notes_label()}
						hide-label
						rows={2}
						autosize
						autofocus
						value={notesDraft}
						placeholder={m.asset_page_notes_placeholder()}
						onmm-input={(e: CustomEvent<{ value: string }>) => (notesDraft = e.detail.value)}
					></mm-textarea>
					<div class="mm-cluster">
						<mm-button
							variant="primary"
							size="sm"
							loading={savingNotes}
							onclick={() => saveNotes(notesDraft)}>{m.asset_page_save_notes()}</mm-button
						>
						<mm-button
							variant="ghost"
							size="sm"
							disabled={savingNotes}
							onclick={() => (editingNotes = false)}>{m.common_cancel()}</mm-button
						>
						{#if asset.user_description}
							<mm-button
								variant="ghost"
								size="sm"
								disabled={savingNotes}
								onclick={() => saveNotes('')}>{m.asset_page_remove_notes()}</mm-button
							>
						{/if}
					</div>
				</div>
			{/if}

			{#each asset.tags ?? [] as tag (tag)}
				<mm-tag
					slot="tags"
					href={`${resolve('/discover')}?tags=${encodeURIComponent(tag)}`}
					removable={canManageAssets}
					onmm-remove={() => saveTags((asset?.tags ?? []).filter((t) => t !== tag))}>{tag}</mm-tag
				>
			{/each}
			{#if canManageAssets}
				<mm-popover
					slot="tags"
					label={m.asset_page_add_tag()}
					open={tagPopoverOpen}
					onmm-toggle={(e: CustomEvent<{ open: boolean }>) => (tagPopoverOpen = e.detail.open)}
				>
					<mm-button slot="trigger" variant="ghost" size="sm" icon="add"
						>{m.asset_page_add_tag()}</mm-button
					>
					<form class="mm-stack" style="--mm-gap: var(--space-3)" onsubmit={addTag}>
						<mm-text-field
							label={m.asset_page_tag_label()}
							value={tagDraft}
							autofocus
							onmm-input={(e: CustomEvent<{ value: string }>) => (tagDraft = e.detail.value)}
						></mm-text-field>
						<div class="mm-cluster">
							<mm-button type="submit" variant="primary" size="sm" icon="add"
								>{m.asset_page_add_tag()}</mm-button
							>
						</div>
					</form>
				</mm-popover>
			{/if}

			{#each owners as owner (`${owner.type}-${owner.id}`)}
				<mm-owner-chip
					slot="owners"
					name={owner.name}
					kind={owner.type === 'team' ? 'team' : 'person'}
					src={owner.profile_picture}
					href={owner.type === 'team'
						? resolve('/settings/teams/[id]', { id: owner.id })
						: undefined}
					email={owner.email}
					removable={canManageAssets}
					onmm-remove={() => removeOwner(owner)}
				></mm-owner-chip>
			{/each}
			{#if canManageAssets}
				<span slot="owners" style="display: contents">
					<AssetOwnerPicker {owners} onadd={addOwner} />
				</span>
			{/if}

			{#each manualLinks as link, i (`m-${i}-${link.url}`)}
				<mm-button slot="links" size="sm" href={link.url} external>
					<mm-icon slot="icon" name={linkIcon(link.icon)} aria-hidden="true"></mm-icon>
					{link.name}
				</mm-button>
			{/each}
			{#each ruleLinks as link, i (`r-${i}-${link.url}`)}
				<mm-button slot="links" size="sm" href={link.url} external>
					<mm-icon slot="icon" name={linkIcon(link.icon)} aria-hidden="true"></mm-icon>
					{link.name}
				</mm-button>
			{/each}
			{#if canManageAssets}
				<span slot="links" style="display: contents">
					<AssetLinksEditor bind:links={asset.external_links} assetId={asset.id} />
				</span>
			{/if}

			<span slot="actions" style="display: contents">
				<SubscribeButton assetId={asset.id} />
			</span>
			{#if canManageAssets}
				<mm-menu-button
					slot="actions"
					variant="secondary"
					icon="more-horiz"
					icon-only
					label={m.asset_page_more_actions({ name: asset.name })}
				>
					<mm-menu onmm-select={onMoreSelect}>
						<mm-menu-item value="edit-notes" icon="edit-note"
							>{m.asset_page_edit_notes()}</mm-menu-item
						>
						<mm-menu-separator></mm-menu-separator>
						<mm-menu-item value="delete" icon="delete" danger
							>{m.asset_page_delete_asset()}</mm-menu-item
						>
					</mm-menu>
				</mm-menu-button>
			{/if}
			{#if canQueryData && isTable}
				<mm-button slot="actions" variant="primary" icon="terminal" href={queryHref}
					>{m.asset_page_query_table()}</mm-button
				>
			{/if}
		</mm-page-header>

		<div class="mm-stack" style="--mm-gap: var(--space-6)">
			<mm-tabs label={m.asset_page_tabs_label({ name: asset.name })} value={activeTab}>
				{#each tabs as t (t.id)}
					<mm-tab value={t.id} href={tabHref(t.id)} icon={t.icon} count={t.count}>{t.label}</mm-tab>
				{/each}
			</mm-tabs>

			{#if activeTab === 'documentation'}
				<DocumentationSystem entityType="asset" entityId={asset.mrn} />
			{:else if activeTab === 'metadata'}
				<AssetMetadata {asset} />
			{:else if activeTab === 'runs'}
				<AgentRunsView {asset} />
			{:else if activeTab === 'query'}
				<mm-code-sample
					language={(asset.query_language || 'sql').toLowerCase()}
					code={asset.query ?? ''}
					label={m.discover_tab_query()}
					wrap
				></mm-code-sample>
			{:else if activeTab === 'preview'}
				<DataPreviewTable
					columnNames={previewData?.column_names ?? []}
					rows={previewData?.rows ?? []}
					loading={previewLoading}
					error={previewError}
					onretry={fetchPreviewData}
				/>
			{:else if activeTab === 'queries' && asset.mrn}
				<AssetQueryPanel mrn={asset.mrn} name={asset.name ?? ''} provider={providerKey(asset)} />
			{:else if activeTab === 'environments'}
				<AssetEnvironmentsView environments={asset.environments ?? {}} />
			{:else if activeTab === 'schema'}
				<SchemaEditor {asset} />
			{:else if activeTab === 'run-history'}
				<AssetRunHistory {asset} />
			{:else if activeTab === 'contents'}
				<AssetContents {children} {parents} loading={loadingChildren} />
			{:else if activeTab === 'lineage'}
				<Lineage currentAsset={asset} />
			{/if}
		</div>
	</div>

	<!-- The Lineage tab opens its own node blade, so the asset blade steps aside there to keep one blade on screen. -->
	{#if activeTab !== 'lineage'}
		<BladeSlot label={m.asset_page_blade_heading()} persistent>
			<AssetPageBlade {asset} collapsed={bladeCollapsed} oncollapse={(c) => (bladeOverride = c)} />
		</BladeSlot>
	{/if}

	<mm-confirm-dialog
		bind:this={deleteDialog}
		heading={m.asset_page_delete_heading({ name: asset.name })}
		confirm-label={m.asset_page_delete_asset()}
		confirm-text={asset.name}
		loading={deleting}
		onmm-confirm={onDeleteConfirm}
	>
		<mm-text variant="dense" block>{m.asset_page_delete_body()}</mm-text>
	</mm-confirm-dialog>
{/if}
