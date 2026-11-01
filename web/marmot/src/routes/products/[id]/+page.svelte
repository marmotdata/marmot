<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import '@marmotdata/elements/popularity.js';
	import '@marmotdata/elements/proportion-bar.js';
	import { toast } from '@marmotdata/elements/toast.js';
	import { fetchApi } from '$lib/api';
	import { setCrumbs } from '$lib/mm';
	import { auth } from '$lib/stores/auth';
	import { m } from '$lib/paraglide/messages';
	import type { DataProduct, Member, Owner, Rule } from '$lib/dataproducts/types';
	import { getProductMembers } from '$lib/dataproducts/api';
	import { assetRouteParams, normaliseProduct, popularityLevel } from '$lib/dataproducts/assets';
	import { plainQuery } from '$lib/dataproducts/plain';
	import MetadataView from '$components/shared/MetadataView.svelte';
	import DocumentationSystem from '$components/docs/DocumentationSystem.svelte';
	import IconUploader from '$components/product/IconUploader.svelte';
	import OwnerField from '$components/product/OwnerField.svelte';
	import TagField from '$components/product/TagField.svelte';
	import ProductAssets from '$components/product/ProductAssets.svelte';
	import ProductRules from '$components/product/ProductRules.svelte';

	type Tab = 'overview' | 'assets' | 'rules' | 'documentation' | 'metadata';
	const TABS: Tab[] = ['overview', 'assets', 'rules', 'documentation', 'metadata'];
	const MOST_USED = 5;

	let productId = $derived(page.params.id ?? '');
	let tab = $state<Tab>(readTab());

	let product = $state<DataProduct | null>(null);
	let loading = $state(true);
	let loadError = $state<string | null>(null);
	let editingRule = $state<string | null>(null);

	let editing = $state(false);
	let draft = $state({ name: '', description: '', owners: [] as Owner[], tags: [] as string[] });
	let nameError = $state('');
	let saving = $state(false);
	let saveError = $state('');

	let confirmDelete = $state(false);
	let deleting = $state(false);

	let mostUsed = $state<Member[]>([]);

	const canManage = auth.hasPermission('assets', 'manage');

	$effect(() => {
		setCrumbs([
			{ label: m.products_heading(), href: resolve('/products') },
			{ label: product?.name ?? '' }
		]);
	});

	$effect(() => {
		if (productId) load(productId);
	});

	function readTab(): Tab {
		const value = page.url.searchParams.get('tab') as Tab | null;
		return value && TABS.includes(value) ? value : 'overview';
	}

	function selectTab(value: string) {
		if (!TABS.includes(value as Tab)) return;
		tab = value as Tab;
		// The tab lives in the URL so it survives a reload and can be shared; the path is resolved and only the query string is added.
		// eslint-disable-next-line svelte/no-navigation-without-resolve
		goto(`${resolve('/products/[id]', { id: productId })}?tab=${tab}`, {
			replaceState: true,
			noScroll: true,
			keepFocus: true
		});
	}

	async function load(id: string, quiet = false) {
		if (!quiet) loading = true;
		loadError = null;
		try {
			const res = await fetchApi(`/products/${id}`);
			if (!res.ok) {
				const data = await res.json().catch(() => ({}));
				throw new Error(data.error || m.products_error_load());
			}
			product = normaliseProduct(await res.json());
			loadMostUsed(id);
		} catch (e) {
			loadError = e instanceof Error ? e.message : m.products_error_load();
		} finally {
			loading = false;
		}
	}

	async function loadMostUsed(id: string) {
		try {
			const res = await getProductMembers(id, { sort: 'popular', limit: MOST_USED });
			mostUsed = res.members;
		} catch {
			mostUsed = [];
		}
	}

	function memberCaption(member: Member): string {
		const type = member.type.replace(/_/g, ' ');
		const kind = type.charAt(0).toUpperCase() + type.slice(1);
		return [kind, member.rule_name ?? m.products_member_manual()].join(' · ');
	}

	const reload = () => load(productId, true);

	function startEditing() {
		if (!product) return;
		draft = {
			name: product.name,
			description: product.description ?? '',
			owners: [...product.owners],
			tags: [...product.tags]
		};
		nameError = '';
		saveError = '';
		editing = true;
	}

	async function saveEdits(event: Event) {
		event.preventDefault();
		if (!product) return;
		if (!draft.name.trim()) {
			nameError = m.products_name_required();
			return;
		}
		saving = true;
		saveError = '';
		try {
			const res = await fetchApi(`/products/${product.id}`, {
				method: 'PUT',
				body: JSON.stringify({
					name: draft.name.trim(),
					description: draft.description.trim() || null,
					owners: draft.owners.map((o) => ({ id: o.id, type: o.type })),
					tags: draft.tags
				})
			});
			if (!res.ok) {
				const data = await res.json().catch(() => ({}));
				throw new Error(data.error || m.products_error_update());
			}
			product = normaliseProduct({ ...(await res.json()), icon_url: product.icon_url });
			editing = false;
		} catch (e) {
			saveError = e instanceof Error ? e.message : m.products_error_update();
		} finally {
			saving = false;
		}
	}

	async function copyLink() {
		try {
			await navigator.clipboard.writeText(window.location.href);
			toast({ tone: 'success', message: m.products_link_copied() });
		} catch {
			toast({ tone: 'danger', message: m.products_link_copy_failed() });
		}
	}

	function handleMore(event: CustomEvent<{ value: string }>) {
		if (event.detail.value === 'copy') copyLink();
		else if (event.detail.value === 'delete') confirmDelete = true;
	}

	async function deleteProduct(event: Event) {
		if (!product) return;
		event.preventDefault();
		deleting = true;
		try {
			const res = await fetchApi(`/products/${product.id}`, { method: 'DELETE' });
			if (!res.ok) {
				const data = await res.json().catch(() => ({}));
				throw new Error(data.error || m.products_error_delete());
			}
			toast({ tone: 'success', message: m.products_deleted({ name: product.name }) });
			confirmDelete = false;
			goto(resolve('/products'));
		} catch (e) {
			toast({
				tone: 'danger',
				message: e instanceof Error ? e.message : m.products_error_delete()
			});
		} finally {
			deleting = false;
		}
	}

	function editRule(rule: Rule) {
		editingRule = rule.id;
		selectTab('rules');
	}
</script>

<svelte:head>
	<title>{product ? `${product.name} - Marmot` : m.products_page_title()}</title>
</svelte:head>

{#if loading}
	<mm-skeleton loading count={6}></mm-skeleton>
{:else if loadError || !product}
	<mm-callout tone="danger" heading={m.products_load_failed_title()}>
		{loadError}
		<mm-button slot="actions" size="sm" icon="refresh" onclick={() => load(productId)}
			>{m.products_try_again()}</mm-button
		>
		<mm-button slot="actions" size="sm" variant="ghost" href={resolve('/products')}
			>{m.products_back_to_products()}</mm-button
		>
	</mm-callout>
{:else}
	{@const p = product}
	<div class="mm-stack" style="--mm-gap: var(--space-6)">
		{#if editing}
			<form class="mm-stack edit" style="--mm-gap: var(--space-5)" onsubmit={saveEdits} novalidate>
				<mm-heading level={1} variant="heading">{m.products_edit_product()}</mm-heading>
				<!-- svelte-ignore a11y_autofocus -->
				<mm-text-field
					label={m.common_name()}
					required
					value={draft.name}
					error={nameError}
					autofocus
					oninput={(e: Event) => {
						draft.name = (e.target as HTMLInputElement).value;
						if (draft.name.trim()) nameError = '';
					}}
				></mm-text-field>
				<OwnerField bind:owners={draft.owners} label={m.common_owners()} />
				<mm-textarea
					label={m.common_description()}
					optional
					rows={3}
					value={draft.description}
					placeholder={m.products_description_placeholder()}
					oninput={(e: Event) => (draft.description = (e.target as HTMLTextAreaElement).value)}
				></mm-textarea>
				<TagField bind:tags={draft.tags} label={m.common_tags()} />
				{#if saveError}
					<mm-callout tone="danger" compact heading={saveError}></mm-callout>
				{/if}
				<div class="mm-cluster" style="--mm-gap: var(--space-2); justify-content: flex-end">
					<mm-button onclick={() => (editing = false)} disabled={saving}
						>{m.common_cancel()}</mm-button
					>
					<mm-button variant="primary" type="submit" loading={saving}
						>{m.products_save_changes()}</mm-button
					>
				</div>
			</form>
		{:else}
			<mm-page-header heading={p.name} description={p.description ?? ''}>
				<span slot="leading">
					<IconUploader
						productId={p.id}
						currentIconUrl={p.icon_url}
						disabled={!canManage}
						size="lg"
						onIconChange={(url) => {
							if (product) product.icon_url = url ?? undefined;
						}}
					/>
				</span>
				{#each p.tags as tag (tag)}
					<mm-tag slot="tags">{tag}</mm-tag>
				{/each}
				{#if p.owners.length > 0}
					<span slot="owners" class="mm-cluster">
						{#each p.owners as owner (owner.type + owner.id)}
							<mm-owner-chip
								name={owner.name || owner.username || owner.id}
								kind={owner.type === 'team' ? 'team' : 'person'}
								src={owner.profile_picture ?? ''}
							></mm-owner-chip>
						{/each}
					</span>
				{/if}
				<mm-menu-button
					slot="actions"
					variant="ghost"
					icon="more-horiz"
					icon-only
					label={m.products_more_for({ name: p.name })}
				>
					<mm-menu onmm-select={handleMore}>
						<mm-menu-item value="copy" icon="link">{m.products_copy_link()}</mm-menu-item>
						{#if canManage}
							<mm-menu-separator></mm-menu-separator>
							<mm-menu-item value="delete" icon="delete" danger
								>{m.products_delete_product()}</mm-menu-item
							>
						{/if}
					</mm-menu>
				</mm-menu-button>
				{#if canManage}
					<mm-button slot="actions" icon="edit" onclick={startEditing}
						>{m.products_edit_product()}</mm-button
					>
				{/if}
			</mm-page-header>
		{/if}

		<mm-tabs
			label={m.products_tabs_label()}
			value={tab}
			onmm-tab-change={(e: CustomEvent<{ value: string }>) => selectTab(e.detail.value)}
		>
			<mm-tab value="overview" icon="space-dashboard">{m.common_overview()}</mm-tab>
			<mm-tab value="assets" icon="dataset" count={p.asset_count ?? 0}>{m.products_assets()}</mm-tab
			>
			<mm-tab value="rules" icon="rule" count={p.rules.length}>{m.products_tab_rules()}</mm-tab>
			<mm-tab value="documentation" icon="description">{m.common_documentation()}</mm-tab>
			<mm-tab value="metadata" icon="data-object">{m.products_tab_metadata()}</mm-tab>

			<mm-tab-panel value="overview">
				{#if tab === 'overview'}
					<div class="mm-split overview">
						<div class="mm-stack" style="--mm-gap: 0">
							<mm-section heading={m.products_blade_inside()}>
								{#if (p.asset_types ?? []).length > 0}
									<mm-proportion-bar
										label={m.products_assets_by_type()}
										data={(p.asset_types ?? []).map((t) => ({
											label: t.type.replace(/_/g, ' '),
											value: t.count
										}))}
										highlight={(p.asset_types ?? [])[0]?.type.replace(/_/g, ' ') ?? ''}
										category-label={m.common_type()}
										unit={m.products_assets_unit()}
									></mm-proportion-bar>
								{:else}
									<mm-text variant="dense" tone="secondary">{m.products_no_assets_title()}</mm-text>
								{/if}
							</mm-section>
							{#if mostUsed.length > 0}
								<mm-section heading={m.products_most_used()}>
									<mm-button
										slot="actions"
										size="sm"
										variant="ghost"
										onclick={() => selectTab('assets')}>{m.products_all_assets()}</mm-button
									>
									<mm-list
										aria-label={m.products_most_used_label({ name: p.name })}
										style="--mm-list-inset: 0"
									>
										{#each mostUsed as member (member.asset_id)}
											{@const params = assetRouteParams(member)}
											{@const level = popularityLevel(member.popularity)}
											<mm-list-item
												href={params
													? resolve('/discover/[type]/[service]/[name]', params)
													: undefined}
												secondary={memberCaption(member)}
											>
												<mm-provider-logo
													slot="start"
													provider={member.providers.length === 1
														? member.providers[0].toLowerCase()
														: member.type}
													size={20}
													decorative
												></mm-provider-logo>
												{member.name}
												{#if level}<mm-popularity slot="end" {level}></mm-popularity>{/if}
											</mm-list-item>
										{/each}
									</mm-list>
								</mm-section>
							{/if}
							{#if p.description}
								<mm-section heading={m.products_blade_about()}>
									<mm-button
										slot="actions"
										size="sm"
										variant="ghost"
										onclick={() => selectTab('documentation')}>{m.products_read_docs()}</mm-button
									>
									<p class="mm-dense mm-measure about">{p.description}</p>
								</mm-section>
							{/if}
						</div>
						<div class="mm-stack" style="--mm-gap: 0">
							{#if p.owners.length > 0}
								<mm-section heading={m.common_owners()}>
									<div class="mm-stack" style="--mm-gap: var(--space-3)">
										{#each p.owners as owner (owner.type + owner.id)}
											<mm-owner-chip
												name={owner.name || owner.username || owner.id}
												kind={owner.type === 'team' ? 'team' : 'person'}
												src={owner.profile_picture ?? ''}
												href={owner.type === 'team' ? resolve('/teams/[id]', { id: owner.id }) : ''}
											></mm-owner-chip>
										{/each}
									</div>
								</mm-section>
							{/if}
							<mm-section heading={m.products_tab_rules()}>
								{#if canManage}
									<mm-button
										slot="actions"
										size="sm"
										variant="ghost"
										onclick={() => selectTab('rules')}>{m.products_edit_rules()}</mm-button
									>
								{/if}
								{#if p.rules.length > 0}
									<mm-list dense aria-label={m.products_tab_rules()} style="--mm-list-inset: 0">
										{#each p.rules as rule (rule.id)}
											<mm-list-item
												secondary={plainQuery(rule.query_expression) ?? rule.query_expression ?? ''}
											>
												<mm-icon slot="start" name="rule" aria-hidden="true"></mm-icon>
												{rule.name}
												<mm-text slot="end" variant="caption" tone="secondary" numeric>
													{rule.is_enabled
														? m.products_assets_count({ count: rule.matched_asset_count ?? 0 })
														: m.products_rule_off()}
												</mm-text>
											</mm-list-item>
										{/each}
									</mm-list>
								{:else}
									<mm-text variant="dense" tone="secondary">{m.products_no_rules_title()}</mm-text>
								{/if}
							</mm-section>
						</div>
					</div>
				{/if}
			</mm-tab-panel>

			<mm-tab-panel value="assets">
				{#if tab === 'assets'}
					<ProductAssets product={p} {canManage} onChanged={reload} onEditRule={editRule} />
				{/if}
			</mm-tab-panel>

			<mm-tab-panel value="rules">
				{#if tab === 'rules'}
					<ProductRules product={p} {canManage} bind:editing={editingRule} onChanged={reload} />
				{/if}
			</mm-tab-panel>

			<mm-tab-panel value="documentation">
				{#if tab === 'documentation'}
					<div class="docs">
						<DocumentationSystem entityType="data_product" entityId={p.id} />
					</div>
				{/if}
			</mm-tab-panel>

			<mm-tab-panel value="metadata">
				{#if tab === 'metadata'}
					<MetadataView
						metadata={p.metadata}
						endpoint="/products"
						id={p.id}
						readOnly={!canManage}
						permissionResource="assets"
						permissionAction="manage"
					/>
				{/if}
			</mm-tab-panel>
		</mm-tabs>
	</div>

	<mm-confirm-dialog
		open={confirmDelete}
		heading={m.products_delete_title()}
		body={m.products_delete_confirm_message({ name: p.name })}
		confirm-label={m.products_delete_product()}
		confirm-icon="delete"
		loading={deleting}
		onmm-confirm={deleteProduct}
		onmm-close={() => (confirmDelete = false)}
	></mm-confirm-dialog>
{/if}

<style>
	.edit {
		max-width: 44rem;
	}
	.overview {
		padding-block-start: var(--space-2);
	}
	.about {
		margin: 0;
		white-space: pre-wrap;
	}
	.docs {
		height: calc(100vh - 320px);
		min-height: 400px;
		padding-block-start: var(--space-4);
	}
</style>
